package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// PlanRepository handles plan database operations.
type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) List(ctx context.Context) ([]entity.Plan, error) {
	var plans []entity.Plan
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&plans).Error
	return plans, err
}

func (r *PlanRepository) GetByID(ctx context.Context, id int64) (*entity.Plan, error) {
	var plan entity.Plan
	err := r.db.WithContext(ctx).First(&plan, id).Error
	return &plan, err
}

func (r *PlanRepository) Create(ctx context.Context, plan *entity.Plan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *PlanRepository) Update(ctx context.Context, plan *entity.Plan) error {
	return r.db.WithContext(ctx).Save(plan).Error
}

func (r *PlanRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete plan nodes
		if err := tx.Where("plan_id = ?", id).Delete(&entity.PlanNode{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.Plan{}, id).Error
	})
}

// PlanNodeRepository handles plan-node mapping database operations.
type PlanNodeRepository struct {
	db *gorm.DB
}

func NewPlanNodeRepository(db *gorm.DB) *PlanNodeRepository {
	return &PlanNodeRepository{db: db}
}

func (r *PlanNodeRepository) ListByPlanID(ctx context.Context, planID int64) ([]entity.PlanNode, error) {
	var nodes []entity.PlanNode
	err := r.db.WithContext(ctx).Where("plan_id = ?", planID).Order("id ASC").Find(&nodes).Error
	return nodes, err
}

func (r *PlanNodeRepository) Create(ctx context.Context, node *entity.PlanNode) error {
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *PlanNodeRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.PlanNode{}, id).Error
}

func (r *PlanNodeRepository) DeleteByPlanID(ctx context.Context, planID int64) error {
	return r.db.WithContext(ctx).Where("plan_id = ?", planID).Delete(&entity.PlanNode{}).Error
}

// ClientRepository handles client database operations.
type ClientRepository struct {
	db *gorm.DB
}

func NewClientRepository(db *gorm.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) List(ctx context.Context, planID int64, status string, limit, offset int) ([]entity.Client, int64, error) {
	var clients []entity.Client
	var total int64

	q := r.db.WithContext(ctx).Model(&entity.Client{})
	if planID > 0 {
		q = q.Where("plan_id = ?", planID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q.Count(&total)

	err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&clients).Error
	return clients, total, err
}

// ListByPlanID returns all active clients bound to the given plan template ID.
// Used for batch account synchronization when a new node is added to a plan.
func (r *ClientRepository) ListByPlanID(ctx context.Context, planID int64) ([]entity.Client, error) {
	var clients []entity.Client
	err := r.db.WithContext(ctx).
		Where("plan_id = ? AND status = ?", planID, "active").
		Order("id ASC").
		Find(&clients).Error
	return clients, err
}

func (r *ClientRepository) GetByID(ctx context.Context, id int64) (*entity.Client, error) {
	var client entity.Client
	err := r.db.WithContext(ctx).First(&client, id).Error
	return &client, err
}

func (r *ClientRepository) GetByEmail(ctx context.Context, email string) (*entity.Client, error) {
	var client entity.Client
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&client).Error
	return &client, err
}

func (r *ClientRepository) Create(ctx context.Context, client *entity.Client) error {
	return r.db.WithContext(ctx).Create(client).Error
}

func (r *ClientRepository) Update(ctx context.Context, client *entity.Client) error {
	return r.db.WithContext(ctx).Save(client).Error
}

func (r *ClientRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Client{}, id).Error
}

func (r *ClientRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ClientRepository) UpdateUsage(ctx context.Context, id int64, usedBytes int64) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).Where("id = ?", id).Update("used_bytes", usedBytes).Error
}

func (r *ClientRepository) UpdateSubscriptionID(ctx context.Context, id int64, subID int64) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).Where("id = ?", id).Update("subscription_id", subID).Error
}
