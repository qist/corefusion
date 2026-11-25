package repository

import (
	"context"

	"corefusion/internal/model/entity"
	"gorm.io/gorm"
)

type NodeCheckupRepository struct {
	db *gorm.DB
}

func NewNodeCheckupRepository(db *gorm.DB) *NodeCheckupRepository {
	return &NodeCheckupRepository{db: db}
}

func (r *NodeCheckupRepository) Upsert(ctx context.Context, res *entity.NodeCheckupResult) error {
	return r.db.WithContext(ctx).Save(res).Error
}

func (r *NodeCheckupRepository) Get(ctx context.Context, serverID int64) (*entity.NodeCheckupResult, error) {
	var out entity.NodeCheckupResult
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *NodeCheckupRepository) List(ctx context.Context) ([]entity.NodeCheckupResult, error) {
	var out []entity.NodeCheckupResult
	if err := r.db.WithContext(ctx).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
