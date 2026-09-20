package coreconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// SupportedCoreTypes lists all supported proxy core types.
var SupportedCoreTypes = []string{"xray", "singbox", "hysteria2", "tuic"}

// CoreSections defines the ordered sections for each core type.
var CoreSections = map[string][]string{
	"xray":       {"log", "dns", "routing", "policy", "inbounds", "outbounds", "stats", "metrics", "api", "fakedns", "observatory", "burstObservatory", "version", "geodata"},
	"singbox":    {"log", "dns", "ntp", "certificate", "certificate_providers", "http_clients", "network_namespaces", "route", "inbounds", "outbounds", "services", "experimental"},
	"hysteria2":  {"listen", "tls", "auth", "bandwidth", "quic", "trafficStats"},
	"tuic":       {"server", "users", "certificate", "congestion_control"},
}

// DefaultSectionJSON provides default JSON for each section.
var DefaultSectionJSON = map[string]map[string]string{
	"xray": {
		"log": `{
  "access": "none",
  "error": "none",
  "loglevel": "info",
  "dnsLog": false,
  "timestamp": true,
  "maskAddress": ""
}`,
		"dns":         `{}`,
		"routing": `{
  "domainStrategy": "IPIfNonMatch",
  "domainMatcher": "",
  "rules": [],
  "balancers": []
}`,
		"policy": `{
  "levels": {
    "0": {
      "statsUserUplink": true,
      "statsUserDownlink": true
    }
  },
  "system": {
    "statsInboundUplink": true,
    "statsInboundDownlink": true,
    "statsOutboundUplink": false,
    "statsOutboundDownlink": false
  }
}`,
		"inbounds": `[]`,
		"outbounds": `[
  {"protocol": "freedom", "tag": "direct"},
  {"protocol": "blackhole", "tag": "blocked"}
]`,
		"stats": `{}`,
		"metrics": `{}`,
		"fakedns": `{}`,
		"observatory":  `{}`,
		"burstObservatory": `{}`,
		"api": `{
  "tag": "api",
  "services": ["HandlerService", "LoggerService", "StatsService"]
}`,
		"version":    `{}`,
		"geodata":   `{}`,
	},
	"singbox": {
		"log":          `{"level":"info","timestamp":true}`,
		"dns":          `{"servers":[],"rules":[],"final":""}`,
		"ntp":          `{}`,
		"certificate":  `{}`,
		"certificate_providers": `[]`,
		"http_clients":          `[]`,
		"network_namespaces":    `[]`,
		"route":       `{"rules":[],"auto_detect_interface":true,"final":"direct"}`,
		"inbounds":    `[]`,
		"outbounds":   `[{"type":"direct","tag":"direct"},{"type":"block","tag":"block"}]`,
		"services":    `[]`,
		"experimental":`{"v2ray_api":{"listen":"127.0.0.1:10086","stats":{"enabled":true}}}`,
	},
	"hysteria2": {
		"listen":       `":443"`,
		"tls":          `{"cert":"","key":""}`,
		"auth":         `{"type":"password","password":""}`,
		"bandwidth":    `{"up":"","down":""}`,
		"quic":         `{}`,
		"trafficStats": `{"listen":"","secret":""}`,
	},
	"tuic": {
		"server":             `"[::]:443"`,
		"users":              `{}`,
		"certificate":        `{"certificate":"","private_key":""}`,
		"congestion_control": `"bbr"`,
	},
}

// Service handles core config business logic.
type Service struct {
	repo *repository.CoreConfigRepository
}

// NewService creates a new core config service.
func NewService(repo *repository.CoreConfigRepository) *Service {
	return &Service{repo: repo}
}

// SeedDefaults creates default config sections for all supported core types on a specific server.
func (s *Service) SeedDefaults(ctx context.Context, serverID int64) error {
	for _, coreType := range SupportedCoreTypes {
		sections := CoreSections[coreType]
		defaults := DefaultSectionJSON[coreType]
		for _, section := range sections {
			_, err := s.repo.GetSection(ctx, serverID, coreType, section)
			if err == nil {
				continue // already exists
			}
			defaultJSON := "{}"
			if defaults != nil {
				if v, ok := defaults[section]; ok {
					defaultJSON = v
				}
			}
			cfg := &entity.CoreConfig{
				ServerID:   serverID,
				CoreType:   coreType,
				Section:    section,
				ConfigJSON: defaultJSON,
				UpdatedAt:  time.Now(),
			}
			if err := s.repo.UpsertSection(ctx, cfg); err != nil {
				return fmt.Errorf("seed %s/%s: %w", coreType, section, err)
			}
		}
	}
	return nil
}

