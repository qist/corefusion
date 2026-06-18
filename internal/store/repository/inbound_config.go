package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// InboundConfigRepository handles inbound_config database operations.
type InboundConfigRepository struct {
	db *gorm.DB
}

// NewInboundConfigRepository creates a new InboundConfigRepository.
func NewInboundConfigRepository(db *gorm.DB) *InboundConfigRepository {
	return &InboundConfigRepository{db: db}
}

// List retrieves all inbound configs for a core type.
func (r *InboundConfigRepository) List(ctx context.Context, coreType string) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	query := r.db.WithContext(ctx)
	if coreType != "" {
		query = query.Where("core_type = ?", coreType)
	}
	err := query.Order("id").Find(&configs).Error
	return configs, err
}

// GetByID retrieves an inbound config by ID.
func (r *InboundConfigRepository) GetByID(ctx context.Context, id int64) (*entity.InboundConfig, error) {
	var cfg entity.InboundConfig
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetByTag retrieves an inbound config by tag.
func (r *InboundConfigRepository) GetByTag(ctx context.Context, tag string) (*entity.InboundConfig, error) {
	var cfg entity.InboundConfig
	err := r.db.WithContext(ctx).Where("tag = ?", tag).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Create creates a new inbound config.
func (r *InboundConfigRepository) Create(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(cfg).Error
}

// Update updates an inbound config.
func (r *InboundConfigRepository) Update(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Upsert creates or updates an inbound config by tag.
func (r *InboundConfigRepository) Upsert(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tag"}},
		DoUpdates: clause.AssignmentColumns([]string{"remark", "protocol", "listen", "port", "config_json", "enabled", "updated_at"}),
	}).Create(cfg).Error
}

// Delete deletes an inbound config by ID.
func (r *InboundConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.InboundConfig{}).Error
}

// DeleteByTag deletes an inbound config by tag.
func (r *InboundConfigRepository) DeleteByTag(ctx context.Context, tag string) error {
	return r.db.WithContext(ctx).Where("tag = ?", tag).Delete(&entity.InboundConfig{}).Error
}
