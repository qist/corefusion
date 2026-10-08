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
	err := query.Order("sort_order ASC, id ASC").Find(&configs).Error
	if err == nil {
		decryptConfigs(configs, func(c *entity.OutboundConfig) *string { return &c.ConfigJSON })
	}
	return configs, err
}

// GetByID retrieves an outbound config by ID.
func (r *OutboundConfigRepository) GetByID(ctx context.Context, serverID int64, id int64) (*entity.OutboundConfig, error) {
	var cfg entity.OutboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND id = ?", serverID, id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	cfg.ConfigJSON = DecryptConfigJSON(cfg.ConfigJSON)
	return &cfg, nil
}

// GetByTag retrieves an outbound config by tag on a specific server.
func (r *OutboundConfigRepository) GetByTag(ctx context.Context, serverID int64, tag string) (*entity.OutboundConfig, error) {
	var cfg entity.OutboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND tag = ?", serverID, tag).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	cfg.ConfigJSON = DecryptConfigJSON(cfg.ConfigJSON)
	return &cfg, nil
}

// Create creates a new outbound config.
func (r *OutboundConfigRepository) Create(ctx context.Context, cfg *entity.OutboundConfig) error {
	cfg.ConfigJSON = EncryptConfigJSON(cfg.ConfigJSON)
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(cfg).Error
}

// Update updates an outbound config.
func (r *OutboundConfigRepository) Update(ctx context.Context, cfg *entity.OutboundConfig) error {
	cfg.ConfigJSON = EncryptConfigJSON(cfg.ConfigJSON)
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Upsert creates or updates an outbound config by (server_id, tag).
func (r *OutboundConfigRepository) Upsert(ctx context.Context, cfg *entity.OutboundConfig) error {
	cfg.ConfigJSON = EncryptConfigJSON(cfg.ConfigJSON)
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}, {Name: "tag"}},
		DoUpdates: clause.AssignmentColumns([]string{"remark", "protocol", "config_json", "enabled", "is_default", "sort_order", "updated_at"}),
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
	err := r.db.WithContext(ctx).Where("server_id = ? AND core_type = ? AND is_default = ?", serverID, coreType, true).Order("sort_order ASC, id ASC").Find(&configs).Error
	if err == nil {
		decryptConfigs(configs, func(c *entity.OutboundConfig) *string { return &c.ConfigJSON })
	}
	return configs, err
}

// GetMaxSortOrder returns the highest sort_order for a given server and core type.
func (r *OutboundConfigRepository) GetMaxSortOrder(ctx context.Context, serverID int64, coreType string) (int, error) {
	var maxOrder int
	err := r.db.WithContext(ctx).Model(&entity.OutboundConfig{}).
		Where("server_id = ? AND core_type = ?", serverID, coreType).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder).Error
	return maxOrder, err
}

// Reorder updates the sort_order of multiple outbound configs in a single transaction.
// orders is a map of outbound ID → new sort_order value.
func (r *OutboundConfigRepository) Reorder(ctx context.Context, serverID int64, orders map[int64]int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, order := range orders {
			if err := tx.Model(&entity.OutboundConfig{}).
				Where("server_id = ? AND id = ?", serverID, id).
				Update("sort_order", order).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
