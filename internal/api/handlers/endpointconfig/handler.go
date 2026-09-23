package endpointconfig

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/endpointconfig"
)

// ConfigSyncHook triggers config sync to a specific server after data changes.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// Handler handles endpoint config HTTP requests.
type Handler struct {
	svc      *endpointconfig.Service
	syncHook ConfigSyncHook
}

// NewHandler creates a new endpoint config handler.
func NewHandler(svc *endpointconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// SetSyncHook sets the config sync hook for triggering remote sync after changes.
func (h *Handler) SetSyncHook(hook ConfigSyncHook) {
	h.syncHook = hook
}

// List handles GET /api/v1/node-servers/:id/endpoint-configs.
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

// Get handles GET /api/v1/node-servers/:id/endpoint-configs/:cfgId.
func (h *Handler) Get(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		cfgID = 0
	}
	cfg, err := h.svc.GetByID(c.Request.Context(), cfgID, serverID)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, cfg)
}

// Create handles POST /api/v1/node-servers/:id/endpoint-configs.
func (h *Handler) Create(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	var req struct {
		CoreType     string          `json:"core_type"`
		EndpointType string          `json:"endpoint_type"`
		Remark       string          `json:"remark"`
		Tag          string          `json:"tag"`
		ConfigJSON   json.RawMessage `json:"config_json"`
		Enabled      bool            `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Create(c.Request.Context(), serverID, req.CoreType, req.EndpointType, req.Tag, string(req.ConfigJSON), req.Enabled)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	// ⚠ 2026-09-23：去掉 `serverID > 1` —— 原来在**本地节点（serverID=1）**改端点
	// 不会触发同步 ✗，端点只躺在 endpoint_configs 表里、运行时不生效，
	// 表现为"加了 WireGuard/WARP 端点却看不到" ✗（与 inbound 那次修复同一个毛病）。
	if h.syncHook != nil {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, req.CoreType)
	}

	response.OK(c, cfg)
}

// Update handles PUT /api/v1/node-servers/:id/endpoint-configs/:cfgId.
func (h *Handler) Update(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config ID")
		return
	}
	var req struct {
		EndpointType string          `json:"endpoint_type"`
		Remark       string          `json:"remark"`
		Tag          string          `json:"tag"`
		ConfigJSON   json.RawMessage `json:"config_json"`
		Enabled      bool            `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Update(c.Request.Context(), cfgID, serverID, req.EndpointType, req.Tag, string(req.ConfigJSON), req.Enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	// ⚠ 2026-09-23：去掉 `serverID > 1` —— 原来在**本地节点（serverID=1）**改端点
	// 不会触发同步 ✗，端点只躺在 endpoint_configs 表里、运行时不生效，
	// 表现为"加了 WireGuard/WARP 端点却看不到" ✗（与 inbound 那次修复同一个毛病）。
	if h.syncHook != nil {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, "singbox")
	}

	response.OK(c, cfg)
}

// Delete handles DELETE /api/v1/node-servers/:id/endpoint-configs/:cfgId.
func (h *Handler) Delete(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		serverID = 1
	}
	cfgID, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid config ID")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), cfgID, serverID); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	// ⚠ 2026-09-23：去掉 `serverID > 1` —— 原来在**本地节点（serverID=1）**改端点
	// 不会触发同步 ✗，端点只躺在 endpoint_configs 表里、运行时不生效，
	// 表现为"加了 WireGuard/WARP 端点却看不到" ✗（与 inbound 那次修复同一个毛病）。
	if h.syncHook != nil {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, "singbox")
	}

	response.OK(c, gin.H{"deleted": true})
}
