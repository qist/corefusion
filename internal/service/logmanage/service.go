package logmanage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// Supported log source types.
var (
	// proxyLogTypes are the proxy process types managed by the log center.
	proxyLogTypes = []string{"nginx", "caddy", "waf"}
	// coreLogTypes are the core process types managed by the log center.
	coreLogTypes = []string{"xray", "singbox", "hysteria2", "tuic"}
	// allLogTypes is the combined list for iteration.
	allLogTypes = append(append([]string{}, proxyLogTypes...), coreLogTypes...)
)

// AllLogTypes returns all supported log source types (proxy + core).
func AllLogTypes() []string {
	return allLogTypes
}

// LogBrokerReader is the interface for reading from a proxy log broker.
type LogBrokerReader interface {
	Snapshot() []string
	Subscribe() (<-chan string, func())
}

// ProxyLogBroker provides access to proxy stdout log broker.
type ProxyLogBroker interface {
	GetLogBroker(proxyType string) LogBrokerReader
}

// ProxyBrokerAdapter adapts proxy.Manager to ProxyLogBroker interface.
type ProxyBrokerAdapter struct {
	GetBrokerFn func(proxyType string) *LogBrokerWrapper
}

func (a *ProxyBrokerAdapter) GetLogBroker(proxyType string) LogBrokerReader {
	if a == nil || a.GetBrokerFn == nil {
		return nil
	}
	w := a.GetBrokerFn(proxyType)
	if w == nil {
		return nil
	}
	return w
}

// LogBrokerWrapper wraps the proxy LogBroker for the logmanage package.
type LogBrokerWrapper struct {
	SnapshotFn  func() []string
	SubscribeFn func() (<-chan string, func())
}

func (w *LogBrokerWrapper) Snapshot() []string        { return w.SnapshotFn() }
func (w *LogBrokerWrapper) Subscribe() (<-chan string, func()) { return w.SubscribeFn() }

// CoreLogBrokerProvider provides access to core process log brokers.
type CoreLogBrokerProvider interface {
	GetCoreLogBroker(coreType string) LogBrokerReader
}

// CoreConfigSectionGetter provides access to core config sections (e.g. xray log section).
// Used to derive the log output mode from the actual protocol center config.
type CoreConfigSectionGetter interface {
	GetSection(ctx context.Context, serverID int64, coreType, section string) (interface{}, error)
}

// Service provides log management operations.
type Service struct {
	policyRepo    *repository.LogRotationPolicyRepository
	jobRepo       *repository.LogRotationJobRepository
	settingStore  SettingStore
	caddySiteRepo *repository.CaddySiteRepository
	dataDir       string
	scheduler     *Scheduler
	proxyBroker   ProxyLogBroker
	coreBroker    CoreLogBrokerProvider
	configGetter  CoreConfigSectionGetter
	writers       map[string]*LogWriter
	onModeSwitch  func(proxyType string) // callback when output mode is switched
	clientLogDeps *ClientLogDeps          // optional: for client log packaging
}

// NewService creates a new log management service.
func NewService(
	policyRepo *repository.LogRotationPolicyRepository,
	jobRepo *repository.LogRotationJobRepository,
	settingStore SettingStore,
	caddySiteRepo *repository.CaddySiteRepository,
	dataDir string,
	proxyBroker ProxyLogBroker,
) *Service {
	svc := &Service{
		policyRepo:    policyRepo,
		jobRepo:       jobRepo,
		settingStore:  settingStore,
		caddySiteRepo: caddySiteRepo,
		dataDir:       dataDir,
		proxyBroker:   proxyBroker,
		writers:       make(map[string]*LogWriter),
	}
	svc.scheduler = NewScheduler(policyRepo, svc)
	return svc
}

// SetCoreLogBrokerProvider sets the provider for core process log brokers.
func (s *Service) SetCoreLogBrokerProvider(provider CoreLogBrokerProvider) {
	s.coreBroker = provider
}

// SetCoreConfigSectionGetter sets the provider for reading core config sections.
// This enables deriving the log output mode from the actual protocol center config.
func (s *Service) SetCoreConfigSectionGetter(getter CoreConfigSectionGetter) {
	s.configGetter = getter
}

// StartAutoWriters starts writers based on the saved LogOutputConfig and CoreLogOutputConfig.
// It reads the real mode and managed flag from the output configs (not the writer internal config)
// and uses StartWriterManaged to correctly set the paused state.
func (s *Service) StartAutoWriters() {
	s.StartAutoWritersForServer(1)
}

