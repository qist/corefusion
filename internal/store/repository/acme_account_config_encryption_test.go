package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

func newACMECfgTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.ACMEAccountConfig{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

const acmeCfgMAC = "S30IILacK_mOa6pdcByyIIRxKL_KbSZelD3UsxAeaXge1DExample"

// 账号绑定的 EAB MAC 必须加密落库 ✓，而 service / 签发链路拿到的**永远是明文** ✓。
func TestACMEAccountConfigEABMACEncryptedAtRest(t *testing.T) {
	db := newACMECfgTestDB(t)
	repo := NewACMEAccountConfigRepository(db)
	ctx := context.Background()

	cfg := &entity.ACMEAccountConfig{
		Email:      "test@example.com",
		IsDefault:  true,
		Status:     "active",
		CAProvider: "zerossl",
		EABKeyID:   "koQayFUN4XpLF-8udnQjSQ",
		EABMACKey:  acmeCfgMAC,
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	// ① service 会把 cfg 直接返回给前端 → ID 必须回填（否则前端拿到 id=0 ✗）
	if cfg.ID == 0 {
		t.Error("Create 未回填 ID")
	}
	// ② 调用方实体必须仍是明文（签发时会用这一份 ✓）
	if cfg.EABMACKey != acmeCfgMAC {
		t.Errorf("Create 改写了调用方实体的 MAC：%q", clip(cfg.EABMACKey))
	}

	// ③ 库内必须是密文
	var stored entity.ACMEAccountConfig
	if err := db.First(&stored, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.EABMACKey, "enc:") {
		t.Fatalf("eabmac_key 未加密落库：%q", clip(stored.EABMACKey))
	}
	if strings.Contains(stored.EABMACKey, acmeCfgMAC[:24]) {
		t.Error("密文中仍能看到明文片段")
	}

	// ④ 读路径给回明文（service 的 has_eab 判断依赖"明文非空" ✓）
	got, err := repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EABMACKey != acmeCfgMAC {
		t.Errorf("GetByID 未返回明文：%q", clip(got.EABMACKey))
	}

	def, err := repo.GetDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if def.EABMACKey != acmeCfgMAC || def.EABKeyID != "koQayFUN4XpLF-8udnQjSQ" {
		t.Errorf("GetDefault 未返回明文：mac=%q", clip(def.EABMACKey))
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].EABMACKey != acmeCfgMAC {
		t.Errorf("List 未返回明文：%+v", list)
	}

	// ⑤ 真实编辑路径：GetByID → 前端不回填密钥 → Update（"留空 = 保持原值"）→ 不得双层加密 ✗
	got.Email = "changed@example.com"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	var after entity.ACMEAccountConfig
	if err := db.First(&after, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(after.EABMACKey, "enc:"); n != 1 {
		t.Errorf("eabmac_key 被加密了 %d 层（应为 1）", n)
	}
	back, err := repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.EABMACKey != acmeCfgMAC {
		t.Errorf("回写后再读拿不到明文：%q", clip(back.EABMACKey))
	}
	if back.Email != "changed@example.com" {
		t.Errorf("普通字段更新丢失：%q", back.Email)
	}

	// ⑥ unsetDefaults 路径：List 出来的实体直接被 Save 回去 → 同样不得双包 ✗
	list, err = repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	item := list[0]
	item.IsDefault = false
	if err := repo.Update(ctx, &item); err != nil {
		t.Fatal(err)
	}
	var after2 entity.ACMEAccountConfig
	if err := db.First(&after2, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(after2.EABMACKey, "enc:"); n != 1 {
		t.Errorf("List→Update 路径导致加密 %d 层", n)
	}
}

// 没配 EAB 的账号（纯 Let's Encrypt）不该被写出一坨密文 ✗，空值必须保持空 ✓。
func TestACMEAccountConfigEmptyEABStaysEmpty(t *testing.T) {
	db := newACMECfgTestDB(t)
	repo := NewACMEAccountConfigRepository(db)
	ctx := context.Background()

	cfg := &entity.ACMEAccountConfig{
		Email: "let@example.com", Status: "active", CAProvider: "lets_encrypt",
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	var stored entity.ACMEAccountConfig
	if err := db.First(&stored, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.EABMACKey != "" {
		t.Errorf("空的 eabmac_key 被写成 %q", clip(stored.EABMACKey))
	}

	got, err := repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EABMACKey != "" {
		t.Errorf("空 MAC 读出来应为空，实际 %q", clip(got.EABMACKey))
	}
}
