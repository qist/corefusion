package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// FailoverNodeManager 是 FailoverManager 对 Agent 连接与状态的依赖。
// 抽成接口便于单测（真机实现是 *NodeServerManager）。
type FailoverNodeManager interface {
	IsOnline(serverID int64) bool
	GetServerStatus(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error)
	SyncDataToServer(ctx context.Context, serverID int64, applyAndStart bool) (*agentpb.SyncDataResponse, error)
}

// UserSyncer is the subset of UserSyncService used by the FailoverManager to
// replay credentials after a role switch. Defined locally to avoid an import
// cycle with the service layer.
type UserSyncer interface {
	// SyncServerFull re-sends all in-scope users to the given server.
	SyncServerFull(ctx context.Context, serverID int64) error
	// RemoveServerUsers removes all users from the given server.
	RemoveServerUsers(ctx context.Context, serverID int64) error
}

// RecoverMode controls how the FailoverManager handles recovery after a
// failover event.
type RecoverMode string

const (
	// RecoverModeAutoSwitchBack: when the original primary recovers (Agent
	// online + all cores running for the grace period), traffic is
	// automatically switched back to it. This is the default behavior and
	// suitable for scenarios where the primary node has better hardware,
	// network, or pricing.
	RecoverModeAutoSwitchBack RecoverMode = "auto_switch_back"

	// RecoverModeStayOnPromoted: the promoted backup stays as the new
	// primary permanently. The original primary is demoted to a regular
	// backup. Roles only swap again on the next failover event (i.e. if
	// the new primary also fails). This is suitable for scenarios where
	// you want to minimize switching and let the surviving node keep
	// serving traffic until it itself fails.
	RecoverModeStayOnPromoted RecoverMode = "stay_on_promoted"
)

// getServerRecoverMode returns the recover mode for a server.
// It reads from the server's RecoverMode field, defaulting to auto_switch_back.
func getServerRecoverMode(server *entity.NodeServer) RecoverMode {
	if server.RecoverMode == string(RecoverModeStayOnPromoted) {
		return RecoverModeStayOnPromoted
	}
	return RecoverModeAutoSwitchBack
}

// FailoverManager monitors primary_backup node pairs and automatically
// promotes a backup to primary when the primary goes offline or its core
// processes are all down. When the primary recovers, the behavior depends
// on the server's RecoverMode setting (per-pair configuration).
type FailoverManager struct {
	serverRepo        *repository.NodeServerRepository
	nodeMgr           FailoverNodeManager
	checkInterval     time.Duration
	offlineThreshold  time.Duration // how long before triggering failover
	recoverGrace      time.Duration // how long the original primary must be stable before switching back
	coreDownThreshold time.Duration // how long all cores must be down before triggering failover (even if Agent is online)

	// Track which primaries have been failed over (to detect recovery)
	mu                  sync.RWMutex
	failedOverPrimaries map[int64]bool // original primary server_id → failed over?

	// Track when all cores first went down on each primary (to measure
	// coreDownThreshold). Cleared when any core comes back up.
	coreDownMu    sync.Mutex
	coreDownSince map[int64]time.Time // server_id → time when all cores first detected down

	// Track since when a backup_recovering node has been continuously healthy.
	// 原实现只看「心跳足够新」，等于原主一上线就切回，网络抖动会来回切；
	// 现在要求连续健康 recoverGrace 才回切（见 checkRecovering）。
	recoverMu    sync.Mutex
	recoverSince map[int64]time.Time // server_id → 本轮连续健康的起点

	// userSyncer replays user credentials after a failover switches the
	// active primary. Guarded by userSyncMu.
	userSyncMu sync.Mutex
	userSyncer UserSyncer
}

