package clientauth

import (
	"context"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

// 套餐「续费累加 / 升级到期切换」端到端验证：真实 service + 内存库，
// 覆盖 余额下单预约 → 回调预约 → 调度器切换 → 预约单退款 / 已生效退款 全链路。

type stubSettingStore struct{}

func (stubSettingStore) GetByKey(ctx context.Context, key string) (string, error) {
	return "", nil
}

func newFlowDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&entity.Client{},
		&entity.PlanTemplate{},
		&entity.PlanVariant{},
		&entity.PlanOrder{},
		&entity.Subscription{},
		&entity.ClientBalance{},
		&entity.BalanceTransaction{},
		&entity.ChargeOrder{},
		&entity.Promotion{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newCommerceForFlow(db *gorm.DB) *CommerceService {
	return NewCommerceService(
		db,
		repository.NewPlanTemplateRepository(db),
		repository.NewPlanVariantRepository(db),
		repository.NewPlanOrderRepository(db),
		repository.NewPromotionRepository(db),
		repository.NewClientRepository(db),
		repository.NewSubscriptionRepository(db),
		repository.NewChargeSiteRepository(db),
		repository.NewChargeOrderRepository(db),
		repository.NewChargeCardRepository(db),
		repository.NewClientBalanceRepository(db),
		repository.NewBalanceTransactionRepository(db),
		stubSettingStore{},
		"test-secret",
	)
}

func newPlanServiceForFlow(db *gorm.DB) *plan.Service {
	svc := plan.NewService(
		db,
		repository.NewPlanTemplateRepository(db),
		repository.NewPlanVariantRepository(db),
		repository.NewNodeGroupRepository(db),
		repository.NewNodeGroupMemberRepository(db),
		repository.NewPromotionRepository(db),
		repository.NewPlanOrderRepository(db),
		repository.NewClientRepository(db),
		repository.NewInboundConfigRepository(db),
		repository.NewExternalNodeRepository(db),
	)
	svc.SetSubscriptionRepo(repository.NewSubscriptionRepository(db))
	return svc
}

type flowFixture struct {
	db       *gorm.DB
	commerce *CommerceService
	plans    *plan.Service
	client   *entity.Client
	low      *entity.PlanVariant
	high     *entity.PlanVariant
	expiry   time.Time
}

func seedFlowFixture(t *testing.T) *flowFixture {
	t.Helper()
	db := newFlowDB(t)

	lowTmpl := &entity.PlanTemplate{Name: "基础版", Enabled: true, DefaultQuotaBytes: 100}
	highTmpl := &entity.PlanTemplate{Name: "高级版", Enabled: true, DefaultQuotaBytes: 300}
	if err := db.Create(lowTmpl).Error; err != nil {
		t.Fatalf("create low tmpl: %v", err)
	}
	if err := db.Create(highTmpl).Error; err != nil {
		t.Fatalf("create high tmpl: %v", err)
	}

	low := &entity.PlanVariant{TemplateID: lowTmpl.ID, Name: "基础-月付", DurationDays: 30, Price: 1000, QuotaBytes: 100, Enabled: true}
	high := &entity.PlanVariant{TemplateID: highTmpl.ID, Name: "高级-月付", DurationDays: 30, Price: 3000, QuotaBytes: 300, Enabled: true}
	if err := db.Create(low).Error; err != nil {
		t.Fatalf("create low variant: %v", err)
	}
	if err := db.Create(high).Error; err != nil {
		t.Fatalf("create high variant: %v", err)
	}

	expiry := time.Now().AddDate(0, 0, 10)
	client := &entity.Client{
		Email: "flow@example.com", Password: "x", Status: "active",
		PlanID: lowTmpl.ID, PlanVariantID: low.ID,
		QuotaBytes: 100, UsedBytes: 50, ExpiresAt: &expiry,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&entity.ClientBalance{ClientID: client.ID, Balance: 100000, Currency: "CNY"}).Error; err != nil {
		t.Fatalf("create balance: %v", err)
	}

	return &flowFixture{
		db: db, commerce: newCommerceForFlow(db), plans: newPlanServiceForFlow(db),
		client: client, low: low, high: high, expiry: expiry,
	}
}

// 余额下单升级：只落 scheduled 预约单，客户套餐/配额/到期时间/已用流量一律不动。
func TestBalanceUpgradeSchedulesWithoutTouchingClient(t *testing.T) {
	f := seedFlowFixture(t)
	ctx := context.Background()

	res, err := f.commerce.CreateOrder(ctx, f.client.ID, CreateOrderInput{
		VariantID: f.high.ID, OrderType: entity.OrderTypeUpgrade, PayMethod: "balance",
	})
	if err != nil {
		t.Fatalf("create upgrade order: %v", err)
	}
	if res.Status != entity.OrderStatusScheduled {
		t.Fatalf("升级订单状态应为 scheduled，实际 %q", res.Status)
	}

	var order entity.PlanOrder
	if err := f.db.First(&order, res.OrderID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if order.Status != entity.OrderStatusScheduled {
		t.Errorf("库内订单状态应为 scheduled，实际 %q", order.Status)
	}
	if order.EffectiveFrom.Sub(time.Now().AddDate(0, 0, 10)).Abs() > time.Minute {
		t.Errorf("生效时间应等于当前到期时间，实际 %v", order.EffectiveFrom)
	}

	var client entity.Client
	if err := f.db.First(&client, f.client.ID).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	if client.PlanVariantID != f.low.ID || client.PlanID != f.low.TemplateID {
		t.Error("预约阶段不应切换客户套餐")
	}
	if client.QuotaBytes != 100 || client.UsedBytes != 50 {
		t.Errorf("预约阶段不应改配额/已用，实际 quota=%d used=%d", client.QuotaBytes, client.UsedBytes)
	}
	if client.ExpiresAt == nil || !client.ExpiresAt.Equal(f.expiry) {
		t.Errorf("预约阶段不应改到期时间，实际 %v", client.ExpiresAt)
	}

	var bal entity.ClientBalance
	f.db.Where("client_id = ?", f.client.ID).First(&bal)
	if bal.Balance != 100000-f.high.Price {
		t.Errorf("余额应扣 %d，实际 %d", f.high.Price, bal.Balance)
	}

	// 同一客户重复预约必须被拒绝
	if _, err := f.commerce.CreateOrder(ctx, f.client.ID, CreateOrderInput{
		VariantID: f.high.ID, OrderType: entity.OrderTypeUpgrade, PayMethod: "balance",
	}); err == nil {
		t.Error("已有待生效预约时应拒绝再次预约")
	}
}

// 三方支付回调命中预约单：只置 scheduled，绝不提前激活。
func TestChargeCallbackMarksScheduledOnly(t *testing.T) {
	f := seedFlowFixture(t)
	ctx := context.Background()

	pending := &entity.PlanOrder{
		ClientID: f.client.ID, OrderType: entity.OrderTypeUpgrade,
		ToVariantID: f.high.ID, OriginalAmount: f.high.Price, PaidAmount: f.high.Price,
		EffectiveFrom: time.Now(), Status: entity.OrderStatusPending,
	}
	if err := f.db.Create(pending).Error; err != nil {
		t.Fatalf("create pending plan order: %v", err)
	}
	charge := &entity.ChargeOrder{
		ClientID: f.client.ID, TradeNo: "FLOW-TRADE-1", Source: entity.ChargeSourceSite,
		Amount: f.high.Price, Currency: "CNY", PlanVariantID: &f.high.ID,
		OrderType: entity.ChargeOrderTypePurchase, Status: entity.ChargeStatusPending,
	}
	if err := f.db.Create(charge).Error; err != nil {
		t.Fatalf("create charge order: %v", err)
	}

	if err := f.commerce.processChargePayment(ctx, f.db, charge); err != nil {
		t.Fatalf("process charge payment: %v", err)
	}

	var order entity.PlanOrder
	if err := f.db.First(&order, pending.ID).Error; err != nil {
		t.Fatalf("reload plan order: %v", err)
	}
	if order.Status != entity.OrderStatusScheduled {
		t.Errorf("回调后预约单应为 scheduled，实际 %q", order.Status)
	}
	if order.EffectiveFrom.Sub(f.expiry).Abs() > time.Minute {
		t.Errorf("回调应把生效时间对齐到当前到期时间，实际 %v", order.EffectiveFrom)
	}

	var client entity.Client
	f.db.First(&client, f.client.ID)
	if client.PlanVariantID != f.low.ID {
		t.Error("回调阶段不应提前切换套餐")
	}
	if client.ExpiresAt == nil || !client.ExpiresAt.Equal(f.expiry) {
		t.Errorf("回调阶段不应改到期时间，实际 %v", client.ExpiresAt)
	}
}

// 调度器：未到生效时间不动；到点后按新套餐一个完整周期切换，配额重置、已用清零。
func TestSchedulerSwitchesAtExpiry(t *testing.T) {
	f := seedFlowFixture(t)
	ctx := context.Background()

	res, err := f.commerce.CreateOrder(ctx, f.client.ID, CreateOrderInput{
		VariantID: f.high.ID, OrderType: entity.OrderTypeUpgrade, PayMethod: "balance",
	})
	if err != nil {
		t.Fatalf("create upgrade order: %v", err)
	}

	if n, err := f.plans.ProcessPendingDowngrades(ctx); err != nil {
		t.Fatalf("scan before expiry: %v", err)
	} else if n != 0 {
		t.Errorf("未到生效时间不应切换，实际处理 %d 单", n)
	}

	// 模拟时间到达：把生效时间推到过去
	past := time.Now().Add(-time.Minute)
	if err := f.db.Model(&entity.PlanOrder{}).Where("id = ?", res.OrderID).
		Update("effective_from", past).Error; err != nil {
		t.Fatalf("rewind effective_from: %v", err)
	}

	if n, err := f.plans.ProcessPendingDowngrades(ctx); err != nil {
		t.Fatalf("scan after expiry: %v", err)
	} else if n != 1 {
		t.Fatalf("到点应处理 1 单，实际 %d", n)
	}

	var client entity.Client
	f.db.First(&client, f.client.ID)
	if client.PlanVariantID != f.high.ID || client.PlanID != f.high.TemplateID {
		t.Errorf("应切到新套餐，实际 variant=%d plan=%d", client.PlanVariantID, client.PlanID)
	}
	if client.QuotaBytes != f.high.QuotaBytes {
		t.Errorf("新周期配额应为 %d，实际 %d", f.high.QuotaBytes, client.QuotaBytes)
	}
	if client.UsedBytes != 0 {
		t.Errorf("新周期已用应清零，实际 %d", client.UsedBytes)
	}
	// 基准取「原到期时间」（晚于预约生效时间），新周期 = 原到期 + 30 天
	wantExpiry := f.expiry.AddDate(0, 0, f.high.DurationDays)
	if client.ExpiresAt == nil || client.ExpiresAt.Sub(wantExpiry).Abs() > time.Minute {
		t.Errorf("到期时间应为 %v，实际 %v", wantExpiry, client.ExpiresAt)
	}

	var order entity.PlanOrder
	f.db.First(&order, res.OrderID)
	if order.Status != entity.OrderStatusCompleted {
		t.Errorf("生效后订单应为 completed，实际 %q", order.Status)
	}
	if order.EffectiveTo == nil {
		t.Error("生效后应回填 effective_to")
	}
}

// 未生效预约单退款：只撤销预约，绝不回减客户到期时间/配额。
func TestRefundScheduledOrderOnlyCancels(t *testing.T) {
	f := seedFlowFixture(t)
	ctx := context.Background()

	res, err := f.commerce.CreateOrder(ctx, f.client.ID, CreateOrderInput{
		VariantID: f.high.ID, OrderType: entity.OrderTypeUpgrade, PayMethod: "balance",
	})
	if err != nil {
		t.Fatalf("create upgrade order: %v", err)
	}
	charge := &entity.ChargeOrder{
		ClientID: f.client.ID, TradeNo: "FLOW-REFUND-1", Source: entity.ChargeSourceSite,
		Amount: f.high.Price, Currency: "CNY", PlanVariantID: &f.high.ID,
		OrderType: entity.ChargeOrderTypePurchase, Status: entity.ChargeStatusPaid,
	}
	if err := f.db.Create(charge).Error; err != nil {
		t.Fatalf("create charge order: %v", err)
	}

	if err := f.commerce.RefundChargeOrder(ctx, charge.ID, "test refund"); err != nil {
		t.Fatalf("refund: %v", err)
	}

	var order entity.PlanOrder
	f.db.First(&order, res.OrderID)
	if order.Status != entity.OrderStatusCancelled {
		t.Errorf("预约单应被撤销，实际 %q", order.Status)
	}

	var client entity.Client
	f.db.First(&client, f.client.ID)
	if client.ExpiresAt == nil || !client.ExpiresAt.Equal(f.expiry) {
		t.Errorf("撤销预约不应改到期时间，实际 %v", client.ExpiresAt)
	}
	if client.QuotaBytes != 100 || client.PlanVariantID != f.low.ID {
		t.Errorf("撤销预约不应改配额/套餐，实际 quota=%d variant=%d", client.QuotaBytes, client.PlanVariantID)
	}
}

// 已生效（无预约单）的续费退款：回退时长与累加配额。
func TestRefundActivatedRenewRollsBack(t *testing.T) {
	f := seedFlowFixture(t)
	ctx := context.Background()

	if _, err := f.commerce.CreateOrder(ctx, f.client.ID, CreateOrderInput{
		VariantID: f.low.ID, OrderType: entity.OrderTypeRenew, PayMethod: "balance",
	}); err != nil {
		t.Fatalf("renew: %v", err)
	}

	var renewed entity.Client
	f.db.First(&renewed, f.client.ID)
	if renewed.QuotaBytes != 200 {
		t.Fatalf("续费应累加配额到 200，实际 %d", renewed.QuotaBytes)
	}
	if renewed.UsedBytes != 50 {
		t.Errorf("续费不应清空已用流量，实际 %d", renewed.UsedBytes)
	}
	wantExpiry := f.expiry.AddDate(0, 0, 30)
	if renewed.ExpiresAt == nil || renewed.ExpiresAt.Sub(wantExpiry).Abs() > time.Minute {
		t.Fatalf("续费应叠加 30 天，实际 %v", renewed.ExpiresAt)
	}

	charge := &entity.ChargeOrder{
		ClientID: f.client.ID, TradeNo: "FLOW-REFUND-2", Source: entity.ChargeSourceSite,
		Amount: f.low.Price, Currency: "CNY", PlanVariantID: &f.low.ID,
		OrderType: entity.ChargeOrderTypePurchase, Status: entity.ChargeStatusPaid,
	}
	if err := f.db.Create(charge).Error; err != nil {
		t.Fatalf("create charge order: %v", err)
	}
	if err := f.commerce.RefundChargeOrder(ctx, charge.ID, "test refund"); err != nil {
		t.Fatalf("refund: %v", err)
	}

	var after entity.Client
	f.db.First(&after, f.client.ID)
	if after.ExpiresAt == nil || after.ExpiresAt.Sub(f.expiry).Abs() > time.Minute {
		t.Errorf("退款应回退 30 天到 %v，实际 %v", f.expiry, after.ExpiresAt)
	}
	if after.QuotaBytes != 100 {
		t.Errorf("退款应扣回本次配额到 100，实际 %d", after.QuotaBytes)
	}
}