// StartAutoWritersForServer is the same as StartAutoWriters but uses the given serverID
// for reading configs. This is needed on the Agent where the serverID may differ from 1.
func (s *Service) StartAutoWritersForServer(serverID int64) {
	ctx := context.Background()

	// Read proxy output config
	proxyCfg, _ := s.GetLogOutputForServer(ctx, serverID)
	if proxyCfg != nil {
		s.syncWriterFilePath("nginx", proxyCfg.Nginx.CapturePath)
		s.syncWriterFilePath("caddy", proxyCfg.Caddy.CapturePath)
		if err := s.StartWriterManaged("nginx", proxyCfg.Nginx.Mode, proxyCfg.Nginx.Managed); err != nil {
			log.Printf("logmanage: auto-start writer for nginx failed: %v", err)
		}
		if err := s.StartWriterManaged("caddy", proxyCfg.Caddy.Mode, proxyCfg.Caddy.Managed); err != nil {
			log.Printf("logmanage: auto-start writer for caddy failed: %v", err)
		}
	}

	// Read core output config
	coreCfg, _ := s.GetCoreLogOutputForServer(ctx, serverID)
	if coreCfg != nil {
		coreModes := map[string]struct {
			Mode        string
			Managed     bool
			CapturePath string
		}{
			"xray":      {coreCfg.Xray.Mode, coreCfg.Xray.Managed, coreCfg.Xray.CapturePath},
			"singbox":   {coreCfg.Singbox.Mode, coreCfg.Singbox.Managed, coreCfg.Singbox.CapturePath},
			"hysteria2": {coreCfg.Hysteria2.Mode, coreCfg.Hysteria2.Managed, coreCfg.Hysteria2.CapturePath},
			"tuic":      {coreCfg.Tuic.Mode, coreCfg.Tuic.Managed, coreCfg.Tuic.CapturePath},
		}
		for coreType, m := range coreModes {
			s.syncWriterFilePath(coreType, m.CapturePath)
			if err := s.StartWriterManaged(coreType, m.Mode, m.Managed); err != nil {
				log.Printf("logmanage: auto-start writer for %s failed: %v", coreType, err)
			}
		}
	}
}

// SetOnModeSwitch sets the callback for when output mode is switched.
func (s *Service) SetOnModeSwitch(fn func(proxyType string)) {
	s.onModeSwitch = fn
}

// Start starts the background scheduler with configurable cleanup intervals.
func (s *Service) Start(clientLogCleanupInterval, wafLogCleanupInterval time.Duration) {
	if clientLogCleanupInterval <= 0 {
		clientLogCleanupInterval = 10 * time.Minute
	}
	if wafLogCleanupInterval <= 0 {
		wafLogCleanupInterval = 24 * time.Hour
	}
	s.scheduler.Start()
	// Start expired client log package cleanup if configured
	if s.clientLogDeps != nil {
		go s.clientLogCleanupLoop(clientLogCleanupInterval)
	}
	// Start WAF log cleanup (daily, removes date-based WAF logs older than retain days)
	go s.wafLogCleanupLoop(wafLogCleanupInterval)
}

// Stop stops the background scheduler.
func (s *Service) Stop() {
	s.scheduler.Stop()
}

// --- Policy CRUD ---

// ListPolicies returns all rotation policies for the local node.
func (s *Service) ListPolicies(ctx context.Context) ([]entity.LogRotationPolicy, error) {
	return s.ListPoliciesForServer(ctx, 1)
}

// ListPoliciesForServer returns rotation policies for a specific server.
func (s *Service) ListPoliciesForServer(ctx context.Context, serverID int64) ([]entity.LogRotationPolicy, error) {
	return s.policyRepo.ListByServerID(ctx, serverID)
}

// GetPolicy returns a policy by ID.
func (s *Service) GetPolicy(ctx context.Context, id int64) (*entity.LogRotationPolicy, error) {
	return s.policyRepo.GetByID(ctx, id)
}

// CreatePolicy creates a new rotation policy.
func (s *Service) CreatePolicy(ctx context.Context, policy *entity.LogRotationPolicy) error {
	if policy.ServerID == 0 {
		policy.ServerID = 1 // default to local node
	}
	if policy.RotateCron == "" {
		policy.RotateCron = "0 0 * * *"
	}
	if policy.RetainDays <= 0 {
		policy.RetainDays = 30
	}
	err := s.policyRepo.Create(ctx, policy)
	if err == nil && policy.Enabled {
		s.scheduler.RefreshJob(policy.ID, true, policy.RotateCron)
	}
	return err
}

// UpdatePolicy updates an existing rotation policy.
func (s *Service) UpdatePolicy(ctx context.Context, policy *entity.LogRotationPolicy) error {
	err := s.policyRepo.Update(ctx, policy)
	if err == nil {
		s.scheduler.RefreshJob(policy.ID, policy.Enabled, policy.RotateCron)
	}
	return err
}

// DeletePolicy deletes a rotation policy.
func (s *Service) DeletePolicy(ctx context.Context, id int64) error {
	s.scheduler.RemoveJob(id)
	return s.policyRepo.Delete(ctx, id)
}

// --- Job History ---

// ListJobsByPolicy returns job history for a specific policy.
func (s *Service) ListJobsByPolicy(ctx context.Context, policyID int64, limit, offset int) ([]entity.LogRotationJob, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.jobRepo.ListByPolicyID(ctx, policyID, limit, offset)
}

