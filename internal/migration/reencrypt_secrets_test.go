package migration

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

// encryptWithKey 用**指定**密钥产出与 crypto.EncryptSecret 完全相同的 `enc:` 格式。
//
// 为什么不直接用 crypto.EncryptSecret：迁移测试需要同时存在"旧主机名密钥"和
// "当前主密钥"两种密文，而 crypto 包的密钥是进程级单例（SetKey 会改全局）。
func encryptWithKey(t *testing.T, key []byte, plaintext string) string {
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

// legacyHostnameKeyForTest 复刻 crypto 包的历史主机名派生规则（sha256(host+"cf-secret-salt-v1")）。
func legacyHostnameKeyForTest(t *testing.T) []byte {
	t.Helper()
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "corefusion-default"
	}
	sum := sha256.Sum256([]byte(host + "cf-secret-salt-v1"))
	return sum[:]
}

// TestReencryptLegacySecrets 锁定存量密文迁移的行为：
//   - 旧主机名密钥加密的值 → 解出来用当前主密钥重写（否则换机/回退逻辑变化后永久解不开）；
//   - 已用当前主密钥加密的值 → 原样保留（绝不重复加密）；
//   - 明文/非密文 → 不触碰；
//   - 幂等：跑第二次不再变化。
func TestReencryptLegacySecrets(t *testing.T) {
	db := newMigrationTestDB(t, &entity.DNSChannel{})

	const plaintext = "CF_Token=real-secret-token"
	legacyEnc := encryptWithKey(t, legacyHostnameKeyForTest(t), plaintext)

	// 当前主密钥：测试里显式固定，保证"主密钥"与主机名派生密钥必然不同。
	primaryKey := make([]byte, 32)
	if _, err := rand.Read(primaryKey); err != nil {
		t.Fatalf("random key: %v", err)
	}
	if err := crypto.SetKey(hex.EncodeToString(primaryKey)); err != nil {
		t.Fatalf("set key: %v", err)
	}
	t.Cleanup(func() {
		// 恢复成测试环境的默认口径（无环境变量时的主机名派生），避免影响同包其他测试。
		_ = crypto.SetKey(hex.EncodeToString(legacyHostnameKeyForTest(t)))
	})

	primaryEnc, err := crypto.EncryptSecret("already-current-key")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	legacyRow := &entity.DNSChannel{Name: "legacy", Provider: "cloudflare", EnvVars: legacyEnc, Status: "active"}
	currentRow := &entity.DNSChannel{Name: "current", Provider: "cloudflare", EnvVars: primaryEnc, Status: "active"}
	plainRow := &entity.DNSChannel{Name: "plain", Provider: "cloudflare", EnvVars: "not-encrypted", Status: "active"}
	for _, r := range []*entity.DNSChannel{legacyRow, currentRow, plainRow} {
		if err := db.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	reencryptLegacySecrets(db)

	var gotLegacy, gotCurrent, gotPlain entity.DNSChannel
	if err := db.First(&gotLegacy, legacyRow.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotCurrent, currentRow.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotPlain, plainRow.ID).Error; err != nil {
		t.Fatal(err)
	}

	// ① 旧密文必须被重写（密文变化）且能还原出原文 ✓
	if gotLegacy.EnvVars == legacyEnc {
		t.Error("旧主机名密钥密文没有被重写")
	}
	plainOut, err := crypto.DecryptSecret(gotLegacy.EnvVars)
	if err != nil {
		t.Fatalf("重写后的密文解不开：%v", err)
	}
	if plainOut != plaintext {
		t.Errorf("重写改变了明文：%q → %q", plaintext, plainOut)
	}
	if !crypto.CanDecryptWithPrimary(gotLegacy.EnvVars) {
		t.Error("重写后的密文仍不能由当前主密钥解开")
	}

	// ② 已用当前主密钥加密的行不动 ✓
	if gotCurrent.EnvVars != primaryEnc {
		t.Error("已用当前主密钥加密的值被重复加密")
	}

	// ③ 非密文不动 ✓
	if gotPlain.EnvVars != "not-encrypted" {
		t.Errorf("普通字段被改坏：%q", gotPlain.EnvVars)
	}

	// ④ 幂等 ✓
	before := gotLegacy.EnvVars
	reencryptLegacySecrets(db)
	var again entity.DNSChannel
	if err := db.First(&again, legacyRow.ID).Error; err != nil {
		t.Fatal(err)
	}
	if again.EnvVars != before {
		t.Errorf("迁移不幂等：%q → %q", before, again.EnvVars)
	}
}
