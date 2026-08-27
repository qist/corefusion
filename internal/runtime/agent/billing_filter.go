package agent

import (
	"context"
	"log"
	"sync"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// BillingFilter determines which servers participate in user creation and
// traffic billing. Servers with billing_enabled=false (chain_exit, mirror
// non-primary, backup nodes) are skipped during user sync and traffic collection.
type BillingFilter struct {
	serverRepo *repository.NodeServerRepository
	// Cache for performance
	mu      sync.RWMutex
	cache   map[int64]*entity.NodeServer
}

// NewBillingFilter creates a new BillingFilter.
func NewBillingFilter(serverRepo *repository.NodeServerRepository) *BillingFilter {
	return &BillingFilter{
		serverRepo: serverRepo,
		cache:     make(map[int64]*entity.NodeServer),
	}
}

// ShouldSyncUsers returns true if users should be synced to this server.
// chain_exit: never sync users (no user inbounds)
// mirror: do NOT sync via AlterUsers — user credentials are included in the
//   primary's config_json which is synced via SyncData. User add/remove is
//   handled by InboundManager.fanOutAddUserToMirrors/fanOutRemoveUserToMirrors
//   which directly calls xray API on the mirror node (using the primary's tag).
// primary_backup backup: sync users only when it's the active primary (after failover)
// standalone, chain_entry: always sync users
func (f *BillingFilter) ShouldSyncUsers(ctx context.Context, serverID int64) bool {
	server, err := f.getServer(ctx, serverID)
	if err != nil || server == nil {
		return false
	}

	// Disabled servers don't get users
	if !server.Enabled {
		return false
	}

	switch server.NodeType {
	case entity.NodeTypeChainExit:
		// chain_exit never gets user sync
		return false

	case entity.NodeTypeMirror:
		// Mirror nodes do NOT receive AlterUsers — their config comes from
		// the primary via SyncData. User add/remove hot operations go through
		// InboundManager.fanOutAddUserToMirrors / fanOutRemoveUserToMirrors.
		return false

	case entity.NodeTypePrimaryBackup:
		// Only the active primary gets users (not the backup)
		return server.FailoverRole == entity.FailoverRolePrimary ||
			server.FailoverRole == "backup_recovering" // recovering gets users to be ready

	case entity.NodeTypeStandalone, entity.NodeTypeChainEntry, entity.NodeTypeSync:
		// Sync nodes have independent billing and user records — they need AlterUsers.
		return server.BillingEnabled

	default:
		return server.BillingEnabled
	}
}

// ShouldCollectTraffic returns true if traffic should be collected from this server.
// chain_exit: no traffic collection
// mirror: collect traffic but merge to primary
// primary_backup backup (not active): no traffic collection
func (f *BillingFilter) ShouldCollectTraffic(ctx context.Context, serverID int64) bool {
	server, err := f.getServer(ctx, serverID)
	if err != nil || server == nil {
		return false
	}

	if !server.Enabled {
		return false
	}

	switch server.NodeType {
	case entity.NodeTypeChainExit:
		return false

	case entity.NodeTypePrimaryBackup:
		// Collect from active primary and recovering backup (to capture traffic
		// from existing connections that haven't switched to the new primary yet).
		// No duplicate counting since connections don't overlap:
		// - primary: new connections after failover
		// - backup_recovering: existing connections before failover
		return server.FailoverRole == entity.FailoverRolePrimary ||
			server.FailoverRole == entity.FailoverRoleBackupRecovering

	case entity.NodeTypeMirror:
		// Collect from mirror, but attribute to primary
		return true

	default:
		return server.BillingEnabled
	}
}

// ShouldAppearInSubscription returns true if the server's address should
// appear in user subscription output.
// chain_exit: never appears in subscription
// mirror: appears (provides additional entry points)
// primary_backup backup: appears as fallback (lower priority)
func (f *BillingFilter) ShouldAppearInSubscription(ctx context.Context, serverID int64) bool {
	server, err := f.getServer(ctx, serverID)
	if err != nil || server == nil {
		return false
	}

	if !server.Enabled {
		return false
	}

	switch server.NodeType {
	case entity.NodeTypeChainExit:
		return false

	case entity.NodeTypePrimaryBackup:
		// Both primary and backup can appear (backup as fallback)
		return true

	default:
		return true
	}
}

// GetTrafficAttributionServerID returns the server ID that traffic from this
// server should be attributed to. For mirrors, returns the primary's ID.
// For all others, returns the same server ID.
func (f *BillingFilter) GetTrafficAttributionServerID(ctx context.Context, serverID int64) int64 {
	server, err := f.getServer(ctx, serverID)
	if err != nil || server == nil {
		return serverID
	}

	if server.NodeType == entity.NodeTypeMirror && server.PartnerServerID > 0 {
		return server.PartnerServerID
	}

	return serverID
}

// ListBillableServerIDs returns server IDs that should receive user sync.
func (f *BillingFilter) ListBillableServerIDs(ctx context.Context) ([]int64, error) {
	servers, err := f.serverRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, s := range servers {
		if f.ShouldSyncUsers(ctx, s.ID) {
			ids = append(ids, s.ID)
		}
	}

	// Invalidate cache since we just loaded fresh data
	f.mu.Lock()
	for _, s := range servers {
		sc := s
		f.cache[s.ID] = &sc
	}
	f.mu.Unlock()

	log.Printf("[BillingFilter] %d/%d servers are billable", len(ids), len(servers))
	return ids, nil
}

// InvalidateCache clears the cached server data.
func (f *BillingFilter) InvalidateCache() {
	f.mu.Lock()
	f.cache = make(map[int64]*entity.NodeServer)
	f.mu.Unlock()
}

// InvalidateOne removes a single server from cache.
func (f *BillingFilter) InvalidateOne(serverID int64) {
	f.mu.Lock()
	delete(f.cache, serverID)
	f.mu.Unlock()
}

// getServer returns a server, using cache when possible.
func (f *BillingFilter) getServer(ctx context.Context, serverID int64) (*entity.NodeServer, error) {
	f.mu.RLock()
	if s, ok := f.cache[serverID]; ok {
		f.mu.RUnlock()
		return s, nil
	}
	f.mu.RUnlock()

	server, err := f.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.cache[serverID] = server
	f.mu.Unlock()

	return server, nil
}

// Note: TrafficEvent is defined in manager.go and reused here for mirror traffic attribution.