// ListJobs returns all rotation jobs.
func (s *Service) ListJobs(ctx context.Context, limit, offset int) ([]entity.LogRotationJob, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.jobRepo.List(ctx, limit, offset)
}

// --- Log Writer Management ---

const settingKeyLogWriters = "proxy.log_writers"

// LogWriterStatus represents the status of a log writer.
type LogWriterStatus struct {
	ProxyType  string `json:"proxy_type"`
	Mode       string `json:"mode"`
	Managed    bool   `json:"managed"`
	Active     bool   `json:"active"`
	Paused     bool   `json:"paused"`
	FilePath   string `json:"file_path"`
	LogDir     string `json:"log_dir"`
	RetainDays int    `json:"retain_days"`
	Compress   bool   `json:"compress"`
	MaxSizeMB  int    `json:"max_size_mb"`
}

// StartWriter starts or updates the log writer for a proxy type.
// The mode is the actual output mode (stdout/file/off) and is saved as-is.
// The managed parameter controls whether the writer is active (writing to file)
// or paused. When mode=stdout but managed=false, the writer is created and
// subscribed to the broker (so stdout is still viewable in real-time), but
// file writing is paused.
func (s *Service) StartWriter(proxyType string, mode string) error {
	return s.StartWriterManaged(proxyType, mode, true)
}

// StartWriterManaged is like StartWriter but with an explicit managed flag.
// When managed=false and mode=stdout, the writer is paused (not writing to file)
// but still subscribed to the broker for real-time viewing.
func (s *Service) StartWriterManaged(proxyType string, mode string, managed bool) error {
	// Save config with the REAL mode (not EffectiveWriterMode)
	cfg := s.loadWriterConfig(proxyType)
	cfg.Mode = mode
	s.saveWriterConfig(proxyType, cfg)

	// Determine if the writer should be active
	shouldPause := mode != "stdout" || !managed

	// If writer already exists, just toggle pause state
	if w, ok := s.writers[proxyType]; ok && w.IsActive() {
		if shouldPause {
			w.SetPaused(true)
			log.Printf("logmanage: paused writer for %s (mode=%s, managed=%v)", proxyType, mode, managed)
		} else {
			w.SetPaused(false)
			log.Printf("logmanage: resumed writer for %s (mode=%s, managed=%v)", proxyType, mode, managed)
		}
		return nil
	}

	// Create new writer
	writer := NewLogWriter(cfg, s.dataDir)
	if err := writer.SetMode(mode); err != nil {
		return err
	}
	s.writers[proxyType] = writer

	// Always subscribe to LogBroker (keeps running even when paused)
	go s.subscribeToBroker(proxyType, writer)

	if shouldPause {
		writer.SetPaused(true)
	}

	log.Printf("logmanage: started writer for %s mode=%s managed=%v", proxyType, mode, managed)
	return nil
}

// syncWriterFilePath updates the writer config's file path from the output config's capture_path.
// If the writer is already active and the file path changed, it restarts the writer.
func (s *Service) syncWriterFilePath(proxyType string, capturePath string) {
	// Use default capture path if not specified
	if capturePath == "" {
		switch proxyType {
		case "nginx":
			capturePath = "logs/nginx-stdout.log"
		case "caddy":
			capturePath = "logs/caddy-stdout.log"
		case "xray":
			capturePath = "logs/xray-stdout.log"
		case "singbox":
			capturePath = "logs/singbox-stdout.log"
		case "hysteria2":
			capturePath = "logs/hysteria2-stdout.log"
		case "tuic":
			capturePath = "logs/tuic-stdout.log"
		default:
			return
		}
	}
	cfg := s.loadWriterConfig(proxyType)
	if cfg.FilePath == capturePath {
		return // already in sync
	}
	cfg.FilePath = capturePath
	s.saveWriterConfig(proxyType, cfg)

	// If writer is active, restart it to pick up the new file path
	if w, ok := s.writers[proxyType]; ok && w.IsActive() {
		w.Stop()
		delete(s.writers, proxyType)
		if err := s.StartWriter(proxyType, cfg.Mode); err != nil {
			log.Printf("logmanage: failed to restart writer for %s after file path change: %v", proxyType, err)
		}
	}
}

