package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ExitHealthStatus represents the health of a single chain exit node.
type ExitHealthStatus struct {
	ServerID    int64         `json:"server_id"`
	Online      bool          `json:"online"`
	Latency     time.Duration `json:"latency_ms"`
	LastCheckAt time.Time    `json:"last_check_at"`
	ConsecFail  int           `json:"consec_fail"`
	Health      string        `json:"health"` // healthy | degraded | down
}

// EntryHealthStatus represents the health of a chain entry node and all its exits.
type EntryHealthStatus struct {
	EntryServerID  int64               `json:"entry_server_id"`
	EntryInboundID int64               `json:"entry_inbound_id"`
	Online         bool               `json:"online"`
	Exits          []ExitHealthStatus  `json:"exits"`
	HealthyExits   int                `json:"healthy_exits"`
	TotalExits     int                `json:"total_exits"`
	LastCheckAt    time.Time          `json:"last_check_at"`
}

// ChainHealthMonitor monitors the health of chain proxy exits
// using the new chain_links table (not the legacy node_type/chain_config approach).
type ChainHealthMonitor struct {
	mu            sync.RWMutex
	nodeMgr       *NodeServerManager
	serverRepo    *repository.NodeServerRepository
	chainLinkRepo *repository.ChainLinkRepository
	healthStatus  map[int64]*ExitHealthStatus // exit_server_id → status
	checkInterval time.Duration
	failThreshold int

	// Callback when health status changes (triggers config regeneration)
	onStatusChange func()
}

// NewChainHealthMonitor creates a new ChainHealthMonitor.
func NewChainHealthMonitor(
	nodeMgr *NodeServerManager,
	serverRepo *repository.NodeServerRepository,
	chainLinkRepo *repository.ChainLinkRepository,
) *ChainHealthMonitor {
	return &ChainHealthMonitor{
		nodeMgr:        nodeMgr,
		serverRepo:     serverRepo,
		chainLinkRepo:  chainLinkRepo,
		healthStatus:   make(map[int64]*ExitHealthStatus),
		checkInterval:  30 * time.Second,
		failThreshold:  3,
	}
}

// WithCheckInterval sets the health check interval.
func (m *ChainHealthMonitor) WithCheckInterval(d time.Duration) *ChainHealthMonitor {
	m.checkInterval = d
	return m
}

// WithFailThreshold sets the consecutive failure count before marking an exit as down.
func (m *ChainHealthMonitor) WithFailThreshold(n int) *ChainHealthMonitor {
	m.failThreshold = n
	return m
}

// OnStatusChange sets a callback invoked when any exit health status changes.
func (m *ChainHealthMonitor) OnStatusChange(fn func()) {
	m.onStatusChange = fn
}

