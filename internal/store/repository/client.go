package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

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

// GetTokenVersion 读取客户端当前令牌版本（供鉴权中间件与刷新校验）。
func (r *ClientRepository) GetTokenVersion(ctx context.Context, clientID int64) (int, error) {
	var v int
	err := r.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", clientID).
		Select("COALESCE(token_version, 0)").
		Scan(&v).Error
	return v, err
}

// BumpTokenVersion 把客户端令牌版本 +1 → 此前签发的所有 token 立即失效（登出/踢下线）。
func (r *ClientRepository) BumpTokenVersion(ctx context.Context, clientID int64) error {
	return r.db.WithContext(ctx).Model(&entity.Client{}).
		Where("id = ?", clientID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

func (r *ClientRepository) GetByEmail(ctx context.Context, email string) (*entity.Client, error) {
	var client entity.Client
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&client).Error
	return &client, err
}

// FindByIDs returns clients keyed by ID for the given IDs.
func (r *ClientRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*entity.Client, error) {
	result := make(map[int64]*entity.Client)
	if len(ids) == 0 {
		return result, nil
	}
	var clients []entity.Client
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&clients).Error; err != nil {
		return nil, err
	}
	for i := range clients {
		result[clients[i].ID] = &clients[i]
	}
	return result, nil
}

// SearchByEmail returns clients whose email matches the keyword (LIKE %keyword%).
func (r *ClientRepository) SearchByEmail(ctx context.Context, keyword string, limit int) ([]entity.Client, error) {
	var clients []entity.Client
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	err := r.db.WithContext(ctx).
		Where("email LIKE ?", "%"+keyword+"%").
		Order("id DESC").
		Limit(limit).
		Find(&clients).Error
	return clients, err
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
