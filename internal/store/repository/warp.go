package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// WarpRepository handles WARP config database operations.
type WarpRepository struct {
	db *gorm.DB
}

// NewWarpRepository creates a new WarpRepository.
func NewWarpRepository(db *gorm.DB) *WarpRepository {
	return &WarpRepository{db: db}
}

// Create creates a new WARP config.
func (r *WarpRepository) Create(ctx context.Context, cfg *entity.WarpConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

// GetByID retrieves a WARP config by ID.
func (r *WarpRepository) GetByID(ctx context.Context, id int64) (*entity.WarpConfig, error) {
	var cfg entity.WarpConfig
	err := r.db.WithContext(ctx).First(&cfg, id).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetActive retrieves the latest active WARP config.
func (r *WarpRepository) GetActive(ctx context.Context) (*entity.WarpConfig, error) {
	var cfg entity.WarpConfig
	err := r.db.WithContext(ctx).Where("status != ?", "not_registered").Order("id DESC").First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Update updates a WARP config.
func (r *WarpRepository) Update(ctx context.Context, cfg *entity.WarpConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Delete deletes a WARP config by ID.
func (r *WarpRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.WarpConfig{}, id).Error
}
