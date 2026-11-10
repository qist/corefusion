package plan

import (
	"context"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// 规格改成「永久有效」后，历史客户/订阅残留的 expires_at 必须被清掉，
// 否则订阅头继续显示到期时间、过期任务还会把账号停掉。
func TestReconcilePermanentExpiryClearsStaleRows(t *testing.T) {
	db := newReconcileDB(t)
	ctx := context.Background()

	permanent := &entity.PlanVariant{TemplateID: 1, Name: "永久", DurationDays: entity.DurationPermanent}
	monthly := &entity.PlanVariant{TemplateID: 1, Name: "月付", DurationDays: 30}
	if err := db.Create(permanent).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(monthly).Error; err != nil {
		t.Fatal(err)
	}

	expireAt := time.Now().Add(30 * 24 * time.Hour)
	expiredAt := time.Now().Add(-24 * time.Hour)
	stale := &entity.Client{
		Email: "perm@example.com", PlanID: 1, PlanVariantID: permanent.ID,
		Status: "expired", ExpiresAt: &expiredAt, SuspendReason: "auto_expired",
	}
	keep := &entity.Client{
		Email: "monthly@example.com", PlanID: 1, PlanVariantID: monthly.ID,
		Status: "active", ExpiresAt: &expireAt,
	}
	for _, c := range []*entity.Client{stale, keep} {
		if err := db.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}
	subs := []*entity.Subscription{
		{ClientID: stale.ID, Token: "tok-stale", Status: "expired", QuotaState: "expired_days", ExpiresAt: &expiredAt},
		{ClientID: keep.ID, Token: "tok-keep", Status: "active", QuotaState: "normal", ExpiresAt: &expireAt},
	}
	for _, s := range subs {
		if err := db.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}

	svc := NewService(db,
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

	n, err := svc.ReconcilePermanentExpiry(ctx)
	if err != nil {
		t.Fatalf("ReconcilePermanentExpiry: %v", err)
	}
	if n != 1 {
		t.Fatalf("cleared = %d, want 1", n)
	}

	var gotClient entity.Client
	if err := db.First(&gotClient, stale.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotClient.ExpiresAt != nil {
		t.Errorf("永久客户 expires_at 应被清空，实际 %v", gotClient.ExpiresAt)
	}
	if gotClient.Status != "active" {
		t.Errorf("因到期停用的永久客户应恢复 active，实际 %q", gotClient.Status)
	}

	var gotSub entity.Subscription
	if err := db.First(&gotSub, subs[0].ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotSub.ExpiresAt != nil {
		t.Errorf("永久订阅 expires_at 应被清空，实际 %v", gotSub.ExpiresAt)
	}
	if gotSub.Status != "active" || gotSub.QuotaState != "normal" {
		t.Errorf("永久订阅应恢复 active/normal，实际 %q/%q", gotSub.Status, gotSub.QuotaState)
	}

	// 月付客户不能被误伤。
	var other entity.Client
	if err := db.First(&other, keep.ID).Error; err != nil {
		t.Fatal(err)
	}
	if other.ExpiresAt == nil || other.Status != "active" {
		t.Errorf("非永久客户不应被改动: expires=%v status=%q", other.ExpiresAt, other.Status)
	}
	var otherSub entity.Subscription
	if err := db.First(&otherSub, subs[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if otherSub.ExpiresAt == nil {
		t.Error("非永久订阅的 expires_at 不应被清空")
	}
}

// 幂等：没有残留到期时间时不产生任何写入。
func TestReconcilePermanentExpiryIdempotent(t *testing.T) {
	db := newReconcileDB(t)
	ctx := context.Background()

	permanent := &entity.PlanVariant{TemplateID: 1, Name: "永久", DurationDays: entity.DurationPermanent}
	if err := db.Create(permanent).Error; err != nil {
		t.Fatal(err)
	}
	client := &entity.Client{Email: "perm2@example.com", PlanID: 1, PlanVariantID: permanent.ID, Status: "active"}
	if err := db.Create(client).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewService(db,
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
	n, err := svc.ReconcilePermanentExpiry(ctx)
	if err != nil {
		t.Fatalf("ReconcilePermanentExpiry: %v", err)
	}
	if n != 0 {
		t.Fatalf("cleared = %d, want 0", n)
	}
}

func newReconcileDB(t *testing.T) *gorm.DB {
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
		&entity.Subscription{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
