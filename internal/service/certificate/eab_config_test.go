package certificate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	sqlitedrv "github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func setupEABSvc(t *testing.T) *EABConfigService {
	t.Helper()
	db, err := gorm.Open(sqlitedrv.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&entity.EABConfig{}); err != nil {
		t.Fatal(err)
	}
	return NewEABConfigService(repository.NewEABConfigRepository(db))
}

// 核心：EAB 做成独立配置后必须**存得住、取得到** ✓（用户要的"不用每次填写" ✗）
func TestEABConfigCRUD(t *testing.T) {
	svc := setupEABSvc(t)
	ctx := context.Background()

	c, err := svc.Create(ctx, EABConfigInput{
		Name: "Google CA - qist", CAProvider: "google", KeyID: "kid-1", MACKey: "mac-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.CAProvider != "google" || c.KeyID != "kid-1" || c.MACKey != "mac-1" {
		t.Fatalf("创建后内容不对：%+v", c)
	}
	if !c.HasMACKey {
		t.Fatalf("应标记 has_mac_key ✓")
	}

	got, err := svc.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.KeyID != "kid-1" || got.MACKey != "mac-1" {
		t.Fatalf("按 ID 取回不一致：%+v", got)
	}

	upd, err := svc.Update(ctx, c.ID, EABConfigInput{Name: "改名", CAProvider: "zerossl", KeyID: "kid-2"})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != "改名" || upd.CAProvider != "zerossl" || upd.KeyID != "kid-2" {
		t.Fatalf("更新失败：%+v", upd)
	}
	if upd.MACKey != "mac-1" {
		t.Fatalf("HMAC-Key 留空应保持原值 ✓，实际 %q", upd.MACKey)
	}

	if err := svc.Delete(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetByID(ctx, c.ID); err == nil {
		t.Fatalf("删除后不该还能取到 ✗")
	}
}

// HMAC-Key **永不外泄** ✓（前端只看 has_mac_key ✓）
func TestEABConfigNeverLeaksMACKey(t *testing.T) {
	svc := setupEABSvc(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, EABConfigInput{
		Name: "g", CAProvider: "google", KeyID: "kid", MACKey: "SUPER-SECRET-MAC",
	}); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "SUPER-SECRET-MAC") {
		t.Fatalf("HMAC-Key 泄漏 ✗：%s", raw)
	}
	if !strings.Contains(string(raw), `"has_mac_key":true`) {
		t.Fatalf("应带 has_mac_key ✓：%s", raw)
	}
}

func TestEABConfigRequiresName(t *testing.T) {
	svc := setupEABSvc(t)
	if _, err := svc.Create(context.Background(), EABConfigInput{CAProvider: "google", KeyID: "k"}); err == nil {
		t.Fatalf("没有名字应报错 ✗")
	}
}

func TestEABConfigDefaultCA(t *testing.T) {
	svc := setupEABSvc(t)
	c, err := svc.Create(context.Background(), EABConfigInput{Name: "x", KeyID: "k", MACKey: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if c.CAProvider != "lets_encrypt" {
		t.Fatalf("CA 留空应默认 lets_encrypt ✓，实际 %q", c.CAProvider)
	}
}
