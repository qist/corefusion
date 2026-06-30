package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// PlanOrderRepository handles plan_orders database operations.
type PlanOrderRepository struct {
	db *gorm.DB
}

func NewPlanOrderRepository(db *gorm.DB) *PlanOrderRepository {
	return &PlanOrderRepository{db: db}
}

func (r *PlanOrderRepository) ListByClientID(ctx context.Context, clientID int64, limit, offset int) ([]entity.PlanOrder, int64, error) {
	var items []entity.PlanOrder
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.PlanOrder{}).Where("client_id = ?", clientID)
	q.Count(&total)
	err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	return items, total, err
}

func (r *PlanOrderRepository) GetByID(ctx context.Context, id int64) (*entity.PlanOrder, error) {
	var item entity.PlanOrder
	err := r.db.WithContext(ctx).First(&item, id).Error
	return &item, err
}

func (r *PlanOrderRepository) Create(ctx context.Context, item *entity.PlanOrder) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *PlanOrderRepository) Update(ctx context.Context, item *entity.PlanOrder) error {
	return r.db.WithContext(ctx).Save(item).Error
}
