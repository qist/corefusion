package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ServerTagRepository handles server_tags database operations.
type ServerTagRepository struct {
	db *gorm.DB
}

// NewServerTagRepository creates a new ServerTagRepository.
func NewServerTagRepository(db *gorm.DB) *ServerTagRepository {
	return &ServerTagRepository{db: db}
}

// ListByServer returns all tags for a specific server.
func (r *ServerTagRepository) ListByServer(ctx context.Context, serverID int64) ([]entity.ServerTag, error) {
	var tags []entity.ServerTag
	err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("tag_key ASC").
		Find(&tags).Error
	return tags, err
}

// Create creates a new tag. If the tag key already exists, it updates the value.
func (r *ServerTagRepository) Create(ctx context.Context, tag *entity.ServerTag) error {
	return r.db.WithContext(ctx).Save(tag).Error
}

// Delete removes a tag by server ID and tag key.
func (r *ServerTagRepository) Delete(ctx context.Context, serverID int64, tagKey string) error {
	return r.db.WithContext(ctx).
		Where("server_id = ? AND tag_key = ?", serverID, tagKey).
		Delete(&entity.ServerTag{}).Error
}

// FindServersByTag returns server IDs that have the given tag key/value.
func (r *ServerTagRepository) FindServersByTag(ctx context.Context, tagKey, tagValue string) ([]int64, error) {
	var ids []int64
	query := r.db.WithContext(ctx).
		Model(&entity.ServerTag{}).
		Where("tag_key = ?", tagKey)
	if tagValue != "" {
		query = query.Where("tag_value = ?", tagValue)
	}
	err := query.Distinct("server_id").Pluck("server_id", &ids).Error
	return ids, err
}

// DeleteByServer removes all tags for a server (used when deleting a server).
func (r *ServerTagRepository) DeleteByServer(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Delete(&entity.ServerTag{}).Error
}