// subscribeToBroker subscribes to the proxy/core LogBroker and writes lines to the writer.
// Keeps retrying and re-subscribing as long as the writer exists.
func (s *Service) subscribeToBroker(proxyType string, writer *LogWriter) {
	// Determine if this is a core type or proxy type
	isCore := false
	for _, ct := range coreLogTypes {
		if proxyType == ct {
			isCore = true
			break
		}
	}

	for {
		var broker LogBrokerReader

		if isCore {
			// Core type: use coreBroker
			if s.coreBroker == nil {
				time.Sleep(2 * time.Second)
				continue
			}
			broker = s.coreBroker.GetCoreLogBroker(proxyType)
		} else {
			// Proxy type: use proxyBroker
			if s.proxyBroker == nil {
				time.Sleep(2 * time.Second)
				continue
			}
			broker = s.proxyBroker.GetLogBroker(proxyType)
		}

		if broker == nil {
			time.Sleep(2 * time.Second)
			continue
		}

		// Write existing snapshot to file first (catches up on lines produced
		// before the writer subscribed, e.g. process startup output)
		for _, line := range broker.Snapshot() {
			writer.WriteLine(line)
		}

		ch, cancel := broker.Subscribe()
		writer.SetCancel(cancel)
		log.Printf("logmanage: subscribed to %s log broker", proxyType)

		// Read lines until channel closes
		for line := range ch {
			writer.WriteLine(line) // respects pause state internally
		}

		// Channel closed (broker restarted), re-subscribe
		cancel()
		log.Printf("logmanage: %s broker channel closed, re-subscribing...", proxyType)
		time.Sleep(1 * time.Second)
	}
}

// StopWriter stops the log writer for a proxy type.
func (s *Service) StopWriter(proxyType string) error {
	writer, ok := s.writers[proxyType]
	if !ok {
		return fmt.Errorf("writer for %s not found", proxyType)
	}
	writer.Stop()
	delete(s.writers, proxyType)

	// Update config
	cfg := s.loadWriterConfig(proxyType)
	cfg.ProxyType = proxyType
	s.saveWriterConfig(proxyType, cfg)

	return nil
}

// GetWriterStatus returns the status of all log writers.
// The mode and managed fields are read from the actual LogOutputConfig / CoreLogOutputConfig
// (not the writer's internal config) so they always reflect the real process output mode.
func (s *Service) GetWriterStatus() []LogWriterStatus {
	// Read real modes from output configs
	proxyCfg, _ := s.GetLogOutput(context.Background())
	coreCfg, _ := s.GetCoreLogOutput(context.Background())

	realModes := map[string]struct {
		Mode    string
		Managed bool
	}{}
	if proxyCfg != nil {
		realModes["nginx"] = struct {
			Mode    string
			Managed bool
		}{proxyCfg.Nginx.Mode, proxyCfg.Nginx.Managed}
		realModes["caddy"] = struct {
			Mode    string
			Managed bool
		}{proxyCfg.Caddy.Mode, proxyCfg.Caddy.Managed}
	}
	if coreCfg != nil {
		realModes["xray"] = struct {
			Mode    string
			Managed bool
		}{coreCfg.Xray.Mode, coreCfg.Xray.Managed}
		realModes["singbox"] = struct {
			Mode    string
			Managed bool
		}{coreCfg.Singbox.Mode, coreCfg.Singbox.Managed}
		realModes["hysteria2"] = struct {
			Mode    string
			Managed bool
		}{coreCfg.Hysteria2.Mode, coreCfg.Hysteria2.Managed}
		realModes["tuic"] = struct {
			Mode    string
			Managed bool
		}{coreCfg.Tuic.Mode, coreCfg.Tuic.Managed}
	}

	var statuses []LogWriterStatus
	for _, pType := range allLogTypes {
		// Writer-specific config (file rotation settings)
		wCfg := s.loadWriterConfig(pType)

		// Real mode and managed from output config
		rm := realModes[pType]
		mode := rm.Mode
		if mode == "" {
			mode = "file"
		}
		managed := rm.Managed

		active := false
		paused := true
		if w, ok := s.writers[pType]; ok {
			active = w.IsActive()
			paused = w.IsPaused()
		}
		// If no writer exists, derive paused from mode+managed
		if _, ok := s.writers[pType]; !ok {
			paused = mode != "stdout" || !managed
		}

		logDir := filepath.Join(s.dataDir, "logs")
		filePath := wCfg.FilePath
		if filePath == "" {
			filePath = fmt.Sprintf("logs/%s-stdout.log", pType)
		}
		if w, ok := s.writers[pType]; ok {
			logDir = w.GetLogDir()
			filePath = w.GetFilePath()
		}
		statuses = append(statuses, LogWriterStatus{
			ProxyType:  pType,
			Mode:       mode,
			Managed:    managed,
			Active:     active,
			Paused:     paused,
			FilePath:   filePath,
			LogDir:     logDir,
			RetainDays: wCfg.RetainDays,
			Compress:   wCfg.Compress,
			MaxSizeMB:  wCfg.MaxSizeMB,
		})
	}
	return statuses
}

