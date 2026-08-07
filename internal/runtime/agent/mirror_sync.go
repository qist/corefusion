package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// MirrorSyncManager manages mirror node configuration replication.
// Mirror nodes copy the primary node's inbound configs and user lists,
// but do not independently bill — their traffic is merged into the primary.
type MirrorSyncManager struct {
	serverRepo      *repository.NodeServerRepository
	inboundRepo     *repository.InboundConfigRepository
	caddySiteRepo   *repository.CaddySiteRepository
	nginxSiteRepo   *repository.NginxSiteRepository
	settingRepo     *repository.SystemSettingRepository
	nodeMgr         *NodeServerManager
	syncInterval    time.Duration

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
		serverRepo:    serverRepo,
		inboundRepo:   inboundRepo,
		nodeMgr:       nodeMgr,
		syncInterval:  5 * time.Minute,
		lastSync:      make(map[int64]time.Time),
	}
}

// SetCaddySiteRepository sets the Caddy site repository for syncing Caddy configs.
func (m *MirrorSyncManager) SetCaddySiteRepository(repo *repository.CaddySiteRepository) *MirrorSyncManager {
	m.caddySiteRepo = repo
	return m
}

// SetNginxSiteRepository sets the Nginx site repository for syncing Nginx configs.
func (m *MirrorSyncManager) SetNginxSiteRepository(repo *repository.NginxSiteRepository) *MirrorSyncManager {
	m.nginxSiteRepo = repo
	return m
}

// SetSettingRepository sets the system setting repository for syncing SNI settings.
func (m *MirrorSyncManager) SetSettingRepository(repo *repository.SystemSettingRepository) *MirrorSyncManager {
	m.settingRepo = repo
	return m
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
// It performs a full bidirectional sync:
//   - New inbounds on primary → cloned to mirror (tag suffixed with -mirror)
//   - Existing inbounds → updated (enabled status, config, port, etc.)
//   - Inbounds deleted on primary → deleted from mirror
//   - Caddy/Nginx sites → same create/update/delete logic
// After DB sync, pushes config to mirror's Agent via SyncData and hot-reloads
// inbounds via xray gRPC API (addInbound for new, delInbound for removed).
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

	// Build a map of existing mirror inbounds by tag (for lookup)
	existingMap := make(map[string]*entity.InboundConfig)
	for i := range mirrorInbounds {
		existingMap[mirrorInbounds[i].Tag] = &mirrorInbounds[i]
	}

	// Track which mirror tags correspond to primary inbounds (for pruning)
	primaryMirrorTags := make(map[string]bool)

	// Track what changed for hot-reload
	var newInbounds []entity.InboundConfig  // inbounds newly created on mirror
	var removedTags []string                // inbounds deleted from mirror
	inboundChanged := false                  // any create/update/delete happened

	for _, pib := range primaryInbounds {
		mirrorTag := fmt.Sprintf("%s-mirror", pib.Tag)
		primaryMirrorTags[mirrorTag] = true

		if existing, ok := existingMap[mirrorTag]; ok {
			// Update existing mirror inbound to match primary
			updated := pib
			updated.ID = existing.ID
			updated.ServerID = mirror.ID
			updated.Tag = mirrorTag
			updated.Region = mirror.Region
			updated.CreatedAt = existing.CreatedAt
			updated.UpdatedAt = time.Now()

			if err := m.inboundRepo.Update(ctx, &updated); err != nil {
				log.Printf("[MirrorSync] failed to update inbound %s on mirror %d: %v",
					mirrorTag, mirror.ID, err)
				continue
			}
			inboundChanged = true
		} else {
			// Clone new inbound for mirror
			mib := pib
			mib.ID = 0
			mib.ServerID = mirror.ID
			mib.Tag = mirrorTag
			mib.Region = mirror.Region
			mib.CreatedAt = time.Now()
			mib.UpdatedAt = time.Now()

			if err := m.inboundRepo.Create(ctx, &mib); err != nil {
				log.Printf("[MirrorSync] failed to clone inbound %s to mirror %d: %v",
					pib.Tag, mirror.ID, err)
				continue
			}
			newInbounds = append(newInbounds, pib)
			inboundChanged = true
		}
	}

	// Delete mirror inbounds whose primary counterpart no longer exists
	for _, mib := range mirrorInbounds {
		if !primaryMirrorTags[mib.Tag] {
			if err := m.inboundRepo.Delete(ctx, mib.ID); err != nil {
				log.Printf("[MirrorSync] failed to delete stale inbound %s on mirror %d: %v",
					mib.Tag, mirror.ID, err)
				continue
			}
			removedTags = append(removedTags, mib.Tag)
			inboundChanged = true
		}
	}

	if inboundChanged {
		log.Printf("[MirrorSync] inbound sync: %d new, %d removed for mirror %s",
			len(newInbounds), len(removedTags), mirror.Name)
	}

	// Sync Caddy sites from primary to mirror
	if m.caddySiteRepo != nil {
		if err := m.syncCaddySites(ctx, primary.ID, mirror.ID); err != nil {
			log.Printf("[MirrorSync] failed to sync Caddy sites for mirror %d: %v", mirror.ID, err)
		}
	}

	// Sync Nginx sites from primary to mirror
	if m.nginxSiteRepo != nil {
		if err := m.syncNginxSites(ctx, primary.ID, mirror.ID); err != nil {
			log.Printf("[MirrorSync] failed to sync Nginx sites for mirror %d: %v", mirror.ID, err)
		}
	}

	// Sync SNI settings (proxy.sni_enabled, proxy.sni_listen_port, proxy.sni_local_port,
	// proxy.proxy_protocol_rules) from primary to mirror in the panel DB.
	// Without this, the frontend still shows the mirror's old SNI state.
	if m.settingRepo != nil {
		if err := m.syncSNISettings(ctx, primary.ID, mirror.ID); err != nil {
			log.Printf("[MirrorSync] failed to sync SNI settings for mirror %d: %v", mirror.ID, err)
		}
	}

	// After syncing DB data, push the updated config to the mirror's Agent
	// and apply it immediately. SyncData writes data to the Agent's local SQLite
	// DB, generates the runtime config file, and starts the core if not running.
	// However, if the core is already running, SyncData does NOT restart it —
	// so we also hot-reload via xray gRPC API (addInbound/delInbound).
	if m.nodeMgr != nil && !m.nodeMgr.isLocalServer(mirror.ID) && m.nodeMgr.dataSyncProvider != nil {
		syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		resp, err := m.nodeMgr.SyncDataToServer(syncCtx, mirror.ID, true)
		if err != nil {
			log.Printf("[MirrorSync] SyncData to mirror %d failed: %v", mirror.ID, err)
		} else {
			log.Printf("[MirrorSync] SyncData to mirror %d: success=%v core_started=%v",
				mirror.ID, resp.Success, resp.CoreStarted)
		}

		// Hot-reload: addInbound for new inbounds, delInbound for removed ones.
		// SyncData updates the config file but doesn't restart a running core.
		if len(newInbounds) > 0 {
			m.hotReloadAddInbounds(syncCtx, mirror.ID, newInbounds)
		}
		if len(removedTags) > 0 {
			m.hotReloadDelInbounds(syncCtx, mirror.ID, removedTags)
		}
	}

	// Record sync time
	m.mu.Lock()
	m.lastSync[mirror.ID] = time.Now()
	m.mu.Unlock()

	return nil
}

