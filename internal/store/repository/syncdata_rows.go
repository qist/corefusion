package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CoreConfigRow is a flat representation of a core_configs row for data sync.
type CoreConfigRow struct {
	ID         int64
	ServerID   int64
	CoreType   string
	Section    string
	ConfigJSON string
	UpdatedAt  time.Time
}

// InboundConfigRow is a flat representation of an inbound_configs row for data sync.
type InboundConfigRow struct {
	ID         int64
	Remark     string
	CoreType   string
	Protocol   string
	Tag        string
	Listen     string
	Port       int
	ConfigJSON string
	Enabled    bool
	ServerID   int64
	Region     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OutboundConfigRow is a flat representation of an outbound_configs row for data sync.
type OutboundConfigRow struct {
	ID         int64
	ServerID   int64
	Remark     string
	CoreType   string
	Protocol   string
	Tag        string
	ConfigJSON string
	Enabled    bool
	IsDefault  bool
	SortOrder  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// WarpConfigRow is a flat representation of a warp_configs row for data sync.
type WarpConfigRow = entity.WarpConfig

// ShareDomainConfigRow is a flat representation of a share_domain_configs row for data sync.
type ShareDomainConfigRow = entity.ShareDomainConfig

// ListRows returns all core_config rows for a server (all core types).
func (r *CoreConfigRepository) ListRows(ctx context.Context, serverID int64) ([]CoreConfigRow, error) {
	var rows []CoreConfigRow
	err := r.db.WithContext(ctx).
		Model(&entity.CoreConfig{}).
		Where("server_id = ?", serverID).
		Order("core_type, section").
		Find(&rows).Error
	return rows, err
}

// ListRows returns all inbound_config rows for a server.
func (r *InboundConfigRepository) ListRows(ctx context.Context, serverID int64) ([]InboundConfigRow, error) {
	var rows []InboundConfigRow
	err := r.db.WithContext(ctx).
		Model(&entity.InboundConfig{}).
		Where("server_id = ?", serverID).
		Order("id").
		Find(&rows).Error
	return rows, err
}

// ListRows returns all outbound_config rows for a server.
func (r *OutboundConfigRepository) ListRows(ctx context.Context, serverID int64) ([]OutboundConfigRow, error) {
	var rows []OutboundConfigRow
	err := r.db.WithContext(ctx).
		Model(&entity.OutboundConfig{}).
		Where("server_id = ?", serverID).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	return rows, err
}

// ListRows returns all warp config rows for a specific server.
func (r *WarpRepository) ListRows(ctx context.Context, serverID int64) ([]WarpConfigRow, error) {
	var rows []WarpConfigRow
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND status != ?", serverID, "not_registered").
		Order("id DESC").
		Find(&rows).Error
	return rows, err
}

// ListRows returns effective share domain config rows for a server.
// This includes global (server_id=0), server-specific (server_id=serverID)
// and multi-node bindings (server_ids JSON containing serverID).
func (r *ShareDomainConfigRepository) ListRows(ctx context.Context, serverID int64) ([]ShareDomainConfigRow, error) {
	var rows []ShareDomainConfigRow
	err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("server_id DESC, sort_order ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	// 多节点绑定（server_ids JSON）统一内存过滤（跨 SQLite/MySQL 兼容）
	out := make([]ShareDomainConfigRow, 0, len(rows))
	for _, row := range rows {
		if row.MatchServer(serverID) {
			out = append(out, row)
		}
	}
	return out, nil
}

// Ensure UnusedDB is available for type compatibility.
var _ *gorm.DB
