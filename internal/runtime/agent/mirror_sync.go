package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// MirrorSyncManager manages mirror node configuration replication.
//
// In the new design (mirror-sync-redesign), mirror nodes do NOT have their own
// DB records in the panel DB. Instead, BuildSyncDataRequest detects mirror nodes
// and generates their config from the primary's DB data at runtime.
//
// The MirrorSyncManager now only serves as:
//  1. A periodic fallback SyncData trigger (RunLoop) — ensures mirror nodes
//     receive SyncData even if event-driven sync misses a trigger.
//  2. A status query interface (GetMirrorStatus) for the UI.
//  3. A manual sync trigger (SyncMirrorNow) for the UI "sync mirror" button.
//
// All DB-level cloning logic (syncMirrorWithPrimary, syncCaddySites,
// syncNginxSites, hotReload*, etc.) has been removed. The Agent-side
// handleSyncData now handles config reload via reload_strategy=restart.
type MirrorSyncManager struct {
	serverRepo   *repository.NodeServerRepository
	nodeMgr      *NodeServerManager
	syncInterval time.Duration

	// Track last sync time per mirror
	mu       sync.RWMutex
	lastSync map[int64]time.Time
}

// NewMirrorSyncManager creates a new MirrorSyncManager.
func NewMirrorSyncManager(
	serverRepo *repository.NodeServerRepository,
	_ *repository.InboundConfigRepository, // kept for API compat; no longer used
	nodeMgr *NodeServerManager,
) *MirrorSyncManager {
	return &MirrorSyncManager{
		serverRepo:   serverRepo,
		nodeMgr:      nodeMgr,
		syncInterval: 10 * time.Minute, // increased from 5min — event-driven sync is primary now
		lastSync:     make(map[int64]time.Time),
	}
}

// SetCaddySiteRepository is kept for API compatibility but is a no-op.
// Caddy site generation now happens at runtime via GenerateConfigForServer.
func (m *MirrorSyncManager) SetCaddySiteRepository(_ *repository.CaddySiteRepository) *MirrorSyncManager {
	return m
}

// SetNginxSiteRepository is kept for API compatibility but is a no-op.
func (m *MirrorSyncManager) SetNginxSiteRepository(_ *repository.NginxSiteRepository) *MirrorSyncManager {
	return m
}

// SetSettingRepository is kept for API compatibility but is a no-op.
// SNI settings are now read from the primary at runtime.
func (m *MirrorSyncManager) SetSettingRepository(_ *repository.SystemSettingRepository) *MirrorSyncManager {
	return m
}

// WithSyncInterval sets the sync check interval.
func (m *MirrorSyncManager) WithSyncInterval(d time.Duration) *MirrorSyncManager {
	m.syncInterval = d
	return m
}

// RunLoop starts the mirror sync loop. Blocks until ctx is cancelled.
// This is a fallback/compensating mechanism — the primary sync path is
// event-driven (fanOutMirrorSync in inboundconfig handler).
func (m *MirrorSyncManager) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	log.Println("[MirrorSync] started, checking every", m.syncInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[MirrorSync] stopped")
			return
		case <-ticker.C:
			if err := m.syncAllMirrors(ctx); err != nil {
				log.Printf("[MirrorSync] sync error: %v", err)
			}
		}
	}
}

// syncAllMirrors triggers SyncData to all online mirror nodes.
// SyncData will detect the mirror node and generate config from the primary's data.
func (m *MirrorSyncManager) syncAllMirrors(ctx context.Context) error {
	mirrors, err := m.serverRepo.ListByType(ctx, entity.NodeTypeMirror)
	if err != nil {
		return err
	}

	for _, mirror := range mirrors {
		if mirror.PartnerServerID == 0 {
			continue
		}

		// Skip if recently synced
		m.mu.RLock()
		last := m.lastSync[mirror.ID]
		m.mu.RUnlock()
		if time.Since(last) < m.syncInterval {
			continue
		}

		// Skip if mirror's Agent is offline
		if !m.nodeMgr.IsOnline(mirror.ID) {
			continue
		}

		// Trigger SyncData — BuildSyncDataRequest will detect the mirror node
		// and generate config from the primary's DB data.
		// The Agent will restart xray (reload_strategy=restart) to apply changes.
		if m.nodeMgr != nil && !m.nodeMgr.isLocalServer(mirror.ID) && m.nodeMgr.dataSyncProvider != nil {
			syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			resp, err := m.nodeMgr.SyncDataToServer(syncCtx, mirror.ID, true)
			cancel()
			if err != nil {
				log.Printf("[MirrorSync] SyncData to mirror %d failed: %v", mirror.ID, err)
			} else {
				log.Printf("[MirrorSync] SyncData to mirror %d: success=%v core_started=%v core_restarted=%v",
					mirror.ID, resp.Success, resp.CoreStarted, resp.CoreRestarted)
			}
		}

		// Record sync time
		m.mu.Lock()
		m.lastSync[mirror.ID] = time.Now()
		m.mu.Unlock()
	}

	return nil
}

