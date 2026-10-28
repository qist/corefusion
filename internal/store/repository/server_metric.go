package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ServerMetricRepository 读取节点指标（订阅容量判定用）。
type ServerMetricRepository struct {
	db *gorm.DB
}

// NewServerMetricRepository 构造指标仓储。
func NewServerMetricRepository(db *gorm.DB) *ServerMetricRepository {
	return &ServerMetricRepository{db: db}
}

// LatestByServer 返回每个节点最近一条指标。
// 用 MAX(id) 子查询而不是窗口函数，SQLite/MySQL/PostgreSQL 都能跑。
func (r *ServerMetricRepository) LatestByServer(ctx context.Context) (map[int64]entity.ServerMetric, error) {
	var rows []entity.ServerMetric
	err := r.db.WithContext(ctx).
		Where("id IN (SELECT MAX(id) FROM server_metrics GROUP BY server_id)").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]entity.ServerMetric, len(rows))
	for _, m := range rows {
		out[m.ServerID] = m
	}
	return out, nil
}
