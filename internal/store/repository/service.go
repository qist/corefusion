package repository

import (
	"context"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// ServiceRepository handles inbound service database operations.
type ServiceRepository struct {
	db *gorm.DB
}

// NewServiceRepository creates a new ServiceRepository.
func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	return &ServiceRepository{db: db}
}

// Create creates a new inbound service.
func (r *ServiceRepository) Create(ctx context.Context, svc *entity.InboundService) error {
	return r.db.WithContext(ctx).Create(svc).Error
}

// GetByID retrieves an inbound service by ID.
func (r *ServiceRepository) GetByID(ctx context.Context, id int64) (*entity.InboundService, error) {
	var svc entity.InboundService
	err := r.db.WithContext(ctx).First(&svc, id).Error
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

// List retrieves inbound services for a specific node server with pagination and optional core_type filter.
func (r *ServiceRepository) List(ctx context.Context, serverID int64, coreType string, limit, offset int) ([]entity.InboundService, int64, error) {
	var services []entity.InboundService
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.InboundService{}).Where("server_id = ?", serverID)
	if coreType != "" {
		query = query.Where("core_type = ?", coreType)
	}

	query.Count(&total)
	err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&services).Error
	return services, total, err
}

// Update updates an inbound service.
func (r *ServiceRepository) Update(ctx context.Context, svc *entity.InboundService) error {
	return r.db.WithContext(ctx).Save(svc).Error
}

// Delete deletes an inbound service by ID.
func (r *ServiceRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.InboundService{}, id).Error
}

// EndpointRepository handles service endpoint database operations.
type EndpointRepository struct {
	db *gorm.DB
}

// NewEndpointRepository creates a new EndpointRepository.
func NewEndpointRepository(db *gorm.DB) *EndpointRepository {
	return &EndpointRepository{db: db}
}

// Create creates a new endpoint.
func (r *EndpointRepository) Create(ctx context.Context, ep *entity.ServiceEndpoint) error {
	return r.db.WithContext(ctx).Create(ep).Error
}

// ListByServiceID retrieves all endpoints for a service.
func (r *EndpointRepository) ListByServiceID(ctx context.Context, serviceID int64) ([]entity.ServiceEndpoint, error) {
	var endpoints []entity.ServiceEndpoint
	err := r.db.WithContext(ctx).Where("service_id = ?", serviceID).Order("priority ASC, id ASC").Find(&endpoints).Error
	return endpoints, err
}

// GetByID retrieves an endpoint by ID.
func (r *EndpointRepository) GetByID(ctx context.Context, id int64) (*entity.ServiceEndpoint, error) {
	var ep entity.ServiceEndpoint
	err := r.db.WithContext(ctx).First(&ep, id).Error
	if err != nil {
		return nil, err
	}
	return &ep, nil
}

// Update updates an endpoint.
func (r *EndpointRepository) Update(ctx context.Context, ep *entity.ServiceEndpoint) error {
	return r.db.WithContext(ctx).Save(ep).Error
}

// Delete deletes an endpoint by ID.
func (r *EndpointRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&entity.ServiceEndpoint{}, id).Error
}
