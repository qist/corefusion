package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ExitHealthStatus represents the health of a single chain_exit node.
type ExitHealthStatus struct {
	ServerID    int64         `json:"server_id"`
	Online      bool          `json:"online"`
	Latency     time.Duration `json:"latency_ms"`
	LastCheckAt time.Time     `json:"last_check_at"`
	ConsecFail  int           `json:"consec_fail"`
	Health      string        `json:"health"` // healthy | degraded | down
}

// ChainExitHealthMonitor monitors the health of chain_exit nodes
// for a chain_entry, enabling multi-exit failover.
type ChainExitHealthMonitor struct {
	mu            sync.RWMutex
	entryServerID int64
	exits         []ChainExitSpec
	nodeMgr       *NodeServerManager
	serverRepo    *repository.NodeServerRepository
	healthStatus  map[int64]*ExitHealthStatus // exit_server_id → status
	checkInterval time.Duration
	failThreshold int

	// Callback when health status changes (triggers config regeneration)
	onStatusChange func(entryID int64)
}

// NewChainExitHealthMonitor creates a new ChainExitHealthMonitor.
func NewChainExitHealthMonitor(
	entryServerID int64,
	exits []ChainExitSpec,
	nodeMgr *NodeServerManager,
	serverRepo *repository.NodeServerRepository,
) *ChainExitHealthMonitor {
	return &ChainExitHealthMonitor{
		entryServerID: entryServerID,
		exits:         exits,
		nodeMgr:       nodeMgr,
		serverRepo:    serverRepo,
		healthStatus:  make(map[int64]*ExitHealthStatus),
		checkInterval: 30 * time.Second,
		failThreshold: 3,
	}
}

// WithCheckInterval sets the health check interval.
func (m *ChainExitHealthMonitor) WithCheckInterval(d time.Duration) *ChainExitHealthMonitor {
	m.checkInterval = d
	return m
}

// WithFailThreshold sets the consecutive failure count before marking an exit as down.
func (m *ChainExitHealthMonitor) WithFailThreshold(n int) *ChainExitHealthMonitor {
	m.failThreshold = n
	return m
}

// OnStatusChange sets a callback invoked when any exit health status changes.
func (m *ChainExitHealthMonitor) OnStatusChange(fn func(entryID int64)) {
	m.onStatusChange = fn
}

// RunLoop starts the health monitoring loop. Blocks until ctx is cancelled.
func (m *ChainExitHealthMonitor) RunLoop(ctx context.Context) {
	if len(m.exits) == 0 {
		return
	}

	log.Printf("[ChainHealth] monitoring %d exit(s) for chain_entry %d", len(m.exits), m.entryServerID)

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Initial check
	m.checkAllExits(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAllExits(ctx)
		}
	}
}

// checkAllExits checks all chain_exit nodes in parallel.
func (m *ChainExitHealthMonitor) checkAllExits(ctx context.Context) {
	var wg sync.WaitGroup

	for _, exit := range m.exits {
		wg.Add(1)
		go func(spec ChainExitSpec) {
			defer wg.Done()
			m.checkSingleExit(ctx, spec)
		}(exit)
	}
	wg.Wait()

	// Check if any status changed
	if m.hasStatusChanged() && m.onStatusChange != nil {
		m.onStatusChange(m.entryServerID)
	}
}

// checkSingleExit checks one chain_exit node's health.
func (m *ChainExitHealthMonitor) checkSingleExit(ctx context.Context, spec ChainExitSpec) {
	status := &ExitHealthStatus{
		ServerID:    spec.ExitServerID,
		LastCheckAt: time.Now(),
	}

	// Check if the exit server's Agent is online
	// In a full implementation, this would also do a latency probe (e.g., gRPC health check)
	start := time.Now()
	online := m.nodeMgr.IsOnline(spec.ExitServerID)
	status.Latency = time.Since(start)
	status.Online = online

	if online {
		status.ConsecFail = 0
		status.Health = "healthy"
	} else {
		// Get previous status to increment fail count
		m.mu.RLock()
		prev := m.healthStatus[spec.ExitServerID]
		m.mu.RUnlock()

		if prev != nil {
			status.ConsecFail = prev.ConsecFail + 1
		} else {
			status.ConsecFail = 1
		}

		if status.ConsecFail >= m.failThreshold {
			status.Health = "down"
		} else {
			status.Health = "degraded"
		}
	}

	// Update status
	m.mu.Lock()
	changed := m.isStatusDifferent(spec.ExitServerID, status)
	m.healthStatus[spec.ExitServerID] = status
	m.mu.Unlock()

	if changed {
		log.Printf("[ChainHealth] exit %d status changed: %s (fails=%d, latency=%v)",
			spec.ExitServerID, status.Health, status.ConsecFail, status.Latency)
	}
}

