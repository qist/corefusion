package clientauth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 安全审计 v7 H-1：套餐订单金额必须由服务端按 variant.Price 定价，
// 客户端传 amount=1 也只能按真实价格建单。
func TestCreateChargeOrderUsesServerSideVariantPrice(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "commerce.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.ChargeSite{}, &entity.Client{}, &entity.ChargeOrder{}, &entity.PlanVariant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	paySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"payment_url":"https://pay.example.com/checkout/1"}`)
	}))
	defer paySrv.Close()

	siteRepo := repository.NewChargeSiteRepository(db)
	orderRepo := repository.NewChargeOrderRepository(db)
	clientRepo := repository.NewClientRepository(db)
	variantRepo := repository.NewPlanVariantRepository(db)

	site := &entity.ChargeSite{Name: "test", SiteURL: paySrv.URL, Currency: "CNY", Enabled: true, Verified: true}
	if err := siteRepo.Create(ctx, site); err != nil {
		t.Fatalf("create site: %v", err)
	}
	client := &entity.Client{Email: "buyer@example.com", Password: "x", Status: "active"}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	variant := &entity.PlanVariant{TemplateID: 1, VariantType: entity.VariantTypePlan, Name: "Pro", Price: 9900, Currency: "CNY", Enabled: true}
	if err := variantRepo.Create(ctx, variant); err != nil {
		t.Fatalf("create variant: %v", err)
	}

	svc := NewCommerceService(
		db, nil, variantRepo, nil, nil, clientRepo, nil,
		siteRepo, orderRepo, nil, nil, nil,
		fakeSettingStore{m: map[string]string{"panel.external_url": "https://panel.example.com"}},
		"test-secret",
	)

	res, err := svc.CreateChargeOrder(ctx, client.ID, CreateChargeInput{
		SiteID:        site.ID,
		Amount:        1, // 攻击者只付 1 分
		PlanVariantID: &variant.ID,
	})
	if err != nil {
		t.Fatalf("CreateChargeOrder: %v", err)
	}
	order, err := orderRepo.GetByTradeNo(ctx, res.TradeNo)
	if err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Amount != variant.Price {
		t.Fatalf("订单金额 = %d, want 服务端定价 %d（客户端金额必须被忽略）", order.Amount, variant.Price)
	}
	if order.OrderType != entity.ChargeOrderTypePurchase {
		t.Fatalf("订单类型 = %s, want %s", order.OrderType, entity.ChargeOrderTypePurchase)
	}
}

// 纯充值（无套餐）仍然按客户端金额建单。
func TestCreateChargeOrderKeepsRechargeAmount(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "commerce.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.ChargeSite{}, &entity.Client{}, &entity.ChargeOrder{}, &entity.PlanVariant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	paySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"payment_url":"https://pay.example.com/checkout/2"}`)
	}))
	defer paySrv.Close()

	siteRepo := repository.NewChargeSiteRepository(db)
	orderRepo := repository.NewChargeOrderRepository(db)
	clientRepo := repository.NewClientRepository(db)

	site := &entity.ChargeSite{Name: "test", SiteURL: paySrv.URL, Currency: "CNY", Enabled: true, Verified: true}
	if err := siteRepo.Create(ctx, site); err != nil {
		t.Fatalf("create site: %v", err)
	}
	client := &entity.Client{Email: "recharge@example.com", Password: "x", Status: "active"}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}

	svc := NewCommerceService(
		db, nil, repository.NewPlanVariantRepository(db), nil, nil, clientRepo, nil,
		siteRepo, orderRepo, nil, nil, nil,
		fakeSettingStore{m: map[string]string{"panel.external_url": "https://panel.example.com"}},
		"test-secret",
	)

	res, err := svc.CreateChargeOrder(ctx, client.ID, CreateChargeInput{SiteID: site.ID, Amount: 5000})
	if err != nil {
		t.Fatalf("CreateChargeOrder: %v", err)
	}
	order, err := orderRepo.GetByTradeNo(ctx, res.TradeNo)
	if err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Amount != 5000 {
		t.Fatalf("充值订单金额 = %d, want 5000", order.Amount)
	}
	if order.OrderType != entity.ChargeOrderTypeCharge {
		t.Fatalf("订单类型 = %s, want %s", order.OrderType, entity.ChargeOrderTypeCharge)
	}
}
