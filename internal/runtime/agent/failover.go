package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// FailoverManager monitors primary_backup node pairs and automatically
// promotes a backup to primary when the primary goes offline.
// When the primary recovers, it re-syncs and switches back.
type FailoverManager struct {
	serverRepo    *repository.NodeServerRepository
	nodeMgr       *NodeServerManager
	checkInterval time.Duration
	offlineThreshold time.Duration // how long before triggering failover

	// Track which primaries have been failed over (to detect recovery)
	mu              sync.RWMutex
	failedOverPrimaries map[int64]bool // original primary server_id → failed over?
}

// NewFailoverManager creates a new FailoverManager.
func NewFailoverManager(serverRepo *repository.NodeServerRepository, nodeMgr *NodeServerManager) *FailoverManager {
	return &FailoverManager{
		serverRepo:          serverRepo,
		nodeMgr:             nodeMgr,
		checkInterval:       30 * time.Second,
		offlineThreshold:    2 * time.Minute,
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

	for _, server := range servers {
		if server.FailoverRole == entity.FailoverRolePrimary {
			m.checkPrimary(ctx, &server)
		}
	}
	return nil
}

// checkPrimary checks if a primary node needs failover or recovery.
func (m *FailoverManager) checkPrimary(ctx context.Context, primary *entity.NodeServer) {
	// Skip local servers
	if primary.IsLocal {
		return
	}

	// Check if primary is online
	if m.nodeMgr.IsOnline(primary.ID) {
		// Primary is online — check if we need to recover
		m.mu.RLock()
		failedOver := m.failedOverPrimaries[primary.ID]
		m.mu.RUnlock()

		if failedOver {
			// Primary recovered — attempt switch back
			m.recoverPrimary(ctx, primary)
		}
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

	// Note: User sync and subscription address switching will be handled
	// by the UserSyncService (Phase 4) which reads failover_role to determine
	// which server to sync users to.
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
