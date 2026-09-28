package healthcheck

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// ServerStatusChecker is the interface for checking server status.
// agent.NodeServerManager implements this interface.
// Using an interface here avoids an import cycle (agent → process → healthcheck → agent).
type ServerStatusChecker interface {
	IsOnline(serverID int64) bool
	GetServerStatus(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error)
}

// MultiServerHealthChecker periodically checks the health of all remote node servers.
// It updates the agent_status in the database and triggers alerts when servers go offline.
// Local servers are skipped — they are monitored by the existing Guardian.
type MultiServerHealthChecker struct {
	serverMgr     ServerStatusChecker
	serverRepo    *repository.NodeServerRepository
	alertHandler  AlertHandler
	checkInterval time.Duration

	// Track previous states for online→offline detection
	mu         sync.RWMutex
	prevStates map[int64]string // server_id → last known status
}

// AlertHandler is a callback for alert events (webhook, email, Telegram, etc.)
type AlertHandler func(server *entity.NodeServer, alertType string, message string)

// NewMultiServerHealthChecker creates a new MultiServerHealthChecker.
func NewMultiServerHealthChecker(
	serverMgr ServerStatusChecker,
	serverRepo *repository.NodeServerRepository,
) *MultiServerHealthChecker {
	return &MultiServerHealthChecker{
		serverMgr:     serverMgr,
		serverRepo:    serverRepo,
		checkInterval: 30 * time.Second,
		prevStates:    make(map[int64]string),
	}
}

// WithCheckInterval sets the check interval.
func (c *MultiServerHealthChecker) WithCheckInterval(d time.Duration) *MultiServerHealthChecker {
	c.checkInterval = d
	return c
}

// WithAlertHandler sets the alert handler callback.
func (c *MultiServerHealthChecker) WithAlertHandler(handler AlertHandler) *MultiServerHealthChecker {
	c.alertHandler = handler
	return c
}

// RunLoop starts the health check loop. Blocks until ctx is cancelled.
func (c *MultiServerHealthChecker) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	log.Printf("[MultiHealthCheck] started, checking every %v", c.checkInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[MultiHealthCheck] stopped")
			return
		case <-ticker.C:
			if err := c.checkAll(ctx); err != nil {
				log.Printf("[MultiHealthCheck] check error: %v", err)
			}
		}
	}
}

// checkAll checks all remote servers and updates their status.
func (c *MultiServerHealthChecker) checkAll(ctx context.Context) error {
	servers, err := c.serverRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		if server.IsLocal {
			continue // Local servers are handled by Guardian
		}
		c.checkSingleServer(ctx, &server)
	}
	return nil
}

// checkSingleServer checks one server's health and updates its status.
func (c *MultiServerHealthChecker) checkSingleServer(ctx context.Context, server *entity.NodeServer) {
	status := c.determineStatus(ctx, server)

	// Get previous state
	c.mu.RLock()
	prevStatus := c.prevStates[server.ID]
	c.mu.RUnlock()

	// Update database if status changed
	if status != server.AgentStatus {
		log.Printf("[MultiHealthCheck] server %s (id=%d) status: %s → %s",
			server.Name, server.ID, server.AgentStatus, status)
		_ = c.serverRepo.UpdateAgentStatus(ctx, server.ID, status, server.AgentVersion)
	}

	// Track state
	c.mu.Lock()
	c.prevStates[server.ID] = status
	c.mu.Unlock()

	// Trigger alert on online→offline transition
	if status == entity.AgentStatusOffline && prevStatus == entity.AgentStatusOnline {
		c.triggerAlert(server, "server_offline",
			"Server %s (region: %s) went offline")
	}

	// Trigger alert on online→degraded transition
	if status == entity.AgentStatusDegraded && prevStatus == entity.AgentStatusOnline {
		c.triggerAlert(server, "server_degraded",
			"Server %s (region: %s) is degraded — some cores may be down")
	}

	// Alert on recovery
	if status == entity.AgentStatusOnline && prevStatus == entity.AgentStatusOffline {
		c.triggerAlert(server, "server_recovered",
			"Server %s (region: %s) is back online")
	}
}

