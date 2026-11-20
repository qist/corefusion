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

// 续费时按配额策略处理：accumulate 保留剩余额度并叠加；reset_on_renew 按新套餐配额重置。
func TestActivatePlanRenewQuotaPolicy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	cases := []struct {
		name      string
		policy    string
		wantQuota int64
		wantUsed  int64
		wantCache int64
	}{
		{"累加", entity.QuotaPolicyAccumulate, 20 << 30, 3 << 30, 3 << 30},
		{"续费重置", entity.QuotaPolicyResetOnRenew, 10 << 30, 0, 0},
		{"到期清空策略续费也重置", entity.QuotaPolicyResetOnExpire, 10 << 30, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(dir, tc.name+".db")), &gorm.Config{})
			if err != nil {
				t.Fatalf("open sqlite: %v", err)
			}
			if err := db.AutoMigrate(&entity.Client{}, &entity.Subscription{}, &entity.PlanTemplate{}, &entity.PlanVariant{}); err != nil {
				t.Fatalf("migrate: %v", err)
			}
			clientRepo := repository.NewClientRepository(db)
			subRepo := repository.NewSubscriptionRepository(db)

			tmpl := &entity.PlanTemplate{Name: "标准", DefaultQuotaBytes: 10 << 30, DefaultQuotaPolicy: entity.QuotaPolicyAccumulate, Enabled: true}
			sub := &entity.Subscription{ClientID: 1, Format: "xray", Token: "tok", Status: "active", QuotaBytes: 10 << 30, UsedBytesCache: 3 << 30}
			client := &entity.Client{
				Email: "renew@test.local", Password: "x", Status: "active",
				PlanID: 1, QuotaBytes: 10 << 30, UsedBytes: 3 << 30,
				SubscriptionID: nil,
			}
			if err := db.Create(tmpl).Error; err != nil {
				t.Fatalf("create tmpl: %v", err)
			}
			exp := time.Now().Add(10 * 24 * time.Hour)
			client.ExpiresAt = &exp
			if err := clientRepo.Create(ctx, client); err != nil {
				t.Fatalf("create client: %v", err)
			}
			sub.ClientID = client.ID
			if err := db.Create(sub).Error; err != nil {
				t.Fatalf("create sub: %v", err)
			}
			client.SubscriptionID = &sub.ID
			if err := clientRepo.Update(ctx, client); err != nil {
				t.Fatalf("update client: %v", err)
			}

			variant := &entity.PlanVariant{
				TemplateID: tmpl.ID, VariantType: entity.VariantTypePlan, Name: "月付",
				DurationDays: 30, Price: 990, QuotaBytes: 10 << 30,
				QuotaPolicy: tc.policy, Enabled: true,
			}
			svc := &CommerceService{db: db, clientRepo: clientRepo, subRepo: subRepo}

			if err := svc.activatePlan(ctx, client, tmpl, variant, entity.OrderTypeRenew); err != nil {
				t.Fatalf("activatePlan: %v", err)
			}

			got, err := clientRepo.GetByID(ctx, client.ID)
			if err != nil {
				t.Fatalf("reload client: %v", err)
			}
			if got.QuotaBytes != tc.wantQuota {
				t.Errorf("quota = %d, want %d", got.QuotaBytes, tc.wantQuota)
			}
			if got.UsedBytes != tc.wantUsed {
				t.Errorf("used = %d, want %d", got.UsedBytes, tc.wantUsed)
			}
			gotSub, err := subRepo.GetByID(ctx, sub.ID)
			if err != nil {
				t.Fatalf("reload sub: %v", err)
			}
			if gotSub.QuotaBytes != tc.wantQuota {
				t.Errorf("sub quota = %d, want %d", gotSub.QuotaBytes, tc.wantQuota)
			}
			if gotSub.UsedBytesCache != tc.wantCache {
				t.Errorf("sub used cache = %d, want %d", gotSub.UsedBytesCache, tc.wantCache)
			}
		})
	}
}
