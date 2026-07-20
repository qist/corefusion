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
	var formats []entity.SubscriptionFormat
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&formats).Error
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

// GetByID returns a subscription format by ID.
func (r *SubscriptionFormatRepository) GetByID(ctx context.Context, id int64) (*entity.SubscriptionFormat, error) {
	var format entity.SubscriptionFormat
	err := r.db.WithContext(ctx).First(&format, id).Error
	return &format, err
}

// Create creates a new subscription format.
func (r *SubscriptionFormatRepository) Create(ctx context.Context, format *entity.SubscriptionFormat) error {
	return r.db.WithContext(ctx).Create(format).Error
}

// Update updates an existing subscription format.
func (r *SubscriptionFormatRepository) Update(ctx context.Context, format *entity.SubscriptionFormat) error {
	return r.db.WithContext(ctx).Save(format).Error
}

// Delete deletes a subscription format by ID.
func (r *SubscriptionFormatRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete associated rules first
		if err := tx.WithContext(ctx).Where("format_id = ?", id).Delete(&entity.SubscriptionRule{}).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Delete(&entity.SubscriptionFormat{}, id).Error
	})
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

// ListByFormatID returns all rules for a format.
func (r *SubscriptionRuleRepository) ListByFormatID(ctx context.Context, formatID int64) ([]entity.SubscriptionRule, error) {
	var rules []entity.SubscriptionRule
	err := r.db.WithContext(ctx).Where("format_id = ?", formatID).Order("priority ASC, id ASC").Find(&rules).Error
	return rules, err
}

// GetByID returns a rule by ID.
func (r *SubscriptionRuleRepository) GetByID(ctx context.Context, id int64) (*entity.SubscriptionRule, error) {
	var rule entity.SubscriptionRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	return &rule, err
}

// Create creates a new rule.
func (r *SubscriptionRuleRepository) Create(ctx context.Context, rule *entity.SubscriptionRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

// Update updates an existing rule.
func (r *SubscriptionRuleRepository) Update(ctx context.Context, rule *entity.SubscriptionRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

// Delete deletes a rule by ID.
func (r *SubscriptionRuleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.SubscriptionRule{}, id).Error
}

// DeleteByFormatID deletes all rules for a format.
func (r *SubscriptionRuleRepository) DeleteByFormatID(ctx context.Context, formatID int64) error {
	return r.db.WithContext(ctx).Where("format_id = ?", formatID).Delete(&entity.SubscriptionRule{}).Error
}

// BatchUpdatePriority updates priority for multiple rules.
func (r *SubscriptionRuleRepository) BatchUpdatePriority(ctx context.Context, updates []struct {
	ID       int64
	Priority int
}) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, u := range updates {
			if err := tx.Model(&entity.SubscriptionRule{}).Where("id = ?", u.ID).Update("priority", u.Priority).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