// determineStatus checks the server's health and returns the appropriate status.
func (c *MultiServerHealthChecker) determineStatus(ctx context.Context, server *entity.NodeServer) string {
	// Check if Agent is online via heartbeat
	if !c.serverMgr.IsOnline(server.ID) {
		return entity.AgentStatusOffline
	}

	// Agent is connected — check core process status via gRPC
	resp, err := c.serverMgr.GetServerStatus(ctx, server.ID)
	if err != nil {
		// Can't get status, but Agent heartbeat is alive → degraded
		return entity.AgentStatusDegraded
	}

	// Check if all cores are running
	allRunning := true
	for _, core := range resp.Cores {
		if core.State != "running" {
			allRunning = false
			log.Printf("[MultiHealthCheck] server %s: core %s is %s",
				server.Name, core.CoreType, core.State)
		}
	}

	if allRunning {
		return entity.AgentStatusOnline
	}
	return entity.AgentStatusDegraded
}

// triggerAlert fires the alert handler if configured.
func (c *MultiServerHealthChecker) triggerAlert(server *entity.NodeServer, alertType, msgTemplate string) {
	log.Printf("[MultiHealthCheck] ALERT %s: server=%s region=%s type=%s",
		alertType, server.Name, server.Region, server.NodeType)

	if c.alertHandler != nil {
		c.alertHandler(server, alertType, msgTemplate)
	}
}

// GetServerStatusSummary returns a summary of all servers' health.
func (c *MultiServerHealthChecker) GetServerStatusSummary(ctx context.Context) (interface{}, error) {
	servers, err := c.serverRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	summary := &StatusSummary{
		Total:    len(servers),
		ByStatus: make(map[string]int),
		ByType:   make(map[string]int),
		ByRegion: make(map[string]int),
		Servers:  make([]ServerHealthInfo, 0, len(servers)),
	}

	for _, s := range servers {
		// Determine current status
		status := s.AgentStatus
		if !s.IsLocal {
			status = c.determineStatus(ctx, &s)
		}

		summary.ByStatus[status]++
		summary.ByType[s.NodeType]++
		summary.ByRegion[s.Region]++

		info := ServerHealthInfo{
			ID:             s.ID,
			Name:           s.Name,
			Hostname:       s.Hostname,
			Region:         s.Region,
			NodeType:       s.NodeType,
			Status:         status,
			Online:         status == entity.AgentStatusOnline,
			IsLocal:        s.IsLocal,
			BillingEnabled: s.BillingEnabled,
		}

		if s.LastHeartbeatAt != nil {
			info.LastHeartbeatAge = time.Since(*s.LastHeartbeatAt)
		}

		summary.Servers = append(summary.Servers, info)
	}

	summary.Online = summary.ByStatus[entity.AgentStatusOnline]
	summary.Offline = summary.ByStatus[entity.AgentStatusOffline]
	summary.Degraded = summary.ByStatus[entity.AgentStatusDegraded]

	return summary, nil
}

// StatusSummary represents an aggregate health summary of all servers.
type StatusSummary struct {
	Total    int                `json:"total"`
	Online   int                `json:"online"`
	Offline  int                `json:"offline"`
	Degraded int                `json:"degraded"`
	ByStatus map[string]int     `json:"by_status"`
	ByType   map[string]int     `json:"by_type"`
	ByRegion map[string]int     `json:"by_region"`
	Servers  []ServerHealthInfo `json:"servers"`
}

// ServerHealthInfo represents the health info of a single server.
type ServerHealthInfo struct {
	ID               int64         `json:"id"`
	Name             string        `json:"name"`
	Hostname         string        `json:"hostname"`
	Region           string        `json:"region"`
	NodeType         string        `json:"node_type"`
	Status           string        `json:"status"`
	Online           bool          `json:"online"`
	IsLocal          bool          `json:"is_local"`
	BillingEnabled   bool          `json:"billing_enabled"`
	LastHeartbeatAge time.Duration `json:"last_heartbeat_age_seconds"`
}
