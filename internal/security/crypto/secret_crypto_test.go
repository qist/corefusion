package crypto

import (
	"strings"
	"testing"
)

// 凭证类明文（订阅 token / 短链 code）需要确定性密文：同一明文必须得到同一密文，
// 才能继续用 WHERE token = ? 精确查询；同时仍然可以正常解密。
func TestEncryptSecretDeterministic(t *testing.T) {
	const plain = "0123456789abcdef0123456789abcdef"

	first, err := EncryptSecretDeterministic(plain)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncryptSecretDeterministic(plain)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("同一明文应得到同一密文：%q != %q", first, second)
	}
	if !strings.HasPrefix(first, "enc:") {
		t.Fatalf("密文缺少 enc: 前缀：%q", first)
	}
	if strings.Contains(first, plain) {
		t.Fatalf("密文里不应出现明文：%q", first)
	}

	got, err := DecryptSecret(first)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("解密结果 = %q, want %q", got, plain)
	}

	other, err := EncryptSecretDeterministic("fedcba9876543210")
	if err != nil {
		t.Fatal(err)
	}
	if other == first {
		t.Fatal("不同明文不应得到同一密文")
	}

	// 已经是密文时不得二次加密
	again, err := EncryptSecretDeterministic(first)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Fatalf("已加密值被二次加密：%q -> %q", first, again)
	}
}

// 指纹是单向的：同一明文指纹固定、长度 64 hex、不可反推。
func TestFingerprintSecret(t *testing.T) {
	const plain = "abcdefghjk0123456789"
	a, err := FingerprintSecret(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := FingerprintSecret(plain)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("同一明文指纹不稳定：%q != %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("指纹长度 = %d, want 64", len(a))
	}
	if strings.Contains(a, plain) {
		t.Fatalf("指纹里不应出现明文：%q", a)
	}
	other, err := FingerprintSecret("abcdefghjk0123456780")
	if err != nil {
		t.Fatal(err)
	}
	if other == a {
		t.Fatal("不同明文不应得到同一指纹")
	}
	if empty, err := FingerprintSecret(""); err != nil || empty != "" {
		t.Fatalf("空明文指纹 = %q, err=%v", empty, err)
	}
}
