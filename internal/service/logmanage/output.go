package logmanage

import (
	"context"
	"encoding/json"
	"log"
)

// LogOutputConfig defines the log capture and output mode for nginx and caddy.
type LogOutputConfig struct {
	Nginx ProxyLogMode `json:"nginx"`
	Caddy ProxyLogMode `json:"caddy"`
}

// ProxyLogMode defines how a proxy's logs are handled.
type ProxyLogMode struct {
	Mode      string `json:"mode"`       // "file" (proxy writes to file), "stdout" (proxy writes to stdout)
	Managed   bool   `json:"managed"`    // true = log management center captures stdout to file
	CapturePath string `json:"capture_path"` // file path where captured stdout is written (e.g. "logs/nginx-stdout.log")
}

const settingKeyLogOutput = "proxy.log_output_config"

// DefaultCapturePaths are the default capture file paths.
const (
	DefaultNginxCapturePath = "logs/nginx-access.log"
	DefaultCaddyCapturePath = "logs/caddy-access.log"
)

// GetLogOutput reads the log output configuration from database.
func (s *Service) GetLogOutput(ctx context.Context) (*LogOutputConfig, error) {
	cfg := &LogOutputConfig{
		Nginx: ProxyLogMode{Mode: "file", Managed: false},
		Caddy: ProxyLogMode{Mode: "file", Managed: false},
	}
	if s.settingStore == nil {
		return cfg, nil
	}
	v, err := s.settingStore.GetByKey(ctx, settingKeyLogOutput)
	if err != nil || v == "" {
		return cfg, nil
	}
	var saved LogOutputConfig
	if err := json.Unmarshal([]byte(v), &saved); err != nil {
		return cfg, nil
	}
	if saved.Nginx.Mode == "" {
		saved.Nginx.Mode = "file"
	}
	if saved.Caddy.Mode == "" {
		saved.Caddy.Mode = "file"
	}
	return &saved, nil
}

// SaveLogOutput saves the log output configuration to database.
func (s *Service) SaveLogOutput(ctx context.Context, cfg *LogOutputConfig) error {
	if s.settingStore == nil {
		return nil
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.settingStore.Set(ctx, settingKeyLogOutput, string(data)); err != nil {
		return err
	}

	// Start/stop log writers (doesn't affect nginx/caddy processes)
	if err := s.StartWriter("nginx", cfg.Nginx.Mode); err != nil {
		log.Printf("logmanage: failed to start nginx writer: %v", err)
	}
	if err := s.StartWriter("caddy", cfg.Caddy.Mode); err != nil {
		log.Printf("logmanage: failed to start caddy writer: %v", err)
	}

	// Note: proxy config reload is handled by the caller (frontend)
	return nil
}

// TakeoverStdout enables stdout capture for a proxy.
// It sets Managed=true and configures a capture file path.
// The proxy stays in stdout mode - the log management center captures the output.
func (s *Service) TakeoverStdout(ctx context.Context, proxyType string) error {
	cfg, err := s.GetLogOutput(ctx)
	if err != nil {
		return err
	}
	switch proxyType {
	case "nginx":
		cfg.Nginx.Managed = true
		if cfg.Nginx.CapturePath == "" {
			cfg.Nginx.CapturePath = DefaultNginxCapturePath
		}
	case "caddy":
		cfg.Caddy.Managed = true
		if cfg.Caddy.CapturePath == "" {
			cfg.Caddy.CapturePath = DefaultCaddyCapturePath
		}
	default:
		return nil
	}
	return s.SaveLogOutput(ctx, cfg)
}

// ReleaseStdout disables stdout capture for a proxy.
func (s *Service) ReleaseStdout(ctx context.Context, proxyType string) error {
	cfg, err := s.GetLogOutput(ctx)
	if err != nil {
		return err
	}
	switch proxyType {
	case "nginx":
		cfg.Nginx.Managed = false
	case "caddy":
		cfg.Caddy.Managed = false
	default:
		return nil
	}
	return s.SaveLogOutput(ctx, cfg)
}
