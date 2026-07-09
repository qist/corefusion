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
// For xray and singbox, the mode is derived from the actual log section in core_configs
// to stay in sync with changes made in the protocol center.
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
		// No saved config — still try to derive mode from actual log section
		s.deriveModesFromConfig(ctx, serverID, cfg)
		return cfg, nil
	}
	var saved CoreLogOutputConfig
	if err := json.Unmarshal([]byte(v), &saved); err != nil {
		s.deriveModesFromConfig(ctx, serverID, cfg)
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

	// Override modes from actual log section for xray and singbox
	s.deriveModesFromConfig(ctx, serverID, &saved)

	return &saved, nil
}

// deriveModesFromConfig reads the actual log section from core_configs and
// derives the output mode for xray and singbox. This keeps the log management
// UI in sync with changes made in the protocol center.
func (s *Service) deriveModesFromConfig(ctx context.Context, serverID int64, cfg *CoreLogOutputConfig) {
	if s.configGetter == nil {
		return
	}

	// Derive xray mode from log section
	if logSection, err := s.configGetter.GetSection(ctx, serverID, "xray", "log"); err == nil {
		if m, ok := logSection.(map[string]interface{}); ok {
			derived := inferXrayLogMode(m)
			if derived != "" {
				cfg.Xray.Mode = derived
			}
		}
	}

	// Derive singbox mode from log section
	if logSection, err := s.configGetter.GetSection(ctx, serverID, "singbox", "log"); err == nil {
		if m, ok := logSection.(map[string]interface{}); ok {
			derived := inferSingboxLogMode(m)
			if derived != "" {
				cfg.Singbox.Mode = derived
			}
		}
	}
}

// inferXrayLogMode derives the output mode from the xray log section.
// Rules:
//   - loglevel: "none" → off
//   - access: "none" AND error: "none" → off
//   - access: "" OR error: "" OR error: "/dev/stderr" → stdout
//   - Otherwise → file (has file paths)
func inferXrayLogMode(log map[string]interface{}) string {
	access, _ := log["access"].(string)
	errorVal, _ := log["error"].(string)
	loglevel, _ := log["loglevel"].(string)

	// off mode
	if loglevel == "none" {
		return "off"
	}
	if access == "none" && errorVal == "none" {
		return "off"
	}

	// stdout mode — empty string means stdout in xray, /dev/stderr is LogBroker capture
	if access == "" || errorVal == "" || errorVal == "/dev/stderr" {
		return "stdout"
	}

	// file mode — has actual file paths
	return "file"
}

// inferSingboxLogMode derives the output mode from the singbox log section.
// Rules:
//   - level: "silent" → off
//   - output: "stderr" → stdout
//   - Otherwise → file
func inferSingboxLogMode(log map[string]interface{}) string {
	level, _ := log["level"].(string)
	output, _ := log["output"].(string)

	if level == "silent" {
		return "off"
	}
	if output == "stderr" {
		return "stdout"
	}
	return "file"
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

// GetNginxSettingsRaw returns the raw JSON of proxy.nginx_settings from the database.
// Used for syncing nginx settings to remote agents for log path discovery.
func (s *Service) GetNginxSettingsRaw(ctx context.Context) string {
	if s.settingStore == nil {
		return ""
	}
	v, err := s.settingStore.GetByKey(ctx, "proxy.nginx_settings")
	if err != nil {
		return ""
	}
	return v
}

// GetLogMode returns the log mode for a specific core type (local node only).
// This implements the process.LogModeProvider interface.
func (s *Service) GetLogMode(ctx context.Context, coreType string) string {
	return s.GetLogModeForServer(ctx, 1, coreType)
}

// GetLogModeForServer returns the log mode for a specific server and core type.
// It reads the per-node key (e.g. "core.log_output_config.2") first, and falls
// back to the global key ("core.log_output_config") for backward compatibility.
// This implements the configsync.LogModeProvider interface.
func (s *Service) GetLogModeForServer(ctx context.Context, serverID int64, coreType string) string {
	if s.settingStore == nil {
		return "file"
	}

	// Try per-node key first, fall back to global key
	key := perNodeKey(settingKeyCoreLogOutput, serverID)
	v, err := s.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		// Fall back to global key (local node config)
		v, err = s.settingStore.GetByKey(ctx, settingKeyCoreLogOutput)
		if err != nil || v == "" {
			return "file"
		}
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
