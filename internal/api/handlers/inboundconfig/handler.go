package inboundconfig

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/domain"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/inboundconfig"
)

// ConfigSyncHook triggers config sync to a specific server after data changes.
// Implemented by configSyncHookAdapter in router.go.
// When triggerMirrorSync=true (inbound changes), the adapter also triggers
// SyncMirrorsForPrimary, which calls SyncDataToServer for each mirror node.
// SyncData detects mirror nodes and generates config from the primary's data.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// HotApplySyncHook 是 ConfigSyncHook 的可选扩展：声明该核心的改动已经通过核心 API
// 热下发到运行中的进程（xray 的入站/出站/路由），配置写盘后不要重启核心。
// Implemented by configSyncHookAdapter in router.go.
type HotApplySyncHook interface {
	SyncServerConfigHotApplied(ctx context.Context, serverID int64, coreType string)
}

// triggerConfigSync 触发该节点的配置同步。
//
// 支持热下发的核心（见 domain.CoreType.SupportsHotApply）只写盘、不重启核心：面板的
// 入站/出站/路由编辑器写库后一定会调用 xray/control/* 把改动下发到运行中的进程。
// 其余核心（sing-box）没有热下发接口，必须重启核心才能生效。
func (h *Handler) triggerConfigSync(serverID int64, coreType string) {
	if h.syncHook == nil {
		return
	}
	if domain.CoreType(coreType).SupportsHotApply() {
		if hot, ok := h.syncHook.(HotApplySyncHook); ok {
			go hot.SyncServerConfigHotApplied(context.Background(), serverID, coreType)
			return
		}
	}
	// 兜底：hook 未实现热下发扩展（或核心不支持），照旧同步（会重启核心）
	go h.syncHook.SyncServerConfig(context.Background(), serverID, coreType)
}

// SyncDataHook triggers a full SyncData push to a remote Agent.
// This updates the Agent's local SQLite DB (inbound_configs, etc).
type SyncDataHook interface {
	SyncServerData(ctx context.Context, serverID int64)
}

// MirrorChecker checks if a server is a mirror node.
// Mirror nodes are read-only — inbound configs can only be modified on
// the primary node. Implemented by nodeserver.Service or a closure.
type MirrorChecker interface {
	IsMirrorNode(ctx context.Context, serverID int64) bool
}

// Handler handles inbound config HTTP requests.
type Handler struct {
	svc           *inboundconfig.Service
	syncHook      ConfigSyncHook
	syncDataHook  SyncDataHook
	mirrorChecker MirrorChecker
	downloadSync  DownloadAccountSyncHook
}

// DownloadAccountSyncHook 在入站保存后，为「该入站引用的下行入口」补齐已有客户账号。
// 没有它时，先建客户、后加入站关联的顺序会导致下行入站里没有对应凭据 → 客户端
// 从下行入口认证失败（下行入站通常不在套餐节点组里，不会被批量建号覆盖）。
type DownloadAccountSyncHook interface {
	SyncDownloadAccounts(ctx context.Context, uplinkInboundID int64) error
}

// SetDownloadAccountSyncHook 注入下行入站账号补齐能力。
func (h *Handler) SetDownloadAccountSyncHook(hook DownloadAccountSyncHook) {
	h.downloadSync = hook
}

// NewHandler creates a new inbound config handler.
func NewHandler(svc *inboundconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// SetSyncHook sets the config sync hook for triggering remote sync after changes.
// The hook adapter (configSyncHookAdapter) handles both config sync and mirror
// fanout — when triggerMirrorSync=true, it calls SyncMirrorsForPrimary which
// triggers SyncData to all mirror nodes. SyncData detects mirror nodes and
// generates config from the primary's data at runtime.
func (h *Handler) SetSyncHook(hook ConfigSyncHook) {
	h.syncHook = hook
}

// SetSyncDataHook sets the SyncData hook for triggering full data sync to remote Agents.
func (h *Handler) SetSyncDataHook(hook SyncDataHook) {
	h.syncDataHook = hook
}

// SetMirrorChecker sets the mirror checker for read-only enforcement.
// When set, Create/Update/Delete on mirror nodes will be rejected with
// MIRROR_READONLY error.
func (h *Handler) SetMirrorChecker(checker MirrorChecker) {
	h.mirrorChecker = checker
}

// List handles GET /api/v1/node-servers/:id/inbound-configs.
func (h *Handler) List(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1 // fallback for legacy callers
	}
	coreType := c.Query("core_type")
	configs, err := h.svc.List(c.Request.Context(), serverID, coreType)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	// 流量计数已拆到 inbound_user_traffic 表，这里注回响应副本供 UI 展示；
	// 注回失败只影响展示（前端仍显示 0），不阻断列表。
	_ = h.svc.EnrichWithTraffic(c.Request.Context(), configs)
	response.OK(c, configs)
}

