package service

import (
	"context"
	"encoding/json"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles inbound service business logic.
type Service struct {
	repo       *repository.ServiceRepository
	endpointRepo *repository.EndpointRepository
}

// NewService creates a new inbound service service.
func NewService(repo *repository.ServiceRepository, endpointRepo *repository.EndpointRepository) *Service {
	return &Service{repo: repo, endpointRepo: endpointRepo}
}

// CreateServiceInput is the input for creating an inbound service.
type CreateServiceInput struct {
	ServerID   int64  `json:"server_id"`
	Name       string `json:"name" binding:"required"`
	CoreType   string `json:"core_type" binding:"required"`
	Protocol   string `json:"protocol" binding:"required"`
	ListenHost string `json:"listen_host" binding:"required"`
	ListenPort int    `json:"listen_port" binding:"required"`
	Transport  string `json:"transport"`
	TLSEnabled bool   `json:"tls_enabled"`
	ConfigJSON string `json:"config_json"`
}

// UpdateServiceInput is the input for updating an inbound service.
type UpdateServiceInput struct {
	Name       *string `json:"name"`
	CoreType   *string `json:"core_type"`
	Protocol   *string `json:"protocol"`
	ListenHost *string `json:"listen_host"`
	ListenPort *int    `json:"listen_port"`
	Transport  *string `json:"transport"`
	TLSEnabled *bool   `json:"tls_enabled"`
	ConfigJSON *string `json:"config_json"`
}

// Create creates a new inbound service.
func (s *Service) Create(ctx context.Context, input CreateServiceInput) (*entity.InboundService, error) {
	tlsEnabled := 0
	if input.TLSEnabled {
		tlsEnabled = 1
	}

	configJSON := input.ConfigJSON
	if configJSON == "" {
		configJSON = "{}"
	}

	serverID := input.ServerID
	if serverID == 0 {
		serverID = 1 // default to local node
	}

	svc := &entity.InboundService{
		ServerID:   serverID,
		Name:       input.Name,
		CoreType:   input.CoreType,
		Protocol:   input.Protocol,
		ListenHost: input.ListenHost,
		ListenPort: input.ListenPort,
		Transport:  input.Transport,
		TLSEnabled: tlsEnabled,
		ConfigJSON: configJSON,
	}

	if err := s.repo.Create(ctx, svc); err != nil {
		return nil, err
	}

	return svc, nil
}

// GetByID retrieves an inbound service by ID with its endpoints.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.InboundService, []entity.ServiceEndpoint, error) {
	svc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	endpoints, err := s.endpointRepo.ListByServiceID(ctx, id)
	if err != nil {
		return svc, nil, err
	}

	return svc, endpoints, nil
}

// List retrieves inbound services for a specific node server with pagination.
func (s *Service) List(ctx context.Context, serverID int64, coreType string, limit, offset int) ([]entity.InboundService, int64, error) {
	return s.repo.List(ctx, serverID, coreType, limit, offset)
}

// Update updates an inbound service.
func (s *Service) Update(ctx context.Context, id int64, input UpdateServiceInput) (*entity.InboundService, error) {
	svc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		svc.Name = *input.Name
	}
	if input.CoreType != nil {
		svc.CoreType = *input.CoreType
	}
	if input.Protocol != nil {
		svc.Protocol = *input.Protocol
	}
	if input.ListenHost != nil {
		svc.ListenHost = *input.ListenHost
	}
	if input.ListenPort != nil {
		svc.ListenPort = *input.ListenPort
	}
	if input.Transport != nil {
		svc.Transport = *input.Transport
	}
	if input.TLSEnabled != nil {
		if *input.TLSEnabled {
			svc.TLSEnabled = 1
		} else {
			svc.TLSEnabled = 0
		}
	}
	if input.ConfigJSON != nil {
		svc.ConfigJSON = *input.ConfigJSON
	}

	if err := s.repo.Update(ctx, svc); err != nil {
		return nil, err
	}

	return svc, nil
}

// Delete deletes an inbound service by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// CreateEndpointInput is the input for creating an endpoint.
type CreateEndpointInput struct {
	EndpointType  string `json:"endpoint_type" binding:"required"`
	Address       string `json:"address" binding:"required"`
	ServerName    string `json:"server_name"`
	ListenHost    string `json:"listen_host"`
	ListenPort    int    `json:"listen_port"`
	Transport     string `json:"transport"`
	TLSEnabled    bool   `json:"tls_enabled"`
	CertificateID *int64 `json:"certificate_id"`
	IsDefault     bool   `json:"is_default"`
	Priority      int    `json:"priority"`
}

