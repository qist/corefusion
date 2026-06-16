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
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/store/repository"
)

// Manager manages core process lifecycle with persistence and daemon support.
type Manager struct {
	processes  map[int64]*managedProcess
	guardians  map[int64]*healthcheck.Guardian
	mu         sync.RWMutex
	dataDir    string
	svcRepo    *repository.ServiceRepository
	guardRepo  *repository.ProcessGuardianRepository
	crashRepo  *repository.CrashLogRepository
	resolver   BinaryResolver
	configGen  ConfigGenerator
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
	pid        int
	state      string
	startedAt  time.Time
	lastError  error
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
	return &Manager{
		processes: make(map[int64]*managedProcess),
		guardians: make(map[int64]*healthcheck.Guardian),
		dataDir:   dataDir,
		svcRepo:   svcRepo,
		guardRepo: guardRepo,
		crashRepo: crashRepo,
		resolver:  resolver,
	}
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
		services, _, err := m.svcRepo.List(ctx, c.CoreType, 1, 0)
		if err != nil {
			log.Printf("seed: failed to check %s: %v", c.CoreType, err)
			continue
		}
		if len(services) > 0 {
			continue
		}

		svc := &entity.InboundService{
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
	m.mu.Lock()
	defer m.mu.Unlock()

	// Kill existing process if any
	if p, ok := m.processes[serviceID]; ok {
		if p.cmd != nil && p.cmd.Process != nil {
			killProcess(p.cmd.Process.Pid)
		}
		delete(m.processes, serviceID)
	}

	// Resolve binary
	binaryPath, _, err := m.resolver.Resolve(coreType)
	if err != nil {
		return fmt.Errorf("binary not found for %s: %w", coreType, err)
	}

	// Config path - always regenerate from core_configs on start
	configDir := filepath.Join(m.dataDir, "runtime")
	os.MkdirAll(configDir, 0750)
	configPath := filepath.Join(configDir, coreType+".jsonc")
	if m.configGen != nil {
		if err := m.configGen(ctx, coreType, configPath); err != nil {
			return fmt.Errorf("generate config: %w", err)
		}
	} else {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
		}
	}

	// Log file
	logDir := filepath.Join(m.dataDir, "logs")
	os.MkdirAll(logDir, 0750)
	logPath := filepath.Join(logDir, coreType+".log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.CommandContext(context.Background(), binaryPath, "-c", configPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
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
		pid:        cmd.Process.Pid,
		state:      "running",
		startedAt:  time.Now(),
	}
	m.processes[serviceID] = p

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
func (m *Manager) createGuardian(serviceID int64, coreType string) *healthcheck.Guardian {
	cfg := healthcheck.DefaultConfig(coreType, serviceID)
	return healthcheck.New(cfg,
		func() bool {
			m.mu.RLock()
			p, ok := m.processes[serviceID]
			m.mu.RUnlock()
			if !ok || p.cmd == nil || p.cmd.Process == nil {
				return false
			}
			return isProcessAlive(p.pid)
		},
		func(ctx context.Context) error {
			// Reap zombie before restart
			m.reapZombies(serviceID)
			return m.Start(ctx, serviceID, coreType)
		},
	)
}

// Stop stops a core process.
func (m *Manager) Stop(ctx context.Context, serviceID int64) error {
	m.mu.Lock()
	p, ok := m.processes[serviceID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("service %d not found", serviceID)
	}
	if p.state != "running" {
		m.mu.Unlock()
		return fmt.Errorf("service %d is not running", serviceID)
	}
	p.state = "stopping"

	// Stop guardian to prevent auto-restart
	if g, ok := m.guardians[serviceID]; ok {
		g.Stop()
		delete(m.guardians, serviceID)
	}
	m.mu.Unlock()

	// Send SIGTERM to process group
	if p.cmd != nil && p.cmd.Process != nil {
		killProcess(p.pid)
	}

	// Wait for graceful shutdown and reap zombie
	done := make(chan error, 1)
	go func() {
		if p.cmd != nil {
			done <- p.cmd.Wait()
		} else {
			done <- nil
		}
	}()

	select {
	case <-ctx.Done():
		forceKillProcess(p.pid)
	case <-time.After(10 * time.Second):
		forceKillProcess(p.pid)
	case <-done:
	}

	m.mu.Lock()
	p.state = "stopped"
	m.mu.Unlock()

	// Mark manually stopped
	m.persistState(serviceID, "stopped", 0, true)
	log.Printf("stopped service %d", serviceID)
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
func (m *Manager) Restart(ctx context.Context, serviceID int64, coreType string) error {
	if err := m.Stop(ctx, serviceID); err != nil {
		log.Printf("stop before restart: %v", err)
	}
	time.Sleep(1 * time.Second)
	return m.Start(ctx, serviceID, coreType)
}

// GetStatus returns the status of a core process.
func (m *Manager) GetStatus(serviceID int64) (string, int, time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.processes[serviceID]
	if !ok {
		return "stopped", 0, time.Time{}, nil
	}
	return p.state, p.pid, p.startedAt, p.lastError
}

// IsRunning checks if a core process is running.
func (m *Manager) IsRunning(serviceID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.processes[serviceID]
	return ok && p.state == "running"
}

func (m *Manager) isRunning(serviceID int64) bool {
	p, ok := m.processes[serviceID]
	return ok && p.state == "running"
}

// ListAll returns the status of all managed processes.
func (m *Manager) ListAll() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ProcessInfo, 0, len(m.processes))
	for _, p := range m.processes {
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
	return result
}

// GetUptime returns the uptime of a running process in seconds.
func (m *Manager) GetUptime(serviceID int64) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.processes[serviceID]
	if !ok || p.state != "running" {
		return 0
	}
	return int64(time.Since(p.startedAt).Seconds())
}

// StopAll gracefully stops all running processes (called on CoreFusion shutdown).
func (m *Manager) StopAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all health check guardians
	for id, g := range m.guardians {
		g.Stop()
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
			ServiceID:       serviceID,
			AutoStart:       false,
			AutoRestart:     false,
			MaxRestartCount: 5,
			RestartWindowSec: 600,
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
