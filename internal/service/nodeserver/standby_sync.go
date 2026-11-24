package nodeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// standbyConfigSyncKey 是主备配置同步总开关（system_settings）。
// 缺省 = 开启；写入 "false"/"0"/"off" 可一键回到 P0 的冷备行为。
const standbyConfigSyncKey = "failover.config_sync_enabled"

// standbySnapshotKeep 是每节点保留的配置快照份数（回滚用）。
const standbySnapshotKeep = 20

// StandbyReconciler 把主节点（role=primary）的配置**单向对齐**到主备对里的备节点
// （role=backup）：主有备无 → 建；两边都有 → 改；主无备有 → 删。
//
// 为什么需要：备节点必须在任意时刻「配置等价」才能接管。只增不删会让主上已停用的
// 入口在切换后复活，残留的端口/SNI 也会冲突。
//
// 安全边界：
//   - 只处理 server_id=备节点 的 inbounds / core_configs / nginx_sites / caddy_sites；
//   - 只由 role=primary 发起，backup_recovering 不反向覆盖；
//   - 对齐前先落快照，写库中途失败按快照恢复（推送失败不回滚，Agent 重连会用最新 DB）。
type StandbyReconciler struct {
	serverRepo  *repository.NodeServerRepository
	inboundRepo *repository.InboundConfigRepository
	coreRepo    *repository.CoreConfigRepository
	nginxRepo   *repository.NginxSiteRepository
	caddyRepo   *repository.CaddySiteRepository
	settingRepo *repository.SystemSettingRepository
	dataDir     string
	// push 把最新配置推给备节点的 Agent（失败只记日志）。
	push func(ctx context.Context, serverID int64) error
}

// NewStandbyReconciler 创建主备配置对齐器。
func NewStandbyReconciler(
	serverRepo *repository.NodeServerRepository,
	inboundRepo *repository.InboundConfigRepository,
	coreRepo *repository.CoreConfigRepository,
	nginxRepo *repository.NginxSiteRepository,
	caddyRepo *repository.CaddySiteRepository,
	settingRepo *repository.SystemSettingRepository,
	dataDir string,
	push func(ctx context.Context, serverID int64) error,
) *StandbyReconciler {
	return &StandbyReconciler{
		serverRepo:  serverRepo,
		inboundRepo: inboundRepo,
		coreRepo:    coreRepo,
		nginxRepo:   nginxRepo,
		caddyRepo:   caddyRepo,
		settingRepo: settingRepo,
		dataDir:     dataDir,
		push:        push,
	}
}

func (r *StandbyReconciler) enabled(ctx context.Context) bool {
	if r.settingRepo == nil {
		return true
	}
	v, err := r.settingRepo.GetByKey(ctx, standbyConfigSyncKey)
	if err != nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false", "0", "off", "no":
		return false
	}
	return true
}

// ReconcileForPair 对主备对中的任意一方触发一次对齐（内部按角色找同步方向）。
func (r *StandbyReconciler) ReconcileForPair(ctx context.Context, serverID int64) error {
	if r == nil || r.serverRepo == nil {
		return nil
	}
	srv, err := r.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("节点 %d 不存在：%w", serverID, err)
	}
	if srv.NodeType != entity.NodeTypePrimaryBackup {
		return fmt.Errorf("节点 %s 不是主备类型（%s）", srv.Name, srv.NodeType)
	}
	primaryID := srv.ID
	if srv.FailoverRole != entity.FailoverRolePrimary {
		if srv.PartnerServerID == 0 {
			return fmt.Errorf("节点 %s 未配置主备伙伴", srv.Name)
		}
		primaryID = srv.PartnerServerID
	}
	return r.ReconcileForPrimary(ctx, primaryID)
}