// UpdateWriterConfig updates the writer configuration and restarts if active.
func (s *Service) UpdateWriterConfig(proxyType string, cfg LogWriterConfig) error {
	cfg.ProxyType = proxyType

	// If mode is empty, load from saved config
	if cfg.Mode == "" {
		saved := s.loadWriterConfig(proxyType)
		cfg.Mode = saved.Mode
	}
	// If file_path is empty, keep the existing one
	if cfg.FilePath == "" {
		saved := s.loadWriterConfig(proxyType)
		cfg.FilePath = saved.FilePath
	}

	s.saveWriterConfig(proxyType, cfg)

	// Restart writer if it was active
	if w, ok := s.writers[proxyType]; ok && w.IsActive() {
		w.Stop()
		delete(s.writers, proxyType)
		return s.StartWriter(proxyType, cfg.Mode)
	}
	return nil
}

// ListWriterFiles returns the log files for a proxy type's writer.
func (s *Service) ListWriterFiles(proxyType string) []LogFileInfo {
	w, ok := s.writers[proxyType]
	if !ok {
		return nil
	}
	return w.ListLogFiles()
}

func (s *Service) loadWriterConfig(proxyType string) LogWriterConfig {
	cfg := LogWriterConfig{
		ProxyType:  proxyType,
		Mode:       "file",
		FilePath:   fmt.Sprintf("logs/%s-stdout.log", proxyType),
		RetainDays: 30,
		Compress:   false,
		MaxSizeMB:  100,
	}
	if s.settingStore == nil {
		return cfg
	}
	key := settingKeyLogWriters + "." + proxyType
	v, err := s.settingStore.GetByKey(context.Background(), key)
	if err != nil || v == "" {
		return cfg
	}
	var saved LogWriterConfig
	if json.Unmarshal([]byte(v), &saved) == nil {
		if saved.Mode != "" {
			cfg.Mode = saved.Mode
		}
		if saved.FilePath != "" {
			cfg.FilePath = saved.FilePath
		}
		if saved.RetainDays > 0 {
			cfg.RetainDays = saved.RetainDays
		}
		if saved.MaxSizeMB > 0 {
			cfg.MaxSizeMB = saved.MaxSizeMB
		}
		cfg.Compress = saved.Compress
	}
	return cfg
}

func (s *Service) saveWriterConfig(proxyType string, cfg LogWriterConfig) {
	if s.settingStore == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	s.settingStore.Set(context.Background(), settingKeyLogWriters+"."+proxyType, string(data))
}

// --- Discovery ---

// DiscoveredLog represents a discovered log file.
type DiscoveredLog struct {
	SourceType string `json:"source_type"`
	Domain     string `json:"domain,omitempty"`
	LogPath    string `json:"log_path"`
	LogFormat  string `json:"log_format,omitempty"`
	Exists     bool   `json:"exists"`
	HasPolicy  bool   `json:"has_policy"`
}

// DiscoverLogs discovers all configured log files from core, nginx and caddy settings.
func (s *Service) DiscoverLogs(ctx context.Context) ([]DiscoveredLog, error) {
	return s.DiscoverLogsForServer(ctx, 1)
}

// DiscoverLogsForServer discovers log files for a specific server.
// For local node (serverID=1), reads from local settings.
// For remote nodes, returns an empty list (remote discovery is done via gRPC by the handler).
func (s *Service) DiscoverLogsForServer(ctx context.Context, serverID int64) ([]DiscoveredLog, error) {
	if serverID > 1 {
		// Remote discovery is handled by the handler via gRPC
		return []DiscoveredLog{}, nil
	}
	return s.discoverLocalLogs(ctx)
}

// discoverLocalLogs discovers all configured log files from core, nginx and caddy settings.
func (s *Service) discoverLocalLogs(ctx context.Context) ([]DiscoveredLog, error) {
	var logs []DiscoveredLog

	// Discover core process logs (xray, singbox, hysteria2, tuic)
	coreLogs, err := s.discoverCoreLogs(ctx)
	if err != nil {
		log.Printf("logmanage: failed to discover core logs: %v", err)
	} else {
		logs = append(logs, coreLogs...)
	}

	// Discover nginx logs from settings
	nginxLogs, err := s.discoverNginxLogs(ctx)
	if err != nil {
		log.Printf("logmanage: failed to discover nginx logs: %v", err)
	} else {
		logs = append(logs, nginxLogs...)
	}

	// Discover caddy logs (unified access log file)
	caddyLogs, err := s.discoverCaddyLogs(ctx)
	if err != nil {
		log.Printf("logmanage: failed to discover caddy logs: %v", err)
	} else {
		logs = append(logs, caddyLogs...)
	}

	// Discover WAF logs (date-based, e.g. logs/2026-08-25_waf.log)
	wafLogs, err := s.discoverWAFLogs(ctx)
	if err != nil {
		log.Printf("logmanage: failed to discover WAF logs: %v", err)
	} else {
		logs = append(logs, wafLogs...)
	}

	// Cross-check with existing local policies
	existingPolicies, err := s.policyRepo.ListByServerID(ctx, 1)
	if err == nil {
		policyPaths := make(map[string]bool)
		for _, p := range existingPolicies {
			policyPaths[p.LogPath] = true
		}
		for i := range logs {
			if policyPaths[logs[i].LogPath] {
				logs[i].HasPolicy = true
			}
		}
	}

	return logs, nil
}

