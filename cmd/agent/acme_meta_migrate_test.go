package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"corefusion/internal/security/crypto"
)

// encryptWithGivenKey 用指定密钥产出与 crypto.EncryptSecret 相同格式的 `enc:` 密文。
func encryptWithGivenKey(t *testing.T, key []byte, plaintext string) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("nonce: %v", err)
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return fmt.Sprintf("enc:%s:%s",
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ct))
}

func legacyHostnameKeyForAgentTest(t *testing.T) []byte {
	t.Helper()
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "corefusion-default"
	}
	sum := sha256.Sum256([]byte(host + "cf-secret-salt-v1"))
	return sum[:]
}

// TestMigrateCertMetaEncryption 锁定节点侧存量迁移：
// 升级前 meta.json 里的 DNS 凭据是历史主机名派生密钥加密的 →
// 启动迁移要解出来用当前本地密钥重写，否则换机/回退逻辑变化后该证书永久无法自续。
func TestMigrateCertMetaEncryption(t *testing.T) {
	dataDir := t.TempDir()
	name := "migtest.example.com"
	dir := filepath.Join(dataDir, "certs", name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	const token = "cf-real-dns-token"
	meta := certMeta{
		Identifiers: []string{name},
		Email:       "admin@example.com",
		CA:          "lets_encrypt",
		Challenge:   "dns-01",
		DNSProvider: "cloudflare",
		DNSEnvVars: map[string]string{
			"CF_Token": encryptWithGivenKey(t, legacyHostnameKeyForAgentTest(t), token),
		},
	}
	raw, err := json.MarshalIndent(&meta, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(dir, certMetaFile)
	if err := os.WriteFile(metaPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// 当前本地密钥：固定为随机值，与主机名派生密钥必然不同。
	primary := make([]byte, 32)
	if _, err := rand.Read(primary); err != nil {
		t.Fatal(err)
	}
	if err := crypto.SetKey(hex.EncodeToString(primary)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = crypto.SetKey(hex.EncodeToString(legacyHostnameKeyForAgentTest(t)))
	})

	migrateCertMetaEncryption(dataDir)

	migrated, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(migrated) == string(raw) {
		t.Fatal("历史主机名密钥密文没有被重写")
	}

	var onDisk certMeta
	if err := json.Unmarshal(migrated, &onDisk); err != nil {
		t.Fatal(err)
	}
	if !crypto.CanDecryptWithPrimary(onDisk.DNSEnvVars["CF_Token"]) {
		t.Error("迁移后的密文仍不能由当前本地密钥解开")
	}
	// 读路径必须还原出原凭据 ✓
	read, err := readCertMeta(dataDir, name)
	if err != nil {
		t.Fatalf("迁移后读回失败：%v", err)
	}
	if read.DNSEnvVars["CF_Token"] != token {
		t.Errorf("迁移改变了凭据：%q", read.DNSEnvVars["CF_Token"])
	}
	if read.Challenge != "dns-01" || read.DNSProvider != "cloudflare" {
		t.Errorf("迁移损坏了续期参数：%+v", read)
	}

	// 幂等 ✓
	migrateCertMetaEncryption(dataDir)
	again, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(migrated) {
		t.Error("迁移不幂等：第二次运行又改写了 meta.json")
	}
}
