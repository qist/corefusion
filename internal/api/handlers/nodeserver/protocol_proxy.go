package nodeserver

import (
	"context"
	"encoding/json"
	"io"
	"log"
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

// syncConfigToServer generates the full config (including inbounds/outbounds)
// and syncs it to the target node's config file for persistence.
// It does NOT restart the core — inbounds/outbounds/routing are hot-updated
// via Xray gRPC API by the frontend; other sections are restarted by the
// frontend's saveSection() if the core is running. Config file persistence
// is also handled by the 5-minute configsync timer.
func (h *Handler) syncConfigToServer(serverID int64, coreType string) {
	if h.coreConfigSvc == nil {
		return
	}
	ctx := context.Background()

	// 1. Generate full config from core_configs table
	fullConfig, err := h.coreConfigSvc.GetFull(ctx, serverID, coreType)
	if err != nil {
		log.Printf("[nodeserver] syncConfigToServer: GetFull failed for server %d: %v", serverID, err)
		return
	}

	// 2. Clean empty fields
	cleanEmptyMap(fullConfig)

	// 3. Remove outbounds from core_configs (migrated to outbound_configs table)
	delete(fullConfig, "outbounds")

	// 4. Ensure stats is present
	if _, exists := fullConfig["stats"]; !exists {
		fullConfig["stats"] = map[string]interface{}{}
	}

	// 5. Handle API inbound/routing (same logic as configsync)
	if coreType == "xray" {
		applyAPIInboundRouting(fullConfig)
	}

	// 6. Merge enabled inbounds from inbound_configs table
	if h.inboundConfigSvc != nil {
		enabledInbounds, err := h.inboundConfigSvc.GetEnabledInbounds(ctx, serverID, coreType)
		if err != nil {
			log.Printf("[nodeserver] syncConfigToServer: failed to load inbounds for server %d: %v", serverID, err)
		} else if len(enabledInbounds) > 0 {
			existingInbounds, _ := fullConfig["inbounds"].([]interface{})
			for _, ib := range enabledInbounds {
				var inbound map[string]interface{}
				if json.Unmarshal(ib, &inbound) == nil {
					existingInbounds = append(existingInbounds, inbound)
				}
			}
			fullConfig["inbounds"] = existingInbounds
		}
	}

	// 7. Merge enabled outbounds from outbound_configs table
	if h.outboundConfigSvc != nil {
		enabledOutbounds, err := h.outboundConfigSvc.GetEnabledOutbounds(ctx, serverID, coreType)
		if err != nil {
			log.Printf("[nodeserver] syncConfigToServer: failed to load outbounds for server %d: %v", serverID, err)
		} else if len(enabledOutbounds) > 0 {
			fullConfig["outbounds"] = enabledOutbounds
		}
	}

	// 8. Marshal to JSON
	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		log.Printf("[nodeserver] syncConfigToServer: marshal failed for server %d: %v", serverID, err)
		return
	}

	// 9. Sync config file to target node (for persistence, no restart)
	if err := h.svc.SyncServerConfig(ctx, serverID, coreType, data, 0); err != nil {
		log.Printf("[nodeserver] sync config to server %d failed: %v", serverID, err)
	} else {
		log.Printf("[nodeserver] config file synced to server %d (%s)", serverID, coreType)
	}
}

// applyAPIInboundRouting adds API inbound (dokodemo-door) and routing rule
// if api.listen is not set. Mirrors the logic in configsync.syncCore.
func applyAPIInboundRouting(fullConfig map[string]interface{}) {
	apiRaw, ok := fullConfig["api"]
	if !ok {
		return
	}
	apiMap, ok := apiRaw.(map[string]interface{})
	if !ok {
		return
	}

	apiMap["services"] = []string{"HandlerService", "LoggerService", "StatsService", "RoutingService"}

	listen, _ := apiMap["listen"].(string)
	tag, _ := apiMap["tag"].(string)
	if tag == "" {
		tag = "api"
	}

	if listen != "" {
		return // listen is set, use it directly
	}

	delete(apiMap, "listen")

	// Add API inbound (dokodemo-door) if not exists
	inbounds, _ := fullConfig["inbounds"].([]interface{})
	hasAPIInbound := false
	for _, ib := range inbounds {
		if ibMap, ok := ib.(map[string]interface{}); ok {
			if ibMap["tag"] == tag {
				hasAPIInbound = true
				break
			}
		}
	}
	if !hasAPIInbound {
		apiInbound := map[string]interface{}{
			"tag":      tag,
			"listen":   "127.0.0.1",
			"port":     10085,
			"protocol": "dokodemo-door",
			"settings": map[string]interface{}{"address": "127.0.0.1"},
		}
		fullConfig["inbounds"] = append(inbounds, apiInbound)
	}

	// Add routing rule for API if not exists
	routing, _ := fullConfig["routing"].(map[string]interface{})
	if routing == nil {
		routing = map[string]interface{}{"domainStrategy": "IPIfNonMatch"}
	}
	rules, _ := routing["rules"].([]interface{})
	hasAPIRule := false
	for _, r := range rules {
		if rm, ok := r.(map[string]interface{}); ok {
			if tags, ok := rm["inboundTag"].([]interface{}); ok {
				for _, t := range tags {
					if t == tag {
						hasAPIRule = true
						break
					}
				}
			}
		}
	}
	if !hasAPIRule {
		apiRule := map[string]interface{}{
			"type":        "field",
			"inboundTag":  []string{tag},
			"outboundTag": tag,
		}
		routing["rules"] = append([]interface{}{apiRule}, rules...)
		fullConfig["routing"] = routing
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
