package clientauth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newSettingsTestService(t *testing.T) (*Service, *entity.Client, *entity.PlanTemplate) {
	t.Helper()
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := d.AutoMigrate(&entity.Client{}, &entity.PlanTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clientRepo := repository.NewClientRepository(d)
	tmplRepo := repository.NewPlanTemplateRepository(d)
	svc := &Service{clientRepo: clientRepo, tmplRepo: tmplRepo}

	ctx := context.Background()
	tmpl := &entity.PlanTemplate{Name: "basic", DeviceLimit: 3, Enabled: true}
	if err := tmplRepo.Create(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	client := &entity.Client{Email: "settings@example.com", Password: "x", Status: "active", PlanID: tmpl.ID, DeviceLimit: 1}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	return svc, client, tmpl
}

// 安全审计 v7 L-10：客户自助设置的设备数不能超过套餐上限。
func TestUpdateSettingsCannotRaiseDeviceLimitAbovePlan(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newSettingsTestService(t)

	if err := svc.UpdateSettings(ctx, client.ID, "", 10); err == nil {
		t.Fatal("超过套餐上限（3）应被拒绝")
	}
	if err := svc.UpdateSettings(ctx, client.ID, "", 3); err != nil {
		t.Fatalf("等于套餐上限应允许：%v", err)
	}
	got, err := svc.clientRepo.GetByID(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceLimit != 3 {
		t.Fatalf("device_limit = %d, want 3", got.DeviceLimit)
	}

	// 0 = 继承套餐默认值，始终允许
	if err := svc.UpdateSettings(ctx, client.ID, "", 0); err != nil {
		t.Fatalf("0 应允许（继承套餐）：%v", err)
	}
}

// 套餐不限设备数（0）时，客户可以自行设置任意设备数。
func TestUpdateSettingsAllowsAnyDeviceLimitWhenPlanUnlimited(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newSettingsTestService(t)

	tmpl, err := svc.tmplRepo.GetByID(ctx, client.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	tmpl.DeviceLimit = 0
	if err := svc.tmplRepo.Update(ctx, tmpl); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateSettings(ctx, client.ID, "", 100); err != nil {
		t.Fatalf("套餐不限设备数时应允许：%v", err)
	}
}

// IP 白名单仍由客户自助维护（只作用于订阅拉取），非法格式必须拒绝。
func TestUpdateSettingsValidatesIPWhitelist(t *testing.T) {
	ctx := context.Background()
	svc, client, _ := newSettingsTestService(t)

	if err := svc.UpdateSettings(ctx, client.ID, "not-an-ip", 0); err == nil {
		t.Fatal("非法 IP 白名单应被拒绝")
	}
	if err := svc.UpdateSettings(ctx, client.ID, "203.0.113.7, 198.51.100.0/24", 0); err != nil {
		t.Fatalf("合法 IP 白名单应允许：%v", err)
	}
}