// CreateEndpoint creates a new endpoint for a service.
func (s *Service) CreateEndpoint(ctx context.Context, serviceID int64, input CreateEndpointInput) (*entity.ServiceEndpoint, error) {
	tlsEnabled := 0
	if input.TLSEnabled {
		tlsEnabled = 1
	}
	isDefault := 0
	if input.IsDefault {
		isDefault = 1
	}
	priority := input.Priority
	if priority == 0 {
		priority = 100
	}

	ep := &entity.ServiceEndpoint{
		ServiceID:     serviceID,
		EndpointType:  input.EndpointType,
		Address:       input.Address,
		ServerName:    input.ServerName,
		ListenHost:    input.ListenHost,
		ListenPort:    input.ListenPort,
		Transport:     input.Transport,
		TLSEnabled:    tlsEnabled,
		CertificateID: input.CertificateID,
		IsDefault:     isDefault,
		Priority:      priority,
		Status:        1,
	}

	if err := s.endpointRepo.Create(ctx, ep); err != nil {
		return nil, err
	}

	return ep, nil
}

// ListEndpoints lists all endpoints for a service.
func (s *Service) ListEndpoints(ctx context.Context, serviceID int64) ([]entity.ServiceEndpoint, error) {
	return s.endpointRepo.ListByServiceID(ctx, serviceID)
}

// UpdateEndpointInput is the input for updating an endpoint.
type UpdateEndpointInput struct {
	EndpointType  *string `json:"endpoint_type"`
	Address       *string `json:"address"`
	ServerName    *string `json:"server_name"`
	ListenHost    *string `json:"listen_host"`
	ListenPort    *int    `json:"listen_port"`
	Transport     *string `json:"transport"`
	TLSEnabled    *bool   `json:"tls_enabled"`
	CertificateID *int64  `json:"certificate_id"`
	IsDefault     *bool   `json:"is_default"`
	Priority      *int    `json:"priority"`
	Status        *int    `json:"status"`
}

// UpdateEndpoint updates an endpoint.
func (s *Service) UpdateEndpoint(ctx context.Context, endpointID int64, input UpdateEndpointInput) (*entity.ServiceEndpoint, error) {
	ep, err := s.endpointRepo.GetByID(ctx, endpointID)
	if err != nil {
		return nil, err
	}

	if input.EndpointType != nil {
		ep.EndpointType = *input.EndpointType
	}
	if input.Address != nil {
		ep.Address = *input.Address
	}
	if input.ServerName != nil {
		ep.ServerName = *input.ServerName
	}
	if input.ListenHost != nil {
		ep.ListenHost = *input.ListenHost
	}
	if input.ListenPort != nil {
		ep.ListenPort = *input.ListenPort
	}
	if input.Transport != nil {
		ep.Transport = *input.Transport
	}
	if input.TLSEnabled != nil {
		if *input.TLSEnabled {
			ep.TLSEnabled = 1
		} else {
			ep.TLSEnabled = 0
		}
	}
	if input.CertificateID != nil {
		ep.CertificateID = input.CertificateID
	}
	if input.IsDefault != nil {
		if *input.IsDefault {
			ep.IsDefault = 1
		} else {
			ep.IsDefault = 0
		}
	}
	if input.Priority != nil {
		ep.Priority = *input.Priority
	}
	if input.Status != nil {
		ep.Status = *input.Status
	}

	if err := s.endpointRepo.Update(ctx, ep); err != nil {
		return nil, err
	}

	return ep, nil
}

// DeleteEndpoint deletes an endpoint.
func (s *Service) DeleteEndpoint(ctx context.Context, endpointID int64) error {
	return s.endpointRepo.Delete(ctx, endpointID)
}

// ServiceToJSON converts a service to a JSON-friendly map.
func ServiceToJSON(svc *entity.InboundService) map[string]interface{} {
	result := map[string]interface{}{
		"id":             svc.ID,
		"server_id":      svc.ServerID,
		"name":           svc.Name,
		"core_type":      svc.CoreType,
		"protocol":       svc.Protocol,
		"listen_host":    svc.ListenHost,
		"listen_port":    svc.ListenPort,
		"transport":      svc.Transport,
		"tls_enabled":    svc.TLSEnabled == 1,
		"process_status": svc.ProcessStatus,
		"created_at":     svc.CreatedAt,
		"updated_at":     svc.UpdatedAt,
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(svc.ConfigJSON), &config); err == nil {
		result["config"] = config
	}

	return result
}
