package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"corefusion/internal/model/entity"
)

// InboundUserTrafficRepository 读写入站用户级流量计数（inbound_user_traffic）。
//
// 所有写入都是**原子累加/清零**，不再出现「读整份 config_json → 改 → 回写」的
// 读改写循环；这既是性能优化，也是 config_json 能整体加密的前提。
type InboundUserTrafficRepository struct {
	db *gorm.DB
}

func NewInboundUserTrafficRepository(db *gorm.DB) *InboundUserTrafficRepository {
	return &InboundUserTrafficRepository{db: db}
}

// AddUsage 累加一次流量增量（UPSERT）。
//
// 冲突时用 "used_up = used_up + ?" 这类「常量增量」表达式，而不是引用
// excluded.* / VALUES(*)：前者在 SQLite / MySQL / PostgreSQL 三种方言下都能
// 直接渲染，后者各家写法不同。
func (r *InboundUserTrafficRepository) AddUsage(ctx context.Context, serverID, inboundID int64, email string, up, down int64) error {
	if email == "" || (up == 0 && down == 0) {
		return nil
	}
	row := entity.InboundUserTraffic{
		ServerID:  serverID,
		InboundID: inboundID,
		Email:     email,
		UsedUp:    up,
		UsedDown:  down,
		UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "server_id"}, {Name: "inbound_id"}, {Name: "email"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"used_up":    gorm.Expr("used_up + ?", up),
			"used_down":  gorm.Expr("used_down + ?", down),
			"updated_at": time.Now(),
		}),
	}).Create(&row).Error
}

// ResetForServer 把某节点（serverID<=0 表示全部节点）所有入站用户的流量清零，
// 返回被清零的行数。
func (r *InboundUserTrafficRepository) ResetForServer(ctx context.Context, serverID int64) (int64, error) {
	q := r.db.WithContext(ctx).Model(&entity.InboundUserTraffic{})
	if serverID > 0 {
		q = q.Where("server_id = ?", serverID)
	}
	res := q.Updates(map[string]interface{}{
		"used_up":   0,
		"used_down": 0,
	})
	return res.RowsAffected, res.Error
}

// ResetTarget 描述「把某个入站里的一批 email 的流量清零」。
type ResetTarget struct {
	ServerID  int64
	InboundID int64
	Emails    []string
}

// ResetTargets 按目标批量清零（按 email 精确匹配，语义与旧版
// ResetUserTrafficByEmail 的「精确 email 或 email_ 前缀」一致——调用方先从
// config_json 里解析出匹配到的 email，再交给我们执行）。
func (r *InboundUserTrafficRepository) ResetTargets(ctx context.Context, targets []ResetTarget) (int64, error) {
	var total int64
	for _, t := range targets {
		if len(t.Emails) == 0 {
			continue
		}
		res := r.db.WithContext(ctx).
			Model(&entity.InboundUserTraffic{}).
			Where("server_id = ? AND inbound_id = ? AND email IN ?", t.ServerID, t.InboundID, t.Emails).
			Updates(map[string]interface{}{
				"used_up":   0,
				"used_down": 0,
			})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}

// Prune 删除某入站下不再存在的用户流量行（用户被移除 / 入站被改写后调用）。
// keepEmails 为空表示该入站已无用户，全部删除。
func (r *InboundUserTrafficRepository) Prune(ctx context.Context, serverID, inboundID int64, keepEmails []string) error {
	q := r.db.WithContext(ctx).
		Where("server_id = ? AND inbound_id = ?", serverID, inboundID)
	if len(keepEmails) > 0 {
		q = q.Where("email NOT IN ?", keepEmails)
	}
	return q.Delete(&entity.InboundUserTraffic{}).Error
}

// DeleteByInbound 删除某个入站的全部流量行（入站被删除时调用）。
func (r *InboundUserTrafficRepository) DeleteByInbound(ctx context.Context, inboundID int64) error {
	return r.db.WithContext(ctx).
		Where("inbound_id = ?", inboundID).
		Delete(&entity.InboundUserTraffic{}).Error
}

// MapForInbounds 返回 inboundID → 流量行列表，供读路径把流量注回响应。
func (r *InboundUserTrafficRepository) MapForInbounds(ctx context.Context, inboundIDs []int64) (map[int64][]entity.InboundUserTraffic, error) {
	out := make(map[int64][]entity.InboundUserTraffic, len(inboundIDs))
	if len(inboundIDs) == 0 {
		return out, nil
	}
	var rows []entity.InboundUserTraffic
	if err := r.db.WithContext(ctx).
		Where("inbound_id IN ?", inboundIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.InboundID] = append(out[row.InboundID], row)
	}
	return out, nil
}
