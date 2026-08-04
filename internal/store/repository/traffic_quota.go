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

// ListQuotas returns all traffic quotas, optionally filtered by subject type.
func (r *TrafficQuotaRepository) ListQuotas(ctx context.Context, subjectType string) ([]entity.TrafficQuota, error) {
	var items []entity.TrafficQuota
	q := r.db.WithContext(ctx).Order("id DESC")
	if subjectType != "" {
		q = q.Where("subject_type = ?", subjectType)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetQuota returns a single quota by id.
func (r *TrafficQuotaRepository) GetQuota(ctx context.Context, id int64) (*entity.TrafficQuota, error) {
	var q entity.TrafficQuota
	if err := r.db.WithContext(ctx).First(&q, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &q, nil
}

// CreateQuota inserts a new quota.
func (r *TrafficQuotaRepository) CreateQuota(ctx context.Context, q *entity.TrafficQuota) error {
	return r.db.WithContext(ctx).Create(q).Error
}

// UpdateQuota updates mutable fields of an existing quota.
func (r *TrafficQuotaRepository) UpdateQuota(ctx context.Context, q *entity.TrafficQuota) error {
	return r.db.WithContext(ctx).Model(&entity.TrafficQuota{}).
		Where("id = ?", q.ID).
		Updates(map[string]any{
			"subject_type": q.SubjectType,
			"subject_id":   q.SubjectID,
			"reset_cycle":  q.ResetCycle,
			"quota_bytes":  q.QuotaBytes,
			"hard_limit":   q.HardLimit,
			"grace_bytes":  q.GraceBytes,
		}).Error
}

// DeleteQuota removes a quota by id.
func (r *TrafficQuotaRepository) DeleteQuota(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.TrafficQuota{}, "id = ?", id).Error
}

// ListUsageSummary returns a paginated view of traffic usage summaries.
func (r *TrafficQuotaRepository) ListUsageSummary(ctx context.Context, subjectType string, limit, offset int) ([]entity.TrafficUsageSummary, int64, error) {
	var items []entity.TrafficUsageSummary
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.TrafficUsageSummary{})
	if subjectType != "" {
		q = q.Where("subject_type = ?", subjectType)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("period_start DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListSessions returns a paginated view of traffic sessions.
func (r *TrafficQuotaRepository) ListSessions(ctx context.Context, status string, limit, offset int) ([]entity.TrafficSession, int64, error) {
	var items []entity.TrafficSession
	var total int64
	q := r.db.WithContext(ctx).Model(&entity.TrafficSession{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("started_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
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

// InsertResetLog records a manual quota reset event.
func (r *TrafficQuotaRepository) InsertResetLog(ctx context.Context, log *entity.TrafficQuotaResetLog) error {
	return r.db.WithContext(ctx).Create(log).Error
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

// CurrentPeriodStart computes the start of the current reset cycle for a quota.
// For monthly quotas it returns the first day of the current month.
func CurrentPeriodStart(resetCycle string, now time.Time) time.Time {
	switch resetCycle {
	case "daily":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		// ISO week start (Monday)
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		delta := weekday - 1
		start := now.AddDate(0, 0, -delta)
		return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
	default: // monthly
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
}