// RunLoop starts the health monitoring loop. Blocks until ctx is cancelled.
func (m *ChainHealthMonitor) RunLoop(ctx context.Context) {
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

// checkAllExits discovers all unique exit servers from chain_links and checks them.
func (m *ChainHealthMonitor) checkAllExits(ctx context.Context) {
	links, err := m.chainLinkRepo.ListAll(ctx)
	if err != nil {
		log.Printf("[ChainHealth] failed to list chain links: %v", err)
		return
	}

	// Collect unique exit server IDs
	exitIDs := make(map[int64]struct{})
	for _, link := range links {
		if !link.Enabled {
			continue
		}
		if link.ExitExternalID > 0 {
			// External exit — skip gRPC health check (no Agent)
			continue
		}
		if link.ExitServerID > 0 {
			exitIDs[link.ExitServerID] = struct{}{}
		}
	}

	var wg sync.WaitGroup
	changed := false

	for exitID := range exitIDs {
		wg.Add(1)
		go func(serverID int64) {
			defer wg.Done()
			if m.checkSingleExit(ctx, serverID) {
				changed = true
			}
		}(exitID)
	}
	wg.Wait()

	if changed && m.onStatusChange != nil {
		m.onStatusChange()
	}
}

// checkSingleExit checks one exit node's health. Returns true if status changed.
func (m *ChainHealthMonitor) checkSingleExit(ctx context.Context, serverID int64) bool {
	status := &ExitHealthStatus{
		ServerID:    serverID,
		LastCheckAt: time.Now(),
	}

	// Check if the exit server's Agent is online
	start := time.Now()
	online := m.nodeMgr.IsOnline(serverID)
	status.Latency = time.Since(start)
	status.Online = online

	if online {
		status.ConsecFail = 0
		status.Health = "healthy"
	} else {
		m.mu.RLock()
		prev := m.healthStatus[serverID]
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

	m.mu.Lock()
	changed := m.isStatusDifferent(serverID, status)
	m.healthStatus[serverID] = status
	m.mu.Unlock()

	if changed {
		log.Printf("[ChainHealth] exit %d status changed: %s (online=%v, fails=%d, latency=%v)",
			serverID, status.Health, status.Online, status.ConsecFail, status.Latency)
	}

	return changed
}

// isStatusDifferent compares new status with stored status. Caller must hold m.mu.
func (m *ChainHealthMonitor) isStatusDifferent(serverID int64, newStatus *ExitHealthStatus) bool {
	old, ok := m.healthStatus[serverID]
	if !ok {
		return true // New entry
	}
	return old.Health != newStatus.Health
}

// GetExitStatus returns the health status of a specific exit.
func (m *ChainHealthMonitor) GetExitStatus(serverID int64) *ExitHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.healthStatus[serverID]; ok {
		return s
	}
	return nil
}

// GetAllExitStatuses returns the health status of all exits.
func (m *ChainHealthMonitor) GetAllExitStatuses() map[int64]*ExitHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[int64]*ExitHealthStatus, len(m.healthStatus))
	for k, v := range m.healthStatus {
		result[k] = v
	}
	return result
}

