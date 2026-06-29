package outboundconfig

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles outbound config business logic.
type Service struct {
	repo *repository.OutboundConfigRepository
}

// NewService creates a new outbound config service.
func NewService(repo *repository.OutboundConfigRepository) *Service {
	return &Service{repo: repo}
}

// List returns all outbound configs for a core type.
func (s *Service) List(ctx context.Context, coreType string) ([]entity.OutboundConfig, error) {
	return s.repo.List(ctx, coreType)
}

// GetByID returns an outbound config by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.OutboundConfig, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new outbound config.
func (s *Service) Create(ctx context.Context, coreType, remark, protocol, tag string, configJSON string, enabled bool, isDefault bool) (*entity.OutboundConfig, error) {
	cfg := &entity.OutboundConfig{
		CoreType:   coreType,
		Remark:     remark,
		Protocol:   protocol,
		Tag:        tag,
		ConfigJSON: configJSON,
		Enabled:    enabled,
		IsDefault:  isDefault,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update updates an outbound config.
func (s *Service) Update(ctx context.Context, id int64, remark, protocol, tag string, configJSON string, enabled bool, isDefault bool) (*entity.OutboundConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cfg.Remark = remark
	cfg.Protocol = protocol
	cfg.Tag = tag
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.IsDefault = isDefault
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Delete deletes an outbound config.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// GetEnabledOutbounds returns enabled outbound configs as JSON objects for config generation.
func (s *Service) GetEnabledOutbounds(ctx context.Context, coreType string) ([]map[string]interface{}, error) {
	configs, err := s.repo.List(ctx, coreType)
	if err != nil {
		return nil, err
	}
	var outbounds []map[string]interface{}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		var ob map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &ob); err != nil {
			continue
		}
		normalizeRestrictedOutbound(cfg.Protocol, cfg.Tag, ob)
		// Set tag from DB
		ob["tag"] = cfg.Tag
		ob["protocol"] = cfg.Protocol
		outbounds = append(outbounds, ob)
	}
	return outbounds, nil
}

func normalizeRestrictedOutbound(protocol, tag string, ob map[string]interface{}) {
	if protocol != "wireguard" || rpFilterWritable() {
		return
	}

	settings, _ := ob["settings"].(map[string]interface{})
	if settings == nil {
		settings = make(map[string]interface{})
		ob["settings"] = settings
	}
	if current, ok := settings["noKernelTun"].(bool); ok && current {
		return
	}

	settings["noKernelTun"] = true
	log.Printf("outboundconfig: forced noKernelTun=true for wireguard outbound %q because rp_filter is not writable in current environment", tag)
}

func rpFilterWritable() bool {
	f, err := os.OpenFile("/proc/sys/net/ipv4/conf/all/rp_filter", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// GetDefaultOutbounds returns default outbound configs.
func (s *Service) GetDefaultOutbounds(ctx context.Context, coreType string) ([]entity.OutboundConfig, error) {
	return s.repo.GetDefault(ctx, coreType)
}
