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

// List retrieves all inbound configs for a core type on a specific server.
func (r *InboundConfigRepository) List(ctx context.Context, serverID int64, coreType string) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	query := r.db.WithContext(ctx).Where("server_id = ?", serverID)
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

// GetByTag retrieves an inbound config by tag on a specific server.
func (r *InboundConfigRepository) GetByTag(ctx context.Context, serverID int64, tag string) (*entity.InboundConfig, error) {
	var cfg entity.InboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND tag = ?", serverID, tag).First(&cfg).Error
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

// Upsert creates or updates an inbound config by (server_id, tag).
func (r *InboundConfigRepository) Upsert(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}, {Name: "tag"}},
		DoUpdates: clause.AssignmentColumns([]string{"remark", "protocol", "listen", "port", "config_json", "enabled", "region", "updated_at"}),
	}).Create(cfg).Error
}

// Delete deletes an inbound config by ID.
func (r *InboundConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.InboundConfig{}).Error
}

// DeleteByTag deletes an inbound config by tag on a specific server.
func (r *InboundConfigRepository) DeleteByTag(ctx context.Context, serverID int64, tag string) error {
	return r.db.WithContext(ctx).Where("server_id = ? AND tag = ?", serverID, tag).Delete(&entity.InboundConfig{}).Error
}

// ListAll retrieves all inbound configs across all servers (for subscription generation).
func (r *InboundConfigRepository) ListAll(ctx context.Context, coreType string) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	query := r.db.WithContext(ctx)
	if coreType != "" {
		query = query.Where("core_type = ?", coreType)
	}
	err := query.Order("id").Find(&configs).Error
	return configs, err
}

// ListByServerID retrieves all inbound configs for a specific server.
func (r *InboundConfigRepository) ListByServerID(ctx context.Context, serverID int64) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("id").
		Find(&configs).Error
	return configs, err
}

// ListByServerIDs retrieves all inbound configs for the given server IDs.
func (r *InboundConfigRepository) ListByServerIDs(ctx context.Context, serverIDs []int64) ([]entity.InboundConfig, error) {
	if len(serverIDs) == 0 {
		return nil, nil
	}
	var configs []entity.InboundConfig
	err := r.db.WithContext(ctx).
		Where("server_id IN ?", serverIDs).
		Order("server_id, id").
		Find(&configs).Error
	return configs, err
}
