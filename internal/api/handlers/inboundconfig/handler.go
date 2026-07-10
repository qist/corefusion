package inboundconfig

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/inboundconfig"
)

// ConfigSyncHook triggers config sync to a specific server after data changes.
// Implemented by a closure that calls configsync.Service or nodeserver.Service.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// Handler handles inbound config HTTP requests.
type Handler struct {
	svc        *inboundconfig.Service
	syncHook   ConfigSyncHook
}

// NewHandler creates a new inbound config handler.
func NewHandler(svc *inboundconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// SetSyncHook sets the config sync hook for triggering remote sync after changes.
func (h *Handler) SetSyncHook(hook ConfigSyncHook) {
	h.syncHook = hook
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
	response.OK(c, configs)
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
	response.OK(c, cfg)
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
		Enabled    bool            `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Create(c.Request.Context(), serverID, req.CoreType, req.Remark, req.Protocol, req.Tag, req.Listen, req.Port, string(req.ConfigJSON), req.Enabled)
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
		Enabled    bool            `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg, err := h.svc.Update(c.Request.Context(), id, req.Remark, req.Protocol, req.Tag, req.Listen, req.Port, string(req.ConfigJSON), req.Enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	if h.syncHook != nil && cfg.ServerID > 1 {
		go h.syncHook.SyncServerConfig(context.Background(), cfg.ServerID, cfg.CoreType)
	}

	response.OK(c, cfg)
}

// Delete handles DELETE /api/v1/node-servers/:id/inbound-configs/:cfgId.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("cfgId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	// Get server_id before deletion for sync trigger
	serverID := int64(1)
	coreType := "xray"
	if existing, err := h.svc.GetByID(c.Request.Context(), id); err == nil {
		serverID = existing.ServerID
		coreType = existing.CoreType
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}

	// Trigger config sync to the target server asynchronously
	if h.syncHook != nil && serverID > 1 {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, coreType)
	}

	response.OK(c, gin.H{"message": "deleted"})
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

	// Trigger config sync to the target server asynchronously
	if h.syncHook != nil && serverID > 1 {
		go h.syncHook.SyncServerConfig(context.Background(), serverID, "xray")
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

	// Find user in settings.users or settings.clients
	settings, ok := config["settings"].(map[string]interface{})
	if !ok {
		response.BadRequest(c, "INVALID_CONFIG", "no settings found")
		return
	}

	found := false
	for _, key := range []string{"users", "clients"} {
		users, ok := settings[key].([]interface{})
		if !ok {
			continue
		}
		for _, u := range users {
			user, ok := u.(map[string]interface{})
			if !ok {
				continue
			}
		 userEmail, _ := user["email"].(string)
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
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	if paused {
		response.OK(c, gin.H{"message": "user paused", "email": email})
	} else {
		response.OK(c, gin.H{"message": "user resumed", "email": email})
	}
}
