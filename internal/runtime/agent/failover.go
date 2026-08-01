package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// UserSyncer is the subset of UserSyncService used by the FailoverManager to
// replay credentials after a role switch. Defined locally to avoid an import
// cycle with the service layer.
type UserSyncer interface {
	// SyncServerFull re-sends all in-scope users to the given server.
	SyncServerFull(ctx context.Context, serverID int64) error
	// RemoveServerUsers removes all users from the given server.
	RemoveServerUsers(ctx context.Context, serverID int64) error
}

// FailoverManager monitors primary_backup node pairs and automatically
// promotes a backup to primary when the primary goes offline.
// When the primary recovers, it re-syncs and switches back.
type FailoverManager struct {
	serverRepo    *repository.NodeServerRepository
	nodeMgr       *NodeServerManager
	checkInterval time.Duration
	offlineThreshold time.Duration // how long before triggering failover
	recoverGrace     time.Duration // how long the original primary must be stable before switching back

	// Track which primaries have been failed over (to detect recovery)
	mu                  sync.RWMutex
	failedOverPrimaries map[int64]bool // original primary server_id → failed over?

	// userSyncer replays user credentials after a failover switches the
	// active primary. Guarded by userSyncMu.
	userSyncMu sync.Mutex
	userSyncer UserSyncer
}

// NewFailoverManager creates a new FailoverManager.
func NewFailoverManager(serverRepo *repository.NodeServerRepository, nodeMgr *NodeServerManager) *FailoverManager {
	return &FailoverManager{
		serverRepo:          serverRepo,
		nodeMgr:             nodeMgr,
		checkInterval:       30 * time.Second,
		offlineThreshold:    2 * time.Minute,
		recoverGrace:        1 * time.Minute,
		failedOverPrimaries: make(map[int64]bool),
	}
}

// WithCheckInterval sets the check interval.
func (m *FailoverManager) WithCheckInterval(d time.Duration) *FailoverManager {
	m.checkInterval = d
	return m
}

// WithOfflineThreshold sets how long a primary must be offline before failover.
func (m *FailoverManager) WithOfflineThreshold(d time.Duration) *FailoverManager {
	m.offlineThreshold = d
	return m
}

// WithRecoverGrace sets how long the original primary must be online and stable
// before the failover manager switches traffic back to it.
func (m *FailoverManager) WithRecoverGrace(d time.Duration) *FailoverManager {
	m.recoverGrace = d
	return m
}

// SetUserSync injects the UserSyncService so that credential sets are re-pushed
// to the newly active primary (and removed from the demoted node) after a
// failover/recovery. This is the Phase-4 wiring that makes promotion live.
func (m *FailoverManager) SetUserSync(s UserSyncer) {
	m.userSyncMu.Lock()
	m.userSyncer = s
	m.userSyncMu.Unlock()
}

// RunLoop starts the failover check loop. Blocks until ctx is cancelled.
func (m *FailoverManager) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	log.Println("[FailoverManager] started, checking every", m.checkInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[FailoverManager] stopped")
			return
		case <-ticker.C:
			if err := m.checkAllPairs(ctx); err != nil {
				log.Printf("[FailoverManager] check error: %v", err)
			}
		}
	}
}

// checkAllPairs checks all primary_backup pairs for failover conditions.
func (m *FailoverManager) checkAllPairs(ctx context.Context) error {
	// Get all primary_backup servers
	servers, err := m.serverRepo.ListByType(ctx, entity.NodeTypePrimaryBackup)
	if err != nil {
		return err
	}

	for i := range servers {
		server := servers[i]
		switch server.FailoverRole {
		case entity.FailoverRolePrimary:
			// Current active primary: fail over if it has been offline too long.
			m.checkActivePrimary(ctx, &server)
		case "backup_recovering":
			// Original primary that was demoted during failover; switch back
			// once it has been stable online for the grace period.
			m.checkRecovering(ctx, &server)
		}
	}
	return nil
}

