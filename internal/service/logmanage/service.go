package logmanage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
	"corefusion/internal/util/safepath"
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
	coreLogTypes = []string{"xray", "singbox"}
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

func (w *LogBrokerWrapper) Snapshot() []string                 { return w.SnapshotFn() }
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

// ProcessAvailability reports whether a log producer is actually usable on this
// node (binary installed and process running). Discovery uses it so logs of
// uninstalled/stopped cores and proxies are not offered to the user.
type ProcessAvailability interface {
	Available(sourceType string) bool
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
	availability  ProcessAvailability
	onModeSwitch  func(proxyType string) // callback when output mode is switched
	clientLogDeps *ClientLogDeps         // optional: for client log packaging
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
	}
	svc.scheduler = NewScheduler(policyRepo, jobRepo, svc)
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

// SetProcessAvailability sets the provider used to filter discovery by whether
// the core/proxy process is actually installed and running.
func (s *Service) SetProcessAvailability(p ProcessAvailability) {
	s.availability = p
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

// --- Log Source Status ---

// LogWriterStatus 描述一个日志源当前的状态（类型 + 模式 + 文件）。
// 命名沿用历史（前端字段依赖），但 LogWriter（stdout 落盘）已下线：
// 进程一律写真实文件，由 LogBroker tail + 轮转策略管理。
type LogWriterStatus struct {
	ProxyType string `json:"proxy_type"`
	Mode      string `json:"mode"`
	FilePath  string `json:"file_path"`
	LogDir    string `json:"log_dir"`
}

// GetWriterStatus 返回各日志源当前模式与文件路径，供「实时日志」列表使用。
func (s *Service) GetWriterStatus() []LogWriterStatus {
	ctx := context.Background()
	proxyCfg, _ := s.GetLogOutput(ctx)
	coreCfg, _ := s.GetCoreLogOutput(ctx)

	modes := make(map[string]string)
	if proxyCfg != nil {
		modes["nginx"] = proxyCfg.Nginx.Mode
		modes["caddy"] = proxyCfg.Caddy.Mode
	}
	if coreCfg != nil {
		modes["xray"] = coreCfg.Xray.Mode
		modes["singbox"] = coreCfg.Singbox.Mode
	}

	logDir := filepath.Join(s.dataDir, "logs")
	statuses := make([]LogWriterStatus, 0, len(allLogTypes))
	for _, pType := range allLogTypes {
		mode := modes[pType]
		if mode == "" {
			mode = "file"
		}
		statuses = append(statuses, LogWriterStatus{
			ProxyType: pType,
			Mode:      mode,
			FilePath:  s.logFileForSource(pType),
			LogDir:    logDir,
		})
	}
	return statuses
}

// logFileForSource 返回日志源默认的落盘文件（仅用于界面展示）。
func (s *Service) logFileForSource(sourceType string) string {
	switch sourceType {
	case "nginx":
		return s.resolveLogPath("logs/access.log")
	case "caddy":
		return s.resolveLogPath("logs/caddy_access.log")
	case "waf":
		return s.resolveLogPath(fmt.Sprintf("logs/%s_waf.log", time.Now().Format("2006-01-02")))
	default:
		return s.resolveLogPath(fmt.Sprintf("logs/%s.log", sourceType))
	}
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

	// Discover core process logs (xray, singbox)
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

// discoverCoreLogs discovers log files for core processes (xray, singbox).
// Uses the actual log mode derived from the runtime config (not the stale
// log_output_config) to decide which logs to discover.
func (s *Service) discoverCoreLogs(ctx context.Context) ([]DiscoveredLog, error) {
	var logs []DiscoveredLog

	for _, coreType := range coreLogTypes {
		// 未安装或未运行的核心不会产生日志，不要去"发现"它的历史默认路径。
		if s.availability != nil && !s.availability.Available(coreType) {
			continue
		}
		logMode := s.GetCoreLogMode(ctx, coreType)

		switch logMode {
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
				// 进程自身 stdout/stderr（logs/xray.log）：启动横幅、配置解析失败、
				// 端口占用这类"起不来"的原因只落在这里 —— xray 自己的 error 日志要
				// 先跑起来才会写 ✗。以前这里只报 access/error 两个文件，前端既发现
				// 不到也看不了启动错误 ✓（真机：xray.log 一直没人看 ✓）。
				if p := s.resolveLogPath(fmt.Sprintf("logs/%s.log", coreType)); p != accessPath && p != errorPath {
					logs = append(logs, DiscoveredLog{
						SourceType: coreType,
						Domain:     "startup",
						LogPath:    p,
					})
				}
			} else {
				// sing-box 的进程输出就是 logs/singbox.log ✓（与 xray 的 startup 同一个含义 ✓）
				logs = append(logs, DiscoveredLog{
					SourceType: coreType,
					Domain:     "startup",
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
	if s.availability != nil && !s.availability.Available("nginx") {
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
	// off 模式 nginx 不写日志（error_log /dev/null），不参与发现/轮转。
	if normalizeLogMode(logMode) == "off" {
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
	if s.availability != nil && !s.availability.Available("caddy") {
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
	// caddy 进程 stdout/stderr：启动错误、端口占用、配置解析失败只会落在这里，
	// 不纳入发现就没人看得到"起不来"的原因。
	logs = append(logs, DiscoveredLog{
		SourceType: "caddy",
		Domain:     "process",
		LogPath:    s.resolveLogPath("logs/caddy-process.log"),
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
// WAF writes logs by date: logs/<date>_waf.log (e.g. logs/2026-08-25_waf.log),
// i.e. the WAF engine itself splits logs per day. We therefore enumerate the
// files that actually exist on disk (newest first) instead of pointing at a
// fixed "today" path — this exposes every retained day to the log viewer.
// WAF files are never given rotation policies (see CreatePoliciesFromDiscovered);
// retention is handled by cleanExpiredWAFLogs.
func (s *Service) discoverWAFLogs(ctx context.Context) ([]DiscoveredLog, error) {
	logDir := filepath.Join(s.dataDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, nil
	}

	type wafFile struct {
		date string
		path string
	}
	var files []wafFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, "_waf.log") {
			continue
		}
		datePart := strings.TrimSuffix(name, "_waf.log")
		if _, err := time.Parse("2006-01-02", datePart); err != nil {
			continue
		}
		files = append(files, wafFile{date: datePart, path: filepath.Join(logDir, name)})
	}
	// Newest day first so the viewer's file selector defaults to the latest log.
	sort.Slice(files, func(i, j int) bool { return files[i].date > files[j].date })

	logs := make([]DiscoveredLog, 0, len(files))
	for _, f := range files {
		logs = append(logs, DiscoveredLog{
			SourceType: "waf",
			Domain:     f.date,
			LogPath:    f.path,
			LogFormat:  "json",
		})
	}
	return logs, nil
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
	return s.CreatePoliciesFromDiscovered(ctx, serverID, discovered)
}

// CreatePoliciesFromDiscovered creates rotation policies for already-discovered
// logs (skipping ones that have a policy). It is the shared tail of
// SyncPoliciesForServer — remote nodes resolve "discovered" via gRPC in the
// handler and call this directly.
func (s *Service) CreatePoliciesFromDiscovered(ctx context.Context, serverID int64, discovered []DiscoveredLog) (int, error) {
	created := 0
	for _, d := range discovered {
		// WAF 引擎自己按天切分（logs/<date>_waf.log），再做轮转只会和它的
		// 日期文件名打架；保留天数清理由 cleanExpiredWAFLogs 负责。
		if d.SourceType == "waf" {
			continue
		}
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
// Works for both proxy (nginx/caddy) and core (xray/singbox) processes.
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
// 安全审计 M-2：log_path 可经 HTTP 查询参数直达（GetNodeLog/StreamNodeLog），
// 绝对路径直通、相对路径 Join 都必须拒绝 `..`/NUL，否则构成面板本地任意文件读取。
// 非法路径返回空串（调用方按"文件不存在"处理并回退 broker）。
func (s *Service) resolveLogPath(path string) string {
	if path == "" {
		return path
	}
	if err := safepath.RejectTraversal("log_path", path); err != nil {
		log.Printf("[logmanage] rejected unsafe log path %q: %v", path, err)
		return ""
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
