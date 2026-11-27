package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// SubscriptionShortLinkRevocationRepository 管理已撤销短码的 HMAC 指纹。
type SubscriptionShortLinkRevocationRepository struct {
	db *gorm.DB
}

func NewSubscriptionShortLinkRevocationRepository(db *gorm.DB) *SubscriptionShortLinkRevocationRepository {
	return &SubscriptionShortLinkRevocationRepository{db: db}
}

// Create 幂等写入：同一个指纹重复撤销不报错。
func (r *SubscriptionShortLinkRevocationRepository) Create(ctx context.Context, rev *entity.SubscriptionShortLinkRevocation) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(rev).Error
}

// Exists 判断某个短码指纹是否已撤销。
func (r *SubscriptionShortLinkRevocationRepository) Exists(ctx context.Context, codeHMAC string) (bool, error) {
	if codeHMAC == "" {
		return false, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&entity.SubscriptionShortLinkRevocation{}).
		Where("code_hmac = ?", codeHMAC).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