// GetEntryHealthStatus returns the health status for a chain entry,
// including the health of all its exit nodes.
func (m *ChainHealthMonitor) GetEntryHealthStatus(ctx context.Context, entryServerID int64, entryInboundID int64) *EntryHealthStatus {
	links, err := m.chainLinkRepo.ListByEntry(ctx, entryServerID, entryInboundID)
	if err != nil {
		return nil
	}

	entryStatus := &EntryHealthStatus{
		EntryServerID:  entryServerID,
		EntryInboundID: entryInboundID,
		Online:         m.nodeMgr.IsOnline(entryServerID),
		LastCheckAt:   time.Now(),
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	seenExits := make(map[int64]struct{})
	for _, link := range links {
		if link.ExitExternalID > 0 {
			// External exits are always considered healthy (no Agent to check)
			entryStatus.Exits = append(entryStatus.Exits, ExitHealthStatus{
				ServerID: link.ExitExternalID,
				Online:   true,
				Health:   "healthy",
			})
			entryStatus.HealthyExits++
			entryStatus.TotalExits++
			continue
		}
		if _, seen := seenExits[link.ExitServerID]; seen {
			continue
		}
		seenExits[link.ExitServerID] = struct{}{}

		status, ok := m.healthStatus[link.ExitServerID]
		if !ok {
			status = &ExitHealthStatus{
				ServerID: link.ExitServerID,
				Health:   "degraded",
			}
		}
		entryStatus.Exits = append(entryStatus.Exits, *status)
		entryStatus.TotalExits++
		if status.Health == "healthy" {
			entryStatus.HealthyExits++
		}
	}

	return entryStatus
}

// GetAllEntriesHealthStatus returns health status for all chain entries.
func (m *ChainHealthMonitor) GetAllEntriesHealthStatus(ctx context.Context) []EntryHealthStatus {
	links, err := m.chainLinkRepo.ListAll(ctx)
	if err != nil {
		return nil
	}

	// Group by entry (server_id + inbound_id)
	type entryKey struct {
		ServerID  int64
		InboundID int64
	}
	entries := make(map[entryKey][]entity.ChainLink)
	for _, link := range links {
		if !link.Enabled {
			continue
		}
		key := entryKey{link.EntryServerID, link.EntryInboundID}
		entries[key] = append(entries[key], link)
	}

	var result []EntryHealthStatus
	for key, entryLinks := range entries {
		status := &EntryHealthStatus{
			EntryServerID:  key.ServerID,
			EntryInboundID: key.InboundID,
			Online:         m.nodeMgr.IsOnline(key.ServerID),
			LastCheckAt:   time.Now(),
		}

		m.mu.RLock()
		seenExits := make(map[int64]struct{})
		for _, link := range entryLinks {
			if link.ExitExternalID > 0 {
				status.Exits = append(status.Exits, ExitHealthStatus{
					ServerID: link.ExitExternalID,
					Online:   true,
					Health:   "healthy",
				})
				status.HealthyExits++
				status.TotalExits++
				continue
			}
			if _, seen := seenExits[link.ExitServerID]; seen {
				continue
			}
			seenExits[link.ExitServerID] = struct{}{}

			exitStatus, ok := m.healthStatus[link.ExitServerID]
			if !ok {
				exitStatus = &ExitHealthStatus{
					ServerID: link.ExitServerID,
					Health:   "degraded",
				}
			}
			status.Exits = append(status.Exits, *exitStatus)
			status.TotalExits++
			if exitStatus.Health == "healthy" {
				status.HealthyExits++
			}
		}
		m.mu.RUnlock()

		result = append(result, *status)
	}

	return result
}

// ChainHealthMonitorRegistry manages the health monitor for all chain links.
// In the new model there is a single monitor that checks all exit nodes
// referenced by chain_links (instead of per-entry monitors in the old model).
type ChainHealthMonitorRegistry struct {
	mu          sync.RWMutex
	monitor     *ChainHealthMonitor
	nodeMgr     *NodeServerManager
	serverRepo  *repository.NodeServerRepository
	chainLinkRepo *repository.ChainLinkRepository
	started     bool
	cancel      context.CancelFunc
}

// NewChainHealthMonitorRegistry creates a new registry.
func NewChainHealthMonitorRegistry(nodeMgr *NodeServerManager, serverRepo *repository.NodeServerRepository) *ChainHealthMonitorRegistry {
	return &ChainHealthMonitorRegistry{
		nodeMgr:    nodeMgr,
		serverRepo: serverRepo,
	}
}

// WithChainRepos injects the chain link repository.
func (r *ChainHealthMonitorRegistry) WithChainRepos(chainLinkRepo *repository.ChainLinkRepository) *ChainHealthMonitorRegistry {
	r.chainLinkRepo = chainLinkRepo
	return r
}

// StartAll starts the health monitor for all chain links.
func (r *ChainHealthMonitorRegistry) StartAll(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return nil
	}

	if r.chainLinkRepo == nil {
		log.Println("[ChainHealthRegistry] chain link repo not set, skipping")
		return nil
	}

	monitorCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	monitor := NewChainHealthMonitor(r.nodeMgr, r.serverRepo, r.chainLinkRepo)
	monitor.OnStatusChange(func() {
		log.Println("[ChainHealthRegistry] exit status changed, config sync will pick up on next cycle")
	})

	r.monitor = monitor
	r.started = true

	go monitor.RunLoop(monitorCtx)

	log.Println("[ChainHealthRegistry] health monitor started")
	return nil
}

// GetMonitor returns the health monitor.
func (r *ChainHealthMonitorRegistry) GetMonitor() *ChainHealthMonitor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.monitor
}

// StopAll stops the health monitor.
func (r *ChainHealthMonitorRegistry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		r.cancel()
	}
	r.started = false
	r.monitor = nil
	log.Println("[ChainHealthRegistry] health monitor stopped")
}
