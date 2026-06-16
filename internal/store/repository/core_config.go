package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// CoreConfigRepository handles core_config database operations.
type CoreConfigRepository struct {
	db *gorm.DB
}

// NewCoreConfigRepository creates a new CoreConfigRepository.
func NewCoreConfigRepository(db *gorm.DB) *CoreConfigRepository {
	return &CoreConfigRepository{db: db}
}

// GetSection retrieves one section config for a core type.
func (r *CoreConfigRepository) GetSection(ctx context.Context, coreType, section string) (*entity.CoreConfig, error) {
	var cfg entity.CoreConfig
	err := r.db.WithContext(ctx).
		Where("core_type = ? AND section = ?", coreType, section).
		First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetAllSections retrieves all section configs for a core type.
func (r *CoreConfigRepository) GetAllSections(ctx context.Context, coreType string) ([]entity.CoreConfig, error) {
	var configs []entity.CoreConfig
	err := r.db.WithContext(ctx).
		Where("core_type = ?", coreType).
		Order("section").
		Find(&configs).Error
	return configs, err
}

// List retrieves summary (core_type + section + updated_at) for all cores.
func (r *CoreConfigRepository) List(ctx context.Context) ([]entity.CoreConfig, error) {
	var configs []entity.CoreConfig
	err := r.db.WithContext(ctx).
		Order("core_type, section").
		Find(&configs).Error
	return configs, err
}

// UpsertSection creates or updates a single section.
func (r *CoreConfigRepository) UpsertSection(ctx context.Context, cfg *entity.CoreConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "core_type"}, {Name: "section"}},
		DoUpdates: clause.AssignmentColumns([]string{"config_json", "updated_at"}),
	}).Create(cfg).Error
}

// DeleteSection removes a single section.
func (r *CoreConfigRepository) DeleteSection(ctx context.Context, coreType, section string) error {
	return r.db.WithContext(ctx).
		Where("core_type = ? AND section = ?", coreType, section).
		Delete(&entity.CoreConfig{}).Error
}