// NewFailoverManager creates a new FailoverManager.
func NewFailoverManager(serverRepo *repository.NodeServerRepository, nodeMgr FailoverNodeManager) *FailoverManager {
	return &FailoverManager{
		serverRepo:          serverRepo,
		nodeMgr:             nodeMgr,
		checkInterval:       30 * time.Second,
		offlineThreshold:    2 * time.Minute,
		recoverGrace:        1 * time.Minute,
		coreDownThreshold:   3 * time.Minute,
		failedOverPrimaries: make(map[int64]bool),
		coreDownSince:       make(map[int64]time.Time),
		recoverSince:        make(map[int64]time.Time),
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

// WithCoreDownThreshold sets how long all cores must be down (while Agent is
// still online) before triggering failover.
func (m *FailoverManager) WithCoreDownThreshold(d time.Duration) *FailoverManager {
	m.coreDownThreshold = d
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

	log.Printf("[FailoverManager] started, checking every %v (per-pair recover mode)", m.checkInterval)

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
			// Current active primary: fail over if it has been offline too long
			// or if all its core processes have been down too long.
			m.checkActivePrimary(ctx, &server)
		case entity.FailoverRoleBackupRecovering:
			// Original primary that was demoted during failover; switch back
			// once it has been stable online for the grace period.
			// The recover mode is read from the server itself.
			mode := getServerRecoverMode(&server)
			if mode == RecoverModeAutoSwitchBack {
				m.checkRecovering(ctx, &server)
			} else {
				// stay_on_promoted: convert backup_recovering → backup permanently
				server.FailoverRole = entity.FailoverRoleBackup
				server.UpdatedAt = time.Now()
				if err := m.serverRepo.Update(ctx, &server); err != nil {
					log.Printf("[FailoverManager] failed to convert backup_recovering → backup for server %d: %v", server.ID, err)
				} else {
					log.Printf("[FailoverManager] stay_on_promoted: converted server %d from backup_recovering to backup", server.ID)
				}
				m.mu.Lock()
				delete(m.failedOverPrimaries, server.ID)
				m.mu.Unlock()
			}
		}
	}
	return nil
}

// checkActivePrimary fails over a primary that has been offline past the
// threshold, or whose core processes have all been down past the
// coreDownThreshold. It then hands user replay to the UserSyncService.
func (m *FailoverManager) checkActivePrimary(ctx context.Context, primary *entity.NodeServer) {
	// Skip local servers
	if primary.IsLocal {
		return
	}

	// Check if already failed over
	m.mu.RLock()
	failedOver := m.failedOverPrimaries[primary.ID]
	m.mu.RUnlock()
	if failedOver {
		return // Already failed over, waiting for recovery (auto_switch_back) or permanently demoted (stay_on_promoted)
	}

	// ── Condition 1: Agent offline (heartbeat stopped) ──
	if !m.nodeMgr.IsOnline(primary.ID) {
		// Clear any core-down tracking (Agent is fully offline, use heartbeat path)
		m.clearCoreDownTracking(primary.ID)

		if primary.LastHeartbeatAt == nil {
			return
		}

		timeSinceHeartbeat := time.Since(*primary.LastHeartbeatAt)
		if timeSinceHeartbeat < m.offlineThreshold {
			return // Not offline long enough yet
		}

		log.Printf("[FailoverManager] primary %s (id=%d) Agent offline for %v, triggering failover",
			primary.Name, primary.ID, timeSinceHeartbeat)
		if err := m.promoteBackup(ctx, primary, "agent offline"); err != nil {
			log.Printf("[FailoverManager] failover failed: %v", err)
		}
		return
	}

	// ── Condition 2: Agent online but all cores down ──
	// Check if all core processes are in a non-running state.
	// This catches the scenario where the Agent is alive but the core (e.g.
	// xray) has crashed and the Guardian has exhausted its restart attempts.
	allCoresDown, err := m.checkAllCoresDown(ctx, primary.ID)
	if err != nil {
		// Can't determine core status (e.g. status not yet reported) —
		// don't trigger failover on unknown, but also don't clear tracking
		// since we don't know the actual state.
		return
	}

	if !allCoresDown {
		// At least one core is running — clear any core-down tracking
		m.clearCoreDownTracking(primary.ID)
		return
	}

	// All cores are down — check how long this has been the case
	m.coreDownMu.Lock()
	firstDownAt, exists := m.coreDownSince[primary.ID]
	if !exists {
		// First time detecting all cores down — record the time
		m.coreDownSince[primary.ID] = time.Now()
		m.coreDownMu.Unlock()
		log.Printf("[FailoverManager] primary %s (id=%d): all cores down, starting %v countdown",
			primary.Name, primary.ID, m.coreDownThreshold)
		return
	}
	m.coreDownMu.Unlock()

	coreDownDuration := time.Since(firstDownAt)
	if coreDownDuration < m.coreDownThreshold {
		log.Printf("[FailoverManager] primary %s (id=%d): all cores down for %v (threshold %v)",
			primary.Name, primary.ID, coreDownDuration, m.coreDownThreshold)
		return
	}

	log.Printf("[FailoverManager] primary %s (id=%d): all cores down for %v, triggering failover",
		primary.Name, primary.ID, coreDownDuration)
	if err := m.promoteBackup(ctx, primary, "all cores down"); err != nil {
		log.Printf("[FailoverManager] failover failed: %v", err)
	}
}

