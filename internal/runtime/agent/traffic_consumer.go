package agent

import (
	"context"
	"log"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// NodeTrafficAccumulator accumulates xray user traffic deltas into a
// node-level monthly traffic record. Implemented by
// nodeserver.NodeTrafficQuotaService.AddUserTrafficDelta.
type NodeTrafficAccumulator interface {
	AddUserTrafficDelta(serverID int64, upDelta, downDelta int64)
}

// InboundConfigUpdater is the interface needed by TrafficEventConsumer
// to persist traffic deltas into the database.
// Implemented by inboundconfig.Service.
type InboundConfigUpdater interface {
	UpdateTraffic(ctx context.Context, tag string, email string, upDelta int64, downDelta int64) error
}

// TrafficEventConsumer consumes TrafficEvent from the NodeServerManager's
// onTraffic channel and persists traffic deltas to the database.
//
// It handles:
//   - Mirror node traffic attribution (via BillingFilter → primary server)
//   - Database persistence (via InboundConfigUpdater)
//   - Graceful shutdown via context cancellation
type TrafficEventConsumer struct {
	nodeMgr           *NodeServerManager
	billingFilter     *BillingFilter
	inboundConfigSvc  InboundConfigUpdater
	serverRepo        *repository.NodeServerRepository
	nodeAccum         NodeTrafficAccumulator
}

// NewTrafficEventConsumer creates a new TrafficEventConsumer.
func NewTrafficEventConsumer(
	nodeMgr *NodeServerManager,
	billingFilter *BillingFilter,
	inboundConfigSvc InboundConfigUpdater,
) *TrafficEventConsumer {
	return &TrafficEventConsumer{
		nodeMgr:          nodeMgr,
		billingFilter:    billingFilter,
		inboundConfigSvc: inboundConfigSvc,
	}
}

// WithServerRepo sets the server repository for mirror traffic attribution.
// Optional: if not set, mirror traffic will not be redirected to primary.
func (c *TrafficEventConsumer) WithServerRepo(repo *repository.NodeServerRepository) *TrafficEventConsumer {
	c.serverRepo = repo
	return c
}

// WithNodeAccumulator injects a node traffic accumulator to also update
// node-level monthly traffic records when processing remote node traffic events.
func (c *TrafficEventConsumer) WithNodeAccumulator(acc NodeTrafficAccumulator) *TrafficEventConsumer {
	c.nodeAccum = acc
	return c
}

// Start starts consuming traffic events. Blocks until ctx is cancelled.
func (c *TrafficEventConsumer) Start(ctx context.Context) {
	log.Println("[TrafficEventConsumer] started, listening for remote node traffic events")

	// Preload server list for mirror attribution if repo is available
	var servers []entity.NodeServer
	if c.serverRepo != nil {
		var err error
		servers, err = c.serverRepo.ListAll(ctx)
		if err != nil {
			log.Printf("[TrafficEventConsumer] warning: failed to load servers for mirror attribution: %v", err)
		}
	}

	ch := c.nodeMgr.TrafficEvents()

	for {
		select {
		case <-ctx.Done():
			log.Println("[TrafficEventConsumer] stopped")
			return

		case event := <-ch:
			c.processEvent(ctx, event, servers)
		}
	}
}

// processEvent handles a single TrafficEvent: applies mirror attribution
// and persists the traffic delta to the database.
func (c *TrafficEventConsumer) processEvent(ctx context.Context, event *TrafficEvent, servers []entity.NodeServer) {
	if event == nil {
		return
	}

	// Apply mirror traffic attribution if we have server data
	processed := event
	if len(servers) > 0 {
		processed = MergeTrafficToPrimary(servers, event)
	}

	// Skip if billing filter says this server should not collect traffic
	if c.billingFilter != nil {
		if !c.billingFilter.ShouldCollectTraffic(ctx, processed.ServerID) {
			return
		}
	}

	// Persist to database
	persistCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := c.inboundConfigSvc.UpdateTraffic(persistCtx, "", processed.Email, processed.Uplink, processed.Downlink); err != nil {
		log.Printf("[TrafficEventConsumer] failed to update traffic for %s (server %d): %v",
			processed.Email, processed.ServerID, err)
	}

	// Also accumulate into node-level monthly traffic record
	if c.nodeAccum != nil && (processed.Uplink > 0 || processed.Downlink > 0) {
		c.nodeAccum.AddUserTrafficDelta(processed.ServerID, processed.Uplink, processed.Downlink)
	}
}
