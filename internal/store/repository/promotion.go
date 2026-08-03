package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// PromotionRepository handles promotions database operations.
type PromotionRepository struct {
	db *gorm.DB
}

func NewPromotionRepository(db *gorm.DB) *PromotionRepository {
	return &PromotionRepository{db: db}
}

func (r *PromotionRepository) List(ctx context.Context, limit, offset int) ([]entity.Promotion, int64, error) {
	var items []entity.Promotion
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.Promotion{})
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *PromotionRepository) GetByID(ctx context.Context, id int64) (*entity.Promotion, error) {
	var item entity.Promotion
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *PromotionRepository) GetByCode(ctx context.Context, code string) (*entity.Promotion, error) {
	var item entity.Promotion
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	return &item, err
}

func (r *PromotionRepository) Create(ctx context.Context, item *entity.Promotion) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PromotionRepository) Update(ctx context.Context, item *entity.Promotion) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *PromotionRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Promotion{}, id).Error
}

// IncrementUsedCount atomically increments the used_count.
func (r *PromotionRepository) IncrementUsedCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&entity.Promotion{}).
		Where("id = ?", id).
		UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
}

// TryIncrementUsedCount atomically increments used_count only if the max_uses
// limit has not been reached. Returns true if the increment succeeded.
// When maxUses is 0, the promotion has unlimited total uses (always succeeds).
func (r *PromotionRepository) TryIncrementUsedCount(ctx context.Context, id int64, maxUses int) (bool, error) {
	q := r.db.WithContext(ctx).Model(&entity.Promotion{}).Where("id = ?", id)
	if maxUses > 0 {
		q = q.Where("used_count < ?", maxUses)
	}
	result := q.UpdateColumn("used_count", gorm.Expr("used_count + 1"))
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
