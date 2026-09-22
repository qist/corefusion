package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// NodeCertificateRepository 维护"节点 ↔ 证书"的多对多绑定 ✓。
//
// 放在 NodeServerRepository 之外、但共用同一个 DB ✓：
// 既保持职责单一 ✓，也避免改动各处已有的构造与接线 ✗。
type NodeCertificateRepository struct {
	db *gorm.DB
}

// NewNodeCertificateRepository 创建仓库。
func NewNodeCertificateRepository(db *gorm.DB) *NodeCertificateRepository {
	return &NodeCertificateRepository{db: db}
}

// Associate 建立绑定（幂等 ✓：重复绑不会报错也不会重复插入 ✓）。
func (r *NodeCertificateRepository) Associate(ctx context.Context, nodeServerID, certificateID int64) error {
	if nodeServerID <= 0 || certificateID <= 0 {
		return nil
	}
	var exist int64
	if err := r.db.WithContext(ctx).Model(&entity.NodeCertificate{}).
		Where("node_server_id = ? AND certificate_id = ?", nodeServerID, certificateID).
		Count(&exist).Error; err == nil && exist > 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&entity.NodeCertificate{
		NodeServerID:  nodeServerID,
		CertificateID: certificateID,
		CreatedAt:     time.Now(),
	}).Error
}

// Dissociate 解除绑定（幂等 ✓）。
func (r *NodeCertificateRepository) Dissociate(ctx context.Context, nodeServerID, certificateID int64) error {
	q := r.db.WithContext(ctx).Where("node_server_id = ?", nodeServerID)
	if certificateID > 0 {
		q = q.Where("certificate_id = ?", certificateID)
	}
	return q.Delete(&entity.NodeCertificate{}).Error
}

// ListCertificateIDsByNode 返回该节点绑定的**全部**证书 ID ✓（去重 ✓，稳定顺序 ✓）。
func (r *NodeCertificateRepository) ListCertificateIDsByNode(ctx context.Context, nodeServerID int64) ([]int64, error) {
	var rows []entity.NodeCertificate
	if err := r.db.WithContext(ctx).
		Where("node_server_id = ?", nodeServerID).
		Order("certificate_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	seen := map[int64]bool{}
	for _, row := range rows {
		if row.CertificateID > 0 && !seen[row.CertificateID] {
			seen[row.CertificateID] = true
			out = append(out, row.CertificateID)
		}
	}
	return out, nil
}

// ListNodeIDsByCertificate 返回**绑定了这张证书的全部节点 ID** ✓。
// 公网证书的自动续期靠它找"该让哪些节点去续" ✓（一个证书可能被多个节点共用 ✓）。
func (r *NodeCertificateRepository) ListNodeIDsByCertificate(ctx context.Context, certificateID int64) ([]int64, error) {
	var rows []entity.NodeCertificate
	if err := r.db.WithContext(ctx).
		Where("certificate_id = ?", certificateID).
		Order("node_server_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	seen := map[int64]bool{}
	for _, row := range rows {
		if row.NodeServerID > 0 && !seen[row.NodeServerID] {
			seen[row.NodeServerID] = true
			out = append(out, row.NodeServerID)
		}
	}
	return out, nil
}

// MigrateLegacyCertificateID 把历史上的**单个** `node_servers.certificate_id` 搬进多对多表 ✓。
//
// 幂等 ✓：已存在的绑定会被 Associate 跳过 ✓，所以每次启动都可以安全跑 ✓。
func (r *NodeCertificateRepository) MigrateLegacyCertificateID(ctx context.Context) (int, error) {
	type legacy struct {
		ID            int64
		CertificateID int64
	}
	var rows []legacy
	if err := r.db.WithContext(ctx).Model(&entity.NodeServer{}).
		Select("id, certificate_id").
		Where("certificate_id > 0").Scan(&rows).Error; err != nil {
		return 0, err
	}
	n := 0
	for _, row := range rows {
		var exist int64
		if err := r.db.WithContext(ctx).Model(&entity.NodeCertificate{}).
			Where("node_server_id = ? AND certificate_id = ?", row.ID, row.CertificateID).
			Count(&exist).Error; err != nil {
			return n, err
		}
		if exist > 0 {
			continue
		}
		if err := r.Associate(ctx, row.ID, row.CertificateID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
