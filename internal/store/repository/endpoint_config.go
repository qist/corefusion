package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// EndpointConfigRepository provides data access for endpoint configs.
type EndpointConfigRepository struct {
	db *gorm.DB
}

// NewEndpointConfigRepository creates a new endpoint config repository.
func NewEndpointConfigRepository(db *gorm.DB) *EndpointConfigRepository {
	return &EndpointConfigRepository{db: db}
}

// List returns all endpoint configs for a server and core type.
func (r *EndpointConfigRepository) List(ctx context.Context, serverID int64, coreType string) ([]entity.EndpointConfig, error) {
	var configs []entity.EndpointConfig
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND core_type = ?", serverID, coreType).
		Order("id ASC").
		Find(&configs).Error
	return configs, err
}

// GetByID returns a single endpoint config by ID and server ID.
func (r *EndpointConfigRepository) GetByID(ctx context.Context, id, serverID int64) (*entity.EndpointConfig, error) {
	var cfg entity.EndpointConfig
	err := r.db.WithContext(ctx).
		Where("id = ? AND server_id = ?", id, serverID).
		First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Create inserts a new endpoint config.
func (r *EndpointConfigRepository) Create(ctx context.Context, cfg *entity.EndpointConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

// Update modifies an existing endpoint config.
func (r *EndpointConfigRepository) Update(ctx context.Context, cfg *entity.EndpointConfig) error {
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Delete removes an endpoint config by ID and server ID.
func (r *EndpointConfigRepository) Delete(ctx context.Context, id, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND server_id = ?", id, serverID).
		Delete(&entity.EndpointConfig{}).Error
}
