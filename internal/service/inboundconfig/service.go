package inboundconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Service handles inbound config business logic.
type Service struct {
	repo *repository.InboundConfigRepository
}

// NewService creates a new inbound config service.
func NewService(repo *repository.InboundConfigRepository) *Service {
	return &Service{repo: repo}
}

// List returns all inbound configs for a core type.
func (s *Service) List(ctx context.Context, coreType string) ([]entity.InboundConfig, error) {
	return s.repo.List(ctx, coreType)
}

// GetByID returns an inbound config by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.InboundConfig, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new inbound config.
func (s *Service) Create(ctx context.Context, coreType, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	cfg := &entity.InboundConfig{
		CoreType:   coreType,
		Remark:     remark,
		Protocol:   protocol,
		Tag:        tag,
		Listen:     listen,
		Port:       port,
		ConfigJSON: configJSON,
		Enabled:    enabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.repo.Create(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Update updates an existing inbound config.
func (s *Service) Update(ctx context.Context, id int64, remark, protocol, tag, listen string, port int, configJSON string, enabled bool) (*entity.InboundConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	cfg.Remark = remark
	cfg.Protocol = protocol
	cfg.Tag = tag
	cfg.Listen = listen
	cfg.Port = port
	cfg.ConfigJSON = configJSON
	cfg.Enabled = enabled
	cfg.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Delete deletes an inbound config.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// GetEnabledInbounds returns all enabled inbound configs as parsed JSON.
func (s *Service) GetEnabledInbounds(ctx context.Context, coreType string) ([]json.RawMessage, error) {
	configs, err := s.repo.List(ctx, coreType)
	if err != nil {
		return nil, err
	}
	var result []json.RawMessage
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		// Parse the config JSON (contains full inbound object with settings, streamSettings, etc.)
		var inbound map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &inbound); err != nil {
			return nil, fmt.Errorf("parse inbound %s: %w", cfg.Tag, err)
		}
		// Ensure required fields are set from the record
		if cfg.Tag != "" {
			inbound["tag"] = cfg.Tag
		}
		if cfg.Listen != "" {
			inbound["listen"] = cfg.Listen
		}
		if cfg.Port > 0 {
			inbound["port"] = cfg.Port
		}
		if cfg.Protocol != "" {
			inbound["protocol"] = cfg.Protocol
		}
		// Remove internal fields
		delete(inbound, "_remark")
		delete(inbound, "_enabled")
		delete(inbound, "_listenType")
		delete(inbound, "_socketPath")
		delete(inbound, "_socketPerm")
		delete(inbound, "_wgPublicKey")
		// Clean nested _ fields from users/clients/accounts at top level and inside settings
		containers := []map[string]interface{}{inbound}
		if settings, ok := inbound["settings"].(map[string]interface{}); ok {
			containers = append(containers, settings)
		}
		for _, c := range containers {
			for _, key := range []string{"users", "clients", "accounts"} {
				if items, ok := c[key].([]interface{}); ok {
					for _, item := range items {
						if m, ok := item.(map[string]interface{}); ok {
							delete(m, "_quota")
							delete(m, "_expiry")
							delete(m, "_usedUp")
							delete(m, "_usedDown")
						}
					}
				}
			}
		}
		data, err := json.Marshal(inbound)
		if err != nil {
			return nil, fmt.Errorf("marshal inbound %s: %w", cfg.Tag, err)
		}
		result = append(result, data)
	}
	return result, nil
}
