package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// OutboundConfigRepository handles outbound_config database operations.
type OutboundConfigRepository struct {
	db *gorm.DB
}

// NewOutboundConfigRepository creates a new OutboundConfigRepository.
func NewOutboundConfigRepository(db *gorm.DB) *OutboundConfigRepository {
	return &OutboundConfigRepository{db: db}
}

// List retrieves all outbound configs for a core type on a specific server.
func (r *OutboundConfigRepository) List(ctx context.Context, serverID int64, coreType string) ([]entity.OutboundConfig, error) {
	var configs []entity.OutboundConfig
	query := r.db.WithContext(ctx).Where("server_id = ?", serverID)
	if coreType != "" {
		query = query.Where("core_type = ?", coreType)
	}
	err := query.Order("id").Find(&configs).Error
	return configs, err
}

// GetByID retrieves an outbound config by ID.
func (r *OutboundConfigRepository) GetByID(ctx context.Context, serverID int64, id int64) (*entity.OutboundConfig, error) {
	var cfg entity.OutboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND id = ?", serverID, id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetByTag retrieves an outbound config by tag on a specific server.
func (r *OutboundConfigRepository) GetByTag(ctx context.Context, serverID int64, tag string) (*entity.OutboundConfig, error) {
	var cfg entity.OutboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND tag = ?", serverID, tag).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Create creates a new outbound config.
func (r *OutboundConfigRepository) Create(ctx context.Context, cfg *entity.OutboundConfig) error {
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(cfg).Error
}

// Update updates an outbound config.
func (r *OutboundConfigRepository) Update(ctx context.Context, cfg *entity.OutboundConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Upsert creates or updates an outbound config by (server_id, tag).
func (r *OutboundConfigRepository) Upsert(ctx context.Context, cfg *entity.OutboundConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}, {Name: "tag"}},
		DoUpdates: clause.AssignmentColumns([]string{"remark", "protocol", "config_json", "enabled", "is_default", "updated_at"}),
	}).Create(cfg).Error
}

// Delete deletes an outbound config by ID.
func (r *OutboundConfigRepository) Delete(ctx context.Context, serverID int64, id int64) error {
	return r.db.WithContext(ctx).Where("server_id = ? AND id = ?", serverID, id).Delete(&entity.OutboundConfig{}).Error
}

// DeleteByTag deletes an outbound config by tag on a specific server.
func (r *OutboundConfigRepository) DeleteByTag(ctx context.Context, serverID int64, tag string) error {
	return r.db.WithContext(ctx).Where("server_id = ? AND tag = ?", serverID, tag).Delete(&entity.OutboundConfig{}).Error
}

// GetDefault retrieves default outbound configs for a core type on a specific server.
func (r *OutboundConfigRepository) GetDefault(ctx context.Context, serverID int64, coreType string) ([]entity.OutboundConfig, error) {
	var configs []entity.OutboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND core_type = ? AND is_default = ?", serverID, coreType, true).Order("id").Find(&configs).Error
	return configs, err
}
