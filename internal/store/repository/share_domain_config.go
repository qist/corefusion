package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ShareDomainConfigRepository handles share domain config database operations.
type ShareDomainConfigRepository struct {
	db *gorm.DB
}

// NewShareDomainConfigRepository creates a new share domain config repository.
func NewShareDomainConfigRepository(db *gorm.DB) *ShareDomainConfigRepository {
	return &ShareDomainConfigRepository{db: db}
}

// List returns all share domain configs, optionally filtered by type.
func (r *ShareDomainConfigRepository) List(ctx context.Context, configType string) ([]entity.ShareDomainConfig, error) {
	var configs []entity.ShareDomainConfig
	query := r.db.WithContext(ctx).Order("sort_order ASC, id ASC")
	if configType != "" {
		query = query.Where("type = ?", configType)
	}
	err := query.Find(&configs).Error
	return configs, err
}

// GetByID returns a share domain config by ID.
func (r *ShareDomainConfigRepository) GetByID(ctx context.Context, id int64) (*entity.ShareDomainConfig, error) {
	var config entity.ShareDomainConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	return &config, err
}

// Create creates a new share domain config.
func (r *ShareDomainConfigRepository) Create(ctx context.Context, config *entity.ShareDomainConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// Update updates an existing share domain config.
func (r *ShareDomainConfigRepository) Update(ctx context.Context, config *entity.ShareDomainConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// Delete deletes a share domain config by ID.
func (r *ShareDomainConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ShareDomainConfig{}, id).Error
}

// ClearAll deletes all share domain configs, optionally filtered by type.
func (r *ShareDomainConfigRepository) ClearAll(ctx context.Context, configType string) error {
	query := r.db.WithContext(ctx)
	if configType != "" {
		query = query.Where("type = ?", configType)
	}
	return query.Delete(&entity.ShareDomainConfig{}).Error
}

// Count returns the total number of share domain configs.
func (r *ShareDomainConfigRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.ShareDomainConfig{}).Count(&count).Error
	return count, err
}
