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

// 门户套餐开关：show_in_portal=false 的套餐不出现在列表里，也不能被客户下单购买。
func TestListPlansHidesPortalDisabledTemplates(t *testing.T) {
	ctx := context.Background()
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "plans.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := d.AutoMigrate(&entity.PlanTemplate{}, &entity.PlanVariant{}, &entity.NodeGroupMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tmplRepo := repository.NewPlanTemplateRepository(d)
	variantRepo := repository.NewPlanVariantRepository(d)

	visible := &entity.PlanTemplate{Name: "public", Enabled: true}
	hidden := &entity.PlanTemplate{Name: "self-use", Enabled: true, HiddenFromPortal: true}
	for _, tmpl := range []*entity.PlanTemplate{visible, hidden} {
		if err := tmplRepo.Create(ctx, tmpl); err != nil {
			t.Fatalf("create template: %v", err)
		}
		if err := variantRepo.Create(ctx, &entity.PlanVariant{
			TemplateID: tmpl.ID, VariantType: entity.VariantTypePlan,
			Name: "monthly", Price: 1000, Currency: "CNY", Enabled: true,
		}); err != nil {
			t.Fatalf("create variant: %v", err)
		}
	}

	svc := &CommerceService{db: d, tmplRepo: tmplRepo, variantRepo: variantRepo}
	items, err := svc.ListPlans(ctx)
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(items) != 1 || items[0].ID != visible.ID {
		t.Fatalf("门户应只返回公开套餐，got %+v", items)
	}
}

// 隐藏套餐即使拿到 variant_id 也不能通过 charge/create 下单（自用套餐不被外部购买）。
func TestCreateOrderRejectsHiddenPlanPurchase(t *testing.T) {
	ctx := context.Background()
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "plans.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := d.AutoMigrate(&entity.PlanTemplate{}, &entity.PlanVariant{}, &entity.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tmplRepo := repository.NewPlanTemplateRepository(d)
	variantRepo := repository.NewPlanVariantRepository(d)
	clientRepo := repository.NewClientRepository(d)

	tmpl := &entity.PlanTemplate{Name: "self-use", Enabled: true, HiddenFromPortal: true}
	if err := tmplRepo.Create(ctx, tmpl); err != nil {
		t.Fatal(err)
	}
	variant := &entity.PlanVariant{TemplateID: tmpl.ID, VariantType: entity.VariantTypePlan, Name: "m", Price: 1000, Currency: "CNY", Enabled: true}
	if err := variantRepo.Create(ctx, variant); err != nil {
		t.Fatal(err)
	}
	client := &entity.Client{Email: "buyer@example.com", Password: "x", Status: "active"}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatal(err)
	}

	svc := &CommerceService{db: d, tmplRepo: tmplRepo, variantRepo: variantRepo, clientRepo: clientRepo}
	if _, err := svc.CreateOrder(ctx, client.ID, CreateOrderInput{VariantID: variant.ID, OrderType: entity.OrderTypeNew}); err == nil {
		t.Fatal("隐藏套餐不应允许客户新购")
	}
}
