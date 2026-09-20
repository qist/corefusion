package outboundconfig

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/domain"
	"corefusion/internal/service/outboundconfig"
)

// ConfigSyncHook triggers config sync to a specific server after data changes.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// HotApplySyncHook 是 ConfigSyncHook 的可选扩展：声明该核心的改动已经通过核心 API
// 热下发到运行中的进程（xray 的出站/入站/路由），配置写盘后不要重启核心。
// Implemented by configSyncHookAdapter in router.go.
type HotApplySyncHook interface {
	SyncServerConfigHotApplied(ctx context.Context, serverID int64, coreType string)
}

// shouldSyncOutbound 决定出站变更后是否立即触发配置同步。
//
// 本地节点（serverID==1）过去不触发即时同步，交给 30s 一次的周期性同步写配置 ——
// 但周期性同步一旦发现内容变化就会重启核心，等于「用一次重启去应用一个本该热下发的
// 改动」。因此凡是支持热下发的核心（xray），本地节点也必须立即同步（写盘但不重启）；
// 其余核心（sing-box 等）保持原行为，由周期性同步处理。
func (h *Handler) shouldSyncOutbound(serverID int64, coreType string) bool {
	if h.syncHook == nil {
		return false
	}
	return serverID > 1 || domain.CoreType(coreType).SupportsHotApply()
}

// triggerConfigSync 触发该节点的配置同步（热下发支持的核只写盘不重启，详见调用点注释）。
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

// Handler handles outbound config HTTP requests.
type Handler struct {
	svc      *outboundconfig.Service
	syncHook ConfigSyncHook
}

// NewHandler creates a new outbound config handler.
func NewHandler(svc *outboundconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// SetSyncHook sets the config sync hook for triggering remote sync after changes.
func (h *Handler) SetSyncHook(hook ConfigSyncHook) {
	h.syncHook = hook
}

// List handles GET /api/v1/node-servers/:id/outbound-configs.
func (h *Handler) List(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	coreType := c.Query("core_type")
	configs, err := h.svc.List(c.Request.Context(), serverID, coreType)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// Get handles GET /api/v1/node-servers/:id/outbound-configs/:cfgId.
func (h *Handler) Get(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		cfgID, _ = strconv.ParseInt(c.Param("id"), 10, 64) // fallback
	}
	cfg, err := h.svc.GetByID(c.Request.Context(), serverID, cfgID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Create handles POST /api/v1/node-servers/:id/outbound-configs.
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
		ConfigJSON json.RawMessage `json:"config_json"`
		Enabled    bool            `json:"enabled"`
		IsDefault  bool            `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Create(c.Request.Context(), serverID, req.CoreType, req.Remark, req.Protocol, req.Tag, string(req.ConfigJSON), req.Enabled, req.IsDefault)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	if h.shouldSyncOutbound(serverID, req.CoreType) {
		h.triggerConfigSync(serverID, req.CoreType)
	}

	response.OK(c, cfg)
}

// Update handles PUT /api/v1/node-servers/:id/outbound-configs/:cfgId.
func (h *Handler) Update(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		cfgID, _ = strconv.ParseInt(c.Param("id"), 10, 64) // fallback
	}
	var req struct {
		Remark     string          `json:"remark"`
		Protocol   string          `json:"protocol"`
		Tag        string          `json:"tag"`
		ConfigJSON json.RawMessage `json:"config_json"`
		Enabled    bool            `json:"enabled"`
		IsDefault  bool            `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Update(c.Request.Context(), serverID, cfgID, req.Remark, req.Protocol, req.Tag, string(req.ConfigJSON), req.Enabled, req.IsDefault)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	if h.shouldSyncOutbound(serverID, cfg.CoreType) {
		h.triggerConfigSync(serverID, cfg.CoreType)
	}

	response.OK(c, cfg)
}

// Delete handles DELETE /api/v1/node-servers/:id/outbound-configs/:cfgId.
func (h *Handler) Delete(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		cfgID, _ = strconv.ParseInt(c.Param("id"), 10, 64) // fallback
	}
	// Get core type before deletion for sync trigger
	coreType := "xray"
	if existing, err := h.svc.GetByID(c.Request.Context(), serverID, cfgID); err == nil {
		coreType = existing.CoreType
	}

	if err := h.svc.Delete(c.Request.Context(), serverID, cfgID); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	if h.shouldSyncOutbound(serverID, coreType) {
		h.triggerConfigSync(serverID, coreType)
	}

	response.OK(c, nil)
}

// Reorder handles PUT /api/v1/node-servers/:id/outbound-configs/reorder.
// Request body: { "orders": [ { "id": 1, "sort_order": 0 }, ... ] }
func (h *Handler) Reorder(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	var req struct {
		Orders []struct {
			ID        int64 `json:"id"`
			SortOrder int   `json:"sort_order"`
		} `json:"orders"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	orders := make(map[int64]int, len(req.Orders))
	for _, o := range req.Orders {
		orders[o.ID] = o.SortOrder
	}

	if err := h.svc.Reorder(c.Request.Context(), serverID, orders); err != nil {
		response.InternalError(c, "REORDER_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	coreType := c.Query("core_type")
	if coreType == "" {
		coreType = "xray"
	}
	if h.shouldSyncOutbound(serverID, coreType) {
		h.triggerConfigSync(serverID, coreType)
	}

	response.OK(c, nil)
}