// syncCaddySites syncs Caddy site configs from primary to mirror.
// It creates new sites, updates existing ones (including enabled status,
// SNI target, proxy_protocol, etc.), and deletes sites that no longer
// exist on the primary.
func (m *MirrorSyncManager) syncCaddySites(ctx context.Context, primaryID, mirrorID int64) error {
	// Get primary's Caddy sites
	primarySites, err := m.caddySiteRepo.List(ctx, primaryID)
	if err != nil {
		return fmt.Errorf("list primary Caddy sites: %w", err)
	}

	// Get mirror's existing Caddy sites
	mirrorSites, err := m.caddySiteRepo.List(ctx, mirrorID)
	if err != nil {
		return fmt.Errorf("list mirror Caddy sites: %w", err)
	}

	// Build a map of existing mirror sites by domain for lookup
	existingMap := make(map[string]*entity.CaddySite)
	for i := range mirrorSites {
		existingMap[mirrorSites[i].Domain] = &mirrorSites[i]
	}

	// Track which primary domains exist (for pruning stale mirror sites)
	primaryDomains := make(map[string]bool)

	synced := 0
	for _, primarySite := range primarySites {
		primaryDomains[primarySite.Domain] = true

		if existing, ok := existingMap[primarySite.Domain]; ok {
			// Update existing site to match primary (enabled, SNI target, etc.)
			updated := primarySite
			updated.ID = existing.ID
			updated.ServerID = mirrorID
			updated.CreatedAt = existing.CreatedAt
			updated.UpdatedAt = time.Now()
			if err := m.caddySiteRepo.Update(ctx, &updated); err != nil {
				log.Printf("[MirrorSync] failed to update Caddy site %s on mirror %d: %v",
					primarySite.Domain, mirrorID, err)
				continue
			}
			synced++
		} else {
			// Clone new site for mirror
			mirrorSite := primarySite
			mirrorSite.ID = 0
			mirrorSite.ServerID = mirrorID
			mirrorSite.CreatedAt = time.Now()
			mirrorSite.UpdatedAt = time.Now()

			if err := m.caddySiteRepo.Create(ctx, &mirrorSite); err != nil {
				log.Printf("[MirrorSync] failed to clone Caddy site %s to mirror %d: %v",
					primarySite.Domain, mirrorID, err)
				continue
			}
			synced++
		}
	}

	// Delete sites on mirror that no longer exist on primary
	for _, mirrorSite := range mirrorSites {
		if !primaryDomains[mirrorSite.Domain] {
			if err := m.caddySiteRepo.Delete(ctx, mirrorSite.ID); err != nil {
				log.Printf("[MirrorSync] failed to delete stale Caddy site %s on mirror %d: %v",
					mirrorSite.Domain, mirrorID, err)
			} else {
				synced++
			}
		}
	}

	if synced > 0 {
		log.Printf("[MirrorSync] synced %d Caddy site(s) from primary %d to mirror %d",
			synced, primaryID, mirrorID)
	}

	return nil
}

