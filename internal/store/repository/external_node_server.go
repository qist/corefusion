package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ExternalNodeServerRepository handles external_node_servers database operations.
type ExternalNodeServerRepository struct {
	db *gorm.DB
}

func NewExternalNodeServerRepository(db *gorm.DB) *ExternalNodeServerRepository {
	return &ExternalNodeServerRepository{db: db}
}

// ListAll returns all external node servers.
func (r *ExternalNodeServerRepository) ListAll(ctx context.Context) ([]entity.ExternalNodeServer, error) {
	var servers []entity.ExternalNodeServer
	err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&servers).Error
	return servers, err
}

// ListEnabled returns all enabled external node servers.
func (r *ExternalNodeServerRepository) ListEnabled(ctx context.Context) ([]entity.ExternalNodeServer, error) {
	var servers []entity.ExternalNodeServer
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&servers).Error
	return servers, err
}

// GetByID retrieves an external node server by ID.
func (r *ExternalNodeServerRepository) GetByID(ctx context.Context, id int64) (*entity.ExternalNodeServer, error) {
	var s entity.ExternalNodeServer
	err := r.db.WithContext(ctx).First(&s, id).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetByExternalNodeID retrieves an external node server by its external_node_id.
func (r *ExternalNodeServerRepository) GetByExternalNodeID(ctx context.Context, extNodeID int64) (*entity.ExternalNodeServer, error) {
	var s entity.ExternalNodeServer
	err := r.db.WithContext(ctx).Where("external_node_id = ?", extNodeID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Create creates a new external node server registration.
func (r *ExternalNodeServerRepository) Create(ctx context.Context, s *entity.ExternalNodeServer) error {
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(s).Error
}

// Update updates an external node server.
func (r *ExternalNodeServerRepository) Update(ctx context.Context, s *entity.ExternalNodeServer) error {
	s.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(s).Error
}

// Delete deletes an external node server by ID.
func (r *ExternalNodeServerRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ExternalNodeServer{}, id).Error
}

// DeleteByExternalNodeID deletes an external node server by its external_node_id.
func (r *ExternalNodeServerRepository) DeleteByExternalNodeID(ctx context.Context, extNodeID int64) error {
	return r.db.WithContext(ctx).Where("external_node_id = ?", extNodeID).Delete(&entity.ExternalNodeServer{}).Error
}
