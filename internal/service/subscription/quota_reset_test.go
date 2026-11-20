package subscription

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	plansvc "corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

func newQuotaTestEnv(t *testing.T) (*gorm.DB, *Service, *repository.ClientRepository, *repository.SubscriptionRepository, *repository.PlanVariantRepository, *repository.PlanTemplateRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "quota.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.Client{}, &entity.Subscription{}, &entity.PlanTemplate{}, &entity.PlanVariant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	clientRepo := repository.NewClientRepository(db)
	subRepo := repository.NewSubscriptionRepository(db)
	tmplRepo := repository.NewPlanTemplateRepository(db)
	variantRepo := repository.NewPlanVariantRepository(db)
	planSvc := plansvc.NewService(db, tmplRepo, variantRepo, nil, nil, nil, nil, clientRepo, nil, nil)
	svc := NewService(subRepo, nil, nil, clientRepo, planSvc)
	return db, svc, clientRepo, subRepo, variantRepo, tmplRepo
}

// 月度重置：next_reset_at 到期的订阅清零已用量，并推进下一次重置时间。
func TestEnforceQuotaResets(t *testing.T) {
	ctx := context.Background()
	_, svc, clientRepo, subRepo, _, _ := newQuotaTestEnv(t)

	client := &entity.Client{Email: "monthly@test.local", Password: "x", Status: "active", QuotaBytes: 10 << 30, UsedBytes: 4 << 30}
	if err := clientRepo.Create(ctx, client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	past := time.Now().Add(-time.Hour)
	sub := &entity.Subscription{
		ClientID: client.ID, Format: "xray", Token: "tok1", Status: "active",
		QuotaBytes: 10 << 30, UsedBytesCache: 4 << 30, UploadBytesCache: 1 << 30, DownloadBytesCache: 3 << 30,
		QuotaResetCycle: entity.QuotaResetCycleMonthly, NextResetAt: &past,
	}
	if err := subRepo.Create(ctx, sub); err != nil {
		t.Fatalf("create sub: %v", err)
	}

	if err := svc.EnforceQuotaResets(ctx); err != nil {
		t.Fatalf("EnforceQuotaResets: %v", err)
	}

	gotSub, err := subRepo.GetByID(ctx, sub.ID)
	if err != nil {
		t.Fatalf("reload sub: %v", err)
	}
	if gotSub.UsedBytesCache != 0 || gotSub.UploadBytesCache != 0 || gotSub.DownloadBytesCache != 0 {
		t.Errorf("订阅用量应清零，实际 used=%d up=%d down=%d", gotSub.UsedBytesCache, gotSub.UploadBytesCache, gotSub.DownloadBytesCache)
	}
	if gotSub.NextResetAt == nil || !gotSub.NextResetAt.After(time.Now()) {
		t.Errorf("next_reset_at 应推进到未来，实际 %v", gotSub.NextResetAt)
	}
	gotClient, err := clientRepo.GetByID(ctx, client.ID)
	if err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if gotClient.UsedBytes != 0 {
		t.Errorf("客户端已用量应清零，实际 %d", gotClient.UsedBytes)
	}
	if gotClient.QuotaBytes != 10<<30 {
		t.Errorf("月度重置不应改变配额，实际 %d", gotClient.QuotaBytes)
	}

	// 未到期的订阅不能被重置
	future := time.Now().Add(time.Hour)
	sub2 := &entity.Subscription{
		ClientID: client.ID, Format: "xray", Token: "tok2", Status: "active",
		QuotaBytes: 10 << 30, UsedBytesCache: 1 << 30,
		QuotaResetCycle: entity.QuotaResetCycleMonthly, NextResetAt: &future,
	}
	if err := subRepo.Create(ctx, sub2); err != nil {
		t.Fatalf("create sub2: %v", err)
	}
	if err := svc.EnforceQuotaResets(ctx); err != nil {
		t.Fatalf("EnforceQuotaResets again: %v", err)
	}
	got2, _ := subRepo.GetByID(ctx, sub2.ID)
	if got2.UsedBytesCache != 1<<30 {
		t.Errorf("未到期订阅不应被重置，实际 used=%d", got2.UsedBytesCache)
	}
}

// 到期时按策略清零：reset_on_expire 把未用完的配额作废；accumulate 保持不动。
func TestExpireZeroesQuotaOnlyWhenConfigured(t *testing.T) {
	cases := []struct {
		name       string
		policy     string
		wantQuota  int64
		wantClient int64
	}{
		{"到期清空", entity.QuotaPolicyResetOnExpire, 0, 0},
		{"默认累加保留", entity.QuotaPolicyAccumulate, 10 << 30, 10 << 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, svc, clientRepo, subRepo, variantRepo, tmplRepo := newQuotaTestEnv(t)

			tmpl := &entity.PlanTemplate{Name: "标准", DefaultQuotaBytes: 10 << 30, DefaultQuotaPolicy: tc.policy, Enabled: true}
			if err := tmplRepo.Create(ctx, tmpl); err != nil {
				t.Fatalf("create tmpl: %v", err)
			}
			variant := &entity.PlanVariant{TemplateID: tmpl.ID, VariantType: entity.VariantTypePlan, Name: "月付", DurationDays: 30, QuotaBytes: 10 << 30, Enabled: true}
			if err := variantRepo.Create(ctx, variant); err != nil {
				t.Fatalf("create variant: %v", err)
			}

			pastExpiry := time.Now().Add(-24 * time.Hour)
			client := &entity.Client{
				Email: "expire@test.local", Password: "x", Status: "active",
				PlanID: tmpl.ID, PlanVariantID: variant.ID,
				QuotaBytes: 10 << 30, UsedBytes: 2 << 30, ExpiresAt: &pastExpiry,
			}
			if err := clientRepo.Create(ctx, client); err != nil {
				t.Fatalf("create client: %v", err)
			}
			sub := &entity.Subscription{
				ClientID: client.ID, Format: "xray", Token: "tok", Status: "active",
				QuotaBytes: 10 << 30, UsedBytesCache: 2 << 30, ExpiresAt: &pastExpiry,
			}
			if err := subRepo.Create(ctx, sub); err != nil {
				t.Fatalf("create sub: %v", err)
			}

			if err := svc.EnforceQuotaAndExpiry(ctx); err != nil {
				t.Fatalf("EnforceQuotaAndExpiry: %v", err)
			}

			gotSub, _ := subRepo.GetByID(ctx, sub.ID)
			if gotSub.Status != "expired" {
				t.Errorf("订阅应转为 expired，实际 %s", gotSub.Status)
			}
			if gotSub.QuotaBytes != tc.wantQuota {
				t.Errorf("sub quota = %d, want %d", gotSub.QuotaBytes, tc.wantQuota)
			}
			gotClient, _ := clientRepo.GetByID(ctx, client.ID)
			if gotClient.QuotaBytes != tc.wantClient {
				t.Errorf("client quota = %d, want %d", gotClient.QuotaBytes, tc.wantClient)
			}
			if gotClient.Status == "active" {
				t.Error("到期后客户应被封禁")
			}
			_ = db
		})
	}
}