// discoverCoreLogs discovers log files for core processes (xray, singbox, hysteria2, tuic).
// Uses the actual log mode derived from the runtime config (not the stale
// log_output_config) to decide which logs to discover.
func (s *Service) discoverCoreLogs(ctx context.Context) ([]DiscoveredLog, error) {
	var logs []DiscoveredLog

	for _, coreType := range coreLogTypes {
		logMode := s.GetCoreLogMode(ctx, coreType)

		switch logMode {
		case "stdout":
			continue
		case "off":
			continue
		default: // "file"
			if coreType == "xray" {
				accessPath, errorPath := s.readXrayRuntimeLogPaths()
				if errorPath != "" {
					logs = append(logs, DiscoveredLog{
						SourceType: coreType,
						Domain:     "error",
						LogPath:    errorPath,
					})
				}
				if accessPath != "" {
					logs = append(logs, DiscoveredLog{
						SourceType: coreType,
						Domain:     "access",
						LogPath:    accessPath,
					})
				}
			} else {
				logs = append(logs, DiscoveredLog{
					SourceType: coreType,
					Domain:     "file",
					LogPath:    s.resolveLogPath(fmt.Sprintf("logs/%s.log", coreType)),
				})
			}
		}
	}

	return logs, nil
}

// readXrayRuntimeLogPaths reads the access and error log paths from the local
// xray runtime config file (dataDir/runtime/xray.jsonc).
// Returns resolved absolute paths; empty string if the log is disabled ("none").
func (s *Service) readXrayRuntimeLogPaths() (accessPath, errorPath string) {
	// Defaults
	accessPath = s.resolveLogPath("logs/xray-access.log")
	errorPath = s.resolveLogPath("logs/xray.log")

	configPath := filepath.Join(s.dataDir, "runtime", "xray.jsonc")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	cleaned := stripJSONCCommentsGo(string(data))
	var cfg struct {
		Log struct {
			Access string `json:"access"`
			Error  string `json:"error"`
		} `json:"log"`
	}
	if json.Unmarshal([]byte(cleaned), &cfg) != nil {
		return
	}

	if cfg.Log.Access != "" && cfg.Log.Access != "none" {
		accessPath = s.resolveLogPath(cfg.Log.Access)
	} else {
		accessPath = ""
	}
	if cfg.Log.Error != "" && cfg.Log.Error != "none" {
		errorPath = s.resolveLogPath(cfg.Log.Error)
	} else {
		errorPath = ""
	}
	return
}

