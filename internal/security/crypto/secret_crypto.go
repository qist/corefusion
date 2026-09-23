package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// defaultKey is loaded once from the environment variable or auto-generated.
var defaultKey []byte

// getKey returns the encryption key, loading it from the environment on first use.
// If not set, generates a default key from hostname (consistent across restarts).
// The key must be a 32-byte hex string (64 hex chars).
func getKey() ([]byte, error) {
	if defaultKey != nil {
		return defaultKey, nil
	}
	raw := os.Getenv("CF_SECRET_ENCRYPTION_KEY")
	if raw == "" {
		// Auto-generate default key from hostname + fixed salt
		defaultKey = generateDefaultKey()
		return defaultKey, nil
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("CF_SECRET_ENCRYPTION_KEY must be 64 hex chars (32 bytes), got %d bytes", len(key))
	}
	defaultKey = key
	return key, nil
}

// ExportKey returns the current encryption key as a hex string.
// This is used during backup to include the key in the archive so that
// CA private keys can be decrypted on a different machine during restore.
func ExportKey() (string, error) {
	key, err := getKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// SetKey overrides the encryption key with the given hex string.
// This is used during restore to temporarily use the source machine's key
// so that encrypted secrets (like CA private keys) can be decrypted.
func SetKey(hexKey string) error {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return fmt.Errorf("invalid key: must be 64 hex chars (32 bytes)")
	}
	defaultKey = key
	return nil
}

// generateDefaultKey generates a deterministic 32-byte key from hostname.
// This ensures the same key is generated across restarts on the same machine.
func generateDefaultKey() []byte {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "corefusion-default"
	}
	// Use SHA-256 to derive 32 bytes from hostname + salt
	salt := "cf-secret-salt-v1"
	h := sha256.Sum256([]byte(hostname + salt))
	return h[:]
}

// secretPrefix 是密文的固定前缀 ✓ —— EncryptSecret 产出它 ✓、DecryptSecret 靠它区分"已加密/未加密" ✓。
const secretPrefix = "enc:"

// IsEncrypted 判断一个值是否**已经是** EncryptSecret 的产物 ✓。
//
// 用途：避免把密文**再包一层** ✗ —— 双层密文解一次仍是密文 ✗，
// 现象是"凭据明明存进去了、用的时候却说无效" ✗，排查成本极高 ✗。
// 典型场景：从库里读出的行未经解密就被回写 ✓ → 写路径动手前应先问一句 ✓。
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, secretPrefix)
}

// EncryptSecret encrypts a plaintext secret using AES-256-GCM.
// Returns a string in the format: "enc:<base64_nonce>:<base64_ciphertext>"
func EncryptSecret(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return fmt.Sprintf("%s%s:%s", secretPrefix,
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	), nil
}

// DecryptSecret decrypts a string in the format "enc:<base64_nonce>:<base64_ciphertext>".
// If the input does not have the "enc:" prefix, it returns the input as-is (backward compatible).
func DecryptSecret(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	// Backward compatibility: if not encrypted, return as-is
	if !IsEncrypted(encrypted) {
		return encrypted, nil
	}

	key, err := getKey()
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(encrypted, ":", 3)
	if len(parts) != 3 {
		return "", errors.New("invalid encrypted secret format")
	}

	nonce, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// MaskSecret masks a secret for display, showing only the first 4 chars.
// Example: "v3b92xxxx" → "v3b9****"
func MaskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:4] + "****"
}

// HashSecret returns the SHA-256 hex hash of a secret.
// Used for indexed lookup of encrypted secrets (e.g. charge card codes).
func HashSecret(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

// MaskCardCode masks a card code for display, showing first and last 4 chars.
// Example: "CF-a1b2c3d4-e5f6g7h8" → "CF-a1****g7h8"
func MaskCardCode(code string) string {
	if len(code) <= 8 {
		return "****"
	}
	return code[:4] + "****" + code[len(code)-4:]
}
