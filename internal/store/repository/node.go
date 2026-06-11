package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// NodeRepository handles node database operations.
type NodeRepository struct {
	db *gorm.DB
}

// NewNodeRepository creates a new NodeRepository.
func NewNodeRepository(db *gorm.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

// Create creates a new node.
func (r *NodeRepository) Create(ctx context.Context, node *entity.Node) error {
	return r.db.WithContext(ctx).Create(node).Error
}

// GetByID retrieves a node by ID.
func (r *NodeRepository) GetByID(ctx context.Context, id int64) (*entity.Node, error) {
	var node entity.Node
	err := r.db.WithContext(ctx).First(&node, id).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// List retrieves all nodes with pagination and optional core_type filter.
func (r *NodeRepository) List(ctx context.Context, coreType string, limit, offset int) ([]entity.Node, int64, error) {
	var nodes []entity.Node
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Node{})
	if coreType != "" {
		query = query.Where("core_type = ?", coreType)
	}

	query.Count(&total)
	err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&nodes).Error
	return nodes, total, err
}

// Update updates a node.
func (r *NodeRepository) Update(ctx context.Context, node *entity.Node) error {
	return r.db.WithContext(ctx).Save(node).Error
}

// Delete deletes a node by ID.
func (r *NodeRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.Node{}, id).Error
}