// ReconcileForPrimary 以 primaryID 为源对齐它的主备伙伴。非主备主节点时是 no-op。
func (r *StandbyReconciler) ReconcileForPrimary(ctx context.Context, primaryID int64) error {
	if r == nil || r.serverRepo == nil {
		return nil
	}
	if !r.enabled(ctx) {
		return nil
	}
	primary, err := r.serverRepo.GetByID(ctx, primaryID)
	if err != nil {
		return fmt.Errorf("读取主节点 %d 失败：%w", primaryID, err)
	}
	if primary.NodeType != entity.NodeTypePrimaryBackup ||
		primary.FailoverRole != entity.FailoverRolePrimary ||
		primary.PartnerServerID == 0 {
		return nil
	}
	backup, err := r.serverRepo.GetByID(ctx, primary.PartnerServerID)
	if err != nil {
		return fmt.Errorf("读取备节点 %d 失败：%w", primary.PartnerServerID, err)
	}
	if backup.NodeType != entity.NodeTypePrimaryBackup || backup.FailoverRole != entity.FailoverRoleBackup {
		// 只有「当前备节点」才接收对齐（backup_recovering 是原主，不覆盖）。
		return nil
	}

	snap, err := r.snapshot(ctx, backup.ID)
	if err != nil {
		return fmt.Errorf("快照备节点 %d 配置失败：%w", backup.ID, err)
	}
	if err := r.reconcile(ctx, primary.ID, backup.ID); err != nil {
		log.Printf("[StandbySync] reconcile primary %d → backup %d failed: %v；准备按快照回滚", primary.ID, backup.ID, err)
		if rbErr := r.restore(ctx, backup.ID, snap); rbErr != nil {
			log.Printf("[StandbySync] 回滚备节点 %d 失败：%v（快照文件：%s）", backup.ID, rbErr, snap)
		}
		return err
	}
	log.Printf("[StandbySync] primary %d → backup %d reconciled", primary.ID, backup.ID)

	// 推送失败不回滚 DB：备节点下次上线/重连时 SyncData 会读到已对齐的数据。
	if r.push != nil {
		if err := r.push(ctx, backup.ID); err != nil {
			log.Printf("[StandbySync] 推送配置到备节点 %d 失败（DB 已对齐，重连时会再推）：%v", backup.ID, err)
		}
	}
	return nil
}

func (r *StandbyReconciler) reconcile(ctx context.Context, primaryID, backupID int64) error {
	if err := r.reconcileInbounds(ctx, primaryID, backupID); err != nil {
		return fmt.Errorf("对齐入站：%w", err)
	}
	if err := r.reconcileCoreConfigs(ctx, primaryID, backupID); err != nil {
		return fmt.Errorf("对齐核心配置：%w", err)
	}
	if err := r.reconcileNginxSites(ctx, primaryID, backupID); err != nil {
		return fmt.Errorf("对齐 Nginx 站点：%w", err)
	}
	if err := r.reconcileCaddySites(ctx, primaryID, backupID); err != nil {
		return fmt.Errorf("对齐 Caddy 站点：%w", err)
	}
	return nil
}

// ──────────── 入站（按 tag 对齐） ────────────

func (r *StandbyReconciler) reconcileInbounds(ctx context.Context, primaryID, backupID int64) error {
	if r.inboundRepo == nil {
		return nil
	}
	src, err := r.inboundRepo.ListByServerID(ctx, primaryID)
	if err != nil {
		return err
	}
	dst, err := r.inboundRepo.ListByServerID(ctx, backupID)
	if err != nil {
		return err
	}
	dstByTag := make(map[string]entity.InboundConfig, len(dst))
	for _, d := range dst {
		dstByTag[d.Tag] = d
	}
	seen := make(map[string]bool, len(src))
	for _, s := range src {
		seen[s.Tag] = true
		if d, ok := dstByTag[s.Tag]; ok {
			s.ID = d.ID
			s.ServerID = backupID
			s.CreatedAt = d.CreatedAt
			if err := r.inboundRepo.Update(ctx, &s); err != nil {
				return fmt.Errorf("更新入站 %s：%w", s.Tag, err)
			}
			continue
		}
		s.ID = 0
		s.ServerID = backupID
		s.CreatedAt = time.Time{}
		if err := r.inboundRepo.Create(ctx, &s); err != nil {
			return fmt.Errorf("创建入站 %s：%w", s.Tag, err)
		}
	}
	for tag, d := range dstByTag {
		if seen[tag] {
			continue
		}
		if err := r.inboundRepo.Delete(ctx, d.ID); err != nil {
			return fmt.Errorf("删除备节点多余入站 %s：%w", tag, err)
		}
		log.Printf("[StandbySync] backup %d: removed inbound %s (not on primary)", backupID, tag)
	}
	return nil
}

// ──────────── 核心分段配置（按 core_type+section 对齐） ────────────

