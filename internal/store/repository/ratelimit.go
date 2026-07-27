package repository

import (
	"context"

	"corefusion/internal/model/entity"

	"gorm.io/gorm"
)

// RateLimitConfigRepository handles database operations for rate limit configs.
type RateLimitConfigRepository struct {
	db *gorm.DB
}

// NewRateLimitConfigRepository creates a new repository.
func NewRateLimitConfigRepository(db *gorm.DB) *RateLimitConfigRepository {
	return &RateLimitConfigRepository{db: db}
}

// GetByServerID retrieves the rate limit config for a server.
func (r *RateLimitConfigRepository) GetByServerID(ctx context.Context, serverID int64) (*entity.RateLimitConfig, error) {
	var config entity.RateLimitConfig
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// Upsert creates or updates the rate limit config for a server.
func (r *RateLimitConfigRepository) Upsert(ctx context.Context, config *entity.RateLimitConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// Delete removes the rate limit config for a server.
func (r *RateLimitConfigRepository) Delete(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).Where("server_id = ?", serverID).Delete(&entity.RateLimitConfig{}).Error
}

// ListAll retrieves all rate limit configs.
func (r *RateLimitConfigRepository) ListAll(ctx context.Context) ([]entity.RateLimitConfig, error) {
	var configs []entity.RateLimitConfig
	if err := r.db.WithContext(ctx).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// ListEnabled retrieves all enabled rate limit configs.
func (r *RateLimitConfigRepository) ListEnabled(ctx context.Context) ([]entity.RateLimitConfig, error) {
	var configs []entity.RateLimitConfig
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// ListEnabledServers returns the server IDs that have rate limiting enabled.
func (r *RateLimitConfigRepository) ListEnabledServers(ctx context.Context) ([]entity.NodeServer, error) {
	var servers []entity.NodeServer
	if err := r.db.WithContext(ctx).
		Joins("JOIN rate_limit_configs ON rate_limit_configs.server_id = node_servers.id").
		Where("rate_limit_configs.enabled = ?", true).
		Find(&servers).Error; err != nil {
		return nil, err
	}
	return servers, nil
}
