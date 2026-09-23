package migration

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

// 存量 EAB 凭据（明文 HMAC-Key / GCP 服务账号 JSON）必须在启动迁移后被加密 ✓，且语义不变 ✓、幂等 ✓。
func TestMigrateEABConfigEncryption(t *testing.T) {
	db := newMigrationTestDB(t, &entity.EABConfig{})

	const macKey = "TESTONLY-eab-mac-key-not-a-real-credential"
	const saJSON = `{"type":"service_account","project_id":"demo-gcp-project",` +
		`"private_key":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkq\n-----END PRIVATE KEY-----\n"}`

	// 历史行：两项都是明文（升级前就是这样存的）
	legacy := &entity.EABConfig{
		Name:            "Google Public CA - demo-gcp-project",
		CAProvider:      "google",
		KeyID:           "0123456789abcdef0123456789abcdef",
		MACKey:          macKey,
		GoogleProjectID: "demo-gcp-project",
		GoogleSAJSON:    saJSON,
		Status:          "active",
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}

	// 已经加密的行不得被"二次加密" ✗
	alreadyMAC, err := crypto.EncryptSecret(macKey)
	if err != nil {
		t.Fatal(err)
	}
	enc := &entity.EABConfig{
		Name: "ZeroSSL - already", CAProvider: "zerossl", KeyID: "kid",
		MACKey: alreadyMAC, Status: "active",
	}
	if err := db.Create(enc).Error; err != nil {
		t.Fatal(err)
	}

	migrateEABConfigEncryption(db)

	var gotLegacy entity.EABConfig
	if err := db.First(&gotLegacy, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	// ① 两项都要变成密文，且库里看不到明文
	if !strings.HasPrefix(gotLegacy.MACKey, "enc:") {
		t.Fatalf("mac_key 未加密：%q", gotLegacy.MACKey)
	}
	if !strings.HasPrefix(gotLegacy.GoogleSAJSON, "enc:") {
		t.Fatalf("google_sa_json 未加密：%q", gotLegacy.GoogleSAJSON)
	}
	for _, plain := range []string{macKey, "BEGIN PRIVATE KEY", "demo-gcp-project"} {
		if strings.Contains(gotLegacy.MACKey, plain) || strings.Contains(gotLegacy.GoogleSAJSON, plain) {
			t.Errorf("密文中仍能看到明文片段 %q", plain)
		}
	}

	// ② 语义不变：按**读路径**（DecryptSecret）必须还原出原值 ✓
	macPlane, err := crypto.DecryptSecret(gotLegacy.MACKey)
	if err != nil || macPlane != macKey {
		t.Errorf("mac_key 还原失败：%q (err=%v)", macPlane, err)
	}
	saPlane, err := crypto.DecryptSecret(gotLegacy.GoogleSAJSON)
	if err != nil || saPlane != saJSON {
		t.Errorf("google_sa_json 还原失败：%q (err=%v)", saPlane, err)
	}
	// 其余字段不受影响 ✓
	if gotLegacy.GoogleProjectID != "demo-gcp-project" || gotLegacy.KeyID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("迁移改动了非凭据字段：%+v", gotLegacy)
	}

	// ③ 已加密的行原样不动 ✓
	var gotEnc entity.EABConfig
	if err := db.First(&gotEnc, enc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotEnc.MACKey != alreadyMAC {
		t.Errorf("已加密行被重复加密：%q", gotEnc.MACKey)
	}
	if macPlane, err := crypto.DecryptSecret(gotEnc.MACKey); err != nil || macPlane != macKey {
		t.Errorf("已加密行解出错误：%q (err=%v)", macPlane, err)
	}

	// ④ 幂等：再跑一次结果不变 ✓
	beforeMAC, beforeSA := gotLegacy.MACKey, gotLegacy.GoogleSAJSON
	migrateEABConfigEncryption(db)
	var again entity.EABConfig
	if err := db.First(&again, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if again.MACKey != beforeMAC || again.GoogleSAJSON != beforeSA {
		t.Errorf("迁移不幂等：mac %q → %q", beforeMAC, again.MACKey)
	}
	if macPlane, err := crypto.DecryptSecret(again.MACKey); err != nil || macPlane != macKey {
		t.Errorf("二次迁移后解不出原值：%q (err=%v)", macPlane, err)
	}
}
