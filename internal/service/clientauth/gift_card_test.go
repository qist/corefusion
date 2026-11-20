package clientauth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newGiftCardTestService(t *testing.T) (*CommerceService, *entity.PlanTemplate, *entity.PlanVariant) {
	t.Helper()
	d, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "gift.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := d.AutoMigrate(&entity.PlanTemplate{}, &entity.PlanVariant{}, &entity.ChargeCard{}, &entity.Client{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tmplRepo := repository.NewPlanTemplateRepository(d)
	variantRepo := repository.NewPlanVariantRepository(d)

	tmpl := &entity.PlanTemplate{Name: "标准套餐", Enabled: true}
	if err := tmplRepo.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	variant := &entity.PlanVariant{
		TemplateID: tmpl.ID, VariantType: entity.VariantTypePlan,
		Name: "月付", BillingCycle: "monthly", DurationDays: 30,
		Price: 990, Currency: "CNY", QuotaBytes: 100 << 30, Enabled: true,
	}
	if err := variantRepo.Create(context.Background(), variant); err != nil {
		t.Fatalf("create variant: %v", err)
	}
	svc := &CommerceService{
		db:             d,
		tmplRepo:       tmplRepo,
		variantRepo:    variantRepo,
		chargeCardRepo: repository.NewChargeCardRepository(d),
	}
	return svc, tmpl, variant
}

// 新卡：plan_variant_id 存绑定规格，value 存价格快照。
func TestChargeCardVariantIDNewAndLegacy(t *testing.T) {
	id := int64(11)
	if got := chargeCardVariantID(&entity.ChargeCard{CardType: entity.ChargeCardTypePlan, Value: 990, PlanVariantID: &id}); got != id {
		t.Fatalf("新卡应取 plan_variant_id，得到 %d", got)
	}
	// 旧卡：plan_variant_id 为空，value 就是规格 ID
	if got := chargeCardVariantID(&entity.ChargeCard{CardType: entity.ChargeCardTypePlan, Value: 11}); got != 11 {
		t.Fatalf("旧卡应回退 value=规格ID，得到 %d", got)
	}
	if got := chargeCardVariantID(&entity.ChargeCard{CardType: entity.ChargeCardTypeBalance, Value: 1000}); got != 0 {
		t.Fatalf("金额卡没有规格，得到 %d", got)
	}
	if got := chargeCardVariantID(nil); got != 0 {
		t.Fatalf("nil 卡应返回 0，得到 %d", got)
	}
}

// 门户预览：卡号 → 对应套餐名/规格/价格，前端据此自动选中。
func TestPreviewGiftCard(t *testing.T) {
	ctx := context.Background()
	svc, tmpl, variant := newGiftCardTestService(t)

	card := &entity.ChargeCard{
		CardCode:      "CF-TEST-PREVIEW-0001",
		CardType:      entity.ChargeCardTypePlan,
		Value:         variant.Price,
		Currency:      variant.Currency,
		PlanVariantID: &variant.ID,
		Status:        entity.ChargeCardStatusActive,
	}
	if err := svc.chargeCardRepo.Create(ctx, card); err != nil {
		t.Fatalf("create card: %v", err)
	}

	preview, err := svc.PreviewGiftCard(ctx, "CF-TEST-PREVIEW-0001")
	if err != nil {
		t.Fatalf("PreviewGiftCard: %v", err)
	}
	if preview.VariantID != variant.ID || preview.PlanID != tmpl.ID {
		t.Fatalf("预览绑定了错误的套餐：variant=%d plan=%d", preview.VariantID, preview.PlanID)
	}
	if preview.PlanName != tmpl.Name || preview.VariantName != variant.Name {
		t.Fatalf("预览名称不对：plan=%q variant=%q", preview.PlanName, preview.VariantName)
	}
	if preview.Price != variant.Price || preview.FaceValue != variant.Price {
		t.Fatalf("预览价格不对：price=%d face=%d", preview.Price, preview.FaceValue)
	}
	if preview.IsTrafficPack {
		t.Fatal("套餐卡不应标记为流量包卡")
	}

	// 已使用 / 已过期的卡不可预览
	card.Status = entity.ChargeCardStatusUsed
	card.UsedCount = 1
	if err := svc.chargeCardRepo.Update(ctx, card); err != nil {
		t.Fatalf("update card: %v", err)
	}
	if _, err := svc.PreviewGiftCard(ctx, "CF-TEST-PREVIEW-0001"); err == nil {
		t.Fatal("已使用的卡不应通过预览")
	}

	expired := &entity.ChargeCard{
		CardCode:      "CF-TEST-PREVIEW-0002",
		CardType:      entity.ChargeCardTypePlan,
		Value:         variant.Price,
		Currency:      variant.Currency,
		PlanVariantID: &variant.ID,
		Status:        entity.ChargeCardStatusActive,
		ExpiresAt:     func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
	}
	if err := svc.chargeCardRepo.Create(ctx, expired); err != nil {
		t.Fatalf("create expired card: %v", err)
	}
	if _, err := svc.PreviewGiftCard(ctx, "CF-TEST-PREVIEW-0002"); err == nil {
		t.Fatal("已过期的卡不应通过预览")
	}

	// 旧卡（value=规格ID）也能预览
	legacy := &entity.ChargeCard{
		CardCode: "CF-TEST-LEGACY-0001",
		CardType: entity.ChargeCardTypePlan,
		Value:    variant.ID,
		Currency: variant.Currency,
		Status:   entity.ChargeCardStatusActive,
	}
	if err := svc.chargeCardRepo.Create(ctx, legacy); err != nil {
		t.Fatalf("create legacy card: %v", err)
	}
	preview, err = svc.PreviewGiftCard(ctx, "CF-TEST-LEGACY-0001")
	if err != nil {
		t.Fatalf("旧卡预览应可用：%v", err)
	}
	if preview.VariantID != variant.ID {
		t.Fatalf("旧卡解析规格错误：%d", preview.VariantID)
	}
}