func (r *StandbyReconciler) reconcileCoreConfigs(ctx context.Context, primaryID, backupID int64) error {
	if r.coreRepo == nil {
		return nil
	}
	src, err := r.coreRepo.List(ctx, primaryID)
	if err != nil {
		return err
	}
	dst, err := r.coreRepo.List(ctx, backupID)
	if err != nil {
		return err
	}
	type key struct{ core, section string }
	dstKeys := make(map[key]bool, len(dst))
	for _, d := range dst {
		dstKeys[key{d.CoreType, d.Section}] = true
	}
	seen := make(map[key]bool, len(src))
	for _, s := range src {
		k := key{s.CoreType, s.Section}
		seen[k] = true
		s.ID = 0 // UpsertSection 按 (server_id, core_type, section) 冲突更新
		s.ServerID = backupID
		if err := r.coreRepo.UpsertSection(ctx, &s); err != nil {
			return fmt.Errorf("写入 %s/%s：%w", s.CoreType, s.Section, err)
		}
	}
	for _, d := range dst {
		k := key{d.CoreType, d.Section}
		if seen[k] {
			continue
		}
		if err := r.coreRepo.DeleteSection(ctx, backupID, d.CoreType, d.Section); err != nil {
			return fmt.Errorf("删除备节点多余分段 %s/%s：%w", d.CoreType, d.Section, err)
		}
	}
	return nil
}

// ──────────── Nginx / Caddy 站点（按 template+domain+path 对齐） ────────────

func (r *StandbyReconciler) reconcileNginxSites(ctx context.Context, primaryID, backupID int64) error {
	if r.nginxRepo == nil {
		return nil
	}
	src, err := r.nginxRepo.List(ctx, primaryID)
	if err != nil {
		return err
	}
	dst, err := r.nginxRepo.List(ctx, backupID)
	if err != nil {
		return err
	}
	dstByKey := make(map[string]entity.NginxSite, len(dst))
	for _, d := range dst {
		dstByKey[nginxSiteKey(d)] = d
	}
	seen := make(map[string]bool, len(src))
	for _, s := range src {
		k := nginxSiteKey(s)
		seen[k] = true
		if d, ok := dstByKey[k]; ok {
			s.ID = d.ID
			s.ServerID = backupID
			s.CreatedAt = d.CreatedAt
			if err := r.nginxRepo.Update(ctx, &s); err != nil {
				return fmt.Errorf("更新 Nginx 站点 %s：%w", k, err)
			}
			continue
		}
		s.ID = 0
		s.ServerID = backupID
		s.CreatedAt = time.Time{}
		if err := r.nginxRepo.Create(ctx, &s); err != nil {
			return fmt.Errorf("创建 Nginx 站点 %s：%w", k, err)
		}
	}
	for k, d := range dstByKey {
		if seen[k] {
			continue
		}
		if err := r.nginxRepo.Delete(ctx, d.ID); err != nil {
			return fmt.Errorf("删除备节点多余 Nginx 站点 %s：%w", k, err)
		}
	}
	return nil
}

func (r *StandbyReconciler) reconcileCaddySites(ctx context.Context, primaryID, backupID int64) error {
	if r.caddyRepo == nil {
		return nil
	}
	src, err := r.caddyRepo.List(ctx, primaryID)
	if err != nil {
		return err
	}
	dst, err := r.caddyRepo.List(ctx, backupID)
	if err != nil {
		return err
	}
	dstByKey := make(map[string]entity.CaddySite, len(dst))
	for _, d := range dst {
		dstByKey[caddySiteKey(d)] = d
	}
	seen := make(map[string]bool, len(src))
	for _, s := range src {
		k := caddySiteKey(s)
		seen[k] = true
		if d, ok := dstByKey[k]; ok {
			s.ID = d.ID
			s.ServerID = backupID
			s.CreatedAt = d.CreatedAt
			if err := r.caddyRepo.Update(ctx, &s); err != nil {
				return fmt.Errorf("更新 Caddy 站点 %s：%w", k, err)
			}
			continue
		}
		s.ID = 0
		s.ServerID = backupID
		s.CreatedAt = time.Time{}
		if err := r.caddyRepo.Create(ctx, &s); err != nil {
			return fmt.Errorf("创建 Caddy 站点 %s：%w", k, err)
		}
	}
	for k, d := range dstByKey {
		if seen[k] {
			continue
		}
		if err := r.caddyRepo.Delete(ctx, d.ID); err != nil {
			return fmt.Errorf("删除备节点多余 Caddy 站点 %s：%w", k, err)
		}
	}
	return nil
}

