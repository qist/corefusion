package migration

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
)

// 存量账号绑定的 EAB MAC 必须在启动迁移后转成密文 ✓，语义不变 ✓，幂等 ✓。
func TestMigrateACMEAccountConfigEncryption(t *testing.T) {
	db := newMigrationTestDB(t, &entity.ACMEAccountConfig{})

	const mac = "S30IILacK_mOa6pdcByyIIRxKL_KbSZelD3UsxAeaXge1DExample"

	// 历史行：明文（升级前就是这个样子）。直接 db.Create 绕过 repository ✓ 正是要模拟的状态 ✓
	legacy := &entity.ACMEAccountConfig{
		Email: "test@example.com", Status: "active", CAProvider: "zerossl",
		EABKeyID: "koQayFUN4XpLF-8udnQjSQ", EABMACKey: mac,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}

	// 已加密的行不得被二次加密 ✗
	already, err := crypto.EncryptSecret(mac)
	if err != nil {
		t.Fatal(err)
	}
	enc := &entity.ACMEAccountConfig{
		Email: "zero2@example.com", Status: "active", CAProvider: "zerossl", EABMACKey: already,
	}
	if err := db.Create(enc).Error; err != nil {
		t.Fatal(err)
	}

	migrateACMEAccountConfigEncryption(db)

	var got entity.ACMEAccountConfig
	if err := db.First(&got, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.EABMACKey, "enc:") {
		t.Fatalf("eabmac_key 未加密：%q", got.EABMACKey)
	}
	if strings.Contains(got.EABMACKey, mac[:24]) {
		t.Error("密文中仍能看到明文片段")
	}
	plain, err := crypto.DecryptSecret(got.EABMACKey)
	if err != nil || plain != mac {
		t.Errorf("按读路径还原失败：%q (err=%v)", plain, err)
	}
	// 非凭据字段不受影响 ✓
	if got.EABKeyID != "koQayFUN4XpLF-8udnQjSQ" || got.Email != "test@example.com" || got.CAProvider != "zerossl" {
		t.Errorf("迁移改动了非凭据字段：%+v", got)
	}

	var gotEnc entity.ACMEAccountConfig
	if err := db.First(&gotEnc, enc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotEnc.EABMACKey != already {
		t.Errorf("已加密行被重复加密：%q", gotEnc.EABMACKey)
	}
	if p, err := crypto.DecryptSecret(gotEnc.EABMACKey); err != nil || p != mac {
		t.Errorf("已加密行解出错误：%q (err=%v)", p, err)
	}

	// 幂等 ✓
	before := got.EABMACKey
	migrateACMEAccountConfigEncryption(db)
	var again entity.ACMEAccountConfig
	if err := db.First(&again, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if again.EABMACKey != before {
		t.Errorf("迁移不幂等：%q → %q", before, again.EABMACKey)
	}
	if p, err := crypto.DecryptSecret(again.EABMACKey); err != nil || p != mac {
		t.Errorf("二次迁移后解不出原值：%q (err=%v)", p, err)
	}
}
