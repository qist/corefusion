package outboundconfig

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/outboundconfig"
)

// ConfigSyncHook triggers config sync to a specific server after data changes.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
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
	if h.syncHook != nil && serverID > 1 {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, req.CoreType)
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
	if h.syncHook != nil && serverID > 1 {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, cfg.CoreType)
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
	if h.syncHook != nil && serverID > 1 {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, coreType)
	}

	response.OK(c, nil)
}