// GetFull returns all sections merged into a single JSON object for a specific server.
// If sections are missing from the database, default values are used and persisted.
func (s *Service) GetFull(ctx context.Context, serverID int64, coreType string) (map[string]interface{}, error) {
	configs, err := s.repo.GetAllSections(ctx, serverID, coreType)
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{})
	for _, cfg := range configs {
		var val interface{}
		if err := ParseJSONC([]byte(cfg.ConfigJSON), &val); err != nil {
			val = cfg.ConfigJSON // fallback to raw string
		}
		result[cfg.Section] = val
	}

	// Fill in any missing sections with defaults and persist them to DB
	sections := CoreSections[coreType]
	defaults := DefaultSectionJSON[coreType]
	for _, section := range sections {
		if _, exists := result[section]; !exists {
			defaultJSON := "{}"
			if defaults != nil {
				if v, ok := defaults[section]; ok {
					defaultJSON = v
				}
			}
			var val interface{}
			if err := ParseJSONC([]byte(defaultJSON), &val); err != nil {
				val = defaultJSON
			}
			result[section] = val
			// Persist the default to DB so future reads find it
			cfg := &entity.CoreConfig{
				ServerID:   serverID,
				CoreType:   coreType,
				Section:    section,
				ConfigJSON: defaultJSON,
				UpdatedAt:  time.Now(),
			}
			if err := s.repo.UpsertSection(ctx, cfg); err != nil {
				// Non-fatal: we already have the default in memory
				fmt.Printf("[coreconfig] GetFull: failed to persist default %s/%s for server %d: %v\n", coreType, section, serverID, err)
			}
		}
	}

	return result, nil
}

// GetSection returns a single section's parsed data for a specific server.
// If the section does not exist in the database, the default value from
// DefaultSectionJSON is returned and persisted to DB for future reads.
func (s *Service) GetSection(ctx context.Context, serverID int64, coreType, section string) (interface{}, error) {
	cfg, err := s.repo.GetSection(ctx, serverID, coreType, section)
	if err == nil {
		var val interface{}
		if err := ParseJSONC([]byte(cfg.ConfigJSON), &val); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s/%s: %w", coreType, section, err)
		}
		return val, nil
	}

	// Section not found in DB — try to return and persist the default value
	defaults := DefaultSectionJSON[coreType]
	if defaults != nil {
		if defaultJSON, ok := defaults[section]; ok {
			var val interface{}
			if err := ParseJSONC([]byte(defaultJSON), &val); err != nil {
				val = defaultJSON
			}
			// Persist default to DB so future reads find it
			newCfg := &entity.CoreConfig{
				ServerID:   serverID,
				CoreType:   coreType,
				Section:    section,
				ConfigJSON: defaultJSON,
				UpdatedAt:  time.Now(),
			}
			if err := s.repo.UpsertSection(ctx, newCfg); err != nil {
				fmt.Printf("[coreconfig] GetSection: failed to persist default %s/%s for server %d: %v\n", coreType, section, serverID, err)
			}
			return val, nil
		}
	}

	return nil, fmt.Errorf("section %q not found: %w", section, err)
}

// SaveSection updates a single section for a specific server.
func (s *Service) SaveSection(ctx context.Context, serverID int64, coreType, section string, data json.RawMessage) error {
	if !ValidJSONC(data) {
		return fmt.Errorf("invalid JSON")
	}
	cfg := &entity.CoreConfig{
		ServerID:   serverID,
		CoreType:   coreType,
		Section:    section,
		ConfigJSON: string(data),
	}
	return s.repo.UpsertSection(ctx, cfg)
}

// SaveFull replaces all sections for a core type from a merged JSON object on a specific server.
func (s *Service) SaveFull(ctx context.Context, serverID int64, coreType string, fullJSON map[string]interface{}) error {
	// Get existing sections
	existing, err := s.repo.GetAllSections(ctx, serverID, coreType)
	if err != nil {
		return err
	}

	// Save new/updated sections
	for section, val := range fullJSON {
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshal %s/%s: %w", coreType, section, err)
		}
		if err := s.SaveSection(ctx, serverID, coreType, section, json.RawMessage(data)); err != nil {
			return err
		}
	}

	// Delete sections that are no longer in the config
	for _, cfg := range existing {
		if _, ok := fullJSON[cfg.Section]; !ok {
			s.repo.DeleteSection(ctx, serverID, coreType, cfg.Section)
		}
	}

	return nil
}

// List returns all core configs grouped by core type for a specific server.
func (s *Service) List(ctx context.Context, serverID int64) (map[string][]entity.CoreConfig, error) {
	configs, err := s.repo.List(ctx, serverID)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]entity.CoreConfig)
	for _, cfg := range configs {
		result[cfg.CoreType] = append(result[cfg.CoreType], cfg)
	}
	return result, nil
}

// Validate checks if a JSON/JSONC string is valid.
func (s *Service) Validate(configJSON string) error {
	if !ValidJSONC([]byte(configJSON)) {
		return fmt.Errorf("invalid JSON")
	}
	return nil
}
