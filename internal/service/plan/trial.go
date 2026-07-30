package plan

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
)

var trialSchedulerOnce sync.Once

// StartTrialScheduler launches the hourly trial expiry/auto-convert scanner.
// Safe to call multiple times; only the first call starts the goroutine.
func (s *Service) StartTrialScheduler(ctx context.Context, interval time.Duration) {
	trialSchedulerOnce.Do(func() {
		if interval <= 0 {
			interval = time.Hour
		}
		go func() {
			// Run once shortly after startup, then on the interval.
			timer := time.NewTimer(time.Minute)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.runTrialScan(ctx)
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.runTrialScan(ctx)
				}
			}
		}()
		log.Printf("[Trial] scheduler started (interval=%s)", interval)
	})
}

func (s *Service) runTrialScan(ctx context.Context) {
	if n, err := s.ProcessTrialExpiry(ctx); err != nil {
		log.Printf("[Trial] scan failed: %v", err)
	} else if n > 0 {
		log.Printf("[Trial] processed %d expired trial client(s)", n)
	}
}

// ProcessTrialExpiry finds active clients on expired trial plans and either
// converts them into the package they actually purchased during the trial, or
// marks them expired when no purchase was made. Returns the number of clients
// processed.
func (s *Service) ProcessTrialExpiry(ctx context.Context) (int, error) {
	now := time.Now()

	var clients []entity.Client
	err := s.db.WithContext(ctx).
		Where("status = ?", "active").
		Where("expires_at IS NOT NULL AND expires_at < ?", now).
		Where("plan_id IN (?)", s.db.Model(&entity.PlanTemplate{}).Select("id").Where("is_trial = ?", true)).
		Find(&clients).Error
	if err != nil {
		return 0, fmt.Errorf("query trial clients: %w", err)
	}

	processed := 0
	for _, c := range clients {
		client := c
		tmpl, err := s.tmplRepo.GetByID(ctx, client.PlanID)
		if err != nil || tmpl == nil || !tmpl.IsTrial {
			continue
		}
		// 试用期内买了套餐 → 转正成用户实际购买的套餐；没买 → 直接过期。
		if variant, ok := s.findTrialPurchaseVariant(ctx, client.ID); ok {
			if err := s.convertTrialToVariant(ctx, &client, variant); err != nil {
				log.Printf("[Trial] convert client %d to purchased variant %d failed: %v", client.ID, variant.ID, err)
				continue
			}
		} else {
			if err := s.expireTrial(ctx, &client); err != nil {
				log.Printf("[Trial] expire client %d failed: %v", client.ID, err)
				continue
			}
		}
		processed++
	}
	return processed, nil
}

// convertTrialToVariant converts a trial client to the given already-resolved
// target variant (order_type=trial_convert, zero amount).
func (s *Service) convertTrialToVariant(ctx context.Context, client *entity.Client, variant *entity.PlanVariant) error {
	targetTmpl, err := s.tmplRepo.GetByID(ctx, variant.TemplateID)
	if err != nil || targetTmpl == nil || !targetTmpl.Enabled {
		return fmt.Errorf("target template %d not available", variant.TemplateID)
	}

	now := time.Now()
	newExpiry := now.AddDate(0, 0, variant.DurationDays)

	oldVariantID := client.PlanVariantID
	var fromVariantID *int64
	if oldVariantID > 0 {
		fromVariantID = &oldVariantID
	}
	order := &entity.PlanOrder{
		ClientID:      client.ID,
		OrderType:     entity.OrderTypeTrialConvert,
		FromVariantID: fromVariantID,
		ToVariantID:   variant.ID,
		EffectiveFrom: now,
		EffectiveTo:   &newExpiry,
		Status:        entity.OrderStatusCompleted,
		Remark:        "trial auto-convert",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return fmt.Errorf("create trial_convert order: %w", err)
	}

	quotaBytes := variant.QuotaBytes
	if quotaBytes == 0 {
		quotaBytes = targetTmpl.DefaultQuotaBytes
	}

	client.PlanID = variant.TemplateID
	client.PlanVariantID = variant.ID
	client.QuotaBytes = quotaBytes
	client.UsedBytes = 0
	client.Status = "active"
	client.ExpiresAt = &newExpiry
	if targetTmpl.DeviceLimit > 0 {
		client.DeviceLimit = targetTmpl.DeviceLimit
	}
	if targetTmpl.SpeedLimitMbps > 0 {
		client.SpeedLimitMbps = targetTmpl.SpeedLimitMbps
	}
	if targetTmpl.SpeedLimitUpMbps > 0 {
		client.SpeedLimitUpMbps = targetTmpl.SpeedLimitUpMbps
	}
	if targetTmpl.ConcurrentLimit > 0 {
		client.ConcurrentLimit = targetTmpl.ConcurrentLimit
	}
	client.UpdatedAt = now
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return fmt.Errorf("update client: %w", err)
	}

	// Keep the linked subscription in sync (quota/expiry/status).
	if client.SubscriptionID != nil {
		_ = s.db.WithContext(ctx).Model(&entity.Subscription{}).
			Where("id = ?", *client.SubscriptionID).
			Updates(map[string]interface{}{
				"quota_bytes":      quotaBytes,
				"used_bytes_cache": 0,
				"status":           "active",
				"expires_at":       newExpiry,
				"updated_at":       now,
			}).Error
	}

	log.Printf("[Trial] client %d auto-converted to template %d variant %d (expires %s)",
		client.ID, variant.TemplateID, variant.ID, newExpiry.Format(time.RFC3339))
	return nil
}

// findTrialPurchaseVariant returns the variant the client actually purchased
// during the trial (most recent completed paid order to a non-trial plan
// variant), so that a trial converts into whatever package the user really
// bought.
func (s *Service) findTrialPurchaseVariant(ctx context.Context, clientID int64) (*entity.PlanVariant, bool) {
	var order entity.PlanOrder
	err := s.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Where("status = ?", entity.OrderStatusCompleted).
		Where("order_type IN ?", []string{entity.OrderTypeNew, entity.OrderTypeRenew, entity.OrderTypeUpgrade}).
		Where("to_variant_id > 0").
		Order("created_at DESC, id DESC").
		First(&order).Error
	if err != nil {
		return nil, false
	}

	variant, err := s.variantRepo.GetByID(ctx, order.ToVariantID)
	if err != nil || !variant.Enabled || variant.VariantType != entity.VariantTypePlan {
		return nil, false
	}
	vTmpl, err := s.tmplRepo.GetByID(ctx, variant.TemplateID)
	if err != nil || vTmpl == nil || vTmpl.IsTrial {
		return nil, false
	}
	return variant, true
}

// expireTrial marks a trial client (and its subscription) as expired.
func (s *Service) expireTrial(ctx context.Context, client *entity.Client) error {
	now := time.Now()
	client.Status = "expired"
	client.UpdatedAt = now
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return fmt.Errorf("update client: %w", err)
	}
	if client.SubscriptionID != nil {
		_ = s.db.WithContext(ctx).Model(&entity.Subscription{}).
			Where("id = ?", *client.SubscriptionID).
			Updates(map[string]interface{}{
				"status":     "expired",
				"updated_at": now,
			}).Error
	}
	log.Printf("[Trial] client %d trial expired (no purchase during trial)", client.ID)
	return nil
}