// checkActivePrimary fails over a primary that has been offline past the
// threshold, then hands user replay to the UserSyncService.
func (m *FailoverManager) checkActivePrimary(ctx context.Context, primary *entity.NodeServer) {
	// Skip local servers
	if primary.IsLocal {
		return
	}

	// Primary is online — nothing to do.
	if m.nodeMgr.IsOnline(primary.ID) {
		return
	}

	// Primary is offline — check if long enough
	if primary.LastHeartbeatAt == nil {
		return
	}

	timeSinceHeartbeat := time.Since(*primary.LastHeartbeatAt)
	if timeSinceHeartbeat < m.offlineThreshold {
		return // Not offline long enough yet
	}

	// Check if already failed over
	m.mu.RLock()
	failedOver := m.failedOverPrimaries[primary.ID]
	m.mu.RUnlock()
	if failedOver {
		return // Already failed over, waiting for recovery
	}

	// Trigger failover
	m.promoteBackup(ctx, primary)
}

// checkRecovering switches traffic back to the original primary after it has
// been online and stable for the grace period.
func (m *FailoverManager) checkRecovering(ctx context.Context, originalPrimary *entity.NodeServer) {
	if originalPrimary.IsLocal {
		return
	}
	if !m.nodeMgr.IsOnline(originalPrimary.ID) {
		return
	}
	// Require the recovered node to be stable for recoverGrace to avoid
	// flapping right after a brief reconnect.
	if originalPrimary.LastHeartbeatAt != nil &&
		time.Since(*originalPrimary.LastHeartbeatAt) > m.recoverGrace {
		return
	}
	m.recoverPrimary(ctx, originalPrimary)
}

// promoteBackup promotes the backup node to primary.
func (m *FailoverManager) promoteBackup(ctx context.Context, primary *entity.NodeServer) {
	if primary.PartnerServerID == 0 {
		log.Printf("[FailoverManager] primary %d (%s) offline but no partner configured",
			primary.ID, primary.Name)
		return
	}

	backup, err := m.serverRepo.GetByID(ctx, primary.PartnerServerID)
	if err != nil {
		log.Printf("[FailoverManager] primary %d offline but backup %d not found: %v",
			primary.ID, primary.PartnerServerID, err)
		return
	}

	if backup.FailoverRole != entity.FailoverRoleBackup {
		log.Printf("[FailoverManager] primary %d offline but partner %d is not a backup (role=%s)",
			primary.ID, backup.ID, backup.FailoverRole)
		return
	}

	// Check if backup is online
	if !m.nodeMgr.IsOnline(backup.ID) {
		log.Printf("[FailoverManager] primary %d offline but backup %d is also offline — cannot failover",
			primary.ID, backup.ID)
		return
	}

	log.Printf("[FailoverManager] ★ FAILOVER: primary %s (id=%d) offline for %v, promoting backup %s (id=%d)",
		primary.Name, primary.ID, time.Since(*primary.LastHeartbeatAt), backup.Name, backup.ID)

	// Promote backup
	backup.FailoverRole = entity.FailoverRolePrimary
	backup.BillingEnabled = true
	backup.UpdatedAt = time.Now()
	if err := m.serverRepo.Update(ctx, backup); err != nil {
		log.Printf("[FailoverManager] failed to promote backup %d: %v", backup.ID, err)
		return
	}

	// Demote primary (mark for recovery)
	primary.FailoverRole = "backup_recovering"
	primary.BillingEnabled = false
	primary.UpdatedAt = time.Now()
	if err := m.serverRepo.Update(ctx, primary); err != nil {
		log.Printf("[FailoverManager] failed to demote primary %d: %v", primary.ID, err)
	}

	// Track failover
	m.mu.Lock()
	m.failedOverPrimaries[primary.ID] = true
	m.mu.Unlock()

	log.Printf("[FailoverManager] failover complete: %s is now primary, %s is recovering",
		backup.Name, primary.Name)

	// Phase-4: replay user credentials to the newly promoted primary and
	// strip them from the demoted (recovering) node so only the active
	// primary serves traffic.
	m.replayAfterSwitch(ctx, backup.ID, primary.ID)
}