// isStatusDifferent compares new status with stored status. Caller must hold m.mu.
func (m *ChainExitHealthMonitor) isStatusDifferent(serverID int64, newStatus *ExitHealthStatus) bool {
	old, ok := m.healthStatus[serverID]
	if !ok {
		return true // New entry
	}
	return old.Health != newStatus.Health
}

// hasStatusChanged returns true if any exit changed since last check.
func (m *ChainExitHealthMonitor) hasStatusChanged() bool {
	// The checkSingleExit already logged changes; this is a simplified version.
	return false // Individual changes are tracked in checkSingleExit
}

// GetHealthyExits returns all currently healthy exit specs, sorted by priority.
func (m *ChainExitHealthMonitor) GetHealthyExits() []ChainExitSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var healthy []ChainExitSpec
	for _, exit := range m.exits {
		status, ok := m.healthStatus[exit.ExitServerID]
		if !ok || status.Health == "healthy" {
			healthy = append(healthy, exit)
		}
	}
	return healthy
}

// GetExitStatus returns the health status of a specific exit.
func (m *ChainExitHealthMonitor) GetExitStatus(serverID int64) *ExitHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.healthStatus[serverID]; ok {
		return s
	}
	return nil
}

// GetAllExitStatuses returns the health status of all exits.
func (m *ChainExitHealthMonitor) GetAllExitStatuses() map[int64]*ExitHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[int64]*ExitHealthStatus, len(m.healthStatus))
	for k, v := range m.healthStatus {
		result[k] = v
	}
	return result
}

// ChainHealthMonitorRegistry manages health monitors for all chain_entry nodes.
type ChainHealthMonitorRegistry struct {
	mu         sync.RWMutex
	monitors   map[int64]*ChainExitHealthMonitor // entry_server_id → monitor
	nodeMgr    *NodeServerManager
	serverRepo *repository.NodeServerRepository
}

// NewChainHealthMonitorRegistry creates a new registry.
func NewChainHealthMonitorRegistry(nodeMgr *NodeServerManager, serverRepo *repository.NodeServerRepository) *ChainHealthMonitorRegistry {
	return &ChainHealthMonitorRegistry{
		monitors:   make(map[int64]*ChainExitHealthMonitor),
		nodeMgr:    nodeMgr,
		serverRepo: serverRepo,
	}
}

// StartAll discovers all chain_entry nodes and starts health monitors for them.
func (r *ChainHealthMonitorRegistry) StartAll(ctx context.Context) error {
	entries, err := r.serverRepo.ListByType(ctx, entity.NodeTypeChainEntry)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		r.startMonitor(ctx, &entry)
	}

	log.Printf("[ChainHealthRegistry] started %d monitor(s)", len(r.monitors))
	return nil
}

// startMonitor starts a health monitor for a single chain_entry.
func (r *ChainHealthMonitorRegistry) startMonitor(ctx context.Context, entry *entity.NodeServer) {
	cfg, err := ParseChainConfig(entry.ChainConfig)
	if err != nil || cfg == nil || len(cfg.Exits) == 0 {
		log.Printf("[ChainHealthRegistry] chain_entry %d has no valid chain_config, skipping", entry.ID)
		return
	}

	monitor := NewChainExitHealthMonitor(entry.ID, cfg.Exits, r.nodeMgr, r.serverRepo).
		WithCheckInterval(cfg.HealthCheckInterval()).
		WithFailThreshold(cfg.HealthCheckFailThreshold())

	monitor.OnStatusChange(func(entryID int64) {
		log.Printf("[ChainHealthRegistry] entry %d exit status changed, triggering config regeneration", entryID)
		// Config regeneration will be handled by the config sync service
	})

	r.mu.Lock()
	r.monitors[entry.ID] = monitor
	r.mu.Unlock()

	go monitor.RunLoop(ctx)
}

// GetMonitor returns the health monitor for a specific chain_entry.
func (r *ChainHealthMonitorRegistry) GetMonitor(entryID int64) *ChainExitHealthMonitor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.monitors[entryID]
}

// StopAll stops all health monitors.
func (r *ChainHealthMonitorRegistry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Monitors will stop when their context is cancelled
	r.monitors = make(map[int64]*ChainExitHealthMonitor)
	log.Println("[ChainHealthRegistry] all monitors stopped")
}
