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
// Mirror nodes copy the primary node's inbound configs and user lists,
// but do not independently bill — their traffic is merged into the primary.
type MirrorSyncManager struct {
	serverRepo   *repository.NodeServerRepository
	inboundRepo  *repository.InboundConfigRepository
	nodeMgr      *NodeServerManager
	syncInterval time.Duration

	// Track last sync time per mirror
	mu        sync.RWMutex
	lastSync  map[int64]time.Time
}

// NewMirrorSyncManager creates a new MirrorSyncManager.
func NewMirrorSyncManager(
	serverRepo *repository.NodeServerRepository,
	inboundRepo *repository.InboundConfigRepository,
	nodeMgr *NodeServerManager,
) *MirrorSyncManager {
	return &MirrorSyncManager{
		serverRepo:   serverRepo,
		inboundRepo:  inboundRepo,
		nodeMgr:      nodeMgr,
		syncInterval: 5 * time.Minute,
		lastSync:     make(map[int64]time.Time),
	}
}

// WithSyncInterval sets the sync check interval.
func (m *MirrorSyncManager) WithSyncInterval(d time.Duration) *MirrorSyncManager {
	m.syncInterval = d
	return m
}

// RunLoop starts the mirror sync loop. Blocks until ctx is cancelled.
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

// syncAllMirrors syncs all mirror nodes with their primaries.
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

		// Sync this mirror with its primary
		if err := m.syncMirrorWithPrimary(ctx, &mirror); err != nil {
			log.Printf("[MirrorSync] failed to sync mirror %d: %v", mirror.ID, err)
		}
	}

	return nil
}

// syncMirrorWithPrimary replicates the primary's inbound configs to the mirror.
func (m *MirrorSyncManager) syncMirrorWithPrimary(ctx context.Context, mirror *entity.NodeServer) error {
	// Get the primary (partner) server
	primary, err := m.serverRepo.GetByID(ctx, mirror.PartnerServerID)
	if err != nil {
		return fmt.Errorf("primary server %d not found: %w", mirror.PartnerServerID, err)
	}

	// Check if mirror's Agent is online
	if !m.nodeMgr.IsOnline(mirror.ID) {
		log.Printf("[MirrorSync] mirror %d offline, skipping sync", mirror.ID)
		return nil
	}

	// Get primary's inbound configs
	primaryInbounds, err := m.inboundRepo.ListByServerID(ctx, primary.ID)
	if err != nil {
		return fmt.Errorf("list primary inbounds: %w", err)
	}

	// Get mirror's existing inbound configs
	mirrorInbounds, err := m.inboundRepo.ListByServerID(ctx, mirror.ID)
	if err != nil {
		return fmt.Errorf("list mirror inbounds: %w", err)
	}

	// Build a map of existing mirror inbound tags
	existingTags := make(map[string]bool)
	for _, ib := range mirrorInbounds {
		existingTags[ib.Tag] = true
	}

	// For each primary inbound, ensure a mirror copy exists
	synced := 0
	for _, pib := range primaryInbounds {
		mirrorTag := fmt.Sprintf("%s-mirror", pib.Tag)
		if existingTags[mirrorTag] {
			continue // Already exists
		}

		// Clone the inbound config for the mirror
		mib := pib // copy
		mib.ID = 0 // new record
		mib.ServerID = mirror.ID
		mib.Tag = mirrorTag
		mib.Region = mirror.Region
		mib.CreatedAt = time.Now()
		mib.UpdatedAt = time.Now()

		if err := m.inboundRepo.Create(ctx, &mib); err != nil {
			log.Printf("[MirrorSync] failed to clone inbound %s to mirror %d: %v", pib.Tag, mirror.ID, err)
			continue
		}
		synced++
	}

	if synced > 0 {
		log.Printf("[MirrorSync] mirrored %d inbound(s) from primary %s to mirror %s",
			synced, primary.Name, mirror.Name)
	}

	// Record sync time
	m.mu.Lock()
	m.lastSync[mirror.ID] = time.Now()
	m.mu.Unlock()

	return nil
}

// SyncMirrorNow forces an immediate sync for a specific mirror.
func (m *MirrorSyncManager) SyncMirrorNow(ctx context.Context, mirrorID int64) error {
	mirror, err := m.serverRepo.GetByID(ctx, mirrorID)
	if err != nil {
		return err
	}
	if mirror.NodeType != entity.NodeTypeMirror {
		return fmt.Errorf("server %d is not a mirror node", mirrorID)
	}
	return m.syncMirrorWithPrimary(ctx, mirror)
}

// GetMirrorStatus returns sync status for a mirror node.
func (m *MirrorSyncManager) GetMirrorStatus(ctx context.Context, mirrorID int64) (*MirrorStatus, error) {
	mirror, err := m.serverRepo.GetByID(ctx, mirrorID)
	if err != nil {
		return nil, err
	}

	status := &MirrorStatus{
		MirrorID:      mirrorID,
		MirrorName:    mirror.Name,
		PrimaryID:     mirror.PartnerServerID,
		Online:        m.nodeMgr.IsOnline(mirrorID),
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
	MirrorID      int64          `json:"mirror_id"`
	MirrorName    string         `json:"mirror_name"`
	PrimaryID     int64          `json:"primary_id"`
	PrimaryName   string         `json:"primary_name"`
	Online        bool           `json:"online"`
	PrimaryOnline bool           `json:"primary_online"`
	LastSyncAt    *time.Time     `json:"last_sync_at,omitempty"`
	LastSyncAge   time.Duration  `json:"last_sync_age_seconds"`
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
