package charge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// 套餐卡必须绑定有效规格：缺参/类型不符要拒绝，普通金额卡不需要规格。
func TestResolveCardVariant(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "card.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.PlanVariant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	plan := &entity.PlanVariant{TemplateID: 1, VariantType: entity.VariantTypePlan, Name: "monthly", Price: 990, Currency: "CNY", Enabled: true}
	pack := &entity.PlanVariant{TemplateID: 0, VariantType: entity.VariantTypeTrafficPack, Name: "10G", Price: 500, Currency: "CNY", Enabled: true}
	for _, v := range []*entity.PlanVariant{plan, pack} {
		if err := db.Create(v).Error; err != nil {
			t.Fatalf("create variant: %v", err)
		}
	}

	h := &Handler{db: db}
	ctx := context.Background()

	if _, err := h.resolveCardVariant(ctx, entity.ChargeCardTypePlan, nil); err == nil {
		t.Fatal("plan 卡未传规格应被拒绝")
	}
	planID := plan.ID
	if _, err := h.resolveCardVariant(ctx, entity.ChargeCardTypeTrafficPack, &planID); err == nil {
		t.Fatal("流量包卡绑定 plan 规格应被拒绝")
	}
	missing := int64(99999)
	if _, err := h.resolveCardVariant(ctx, entity.ChargeCardTypePlan, &missing); err == nil {
		t.Fatal("不存在的规格应被拒绝")
	}

	got, err := h.resolveCardVariant(ctx, entity.ChargeCardTypePlan, &planID)
	if err != nil {
		t.Fatalf("合法规格不应报错：%v", err)
	}
	if got.Price != 990 || got.Currency != "CNY" {
		t.Fatalf("规格价格快照不对：price=%d currency=%q", got.Price, got.Currency)
	}

	if v, err := h.resolveCardVariant(ctx, entity.ChargeCardTypeBalance, nil); err != nil || v != nil {
		t.Fatalf("金额卡不需要规格：v=%v err=%v", v, err)
	}
}
