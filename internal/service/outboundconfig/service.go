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

// List returns all outbound configs for a core type on a specific server.
func (s *Service) List(ctx context.Context, serverID int64, coreType string) ([]entity.OutboundConfig, error) {
	return s.repo.List(ctx, serverID, coreType)
}

// GetByID returns an outbound config by ID.
func (s *Service) GetByID(ctx context.Context, serverID int64, id int64) (*entity.OutboundConfig, error) {
	return s.repo.GetByID(ctx, serverID, id)
}

// Create creates a new outbound config.
func (s *Service) Create(ctx context.Context, serverID int64, coreType, remark, protocol, tag string, configJSON string, enabled bool, isDefault bool) (*entity.OutboundConfig, error) {
	maxOrder, _ := s.repo.GetMaxSortOrder(ctx, serverID, coreType)
	cfg := &entity.OutboundConfig{
		ServerID:   serverID,
		CoreType:   coreType,
		Remark:     remark,
		Protocol:   protocol,
		Tag:        tag,
		ConfigJSON: configJSON,
		Enabled:    enabled,
		IsDefault:  isDefault,
		SortOrder:  maxOrder + 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update updates an outbound config.
func (s *Service) Update(ctx context.Context, serverID int64, id int64, remark, protocol, tag string, configJSON string, enabled bool, isDefault bool) (*entity.OutboundConfig, error) {
	cfg, err := s.repo.GetByID(ctx, serverID, id)
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
func (s *Service) Delete(ctx context.Context, serverID int64, id int64) error {
	return s.repo.Delete(ctx, serverID, id)
}

// GetEnabledOutbounds returns enabled outbound configs as JSON objects for config generation.
func (s *Service) GetEnabledOutbounds(ctx context.Context, serverID int64, coreType string) ([]map[string]interface{}, error) {
	configs, err := s.repo.List(ctx, serverID, coreType)
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
func (s *Service) GetDefaultOutbounds(ctx context.Context, serverID int64, coreType string) ([]entity.OutboundConfig, error) {
	return s.repo.GetDefault(ctx, serverID, coreType)
}

// Reorder updates the sort_order of multiple outbound configs.
// orders is a map of outbound ID → new sort_order value.
func (s *Service) Reorder(ctx context.Context, serverID int64, orders map[int64]int) error {
	return s.repo.Reorder(ctx, serverID, orders)
}
