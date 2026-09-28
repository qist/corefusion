package process

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/healthcheck"
	systemSvc "corefusion/internal/service/system"
	"corefusion/internal/store/repository"
)

// LogModeProvider provides the log output mode for a core type.
// Returns "file", "stdout", or "off".
type LogModeProvider interface {
	GetLogMode(ctx context.Context, coreType string) string
}

// Manager manages core process lifecycle with persistence and daemon support.
type Manager struct {
	processes     map[int64]*managedProcess
	guardians     map[int64]*healthcheck.Guardian
	mu            sync.RWMutex
	dataDir       string
	svcRepo       *repository.ServiceRepository
	guardRepo     *repository.ProcessGuardianRepository
	crashRepo     *repository.CrashLogRepository
	resolver      BinaryResolver
	configGen     ConfigGenerator
	skipConfigGen atomic.Bool // set by WriteRuntimeConfig to skip configGen in next Start()
	// configMu protects concurrent writes to runtime config files.
	// Both configsync.Service and LocalAgent.SyncConfig may write to the same
	// runtime/<coreType>.jsonc file; this mutex ensures atomic writes.
	configMu sync.Mutex

	// logModeProvider determines the log output mode (file/stdout/off) for each core.
	logModeProvider LogModeProvider
	// logBrokers holds LogBroker instances for core processes.
	// In file mode: broker tails the log file for real-time viewing.
	// In stdout mode: broker captures process stdout/stderr via AttachCmd.
	// In off mode: no broker is created.
	logBrokers map[string]*LogBroker
}

// ConfigGenerator generates a config file for a core type before starting.
// It should write the config to configPath and return nil on success.
type ConfigGenerator func(ctx context.Context, coreType string, configPath string) error

// SetConfigGenerator sets the config generator callback.
func (m *Manager) SetConfigGenerator(gen ConfigGenerator) {
	m.configGen = gen
}

// SetLogModeProvider sets the provider that determines log output mode per core type.
func (m *Manager) SetLogModeProvider(provider LogModeProvider) {
	m.logModeProvider = provider
}

// GetLogBroker returns the LogBroker for a core type, or nil if none exists.
func (m *Manager) GetLogBroker(coreType string) *LogBroker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.logBrokers[coreType]
}

// getLogMode returns the log mode for a core type, defaulting to "file".
func (m *Manager) getLogMode(coreType string) string {
	if m.logModeProvider == nil {
		return "file"
	}
	mode := m.logModeProvider.GetLogMode(context.Background(), coreType)
	if mode == "" {
		mode = "file"
	}
	return mode
}

// BinaryResolver resolves binary paths for core types.
type BinaryResolver interface {
	Resolve(coreType string) (binaryPath, version string, err error)
}

type managedProcess struct {
	ServiceID  int64
	CoreType   string
	BinaryPath string
	ConfigPath string
	LogPath    string
	cmd        *exec.Cmd
	exitDone   chan struct{}
	pid        int
	state      string
	startedAt  time.Time
	lastError  error
}

type processStateChange struct {
	serviceID int64
	state     string
	pid       int
}

