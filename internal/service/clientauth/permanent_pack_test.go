package clientauth

import (
	"context"
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

// 永久流量包（extend_plan + duration_days=-1）：应用后客户端与订阅的到期时间
// 都应变为 nil（永不过期）。此前 -1 会被 `DurationDays > 0` 守卫直接跳过，
// 表现为"勾了永久但到期日没变"。
func TestApplyTrafficPackPermanentClearsExpiry(t *testing.T) {
	db := newFlowDB(t)
	svc := newCommerceForFlow(db)
	ctx := context.Background()

	expiry := time.Now().Add(10 * 24 * time.Hour)
	sub := &entity.Subscription{
		ClientID: 1, Format: "xray", Token: "tok-perm-test", Status: "active",
		QuotaBytes: 100, ExpiresAt: &expiry,
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	client := &entity.Client{
		Email: "perm@example.com", Password: "x", Status: "active",
		QuotaBytes: 100, ExpiresAt: &expiry, SubscriptionID: &sub.ID,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	variant := &entity.PlanVariant{
		VariantType:  entity.VariantTypeTrafficPack,
		Name:         "permanent-pack",
		BillingCycle: entity.BillingCycleOneTime,
		DurationDays: entity.DurationPermanent,
		QuotaBytes:   1024,
		ExpiryMode:   entity.ExpiryModeExtendPlan,
		Enabled:      true,
	}

	if err := svc.applyTrafficPack(ctx, client, variant); err != nil {
		t.Fatalf("apply traffic pack: %v", err)
	}

	var gotClient entity.Client
	if err := db.First(&gotClient, client.ID).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if gotClient.ExpiresAt != nil {
		t.Fatalf("永久流量包应清空客户端到期时间，实际 %v", gotClient.ExpiresAt)
	}
	if gotClient.QuotaBytes != 1124 {
		t.Fatalf("流量应累加（100+1024），实际 %d", gotClient.QuotaBytes)
	}

	var gotSub entity.Subscription
	if err := db.First(&gotSub, sub.ID).Error; err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	if gotSub.ExpiresAt != nil {
		t.Fatalf("永久流量包应清空订阅到期时间，实际 %v", gotSub.ExpiresAt)
	}
}

// 普通流量包（延长时间）仍然按天数累加，不能被永久分支影响。
func TestApplyTrafficPackExtendStillAddsDays(t *testing.T) {
	db := newFlowDB(t)
	svc := newCommerceForFlow(db)
	ctx := context.Background()

	expiry := time.Now().Add(10 * 24 * time.Hour)
	sub := &entity.Subscription{
		ClientID: 1, Format: "xray", Token: "tok-extend-test", Status: "active",
		QuotaBytes: 100, ExpiresAt: &expiry,
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	client := &entity.Client{
		Email: "extend@example.com", Password: "x", Status: "active",
		QuotaBytes: 100, ExpiresAt: &expiry, SubscriptionID: &sub.ID,
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	variant := &entity.PlanVariant{
		VariantType:  entity.VariantTypeTrafficPack,
		Name:         "extend-pack",
		BillingCycle: entity.BillingCycleOneTime,
		DurationDays: 7,
		QuotaBytes:   1024,
		ExpiryMode:   entity.ExpiryModeExtendPlan,
		Enabled:      true,
	}
	if err := svc.applyTrafficPack(ctx, client, variant); err != nil {
		t.Fatalf("apply traffic pack: %v", err)
	}

	var got entity.Client
	if err := db.First(&got, client.ID).Error; err != nil {
		t.Fatalf("reload client: %v", err)
	}
	if got.ExpiresAt == nil {
		t.Fatal("普通流量包不应清空到期时间")
	}
	want := expiry.AddDate(0, 0, 7)
	if !got.ExpiresAt.Equal(want) {
		t.Fatalf("到期时间应 +7 天，want %v got %v", want, *got.ExpiresAt)
	}
}
