package endpointconfig

import (
	"context"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service provides business logic for endpoint configs.
type Service struct {
	repo *repository.EndpointConfigRepository
}

// NewService creates a new endpoint config service.
func NewService(repo *repository.EndpointConfigRepository) *Service {
	return &Service{repo: repo}
}

// List returns all endpoint configs for a server and core type.
func (s *Service) List(ctx context.Context, serverID int64, coreType string) ([]entity.EndpointConfig, error) {
	return s.repo.List(ctx, serverID, coreType)
}

// GetByID returns a single endpoint config by ID.
func (s *Service) GetByID(ctx context.Context, id, serverID int64) (*entity.EndpointConfig, error) {
	return s.repo.GetByID(ctx, id, serverID)
}

// Create creates a new endpoint config.
func (s *Service) Create(ctx context.Context, serverID int64, coreType, endpointType, tag, configJSON string, enabled bool) (*entity.EndpointConfig, error) {
	now := time.Now()
	cfg := &entity.EndpointConfig{
		Remark:       tag,
		CoreType:     coreType,
		EndpointType: endpointType,
		Tag:          tag,
		ConfigJSON:   configJSON,
		Enabled:      enabled,
		ServerID:     serverID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update modifies an existing endpoint config.
func (s *Service) Update(ctx context.Context, id, serverID int64, endpointType, tag, configJSON string, enabled bool) (*entity.EndpointConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id, serverID)
	if err != nil {
		return nil, err
	}
	cfg.EndpointType = endpointType
	cfg.Tag = tag
	cfg.Remark = tag
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Delete removes an endpoint config.
func (s *Service) Delete(ctx context.Context, id, serverID int64) error {
	return s.repo.Delete(ctx, id, serverID)
}
