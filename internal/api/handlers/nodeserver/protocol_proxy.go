package nodeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	agentpb "corefusion/internal/runtime/agent/proto"
	coreconfigSvc "corefusion/internal/service/coreconfig"
)

// ──────────────────── Protocol Config Management ────────────────────

// ListProtocols handles GET /api/v1/node-servers/:id/protocols
// Returns all core types with their section list for a specific server.
func (h *Handler) ListProtocols(c *gin.Context) {
	if h.coreConfigSvc == nil {
		response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
		return
	}
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	result, err := h.coreConfigSvc.List(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_PROTOCOLS_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

// GetProtocol handles GET /api/v1/node-servers/:id/protocols/:coreType
// Returns all sections merged into one JSON object for a specific server.
func (h *Handler) GetProtocol(c *gin.Context) {
	if h.coreConfigSvc == nil {
		response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
		return
	}
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	full, err := h.coreConfigSvc.GetFull(c.Request.Context(), serverID, coreType)
	if err != nil {
		response.NotFound(c, "PROTOCOL_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, full)
}

// SaveProtocol handles PUT /api/v1/node-servers/:id/protocols/:coreType
// Saves config to panel DB, then syncs to the target node.
func (h *Handler) SaveProtocol(c *gin.Context) {
	if h.coreConfigSvc == nil {
		response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
		return
	}
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "read body failed")
		return
	}

	stripped := coreconfigSvc.StripJSONC(raw)
	var body map[string]interface{}
	if err := json.Unmarshal(stripped, &body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.coreConfigSvc.SaveFull(c.Request.Context(), serverID, coreType, body); err != nil {
		response.InternalError(c, "SAVE_PROTOCOL_FAILED", err.Error())
		return
	}

	// Sync to target node asynchronously
	go h.syncConfigToServer(serverID, coreType)

	response.OK(c, gin.H{"message": "saved", "server_id": serverID})
}

// GetProtocolSection handles GET /api/v1/node-servers/:id/protocols/:coreType/section/:section
func (h *Handler) GetProtocolSection(c *gin.Context) {
	if h.coreConfigSvc == nil {
		response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
		return
	}
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Param("coreType")
	section := c.Param("section")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	val, err := h.coreConfigSvc.GetSection(c.Request.Context(), serverID, coreType, section)
	if err != nil {
		response.NotFound(c, "SECTION_NOT_FOUND", err.Error())
		return
	}
	response.OK(c, val)
}

// SaveProtocolSection handles PUT /api/v1/node-servers/:id/protocols/:coreType/section/:section
// Saves section to panel DB, then syncs full config to the target node.
func (h *Handler) SaveProtocolSection(c *gin.Context) {
	if h.coreConfigSvc == nil {
		response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
		return
	}
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Param("coreType")
	section := c.Param("section")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "read body failed")
		return
	}

	stripped := coreconfigSvc.StripJSONC(raw)
	var body json.RawMessage
	if err := json.Unmarshal(stripped, &body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.coreConfigSvc.SaveSection(c.Request.Context(), serverID, coreType, section, body); err != nil {
		response.InternalError(c, "SAVE_SECTION_FAILED", err.Error())
		return
	}

	// Sync to target node asynchronously
	go h.syncConfigToServer(serverID, coreType)

	response.OK(c, gin.H{"message": "saved", "server_id": serverID})
}

// ValidateProtocol handles POST /api/v1/node-servers/:id/protocols/:coreType/validate
func (h *Handler) ValidateProtocol(c *gin.Context) {
	if h.coreConfigSvc == nil {
		response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
		return
	}
	coreType := c.Param("coreType")
	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "INVALID_REQUEST", "read body failed")
		return
	}

	stripped := coreconfigSvc.StripJSONC(raw)
	var body struct {
		ConfigJSON string `json:"config_json"`
	}
	if err := json.Unmarshal(stripped, &body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.coreConfigSvc.Validate(body.ConfigJSON); err != nil {
		response.BadRequest(c, "VALIDATION_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"valid": true})
}

// ──────────────────── Proxy Management ────────────────────

// GetProxyStatus handles GET /api/v1/node-servers/:id/proxy/status
// For local nodes, returns detailed proxy status (via proxyH.GetStatus).
// For remote nodes, queries the Agent via gRPC and transforms the response
// into the same { caddy, nginx, active_proxy } format the frontend expects.
func (h *Handler) GetProxyStatus(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}

	// For local nodes, delegate to the proxy handler
	if h.proxyH != nil && h.svc.IsLocalServer(c.Request.Context(), serverID) {
		h.proxyH.GetStatus(c)
		return
	}

	// For remote nodes, query proxy status via gRPC
	result, err := h.svc.GetProxyStatusRemote(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "PROXY_STATUS_FAILED", err.Error())
		return
	}

	// Transform the gRPC response into the format expected by the frontend.
	// The gRPC GetProxyStatusResponse has a `proxies` array; we need to
	// convert it to { caddy: {...}, nginx: {...}, active_proxy: "..." }.
	resp := transformProxyStatus(result)
	response.OK(c, resp)
}

// ControlProxy handles POST /api/v1/node-servers/:id/proxy/control
// Controls a proxy process (caddy/nginx) on a specific server.
func (h *Handler) ControlProxy(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		ProxyType string `json:"proxy_type" binding:"required"` // caddy or nginx
		Action    string `json:"action" binding:"required"`     // start, stop, restart, reload
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if err := h.svc.ControlProxy(c.Request.Context(), serverID, req.ProxyType, req.Action); err != nil {
		response.InternalError(c, "PROXY_CONTROL_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"controlled": true, "server_id": serverID, "proxy_type": req.ProxyType, "action": req.Action})
}

// SyncProxyConfig handles POST /api/v1/node-servers/:id/proxy/sync
// Generates and syncs proxy config to a specific server.
func (h *Handler) SyncProxyConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	var req struct {
		ProxyType string `json:"proxy_type" binding:"required"`
		Config    string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_BODY", err.Error())
		return
	}

	if err := h.svc.SyncProxyConfig(c.Request.Context(), serverID, req.ProxyType, []byte(req.Config)); err != nil {
		response.InternalError(c, "PROXY_SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"synced": true, "server_id": serverID, "proxy_type": req.ProxyType})
}

// ──────────────────── Internal helpers ────────────────────

// syncConfigToServer generates the full config and syncs it to the target node.
// This runs asynchronously to avoid blocking the HTTP response.
func (h *Handler) syncConfigToServer(serverID int64, coreType string) {
	if h.coreConfigSvc == nil {
		return
	}
	ctx := context.Background()

	// Generate full config (same logic as configsync)
	fullConfig, err := h.coreConfigSvc.GetFull(ctx, serverID, coreType)
	if err != nil {
		return
	}

	// Clean empty fields
	cleanEmptyMap(fullConfig)

	// Remove outbounds from core_configs (migrated to outbound_configs table)
	delete(fullConfig, "outbounds")

	// Ensure stats is present
	if _, exists := fullConfig["stats"]; !exists {
		fullConfig["stats"] = map[string]interface{}{}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return
	}

	// Sync to target node
	if err := h.svc.SyncServerConfig(ctx, serverID, coreType, data, 0); err != nil {
		fmt.Printf("[nodeserver] sync config to server %d failed: %v\n", serverID, err)
	}
}

// cleanEmptyMap removes nil values and empty maps recursively.
func cleanEmptyMap(m map[string]interface{}) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		if sub, ok := v.(map[string]interface{}); ok {
			cleanEmptyMap(sub)
			if len(sub) == 0 {
				delete(m, k)
			}
		}
	}
}

func isValidCoreType(ct string) bool {
	for _, t := range coreconfigSvc.SupportedCoreTypes {
		if t == ct {
			return true
		}
	}
	return false
}

// transformProxyStatus converts a gRPC GetProxyStatusResponse (which has a
// `proxies` array) into the { caddy, nginx, active_proxy } map format that
// the frontend (CaddyProxy.vue / NginxProxy.vue) expects.
func transformProxyStatus(raw interface{}) gin.H {
	resp := gin.H{
		"active_proxy":    "",
		"caddy":           gin.H{"type": "caddy", "installed": false, "status": "unknown", "version": "", "path": "", "managed": false},
		"nginx":           gin.H{"type": "nginx", "installed": false, "status": "unknown", "version": "", "path": "", "managed": false},
		"panel_port":      "8080",
		"caddy_guardian":  nil,
		"nginx_guardian":  nil,
	}

	if pb, ok := raw.(*agentpb.GetProxyStatusResponse); ok && pb != nil {
		active := ""
		for _, p := range pb.Proxies {
			entry := gin.H{
				"type":      p.Type,
				"name":      p.Name,
				"installed": p.Installed,
				"status":    p.Status,
				"version":   p.Version,
				"path":      p.Path,
				"managed":   p.Managed,
			}
			switch p.Type {
			case "caddy":
				resp["caddy"] = entry
				if p.Status == "running" {
					active = "caddy"
				}
			case "nginx":
				resp["nginx"] = entry
				if p.Status == "running" && active == "" {
					active = "nginx"
				}
			}
		}
		resp["active_proxy"] = active
	}

	return resp
}
