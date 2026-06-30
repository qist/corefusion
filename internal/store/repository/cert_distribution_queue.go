package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// CertDistributionQueueRepository handles cert_distribution_queue database operations.
type CertDistributionQueueRepository struct {
	db *gorm.DB
}

// NewCertDistributionQueueRepository creates a new CertDistributionQueueRepository.
func NewCertDistributionQueueRepository(db *gorm.DB) *CertDistributionQueueRepository {
	return &CertDistributionQueueRepository{db: db}
}

// Create creates a new queue item.
func (r *CertDistributionQueueRepository) Create(ctx context.Context, item *entity.CertDistributionQueueItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// ListPending returns all pending distribution items.
func (r *CertDistributionQueueRepository) ListPending(ctx context.Context) ([]entity.CertDistributionQueueItem, error) {
	var items []entity.CertDistributionQueueItem
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "failed"}).
		Order("created_at ASC").
		Find(&items).Error
	return items, err
}

// ListAll returns all queue items with optional status filter.
func (r *CertDistributionQueueRepository) ListAll(ctx context.Context, status string, limit, offset int) ([]entity.CertDistributionQueueItem, int64, error) {
	query := r.db.WithContext(ctx).Model(&entity.CertDistributionQueueItem{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []entity.CertDistributionQueueItem
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&items).Error
	return items, total, err
}

// UpdateStatus updates the status of a queue item.
func (r *CertDistributionQueueRepository) UpdateStatus(ctx context.Context, id int64, status string, lastError string) error {
	return r.db.WithContext(ctx).Model(&entity.CertDistributionQueueItem{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"last_error": lastError,
			"attempts":   gorm.Expr("attempts + 1"),
			"updated_at": time.Now(),
		}).Error
}

// Delete removes a queue item.
func (r *CertDistributionQueueRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.CertDistributionQueueItem{}, id).Error
}

// DeleteByServer removes all queue items for a server.
func (r *CertDistributionQueueRepository) DeleteByServer(ctx context.Context, serverID int64) error {
	return r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Delete(&entity.CertDistributionQueueItem{}).Error
}
