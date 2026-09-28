package healthcheck

import (
	"context"
	"log"
	"sync"
	"time"
)

// RestartFunc is called to restart a service.
type RestartFunc func(ctx context.Context) error

// IsRunningFunc is called to check if a process is running.
type IsRunningFunc func() bool

// OnCrashFunc is called when a crash is detected, before the restart attempt.
// This allows callers to record crash logs, update metrics, etc.
type OnCrashFunc func(name string, attempt int, maxRestarts int)

// Guardian monitors a process and handles auto-restart with backoff.
type Guardian struct {
	mu                 sync.Mutex
	name               string
	serviceID          int64
	pid                int
	state              string // "running", "stopped", "crashed"
	startedAt          time.Time
	lastCrashAt        time.Time
	lastExitCode       int
	currentRestarts    int
	maxRestarts        int
	restartWindowSec   int
	stableThresholdSec int
	windowStartAt      time.Time
	manuallyStopped    bool
	autoRestart        bool
	checkInterval      time.Duration
	failCount          int
	failThreshold      int
	stopCh             chan struct{}
	isRunning          IsRunningFunc
	restartFn          RestartFunc
	onCrashFn          OnCrashFunc
}

// Config holds guardian configuration.
type Config struct {
	Name               string
	ServiceID          int64
	MaxRestarts        int
	RestartWindowSec   int
	StableThresholdSec int
	CheckInterval      time.Duration
	FailThreshold      int
}

// DefaultConfig returns default guardian configuration.
func DefaultConfig(name string, serviceID int64) Config {
	return Config{
		Name:               name,
		ServiceID:          serviceID,
		MaxRestarts:        5,
		RestartWindowSec:   600, // 10 minutes
		StableThresholdSec: 300, // 5 minutes
		CheckInterval:      30 * time.Second,
		FailThreshold:      2,
	}
}

// New creates a new Guardian.
func New(cfg Config, isRunning IsRunningFunc, restartFn RestartFunc) *Guardian {
	return &Guardian{
		name:               cfg.Name,
		serviceID:          cfg.ServiceID,
		state:              "stopped",
		maxRestarts:        cfg.MaxRestarts,
		restartWindowSec:   cfg.RestartWindowSec,
		stableThresholdSec: cfg.StableThresholdSec,
		checkInterval:      cfg.CheckInterval,
		failThreshold:      cfg.FailThreshold,
		autoRestart:        false, // Default off — callers must explicitly enable via SetAutoRestart
		stopCh:             make(chan struct{}, 1),
		isRunning:          isRunning,
		restartFn:          restartFn,
	}
}

// OnCrash sets a callback that is invoked when a crash is detected,
// before the restart attempt. This allows callers to record crash logs,
// update metrics, or send alerts.
func (g *Guardian) OnCrash(fn OnCrashFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onCrashFn = fn
}

// Start begins monitoring a running process.
func (g *Guardian) Start(pid int) {
	g.mu.Lock()
	g.pid = pid
	g.state = "running"
	g.startedAt = time.Now()
	g.failCount = 0
	g.manuallyStopped = false
	g.mu.Unlock()

	go g.monitorLoop()
}

// SetStartedAt overrides the startedAt timestamp. This is used when attaching
// to an already-running process: the real start time is read from /proc/<pid>/stat
// and set here so that uptime calculations are accurate.
func (g *Guardian) SetStartedAt(t time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !t.IsZero() {
		g.startedAt = t
	}
}

// Stop stops the guardian and marks as manually stopped.
func (g *Guardian) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state = "stopped"
	g.manuallyStopped = true
	g.pid = 0
	select {
	case g.stopCh <- struct{}{}:
	default:
	}
}

// MarkCrashed marks the process as crashed (for external use).
func (g *Guardian) MarkCrashed() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state = "crashed"
	g.lastCrashAt = time.Now()
	g.pid = 0
}

// Status returns the guardian status.
func (g *Guardian) Status() map[string]interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	return map[string]interface{}{
		"name":             g.name,
		"service_id":       g.serviceID,
		"state":            g.state,
		"pid":              g.pid,
		"started_at":       g.startedAt,
		"last_crash_at":    g.lastCrashAt,
		"last_exit_code":   g.lastExitCode,
		"current_restarts": g.currentRestarts,
		"max_restarts":     g.maxRestarts,
		"auto_restart":     g.autoRestart,
		"manually_stopped": g.manuallyStopped,
	}
}

// IsManuallyStopped returns whether the process was manually stopped.
func (g *Guardian) IsManuallyStopped() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.manuallyStopped
}

// AutoRestart returns whether auto-restart is enabled.
func (g *Guardian) AutoRestart() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.autoRestart
}

