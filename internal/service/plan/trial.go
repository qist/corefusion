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
// auto-converts them to the configured template's default variant, or marks
// them expired. Returns the number of clients processed.
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
		if tmpl.TrialAutoConvertTemplateID != nil && *tmpl.TrialAutoConvertTemplateID > 0 {
			if err := s.convertTrial(ctx, &client, *tmpl.TrialAutoConvertTemplateID); err != nil {
				log.Printf("[Trial] auto-convert client %d failed: %v", client.ID, err)
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

// convertTrial converts a trial client to the target template's default variant
// (order_type=trial_convert, zero amount).
func (s *Service) convertTrial(ctx context.Context, client *entity.Client, targetTemplateID int64) error {
	targetTmpl, err := s.tmplRepo.GetByID(ctx, targetTemplateID)
	if err != nil || targetTmpl == nil || !targetTmpl.Enabled {
		return fmt.Errorf("target template %d not available", targetTemplateID)
	}

	variant, err := s.pickDefaultVariant(ctx, targetTemplateID)
	if err != nil {
		return err
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

	client.PlanID = targetTemplateID
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
		client.ID, targetTemplateID, variant.ID, newExpiry.Format(time.RFC3339))
	return nil
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
	log.Printf("[Trial] client %d trial expired (no auto-convert template)", client.ID)
	return nil
}

// pickDefaultVariant returns the default (or first enabled) plan variant of a template.
func (s *Service) pickDefaultVariant(ctx context.Context, templateID int64) (*entity.PlanVariant, error) {
	var variants []entity.PlanVariant
	if err := s.db.WithContext(ctx).
		Where("template_id = ? AND enabled = ? AND variant_type = ?", templateID, true, "plan").
		Order("is_default DESC, sort_order ASC, id ASC").
		Find(&variants).Error; err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("template %d has no enabled variant", templateID)
	}
	return &variants[0], nil
}
