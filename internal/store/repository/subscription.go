package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// SubscriptionRepository handles subscription database operations.
type SubscriptionRepository struct {
	db *gorm.DB
}

// NewSubscriptionRepository creates a new SubscriptionRepository.
func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

// Create creates a new subscription.
func (r *SubscriptionRepository) Create(ctx context.Context, sub *entity.Subscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

// GetByID retrieves a subscription by ID.
func (r *SubscriptionRepository) GetByID(ctx context.Context, id int64) (*entity.Subscription, error) {
	var sub entity.Subscription
	err := r.db.WithContext(ctx).First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByToken retrieves a subscription by token.
func (r *SubscriptionRepository) GetByToken(ctx context.Context, token string) (*entity.Subscription, error) {
	var sub entity.Subscription
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// List retrieves all subscriptions with pagination.
func (r *SubscriptionRepository) List(ctx context.Context, status string, limit, offset int) ([]entity.Subscription, int64, error) {
	var subs []entity.Subscription
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Subscription{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&subs).Error
	return subs, total, err
}

// Update updates a subscription.
func (r *SubscriptionRepository) Update(ctx context.Context, sub *entity.Subscription) error {
	return r.db.WithContext(ctx).Save(sub).Error
}

// Delete deletes a subscription by ID.
func (r *SubscriptionRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Subscription{}, id).Error
}

// UpdateAccessStats updates the access stats for a subscription.
func (r *SubscriptionRepository) UpdateAccessStats(ctx context.Context, id int64, ip, userAgent string) error {
	return r.db.WithContext(ctx).Model(&entity.Subscription{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_access_at": gorm.Expr("CURRENT_TIMESTAMP"),
		"access_count":   gorm.Expr("access_count + 1"),
		"last_ip":        ip,
		"last_user_agent": userAgent,
	}).Error
}

// AccessLogRepository handles subscription access log operations.
type AccessLogRepository struct {
	db *gorm.DB
}

// NewAccessLogRepository creates a new AccessLogRepository.
func NewAccessLogRepository(db *gorm.DB) *AccessLogRepository {
	return &AccessLogRepository{db: db}
}

// Create creates a new access log entry.
func (r *AccessLogRepository) Create(ctx context.Context, log *entity.SubscriptionAccessLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListBySubscriptionID retrieves access logs for a subscription.
func (r *AccessLogRepository) ListBySubscriptionID(ctx context.Context, subID int64, limit, offset int) ([]entity.SubscriptionAccessLog, int64, error) {
	var logs []entity.SubscriptionAccessLog
	var total int64

	r.db.WithContext(ctx).Model(&entity.SubscriptionAccessLog{}).Where("subscription_id = ?", subID).Count(&total)
	err := r.db.WithContext(ctx).Where("subscription_id = ?", subID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// ListByQuotaStates retrieves active subscriptions with matching quota states.
func (r *SubscriptionRepository) ListByQuotaStates(ctx context.Context, states []string) ([]entity.Subscription, error) {
	var subs []entity.Subscription
	err := r.db.WithContext(ctx).Where("quota_state IN ? AND status = ?", states, "active").Find(&subs).Error
	return subs, err
}

// RotationRepository handles subscription rotation history operations.
type RotationRepository struct {
	db *gorm.DB
}

// NewRotationRepository creates a new RotationRepository.
func NewRotationRepository(db *gorm.DB) *RotationRepository {
	return &RotationRepository{db: db}
}

// Create creates a new rotation record.
func (r *RotationRepository) Create(ctx context.Context, rot *entity.SubscriptionRotation) error {
	return r.db.WithContext(ctx).Create(rot).Error
}

// ListBySubscriptionID retrieves rotation history for a subscription.
func (r *RotationRepository) ListBySubscriptionID(ctx context.Context, subID int64, limit, offset int) ([]entity.SubscriptionRotation, int64, error) {
	var rotations []entity.SubscriptionRotation
	var total int64

	r.db.WithContext(ctx).Model(&entity.SubscriptionRotation{}).Where("subscription_id = ?", subID).Count(&total)
	err := r.db.WithContext(ctx).Where("subscription_id = ?", subID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&rotations).Error
	return rotations, total, err
}