// syncNginxSites syncs Nginx site configs from primary to mirror.
// It creates new sites, updates existing ones (including enabled status,
// SNI target, proxy_protocol, etc.), and deletes sites that no longer
// exist on the primary.
func (m *MirrorSyncManager) syncNginxSites(ctx context.Context, primaryID, mirrorID int64) error {
	// Get primary's Nginx sites
	primarySites, err := m.nginxSiteRepo.List(ctx, primaryID)
	if err != nil {
		return fmt.Errorf("list primary Nginx sites: %w", err)
	}

	// Get mirror's existing Nginx sites
	mirrorSites, err := m.nginxSiteRepo.List(ctx, mirrorID)
	if err != nil {
		return fmt.Errorf("list mirror Nginx sites: %w", err)
	}

	// Build a map of existing mirror sites by domain for lookup
	existingMap := make(map[string]*entity.NginxSite)
	for i := range mirrorSites {
		existingMap[mirrorSites[i].Domain] = &mirrorSites[i]
	}

	// Track which primary domains exist (for pruning stale mirror sites)
	primaryDomains := make(map[string]bool)

	synced := 0
	for _, primarySite := range primarySites {
		primaryDomains[primarySite.Domain] = true

		if existing, ok := existingMap[primarySite.Domain]; ok {
			// Update existing site to match primary
			updated := primarySite
			updated.ID = existing.ID
			updated.ServerID = mirrorID
			updated.CreatedAt = existing.CreatedAt
			updated.UpdatedAt = time.Now()
			if err := m.nginxSiteRepo.Update(ctx, &updated); err != nil {
				log.Printf("[MirrorSync] failed to update Nginx site %s on mirror %d: %v",
					primarySite.Domain, mirrorID, err)
				continue
			}
			synced++
		} else {
			// Clone new site for mirror
			mirrorSite := primarySite
			mirrorSite.ID = 0
			mirrorSite.ServerID = mirrorID
			mirrorSite.CreatedAt = time.Now()
			mirrorSite.UpdatedAt = time.Now()

			if err := m.nginxSiteRepo.Create(ctx, &mirrorSite); err != nil {
				log.Printf("[MirrorSync] failed to clone Nginx site %s to mirror %d: %v",
					primarySite.Domain, mirrorID, err)
				continue
			}
			synced++
		}
	}

	// Delete sites on mirror that no longer exist on primary
	for _, mirrorSite := range mirrorSites {
		if !primaryDomains[mirrorSite.Domain] {
			if err := m.nginxSiteRepo.Delete(ctx, mirrorSite.ID); err != nil {
				log.Printf("[MirrorSync] failed to delete stale Nginx site %s on mirror %d: %v",
					mirrorSite.Domain, mirrorID, err)
			} else {
				synced++
			}
		}
	}

	if synced > 0 {
		log.Printf("[MirrorSync] synced %d Nginx site(s) from primary %d to mirror %d",
			synced, primaryID, mirrorID)
	}

	return nil
}

