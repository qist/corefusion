package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// NodeAuditLogRepository handles node_audit_logs database operations.
type NodeAuditLogRepository struct {
	db *gorm.DB
}

// NewNodeAuditLogRepository creates a new NodeAuditLogRepository.
func NewNodeAuditLogRepository(db *gorm.DB) *NodeAuditLogRepository {
	return &NodeAuditLogRepository{db: db}
}

// Create creates a new audit log entry.
func (r *NodeAuditLogRepository) Create(ctx context.Context, log *entity.NodeAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// List returns audit logs with optional filtering by server_id and action.
func (r *NodeAuditLogRepository) List(ctx context.Context, serverID int64, action string, limit, offset int) ([]entity.NodeAuditLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.NodeAuditLog{})
	if serverID > 0 {
		query = query.Where("server_id = ?", serverID)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []entity.NodeAuditLog
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error
	return logs, total, err
}

// ListByTimeRange returns audit logs within a time range, limited to prevent
// unbounded memory consumption under high audit traffic.
func (r *NodeAuditLogRepository) ListByTimeRange(ctx context.Context, from, to time.Time, limit int) ([]entity.NodeAuditLog, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	var logs []entity.NodeAuditLog
	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", from, to).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
