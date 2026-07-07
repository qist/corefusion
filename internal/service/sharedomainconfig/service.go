package sharedomainconfig

import (
	"context"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles share domain config business logic.
type Service struct {
	repo *repository.ShareDomainConfigRepository
}

// NewService creates a new share domain config service.
func NewService(repo *repository.ShareDomainConfigRepository) *Service {
	return &Service{repo: repo}
}

// List returns all share domain configs, optionally filtered by type.
func (s *Service) List(ctx context.Context, configType string) ([]entity.ShareDomainConfig, error) {
	return s.repo.List(ctx, configType)
}

// ListByServer returns direct share domain configs for a specific server.
// CF domains are global (server_id=0) and not included here.
// ListByServer returns direct share domain configs effective for a specific server.
// This includes both global (server_id=0) and server-specific (server_id=serverID) direct domains.
// CF domains are global and not included here.
func (s *Service) ListByServer(ctx context.Context, serverID int64) ([]entity.ShareDomainConfig, error) {
	// Get effective configs: global + server-specific
	all, err := s.repo.ListEffectiveForServer(ctx, serverID, "")
	if err != nil {
		return nil, err
	}
	var result []entity.ShareDomainConfig
	for _, d := range all {
		if d.Type == "direct" {
			result = append(result, d)
		}
	}
	return result, nil
}

// GetByID returns a share domain config by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.ShareDomainConfig, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new share domain config for a specific server.
func (s *Service) Create(ctx context.Context, serverID int64, configType, value string, port int, ports string) (*entity.ShareDomainConfig, error) {
	if port <= 0 {
		port = 443
	}
	if ports == "" {
		ports = "[443]"
	}
	config := &entity.ShareDomainConfig{
		ServerID: serverID,
		Type:     configType,
		Value:    value,
		Port:     port,
		Ports:    ports,
		Enabled:  true,
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	if err := s.repo.Create(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

// Update updates an existing share domain config.
func (s *Service) Update(ctx context.Context, id int64, configType, value string, port int, ports string, enabled bool) (*entity.ShareDomainConfig, error) {
	config, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if configType != "" {
		config.Type = configType
	}
	if value != "" {
		config.Value = value
	}
	if port > 0 {
		config.Port = port
	}
	if ports != "" {
		config.Ports = ports
	}
	config.Enabled = enabled
	config.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

// Save saves a share domain config directly (used for server_id binding updates).
func (s *Service) Save(ctx context.Context, config *entity.ShareDomainConfig) error {
	return s.repo.Update(ctx, config)
}

// Delete deletes a share domain config by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// ClearAll deletes all share domain configs, optionally filtered by type.
func (s *Service) ClearAll(ctx context.Context, configType string) error {
	return s.repo.ClearAll(ctx, configType)
}

// ClearByServer deletes direct share domain configs for a specific server.
func (s *Service) ClearByServer(ctx context.Context, serverID int64, configType string) error {
	return s.repo.ClearByServer(ctx, serverID, configType)
}

// SeedDefaults seeds default CF domain configs globally (server_id=0).
func (s *Service) SeedDefaults(ctx context.Context) error {
	// Clear existing cf_domain globally first
	if err := s.repo.ClearAll(ctx, "cf_domain"); err != nil {
		return err
	}

	defaultPorts := "[443]"
	defaults := []entity.ShareDomainConfig{
		{Type: "cf_domain", Value: "mfa.gov.ua", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 1},
		{Type: "cf_domain", Value: "www.shopify.com", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 2},
		{Type: "cf_domain", Value: "www.visa.cn", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 3},
		{Type: "cf_domain", Value: "time.is", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 4},
		{Type: "cf_domain", Value: "icook.hk", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 5},
		{Type: "cf_domain", Value: "icook.tw", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 6},
		{Type: "cf_domain", Value: "cloudflare.182682.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 7},
		{Type: "cf_domain", Value: "tencentapp.cn", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 8},
		{Type: "cf_domain", Value: "cf.877774.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 9},
		{Type: "cf_domain", Value: "cfip.xxxxxxxx.tk", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 10},
		{Type: "cf_domain", Value: "bestcf.030101.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 11},
		{Type: "cf_domain", Value: "777.ai7777777.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 12},
		{Type: "cf_domain", Value: "cdn.2020111.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 13},
		{Type: "cf_domain", Value: "cfip.cfcdn.vip", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 14},
		{Type: "cf_domain", Value: "cf.0sm.com", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 15},
		{Type: "cf_domain", Value: "cfip.1323123.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 16},
		{Type: "cf_domain", Value: "cloudflare-ip.mofashi.ltd", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 17},
		{Type: "cf_domain", Value: "cf.877771.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 18},
		{Type: "cf_domain", Value: "cf.900501.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 19},
		{Type: "cf_domain", Value: "cdns.doon.eu.org", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 20},
		{Type: "cf_domain", Value: "fn.130519.xyz", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 21},
		{Type: "cf_domain", Value: "nrt.xxxxxxxx.nyc.mn", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 22},
		{Type: "cf_domain", Value: "nrtcfdns.zone.id", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 23},
		{Type: "cf_domain", Value: "saas.sin.fan", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 24},
		{Type: "cf_domain", Value: "xn--b6gac.eu.org", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 25},
		{Type: "cf_domain", Value: "freeyx.cloudflare88.eu.org", Port: 443, Ports: defaultPorts, Enabled: true, SortOrder: 26},
	}
	for _, d := range defaults {
		d.CreatedAt = time.Now()
		d.UpdatedAt = time.Now()
		if err := s.repo.Create(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}
