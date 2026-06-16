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
	"xray":       {"log", "dns", "routing", "policy", "inbounds", "outbounds", "transport", "stats", "metrics", "api", "reverse", "fakedns", "observatory", "burstObservatory", "version", "geodata"},
	"singbox":    {"log", "dns", "route", "inbounds", "outbounds", "experimental"},
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
		"dns": `{}`,
		"routing": `{
  "domainStrategy": "IPIfNonMatch",
  "domainMatcher": "",
  "rules": [
    {
      "type": "field",
      "inboundTag": ["api"],
      "outboundTag": "api",
      "ruleTag": "api-rule"
    }
  ],
  "balancers": []
}`,
		"policy": `{
  "levels": {
    "0": {
      "handshake": 4,
      "connIdle": 300,
      "uplinkOnly": 2,
      "downlinkOnly": 5,
      "statsUserUplink": true,
      "statsUserDownlink": true,
      "statsOnline": true,
      "bufferSize": 10240
    }
  },
  "system": {
    "statsInboundUplink": true,
    "statsInboundDownlink": true,
    "statsOutboundUplink": false,
    "statsOutboundDownlink": false
  }
}`,
		"inbounds": `[
  {
    "listen": "127.0.0.1",
    "port": 62789,
    "protocol": "dokodemo-door",
    "settings": {"address": "127.0.0.1"},
    "tag": "api"
  }
]`,
		"outbounds": `[
  {"protocol": "freedom", "tag": "direct"},
  {"protocol": "blackhole", "tag": "blocked"}
]`,
		"transport": `{
  "network": "tcp",
  "security": "none",
  "tlsSettings": {},
  "tcpSettings": {},
  "kcpSettings": {},
  "wsSettings": {},
  "httpSettings": {},
  "quicSettings": {},
  "grpcSettings": {},
  "httpupgradeSettings": {},
  "splithttpSettings": {},
  "sockopt": {}
}`,
		"stats": `{}`,
		"metrics": `{
  "tag": "metrics_out",
  "listen": "127.0.0.1:11111"
}`,
		"reverse": `{
  "bridges": [],
  "portals": []
}`,
		"fakedns": `{
  "ipPool": "198.18.0.0/15",
  "poolSize": 65535
}`,
		"observatory": `{
  "subjectSelector": [],
  "probeURL": "http://www.google.com/generate_204",
  "probeInterval": "120s",
  "enableConcurrency": false
}`,
		"burstObservatory": `{
  "subjectSelector": [],
  "pingConfig": {
    "destination": "http://www.google.com/generate_204",
    "connectivity": "",
    "interval": "120s",
    "sampling": 5,
    "timeout": "16s",
    "httpMethod": "HEAD"
  }
}`,
		"api": `{
  "tag": "api",
  "listen": "127.0.0.1:10085",
  "services": ["HandlerService", "LoggerService", "StatsService"]
}`,
		"version": `{
  "min": "",
  "max": ""
}`,
		"geodata": `{
  "cron": "",
  "outbound": "direct",
  "assets": []
}`,
	},
	"singbox": {
		"log":         `{"level":"info","timestamp":true}`,
		"dns":         `{"servers":[],"rules":[],"final":""}`,
		"route":       `{"rules":[],"auto_detect_interface":true,"final":"direct"}`,
		"inbounds":    `[]`,
		"outbounds":   `[{"type":"direct","tag":"direct"},{"type":"block","tag":"block"}]`,
		"experimental":`{}`,
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

// SeedDefaults creates default config sections for all supported core types.
func (s *Service) SeedDefaults(ctx context.Context) error {
	for _, coreType := range SupportedCoreTypes {
		sections := CoreSections[coreType]
		defaults := DefaultSectionJSON[coreType]
		for _, section := range sections {
			_, err := s.repo.GetSection(ctx, coreType, section)
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

// GetFull returns all sections merged into a single JSON object.
func (s *Service) GetFull(ctx context.Context, coreType string) (map[string]interface{}, error) {
	configs, err := s.repo.GetAllSections(ctx, coreType)
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
	return result, nil
}

// GetSection returns a single section's parsed data.
func (s *Service) GetSection(ctx context.Context, coreType, section string) (interface{}, error) {
	cfg, err := s.repo.GetSection(ctx, coreType, section)
	if err != nil {
		return nil, fmt.Errorf("section %q not found: %w", section, err)
	}
	var val interface{}
	if err := ParseJSONC([]byte(cfg.ConfigJSON), &val); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s/%s: %w", coreType, section, err)
	}
	return val, nil
}

// SaveSection updates a single section.
func (s *Service) SaveSection(ctx context.Context, coreType, section string, data json.RawMessage) error {
	if !ValidJSONC(data) {
		return fmt.Errorf("invalid JSON")
	}
	cfg := &entity.CoreConfig{
		CoreType:   coreType,
		Section:    section,
		ConfigJSON: string(data),
	}
	return s.repo.UpsertSection(ctx, cfg)
}

// SaveFull replaces all sections for a core type from a merged JSON object.
func (s *Service) SaveFull(ctx context.Context, coreType string, fullJSON map[string]interface{}) error {
	for section, val := range fullJSON {
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshal %s/%s: %w", coreType, section, err)
		}
		if err := s.SaveSection(ctx, coreType, section, json.RawMessage(data)); err != nil {
			return err
		}
	}
	return nil
}

// List returns all core configs grouped by core type.
func (s *Service) List(ctx context.Context) (map[string][]entity.CoreConfig, error) {
	configs, err := s.repo.List(ctx)
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
