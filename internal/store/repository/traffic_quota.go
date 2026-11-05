package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// TrafficQuotaRepository handles traffic quota and usage data operations.
type TrafficQuotaRepository struct {
	db *gorm.DB
}

// NewTrafficQuotaRepository creates a new TrafficQuotaRepository.
func NewTrafficQuotaRepository(db *gorm.DB) *TrafficQuotaRepository {
	return &TrafficQuotaRepository{db: db}
}

// ListResetLogs returns a paginated view of quota reset logs.
func (r *TrafficQuotaRepository) ListResetLogs(ctx context.Context, subjectType string, limit, offset int) ([]entity.TrafficQuotaResetLog, int64, error) {
	var items []entity.TrafficQuotaResetLog
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.TrafficQuotaResetLog{})
	if subjectType != "" {
		q = q.Where("subject_type = ?", subjectType)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListTrafficUsage returns raw traffic usage records from node_traffic_usage
// (the table the quota engine actually writes to). Unlike the quota service's
// history helpers, this does NOT filter by interface, so it surfaces all
// recorded accounting rows (e.g. "eth0", "local") for monitoring.
func (r *TrafficQuotaRepository) ListTrafficUsage(ctx context.Context, serverIDs []int64, months int) ([]entity.NodeTrafficUsage, error) {
	if months <= 0 {
		months = 6
	}
	cutoff := time.Now().AddDate(0, -months, 0)
	var items []entity.NodeTrafficUsage
	q := r.db.WithContext(ctx).Model(&entity.NodeTrafficUsage{}).
		Where("period_start >= ?", cutoff)
	if len(serverIDs) > 0 {
		q = q.Where("server_id IN ?", serverIDs)
	}
	if err := q.Order("period_start DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetCurrentTrafficUsage returns the node_traffic_usage row for a server's
// latest period (no interface filter), used by the monitoring view.
func (r *TrafficQuotaRepository) GetCurrentTrafficUsage(ctx context.Context, serverID int64) (*entity.NodeTrafficUsage, error) {
	var u entity.NodeTrafficUsage
	if err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("period_start DESC").
		First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