// SetAutoRestart enables or disables auto-restart.
func (g *Guardian) SetAutoRestart(enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.autoRestart = enabled
}

// State returns the current state.
func (g *Guardian) State() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state
}

// PID returns the current PID.
func (g *Guardian) PID() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pid
}

// CurrentRestarts returns the current restart count.
func (g *Guardian) CurrentRestarts() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.currentRestarts
}

// MaxRestarts returns the max restart count.
func (g *Guardian) MaxRestarts() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.maxRestarts
}

// SetMaxRestarts updates the max restart count.
func (g *Guardian) SetMaxRestarts(n int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n > 0 {
		g.maxRestarts = n
	}
}

// ResetRestarts resets the restart counter when process is stable.
func (g *Guardian) ResetRestarts() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.state == "running" && time.Since(g.startedAt) > time.Duration(g.stableThresholdSec)*time.Second {
		g.currentRestarts = 0
		g.windowStartAt = time.Time{}
	}
}

// monitorLoop periodically checks if the process is running.
func (g *Guardian) monitorLoop() {
	ticker := time.NewTicker(g.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.check()
		}
	}
}

// check performs a single health check.
func (g *Guardian) check() {
	g.mu.Lock()
	if g.manuallyStopped || g.state != "running" {
		g.mu.Unlock()
		return
	}

	running := g.isRunning()
	if running {
		g.failCount = 0
		g.mu.Unlock()
		// Check if we should reset restart counter (stable)
		g.ResetRestarts()
		return
	}

	// Process is dead
	g.failCount++
	failCount := g.failCount
	g.mu.Unlock()

	log.Printf("healthcheck %s: process dead, check %d/%d", g.name, failCount, g.failThreshold)

	if failCount < g.failThreshold {
		return
	}

	g.handleCrash()
}

// handleCrash handles a process crash with restart logic.
func (g *Guardian) handleCrash() {
	g.mu.Lock()

	if g.manuallyStopped {
		g.mu.Unlock()
		return
	}

	// Mark as crashed
	g.state = "crashed"
	g.lastCrashAt = time.Now()
	g.pid = 0
	g.failCount = 0

	log.Printf("healthcheck %s: crashed (restarts: %d/%d)", g.name, g.currentRestarts, g.maxRestarts)

	// Invoke crash callback if set (for logging, metrics, alerts)
	if g.onCrashFn != nil {
		g.mu.Unlock()
		g.onCrashFn(g.name, g.currentRestarts, g.maxRestarts)
		g.mu.Lock()
	}

	// Check if auto-restart is disabled
	if !g.autoRestart {
		log.Printf("healthcheck %s: auto-restart disabled, not restarting", g.name)
		g.mu.Unlock()
		return
	}

	// Check if max restarts reached
	if g.currentRestarts >= g.maxRestarts {
		log.Printf("healthcheck %s: max restart attempts (%d) reached, giving up", g.name, g.maxRestarts)
		g.mu.Unlock()
		return
	}

	// Check restart window
	if g.windowStartAt.IsZero() || time.Since(g.windowStartAt) > time.Duration(g.restartWindowSec)*time.Second {
		g.windowStartAt = time.Now()
		g.currentRestarts = 0
	}

	g.currentRestarts++

	// Exponential backoff: 2^(restarts-1) * 2 seconds, max 80s
	backoff := time.Duration(1<<uint(g.currentRestarts-1)*2) * time.Second
	if backoff > 80*time.Second {
		backoff = 80 * time.Second
	}

	log.Printf("healthcheck %s: restarting in %v (attempt %d/%d)", g.name, backoff, g.currentRestarts, g.maxRestarts)
	g.mu.Unlock()

	// Restart after backoff
	go func() {
		time.Sleep(backoff)

		g.mu.Lock()
		if g.manuallyStopped {
			g.mu.Unlock()
			return
		}
		g.mu.Unlock()

		ctx := context.Background()
		if err := g.restartFn(ctx); err != nil {
			log.Printf("healthcheck %s: restart failed: %v", g.name, err)
			g.mu.Lock()
			g.state = "crashed"
			g.mu.Unlock()
		} else {
			// restartFn (StartProxy / process.Manager.Start) creates a NEW
			// guardian and registers it in the manager's map. This old guardian
			// must stop its monitorLoop to avoid two guardians racing on the
			// same process (duplicate crash detection, duplicate restarts).
			log.Printf("healthcheck %s: restarted successfully, old guardian stopping", g.name)
			g.mu.Lock()
			g.state = "stopped"
			g.mu.Unlock()
			// Signal monitorLoop to exit.
			select {
			case g.stopCh <- struct{}{}:
			default:
			}
		}
	}()
}
