package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ChainExitConfigRepository handles chain_exit_configs database operations.
type ChainExitConfigRepository struct {
	db *gorm.DB
}

func NewChainExitConfigRepository(db *gorm.DB) *ChainExitConfigRepository {
	return &ChainExitConfigRepository{db: db}
}

// GetByServerID retrieves the chain exit config for a server.
func (r *ChainExitConfigRepository) GetByServerID(ctx context.Context, serverID int64) (*entity.ChainExitConfig, error) {
	var cfg entity.ChainExitConfig
	err := r.db.WithContext(ctx).Where("server_id = ?", serverID).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ListAll returns all chain exit configs.
func (r *ChainExitConfigRepository) ListAll(ctx context.Context) ([]entity.ChainExitConfig, error) {
	var configs []entity.ChainExitConfig
	err := r.db.WithContext(ctx).Order("id ASC").Find(&configs).Error
	return configs, err
}

// Create creates a new chain exit config.
func (r *ChainExitConfigRepository) Create(ctx context.Context, cfg *entity.ChainExitConfig) error {
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(cfg).Error
}

// Update updates a chain exit config.
func (r *ChainExitConfigRepository) Update(ctx context.Context, cfg *entity.ChainExitConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Upsert creates or updates a chain exit config by server_id.
func (r *ChainExitConfigRepository) Upsert(ctx context.Context, cfg *entity.ChainExitConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Delete deletes a chain exit config by server_id.
func (r *ChainExitConfigRepository) Delete(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).Where("server_id = ?", serverID).Delete(&entity.ChainExitConfig{}).Error
}