// pathServerID 返回 URL 里的节点 ID（缺失/非法返回 0）。
func pathServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// ensureInboundBelongsToServer 校验 :cfgId 确实属于路径里的 :id 节点。
// 旧实现只按 cfgId 查库，用节点 A 的 URL 可以读/改/删节点 B 的入站（安全审计 v6 M-3）。
func (h *Handler) ensureInboundBelongsToServer(c *gin.Context, cfg *entity.InboundConfig) bool {
	sid := pathServerID(c)
	if sid <= 0 || cfg.ServerID == sid {
		return true
	}
	response.NotFound(c, "NOT_FOUND", "inbound config not found")
	return false
}

// Get handles GET /api/v1/node-servers/:id/inbound-configs/:cfgId.
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	cfg, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	if !h.ensureInboundBelongsToServer(c, cfg) {
		return
	}
	// 同 List：把流量计数注回响应副本供 UI 展示
	list := []entity.InboundConfig{*cfg}
	_ = h.svc.EnrichWithTraffic(c.Request.Context(), list)
	cfg = &list[0]
	response.OK(c, cfg)
}

// checkMirrorReadOnly rejects write operations on mirror nodes.
// Mirror nodes are read-only — their config is synced from the primary.
func (h *Handler) checkMirrorReadOnly(c *gin.Context, serverID int64) bool {
	if h.mirrorChecker == nil {
		return false
	}
	if h.mirrorChecker.IsMirrorNode(c.Request.Context(), serverID) {
		response.BadRequest(c, "MIRROR_READONLY",
			"镜像节点配置只能从主节点同步，不能手动修改")
		return true
	}
	return false
}

