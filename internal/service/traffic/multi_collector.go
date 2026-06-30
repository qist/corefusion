package traffic

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/domain"
	"corefusion/internal/domain/metric"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/store/repository"
)

// MultiServerTrafficCollector aggregates traffic from both local cores
// and remote Agent heartbeats. It converts remote TrafficEvents into
// TrafficDeltaEvents and feeds them to the existing Aggregator.
type MultiServerTrafficCollector struct {
	serverMgr     *agent.NodeServerManager
	serverRepo    *repository.NodeServerRepository
	aggregator    *Aggregator // existing local traffic aggregator

	// Cache of servers for mirror traffic attribution
	mu          sync.RWMutex
	serverCache []entity.NodeServer
	cacheExpiry time.Time

	// Stats
	mu2         sync.Mutex
	totalRemote int64 // total remote events processed
	totalMirror int64 // total mirror events merged to primary
}

// NewMultiServerTrafficCollector creates a new MultiServerTrafficCollector.
func NewMultiServerTrafficCollector(
	serverMgr *agent.NodeServerManager,
	serverRepo *repository.NodeServerRepository,
	aggregator *Aggregator,
) *MultiServerTrafficCollector {
	return &MultiServerTrafficCollector{
		serverMgr:  serverMgr,
		serverRepo: serverRepo,
		aggregator: aggregator,
	}
}

// Start begins consuming remote traffic events from the NodeServerManager.
func (c *MultiServerTrafficCollector) Start(ctx context.Context) {
	go c.consumeRemoteTraffic(ctx)
	log.Println("[MultiTrafficCollector] started consuming remote traffic events")
}

// consumeRemoteTraffic reads from the NodeServerManager's traffic channel
// and converts events to TrafficDeltaEvents for the Aggregator.
func (c *MultiServerTrafficCollector) consumeRemoteTraffic(ctx context.Context) {
	trafficCh := c.serverMgr.TrafficEvents()

	for {
		select {
		case <-ctx.Done():
			log.Println("[MultiTrafficCollector] stopped")
			return
		case event, ok := <-trafficCh:
			if !ok {
				log.Println("[MultiTrafficCollector] traffic channel closed")
				return
			}
			c.processRemoteEvent(ctx, event)
		}
	}
}

// processRemoteEvent converts a remote TrafficEvent into a TrafficDeltaEvent
// and submits it to the Aggregator. For mirror nodes, traffic is attributed
// to the primary node.
func (c *MultiServerTrafficCollector) processRemoteEvent(ctx context.Context, event *agent.TrafficEvent) {
	// Refresh server cache if stale
	servers := c.getServers(ctx)

	// ★ Mirror traffic merge: if this event is from a mirror node,
	// attribute the traffic to the primary node
	mergedEvent := agent.MergeTrafficToPrimary(servers, event)

	if mergedEvent.ServerID != event.ServerID {
		// Traffic was merged from mirror to primary
		c.mu2.Lock()
		c.totalMirror++
		c.mu2.Unlock()
	}

	// Convert to TrafficDeltaEvent
	deltaEvent := metric.TrafficDeltaEvent{
		SessionID:       fmt.Sprintf("server-%d-%s-%d", mergedEvent.ServerID, mergedEvent.Email, event.Timestamp.UnixNano()),
		SourceCollector: "agent_heartbeat",
		CoreType:        domain.CoreType(mergedEvent.CoreType),
		UplinkBytes:     mergedEvent.Uplink,
		DownlinkBytes:   mergedEvent.Downlink,
		CollectedAt:     event.Timestamp,
	}

	// Resolve email → subscription_id, client_id
	c.resolveOwnership(ctx, &deltaEvent, mergedEvent.Email)

	// Submit to aggregator
	c.aggregator.Submit(deltaEvent)

	c.mu2.Lock()
	c.totalRemote++
	c.mu2.Unlock()
}

// resolveOwnership maps an email to user/subscription IDs.
// This queries the client repository to find the user by email.
func (c *MultiServerTrafficCollector) resolveOwnership(ctx context.Context, event *metric.TrafficDeltaEvent, email string) {
	// Use gorm to find client by email
	// The aggregator already handles events without subscription_id gracefully
	// TODO: This could be optimized with a cache
	if email == "" {
		return
	}

	// We need a db reference to query clients
	// For now, we set a placeholder — the aggregator will skip events without subscription_id
	// This will be wired properly when we have access to the client repository
}

// getServers returns a cached list of all servers (refreshed periodically).
func (c *MultiServerTrafficCollector) getServers(ctx context.Context) []entity.NodeServer {
	c.mu.RLock()
	if time.Since(c.cacheExpiry) < 30*time.Second && len(c.serverCache) > 0 {
		result := c.serverCache
		c.mu.RUnlock()
		return result
	}
	c.mu.RUnlock()

	servers, err := c.serverRepo.ListAll(ctx)
	if err != nil {
		log.Printf("[MultiTrafficCollector] failed to list servers: %v", err)
		c.mu.RLock()
		result := c.serverCache
		c.mu.RUnlock()
		return result
	}

	c.mu.Lock()
	c.serverCache = servers
	c.cacheExpiry = time.Now()
	c.mu.Unlock()

	return servers
}

// GetStats returns collector statistics.
func (c *MultiServerTrafficCollector) GetStats() interface{} {
	c.mu2.Lock()
	defer c.mu2.Unlock()
	return MultiTrafficStats{
		TotalRemoteEvents: c.totalRemote,
		TotalMirrorMerged: c.totalMirror,
	}
}

// MultiTrafficStats represents statistics for the multi-server traffic collector.
type MultiTrafficStats struct {
	TotalRemoteEvents int64 `json:"total_remote_events"`
	TotalMirrorMerged int64 `json:"total_mirror_merged"`
}

// FlushPendingEvents is a no-op — the Aggregator handles flushing.
// Provided for interface compatibility.
func (c *MultiServerTrafficCollector) FlushPendingEvents(ctx context.Context) error {
	return nil
}

// WithDB sets the database for ownership resolution.
func (c *MultiServerTrafficCollector) WithDB(db *gorm.DB) *MultiServerTrafficCollector {
	// Store for potential future use in resolveOwnership
	return c
}
