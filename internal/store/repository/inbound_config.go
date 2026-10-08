package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// panelMetaKeys 是寄存在 config_json 用户对象里的**面板元数据**，不随配置落库：
//
//   - _usedUp/_usedDown：流量计数，已拆到 inbound_user_traffic 表；
//   - _quota/_expiry：前端遗留字段，后端从不读取（配额走订阅 used_bytes_cache）。
//
// (_paused / _accessDomain / _portHop 属于配置语义，保留。)
var panelMetaKeys = [...]string{"_usedUp", "_usedDown", "_quota", "_expiry"}

// stripPanelMeta 把面板元数据从 config_json 剥掉，是配置落库的统一收口。
//
// 快路径：没有这些键时不做 JSON 解析（绝大多数配置直接原样返回）。
func stripPanelMeta(configJSON string) string {
	if configJSON == "" {
		return configJSON
	}
	hit := false
	for _, k := range panelMetaKeys {
		if strings.Contains(configJSON, "\""+k+"\"") {
			hit = true
			break
		}
	}
	if !hit {
		return configJSON
	}
	var inbound map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &inbound); err != nil {
		return configJSON
	}
	containers := []map[string]interface{}{inbound}
	if settings, ok := inbound["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}
	changed := false
	for _, c := range containers {
		for _, key := range []string{"users", "clients", "accounts"} {
			items, ok := c[key].([]interface{})
			if !ok {
				continue
			}
			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				for _, k := range panelMetaKeys {
					if _, exists := m[k]; exists {
						delete(m, k)
						changed = true
					}
				}
			}
		}
	}
	if !changed {
		return configJSON
	}
	data, err := json.Marshal(inbound)
	if err != nil {
		return configJSON
	}
	return string(data)
}

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
	if err == nil {
		decryptConfigs(configs, func(c *entity.InboundConfig) *string { return &c.ConfigJSON })
	}
	return configs, err
}

// GetByID retrieves an inbound config by ID.
func (r *InboundConfigRepository) GetByID(ctx context.Context, id int64) (*entity.InboundConfig, error) {
	var cfg entity.InboundConfig
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	cfg.ConfigJSON = DecryptConfigJSON(cfg.ConfigJSON)
	return &cfg, nil
}

// GetByTag retrieves an inbound config by tag on a specific server.
func (r *InboundConfigRepository) GetByTag(ctx context.Context, serverID int64, tag string) (*entity.InboundConfig, error) {
	var cfg entity.InboundConfig
	err := r.db.WithContext(ctx).Where("server_id = ? AND tag = ?", serverID, tag).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	cfg.ConfigJSON = DecryptConfigJSON(cfg.ConfigJSON)
	return &cfg, nil
}

// Create creates a new inbound config.
func (r *InboundConfigRepository) Create(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.ConfigJSON = EncryptConfigJSON(stripPanelMeta(cfg.ConfigJSON))
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(cfg).Error
}

// Update updates an inbound config.
func (r *InboundConfigRepository) Update(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.ConfigJSON = EncryptConfigJSON(stripPanelMeta(cfg.ConfigJSON))
	cfg.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cfg).Error
}

// Upsert creates or updates an inbound config by (server_id, tag).
func (r *InboundConfigRepository) Upsert(ctx context.Context, cfg *entity.InboundConfig) error {
	cfg.ConfigJSON = EncryptConfigJSON(stripPanelMeta(cfg.ConfigJSON))
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
	if err == nil {
		decryptConfigs(configs, func(c *entity.InboundConfig) *string { return &c.ConfigJSON })
	}
	return configs, err
}

// ListByTag retrieves all inbound configs with the given tag across ALL servers.
// This is used for multi-node account synchronization where the same tag
// may exist on both local (serverID=1) and remote (serverID>1) nodes.
func (r *InboundConfigRepository) ListByTag(ctx context.Context, tag string) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	err := r.db.WithContext(ctx).Where("tag = ?", tag).Order("server_id").Find(&configs).Error
	if err == nil {
		decryptConfigs(configs, func(c *entity.InboundConfig) *string { return &c.ConfigJSON })
	}
	return configs, err
}

// ListByServerID retrieves all inbound configs for a specific server.
func (r *InboundConfigRepository) ListByServerID(ctx context.Context, serverID int64) ([]entity.InboundConfig, error) {
	var configs []entity.InboundConfig
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("id").
		Find(&configs).Error
	if err == nil {
		decryptConfigs(configs, func(c *entity.InboundConfig) *string { return &c.ConfigJSON })
	}
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
	if err == nil {
		decryptConfigs(configs, func(c *entity.InboundConfig) *string { return &c.ConfigJSON })
	}
	return configs, err
}