// Create handles POST /api/v1/node-servers/:id/inbound-configs.
func (h *Handler) Create(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	var req struct {
		CoreType   string          `json:"core_type"`
		Remark     string          `json:"remark"`
		Protocol   string          `json:"protocol"`
		Tag        string          `json:"tag"`
		Listen     string          `json:"listen"`
		Port       int             `json:"port"`
		ConfigJSON json.RawMessage `json:"config_json"`
		// 用指针区分"没传 enabled"与"传了 false"：全量覆盖语义下，漏传会把入站意外禁用。
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// Reject writes on mirror nodes (read-only)
	if h.checkMirrorReadOnly(c, serverID) {
		return
	}
	// port 是所有入站都必需的**标识端口**，Unix Socket / 抽象 socket（@name）也一样：
	// 订阅收集会跳过 port==0 的节点、分享链接会生成 ":0"、端口跳跃/防火墙规则也拿它当目标。
	// 以前前端在 sock 模式下不传 port → 行建出来了但"保存没用"（订阅里消失、二维码端口 0）。
	if req.Port <= 0 {
		response.BadRequest(c, "INVALID_PORT", "端口必须 >= 1（Unix Socket / @抽象 socket 也需要：用于分享、订阅与规则标识）")
		return
	}
	listen := strings.TrimSpace(req.Listen)
	if listen == "" {
		// 同 Update：允许只提供 config_json.listen（导入/脚本/旧前端）。
		listen = listenFromConfigJSON(req.ConfigJSON)
	}
	cfg, err := h.svc.Create(c.Request.Context(), serverID, req.CoreType, req.Remark, req.Protocol, req.Tag, listen, req.Port, string(req.ConfigJSON), req.Enabled)
	if err != nil {
		if errors.Is(err, inboundconfig.ErrServerNamesConflict) {
			response.BadRequest(c, "SERVER_NAMES_CONFLICT", err.Error())
			return
		}
		if errors.Is(err, inboundconfig.ErrProtocolSettingsMismatch) {
			response.BadRequest(c, "PROTOCOL_SETTINGS_MISMATCH", err.Error())
			return
		}
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously.
	// No serverID > 1 restriction: for the primary node (serverID=1),
	// the configSyncHookAdapter with triggerMirrorSync=true will
	// call SyncMirrorsForPrimary, which triggers SyncData to all mirror
	// nodes. SyncData detects mirror nodes and generates config from
	// the primary's data at runtime.
	h.triggerConfigSync(serverID, req.CoreType)
	h.syncDownloadAccounts(cfg, string(req.ConfigJSON))

	response.OK(c, cfg)
}

// Update handles PUT /api/v1/node-servers/:id/inbound-configs/:cfgId.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Remark     string          `json:"remark"`
		Protocol   string          `json:"protocol"`
		Tag        string          `json:"tag"`
		Listen     string          `json:"listen"`
		Port       int             `json:"port"`
		ConfigJSON json.RawMessage `json:"config_json"`
		// 用指针区分"没传 enabled"与"传了 false"：全量覆盖语义下漏传会把入站意外禁用。
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// Reject writes on mirror nodes (read-only)
	// First get the config to find its serverID
	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	if !h.ensureInboundBelongsToServer(c, existing) {
		return
	}
	if h.checkMirrorReadOnly(c, existing.ServerID) {
		return
	}
	// ⚠ Update 是**全量覆盖**语义（service 用入参把整行写回）。以前传部分 payload（例如只给
	// {"enabled": true}）会把 tag / protocol / config_json / port 一起清空 → 该入站直接从生成
	// 配置里消失（实测踩过：行被清空后节点上不再监听、运行配置里也搜不到它，排查很久）。
	// 这里对"关键字段缺失或为空"的情况**保留原值**；要真正清空请走删除接口。
	if strings.TrimSpace(req.Tag) == "" {
		req.Tag = existing.Tag
	}
	if strings.TrimSpace(req.Protocol) == "" {
		req.Protocol = existing.Protocol
	}
	if req.Port <= 0 {
		req.Port = existing.Port
	}
	if strings.TrimSpace(req.Listen) == "" {
		// 前端历史上存在"只改了 socket 路径、listen 传空"的路径（buildInbound 没带
		// _socketPath，effectiveListen 返回空串）→ 这里保留旧值就等于"保存没生效"。
		// config_json 里的 listen 是同一份数据的副本，用它兜底（仍为空才保留旧值）。
		req.Listen = listenFromConfigJSON(req.ConfigJSON)
		if strings.TrimSpace(req.Listen) == "" {
			req.Listen = existing.Listen
		}
	}
	if v := strings.TrimSpace(string(req.ConfigJSON)); v == "" || v == "null" {
		req.ConfigJSON = json.RawMessage(existing.ConfigJSON)
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	cfg, err := h.svc.Update(c.Request.Context(), id, req.Remark, req.Protocol, req.Tag, req.Listen, req.Port, string(req.ConfigJSON), enabled)
	if err != nil {
		if errors.Is(err, inboundconfig.ErrServerNamesConflict) {
			response.BadRequest(c, "SERVER_NAMES_CONFLICT", err.Error())
			return
		}
		if errors.Is(err, inboundconfig.ErrProtocolSettingsMismatch) {
			response.BadRequest(c, "PROTOCOL_SETTINGS_MISMATCH", err.Error())
			return
		}
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously.
	// No serverID > 1 restriction: primary node changes must trigger
	// mirror sync via configSyncHookAdapter → SyncMirrorsForPrimary.
	h.triggerConfigSync(cfg.ServerID, cfg.CoreType)
	h.syncDownloadAccounts(cfg, string(req.ConfigJSON))

	response.OK(c, cfg)
}

// syncDownloadAccounts 在入站保存后异步补齐「被该入站引用的下行入口」里的客户账号。
// 只在确实存在下行引用时执行，避免每次保存都全量扫用户。
//
// configJSON 必须传**明文**配置：仓储 Update/Create 会把实体的 ConfigJSON 就地替换成
// 加密串（enc: 前缀），用返回值解析会永远读不到 _downloadInboundId。
func (h *Handler) syncDownloadAccounts(cfg *entity.InboundConfig, configJSON string) {
	if h.downloadSync == nil || cfg == nil {
		return
	}
	targetID := domain.DownloadTargetID(configJSON)
	if targetID <= 0 {
		return
	}
	uplinkID := cfg.ID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := h.downloadSync.SyncDownloadAccounts(ctx, uplinkID); err != nil {
			log.Printf("[inbound] sync download accounts for %d failed: %v", uplinkID, err)
		}
	}()
}

// Delete handles DELETE /api/v1/node-servers/:id/inbound-configs/:cfgId.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	// Get server_id before deletion for sync trigger；同时校验归属节点
	serverID := int64(1)
	coreType := "xray"
	existing, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	if !h.ensureInboundBelongsToServer(c, existing) {
		return
	}
	serverID = existing.ServerID
	coreType = existing.CoreType
	// Reject writes on mirror nodes (read-only)
	if h.checkMirrorReadOnly(c, serverID) {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously.
	// No serverID > 1 restriction: primary node changes must trigger
	// mirror sync via configSyncHookAdapter → SyncMirrorsForPrimary.
	h.triggerConfigSync(serverID, coreType)

	response.OK(c, gin.H{"message": "deleted"})
}

// UsedServerNames handles GET /api/v1/node-servers/:id/used-server-names.
// 返回该节点上所有入站（xray + sing-box）已占用的 Server Names，供编辑器标记「已使用」。
func (h *Handler) UsedServerNames(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}
	items, err := h.svc.UsedServerNames(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}

// PortHopRules handles GET /api/v1/node-servers/:id/port-hop/rules.
// 返回该节点已启用的端口跳跃规则与建议的空闲段，供编辑器提前标出冲突段。
func (h *Handler) PortHopRules(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}
	items, suggest := h.svc.PortHopRules(c.Request.Context(), serverID)
	response.OK(c, gin.H{"items": items, "suggest": suggest})
}

// ResetAllTraffic handles POST /api/v1/node-servers/:id/inbound-configs/reset-all-traffic.
// Resets _usedUp and _usedDown to 0 for all users in all inbound configs on the server.
func (h *Handler) ResetAllTraffic(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}

	count, err := h.svc.ResetAllTraffic(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}

	// Trigger config sync AND SyncData to the target server asynchronously.
	// No serverID > 1 restriction: primary node changes must trigger
	// mirror sync via configSyncHookAdapter → SyncMirrorsForPrimary.
	// 流量重置不改变核心行为（计数由 xray stats API 清零），xray 走热下发不重启。
	h.triggerConfigSync(serverID, "xray")

	if h.syncDataHook != nil {
		go h.syncDataHook.SyncServerData(context.Background(), serverID)
	}

	response.OK(c, gin.H{
		"message":     "all traffic stats reset",
		"users_reset": count,
	})
}

