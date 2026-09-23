package cert

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"path/filepath"
	"testing"

	"github.com/go-acme/lego/v4/certcrypto"
)

// TestResolveKeyType 覆盖 (key_type, key_bits) → lego 密钥类型的映射 ✓。
// ⚠ lego 只支持 EC256 / EC384 ✗（没有 EC521 ✓）→ 521 必须回退并告警 ✓。
func TestResolveKeyType(t *testing.T) {
	cases := []struct {
		keyType string
		keyBits int
		want    certcrypto.KeyType
	}{
		{"ecc", 256, certcrypto.EC256},
		{"ecc", 384, certcrypto.EC384},
		{"ECC", 384, certcrypto.EC384},
		{"ecc", 0, certcrypto.EC256},   // 未指定 → 历史默认 ✓
		{"", 0, certcrypto.EC256},      // 老数据/老请求 ✓ 保持 EC256 ✓
		{"ecc", 521, certcrypto.EC256}, // lego 不支持 ✗ → 回退 ✓
		{"rsa", 2048, certcrypto.RSA2048},
		{"rsa", 3072, certcrypto.RSA3072},
		{"rsa", 4096, certcrypto.RSA4096},
		{"rsa", 8192, certcrypto.RSA8192},
		{"RSA", 0, certcrypto.RSA2048}, // 未指定位数 → RSA2048 ✓
		{"ecc", 12345, certcrypto.EC256},
	}
	for _, c := range cases {
		if got := resolveKeyType(c.keyType, c.keyBits); got != c.want {
			t.Errorf("resolveKeyType(%q,%d) = %v, want %v", c.keyType, c.keyBits, got, c.want)
		}
	}
}

// TestResolveKeyTypeGeneratesRealKey 用 lego **自己的** GeneratePrivateKey 生成私钥并断言类型 ✓。
// 这一步很重要 ✓：lego 的 Obtain 内部正是用该函数按 config.Certificate.KeyType 生密钥 ✓，
// 所以"映射对了 + 真生成对了"就等于"签出来的证书就是那个类型" ✓。
func TestResolveKeyTypeGeneratesRealKey(t *testing.T) {
	cases := []struct {
		keyType string
		keyBits int
		bits    int
	}{
		{"ecc", 256, 256},
		{"ecc", 384, 384},
		{"rsa", 2048, 2048},
		{"rsa", 3072, 3072},
		// 4096/8192 生成太慢 ✗（秒级）→ 只在上面的映射表里覆盖 ✓
	}
	for _, c := range cases {
		kt := resolveKeyType(c.keyType, c.keyBits)
		key, err := certcrypto.GeneratePrivateKey(kt)
		if err != nil {
			t.Fatalf("GeneratePrivateKey(%v) failed: %v", kt, err)
		}
		switch k := key.(type) {
		case *ecdsa.PrivateKey:
			if c.keyType != "ecc" {
				t.Errorf("%s/%d 生成了 EC 密钥 ✗", c.keyType, c.keyBits)
			}
			if k.Curve.Params().BitSize != c.bits {
				t.Errorf("EC 密钥位数 = %d, want %d", k.Curve.Params().BitSize, c.bits)
			}
		case *rsa.PrivateKey:
			if c.keyType != "rsa" {
				t.Errorf("%s/%d 生成了 RSA 密钥 ✗", c.keyType, c.keyBits)
			}
			if k.N.BitLen() != c.bits {
				t.Errorf("RSA 密钥位数 = %d, want %d", k.N.BitLen(), c.bits)
			}
		default:
			t.Errorf("意料之外的密钥类型 %T", key)
		}
	}
}

// TestCertRelPaths 断言 RSA / ECC 落在**不同路径** ✓ —— 这正是"同时签发只剩一份"的修复点 ✓。
func TestCertRelPaths(t *testing.T) {
	cases := []struct {
		name, keyType, wantDir string
	}{
		{"a.com", "ecc", "certs/a.com"},         // 历史布局不变 ✓（已有引用不能断 ✗）
		{"a.com", "ECC", "certs/a.com"},         // 大小写不敏感 ✓
		{"a.com", "", "certs/a.com"},            // 空 = ecc ✓
		{"a.com", "rsa", "certs/a.com/rsa"},     // RSA 进子目录 ✓
		{"a.com", "RSA", "certs/a.com/rsa"},     //
		{"a.com", "rsa4096", "certs/a.com/rsa"}, // 兼容 rsa4096 之类的写法 ✓
	}
	for _, c := range cases {
		certRel, keyRel, fullRel := CertRelPaths(c.name, c.keyType)
		for _, p := range []string{certRel, keyRel, fullRel} {
			if got := filepath.Dir(p); got != c.wantDir {
				t.Errorf("CertRelPaths(%q,%q) 目录 = %q, want %q", c.name, c.keyType, got, c.wantDir)
			}
		}
		if filepath.Base(fullRel) != "fullchain.pem" {
			t.Errorf("fullchain 文件名 = %q", filepath.Base(fullRel))
		}
	}

	// ★ 核心断言：两种类型的路径**必须不同** ✗（否则互相覆盖 ✗）
	ecCert, ecKey, _ := CertRelPaths("a.com", "ecc")
	rsaCert, rsaKey, _ := CertRelPaths("a.com", "rsa")
	if ecCert == rsaCert || ecKey == rsaKey {
		t.Fatalf("ecc 与 rsa 路径相同 ✗ 会互相覆盖：%s vs %s", ecCert, rsaCert)
	}
}

func TestNormalizeKeyType(t *testing.T) {
	for in, want := range map[string]string{
		"": KeyTypeECC, "ecc": KeyTypeECC, "ECC": KeyTypeECC, " ecc ": KeyTypeECC,
		"rsa": KeyTypeRSA, "RSA": KeyTypeRSA, "rsa2048": KeyTypeRSA, "rsa8192": KeyTypeRSA,
	} {
		if got := NormalizeKeyType(in); got != want {
			t.Errorf("NormalizeKeyType(%q) = %q, want %q", in, got, want)
		}
	}
}
