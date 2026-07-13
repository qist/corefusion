package usersync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/service/plan"
	"corefusion/internal/store/repository"
)

// UserSyncService fans out user credential changes to multiple node servers.
// It respects billing_enabled (skipping chain_exit, backup nodes) and
// queues failed operations for retry when servers come back online.
type UserSyncService struct {
	clientRepo    *repository.ClientRepository
	planSvc       *plan.Service
	inboundRepo   *repository.InboundConfigRepository
	serverRepo    *repository.NodeServerRepository
	nodeMgr       *agent.NodeServerManager
	billingFilter *agent.BillingFilter

	// Offline retry queue
	retryMu    sync.Mutex
	retryQueue []retryEntry
}

// retryEntry represents a pending user sync operation for an offline server.
type retryEntry struct {
	ClientID    int64
	ServerID    int64
	Action      string // add, remove, update
	QueuedAt    time.Time
	RetryCount  int
}

// NewUserSyncService creates a new UserSyncService.
func NewUserSyncService(
	clientRepo *repository.ClientRepository,
	planSvc *plan.Service,
	inboundRepo *repository.InboundConfigRepository,
	serverRepo *repository.NodeServerRepository,
	nodeMgr *agent.NodeServerManager,
	billingFilter *agent.BillingFilter,
) *UserSyncService {
	return &UserSyncService{
		clientRepo:    clientRepo,
		planSvc:       planSvc,
		inboundRepo:   inboundRepo,
		serverRepo:    serverRepo,
		nodeMgr:       nodeMgr,
		billingFilter: billingFilter,
	}
}

// SyncUserToServers syncs a user's credentials to all servers in their plan.
// This is the main entry point, triggered on user create/update/delete.
// action: "add" | "update" | "remove"
func (s *UserSyncService) SyncUserToServers(ctx context.Context, clientID int64, action string) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("get client %d: %w", clientID, err)
	}

	// Get accessible nodes (inbound configs) for this client's plan template
	accessResult, err := s.planSvc.GetAccessibleNodes(ctx, client.PlanID)
	if err != nil {
		return fmt.Errorf("get accessible nodes for plan %d: %w", client.PlanID, err)
	}

	// Group inbound configs by server_id
	serverInbounds := make(map[int64][]int64) // server_id → []inbound_id
	for _, inbound := range accessResult.InboundConfigs {
		// server_id = 0 means local (existing behavior)
		serverInbounds[inbound.ServerID] = append(serverInbounds[inbound.ServerID], inbound.ID)
	}

	// Always include local server (server_id = 0) if present
	var wg sync.WaitGroup

	for serverID, inboundIDs := range serverInbounds {
		// ★ Skip non-billable servers (chain_exit, backup nodes)
		if s.billingFilter != nil && !s.billingFilter.ShouldSyncUsers(ctx, serverID) {
			log.Printf("[UserSync] skipping server %d (billing filter: not billable)", serverID)
			continue
		}

		wg.Add(1)
		go func(sid int64, ibIDs []int64) {
			defer wg.Done()
			if err := s.syncUserToServer(ctx, sid, ibIDs, client, action); err != nil {
				log.Printf("[UserSync] server %d: %v", sid, err)
			}
		}(serverID, inboundIDs)
	}

	wg.Wait()
	return nil
}

// syncUserToServer syncs a single user to a single server.
func (s *UserSyncService) syncUserToServer(
	ctx context.Context,
	serverID int64,
	inboundIDs []int64,
	client *entity.Client,
	action string,
) error {
	// Check if server is online
	if !s.nodeMgr.IsOnline(serverID) {
		log.Printf("[UserSync] server %d offline, queuing for retry (client %d, action %s)",
			serverID, client.ID, action)
		s.enqueueRetry(client.ID, serverID, action)
		return nil
	}

	// For each inbound on this server, send AlterUsers command
	for _, inboundID := range inboundIDs {
		inbound, err := s.inboundRepo.GetByID(ctx, inboundID)
		if err != nil {
			continue
		}

		req := &agentpb.AlterUsersRequest{
			CoreType:   inbound.CoreType,
			InboundTag: inbound.Tag,
			Action:     action,
			Users: []*agentpb.UserOp{{
				Email: client.Email,
				Uuid:  fmt.Sprintf("%d", client.ID),
			}},
		}

		// Send via gRPC stream command
		panelCmd := &agentpb.PanelCommand{
			Command: &agentpb.PanelCommand_AlterUsers{
				AlterUsers: req,
			},
		}

		if err := s.nodeMgr.SendCommand(ctx, serverID, panelCmd); err != nil {
			log.Printf("[UserSync] server %d inbound %s: %v", serverID, inbound.Tag, err)
			// Queue for retry
			s.enqueueRetry(client.ID, serverID, action)
		}
	}

	return nil
}