// PauseUser handles POST /api/v1/inbound-configs/:id/users/:email/pause.
// Sets _paused=true on the user in config JSON.
func (h *Handler) PauseUser(c *gin.Context) {
	h.setUserPaused(c, true)
}

// ResumeUser handles POST /api/v1/inbound-configs/:id/users/:email/resume.
// Sets _paused=false on the user in config JSON.
func (h *Handler) ResumeUser(c *gin.Context) {
	h.setUserPaused(c, false)
}

func (h *Handler) setUserPaused(c *gin.Context, paused bool) {
	id, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	email := c.Param("email")
	if email == "" {
		response.BadRequest(c, "INVALID_PARAMS", "email is required")
		return
	}

	cfg, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}

	// Parse config JSON
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
		response.InternalError(c, "PARSE_FAILED", err.Error())
		return
	}

	// Find user in the inbound's user containers.
	// xray-style configs put users in settings.users/clients;
	// sing-box style configs put users directly in the top-level users array.
	containers := []map[string]interface{}{config}
	if settings, ok := config["settings"].(map[string]interface{}); ok {
		containers = append(containers, settings)
	}

	found := false
	for _, container := range containers {
		for _, key := range []string{"users", "clients"} {
			users, ok := container[key].([]interface{})
			if !ok {
				continue
			}
			for _, u := range users {
				user, ok := u.(map[string]interface{})
				if !ok {
					continue
				}
				userEmail, _ := user["email"].(string)
				if userEmail == "" {
					userEmail, _ = user["name"].(string)
				}
				if userEmail == email {
					user["_paused"] = paused
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		response.NotFound(c, "USER_NOT_FOUND", "user not found in config")
		return
	}

	// Save updated config
	configJSON, err := json.Marshal(config)
	if err != nil {
		response.InternalError(c, "MARSHAL_FAILED", err.Error())
		return
	}

	_, err = h.svc.Update(c.Request.Context(), id, cfg.Remark, cfg.Protocol, cfg.Tag, cfg.Listen, cfg.Port, string(configJSON), cfg.Enabled)
	if err != nil {
		if errors.Is(err, inboundconfig.ErrProtocolSettingsMismatch) {
			response.BadRequest(c, "PROTOCOL_SETTINGS_MISMATCH", err.Error())
			return
		}
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	if paused {
		response.OK(c, gin.H{"message": "user paused", "email": email})
	} else {
		response.OK(c, gin.H{"message": "user resumed", "email": email})
	}
}

// listenFromConfigJSON 取 config_json 顶层的 listen（老前端/脚本可能只填这里）。
// 只认字符串且非空；解析失败返回空串。
func listenFromConfigJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var cfg struct {
		Listen string `json:"listen"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Listen)
}
