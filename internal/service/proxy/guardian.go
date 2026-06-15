package proxy

import (
	"context"
	"log"
	"sync"
	"time"
)

// ProxyGuardian monitors a proxy process and handles auto-restart with backoff.
type ProxyGuardian struct {
	mu                sync.Mutex
	proxyType         ProxyType
	pid               int
	state             string // "running", "stopped", "crashed"
	startedAt         time.Time
	lastCrashAt       time.Time
	lastExitCode      int
	currentRestarts   int
	maxRestarts       int
	restartWindowSec  int
	stableThresholdSec int
	windowStartAt     time.Time
	manuallyStopped   bool
	stopCh            chan struct{}
	manager           *Manager
}

// NewProxyGuardian creates a new proxy guardian.
func NewProxyGuardian(proxyType ProxyType, manager *Manager) *ProxyGuardian {
	return &ProxyGuardian{
		proxyType:          proxyType,
		state:              "stopped",
		maxRestarts:        5,
		restartWindowSec:   600,  // 10 minutes
		stableThresholdSec: 300,  // 5 minutes
		stopCh:             make(chan struct{}),
		manager:            manager,
	}
}

// StartMonitoring starts monitoring a running proxy process.
func (g *ProxyGuardian) StartMonitoring(pid int) {
	g.mu.Lock()
	g.pid = pid
	g.state = "running"
	g.startedAt = time.Now()
	g.mu.Unlock()

	go g.monitorLoop()
}

// Stop stops the guardian monitoring.
func (g *ProxyGuardian) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.state = "stopped"
	g.manuallyStopped = true
	select {
	case g.stopCh <- struct{}{}:
	default:
	}
}

// MarkStopped marks the proxy as manually stopped.
func (g *ProxyGuardian) MarkStopped() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.manuallyStopped = true
	g.state = "stopped"
}

// ClearStopped clears the manually stopped flag.
func (g *ProxyGuardian) ClearStopped() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.manuallyStopped = false
}

// IsManuallyStopped returns whether the proxy was manually stopped.
func (g *ProxyGuardian) IsManuallyStopped() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.manuallyStopped
}

// Status returns the guardian status.
func (g *ProxyGuardian) Status() map[string]interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	return map[string]interface{}{
		"proxy_type":     string(g.proxyType),
		"state":          g.state,
		"pid":            g.pid,
		"started_at":     g.startedAt,
		"last_crash_at":  g.lastCrashAt,
		"last_exit_code": g.lastExitCode,
		"current_restarts": g.currentRestarts,
		"max_restarts":   g.maxRestarts,
		"manually_stopped": g.manuallyStopped,
	}
}

// monitorLoop watches the proxy process and handles crashes.
func (g *ProxyGuardian) monitorLoop() {
	for {
		select {
		case <-g.stopCh:
			return
		case <-time.After(2 * time.Second):
			if !g.isProcessRunning() {
				g.handleCrash()
				return
			}
		}
	}
}

// isProcessRunning checks if the proxy process is still running.
func (g *ProxyGuardian) isProcessRunning() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state != "running" {
		return false
	}

	info := g.manager.DetectProxy(g.proxyType)
	return info.Status == "running"
}

// handleCrash handles a proxy process crash.
func (g *ProxyGuardian) handleCrash() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.manuallyStopped || g.state == "stopped" {
		return
	}

	// Unexpected crash
	g.state = "crashed"
	g.lastCrashAt = time.Now()
	g.pid = 0

	log.Printf("proxy %s: crashed (restarts: %d/%d)", g.proxyType, g.currentRestarts, g.maxRestarts)

	// Check if auto-restart is allowed
	if g.currentRestarts >= g.maxRestarts {
		log.Printf("proxy %s: max restart attempts (%d) reached, giving up", g.proxyType, g.maxRestarts)
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

	log.Printf("proxy %s: restarting in %v (attempt %d/%d)", g.proxyType, backoff, g.currentRestarts, g.maxRestarts)

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
		if err := g.manager.StartProxy(ctx, g.proxyType); err != nil {
			log.Printf("proxy %s: restart failed: %v", g.proxyType, err)
			g.mu.Lock()
			g.state = "crashed"
			g.mu.Unlock()
		} else {
			log.Printf("proxy %s: restarted successfully", g.proxyType)
			g.mu.Lock()
			g.state = "running"
			g.startedAt = time.Now()
			g.mu.Unlock()
			// Continue monitoring
			g.monitorLoop()
		}
	}()
}

// ResetRestarts resets the restart counter (called when proxy is stable).
func (g *ProxyGuardian) ResetRestarts() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.state == "running" && time.Since(g.startedAt) > time.Duration(g.stableThresholdSec)*time.Second {
		g.currentRestarts = 0
		g.windowStartAt = time.Time{}
	}
}

// UpdateConfig updates the guardian configuration.
func (g *ProxyGuardian) UpdateConfig(autoRestart bool, maxRestarts int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if maxRestarts > 0 {
		g.maxRestarts = maxRestarts
	}
	// auto_restart is not stored in guardian, it's always enabled
	// The guardian only monitors, it doesn't decide whether to restart
}

// AutoRestart returns whether auto-restart is enabled (always true for guardian).
func (g *ProxyGuardian) AutoRestart() bool {
	return true
}
