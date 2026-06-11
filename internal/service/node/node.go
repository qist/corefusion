package node

import (
	"context"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles node business logic.
type Service struct {
	repo *repository.NodeRepository
}

// NewService creates a new node service.
func NewService(repo *repository.NodeRepository) *Service {
	return &Service{repo: repo}
}

// CreateNodeInput is the input for creating a node.
type CreateNodeInput struct {
	Name      string `json:"name" binding:"required"`
	CoreType  string `json:"core_type" binding:"required"`
	Protocol  string `json:"protocol" binding:"required"`
	Host      string `json:"host" binding:"required"`
	Port      int    `json:"port" binding:"required"`
	Transport string `json:"transport"`
	TLSMode   string `json:"tls_mode"`
	Region    string `json:"region"`
	Tags      string `json:"tags"`
}

// UpdateNodeInput is the input for updating a node.
type UpdateNodeInput struct {
	Name      *string `json:"name"`
	CoreType  *string `json:"core_type"`
	Protocol  *string `json:"protocol"`
	Host      *string `json:"host"`
	Port      *int    `json:"port"`
	Transport *string `json:"transport"`
	TLSMode   *string `json:"tls_mode"`
	Region    *string `json:"region"`
	Tags      *string `json:"tags"`
	Status    *int    `json:"status"`
}

// Create creates a new node.
func (s *Service) Create(ctx context.Context, input CreateNodeInput) (*entity.Node, error) {
	node := &entity.Node{
		Name:      input.Name,
		CoreType:  input.CoreType,
		Protocol:  input.Protocol,
		Host:      input.Host,
		Port:      input.Port,
		Transport: input.Transport,
		TLSMode:   input.TLSMode,
		Region:    input.Region,
		Tags:      input.Tags,
		Status:    1,
	}

	if err := s.repo.Create(ctx, node); err != nil {
		return nil, err
	}

	return node, nil
}

// GetByID retrieves a node by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.Node, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all nodes with pagination.
func (s *Service) List(ctx context.Context, coreType string, limit, offset int) ([]entity.Node, int64, error) {
	return s.repo.List(ctx, coreType, limit, offset)
}

// Update updates a node.
func (s *Service) Update(ctx context.Context, id int64, input UpdateNodeInput) (*entity.Node, error) {
	node, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		node.Name = *input.Name
	}
	if input.CoreType != nil {
		node.CoreType = *input.CoreType
	}
	if input.Protocol != nil {
		node.Protocol = *input.Protocol
	}
	if input.Host != nil {
		node.Host = *input.Host
	}
	if input.Port != nil {
		node.Port = *input.Port
	}
	if input.Transport != nil {
		node.Transport = *input.Transport
	}
	if input.TLSMode != nil {
		node.TLSMode = *input.TLSMode
	}
	if input.Region != nil {
		node.Region = *input.Region
	}
	if input.Tags != nil {
		node.Tags = *input.Tags
	}
	if input.Status != nil {
		node.Status = *input.Status
	}

	if err := s.repo.Update(ctx, node); err != nil {
		return nil, err
	}

	return node, nil
}

// Delete deletes a node by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
