package logmanage

import (
	"context"
	"encoding/json"
	"fmt"
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
const settingKeyCoreLogOutput = "core.log_output_config"

// perNodeKeySuffix returns the settings key suffix for a server ID.
// Local node (serverID=1) uses the legacy global key for backward compatibility.
// Remote nodes use a per-node key.
func perNodeKey(baseKey string, serverID int64) string {
	if serverID <= 1 {
		return baseKey
	}
	return fmt.Sprintf("%s.%d", baseKey, serverID)
}

// DefaultCapturePaths are the default capture file paths.
const (
	DefaultNginxCapturePath = "logs/nginx-access.log"
	DefaultCaddyCapturePath = "logs/caddy-access.log"
)

// GetLogOutput reads the log output configuration from database.
func (s *Service) GetLogOutput(ctx context.Context) (*LogOutputConfig, error) {
	return s.GetLogOutputForServer(ctx, 1)
}

// GetLogOutputForServer reads the log output configuration for a specific server.
func (s *Service) GetLogOutputForServer(ctx context.Context, serverID int64) (*LogOutputConfig, error) {
	cfg := &LogOutputConfig{
		Nginx: ProxyLogMode{Mode: "file", Managed: false},
		Caddy: ProxyLogMode{Mode: "file", Managed: false},
	}
	if s.settingStore == nil {
		return cfg, nil
	}
	key := perNodeKey(settingKeyLogOutput, serverID)
	v, err := s.settingStore.GetByKey(ctx, key)
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
	return s.SaveLogOutputForServer(ctx, 1, cfg)
}

// SaveLogOutputForServer saves the log output configuration for a specific server.
// For local node (serverID=1), it also starts/stops local writers.
// For remote nodes, the caller is responsible for syncing via gRPC.
func (s *Service) SaveLogOutputForServer(ctx context.Context, serverID int64, cfg *LogOutputConfig) error {
	if s.settingStore == nil {
		return nil
	}

	key := perNodeKey(settingKeyLogOutput, serverID)
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.settingStore.Set(ctx, key, string(data)); err != nil {
		return err
	}

	// Only start/stop local writers for the local node
	if serverID <= 1 {
		if err := s.StartWriter("nginx", cfg.Nginx.Mode); err != nil {
			log.Printf("logmanage: failed to start nginx writer: %v", err)
		}
		if err := s.StartWriter("caddy", cfg.Caddy.Mode); err != nil {
			log.Printf("logmanage: failed to start caddy writer: %v", err)
		}
	}

	return nil
}

// TakeoverStdout enables stdout capture for a proxy or core process.
// It sets Managed=true and configures a capture file path.
func (s *Service) TakeoverStdout(ctx context.Context, proxyType string) error {
	// Check if it's a core process type
	for _, ct := range coreLogTypes {
		if proxyType == ct {
			return s.takeoverCoreStdout(ctx, proxyType)
		}
	}

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

// ReleaseStdout disables stdout capture for a proxy or core process.
func (s *Service) ReleaseStdout(ctx context.Context, proxyType string) error {
	// Check if it's a core process type
	for _, ct := range coreLogTypes {
		if proxyType == ct {
			return s.releaseCoreStdout(ctx, proxyType)
		}
	}

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

// ============================================================
// Core process log output config (xray/singbox/hysteria2/tuic)
// ============================================================

// CoreLogOutputConfig defines the log output mode for core processes.
type CoreLogOutputConfig struct {
	Xray      CoreLogMode `json:"xray"`
	Singbox   CoreLogMode `json:"singbox"`
	Hysteria2 CoreLogMode `json:"hysteria2"`
	Tuic      CoreLogMode `json:"tuic"`
}

// CoreLogMode defines how a core process's logs are handled.
type CoreLogMode struct {
	Mode        string `json:"mode"`         // "file", "stdout", "off"
	Managed     bool   `json:"managed"`      // true = log management center captures stdout to file
	CapturePath string `json:"capture_path"`  // file path where captured stdout is written
}

// Default core capture paths.
const (
	DefaultXrayCapturePath      = "logs/xray-stdout.log"
	DefaultSingboxCapturePath   = "logs/singbox-stdout.log"
	DefaultHysteria2CapturePath = "logs/hysteria2-stdout.log"
	DefaultTuicCapturePath      = "logs/tuic-stdout.log"
)

// GetCoreLogOutput reads the core log output configuration from database.
func (s *Service) GetCoreLogOutput(ctx context.Context) (*CoreLogOutputConfig, error) {
	return s.GetCoreLogOutputForServer(ctx, 1)
}

// GetCoreLogOutputForServer reads the core log output configuration for a specific server.
func (s *Service) GetCoreLogOutputForServer(ctx context.Context, serverID int64) (*CoreLogOutputConfig, error) {
	cfg := &CoreLogOutputConfig{
		Xray:      CoreLogMode{Mode: "file", Managed: false},
		Singbox:   CoreLogMode{Mode: "file", Managed: false},
		Hysteria2: CoreLogMode{Mode: "file", Managed: false},
		Tuic:      CoreLogMode{Mode: "file", Managed: false},
	}
	if s.settingStore == nil {
		return cfg, nil
	}
	key := perNodeKey(settingKeyCoreLogOutput, serverID)
	v, err := s.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		return cfg, nil
	}
	var saved CoreLogOutputConfig
	if err := json.Unmarshal([]byte(v), &saved); err != nil {
		return cfg, nil
	}
	if saved.Xray.Mode == "" {
		saved.Xray.Mode = "file"
	}
	if saved.Singbox.Mode == "" {
		saved.Singbox.Mode = "file"
	}
	if saved.Hysteria2.Mode == "" {
		saved.Hysteria2.Mode = "file"
	}
	if saved.Tuic.Mode == "" {
		saved.Tuic.Mode = "file"
	}
	return &saved, nil
}

// SaveCoreLogOutput saves the core log output configuration to database.
func (s *Service) SaveCoreLogOutput(ctx context.Context, cfg *CoreLogOutputConfig) error {
	return s.SaveCoreLogOutputForServer(ctx, 1, cfg)
}

// SaveCoreLogOutputForServer saves the core log output configuration for a specific server.
func (s *Service) SaveCoreLogOutputForServer(ctx context.Context, serverID int64, cfg *CoreLogOutputConfig) error {
	if s.settingStore == nil {
		return nil
	}

	key := perNodeKey(settingKeyCoreLogOutput, serverID)
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.settingStore.Set(ctx, key, string(data)); err != nil {
		return err
	}

	// Only start/stop local writers for the local node
	if serverID <= 1 {
		coreModes := map[string]CoreLogMode{
			"xray":      cfg.Xray,
			"singbox":   cfg.Singbox,
			"hysteria2": cfg.Hysteria2,
			"tuic":      cfg.Tuic,
		}
		for coreType, mode := range coreModes {
			if err := s.StartWriter(coreType, mode.Mode); err != nil {
				log.Printf("logmanage: failed to start %s writer: %v", coreType, err)
			}
		}
	}

	return nil
}

// GetCoreLogMode returns the log mode for a specific core type.
// This implements the process.LogModeProvider interface.
func (s *Service) GetCoreLogMode(ctx context.Context, coreType string) string {
	return s.GetLogMode(ctx, coreType)
}

// GetLogMode returns the log mode for a specific core type.
// This implements the process.LogModeProvider interface.
func (s *Service) GetLogMode(ctx context.Context, coreType string) string {
	if s.settingStore == nil {
		return "file"
	}
	v, err := s.settingStore.GetByKey(ctx, settingKeyCoreLogOutput)
	if err != nil || v == "" {
		return "file"
	}
	var cfg CoreLogOutputConfig
	if json.Unmarshal([]byte(v), &cfg) != nil {
		return "file"
	}
	switch coreType {
	case "xray":
		if cfg.Xray.Mode != "" {
			return cfg.Xray.Mode
		}
	case "singbox":
		if cfg.Singbox.Mode != "" {
			return cfg.Singbox.Mode
		}
	case "hysteria2":
		if cfg.Hysteria2.Mode != "" {
			return cfg.Hysteria2.Mode
		}
	case "tuic":
		if cfg.Tuic.Mode != "" {
			return cfg.Tuic.Mode
		}
	}
	return "file"
}

// takeoverCoreStdout enables stdout capture for a core process.
func (s *Service) takeoverCoreStdout(ctx context.Context, coreType string) error {
	cfg, err := s.GetCoreLogOutput(ctx)
	if err != nil {
		return err
	}
	switch coreType {
	case "xray":
		cfg.Xray.Managed = true
		if cfg.Xray.CapturePath == "" {
			cfg.Xray.CapturePath = DefaultXrayCapturePath
		}
	case "singbox":
		cfg.Singbox.Managed = true
		if cfg.Singbox.CapturePath == "" {
			cfg.Singbox.CapturePath = DefaultSingboxCapturePath
		}
	case "hysteria2":
		cfg.Hysteria2.Managed = true
		if cfg.Hysteria2.CapturePath == "" {
			cfg.Hysteria2.CapturePath = DefaultHysteria2CapturePath
		}
	case "tuic":
		cfg.Tuic.Managed = true
		if cfg.Tuic.CapturePath == "" {
			cfg.Tuic.CapturePath = DefaultTuicCapturePath
		}
	default:
		return nil
	}
	return s.SaveCoreLogOutput(ctx, cfg)
}

// releaseCoreStdout disables stdout capture for a core process.
func (s *Service) releaseCoreStdout(ctx context.Context, coreType string) error {
	cfg, err := s.GetCoreLogOutput(ctx)
	if err != nil {
		return err
	}
	switch coreType {
	case "xray":
		cfg.Xray.Managed = false
	case "singbox":
		cfg.Singbox.Managed = false
	case "hysteria2":
		cfg.Hysteria2.Managed = false
	case "tuic":
		cfg.Tuic.Managed = false
	default:
		return nil
	}
	return s.SaveCoreLogOutput(ctx, cfg)
}
