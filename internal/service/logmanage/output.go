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
	Mode        string `json:"mode"`         // "file" (proxy writes to file), "stdout" (proxy writes to stdout)
	Managed     bool   `json:"managed"`      // true = log management center captures stdout to file
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
	DefaultNginxCapturePath = "logs/nginx-stdout.log"
	DefaultCaddyCapturePath = "logs/caddy-stdout.log"
)

// normalizeLogMode 归一化日志输出模式。
// 标准输出（stdout）模式已下线：面板/Agent 负责核心与代理的日志采集和轮转，
// 进程一律写真实文件，因此历史配置里的 stdout 统一按 file 处理，只保留 file / off。
func normalizeLogMode(mode string) string {
	if mode == "off" {
		return "off"
	}
	return "file"
}

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
	// stdout 模式已下线：读到历史配置时归一为 file（并重置 managed），
	// 同时写回，避免每次读取都重复迁移。
	changed := false
	if n := normalizeLogMode(saved.Nginx.Mode); n != saved.Nginx.Mode {
		saved.Nginx.Mode, saved.Nginx.Managed, changed = n, false, true
	}
	if n := normalizeLogMode(saved.Caddy.Mode); n != saved.Caddy.Mode {
		saved.Caddy.Mode, saved.Caddy.Managed, changed = n, false, true
	}
	if changed {
		if data, err := json.Marshal(&saved); err == nil {
			_ = s.settingStore.Set(ctx, key, string(data))
		}
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

	// stdout 模式已下线：任何写入都归一为 file / off。
	cfg.Nginx.Mode = normalizeLogMode(cfg.Nginx.Mode)
	cfg.Caddy.Mode = normalizeLogMode(cfg.Caddy.Mode)

	// Get old config to detect mode changes
	oldCfg, _ := s.GetLogOutputForServer(ctx, serverID)

	// If mode changed (e.g. protocol center switched it), reset managed to false.
	// The user must manually enable managed after a mode change.
	if oldCfg != nil {
		if oldCfg.Nginx.Mode != cfg.Nginx.Mode {
			cfg.Nginx.Managed = false
		}
		if oldCfg.Caddy.Mode != cfg.Caddy.Mode {
			cfg.Caddy.Managed = false
		}
	}

	// Auto-fix: managed should only be true when mode is stdout
	if cfg.Nginx.Mode != "stdout" {
		cfg.Nginx.Managed = false
	}
	if cfg.Caddy.Mode != "stdout" {
		cfg.Caddy.Managed = false
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
		s.syncWriterFilePath("nginx", cfg.Nginx.CapturePath)
		s.syncWriterFilePath("caddy", cfg.Caddy.CapturePath)
		if err := s.StartWriterManaged("nginx", cfg.Nginx.Mode, cfg.Nginx.Managed); err != nil {
			log.Printf("logmanage: failed to start nginx writer: %v", err)
		}
		if err := s.StartWriterManaged("caddy", cfg.Caddy.Mode, cfg.Caddy.Managed); err != nil {
			log.Printf("logmanage: failed to start caddy writer: %v", err)
		}

		// Trigger mode switch callback if mode changed (need to restart proxy for stdout mode)
		if s.onModeSwitch != nil {
			if oldCfg == nil || oldCfg.Nginx.Mode != cfg.Nginx.Mode {
				s.onModeSwitch("nginx")
			}
			if oldCfg == nil || oldCfg.Caddy.Mode != cfg.Caddy.Mode {
				s.onModeSwitch("caddy")
			}
		}
	}

	return nil
}

// ============================================================
// Core process log output config (xray/singbox)
// ============================================================

// CoreLogOutputConfig defines the log output mode for core processes.
type CoreLogOutputConfig struct {
	Xray    CoreLogMode `json:"xray"`
	Singbox CoreLogMode `json:"singbox"`
}

