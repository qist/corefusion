package process

import (
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
	"corefusion/internal/store/repository"
)

// Manager manages core process lifecycle with persistence and daemon support.
type Manager struct {
	processes map[int64]*managedProcess
	guardians map[int64]*healthcheck.Guardian
	mu        sync.RWMutex
	dataDir   string
	svcRepo   *repository.ServiceRepository
	guardRepo *repository.ProcessGuardianRepository
	crashRepo *repository.CrashLogRepository
	resolver  BinaryResolver
	configGen       ConfigGenerator
	skipConfigGen   atomic.Bool // set by WriteRuntimeConfig to skip configGen in next Start()
	// configMu protects concurrent writes to runtime config files.
	// Both configsync.Service and LocalAgent.SyncConfig may write to the same
	// runtime/<coreType>.jsonc file; this mutex ensures atomic writes.
	configMu sync.Mutex
}

// ConfigGenerator generates a config file for a core type before starting.
// It should write the config to configPath and return nil on success.
type ConfigGenerator func(ctx context.Context, coreType string, configPath string) error

// SetConfigGenerator sets the config generator callback.
func (m *Manager) SetConfigGenerator(gen ConfigGenerator) {
	m.configGen = gen
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
		processes: make(map[int64]*managedProcess),
		guardians: make(map[int64]*healthcheck.Guardian),
		dataDir:   absDir,
		svcRepo:   svcRepo,
		guardRepo: guardRepo,
		crashRepo: crashRepo,
		resolver:  resolver,
	}
}

// WriteRuntimeConfig writes a core configuration file to the runtime directory
// under data/runtime/<coreType>.jsonc. It uses a mutex to prevent concurrent
// writes from configsync.Service and LocalAgent.SyncConfig, which both write
// to the same path.
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
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
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

	// Kill existing process if any
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

	// Log file
	logDir := filepath.Join(m.dataDir, "logs")
	os.MkdirAll(logDir, 0750)
	logPath := filepath.Join(logDir, coreType+".log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	// Use relative paths from dataDir (same as proxy)
	relBinPath, _ := filepath.Rel(m.dataDir, binaryPath)
	relConfigPath, _ := filepath.Rel(m.dataDir, configPath)

	// Ensure geo files are accessible from the panel's lookup paths.
	rulesDir := filepath.Join(m.dataDir, "rules")
	if _, err := os.Stat(rulesDir); err == nil {
		symlinkGeoFiles(rulesDir)
	}

	cmd := exec.CommandContext(context.Background(), relBinPath, "-c", relConfigPath)
	cmd.Dir = m.dataDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// Set resource path for geoip/geosite files
	xrayAssetPath := os.Getenv("XRAY_LOCATION_ASSET")
	if xrayAssetPath == "" {
		xrayAssetPath = rulesDir
	}
	cmd.Env = append(os.Environ(), "XRAY_LOCATION_ASSET="+xrayAssetPath)
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
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
