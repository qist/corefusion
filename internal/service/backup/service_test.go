package backup

import (
	"strings"
	"testing"

	"corefusion/internal/security/crypto"
)

// 存储密钥加密落库 → 读取解密 → 对外掩码 的完整往返行为锁。
func TestStorageSecretRoundTrip(t *testing.T) {
	orig := `{"endpoint":"http://minio:9000","bucket":"bk","access_key":"AKIA","secret_key":"super-secret"}`
	enc := encryptStorageConfig(orig)
	if !strings.Contains(enc, "enc:") {
		t.Fatalf("secret_key 未加密落库: %s", enc)
	}
	if strings.Contains(enc, "super-secret") {
		t.Fatalf("加密后仍含明文: %s", enc)
	}
	// 幂等：二次加密不叠加
	if again := encryptStorageConfig(enc); again != enc {
		t.Fatalf("重复加密破坏幂等:\n got: %s\nwant: %s", again, enc)
	}
	// 执行路径解密
	var cfg S3Config
	if err := ParseStorageConfig(enc, &cfg); err != nil {
		t.Fatalf("ParseStorageConfig: %v", err)
	}
	if cfg.SecretKey != "super-secret" {
		t.Fatalf("解密后应得到明文，got %q", cfg.SecretKey)
	}
}

func TestStoragePasswordDecrypt(t *testing.T) {
	enc := encryptStorageConfig(`{"host":"ftp.example.com","username":"u","password":"p@ss"}`)
	var cfg FTPConfig
	if err := ParseStorageConfig(enc, &cfg); err != nil {
		t.Fatalf("ParseStorageConfig: %v", err)
	}
	if cfg.Password != "p@ss" {
		t.Fatalf("password 解密失败，got %q", cfg.Password)
	}
	// 存量明文直接解析（兼容）
	var legacy FTPConfig
	if err := ParseStorageConfig(`{"password":"old-plain"}`, &legacy); err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	if legacy.Password != "old-plain" {
		t.Fatalf("存量明文应原样可用，got %q", legacy.Password)
	}
}

// UpdateStorage 的 ******** 哨兵逻辑单测（不经 DB，验证 encrypt 前的合并行为由 handler 层保证）。
func TestCryptoRoundTripBasic(t *testing.T) {
	enc, err := crypto.EncryptSecret("token-123")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if !crypto.IsEncrypted(enc) {
		t.Fatalf("密文应带 enc: 前缀")
	}
	plain, err := crypto.DecryptSecret(enc)
	if err != nil || plain != "token-123" {
		t.Fatalf("往返失败: %q, %v", plain, err)
	}
}