// ProcessInfo contains public information about a managed process.
type ProcessInfo struct {
	ServiceID   int64     `json:"service_id"`
	CoreType    string    `json:"core_type"`
	State       string    `json:"state"`
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"started_at"`
	Uptime      int64     `json:"uptime"`
	LastError   string    `json:"last_error,omitempty"`
	AutoStart   bool      `json:"auto_start"`
	AutoRestart bool      `json:"auto_restart"`
	MaxRestarts int       `json:"max_restart_count"`
	CPU         float64   `json:"cpu"`
	Mem         int64     `json:"mem"`
}

// NewManager creates a new process manager.
func NewManager(dataDir string, svcRepo *repository.ServiceRepository, guardRepo *repository.ProcessGuardianRepository, crashRepo *repository.CrashLogRepository, resolver BinaryResolver) *Manager {
	// Convert to absolute path so it works from any directory
	absDir := dataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			absDir = abs
		}
	}
	return &Manager{
		processes:  make(map[int64]*managedProcess),
		guardians:  make(map[int64]*healthcheck.Guardian),
		logBrokers: make(map[string]*LogBroker),
		dataDir:    absDir,
		svcRepo:    svcRepo,
		guardRepo:  guardRepo,
		crashRepo:  crashRepo,
		resolver:   resolver,
	}
}

// WriteRuntimeConfig writes a core configuration file to the runtime directory
// under data/runtime/<coreType>.jsonc. It uses a mutex to prevent concurrent
// writes from configsync.Service and LocalAgent.SyncConfig, which both write
// to the same path.
// RuntimeConfigPath 返回某核心运行时配置的落盘路径。
func (m *Manager) RuntimeConfigPath(coreType string) string {
	return filepath.Join(m.dataDir, "runtime", coreType+".jsonc")
}

// RuntimeConfigChanged 判断把 data 写下去是否会**真的改变**磁盘上的配置。
//
// 用途：SyncData 的 reload_strategy=none 原本是"只写文件、不碰运行中的进程" ✗ ——
// 真机上这会让**配置变更静默不生效**（远程节点核心两天没重载、链式配置写了从未加载 ✗）。
// 用这个判断把语义修正为"**内容变了就重启**"，既保证变更生效 ✓，又避免每次同步都白重启 ✗。
func (m *Manager) RuntimeConfigChanged(coreType string, data []byte) bool {
	old, err := os.ReadFile(m.RuntimeConfigPath(coreType))
	if err != nil {
		return true // 还没有旧配置 → 视为变更
	}
	return !bytes.Equal(bytes.TrimSpace(old), bytes.TrimSpace(data))
}

func (m *Manager) WriteRuntimeConfig(coreType string, data []byte) error {
	m.configMu.Lock()
	defer m.configMu.Unlock()

	runtimeDir := filepath.Join(m.dataDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0750); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	configPath := filepath.Join(runtimeDir, coreType+".jsonc")
	if err := os.WriteFile(configPath, data, 0640); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	// Signal to Start() that config is already written — skip configGen
	m.skipConfigGen.Store(true)
	return nil
}

// SeedCores creates default service entries for all supported core types if they don't exist.
func (m *Manager) SeedCores(ctx context.Context) {
	cores := []struct {
		Name     string
		CoreType string
	}{
		{"Xray", "xray"},
		{"sing-box", "singbox"},
		{"Hysteria 2", "hysteria2"},
		{"TUIC", "tuic"},
	}

	for _, c := range cores {
		// Check if service already exists for this core type
		services, _, err := m.svcRepo.List(ctx, 1, c.CoreType, 1, 0) // server_id=1 (local)
		if err != nil {
			log.Printf("seed: failed to check %s: %v", c.CoreType, err)
			continue
		}
		if len(services) > 0 {
			continue
		}

		svc := &entity.InboundService{
			ServerID:   1, // local node
			Name:       c.Name,
			CoreType:   c.CoreType,
			Protocol:   "-",
			ListenHost: "0.0.0.0",
			ListenPort: 0,
		}
		if err := m.svcRepo.Create(ctx, svc); err != nil {
			log.Printf("seed: failed to create %s: %v", c.CoreType, err)
		} else {
			log.Printf("seed: created service for %s (id=%d)", c.CoreType, svc.ID)
		}
	}
}

// Reconcile kills old processes and starts auto-start services fresh.
func (m *Manager) Reconcile(ctx context.Context) {
	// Load all guardians
	guardians, err := m.guardRepo.List(ctx)
	if err != nil {
		log.Printf("reconcile: failed to load guardians: %v", err)
		return
	}

	// Kill all old processes and reset status
	for _, g := range guardians {
		// Only manage local node (server_id=1) processes.
		// Remote nodes' processes are managed by their own Agents.
		svc, err := m.svcRepo.GetByID(ctx, g.ServiceID)
		if err != nil || svc.ServerID != 1 {
			continue
		}
		if g.PID > 0 && isProcessAlive(g.PID) {
			killProcess(g.PID)
			// Wait a bit for process to exit and reap zombie
			time.Sleep(500 * time.Millisecond)
			log.Printf("reconcile: killed old process %d (service %d)", g.PID, g.ServiceID)
		}
		g.ProcessStatus = "stopped"
		g.PID = 0
		m.guardRepo.Update(ctx, &g)
	}

	// Start auto-start services fresh
	autoStartList, err := m.guardRepo.ListAutoStart(ctx)
	if err != nil {
		log.Printf("reconcile: failed to load auto-start list: %v", err)
		return
	}

	for _, g := range autoStartList {
		svc, err := m.svcRepo.GetByID(ctx, g.ServiceID)
		if err != nil {
			continue
		}
		// Only start services for the local node (server_id=1).
		// Remote nodes' processes are managed by their own Agents.
		if svc.ServerID != 1 {
			continue
		}
		if err := m.Start(ctx, g.ServiceID, svc.CoreType); err != nil {
			log.Printf("reconcile: failed to start service %d: %v", g.ServiceID, err)
		} else {
			log.Printf("reconcile: auto-started service %d", g.ServiceID)
		}
	}

	log.Printf("reconcile: done, %d processes managed", len(m.processes))
}

// reattach reattaches to an already-running process.
func (m *Manager) reattach(g entity.ProcessGuardian) {
	svc, err := m.svcRepo.GetByID(nil, g.ServiceID)
	if err != nil {
		return
	}
	binaryPath, _, _ := m.resolver.Resolve(svc.CoreType)
	configPath := filepath.Join(m.dataDir, "runtime", svc.CoreType+".jsonc")
	logPath := filepath.Join(m.dataDir, "logs", svc.CoreType+".log")

	p := &managedProcess{
		ServiceID:  g.ServiceID,
		CoreType:   svc.CoreType,
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		LogPath:    logPath,
		pid:        g.PID,
		state:      "running",
		startedAt:  g.StartedAt,
	}
	m.processes[g.ServiceID] = p

	// Start monitoring
	go m.monitor(p, g)
}

// Start starts a core process.
func (m *Manager) Start(ctx context.Context, serviceID int64, coreType string) error {
	// Generate config OUTSIDE the lock — configGen may query the database
	// which can be slow and would block all other mu.Lock() callers.
	configDir := filepath.Join(m.dataDir, "runtime")
	os.MkdirAll(configDir, 0750)
	configPath := filepath.Join(configDir, coreType+".jsonc")
	// If WriteRuntimeConfig was just called (e.g. by SyncData), the config
	// file is already up-to-date — skip configGen to avoid overwriting.
	if m.skipConfigGen.CompareAndSwap(true, false) {
		// Config was already written by WriteRuntimeConfig, skip generation.
		// But if the file is missing, fall through to configGen.
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			if m.configGen != nil {
				if err := m.configGen(ctx, coreType, configPath); err != nil {
					return fmt.Errorf("generate config: %w", err)
				}
			} else {
				return fmt.Errorf("config not found: %s", configPath)
			}
		}
	} else if m.configGen != nil {
		if err := m.configGen(ctx, coreType, configPath); err != nil {
			return fmt.Errorf("generate config: %w", err)
		}
	} else {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
		}
	}

	// Resolve binary OUTSIDE the lock (filesystem access)
	binaryPath, _, err := m.resolver.Resolve(coreType)
	if err != nil {
		return fmt.Errorf("binary not found for %s: %w", coreType, err)
	}

	// Now acquire lock for the fast in-memory operations
	m.mu.Lock()

	// If the process is already running, return immediately — don't kill it.
	// Calling Start on an already-running core should be a no-op, not a restart.
	// This prevents accidental core restarts when the panel's status cache is
	// stale (e.g. after panel restart) and the UI sends a spurious "start" command.
	if p, ok := m.processes[serviceID]; ok {
		if p.state == "running" {
			m.mu.Unlock()
			log.Printf("service %d already running (pid=%d), start skipped", serviceID, p.pid)
			return nil
		}
		// If a restart or stop is in progress, skip to avoid starting a
		// duplicate process while the async goroutine is between kill and start.
		if p.state == "restarting" || p.state == "stopping" {
			m.mu.Unlock()
			log.Printf("service %d is %s, start skipped", serviceID, p.state)
			return nil
		}
	}

	// Kill existing process if any (e.g. crashed but not yet cleaned up)
	if p, ok := m.processes[serviceID]; ok {
		if p.cmd != nil && p.cmd.Process != nil {
			killProcess(p.cmd.Process.Pid)
		}
		delete(m.processes, serviceID)
	}

	// Collect guardian to stop after releasing mu (avoid AB-BA deadlock:
	// g.Stop() acquires Guardian.mu; if a Guardian restartFn holds Guardian.mu
	// and waits for mu, calling g.Stop() under mu creates AB-BA deadlock).
	// We stop the old guardian AFTER the lock is released.
	var guardianToStop *healthcheck.Guardian
	if g, ok := m.guardians[serviceID]; ok {
		guardianToStop = g
		delete(m.guardians, serviceID)
	}

	// Release mu first, then stop the old guardian.
	defer func() {
		m.mu.Unlock()
		if guardianToStop != nil {
			guardianToStop.Stop()
		}
	}()

	// Log directory
	logDir := filepath.Join(m.dataDir, "logs")
	os.MkdirAll(logDir, 0750)
	logPath := filepath.Join(logDir, coreType+".log")

	// Determine log mode
	logMode := m.getLogMode(coreType)

	// Stop old LogBroker if any
	if oldBroker, ok := m.logBrokers[coreType]; ok {
		oldBroker.Stop()
		delete(m.logBrokers, coreType)
	}

	// Use relative paths from dataDir (same as proxy)
	relBinPath, _ := filepath.Rel(m.dataDir, binaryPath)
	relConfigPath, _ := filepath.Rel(m.dataDir, configPath)

	// Kill orphan processes left over from a previous panel run BEFORE starting.
	// When the panel itself is killed (e.g. kill -9 during deploy), its core
	// children survive as orphans (own process group). A fresh panel's
	// m.processes map is empty, so the normal PID-based kill path never sees
	// them — the new process would then die on "bind: address already in use"
	// while the orphan keeps serving the OLD config ("new process crashed,
	// old process never closed"). Match orphans by command line: same binary
	// basename + same runtime config path.
	//
	// ⚠ We are holding m.mu (write) here until Start returns — the orphan scan
	// itself must NOT touch m.mu (a RLock under a held Lock deadlocks forever;
	// that deadlock is exactly what froze Reconcile and kept cores down).
	// Collect the managed-PID set inside the lock and pass it by value.
	managedPids := make(map[int]bool, len(m.processes))
	for _, p := range m.processes {
		if p.pid > 0 {
			managedPids[p.pid] = true
		}
	}
	m.killOrphanProcesses(managedPids, relBinPath, relConfigPath)

	// Ensure geo files are accessible from the panel's lookup paths.
	rulesDir := filepath.Join(m.dataDir, "rules")
	if _, err := os.Stat(rulesDir); err == nil {
		symlinkGeoFiles(rulesDir)
	}

	// sing-box needs "run" subcommand: "sing-box run -c config.json"
	// xray uses: "xray -c config.json"
	var cmd *exec.Cmd
	if coreType == "singbox" {
		cmd = exec.CommandContext(context.Background(), relBinPath, "run", "-c", relConfigPath)
	} else {
		cmd = exec.CommandContext(context.Background(), relBinPath, "-c", relConfigPath)
	}
	cmd.Dir = m.dataDir

	// Configure process output based on log mode
	var logFile *os.File
	switch logMode {
	case "stdout":
		// stdout mode: LogBroker captures process output via io.Pipe
		broker := NewLogBroker()
		broker.AttachCmd(cmd)
		m.logBrokers[coreType] = broker
		log.Printf("process: started %s in stdout mode (LogBroker attached)", coreType)

	case "off":
		// off mode: discard all output
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0640)
		if err == nil {
			cmd.Stdout = devNull
			cmd.Stderr = devNull
			// Store to close later (not tracked in managedProcess, will be GC'd)
		}
		log.Printf("process: started %s in off mode (output discarded)", coreType)

	default:
		// file mode (default): write to log file, also tail for real-time viewing
		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		// Create LogBroker and tail the log file for real-time viewing
		broker := NewLogBroker()
		broker.TailFile(logPath)
		m.logBrokers[coreType] = broker
		log.Printf("process: started %s in file mode (log: %s)", coreType, logPath)
	}
	// Set resource path for geoip/geosite files
	xrayAssetPath := os.Getenv("XRAY_LOCATION_ASSET")
	if xrayAssetPath == "" {
		xrayAssetPath = rulesDir
	}
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+xrayAssetPath)
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return fmt.Errorf("start process: %w", err)
	}

	p := &managedProcess{
		ServiceID:  serviceID,
		CoreType:   coreType,
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		LogPath:    logPath,
		cmd:        cmd,
		exitDone:   make(chan struct{}),
		pid:        cmd.Process.Pid,
		state:      "running",
		startedAt:  time.Now(),
	}
	m.processes[serviceID] = p

	go func() {
		_ = cmd.Wait()
		close(p.exitDone)
	}()

	// Persist to database
	m.persistState(serviceID, "running", cmd.Process.Pid, false)

	// Create and start guardian for health check
	guardian := m.createGuardian(serviceID, coreType)
	m.guardians[serviceID] = guardian
	guardian.Start(cmd.Process.Pid)

	log.Printf("started service %d (core=%s, pid=%d)", serviceID, coreType, p.pid)
	return nil
}

// createGuardian creates a health check guardian for a service.
// It reads auto_restart/auto_start from the DB so the Guardian respects
// the persisted configuration rather than the package default (false).
func (m *Manager) createGuardian(serviceID int64, coreType string) *healthcheck.Guardian {
	cfg := healthcheck.DefaultConfig(coreType, serviceID)

	// Load guardian config from DB
	var autoRestart bool
	var maxRestarts int
	if g, err := m.guardRepo.GetByServiceID(nil, serviceID); err == nil {
		autoRestart = g.AutoRestart
		maxRestarts = g.MaxRestartCount
		if maxRestarts > 0 {
			cfg.MaxRestarts = maxRestarts
		}
	}

	var guardian *healthcheck.Guardian
	guardian = healthcheck.New(cfg,
		func() bool {
			// Read PID from process.Manager (no lock needed — pid is atomically
			// updated). Avoid calling guardian.PID() which acquires Guardian.mu,
			// because check() also acquires Guardian.mu, creating a re-entrant
			// lock attempt if the scheduler preempts between Unlock and the
			// isRunning callback.
			m.mu.RLock()
			p, ok := m.processes[serviceID]
			pid := 0
			if ok {
				pid = p.pid
			}
			m.mu.RUnlock()
			if pid <= 0 {
				return false
			}
			return isProcessAlive(pid)
		},
		func(ctx context.Context) error {
			// m.Start will:
			//   1. Kill the old process
			//   2. Stop the old Guardian (this guardian's monitorLoop)
			//   3. Create a new Guardian with its own monitorLoop
			//   4. Start the new process
			// After m.Start returns, this Guardian is no longer in m.guardians
			// and its monitorLoop has been stopped via g.Stop().
			// Reap zombie before restart
			m.reapZombies(serviceID)
			return m.Start(ctx, serviceID, coreType)
		},
	)

	// Apply DB-backed settings
	guardian.SetAutoRestart(autoRestart)

	return guardian
}

// Stop stops a core process.
func (m *Manager) Stop(ctx context.Context, serviceID int64) error {
	m.mu.Lock()
	p, ok := m.processes[serviceID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("service %d not found", serviceID)
	}
	if p.state != "running" && p.state != "restarting" {
		m.mu.Unlock()
		return fmt.Errorf("service %d is not running (state=%s)", serviceID, p.state)
	}

	// Collect guardian reference before releasing mu to avoid AB-BA deadlock:
	// we must NOT call g.Stop() while holding mu, because g.Stop() acquires
	// Guardian.mu, and Guardian's restartFn may acquire Guardian.mu then mu.
	var guardianToStop *healthcheck.Guardian
	if g, gok := m.guardians[serviceID]; gok {
		guardianToStop = g
		delete(m.guardians, serviceID)
	}

	// Mark as stopping so status queries return correct state
	p.state = "stopping"
	oldPID := p.pid
	m.mu.Unlock()

	// Now safely stop the guardian outside of mu
	if guardianToStop != nil {
		guardianToStop.Stop()
	}

	log.Printf("stop service %d (pid=%d): initiating async stop", serviceID, oldPID)

	// Non-blocking: send kill signal and spawn goroutine to wait + cleanup.
	// The caller (HTTP handler or gRPC) returns immediately.
	go func() {
		// Send SIGTERM to process group
		if oldPID > 0 {
			killProcess(oldPID)
		}

		// Wait for exit (up to 10s graceful, then force kill)
		waitForExit := func(timeout time.Duration) bool {
			deadline := time.Now().Add(timeout)
			for isProcessAlive(oldPID) && time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
			}
			return !isProcessAlive(oldPID)
		}

		exited := waitForExit(10 * time.Second)
		if !exited {
			log.Printf("stop service %d: process %d did not exit gracefully, force killing", serviceID, oldPID)
			forceKillProcess(oldPID)
			waitForExit(3 * time.Second)
		}

		m.mu.Lock()
		p.state = "stopped"
		p.pid = 0
		p.lastError = nil
		m.mu.Unlock()

		// Mark manually stopped
		m.persistState(serviceID, "stopped", 0, true)
		log.Printf("stopped service %d (pid=%d)", serviceID, oldPID)
	}()

	return nil
}

// reapZombies cleans up zombie processes for a service.
func (m *Manager) reapZombies(serviceID int64) {
	m.mu.RLock()
	p, ok := m.processes[serviceID]
	m.mu.RUnlock()
	if !ok || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	// Try to reap the process (non-blocking)
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()
	select {
	case <-done:
		// Process reaped
	case <-time.After(100 * time.Millisecond):
		// Timeout, process might not be a zombie
	}
}

// Restart restarts a core process.
// This method is non-blocking: it immediately stops the Guardian (to prevent
// auto-restart interference), marks the process as "restarting", then spawns
// a goroutine to perform the actual stop → sleep → start sequence.
// This prevents the HTTP handler from being blocked for 10+ seconds while
// waiting for the process to exit and the new one to start.
// RestartIfRunning 重启「正在运行」的本地核心（server_id=1）。
//
// 用途：xray / sing-box 都不支持热加载配置文件，面板把新配置写盘后必须重启核心
// 才会生效（远端节点由 Agent 在 SyncConfig 时重启，本地节点此前只写盘不重启，
// 表现为「保存了配置但没生效」）。未运行的核心不会被启动，避免保存配置意外拉起进程。
func (m *Manager) RestartIfRunning(ctx context.Context, coreType string) error {
	if m.svcRepo == nil {
		return fmt.Errorf("service repository not configured")
	}
	services, _, err := m.svcRepo.List(ctx, 1, coreType, 1, 0)
	if err != nil {
		return fmt.Errorf("query local service for %s: %w", coreType, err)
	}
	if len(services) == 0 {
		return fmt.Errorf("no local service found for core type %s", coreType)
	}
	serviceID := services[0].ID
	if !m.IsRunning(serviceID) {
		return nil
	}
	return m.Restart(ctx, serviceID, coreType)
}

// IsCoreRunning 报告指定核心当前是否有正在运行的进程。
//
// 用途：stats/gRPC API 探测前的门禁。核心未运行（或未安装）时不应反复去连它的
// API —— 例如 xray 没装的节点每 30s 去连 127.0.0.1:10085，只会在日志里刷
// “connection failed” 并白等超时。
func (m *Manager) IsCoreRunning(coreType string) bool {
	for _, p := range m.ListAll() {
		if p.CoreType == coreType && p.State == "running" {
			return true
		}
	}
	return false
}

// IsCoreInstalled 报告指定核心的二进制是否已安装。
func (m *Manager) IsCoreInstalled(coreType string) bool {
	if m.resolver == nil {
		return false
	}
	_, _, err := m.resolver.Resolve(coreType)
	return err == nil
}

func (m *Manager) Restart(ctx context.Context, serviceID int64, coreType string) error {
	m.mu.Lock()
	p, ok := m.processes[serviceID]
	if !ok {
		// Process not tracked — just start it
		m.mu.Unlock()
		return m.Start(ctx, serviceID, coreType)
	}

	// Collect guardian reference before releasing mu to avoid AB-BA deadlock
	// (same pattern as Stop — see comment there).
	var guardianToStop *healthcheck.Guardian
	if g, gok := m.guardians[serviceID]; gok {
		guardianToStop = g
		delete(m.guardians, serviceID)
	}

	// Mark as restarting so status queries return correct state
	p.state = "restarting"
	oldPID := p.pid
	m.mu.Unlock()

	// Now safely stop the guardian outside of mu
	if guardianToStop != nil {
		guardianToStop.Stop()
	}

	log.Printf("restart service %d (pid=%d, core=%s): initiating async restart", serviceID, oldPID, coreType)

	// Perform the actual restart in a goroutine
	go func() {
		// Stop the old process (may take up to 13s with graceful shutdown)
		if oldPID > 0 {
			killProcess(oldPID)
			// Wait for process to exit (max 10s graceful + 3s forced)
			deadline := time.Now().Add(13 * time.Second)
			for isProcessAlive(oldPID) && time.Now().Before(deadline) {
				time.Sleep(100 * time.Millisecond)
			}
			if isProcessAlive(oldPID) {
				forceKillProcess(oldPID)
			}
		}

		// Brief pause before starting new process
		time.Sleep(1 * time.Second)

		// Clean up old process entry and start new one
		m.mu.Lock()
		delete(m.processes, serviceID)
		m.mu.Unlock()

		if err := m.Start(context.Background(), serviceID, coreType); err != nil {
			log.Printf("restart service %d: start failed: %v", serviceID, err)
			m.persistState(serviceID, "stopped", 0, false)
		} else {
			log.Printf("restart service %d: completed successfully", serviceID)
		}
	}()

	return nil
}

// GetStatus returns the status of a core process.
func (m *Manager) GetStatus(serviceID int64) (string, int, time.Time, error) {
	m.mu.Lock()
	p, ok, changed := m.refreshProcessStateLocked(serviceID)
	var state string
	var pid int
	var startedAt time.Time
	var lastErr error
	if ok {
		state = p.state
		pid = p.pid
		startedAt = p.startedAt
		lastErr = p.lastError
	}
	m.mu.Unlock()
	if changed {
		m.persistState(serviceID, state, pid, false)
	}
	if !ok {
		return "stopped", 0, time.Time{}, nil
	}
	return state, pid, startedAt, lastErr
}

// IsRunning checks if a core process is running.
func (m *Manager) IsRunning(serviceID int64) bool {
	m.mu.Lock()
	p, ok, changed := m.refreshProcessStateLocked(serviceID)
	running := ok && p.state == "running"
	pid := 0
	if ok {
		pid = p.pid
	}
	m.mu.Unlock()
	if changed {
		status := "stopped"
		if ok {
			status = p.state
		}
		m.persistState(serviceID, status, pid, false)
	}
	return running
}

func (m *Manager) isRunning(serviceID int64) bool {
	p, ok := m.processes[serviceID]
	return ok && p.state == "running"
}

// ListAll returns the status of all managed processes.
func (m *Manager) ListAll() []ProcessInfo {
	m.mu.Lock()

	result := make([]ProcessInfo, 0, len(m.processes))
	var changes []processStateChange
	for _, p := range m.processes {
		if p.state == "running" || p.state == "stopping" {
			if _, _, changed := m.refreshProcessStateLocked(p.ServiceID); changed {
				changes = append(changes, processStateChange{
					serviceID: p.ServiceID,
					state:     p.state,
					pid:       p.pid,
				})
			}
		}
		info := ProcessInfo{
			ServiceID: p.ServiceID,
			CoreType:  p.CoreType,
			State:     p.state,
			PID:       p.pid,
			StartedAt: p.startedAt,
		}
		if p.state == "running" {
			info.Uptime = int64(time.Since(p.startedAt).Seconds())
			info.CPU, info.Mem = getProcessStats(p.pid)
		}
		if p.lastError != nil {
			info.LastError = p.lastError.Error()
		}
		// Load guardian config
		if g, err := m.guardRepo.GetByServiceID(nil, p.ServiceID); err == nil {
			info.AutoStart = g.AutoStart
			info.AutoRestart = g.AutoRestart
			info.MaxRestarts = g.MaxRestartCount
		}
		result = append(result, info)
	}
	m.mu.Unlock()
	for _, change := range changes {
		m.persistState(change.serviceID, change.state, change.pid, false)
	}
	return result
}

// GetUptime returns the uptime of a running process in seconds.
func (m *Manager) GetUptime(serviceID int64) int64 {
	m.mu.Lock()
	p, ok, changed := m.refreshProcessStateLocked(serviceID)
	var uptime int64
	var state string
	var pid int
	if !ok || p.state != "running" {
		if ok {
			state = p.state
			pid = p.pid
		}
		m.mu.Unlock()
		if changed {
			m.persistState(serviceID, state, pid, false)
		}
		return 0
	}
	uptime = int64(time.Since(p.startedAt).Seconds())
	state = p.state
	pid = p.pid
	m.mu.Unlock()
	if changed {
		m.persistState(serviceID, state, pid, false)
	}
	return uptime
}

// StopAll gracefully stops all running processes (called on CoreFusion shutdown).
func (m *Manager) StopAll(ctx context.Context) {
	m.mu.Lock()

	// Stop all log brokers
	for coreType, broker := range m.logBrokers {
		broker.Stop()
		delete(m.logBrokers, coreType)
	}

	// Collect guardians to stop after releasing mu (avoid AB-BA deadlock)
	var guardiansToStop []*healthcheck.Guardian
	for id, g := range m.guardians {
		guardiansToStop = append(guardiansToStop, g)
		delete(m.guardians, id)
	}

	for id, p := range m.processes {
		if p.state == "running" && p.cmd != nil && p.cmd.Process != nil {
			killProcess(p.pid)
			log.Printf("stopall: killed service %d (pid %d)", id, p.pid)
		}
		p.state = "stopped"
	}

	// Update all guardians to stopped status
	guardians, err := m.guardRepo.List(ctx)
	if err == nil {
		for _, g := range guardians {
			g.ProcessStatus = "stopped"
			g.PID = 0
			m.guardRepo.Update(ctx, &g)
		}
	}

	m.processes = make(map[int64]*managedProcess)
	m.mu.Unlock()

	// Stop guardians outside of mu
	for _, g := range guardiansToStop {
		g.Stop()
	}

	log.Printf("stopall: all processes stopped")
}

// monitor monitors a process and handles crashes.
func (m *Manager) monitor(p *managedProcess, g entity.ProcessGuardian) {
	var err error
	if p.cmd != nil {
		err = p.cmd.Wait()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if p.state == "stopping" {
		p.state = "stopped"
		p.pid = 0
		p.lastError = nil
		return
	}

	// Unexpected crash
	p.state = "crashed"
	p.lastError = err

	// Load guardian config if not already loaded
	if g.ServiceID == 0 {
		gPtr, repoErr := m.guardRepo.GetByServiceID(nil, p.ServiceID)
		if repoErr == nil {
			g = *gPtr
		}
	}

	// Record crash log
	crashLog := &entity.CoreCrashLog{
		ServiceID:      p.ServiceID,
		CoreType:       p.CoreType,
		RuntimeSeconds: int64(time.Since(p.startedAt).Seconds()),
		RestartAttempt: g.CurrentRestarts + 1,
		CrashAt:        time.Now(),
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		crashLog.ExitCode = exitErr.ExitCode()
	}
	if p.cmd != nil && p.cmd.ProcessState != nil {
		crashLog.Signal = p.cmd.ProcessState.String()
	}
	m.crashRepo.Create(nil, crashLog)

	log.Printf("service %d crashed: %v (restarts: %d/%d)", p.ServiceID, err, g.CurrentRestarts, g.MaxRestartCount)

	// 核心进程异常告警：admin 模板「核心进程异常告警」未启用时 NotifyTemplate 静默 no-op
	go systemSvc.NotifyTemplate(context.Background(), "核心进程异常告警", map[string]string{
		"core":     p.CoreType,
		"restarts": fmt.Sprintf("%d", g.CurrentRestarts),
		"max":      fmt.Sprintf("%d", g.MaxRestartCount),
		"message":  fmt.Sprintf("%v", err),
		"time":     time.Now().Format("2006-01-02 15:04:05"),
	})

	// Update guardian
	g.ProcessStatus = "crashed"
	g.LastCrashAt = time.Now()
	g.PID = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			g.LastExitCode = exitErr.ExitCode()
		}
	}

	// Check if auto-restart is allowed
	if g.AutoRestart && g.CurrentRestarts < g.MaxRestartCount {
		// Check restart window
		if g.WindowStartAt.IsZero() || time.Since(g.WindowStartAt) > time.Duration(g.RestartWindowSec)*time.Second {
			g.WindowStartAt = time.Now()
			g.CurrentRestarts = 0
		}
		g.CurrentRestarts++
		g.LastRestartAt = time.Now()
		m.guardRepo.Update(nil, &g)

		// Exponential backoff: 2^(restarts-1) * 2 seconds, max 80s
		backoff := time.Duration(1<<uint(g.CurrentRestarts-1)*2) * time.Second
		if backoff > 80*time.Second {
			backoff = 80 * time.Second
		}
		log.Printf("service %d: restarting in %v (attempt %d/%d)", p.ServiceID, backoff, g.CurrentRestarts, g.MaxRestartCount)

		// Restart after backoff
		go func() {
			time.Sleep(backoff)
			m.mu.Lock()
			delete(m.processes, p.ServiceID)
			m.mu.Unlock()

			if err := m.Start(nil, p.ServiceID, p.CoreType); err != nil {
				log.Printf("service %d: auto-restart failed: %v", p.ServiceID, err)
			} else {
				// Reset restart count after stable threshold
				go func() {
					time.Sleep(time.Duration(g.StableThresholdSec) * time.Second)
					if m.IsRunning(p.ServiceID) {
						g2, err := m.guardRepo.GetByServiceID(nil, p.ServiceID)
						if err == nil {
							g2.CurrentRestarts = 0
							m.guardRepo.Update(nil, g2)
						}
					}
				}()
			}
		}()
	} else {
		// Max restarts reached or auto-restart disabled
		g.ProcessStatus = "crashed"
		m.guardRepo.Update(nil, &g)
	}
}

// persistState updates the guardian and service process status in the database.
func (m *Manager) persistState(serviceID int64, status string, pid int, manuallyStopped bool) {
	g, err := m.guardRepo.GetByServiceID(nil, serviceID)
	if err != nil {
		// Create guardian record
		g = &entity.ProcessGuardian{
			ServiceID:          serviceID,
			AutoStart:          false,
			AutoRestart:        false,
			MaxRestartCount:    5,
			RestartWindowSec:   600,
			StableThresholdSec: 300,
		}
		m.guardRepo.Create(nil, g)
	}

	g.ProcessStatus = status
	g.PID = pid
	g.ManuallyStopped = manuallyStopped
	if status == "running" {
		g.StartedAt = time.Now()
	}
	m.guardRepo.Update(nil, g)

	// Update service ProcessStatus
	if svc, err := m.svcRepo.GetByID(nil, serviceID); err == nil {
		svc.ProcessStatus = status
		m.svcRepo.Update(nil, svc)
	}
}

func (m *Manager) refreshProcessStateLocked(serviceID int64) (*managedProcess, bool, bool) {
	p, ok := m.processes[serviceID]
	if !ok {
		return nil, false, false
	}

	if p.state == "running" && !isProcessAlive(p.pid) {
		p.state = "crashed"
		p.pid = 0
		if p.lastError == nil {
			p.lastError = fmt.Errorf("process exited")
		}
		return p, true, true
	}

	if p.state == "stopping" && !isProcessAlive(p.pid) {
		p.state = "stopped"
		p.pid = 0
		return p, true, true
	}

	return p, true, false
}

// GetProcessStats reads CPU and memory from /proc/<pid>/stat (Linux only).
// Exported so that the proxy manager can reuse it for proxy processes.
func GetProcessStats(pid int) (cpu float64, mem int64) {
	return getProcessStats(pid)
}

// GetProcessStartTime returns the wall-clock start time of a process by reading
// /proc/<pid>/stat (field 22 = starttime in clock ticks since boot) and
// /proc/stat (btime = boot time in seconds since epoch).
// On non-Linux or error, returns the zero time.
func GetProcessStartTime(pid int) time.Time {
	if runtime.GOOS != "linux" {
		return time.Time{}
	}

	// Read process stat to get starttime (field 22, 0-indexed = fields[21])
	statData, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return time.Time{}
	}
	fields := strings.Fields(string(statData))
	if len(fields) < 22 {
		return time.Time{}
	}
	starttimeTicks, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return time.Time{}
	}

	// Read boot time from /proc/stat (btime line)
	procStat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	var bootSec int64
	for _, line := range strings.Split(string(procStat), "\n") {
		if strings.HasPrefix(line, "btime ") {
			bootSec, _ = strconv.ParseInt(strings.TrimPrefix(line, "btime "), 10, 64)
			break
		}
	}
	if bootSec == 0 {
		return time.Time{}
	}

	clkTck := int64(100) // standard CLK_TCK on Linux
	startSec := bootSec + starttimeTicks/clkTck
	return time.Unix(startSec, 0)
}

// getProcessStats reads CPU and memory from /proc/<pid>/stat (Linux only).
func getProcessStats(pid int) (cpu float64, mem int64) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}

	// Read /proc/<pid>/stat for CPU time
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	statData, err := os.ReadFile(statPath)
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(string(statData))
	if len(fields) < 23 {
		return 0, 0
	}
	// fields[13] = utime, fields[14] = stime (in clock ticks)
	utime, _ := strconv.ParseInt(fields[13], 10, 64)
	stime, _ := strconv.ParseInt(fields[14], 10, 64)
	totalTime := utime + stime
	// Get system uptime
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err == nil {
		uptimeFields := strings.Fields(string(uptimeData))
		if len(uptimeFields) > 0 {
			sysUptime, _ := strconv.ParseFloat(uptimeFields[0], 64)
			clkTck := int64(100) // Default CLK_TCK
			if sysUptime > 0 && totalTime > 0 {
				cpu = float64(totalTime) / float64(clkTck) / sysUptime * 100
			}
		}
	}

	// Read /proc/<pid>/status for memory
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		return cpu, 0
	}
	for _, line := range strings.Split(string(statusData), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, _ := strconv.ParseInt(parts[1], 10, 64)
				mem = kb * 1024 // Convert KB to bytes
			}
			break
		}
	}

	return cpu, mem
}

// UpdateGuardian updates the guardian configuration for a service.
func (m *Manager) UpdateGuardian(ctx context.Context, serviceID int64, autoStart, autoRestart bool, maxRestarts int) error {
	g, err := m.guardRepo.GetByServiceID(ctx, serviceID)
	if err != nil {
		g = &entity.ProcessGuardian{
			ServiceID:          serviceID,
			MaxRestartCount:    5,
			RestartWindowSec:   600,
			StableThresholdSec: 300,
		}
		m.guardRepo.Create(ctx, g)
	}
	g.AutoStart = autoStart
	g.AutoRestart = autoRestart
	if maxRestarts > 0 {
		g.MaxRestartCount = maxRestarts
	}
	return m.guardRepo.Update(ctx, g)
}

// GetGuardian returns the guardian config for a service.
func (m *Manager) GetGuardian(ctx context.Context, serviceID int64) (*entity.ProcessGuardian, error) {
	return m.guardRepo.GetByServiceID(ctx, serviceID)
}

// ListGuardians returns all guardian configs.
func (m *Manager) ListGuardians(ctx context.Context) ([]entity.ProcessGuardian, error) {
	return m.guardRepo.List(ctx)
}

// ResolveBinary resolves the binary path and version for a core type.
func (m *Manager) ResolveBinary(coreType string) (binaryPath, version string, err error) {
	return m.resolver.Resolve(coreType)
}

// symlinkGeoFiles creates symlinks for geoip/geosite files in the panel's
// lookup directories so xray config parsing can resolve geodata consistently.
func symlinkGeoFiles(rulesDir string) {
	geoFiles := []string{"geoip.dat", "geosite.dat", "geoip.metadb", "country.mmdb", "GeoLite2-ASN.mmdb"}
	targetDirs := geoLookupDirs()
	for _, name := range geoFiles {
		src := filepath.Join(rulesDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		for _, dir := range targetDirs {
			dst := filepath.Join(dir, name)
			if info, err := os.Lstat(dst); err == nil {
				if info.Mode()&os.ModeSymlink == 0 {
					log.Printf("symlinkGeoFiles: skip existing non-symlink %s", dst)
					continue
				}
				linkTarget, err := os.Readlink(dst)
				if err == nil {
					resolvedTarget := linkTarget
					if !filepath.IsAbs(resolvedTarget) {
						resolvedTarget = filepath.Join(dir, resolvedTarget)
					}
					if filepath.Clean(resolvedTarget) == filepath.Clean(src) {
						continue
					}
				}
				if err := os.Remove(dst); err != nil {
					log.Printf("symlinkGeoFiles: remove stale symlink %s failed: %v", dst, err)
					continue
				}
			} else if !os.IsNotExist(err) {
				log.Printf("symlinkGeoFiles: stat %s failed: %v", dst, err)
				continue
			}
			if err := os.Symlink(src, dst); err != nil {
				log.Printf("symlinkGeoFiles: create %s -> %s failed: %v", dst, src, err)
			}
		}
	}
}

// killOrphanProcesses kills leftover core processes (same binary + same runtime
// config on the command line) that are NOT tracked in m.processes — typically
// children of a previous panel run that was killed. Orphans hold the ports the
// new process needs, so they must die before cmd.Start(); SIGTERM first, then
// SIGKILL after a short grace period. The caller passes the managed-PID set
// (collected under m.mu) and this function must NOT touch m.mu — it runs while
// Start holds the write lock, and an RLock here would deadlock forever.
// Best effort: scan errors are logged and ignored.
func (m *Manager) killOrphanProcesses(managed map[int]bool, relBinPath, relConfigPath string) {
	wantBin := filepath.Base(relBinPath)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 || pid == os.Getpid() || managed[pid] {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		fields := strings.Fields(strings.ReplaceAll(string(raw), "\x00", " "))
		if len(fields) == 0 || filepath.Base(fields[0]) != wantBin {
			continue
		}
		// The command line stores the config as a dataDir-relative path
		// (e.g. runtime/singbox.jsonc) — match that exact argument.
		hasCfg := false
		for _, f := range fields {
			if f == relConfigPath {
				hasCfg = true
				break
			}
		}
		if !hasCfg {
			continue
		}
		log.Printf("kill orphan %s process: pid=%d (leftover from previous panel run)", wantBin, pid)
		killProcess(pid)
		deadline := time.Now().Add(3 * time.Second)
		for isProcessAlive(pid) && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if isProcessAlive(pid) {
			forceKillProcess(pid)
		}
	}
}

func geoLookupDirs() []string {
	dirs := make([]string, 0, 2)
	if exePath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exePath))
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}

	seen := make(map[string]struct{}, len(dirs))
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		cleanDir := filepath.Clean(dir)
		if _, ok := seen[cleanDir]; ok {
			continue
		}
		seen[cleanDir] = struct{}{}
		result = append(result, cleanDir)
	}
	return result
}
