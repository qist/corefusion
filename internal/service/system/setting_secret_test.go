package system

import (
	"os"
	"testing"
)

// TestSettingSecretEncryption 验证敏感键（SMTP 密码 / Telegram token / OAuth secret）
// 与充值卡密钥 / TOTP secret 共用同一套 AES-256-GCM（crypto.EncryptSecret）加密落库。
func TestSettingSecretEncryption(t *testing.T) {
	os.Setenv("CF_SECRET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	cases := []struct {
		key   string
		value string
	}{
		{"notify.smtp_pass", "smtp-secret-123"},
		{"notify.telegram_bot_token", "1234567890:AAHtesttoken"},
		{"oauth.github_client_secret", "ghs_client_secret_abc"},
		{"oauth.google_client_secret", "goc_client_secret_xyz"},
	}

	for _, tc := range cases {
		enc, err := encryptSensitiveValue(tc.key, tc.value)
		if err != nil {
			t.Fatalf("%s: encrypt failed: %v", tc.key, err)
		}
		if enc == tc.value {
			t.Fatalf("%s: 敏感值未加密，仍为明文: %q", tc.key, tc.value)
		}
		plain := decryptSensitiveValue(tc.key, enc)
		if plain != tc.value {
			t.Fatalf("%s: 解密结果不一致: got %q want %q", tc.key, plain, tc.value)
		}
	}
}

// TestSettingSecretBackwardCompat 验证存量明文读取向后兼容（未加密值原样返回）。
func TestSettingSecretBackwardCompat(t *testing.T) {
	os.Setenv("CF_SECRET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	legacy := decryptSensitiveValue("notify.smtp_pass", "plain-legacy-password")
	if legacy != "plain-legacy-password" {
		t.Fatalf("legacy plaintext must be returned as-is, got %q", legacy)
	}

	// 非敏感键不加密
	enc, err := encryptSensitiveValue("panel.title", "hello")
	if err != nil {
		t.Fatalf("non-sensitive key should not error: %v", err)
	}
	if enc != "hello" {
		t.Fatalf("non-sensitive key must stay plaintext, got %q", enc)
	}
}

// TestSettingSecretIdempotent 验证重复加密不会叠加（enc: 前缀防双层密文）。
func TestSettingSecretIdempotent(t *testing.T) {
	os.Setenv("CF_SECRET_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")

	first, err := encryptSensitiveValue("oauth.github_client_secret", "ghs_abc")
	if err != nil {
		t.Fatal(err)
	}
	second, err := encryptSensitiveValue("oauth.github_client_secret", first)
	if err != nil {
		t.Fatal(err)
	}
	// 已加密值再写入必须原样保留（不是再包一层）
	if second != first {
		t.Fatalf("已加密值被二次加密: %q -> %q", first, second)
	}

	plain := decryptSensitiveValue("oauth.github_client_secret", second)
	if plain != "ghs_abc" {
		t.Fatalf("解密结果不一致: got %q want %q", plain, "ghs_abc")
	}
}