// SyncMirrorNow forces an immediate SyncData for a specific mirror.
// This is called by the UI "sync mirror" button.
func (m *MirrorSyncManager) SyncMirrorNow(ctx context.Context, mirrorID int64) error {
	mirror, err := m.serverRepo.GetByID(ctx, mirrorID)
	if err != nil {
		return err
	}
	if mirror.NodeType != entity.NodeTypeMirror {
		return fmt.Errorf("server %d is not a mirror node", mirrorID)
	}
	if mirror.PartnerServerID == 0 {
		return fmt.Errorf("mirror %d has no primary server", mirrorID)
	}
	if !m.nodeMgr.IsOnline(mirror.ID) {
		return fmt.Errorf("mirror %d is offline", mirrorID)
	}
	if m.nodeMgr.isLocalServer(mirror.ID) || m.nodeMgr.dataSyncProvider == nil {
		return fmt.Errorf("cannot sync local mirror or dataSyncProvider not set")
	}

	syncCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	resp, err := m.nodeMgr.SyncDataToServer(syncCtx, mirror.ID, true)
	if err != nil {
		return fmt.Errorf("SyncData to mirror %d: %w", mirrorID, err)
	}
	log.Printf("[MirrorSync] manual SyncData to mirror %d: success=%v core_started=%v core_restarted=%v",
		mirrorID, resp.Success, resp.CoreStarted, resp.CoreRestarted)

	// Record sync time
	m.mu.Lock()
	m.lastSync[mirror.ID] = time.Now()
	m.mu.Unlock()

	return nil
}

// GetMirrorStatus returns sync status for a mirror node.
func (m *MirrorSyncManager) GetMirrorStatus(ctx context.Context, mirrorID int64) (*MirrorStatus, error) {
	mirror, err := m.serverRepo.GetByID(ctx, mirrorID)
	if err != nil {
		return nil, err
	}

	status := &MirrorStatus{
		MirrorID:  mirrorID,
		MirrorName: mirror.Name,
		PrimaryID: mirror.PartnerServerID,
		Online:    m.nodeMgr.IsOnline(mirrorID),
	}

	if mirror.PartnerServerID > 0 {
		primary, err := m.serverRepo.GetByID(ctx, mirror.PartnerServerID)
		if err == nil {
			status.PrimaryName = primary.Name
			status.PrimaryOnline = m.nodeMgr.IsOnline(primary.ID)
		}
	}

	m.mu.RLock()
	if last, ok := m.lastSync[mirrorID]; ok {
		status.LastSyncAt = &last
		status.LastSyncAge = time.Since(last)
	}
	m.mu.RUnlock()

	return status, nil
}

// MirrorStatus represents the sync status of a mirror node.
type MirrorStatus struct {
	MirrorID      int64         `json:"mirror_id"`
	MirrorName    string        `json:"mirror_name"`
	PrimaryID     int64         `json:"primary_id"`
	PrimaryName   string        `json:"primary_name"`
	Online        bool          `json:"online"`
	PrimaryOnline bool          `json:"primary_online"`
	LastSyncAt    *time.Time    `json:"last_sync_at,omitempty"`
	LastSyncAge   time.Duration `json:"last_sync_age_seconds"`
}

// MergeTrafficToPrimary marks traffic from a mirror to be attributed to its primary.
// Called when processing traffic reports from mirror nodes.
func MergeTrafficToPrimary(servers []entity.NodeServer, event *TrafficEvent) *TrafficEvent {
	for _, s := range servers {
		if s.ID == event.ServerID && s.NodeType == entity.NodeTypeMirror && s.PartnerServerID > 0 {
			// Attribute traffic to the primary
			return &TrafficEvent{
				ServerID:  s.PartnerServerID,
				Email:     event.Email,
				Uplink:    event.Uplink,
				Downlink:  event.Downlink,
				CoreType:  event.CoreType,
				Timestamp: event.Timestamp,
			}
		}
	}
	return event // Not a mirror, return as-is
}