// checkAllCoresDown returns true if the server has at least one configured core
// and none of them are in "running" state. Returns an error if core status
// cannot be determined (e.g. Agent just connected and hasn't reported yet).
func (m *FailoverManager) checkAllCoresDown(ctx context.Context, serverID int64) (bool, error) {
	resp, err := m.nodeMgr.GetServerStatus(ctx, serverID)
	if err != nil {
		return false, err
	}
	if resp == nil || len(resp.Cores) == 0 {
		// No cores configured — can't determine, don't trigger failover
		return false, nil
	}

	for _, core := range resp.Cores {
		if core.State == "running" {
			return false, nil
		}
	}
	return true, nil
}

// clearCoreDownTracking removes the core-down timestamp for a server.
// Called when the server comes back online, at least one core is running,
// or the server goes fully offline (handled by the heartbeat path instead).
func (m *FailoverManager) clearCoreDownTracking(serverID int64) {
	m.coreDownMu.Lock()
	delete(m.coreDownSince, serverID)
	m.coreDownMu.Unlock()
}

// checkRecovering switches traffic back to the original primary after it has
// been **continuously** healthy (Agent online + at least one core running) for
// recoverGrace. Only called in RecoverModeAutoSwitchBack.
func (m *FailoverManager) checkRecovering(ctx context.Context, originalPrimary *entity.NodeServer) {
	if originalPrimary.IsLocal {
		return
	}
	if !m.nodeMgr.IsOnline(originalPrimary.ID) {
		m.clearRecoverTracking(originalPrimary.ID)
		return
	}

	// 必须至少有一个核心在跑 —— 还没有核心起来的节点切回去也没用。
	allCoresDown, err := m.checkAllCoresDown(ctx, originalPrimary.ID)
	if err != nil || allCoresDown {
		if allCoresDown {
			log.Printf("[FailoverManager] recovering primary %s (id=%d): all cores still down, waiting",
				originalPrimary.Name, originalPrimary.ID)
		}
		m.clearRecoverTracking(originalPrimary.ID)
		return
	}

	// 连续健康计时：任何一次不健康都会清零，避免短暂重连就回切导致抖动。
	m.recoverMu.Lock()
	since, ok := m.recoverSince[originalPrimary.ID]
	if !ok {
		since = time.Now()
		m.recoverSince[originalPrimary.ID] = since
	}
	m.recoverMu.Unlock()

	stable := time.Since(since)
	if stable < m.recoverGrace {
		log.Printf("[FailoverManager] recovering primary %s (id=%d): healthy for %v, need %v before switch-back",
			originalPrimary.Name, originalPrimary.ID, stable.Truncate(time.Second), m.recoverGrace)
		return
	}

	log.Printf("[FailoverManager] recovering primary %s (id=%d): healthy for %v, switching back",
		originalPrimary.Name, originalPrimary.ID, stable.Truncate(time.Second))
	m.recoverPrimary(ctx, originalPrimary)
	m.clearRecoverTracking(originalPrimary.ID)
}

// clearRecoverTracking 重置某节点的连续健康计时。
func (m *FailoverManager) clearRecoverTracking(serverID int64) {
	m.recoverMu.Lock()
	delete(m.recoverSince, serverID)
	m.recoverMu.Unlock()
}

