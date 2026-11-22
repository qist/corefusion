package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ChainLinkRepository handles chain_links database operations.
type ChainLinkRepository struct {
	db *gorm.DB
}

func NewChainLinkRepository(db *gorm.DB) *ChainLinkRepository {
	return &ChainLinkRepository{db: db}
}

// ListByEntry returns all chain links for a given entry (server + optional inbound).
// If inboundID is 0, returns all links for the server (including global ones with entry_inbound_id=0).
// If inboundID > 0, returns both global links (entry_inbound_id=0) and inbound-specific links.
func (r *ChainLinkRepository) ListByEntry(ctx context.Context, serverID int64, inboundID int64) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	q := r.db.WithContext(ctx).Where("entry_server_id = ?", serverID)
	if inboundID > 0 {
		q = q.Where("entry_inbound_id = 0 OR entry_inbound_id = ?", inboundID)
	}
	err := q.Where("enabled = ?", true).
		Order("priority ASC, sort_order ASC, id ASC").Find(&links).Error
	return links, err
}

// ListAllByEntry returns all chain links for a given entry server (including disabled).
func (r *ChainLinkRepository) ListAllByEntry(ctx context.Context, serverID int64) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	err := r.db.WithContext(ctx).
		Where("entry_server_id = ?", serverID).
		Order("priority ASC, sort_order ASC, id ASC").Find(&links).Error
	return links, err
}

// ListByExit returns all chain links pointing to a given exit server.
func (r *ChainLinkRepository) ListByExit(ctx context.Context, serverID int64) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	err := r.db.WithContext(ctx).
		Where("exit_server_id = ?", serverID).
		Order("priority ASC, id ASC").Find(&links).Error
	return links, err
}

// ListAll returns all chain links.
func (r *ChainLinkRepository) ListAll(ctx context.Context) ([]entity.ChainLink, error) {
	var links []entity.ChainLink
	err := r.db.WithContext(ctx).
		Order("priority ASC, sort_order ASC, id ASC").Find(&links).Error
	return links, err
}

// GetByID retrieves a chain link by ID.
func (r *ChainLinkRepository) GetByID(ctx context.Context, id int64) (*entity.ChainLink, error) {
	var link entity.ChainLink
	err := r.db.WithContext(ctx).First(&link, id).Error
	if err != nil {
		return nil, err
	}
	return &link, nil
}

// Create creates a new chain link.
func (r *ChainLinkRepository) Create(ctx context.Context, link *entity.ChainLink) error {
	link.CreatedAt = time.Now()
	link.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(link).Error
}

// Update updates a chain link.
func (r *ChainLinkRepository) Update(ctx context.Context, link *entity.ChainLink) error {
	link.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(link).Error
}

// Delete deletes a chain link.
func (r *ChainLinkRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ChainLink{}, id).Error
}

// DeleteByEntry deletes all chain links for an entry server.
// DeleteByEntryInbound 清掉某个"入口入口（server + inbound）"下的全部链。
//
// ⚠ 粒度必须到 **inbound**，不能只按 server ✗：同一台机器可以有多个入口入站，每个入站有自己
// 的一组出口（设计文档 §1183 "按入口隔离"）。只按 server 清的话，给入站 A 建链会**静默清掉**
// 入站 B 的链 → B 的流量落到 `route.final` 直连出去 ✗（真机踩到：本机 xray 入口 4 的链在给
// sing-box 入口 118 建链时消失 ✓）。读取侧的 ListByEntry 也是按 server + inbound 查询的，保持一致。
// `entry_inbound_id = 0` 是历史数据（早期没有入口入站概念），一并纳入清除。
func (r *ChainLinkRepository) DeleteByEntryInbound(ctx context.Context, serverID int64, inboundID int64) error {
	return r.db.WithContext(ctx).
		Where("entry_server_id = ?", serverID).
		Where("entry_inbound_id = 0 OR entry_inbound_id = ?", inboundID).
		Delete(&entity.ChainLink{}).Error
}

// ReplaceByEntryInbound 在一个事务里“先清后建”某个入口入站的全部链路。
// 旧实现是 handler 里先 Delete 再逐条 Create：中途失败会留下“已删但没建”的状态，
// 该入口所有链路直接消失（流量直连）。
func (r *ChainLinkRepository) ReplaceByEntryInbound(ctx context.Context, serverID, inboundID int64, links []entity.ChainLink) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("entry_server_id = ?", serverID).
			Where("entry_inbound_id = 0 OR entry_inbound_id = ?", inboundID).
			Delete(&entity.ChainLink{}).Error; err != nil {
			return err
		}
		now := time.Now()
		for i := range links {
			links[i].CreatedAt = now
			links[i].UpdatedAt = now
			if err := tx.Create(&links[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteByExit deletes all chain links pointing to an exit server.
func (r *ChainLinkRepository) DeleteByExit(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("exit_server_id = ?", serverID).
		Delete(&entity.ChainLink{}).Error
}
