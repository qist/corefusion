package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ExternalNodeSourceRepository handles external_node_sources database operations.
type ExternalNodeSourceRepository struct {
	db *gorm.DB
}

func NewExternalNodeSourceRepository(db *gorm.DB) *ExternalNodeSourceRepository {
	return &ExternalNodeSourceRepository{db: db}
}

func (r *ExternalNodeSourceRepository) List(ctx context.Context) ([]entity.ExternalNodeSource, error) {
	var sources []entity.ExternalNodeSource
	err := r.db.WithContext(ctx).Order("sort_order, id").Find(&sources).Error
	return sources, err
}

func (r *ExternalNodeSourceRepository) GetByID(ctx context.Context, id int64) (*entity.ExternalNodeSource, error) {
	var source entity.ExternalNodeSource
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&source).Error
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *ExternalNodeSourceRepository) Create(ctx context.Context, source *entity.ExternalNodeSource) error {
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(source).Error
}

func (r *ExternalNodeSourceRepository) Update(ctx context.Context, source *entity.ExternalNodeSource) error {
	source.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(source).Error
}

func (r *ExternalNodeSourceRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.ExternalNodeSource{}).Error
}

// ExternalNodeRepository handles external_nodes database operations.
type ExternalNodeRepository struct {
	db *gorm.DB
}

// decryptProxyJSON / encrypt 走整包口径：external_nodes.proxy_json 内含
// uuid / password / obfs-password 等凭据。
func decryptProxyJSON(nodes []entity.ExternalNode) {
	for i := range nodes {
		nodes[i].ProxyJSON = DecryptConfigJSON(nodes[i].ProxyJSON)
	}
}

func NewExternalNodeRepository(db *gorm.DB) *ExternalNodeRepository {
	return &ExternalNodeRepository{db: db}
}

func (r *ExternalNodeRepository) List(ctx context.Context, sourceID int64) ([]entity.ExternalNode, error) {
	var nodes []entity.ExternalNode
	query := r.db.WithContext(ctx)
	if sourceID > 0 {
		query = query.Where("source_id = ?", sourceID)
	}
	err := query.Order("sort_order, id").Find(&nodes).Error
	if err == nil {
		decryptProxyJSON(nodes)
	}
	return nodes, err
}

func (r *ExternalNodeRepository) ListAll(ctx context.Context) ([]entity.ExternalNode, error) {
	var nodes []entity.ExternalNode
	err := r.db.WithContext(ctx).Order("source_id, sort_order, id").Find(&nodes).Error
	if err == nil {
		decryptProxyJSON(nodes)
	}
	return nodes, err
}

func (r *ExternalNodeRepository) GetByID(ctx context.Context, id int64) (*entity.ExternalNode, error) {
	var node entity.ExternalNode
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&node).Error
	if err != nil {
		return nil, err
	}
	node.ProxyJSON = DecryptConfigJSON(node.ProxyJSON)
	return &node, nil
}

func (r *ExternalNodeRepository) Create(ctx context.Context, node *entity.ExternalNode) error {
	node.ProxyJSON = EncryptConfigJSON(node.ProxyJSON)
	node.CreatedAt = time.Now()
	node.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(node).Error
}

func (r *ExternalNodeRepository) CreateBatch(ctx context.Context, nodes []entity.ExternalNode) error {
	if len(nodes) == 0 {
		return nil
	}
	now := time.Now()
	for i := range nodes {
		nodes[i].ProxyJSON = EncryptConfigJSON(nodes[i].ProxyJSON)
		nodes[i].CreatedAt = now
		nodes[i].UpdatedAt = now
	}
	return r.db.WithContext(ctx).Create(&nodes).Error
}

func (r *ExternalNodeRepository) Update(ctx context.Context, node *entity.ExternalNode) error {
	node.ProxyJSON = EncryptConfigJSON(node.ProxyJSON)
	node.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(node).Error
}

func (r *ExternalNodeRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.ExternalNode{}).Error
}

func (r *ExternalNodeRepository) DeleteBySourceID(ctx context.Context, sourceID int64) error {
	return r.db.WithContext(ctx).Where("source_id = ?", sourceID).Delete(&entity.ExternalNode{}).Error
}
