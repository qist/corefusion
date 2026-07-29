package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// SubscriptionEndpointRepository handles subscription endpoint database operations.
type SubscriptionEndpointRepository struct {
	db *gorm.DB
}

// NewSubscriptionEndpointRepository creates a new subscription endpoint repository.
func NewSubscriptionEndpointRepository(db *gorm.DB) *SubscriptionEndpointRepository {
	return &SubscriptionEndpointRepository{db: db}
}

// List returns all subscription endpoints.
func (r *SubscriptionEndpointRepository) List(ctx context.Context) ([]entity.SubscriptionEndpoint, error) {
	return r.ListPaged(ctx, 0, 0)
}

// ListPaged returns subscription endpoints with pagination (limit<=0 means no limit).
func (r *SubscriptionEndpointRepository) ListPaged(ctx context.Context, limit, offset int) ([]entity.SubscriptionEndpoint, error) {
	var endpoints []entity.SubscriptionEndpoint
	q := r.db.WithContext(ctx).Order("id ASC")
	if limit > 0 {
		q = q.Limit(limit).Offset(offset)
	}
	err := q.Find(&endpoints).Error
	return endpoints, err
}

// Count returns the total number of subscription endpoints.
func (r *SubscriptionEndpointRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.SubscriptionEndpoint{}).Count(&count).Error
	return count, err
}

// GetByID returns a subscription endpoint by ID.
func (r *SubscriptionEndpointRepository) GetByID(ctx context.Context, id int64) (*entity.SubscriptionEndpoint, error) {
	var endpoint entity.SubscriptionEndpoint
	err := r.db.WithContext(ctx).First(&endpoint, id).Error
	return &endpoint, err
}

// Create creates a new subscription endpoint.
func (r *SubscriptionEndpointRepository) Create(ctx context.Context, endpoint *entity.SubscriptionEndpoint) error {
	return r.db.WithContext(ctx).Create(endpoint).Error
}

// Update updates an existing subscription endpoint.
func (r *SubscriptionEndpointRepository) Update(ctx context.Context, endpoint *entity.SubscriptionEndpoint) error {
	return r.db.WithContext(ctx).Save(endpoint).Error
}

// Delete deletes a subscription endpoint by ID.
func (r *SubscriptionEndpointRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.SubscriptionEndpoint{}, id).Error
}

// GetEnabled returns all enabled subscription endpoints.
func (r *SubscriptionEndpointRepository) GetEnabled(ctx context.Context) ([]entity.SubscriptionEndpoint, error) {
	var endpoints []entity.SubscriptionEndpoint
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("id ASC").Find(&endpoints).Error
	return endpoints, err
}
