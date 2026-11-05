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

// GetByID retrieves a node by ID.
func (r *NodeRepository) GetByID(ctx context.Context, id int64) (*entity.Node, error) {
	var node entity.Node
	err := r.db.WithContext(ctx).First(&node, id).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}
