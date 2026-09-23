package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

func newEABTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.EABConfig{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	return db
}

func clip(s string) string {
	if len(s) <= 30 {
		return s
	}
	return s[:30] + "…"
}

// EAB 凭据（EAB-HMAC-Key / GCP 服务账号 JSON）必须加密落库 ✓，
// 而对上层**始终以明文呈现** ✓（router 直接拿 cfg.MACKey 去注册 ACME ✓）。
func TestEABConfigCredentialsEncryptedAtRest(t *testing.T) {
	db := newEABTestDB(t)
	repo := NewEABConfigRepository(db)
	ctx := context.Background()

	const macKey = "TESTONLY-eab-mac-key-not-a-real-credential"
	const saJSON = `{"type":"service_account","project_id":"demo-gcp-project",` +
		`"private_key":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkq\n-----END PRIVATE KEY-----\n",` +
		`"client_email":"acme@demo-gcp-project.iam.gserviceaccount.com"}`

	cfg := &entity.EABConfig{
		Name:            "Google Public CA - demo-gcp-project",
		CAProvider:      "google",
		KeyID:           "0123456789abcdef0123456789abcdef",
		MACKey:          macKey,
		GoogleProjectID: "demo-gcp-project",
		GoogleSAJSON:    saJSON,
		Status:          "active",
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	// ① 调用方实体必须**仍是明文** ✗ —— MintGoogleEAB 会把同一份 cfg 立刻交给 ACME 注册，
	//    若被改写成密文，注册端只会报"EAB 无效"，根因极难定位。
	if cfg.MACKey != macKey {
		t.Errorf("Create 改写了调用方实体的 MACKey：%q", clip(cfg.MACKey))
	}
	if cfg.GoogleSAJSON != saJSON {
		t.Errorf("Create 改写了调用方实体的 GoogleSAJSON")
	}

	// ② 库里必须是密文，且看不到任何明文痕迹
	var stored entity.EABConfig
	if err := db.First(&stored, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored.MACKey, "enc:") {
		t.Fatalf("mac_key 未加密落库：%q", clip(stored.MACKey))
	}
	if !strings.HasPrefix(stored.GoogleSAJSON, "enc:") {
		t.Fatalf("google_sa_json 未加密落库：%q", clip(stored.GoogleSAJSON))
	}
	for _, plain := range []string{macKey, "BEGIN PRIVATE KEY", "client_email", "demo-gcp-project"} {
		if strings.Contains(stored.MACKey, plain) || strings.Contains(stored.GoogleSAJSON, plain) {
			t.Errorf("密文中仍能看到明文片段 %q", plain)
		}
	}

	// ③ 读路径必须给回明文（含多行 GCP 私钥原样还原 ✓）
	got, err := repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MACKey != macKey {
		t.Errorf("GetByID 未返回明文 MACKey：%q", clip(got.MACKey))
	}
	if got.GoogleSAJSON != saJSON {
		t.Errorf("GetByID 未返回明文 GoogleSAJSON：%q", clip(got.GoogleSAJSON))
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].MACKey != macKey || list[0].GoogleSAJSON != saJSON {
		t.Errorf("List 未返回明文凭据：%+v", list)
	}

	// ④ "读出来 → 改个名字 → 存回去"不得**双层加密** ✗（否则解一次还是密文 ✗）
	got.Name = "renamed"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	var stored2 entity.EABConfig
	if err := db.First(&stored2, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(stored2.MACKey, "enc:"); n != 1 {
		t.Errorf("mac_key 被加密了 %d 层（应为 1 层）：%q", n, clip(stored2.MACKey))
	}
	again, err := repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.MACKey != macKey || again.GoogleSAJSON != saJSON {
		t.Errorf("回写后再次读取拿不到明文：mac=%q", clip(again.MACKey))
	}
}

// 只填 KID/MAC 的 ZeroSSL 配置（没有 GCP 字段）也要能正常工作 ✓ —— 空值保持空 ✓，不该被写成一坨密文 ✗。
func TestEABConfigEmptyCredentialsStayEmpty(t *testing.T) {
	db := newEABTestDB(t)
	repo := NewEABConfigRepository(db)
	ctx := context.Background()

	cfg := &entity.EABConfig{
		Name:       "ZeroSSL - test@example.com",
		CAProvider: "zerossl",
		KeyID:      "koQayFUN4XpLF-8udnQjSQ",
		MACKey:     "S30IILacK_mOa6pdcByyIIRxD",
		Status:     "active",
	}
	if err := repo.Create(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	var stored entity.EABConfig
	if err := db.First(&stored, cfg.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.GoogleSAJSON != "" {
		t.Errorf("空的 google_sa_json 不该被写成 %q", clip(stored.GoogleSAJSON))
	}

	got, err := repo.GetByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.MACKey != cfg.MACKey {
		t.Errorf("MACKey 往返不一致：%q", clip(got.MACKey))
	}
	if got.GoogleSAJSON != "" {
		t.Errorf("空的 google_sa_json 读出来应为空，实际 %q", clip(got.GoogleSAJSON))
	}
}