// CoreLogMode defines how a core process's logs are handled.
type CoreLogMode struct {
	Mode        string `json:"mode"`         // "file", "stdout", "off"
	Managed     bool   `json:"managed"`      // true = log management center captures stdout to file
	CapturePath string `json:"capture_path"` // file path where captured stdout is written
}

// Default core capture paths.
const (
	DefaultXrayCapturePath    = "logs/xray-stdout.log"
	DefaultSingboxCapturePath = "logs/singbox-stdout.log"
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
		Xray:    CoreLogMode{Mode: "file", Managed: false},
		Singbox: CoreLogMode{Mode: "file", Managed: false},
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
	// Remember the stored modes before deriving from actual config
	storedXrayMode := saved.Xray.Mode
	storedSingboxMode := saved.Singbox.Mode

	// Override modes from actual log section for xray and singbox
	s.deriveModesFromConfig(ctx, serverID, &saved)

	// If mode changed (protocol center switched it), reset managed to false.
	// The user must manually enable managed after a mode change.
	modeChanged := false
	if saved.Xray.Mode != storedXrayMode {
		saved.Xray.Managed = false
		modeChanged = true
	}
	if saved.Singbox.Mode != storedSingboxMode {
		saved.Singbox.Managed = false
		modeChanged = true
	}

	// Auto-fix: managed should only be true when mode is stdout
	if saved.Xray.Mode != "stdout" {
		saved.Xray.Managed = false
	}
	if saved.Singbox.Mode != "stdout" {
		saved.Singbox.Managed = false
	}
	// If mode changed, persist the corrected config so that SyncData
	// sends the right managed values to remote agents.
	if modeChanged {
		data, _ := json.Marshal(&saved)
		if len(data) > 0 {
			_ = s.settingStore.Set(ctx, key, string(data))
		}
	}

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
//   - Otherwise → file（标准输出模式已下线，access/error 为空统一按文件处理）
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

	return "file"
}

// inferSingboxLogMode derives the output mode from the singbox log section.
// Rules:
//   - level: "silent" → off
//   - Otherwise → file（stderr 即标准输出，已下线）
func inferSingboxLogMode(log map[string]interface{}) string {
	level, _ := log["level"].(string)

	if level == "silent" {
		return "off"
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

	// Get old config to detect mode changes
	oldCfg, _ := s.GetCoreLogOutputForServer(ctx, serverID)

	// stdout 模式已下线：任何写入都归一为 file / off。
	cfg.Xray.Mode = normalizeLogMode(cfg.Xray.Mode)
	cfg.Singbox.Mode = normalizeLogMode(cfg.Singbox.Mode)

	// If mode changed (e.g. protocol center switched it), reset managed to false.
	// The user must manually enable managed after a mode change.
	if oldCfg != nil {
		if oldCfg.Xray.Mode != cfg.Xray.Mode {
			cfg.Xray.Managed = false
		}
		if oldCfg.Singbox.Mode != cfg.Singbox.Mode {
			cfg.Singbox.Managed = false
		}
	}

	// Auto-fix: managed should only be true when mode is stdout
	if cfg.Xray.Mode != "stdout" {
		cfg.Xray.Managed = false
	}
	if cfg.Singbox.Mode != "stdout" {
		cfg.Singbox.Managed = false
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
			"xray":    cfg.Xray,
			"singbox": cfg.Singbox,
		}
		for coreType, mode := range coreModes {
			s.syncWriterFilePath(coreType, mode.CapturePath)
			if err := s.StartWriterManaged(coreType, mode.Mode, mode.Managed); err != nil {
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
// For xray and singbox, the mode is overridden from the actual log section in
// core_configs to stay in sync with changes made in the protocol center.
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

	// Override modes from actual log section for xray and singbox
	s.deriveModesFromConfig(ctx, serverID, &cfg)

	switch coreType {
	case "xray":
		if cfg.Xray.Mode != "" {
			return normalizeLogMode(cfg.Xray.Mode)
		}
	case "singbox":
		if cfg.Singbox.Mode != "" {
			return normalizeLogMode(cfg.Singbox.Mode)
		}
	}
	return "file"
}