// recoverPrimary switches back to the original primary after it recovers.
func (m *FailoverManager) recoverPrimary(ctx context.Context, originalPrimary *entity.NodeServer) {
	// Find the current acting primary (the backup that was promoted)
	if originalPrimary.PartnerServerID == 0 {
		return
	}

	actingPrimary, err := m.serverRepo.GetByID(ctx, originalPrimary.PartnerServerID)
	if err != nil {
		return
	}

	// Only recover if the acting primary was originally a backup
	if actingPrimary.FailoverRole != entity.FailoverRolePrimary {
		return
	}

	log.Printf("[FailoverManager] ★ RECOVERY: original primary %s (id=%d) back online, switching back from %s",
		originalPrimary.Name, originalPrimary.ID, actingPrimary.Name)

	// Restore original primary
	originalPrimary.FailoverRole = entity.FailoverRolePrimary
	originalPrimary.BillingEnabled = true
	originalPrimary.UpdatedAt = time.Now()
	if err := m.serverRepo.Update(ctx, originalPrimary); err != nil {
		log.Printf("[FailoverManager] failed to restore primary %d: %v", originalPrimary.ID, err)
		return
	}

	// Demote acting primary back to backup
	actingPrimary.FailoverRole = entity.FailoverRoleBackup
	actingPrimary.BillingEnabled = false
	actingPrimary.UpdatedAt = time.Now()
	if err := m.serverRepo.Update(ctx, actingPrimary); err != nil {
		log.Printf("[FailoverManager] failed to demote acting primary %d: %v", actingPrimary.ID, err)
	}

	// Clear failover tracking
	m.mu.Lock()
	delete(m.failedOverPrimaries, originalPrimary.ID)
	m.mu.Unlock()

	log.Printf("[FailoverManager] recovery complete: %s is primary again, %s is backup",
		originalPrimary.Name, actingPrimary.Name)

	// Phase-4: replay user credentials back to the restored primary and
	// strip them from the demoted backup.
	m.replayAfterSwitch(ctx, originalPrimary.ID, actingPrimary.ID)
}

// replayAfterSwitch pushes all in-scope users to the now-active primary and
// removes them from the node that just stepped down. It tolerates a nil
// UserSyncService (e.g. during early boot before wiring completes) by logging
// and continuing. Failures are logged but do not roll back the role change.
func (m *FailoverManager) replayAfterSwitch(ctx context.Context, newPrimaryID, demotedID int64) {
	m.userSyncMu.Lock()
	syncer := m.userSyncer
	m.userSyncMu.Unlock()
	if syncer == nil {
		log.Printf("[FailoverManager] user sync not wired; skipping credential replay for primary %d", newPrimaryID)
		return
	}

	if err := syncer.SyncServerFull(ctx, newPrimaryID); err != nil {
		log.Printf("[FailoverManager] failed to replay users to new primary %d: %v", newPrimaryID, err)
	}
	if err := syncer.RemoveServerUsers(ctx, demotedID); err != nil {
		log.Printf("[FailoverManager] failed to remove users from demoted node %d: %v", demotedID, err)
	}
}

// GetFailoverStatus returns the current failover status for a primary_backup pair.
func (m *FailoverManager) GetFailoverStatus(ctx context.Context, serverID int64) (*FailoverStatus, error) {
	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	status := &FailoverStatus{
		ServerID:      serverID,
		NodeType:      server.NodeType,
		CurrentRole:   server.FailoverRole,
		Online:        m.nodeMgr.IsOnline(serverID),
		PartnerID:     server.PartnerServerID,
	}

	if server.LastHeartbeatAt != nil {
		status.LastHeartbeatAge = time.Since(*server.LastHeartbeatAt)
	}

	m.mu.RLock()
	status.HasFailedOver = m.failedOverPrimaries[serverID]
	m.mu.RUnlock()

	return status, nil
}

// FailoverStatus represents the failover state of a server.
type FailoverStatus struct {
	ServerID       int64         `json:"server_id"`
	NodeType       string        `json:"node_type"`
	CurrentRole    string        `json:"current_role"`
	Online         bool          `json:"online"`
	PartnerID      int64         `json:"partner_id"`
	LastHeartbeatAge time.Duration `json:"last_heartbeat_age_seconds"`
	HasFailedOver  bool          `json:"has_failed_over"`
}
