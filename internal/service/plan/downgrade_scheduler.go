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

// StartDowngradeScheduler launches the pending plan-change processor.
// It scans for scheduled upgrade/downgrade orders whose effective_from has
// passed and applies them: updates client plan/quota/expiry, syncs
// subscription, pushes config to all nodes, and marks the order completed.
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
		log.Printf("[PlanChange] scheduler started (interval=%s)", interval)
	})
}

func (s *Service) runDowngradeScan(ctx context.Context) {
	if n, err := s.ProcessPendingDowngrades(ctx); err != nil {
		log.Printf("[PlanChange] scan failed: %v", err)
	} else if n > 0 {
		log.Printf("[PlanChange] processed %d pending plan change order(s)", n)
	}
}

// ProcessPendingDowngrades finds all pending/scheduled upgrade or downgrade
// orders whose effective_from has passed and applies them. Returns the number
// of orders processed.
//
// 升级与降级共用同一条"到期切换"链路：付款/下单只登记预约，真正的套餐切换
// 由这里在当前计费周期结束时执行。
func (s *Service) ProcessPendingDowngrades(ctx context.Context) (int, error) {
	now := time.Now()

	var orders []entity.PlanOrder
	// 只执行真正成交的预约单（scheduled）。
	// 兼容旧版本留下的「未付款 downgrade pending」单：降级不走支付，pending 即预约。
	// pending 的 upgrade 单是「下单未付款」，绝不能在这里生效，否则等于白送套餐。
	err := s.db.WithContext(ctx).
		Where("effective_from <= ?", now).
		Where("status = ? OR (order_type = ? AND status = ?)",
			entity.OrderStatusScheduled, entity.OrderTypeDowngrade, entity.OrderStatusPending).
		Order("id ASC").
		Find(&orders).Error
	if err != nil {
		return 0, fmt.Errorf("query pending plan changes: %w", err)
	}

	processed := 0
	for _, o := range orders {
		order := o
		if err := s.applyScheduledChange(ctx, &order); err != nil {
			log.Printf("[PlanChange] apply order %d failed: %v", order.ID, err)
			continue
		}
		processed++
	}
	return processed, nil
}

// applyScheduledChange executes a single pending plan-change order:
//  1. Look up the target variant and template
//  2. Update client fields (plan_variant_id, plan_id, quota, expiry, status)
//  3. Update the linked subscription (quota, usage cache, expiry, status)
//  4. Push updated config to all core servers via userSyncer
//  5. Ensure accounts exist on the new plan's nodes
//  6. Mark the order as completed
func (s *Service) applyScheduledChange(ctx context.Context, order *entity.PlanOrder) error {
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

	// 新周期从「预约生效时间」起算；若期间客户又续费把到期时间往后推了，
	// 则以更晚的到期时间为基准，避免把续费买到的时长覆盖掉。
	// 若调度器延迟执行（面板停机、队列积压）导致生效时间早已过去，
	// 以 now 兜底，避免刚切换就立刻过期（用户买的是一个完整周期）。
	base := order.EffectiveFrom
	if client.ExpiresAt != nil && client.ExpiresAt.After(base) {
		base = *client.ExpiresAt
	}
	if now.After(base) {
		base = now
	}
	newExpiry := base.AddDate(0, 0, variant.DurationDays)

	// Update client
	client.PlanVariantID = variant.ID
	client.PlanID = variant.TemplateID
	client.ExpiresAt = &newExpiry
	// 被人工停用的账号不因套餐切换而被自动恢复。
	if client.Status != "suspended" {
		client.Status = "active"
	}

	quotaBytes := variant.QuotaBytes
	if quotaBytes == 0 {
		quotaBytes = tmpl.DefaultQuotaBytes
	}
	if quotaBytes > 0 {
		client.QuotaBytes = quotaBytes
	}
	// 到期切换意味着进入新的计费周期：已用流量从 0 重新计量。
	client.UsedBytes = 0
	// 限速三字段与设备数不物化模板值（含降级到"不限速"模板的情况）：
	// 0=订阅拉取时实时继承模板，非 0=个人覆盖（push 回退见 public.go）。
	client.UpdatedAt = now

	if err := s.clientRepo.Update(ctx, client); err != nil {
		return fmt.Errorf("update client: %w", err)
	}

	// Update subscription (quota / reset cycle / expiry) and clear the usage cache
	s.syncSubscriptionAfterOrder(ctx, client, variant)
	if client.SubscriptionID != nil {
		if err := s.db.WithContext(ctx).Model(&entity.Subscription{}).
			Where("id = ?", *client.SubscriptionID).
			Updates(map[string]interface{}{
				"used_bytes_cache": 0,
				"upload_bytes_cache": 0,
				"download_bytes_cache": 0,
				"updated_at":       now,
			}).Error; err != nil {
			log.Printf("[PlanChange] reset subscription usage for client %d failed: %v", client.ID, err)
		}
	}

	// Sync to all core servers
	if s.userSyncer != nil {
		go s.userSyncer.SyncUserToServers(context.Background(), client.ID, "update")
	}

	// 切换模板后用户需要在新的节点组上有账号（幂等：已存在的账号会跳过）
	if s.accountSyncer != nil {
		go func(planID int64) {
			if _, err := s.SyncAccountsForTemplate(context.Background(), planID); err != nil {
				log.Printf("[PlanChange] sync accounts for plan %d failed: %v", planID, err)
			}
		}(variant.TemplateID)
	}

	// Mark order completed
	order.Status = entity.OrderStatusCompleted
	order.UpdatedAt = now
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("update order status: %w", err)
	}

	log.Printf("[PlanChange] client %d switched to template %d variant %d (expires %s)",
		client.ID, variant.TemplateID, variant.ID, newExpiry.Format(time.RFC3339))
	return nil
}
