package plan

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
)

var downgradeSchedulerOnce sync.Once

// StartDowngradeScheduler launches the pending-downgrade processor.
// It scans for downgrade orders whose effective_from has passed and
// applies them: updates client plan/quota/expiry, syncs subscription,
// pushes config to all nodes, and marks the order completed.
// Safe to call multiple times; only the first call starts the goroutine.
func (s *Service) StartDowngradeScheduler(ctx context.Context, interval time.Duration) {
	downgradeSchedulerOnce.Do(func() {
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		go func() {
			timer := time.NewTimer(2 * time.Minute)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.runDowngradeScan(ctx)
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.runDowngradeScan(ctx)
				}
			}
		}()
		log.Printf("[Downgrade] scheduler started (interval=%s)", interval)
	})
}

func (s *Service) runDowngradeScan(ctx context.Context) {
	if n, err := s.ProcessPendingDowngrades(ctx); err != nil {
		log.Printf("[Downgrade] scan failed: %v", err)
	} else if n > 0 {
		log.Printf("[Downgrade] processed %d pending downgrade order(s)", n)
	}
}

// ProcessPendingDowngrades finds all pending downgrade orders whose
// effective_from has passed and applies them. Returns the number of
// orders processed.
func (s *Service) ProcessPendingDowngrades(ctx context.Context) (int, error) {
	now := time.Now()

	var orders []entity.PlanOrder
	err := s.db.WithContext(ctx).
		Where("order_type = ?", entity.OrderTypeDowngrade).
		Where("status = ?", entity.OrderStatusPending).
		Where("effective_from <= ?", now).
		Find(&orders).Error
	if err != nil {
		return 0, fmt.Errorf("query pending downgrades: %w", err)
	}

	processed := 0
	for _, o := range orders {
		order := o
		if err := s.applyDowngrade(ctx, &order); err != nil {
			log.Printf("[Downgrade] apply order %d failed: %v", order.ID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// applyDowngrade executes a single pending downgrade order:
//  1. Look up the target variant and template
//  2. Update client fields (plan_variant_id, plan_id, quota, speed, device, expiry, status)
//  3. Update the linked subscription (quota, expiry, status)
//  4. Push updated config to all core servers via userSyncer
//  5. Mark the order as completed
func (s *Service) applyDowngrade(ctx context.Context, order *entity.PlanOrder) error {
	variant, err := s.variantRepo.GetByID(ctx, order.ToVariantID)
	if err != nil {
		return fmt.Errorf("target variant %d not found: %w", order.ToVariantID, err)
	}

	tmpl, err := s.tmplRepo.GetByID(ctx, variant.TemplateID)
	if err != nil || tmpl == nil || !tmpl.Enabled {
		return fmt.Errorf("target template %d not available", variant.TemplateID)
	}

	client, err := s.clientRepo.GetByID(ctx, order.ClientID)
	if err != nil {
		return fmt.Errorf("client %d not found: %w", order.ClientID, err)
	}

	now := time.Now()

	// Calculate new expiry from the downgrade effective date
	newExpiry := order.EffectiveFrom.AddDate(0, 0, variant.DurationDays)

	// Update client
	client.PlanVariantID = variant.ID
	client.PlanID = variant.TemplateID
	client.ExpiresAt = &newExpiry
	client.Status = "active"

	quotaBytes := variant.QuotaBytes
	if quotaBytes == 0 {
		quotaBytes = tmpl.DefaultQuotaBytes
	}
	if quotaBytes > 0 {
		client.QuotaBytes = quotaBytes
	}
	// 限速三字段与设备数不物化模板值（含降级到"不限速"模板的情况）：
	// 0=订阅拉取时实时继承模板，非 0=个人覆盖（push 回退见 public.go）。
	client.UpdatedAt = now

	if err := s.clientRepo.Update(ctx, client); err != nil {
		return fmt.Errorf("update client: %w", err)
	}

	// Update subscription
	s.syncSubscriptionAfterOrder(ctx, client, variant)

	// Sync to all core servers
	if s.userSyncer != nil {
		go s.userSyncer.SyncUserToServers(context.Background(), client.ID, "update")
	}

	// Mark order completed
	order.Status = entity.OrderStatusCompleted
	order.UpdatedAt = now
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	log.Printf("[Downgrade] client %d downgraded to template %d variant %d (expires %s)",
		client.ID, variant.TemplateID, variant.ID, newExpiry.Format(time.RFC3339))
	return nil
}