func nginxSiteKey(s entity.NginxSite) string {
	return s.Template + "|" + s.Domain + "|" + s.Path
}

func caddySiteKey(s entity.CaddySite) string {
	return s.Template + "|" + s.Domain + "|" + s.Path
}

// ──────────── 快照 / 回滚 ────────────

type standbySnapshotData struct {
	ServerID int64                  `json:"server_id"`
	At       time.Time              `json:"at"`
	Inbounds []entity.InboundConfig `json:"inbounds"`
	Core     []entity.CoreConfig    `json:"core_configs"`
	Nginx    []entity.NginxSite     `json:"nginx_sites"`
	Caddy    []entity.CaddySite     `json:"caddy_sites"`
}

// snapshot 把备节点的四类配置写到 data/backups/failover/，返回文件路径。
func (r *StandbyReconciler) snapshot(ctx context.Context, serverID int64) (string, error) {
	if r.dataDir == "" {
		return "", nil
	}
	data := standbySnapshotData{ServerID: serverID, At: time.Now()}
	if r.inboundRepo != nil {
		data.Inbounds, _ = r.inboundRepo.ListByServerID(ctx, serverID)
	}
	if r.coreRepo != nil {
		data.Core, _ = r.coreRepo.List(ctx, serverID)
	}
	if r.nginxRepo != nil {
		data.Nginx, _ = r.nginxRepo.List(ctx, serverID)
	}
	if r.caddyRepo != nil {
		data.Caddy, _ = r.caddyRepo.List(ctx, serverID)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(r.dataDir, "backups", "failover")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("server-%d-%s.json", serverID, time.Now().Format("20060102-150405")))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	r.pruneSnapshots(dir, serverID)
	return path, nil
}

func (r *StandbyReconciler) pruneSnapshots(dir string, serverID int64) {
	entries, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("server-%d-*.json", serverID)))
	if err != nil || len(entries) <= standbySnapshotKeep {
		return
	}
	sort.Strings(entries)
	for _, old := range entries[:len(entries)-standbySnapshotKeep] {
		_ = os.Remove(old)
	}
}

// restore 用快照恢复备节点配置（尽力而为，失败只返回错误由调用方记日志）。
func (r *StandbyReconciler) restore(ctx context.Context, serverID int64, path string) error {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data standbySnapshotData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}

	// 先清空当前（对齐可能写到一半）。
	if r.inboundRepo != nil {
		if cur, err := r.inboundRepo.ListByServerID(ctx, serverID); err == nil {
			for _, c := range cur {
				_ = r.inboundRepo.Delete(ctx, c.ID)
			}
		}
	}
	if r.coreRepo != nil {
		if cur, err := r.coreRepo.List(ctx, serverID); err == nil {
			for _, c := range cur {
				_ = r.coreRepo.DeleteSection(ctx, serverID, c.CoreType, c.Section)
			}
		}
	}
	if r.nginxRepo != nil {
		if cur, err := r.nginxRepo.List(ctx, serverID); err == nil {
			for _, c := range cur {
				_ = r.nginxRepo.Delete(ctx, c.ID)
			}
		}
	}
	if r.caddyRepo != nil {
		if cur, err := r.caddyRepo.List(ctx, serverID); err == nil {
			for _, c := range cur {
				_ = r.caddyRepo.Delete(ctx, c.ID)
			}
		}
	}

	for i := range data.Inbounds {
		it := data.Inbounds[i]
		it.ID = 0
		it.ServerID = serverID
		if err := r.inboundRepo.Create(ctx, &it); err != nil {
			return err
		}
	}
	for i := range data.Core {
		it := data.Core[i]
		it.ID = 0
		it.ServerID = serverID
		if err := r.coreRepo.UpsertSection(ctx, &it); err != nil {
			return err
		}
	}
	for i := range data.Nginx {
		it := data.Nginx[i]
		it.ID = 0
		it.ServerID = serverID
		if err := r.nginxRepo.Create(ctx, &it); err != nil {
			return err
		}
	}
	for i := range data.Caddy {
		it := data.Caddy[i]
		it.ID = 0
		it.ServerID = serverID
		if err := r.caddyRepo.Create(ctx, &it); err != nil {
			return err
		}
	}
	log.Printf("[StandbySync] backup %d restored from snapshot %s", serverID, path)
	return nil
}
