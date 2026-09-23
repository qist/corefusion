package migration

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

func newMigrationTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

// decodeDNSEnvVars 按**读路径**还原凭据（DecryptSecret → base64 → JSON ✓），
// 与 service/certificate/channel.go 的 decodeEnvVars 保持一致 ✓。
func decodeDNSEnvVars(t *testing.T, stored string) map[string]string {
	t.Helper()
	plain, err := crypto.DecryptSecret(stored)
	if err != nil {
		t.Fatalf("解密失败：%v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(plain)
	if err != nil {
		t.Fatalf("base64 解码失败：%v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(decoded, &m); err != nil {
		t.Fatalf("JSON 解析失败：%v", err)
	}
	return m
}

// 存量 DNS 渠道凭据（明文 base64）必须在启动迁移后被加密 ✓，且语义不变 ✓、幂等 ✓。
func TestMigrateDNSChannelEncryption(t *testing.T) {
	db := newMigrationTestDB(t, &entity.DNSChannel{})

	const token = "y3whwemmJU78D-IhZPekX-DWaIcbJ22fifPe3g7S"
	plainVars := map[string]string{"CF_Token": token, "CF_Account_ID": "acct-999"}
	raw, _ := json.Marshal(plainVars)
	legacyBase64 := base64.StdEncoding.EncodeToString(raw) // 升级前的存法 ✓

	legacy := &entity.DNSChannel{Name: "legacy", Provider: "cloudflare", EnvVars: legacyBase64, Status: "active"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}

	// 已经加密过的行不得被"二次加密"（否则读路径会解出乱码 ✗）
	alreadyEnc, err := crypto.EncryptSecret(legacyBase64)
	if err != nil {
		t.Fatal(err)
	}
	enc := &entity.DNSChannel{Name: "already-enc", Provider: "cloudflare", EnvVars: alreadyEnc, Status: "active"}
	if err := db.Create(enc).Error; err != nil {
		t.Fatal(err)
	}

	migrateDNSChannelEncryption(db)

	var gotLegacy entity.DNSChannel
	if err := db.First(&gotLegacy, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	// ① 迁完必须带 enc: 前缀，且库里不再有明文 token ✓
	if !strings.HasPrefix(gotLegacy.EnvVars, "enc:") {
		t.Fatalf("存量凭据未被加密：%q", gotLegacy.EnvVars)
	}
	if strings.Contains(gotLegacy.EnvVars, token) || strings.Contains(gotLegacy.EnvVars, legacyBase64) {
		t.Errorf("密文中仍能看到明文：%q", gotLegacy.EnvVars)
	}
	// ② 语义不变：读路径必须还原出原来的键值 ✓
	vars := decodeDNSEnvVars(t, gotLegacy.EnvVars)
	if vars["CF_Token"] != token || vars["CF_Account_ID"] != "acct-999" {
		t.Errorf("迁移改变了凭据语义：%+v", vars)
	}

	// ③ 已加密的行原样不动 ✓
	var gotEnc entity.DNSChannel
	if err := db.First(&gotEnc, enc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotEnc.EnvVars != alreadyEnc {
		t.Errorf("已加密行被重复加密了：%q", gotEnc.EnvVars)
	}
	if v := decodeDNSEnvVars(t, gotEnc.EnvVars); v["CF_Token"] != token {
		t.Errorf("已加密行解出错误：%+v", v)
	}

	// ④ 幂等：再跑一次结果不变 ✓
	before := gotLegacy.EnvVars
	migrateDNSChannelEncryption(db)
	var again entity.DNSChannel
	if err := db.First(&again, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if again.EnvVars != before {
		t.Errorf("迁移不幂等：%q → %q", before, again.EnvVars)
	}
	if v := decodeDNSEnvVars(t, again.EnvVars); v["CF_Token"] != token {
		t.Errorf("二次迁移后解不出原值：%+v", v)
	}
}