// promoteBackup promotes the backup node to primary.
// reason is a human-readable string describing why the failover was triggered
// (e.g. "agent offline", "all cores down").
// The recover mode is read from the primary server's configuration.
//
// 自动和手动切换都走这里：校验 → 换角色 → 重推用户 → 兜底同步配置。
// 返回 error 便于手动切换接口把失败原因回给前端；自动路径调用方只记日志。
func (m *FailoverManager) promoteBackup(ctx context.Context, primary *entity.NodeServer, reason string) error {
	if primary == nil {
		return fmt.Errorf("primary 节点为空")
	}
	if primary.PartnerServerID == 0 {
		return fmt.Errorf("节点 %s 未配置主备伙伴", primary.Name)
	}

	backup, err := m.serverRepo.GetByID(ctx, primary.PartnerServerID)
	if err != nil {
		return fmt.Errorf("找不到伙伴节点 %d：%w", primary.PartnerServerID, err)
	}

	if backup.FailoverRole != entity.FailoverRoleBackup {
		return fmt.Errorf("伙伴节点 %s 当前角色是 %q，不是 backup", backup.Name, backup.FailoverRole)
	}

	// Check if backup is online
	if !m.nodeMgr.IsOnline(backup.ID) {
		return fmt.Errorf("伙伴节点 %s 离线，无法切换", backup.Name)
	}

	// Verify backup has at least one running core — don't promote a
	// backup whose cores are also down.
	backupCoresDown, err := m.checkAllCoresDown(ctx, backup.ID)
	if err != nil {
		return fmt.Errorf("无法获取 %s 的核心状态：%w", backup.Name, err)
	}
	if backupCoresDown {
		return fmt.Errorf("%s 的核心全部未运行，无法切换", backup.Name)
	}

	// Read recover mode from the primary server's configuration
	mode := getServerRecoverMode(primary)

	log.Printf("[FailoverManager] ★ FAILOVER: primary %s (id=%d) reason=%s, promoting backup %s (id=%d), recover_mode=%s",
		primary.Name, primary.ID, reason, backup.Name, backup.ID, mode)

	// Promote backup
	backup.FailoverRole = entity.FailoverRolePrimary
	backup.BillingEnabled = true
	backup.UpdatedAt = time.Now()
	if err := m.serverRepo.Update(ctx, backup); err != nil {
		return fmt.Errorf("提升 %s 失败：%w", backup.Name, err)
	}

	// Demote primary based on recover mode
	if mode == RecoverModeStayOnPromoted {
		// Permanently demote to backup — no auto-recovery
		primary.FailoverRole = entity.FailoverRoleBackup
		log.Printf("[FailoverManager] stay_on_promoted: original primary %s demoted to backup permanently", primary.Name)
	} else {
		// Demote to backup_recovering — will auto-recover when stable
		primary.FailoverRole = entity.FailoverRoleBackupRecovering
	}

	primary.BillingEnabled = false
	primary.UpdatedAt = time.Now()
	if err := m.serverRepo.Update(ctx, primary); err != nil {
		// 回滚提升，避免出现两个 primary。
		backup.FailoverRole = entity.FailoverRoleBackup
		backup.BillingEnabled = false
		backup.UpdatedAt = time.Now()
		_ = m.serverRepo.Update(ctx, backup)
		return fmt.Errorf("降级 %s 失败：%w（已回滚提升）", primary.Name, err)
	}

	// Track failover (only for auto_switch_back recovery tracking)
	m.mu.Lock()
	if mode == RecoverModeAutoSwitchBack {
		m.failedOverPrimaries[primary.ID] = true
	} else {
		// stay_on_promoted: no recovery tracking needed
		delete(m.failedOverPrimaries, primary.ID)
	}
	m.mu.Unlock()

	// Clear core-down / recover tracking for the demoted primary
	m.clearCoreDownTracking(primary.ID)
	m.clearRecoverTracking(primary.ID)

	if mode == RecoverModeStayOnPromoted {
		log.Printf("[FailoverManager] failover complete (stay_on_promoted): %s is now primary, %s is backup (permanent)",
			backup.Name, primary.Name)
	} else {
		log.Printf("[FailoverManager] failover complete: %s is now primary, %s is recovering",
			backup.Name, primary.Name)
	}

	// Phase-4: replay user credentials to the newly promoted primary and
	// strip them from the demoted (recovering) node so only the active
	// primary serves traffic.
	m.replayAfterSwitch(ctx, backup.ID, primary.ID)

	// 兜底：给新主补一次全量 SyncData，确保它的运行配置/用户与面板一致
	// （P1 配置常态同步上线前，备节点配置可能还没跟上）。
	go m.syncPromotedPrimary(backup.ID)

	return nil
}

// syncPromotedPrimary 给刚提升的节点补一次全量 SyncData（异步、失败只记日志）。
func (m *FailoverManager) syncPromotedPrimary(serverID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := m.nodeMgr.SyncDataToServer(ctx, serverID, true); err != nil {
		log.Printf("[FailoverManager] sync data to promoted primary %d failed: %v", serverID, err)
	}
}