// stripJSONCCommentsGo removes // line comments and /* block comments */ from
// JSONC content so it can be parsed as standard JSON.
func stripJSONCCommentsGo(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
		} else if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			i += 2
		} else if s[i] == '"' {
			b.WriteByte(s[i])
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					b.WriteByte(s[i])
					b.WriteByte(s[i+1])
					i += 2
				} else {
					b.WriteByte(s[i])
					i++
				}
			}
			if i < len(s) {
				b.WriteByte(s[i])
				i++
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// discoverNginxLogs reads log paths from nginx settings.
// Skips discovery when the proxy log output mode is stdout.
func (s *Service) discoverNginxLogs(ctx context.Context) ([]DiscoveredLog, error) {
	if s.settingStore == nil {
		return nil, nil
	}

	// Check log output mode
	logMode := "file"
	if v, err := s.settingStore.GetByKey(ctx, "proxy.log_output_config"); err == nil && v != "" {
		var cfg struct {
			Nginx struct {
				Mode string `json:"mode"`
			} `json:"nginx"`
		}
		if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Nginx.Mode != "" {
			logMode = cfg.Nginx.Mode
		}
	}
	if logMode == "stdout" {
		return nil, nil
	}

	v, err := s.settingStore.GetByKey(ctx, "proxy.nginx_settings")
	if err != nil || v == "" {
		return nil, nil
	}

	// Parse nginx settings to extract log paths
	var settings struct {
		ErrorLog      bool   `json:"error_log"`
		ErrorLogPath  string `json:"error_log_path"`
		AccessLog     bool   `json:"access_log"`
		AccessLogPath string `json:"access_log_path"`
		StreamLog     bool   `json:"stream_log"`
		StreamLogPath string `json:"stream_log_path"`
	}
	if err := json.Unmarshal([]byte(v), &settings); err != nil {
		return nil, err
	}

	var logs []DiscoveredLog
	if settings.ErrorLog && settings.ErrorLogPath != "" {
		logs = append(logs, DiscoveredLog{
			SourceType: "nginx",
			Domain:     "error",
			LogPath:    s.resolveLogPath(settings.ErrorLogPath),
		})
	}
	if settings.AccessLog && settings.AccessLogPath != "" {
		logs = append(logs, DiscoveredLog{
			SourceType: "nginx",
			Domain:     "access",
			LogPath:    s.resolveLogPath(settings.AccessLogPath),
		})
	}
	if settings.StreamLog && settings.StreamLogPath != "" {
		logs = append(logs, DiscoveredLog{
			SourceType: "nginx",
			Domain:     "stream",
			LogPath:    s.resolveLogPath(settings.StreamLogPath),
		})
	}
	return logs, nil
}

// discoverCaddyLogs discovers the unified Caddy access log file.
// All sites share a single log file (logs/caddy_access.log) via a named logger.
// Per-site custom log paths are also discovered if configured.
func (s *Service) discoverCaddyLogs(ctx context.Context) ([]DiscoveredLog, error) {
	if s.caddySiteRepo == nil {
		return nil, nil
	}

	// Check log output mode
	logMode := "file"
	if s.settingStore != nil {
		if v, err := s.settingStore.GetByKey(ctx, "proxy.log_output_config"); err == nil && v != "" {
			var cfg struct {
				Caddy struct {
					Mode string `json:"mode"`
				} `json:"caddy"`
			}
			if json.Unmarshal([]byte(v), &cfg) == nil && cfg.Caddy.Mode != "" {
				logMode = cfg.Caddy.Mode
			}
		}
	}
	if logMode == "off" {
		return nil, nil
	}

	var logs []DiscoveredLog

	// Unified access log file
	logs = append(logs, DiscoveredLog{
		SourceType: "caddy",
		Domain:     "access",
		LogPath:    s.resolveLogPath("logs/caddy_access.log"),
		LogFormat:  "json",
	})

	// Discover per-site custom log paths (sites that override the global logger)
	sites, err := s.caddySiteRepo.List(ctx, 1)
	if err == nil {
		seen := make(map[string]bool)
		for _, site := range sites {
			if site.Template == "l4" || site.Template == "sni" {
				continue
			}
			if !site.LogEnabled || site.LogPath == "" {
				continue
			}
			path := s.resolveLogPath(site.LogPath)
			if seen[path] {
				continue
			}
			seen[path] = true
			logs = append(logs, DiscoveredLog{
				SourceType: "caddy",
				Domain:     site.Domain,
				LogPath:    path,
				LogFormat:  site.LogFormat,
			})
		}
	}

	return logs, nil
}

// discoverWAFLogs discovers WAF log files.
// WAF writes logs by date: logs/<date>_waf.log (e.g. logs/2026-08-25_waf.log).
// We discover the current day's log file.
func (s *Service) discoverWAFLogs(ctx context.Context) ([]DiscoveredLog, error) {
	today := time.Now().Format("2006-01-02")
	logPath := fmt.Sprintf("logs/%s_waf.log", today)
	return []DiscoveredLog{
		{
			SourceType: "waf",
			Domain:     "waf",
			LogPath:    s.resolveLogPath(logPath),
			LogFormat:  "json",
		},
	}, nil
}

// sanitizeDomain converts a domain to a safe log file name.
func sanitizeDomain(domain string) string {
	if domain == "" || domain == "_" || domain == ":80" || domain == ":443" {
		return "default"
	}
	if strings.HasPrefix(domain, ":") {
		return "port" + domain[1:]
	}
	return domain
}

// SyncPolicies creates policies for discovered logs on the local node.
func (s *Service) SyncPolicies(ctx context.Context) (int, error) {
	return s.SyncPoliciesForServer(ctx, 1)
}

// SyncPoliciesForServer creates policies for discovered logs on a specific server.
func (s *Service) SyncPoliciesForServer(ctx context.Context, serverID int64) (int, error) {
	discovered, err := s.DiscoverLogsForServer(ctx, serverID)
	if err != nil {
		return 0, err
	}

	created := 0
	for _, d := range discovered {
		// Check if policy already exists for this log path on this server
		_, err := s.policyRepo.GetByLogPathAndServer(ctx, d.LogPath, serverID)
		if err == nil {
			continue // already exists
		}

		// Create new policy with defaults
		policy := &entity.LogRotationPolicy{
			ServerID:   serverID,
			Name:       filepath.Base(d.LogPath),
			SourceType: d.SourceType,
			LogPath:    d.LogPath,
			Enabled:    true,
			RotateCron: "0 0 * * *", // daily midnight
			RetainDays: 30,
		}
		if err := s.policyRepo.Create(ctx, policy); err != nil {
			log.Printf("logmanage: failed to create policy for %s: %v", d.LogPath, err)
			continue
		}
		// Only schedule local jobs (remote jobs are handled by the agent)
		if serverID <= 1 {
			s.scheduler.RefreshJob(policy.ID, true, policy.RotateCron)
		}
		created++
	}
	return created, nil
}

// Status returns an overview of the log management system.
type Status struct {
	TotalPolicies  int   `json:"total_policies"`
	EnabledCount   int   `json:"enabled_count"`
	TotalJobs      int64 `json:"total_jobs"`
	SuccessJobs    int64 `json:"success_jobs"`
	FailedJobs     int64 `json:"failed_jobs"`
	ScheduledCount int   `json:"scheduled_count"`
}

// GetStatus returns the log management system status for the local node.
func (s *Service) GetStatus(ctx context.Context) (*Status, error) {
	return s.GetStatusForServer(ctx, 1)
}

// GetStatusForServer returns the log management system status for a specific server.
func (s *Service) GetStatusForServer(ctx context.Context, serverID int64) (*Status, error) {
	policies, err := s.policyRepo.ListByServerID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	status := &Status{
		TotalPolicies: len(policies),
	}
	for _, p := range policies {
		if p.Enabled {
			status.EnabledCount++
			// A policy is "scheduled" if it's enabled and has a rotate cron
			if p.RotateCron != "" {
				status.ScheduledCount++
			}
		}
	}
	return status, nil
}

// GetProxyStdoutLog returns the last N lines from a process's stdout broker.
// Works for both proxy (nginx/caddy) and core (xray/singbox/hysteria2/tuic) processes.
func (s *Service) GetProxyStdoutLog(ctx context.Context, logType string, lastLines int) ([]string, error) {
	// Try proxy broker first
	if s.proxyBroker != nil {
		broker := s.proxyBroker.GetLogBroker(logType)
		if broker != nil {
			lines := broker.Snapshot()
			if len(lines) > lastLines {
				lines = lines[len(lines)-lastLines:]
			}
			return lines, nil
		}
	}
	// Try core broker
	if s.coreBroker != nil {
		broker := s.coreBroker.GetCoreLogBroker(logType)
		if broker != nil {
			lines := broker.Snapshot()
			if len(lines) > lastLines {
				lines = lines[len(lines)-lastLines:]
			}
			return lines, nil
		}
	}
	return []string{fmt.Sprintf("[%s 未运行或未产生标准输出]", logType)}, nil
}

// StreamProxyStdout streams a process's stdout via the broker's pub/sub.
// Works for both proxy and core processes.
func (s *Service) StreamProxyStdout(ctx context.Context, logType string, ch chan<- string) {
	defer close(ch)

	var broker LogBrokerReader
	if s.proxyBroker != nil {
		broker = s.proxyBroker.GetLogBroker(logType)
	}
	if broker == nil && s.coreBroker != nil {
		broker = s.coreBroker.GetCoreLogBroker(logType)
	}
	if broker == nil {
		return
	}
	liveCh, cancel := broker.Subscribe()
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-liveCh:
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				return
			case ch <- line:
			}
		}
	}
}