// syncSNISettings syncs SNI-related system_settings from primary to mirror.
// SNI settings are stored with per-node keys: proxy.sni_enabled.{serverID},
// proxy.sni_listen_port.{serverID}, proxy.sni_local_port.{serverID},
// proxy.proxy_protocol_rules.{serverID}.
// Without this sync, the frontend shows stale SNI state for mirror nodes.
func (m *MirrorSyncManager) syncSNISettings(ctx context.Context, primaryID, mirrorID int64) error {
	if m.settingRepo == nil {
		return nil
	}

	// SNI setting keys that need to be synced (base key → per-node key pattern)
	sniKeys := []string{
		"proxy.sni_enabled",
		"proxy.sni_listen_port",
		"proxy.sni_local_port",
		"proxy.proxy_protocol_rules",
	}

	synced := 0
	for _, baseKey := range sniKeys {
		primaryKey := fmt.Sprintf("%s.%d", baseKey, primaryID)
		mirrorKey := fmt.Sprintf("%s.%d", baseKey, mirrorID)

		// Read primary's value (fall back to global key if per-node doesn't exist)
		val, err := m.settingRepo.GetByKey(ctx, primaryKey)
		if err != nil {
			// Fall back to global key (for primaryID=1 or unconfigured nodes)
			val, err = m.settingRepo.GetByKey(ctx, baseKey)
			if err != nil {
				continue // key doesn't exist, skip
			}
		}

		// Write to mirror's per-node key
		if err := m.settingRepo.Set(ctx, mirrorKey, val); err != nil {
			log.Printf("[MirrorSync] failed to sync SNI setting %s → %s: %v", primaryKey, mirrorKey, err)
			continue
		}
		synced++
	}

	if synced > 0 {
		log.Printf("[MirrorSync] synced %d SNI setting(s) from primary %d to mirror %d",
			synced, primaryID, mirrorID)
	}

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

// hotReloadAddInbounds hot-reloads newly created inbounds on a mirror node via
// xray gRPC API (addInbound). This is needed because SyncData updates the config
// file but does NOT restart a running xray process. By calling addInbound for
// each newly created mirror inbound, the new ports are immediately listening
// without restarting xray.
func (m *MirrorSyncManager) hotReloadAddInbounds(ctx context.Context, mirrorID int64, newInbounds []entity.InboundConfig) {
	for _, pib := range newInbounds {
		mirrorTag := fmt.Sprintf("%s-mirror", pib.Tag)

		// Parse the primary's config_json to get the inbound definition,
		// then adjust the tag to the mirror tag and ensure listen/port are set.
		var inboundCfg map[string]interface{}
		if err := json.Unmarshal([]byte(pib.ConfigJSON), &inboundCfg); err != nil {
			log.Printf("[MirrorSync] hotReload: parse config for inbound %s failed: %v", pib.Tag, err)
			continue
		}
		inboundCfg["tag"] = mirrorTag
		inboundCfg["listen"] = pib.Listen
		inboundCfg["port"] = pib.Port

		payload := map[string]interface{}{
			"config": inboundCfg,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[MirrorSync] hotReload: marshal payload for inbound %s failed: %v", mirrorTag, err)
			continue
		}

		req := &agentpb.XrayAPIRequest{
			Service: "proxyman",
			Method:  "addInbound",
			Payload: payloadBytes,
		}
		resp, err := m.nodeMgr.XrayAPIOnServer(ctx, mirrorID, req)
		if err != nil {
			log.Printf("[MirrorSync] hotReload: addInbound %s to mirror %d failed: %v", mirrorTag, mirrorID, err)
		} else if !resp.Success {
			log.Printf("[MirrorSync] hotReload: addInbound %s to mirror %d returned failure: %s", mirrorTag, mirrorID, resp.Error)
		} else {
			log.Printf("[MirrorSync] hotReload: addInbound %s to mirror %d: success", mirrorTag, mirrorID)
		}
	}
}

// hotReloadDelInbounds removes inbounds from a mirror node's running xray via
// xray gRPC API (delInbound). This is called when an inbound was deleted on
// the primary and the mirror copy was pruned — the port needs to be released
// on the running xray without a full restart.
func (m *MirrorSyncManager) hotReloadDelInbounds(ctx context.Context, mirrorID int64, removedTags []string) {
	for _, tag := range removedTags {
		payload := map[string]interface{}{
			"tag": tag,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("[MirrorSync] hotReload: marshal delInbound payload for %s failed: %v", tag, err)
			continue
		}

		req := &agentpb.XrayAPIRequest{
			Service: "proxyman",
			Method:  "delInbound",
			Payload: payloadBytes,
		}
		resp, err := m.nodeMgr.XrayAPIOnServer(ctx, mirrorID, req)
		if err != nil {
			log.Printf("[MirrorSync] hotReload: delInbound %s on mirror %d failed: %v", tag, mirrorID, err)
		} else if !resp.Success {
			log.Printf("[MirrorSync] hotReload: delInbound %s on mirror %d returned failure: %s", tag, mirrorID, resp.Error)
		} else {
			log.Printf("[MirrorSync] hotReload: delInbound %s on mirror %d: success", tag, mirrorID)
		}
	}
}
