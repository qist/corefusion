package logmanage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
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
	proxyLogTypes = []string{"nginx", "caddy"}
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

// StartAutoWriters starts writers that were previously enabled.
func (s *Service) StartAutoWriters() {
	for _, pType := range allLogTypes {
		key := settingKeyLogWriters + "." + pType
		if s.settingStore == nil {
			continue
		}
		v, err := s.settingStore.GetByKey(context.Background(), key)
		if err != nil || v == "" {
			continue
		}
		// If config exists, the writer was previously enabled
		cfg := s.loadWriterConfig(pType)
		if err := s.StartWriter(pType, cfg.Mode); err != nil {
			log.Printf("logmanage: auto-start writer for %s failed: %v", pType, err)
		}
	}
}

// SetOnModeSwitch sets the callback for when output mode is switched.
func (s *Service) SetOnModeSwitch(fn func(proxyType string)) {
	s.onModeSwitch = fn
}

// Start starts the background scheduler.
func (s *Service) Start() {
	s.scheduler.Start()
	// Start expired client log package cleanup if configured
	if s.clientLogDeps != nil {
		go s.clientLogCleanupLoop()
	}
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
	Active     bool   `json:"active"`
	Paused     bool   `json:"paused"`
	FilePath   string `json:"file_path"`
	LogDir     string `json:"log_dir"`
	RetainDays int    `json:"retain_days"`
	Compress   bool   `json:"compress"`
	MaxSizeMB  int    `json:"max_size_mb"`
}

// StartWriter starts or updates the log writer for a proxy type.
// If already active, just toggles pause state instead of restarting.
func (s *Service) StartWriter(proxyType string, mode string) error {
	// Save config
	cfg := s.loadWriterConfig(proxyType)
	cfg.Mode = mode
	s.saveWriterConfig(proxyType, cfg)

	// If writer already exists, just toggle pause
	if w, ok := s.writers[proxyType]; ok && w.IsActive() {
		if mode == "stdout" {
			w.SetPaused(false)
			log.Printf("logmanage: resumed writer for %s", proxyType)
		} else {
			w.SetPaused(true)
			log.Printf("logmanage: paused writer for %s (mode=%s)", proxyType, mode)
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

	if mode != "stdout" {
		writer.SetPaused(true)
	}

	log.Printf("logmanage: started writer for %s mode=%s", proxyType, mode)
	return nil
}

// subscribeToBroker subscribes to the proxy's LogBroker and writes lines to the writer.
// Keeps retrying and re-subscribing as long as the writer exists.
func (s *Service) subscribeToBroker(proxyType string, writer *LogWriter) {
	for {
		if s.proxyBroker == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		broker := s.proxyBroker.GetLogBroker(proxyType)
		if broker == nil {
			time.Sleep(2 * time.Second)
			continue
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
func (s *Service) GetWriterStatus() []LogWriterStatus {
	var statuses []LogWriterStatus
	for _, pType := range allLogTypes {
		cfg := s.loadWriterConfig(pType)
		mode := cfg.Mode
		if mode == "" {
			mode = "file"
		}
		active := false
		paused := true
		if w, ok := s.writers[pType]; ok {
			active = w.IsActive()
			paused = w.IsPaused()
		}
		logDir := filepath.Join(s.dataDir, "logs")
		filePath := filepath.Join("logs", pType+"-stdout.log")
		if w, ok := s.writers[pType]; ok {
			logDir = w.GetLogDir()
		}
		statuses = append(statuses, LogWriterStatus{
			ProxyType:  pType,
			Mode:       mode,
			Active:     active,
			Paused:     paused,
			FilePath:   filePath,
			LogDir:     logDir,
			RetainDays: cfg.RetainDays,
			Compress:   cfg.Compress,
			MaxSizeMB:  cfg.MaxSizeMB,
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

	// Discover caddy logs from site configs
	caddyLogs, err := s.discoverCaddyLogs(ctx)
	if err != nil {
		log.Printf("logmanage: failed to discover caddy logs: %v", err)
	} else {
		logs = append(logs, caddyLogs...)
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
// Each core process writes to data/logs/{coreType}.log in file mode,
// or data/logs/{coreType}-stdout.log in stdout mode.
func (s *Service) discoverCoreLogs(ctx context.Context) ([]DiscoveredLog, error) {
	var logs []DiscoveredLog

	for _, coreType := range coreLogTypes {
		logMode := s.GetCoreLogMode(ctx, coreType)

		var logPath string
		switch logMode {
		case "stdout":
			logPath = s.resolveLogPath(fmt.Sprintf("logs/%s-stdout.log", coreType))
		case "off":
			continue // no log file for this core
		default: // "file"
			logPath = s.resolveLogPath(fmt.Sprintf("logs/%s.log", coreType))
		}

		logs = append(logs, DiscoveredLog{
			SourceType: coreType,
			Domain:     logMode,
			LogPath:    logPath,
		})
	}

	return logs, nil
}

// discoverNginxLogs reads log paths from nginx settings.
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

	// In stdout mode, all nginx logs go to nginx-stdout.log
	if logMode == "stdout" {
		return []DiscoveredLog{
			{SourceType: "nginx", Domain: "stdout", LogPath: s.resolveLogPath("logs/nginx-stdout.log")},
		}, nil
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

// discoverCaddyLogs reads log paths from caddy site configs.
// Caddy collects logs per-domain. If LogPath is empty, it defaults to logs/<domain>.log.
// L4 and SNI sites are excluded as they don't produce HTTP access logs.
func (s *Service) discoverCaddyLogs(ctx context.Context) ([]DiscoveredLog, error) {
	if s.caddySiteRepo == nil {
		return nil, nil
	}
	sites, err := s.caddySiteRepo.List(ctx, 1)
	if err != nil {
		return nil, err
	}

	var logs []DiscoveredLog
	for _, site := range sites {
		// Skip L4 and SNI sites - they don't have HTTP-level logs
		if site.Template == "l4" || site.Template == "sni" {
			continue
		}
		if !site.LogEnabled {
			continue
		}
		logPath := site.LogPath
		if logPath == "" {
			logPath = fmt.Sprintf("logs/%s.log", sanitizeDomain(site.Domain))
		}
		logs = append(logs, DiscoveredLog{
			SourceType: "caddy",
			Domain:     site.Domain,
			LogPath:    s.resolveLogPath(logPath),
			LogFormat:  site.LogFormat,
		})
	}
	return logs, nil
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
		TotalPolicies:  len(policies),
		ScheduledCount: s.scheduler.ActiveJobs(),
	}
	for _, p := range policies {
		if p.Enabled {
			status.EnabledCount++
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