// resolveLogPath resolves a relative log path against dataDir.
func (s *Service) resolveLogPath(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	// Avoid double-prefixing if path already starts with dataDir
	if strings.HasPrefix(path, s.dataDir+string(filepath.Separator)) {
		return path
	}
	return filepath.Join(s.dataDir, path)
}

// wafLogCleanupLoop runs at the given interval to clean up expired WAF log files.
// WAF logs are date-based: logs/<date>_waf.log (e.g. logs/2026-08-25_waf.log).
// Retain days are read from the "waf.log_retain_days" setting (default 30).
func (s *Service) wafLogCleanupLoop(interval time.Duration) {
	// Run once at startup, then on interval
	s.cleanExpiredWAFLogs()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		s.cleanExpiredWAFLogs()
	}
}

// cleanExpiredWAFLogs scans the logs directory for date-based WAF log files
// and removes those older than the configured retain days.
func (s *Service) cleanExpiredWAFLogs() {
	retainDays := 30
	if s.settingStore != nil {
		// Try scheduler.waf_log_retain_days first, fall back to legacy waf.log_retain_days
		for _, key := range []string{"scheduler.waf_log_retain_days", "waf.log_retain_days"} {
			if v, err := s.settingStore.GetByKey(context.Background(), key); err == nil && v != "" {
				if d, err := strconv.Atoi(v); err == nil && d > 0 {
					retainDays = d
					break
				}
			}
		}
	}

	logDir := filepath.Join(s.dataDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retainDays)
	cleaned := 0
	for _, entry := range entries {
		name := entry.Name()
		// Match: <date>_waf.log or <date>_waf.log.old
		if !strings.HasSuffix(name, "_waf.log") && !strings.HasSuffix(name, "_waf.log.old") {
			continue
		}
		// Extract date prefix: "2026-08-25" from "2026-08-25_waf.log"
		datePart := name[:10] // first 10 chars = date
		fileDate, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if fileDate.Before(cutoff) {
			fullPath := filepath.Join(logDir, name)
			if err := os.Remove(fullPath); err != nil {
				log.Printf("logmanage: failed to delete WAF log %s: %v", fullPath, err)
				continue
			}
			cleaned++
		}
	}
	if cleaned > 0 {
		log.Printf("logmanage: cleaned %d expired WAF log files (retain=%dd)", cleaned, retainDays)
	}
}
