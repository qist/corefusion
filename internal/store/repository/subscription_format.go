package repository

import (
	"context"
	"encoding/json"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// SubscriptionFormatRepository handles subscription format database operations.
type SubscriptionFormatRepository struct {
	db *gorm.DB
}

// NewSubscriptionFormatRepository creates a new subscription format repository.
func NewSubscriptionFormatRepository(db *gorm.DB) *SubscriptionFormatRepository {
	return &SubscriptionFormatRepository{db: db}
}

// List returns all subscription formats.
func (r *SubscriptionFormatRepository) List(ctx context.Context) ([]entity.SubscriptionFormat, error) {
	return r.ListPaged(ctx, 0, 0)
}

// ListPaged returns subscription formats with pagination (limit<=0 means no limit).
func (r *SubscriptionFormatRepository) ListPaged(ctx context.Context, limit, offset int) ([]entity.SubscriptionFormat, error) {
	var formats []entity.SubscriptionFormat
	q := r.db.WithContext(ctx).Order("sort_order ASC, id ASC")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	err := q.Find(&formats).Error
	if err != nil {
		return nil, err
	}
	// Manually load plan_ids for each format (compatibility with MySQL/SQLite)
	for i := range formats {
		var planIDsJSON string
		r.db.WithContext(ctx).Raw("SELECT plan_ids FROM subscription_formats WHERE id = ?", formats[i].ID).Scan(&planIDsJSON)
		if planIDsJSON != "" && planIDsJSON != "null" {
			json.Unmarshal([]byte(planIDsJSON), &formats[i].PlanIDs)
		}
	}
	return formats, nil
}

// Create creates a new subscription format.
func (r *SubscriptionFormatRepository) Create(ctx context.Context, format *entity.SubscriptionFormat) error {
	return r.db.WithContext(ctx).Create(format).Error
}

// Count returns the total number of subscription formats.
func (r *SubscriptionFormatRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.SubscriptionFormat{}).Count(&count).Error
	return count, err
}

// SubscriptionRuleRepository handles subscription rule database operations.
type SubscriptionRuleRepository struct {
	db *gorm.DB
}

// NewSubscriptionRuleRepository creates a new subscription rule repository.
func NewSubscriptionRuleRepository(db *gorm.DB) *SubscriptionRuleRepository {
	return &SubscriptionRuleRepository{db: db}
}

// Create creates a new rule.
func (r *SubscriptionRuleRepository) Create(ctx context.Context, rule *entity.SubscriptionRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}