// SyncUserRemoval removes a user from all servers.
func (s *UserSyncService) SyncUserRemoval(ctx context.Context, clientID int64) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return err
	}

	servers, err := s.serverRepo.ListAll(ctx)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, server := range servers {
		// Skip non-billable servers for removal too (they shouldn't have users)
		if s.billingFilter != nil && !s.billingFilter.ShouldSyncUsers(ctx, server.ID) {
			continue
		}

		if !s.nodeMgr.IsOnline(server.ID) {
			continue
		}

		wg.Add(1)
		go func(sid int64) {
			defer wg.Done()
			inbounds, _ := s.inboundRepo.ListByServerID(ctx, sid)
			for _, ib := range inbounds {
				req := &agentpb.AlterUsersRequest{
					CoreType:   ib.CoreType,
					InboundTag: ib.Tag,
					Action:     "remove",
					Users:      []*agentpb.UserOp{{Email: client.Email}},
				}
				panelCmd := &agentpb.PanelCommand{
					Command: &agentpb.PanelCommand_AlterUsers{
						AlterUsers: req,
					},
				}
				s.nodeMgr.SendCommand(ctx, sid, panelCmd)
			}
		}(server.ID)
	}
	wg.Wait()
	return nil
}

// ─── Offline Retry Queue ───

// enqueueRetry adds a failed sync operation to the retry queue.
func (s *UserSyncService) enqueueRetry(clientID, serverID int64, action string) {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()

	s.retryQueue = append(s.retryQueue, retryEntry{
		ClientID:   clientID,
		ServerID:   serverID,
		Action:     action,
		QueuedAt:   time.Now(),
		RetryCount: 0,
	})
}

// ProcessRetryQueue attempts to replay failed sync operations.
// Called periodically (e.g., every 60 seconds) or when a server reconnects.
func (s *UserSyncService) ProcessRetryQueue(ctx context.Context) {
	s.retryMu.Lock()
	if len(s.retryQueue) == 0 {
		s.retryMu.Unlock()
		return
	}
	entries := s.retryQueue
	s.retryQueue = nil
	s.retryMu.Unlock()

	var failed []retryEntry
	for _, entry := range entries {
		// Check if server is now online
		if !s.nodeMgr.IsOnline(entry.ServerID) {
			// Still offline — re-queue if not too old
			if time.Since(entry.QueuedAt) < 24*time.Hour {
				entry.RetryCount++
				failed = append(failed, entry)
			} else {
				log.Printf("[UserSync] dropping stale retry entry for client %d server %d",
					entry.ClientID, entry.ServerID)
			}
			continue
		}

		// Retry the sync
		if err := s.SyncUserToServers(ctx, entry.ClientID, entry.Action); err != nil {
			log.Printf("[UserSync] retry failed for client %d server %d: %v",
				entry.ClientID, entry.ServerID, err)
			entry.RetryCount++
			if entry.RetryCount < 10 {
				failed = append(failed, entry)
			}
		} else {
			log.Printf("[UserSync] retry succeeded for client %d server %d (attempt %d)",
				entry.ClientID, entry.ServerID, entry.RetryCount+1)
		}
	}

	// Re-queue failures
	if len(failed) > 0 {
		s.retryMu.Lock()
		s.retryQueue = append(s.retryQueue, failed...)
		s.retryMu.Unlock()
	}
}

// RunRetryLoop runs the retry queue processor periodically.
func (s *UserSyncService) RunRetryLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	log.Println("[UserSync] retry loop started (60s interval)")

	for {
		select {
		case <-ctx.Done():
			log.Println("[UserSync] retry loop stopped")
			return
		case <-ticker.C:
			s.ProcessRetryQueue(ctx)
		}
	}
}

// OnServerReconnect handles a server reconnect event by immediately
// processing retry queue entries for that server.
func (s *UserSyncService) OnServerReconnect(ctx context.Context, serverID int64) {
	s.retryMu.Lock()
	var matching []retryEntry
	var remaining []retryEntry
	for _, e := range s.retryQueue {
		if e.ServerID == serverID {
			matching = append(matching, e)
		} else {
			remaining = append(remaining, e)
		}
	}
	s.retryQueue = remaining
	s.retryMu.Unlock()

	if len(matching) == 0 {
		return
	}

	log.Printf("[UserSync] server %d reconnected, replaying %d pending sync(s)",
		serverID, len(matching))

	var wg sync.WaitGroup
	for _, entry := range matching {
		wg.Add(1)
		go func(e retryEntry) {
			defer wg.Done()
			if err := s.SyncUserToServers(ctx, e.ClientID, e.Action); err != nil {
				log.Printf("[UserSync] reconnect replay failed for client %d: %v",
					e.ClientID, err)
				// Re-enqueue
				s.enqueueRetry(e.ClientID, e.ServerID, e.Action)
			}
		}(entry)
	}
	wg.Wait()
}

// GetRetryQueueSize returns the current number of pending retry entries.
func (s *UserSyncService) GetRetryQueueSize() int {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	return len(s.retryQueue)
}

// GetRetryQueue returns a snapshot of the retry queue for inspection.
func (s *UserSyncService) GetRetryQueue() []retryEntry {
	s.retryMu.Lock()
	defer s.retryMu.Unlock()
	result := make([]retryEntry, len(s.retryQueue))
	copy(result, s.retryQueue)
	return result
}