// recoverPrimary switches back to the original primary after it recovers.
// Only called in RecoverModeAutoSwitchBack.
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

	log.Printf("[FailoverManager] ★ RECOVERY: original primary %s (id=%d) back online with healthy cores, switching back from %s",
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

// CheckOnce 立即执行一轮主备健康检查（面板「立即检查」按钮用）。
func (m *FailoverManager) CheckOnce(ctx context.Context) error {
	return m.checkAllPairs(ctx)
}

// ForceFailover 手动切换：跳过故障阈值，但仍校验目标在线且至少一个核心在跑。
// serverID 可以是主备对中的任意一方：传 primary 表示切到伙伴；传 backup 表示把它提升。
func (m *FailoverManager) ForceFailover(ctx context.Context, serverID int64) error {
	srv, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("节点 %d 不存在：%w", serverID, err)
	}
	if srv.NodeType != entity.NodeTypePrimaryBackup {
		return fmt.Errorf("节点 %s 不是主备类型（当前 %s）", srv.Name, srv.NodeType)
	}
	switch srv.FailoverRole {
	case entity.FailoverRolePrimary:
		return m.promoteBackup(ctx, srv, "manual")
	case entity.FailoverRoleBackup:
		if srv.PartnerServerID == 0 {
			return fmt.Errorf("节点 %s 未配置主备伙伴", srv.Name)
		}
		partner, err := m.serverRepo.GetByID(ctx, srv.PartnerServerID)
		if err != nil {
			return fmt.Errorf("找不到伙伴节点 %d：%w", srv.PartnerServerID, err)
		}
		if partner.FailoverRole != entity.FailoverRolePrimary {
			return fmt.Errorf("伙伴节点 %s 当前不是 primary（%q）", partner.Name, partner.FailoverRole)
		}
		return m.promoteBackup(ctx, partner, "manual (requested from backup)")
	case entity.FailoverRoleBackupRecovering:
		return fmt.Errorf("节点 %s 正在恢复观察期，请等自动回切，或对它的伙伴执行切换", srv.Name)
	default:
		return fmt.Errorf("节点 %s 的故障切换角色未知：%q", srv.Name, srv.FailoverRole)
	}
}

// GetFailoverStatus returns the current failover status for a primary_backup pair.
func (m *FailoverManager) GetFailoverStatus(ctx context.Context, serverID int64) (*FailoverStatus, error) {
	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	mode := getServerRecoverMode(server)

	m.mu.RLock()
	failedOver := m.failedOverPrimaries[serverID]
	m.mu.RUnlock()

	status := &FailoverStatus{
		ServerID:    serverID,
		NodeType:    server.NodeType,
		CurrentRole: server.FailoverRole,
		Online:      m.nodeMgr.IsOnline(serverID),
		PartnerID:   server.PartnerServerID,
		RecoverMode: string(mode),
	}

	if server.LastHeartbeatAt != nil {
		status.LastHeartbeatAge = int64(time.Since(*server.LastHeartbeatAt).Seconds())
	}

	// Check core health (only if Agent is online)
	if status.Online {
		allDown, err := m.checkAllCoresDown(ctx, serverID)
		if err == nil {
			status.AllCoresDown = allDown
		}
	}

	status.HasFailedOver = failedOver

	// Include core-down duration if being tracked
	m.coreDownMu.Lock()
	if since, ok := m.coreDownSince[serverID]; ok {
		status.CoreDownDuration = int64(time.Since(since).Seconds())
	}
	m.coreDownMu.Unlock()

	return status, nil
}

// FailoverStatus represents the failover state of a server.
type FailoverStatus struct {
	ServerID     int64  `json:"server_id"`
	NodeType     string `json:"node_type"`
	CurrentRole  string `json:"current_role"`
	Online       bool   `json:"online"`
	AllCoresDown bool   `json:"all_cores_down"`
	// 两个 *_seconds 字段都用秒：time.Duration 直接 JSON 化是纳秒，前端按秒显示会翻 1e9 倍。
	CoreDownDuration int64  `json:"core_down_duration_seconds"`
	PartnerID        int64  `json:"partner_id"`
	LastHeartbeatAge int64  `json:"last_heartbeat_age_seconds"`
	HasFailedOver    bool   `json:"has_failed_over"`
	RecoverMode      string `json:"recover_mode"`
}
