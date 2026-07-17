package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrKeyNotSet is returned when the encryption key environment variable is not set.
var ErrKeyNotSet = errors.New("CF_SECRET_ENCRYPTION_KEY not set")

// defaultKey is loaded once from the environment variable.
var defaultKey []byte

// getKey returns the encryption key, loading it from the environment on first use.
// The key must be a 32-byte hex string (64 hex chars).
func getKey() ([]byte, error) {
	if defaultKey != nil {
		return defaultKey, nil
	}
	raw := os.Getenv("CF_SECRET_ENCRYPTION_KEY")
	if raw == "" {
		return nil, ErrKeyNotSet
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("CF_SECRET_ENCRYPTION_KEY must be 64 hex chars (32 bytes), got %d bytes", len(key))
	}
	defaultKey = key
	return key, nil
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
	return fmt.Sprintf("enc:%s:%s",
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
	if !strings.HasPrefix(encrypted, "enc:") {
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
