package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// SubscriptionShortLinkRepository handles subscription short link persistence.
type SubscriptionShortLinkRepository struct {
	db *gorm.DB
}

func NewSubscriptionShortLinkRepository(db *gorm.DB) *SubscriptionShortLinkRepository {
	return &SubscriptionShortLinkRepository{db: db}
}

func (r *SubscriptionShortLinkRepository) Create(ctx context.Context, link *entity.SubscriptionShortLink) error {
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *SubscriptionShortLinkRepository) GetByID(ctx context.Context, id int64) (*entity.SubscriptionShortLink, error) {
	var link entity.SubscriptionShortLink
	if err := r.db.WithContext(ctx).First(&link, id).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *SubscriptionShortLinkRepository) GetByCode(ctx context.Context, code string) (*entity.SubscriptionShortLink, error) {
	var link entity.SubscriptionShortLink
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

func (r *SubscriptionShortLinkRepository) CodeExists(ctx context.Context, code string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&entity.SubscriptionShortLink{}).
		Where("code = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SubscriptionShortLinkRepository) ListBySubscription(ctx context.Context, subID int64) ([]entity.SubscriptionShortLink, error) {
	var links []entity.SubscriptionShortLink
	err := r.db.WithContext(ctx).
		Where("subscription_id = ?", subID).
		Order("id ASC").
		Find(&links).Error
	return links, err
}

func (r *SubscriptionShortLinkRepository) Update(ctx context.Context, link *entity.SubscriptionShortLink) error {
	return r.db.WithContext(ctx).Save(link).Error
}

func (r *SubscriptionShortLinkRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.SubscriptionShortLink{}, id).Error
}

func (r *SubscriptionShortLinkRepository) DeleteBySubscriptionID(ctx context.Context, subID int64) error {
	return r.db.WithContext(ctx).
		Where("subscription_id = ?", subID).
		Delete(&entity.SubscriptionShortLink{}).Error
}

// SyncTokenVersion 用于 FollowTokenRotation=true 的短链：轮换后同步快照版本。
func (r *SubscriptionShortLinkRepository) SyncTokenVersion(ctx context.Context, subID int64, version int) error {
	return r.db.WithContext(ctx).
		Model(&entity.SubscriptionShortLink{}).
		Where("subscription_id = ? AND follow_token_rotation = ?", subID, true).
		Updates(map[string]interface{}{
			"token_version": version,
			"updated_at":    time.Now(),
		}).Error
}

// RecordAccess 只更新访问统计两列，避免整行 Save 覆盖并发中的其它字段。
func (r *SubscriptionShortLinkRepository) RecordAccess(ctx context.Context, id int64, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&entity.SubscriptionShortLink{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count":   gorm.Expr("access_count + 1"),
			"last_access_at": at,
		}).Error
}
