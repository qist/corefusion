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

// TestValidateKeySpec 断言**lego 能生成**的规格才放行 ✓（与具体 CA 无关 ✓）。
// RSA 8192 本身可以生成 ✓ → 通用校验放行；能不能签出由 ValidateKeySpecForCA 判定 ✓。
func TestValidateKeySpec(t *testing.T) {
	ok := []struct {
		keyType string
		keyBits int
	}{
		{"", 0}, {"ecc", 0}, {"ecc", 256}, {"ecc", 384}, {"ECC", 384},
		{"rsa", 0}, {"rsa", 2048}, {"rsa", 3072}, {"rsa", 4096}, {"RSA", 4096}, {"rsa", 8192},
	}
	for _, c := range ok {
		if err := ValidateKeySpec(c.keyType, c.keyBits); err != nil {
			t.Errorf("ValidateKeySpec(%q,%d) 应通过，却报错：%v", c.keyType, c.keyBits, err)
		}
	}
	bad := []struct {
		keyType string
		keyBits int
	}{
		{"ecc", 521}, {"rsa", 1024}, {"rsa", 16384}, {"ecc", 4096},
	}
	for _, c := range bad {
		if err := ValidateKeySpec(c.keyType, c.keyBits); err == nil {
			t.Errorf("ValidateKeySpec(%q,%d) 应被拒绝，却通过了", c.keyType, c.keyBits)
		}
	}
}

// TestValidateKeySpecForCA 断言 RSA 8192 的上限**按 CA** 判定 ✓：
// ZeroSSL 实测支持 8192 ✓（2026-10-20 真实 DNS-01 签出），
// Let's Encrypt / Google 必须拒绝 ✗（LE 会在 finalize 阶段报 key size not supported）。
func TestValidateKeySpecForCA(t *testing.T) {
	cases := []struct {
		ca      CAProvider
		keyType string
		keyBits int
		wantErr bool
	}{
		{CAProviderZeroSSL, "rsa", 8192, false},
		{CAProvider("ZeroSSL"), "rsa", 8192, false},
		{CAProviderZeroSSL, "rsa", 4096, false},
		{CAProviderZeroSSL, "ecc", 384, false},
		{CAProviderLetsEncrypt, "rsa", 8192, true},
		{CAProvider(""), "rsa", 8192, true},
		{CAProviderLetsEncryptStaging, "rsa", 8192, true},
		{CAProvider("Let's Encrypt"), "rsa", 8192, true},
		{CAProviderGoogle, "rsa", 8192, true},
		{CAProvider("Google Trust Services"), "rsa", 8192, true},
		{CAProviderGoogle, "rsa", 4096, false},
		{CAProviderLetsEncrypt, "ecc", 521, true},
	}
	for _, c := range cases {
		err := ValidateKeySpecForCA(c.ca, c.keyType, c.keyBits)
		if c.wantErr && err == nil {
			t.Errorf("CA=%q %s/%d 应被拒绝，却通过了", c.ca, c.keyType, c.keyBits)
		}
		if !c.wantErr && err != nil {
			t.Errorf("CA=%q %s/%d 应通过，却报错：%v", c.ca, c.keyType, c.keyBits, err)
		}
	}
}

// TestNormalizeCAProvider 断言库里的展示名也能归到同一套 CA 枚举 ✓
// （`issuer` 列实测同时存在 `lets_encrypt` 与 `Let's Encrypt` / `Google Trust Services`）。
func TestNormalizeCAProvider(t *testing.T) {
	cases := map[string]CAProvider{
		"":                          CAProviderLetsEncrypt,
		"lets_encrypt":              CAProviderLetsEncrypt,
		"Let's Encrypt":             CAProviderLetsEncrypt,
		"lets_encrypt_staging":      CAProviderLetsEncryptStaging,
		"zerossl":                   CAProviderZeroSSL,
		"ZeroSSL":                   CAProviderZeroSSL,
		"ZeroSSL GmbH":              CAProviderZeroSSL,
		"google":                    CAProviderGoogle,
		"Google Trust Services":     CAProviderGoogle,
		"Google Trust Services LLC": CAProviderGoogle,
	}
	for in, want := range cases {
		if got := NormalizeCAProvider(in); got != want {
			t.Errorf("NormalizeCAProvider(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCertPathVariants 断言"域名目录 + 历史名称目录"都会被写到 ✓ ——
// name=gg / domain=example.com 时，站点引用的是 certs/gg/，只写 certs/example.com/ 等于没更新。
func TestCertPathVariants(t *testing.T) {
	dirs := func(vs []CertPaths) []string {
		out := make([]string, 0, len(vs))
		for _, v := range vs {
			out = append(out, v.Dir)
		}
		return out
	}
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	// ECC：域名目录在前，名称目录随后 ✓
	got := dirs(CertPathVariants("gg", "example.com", "ecc", "certs/example.com/cert.pem"))
	if want := []string{"certs/example.com", "certs/gg"}; !eq(got, want) {
		t.Errorf("ECC 别名目录 = %v, want %v", got, want)
	}
	// RSA：两套都进 rsa 子目录 ✓
	got = dirs(CertPathVariants("gg", "example.com", "rsa", "certs/example.com/rsa/cert.pem"))
	if want := []string{"certs/example.com/rsa", "certs/gg/rsa"}; !eq(got, want) {
		t.Errorf("RSA 别名目录 = %v, want %v", got, want)
	}
	// 名称 == 域名 → 只有一个目录（不产生重复 ✓）
	got = dirs(CertPathVariants("a.com", "a.com", "ecc", "certs/a.com/cert.pem"))
	if want := []string{"certs/a.com"}; !eq(got, want) {
		t.Errorf("同名证书目录 = %v, want %v", got, want)
	}
	// 库里的历史路径目录也要包含（老版本按消毒名落盘 ✓）
	got = dirs(CertPathVariants("acme-node-owner.example.com", "node-owner.example.com", "ecc", "certs/acme-node-owner_example_com/cert.pem"))
	if want := []string{"certs/node-owner.example.com", "certs/acme-node-owner.example.com", "certs/acme-node-owner_example_com"}; !eq(got, want) {
		t.Errorf("含历史路径的别名目录 = %v, want %v", got, want)
	}
	// 目录穿越的库路径必须被忽略 ✓
	got = dirs(CertPathVariants("gg", "example.com", "ecc", "../../etc/passwd"))
	for _, d := range got {
		if d == "../../etc" || d == "etc" || d == "/etc" {
			t.Fatalf("非法库路径不应产生目录：%v", got)
		}
	}

	v := CertPathVariants("gg", "example.com", "ecc", "")[0]
	if v.Cert != "certs/example.com/cert.pem" || v.Key != "certs/example.com/key.pem" || v.Fullchain != "certs/example.com/fullchain.pem" {
		t.Errorf("主路径文件名不对：%+v", v)
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
