package nodeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/service/audit"
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
		audit.RecordNodeFromCtx(c, serverID, "save_protocol", coreType, false, err.Error())
		response.InternalError(c, "SAVE_PROTOCOL_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, serverID, "save_protocol", fmt.Sprintf("saved %s config", coreType), true, "")

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
		audit.RecordNodeFromCtx(c, serverID, "save_protocol_section", fmt.Sprintf("%s/%s", coreType, section), false, err.Error())
		response.InternalError(c, "SAVE_SECTION_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, serverID, "save_protocol_section", fmt.Sprintf("saved %s/%s", coreType, section), true, "")

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

// GetConfigFile handles GET /api/v1/node-servers/:id/protocols/:coreType/config-file
// Reads the actual config file from the target node (local or remote).
// For local nodes, reads from disk via the LocalAgent.
// For remote nodes, calls the ReadConfigFile gRPC RPC on the Agent.
// If reading from the node fails (e.g. file not found, agent not connected),
// falls back to generating config from the database.
func (h *Handler) GetConfigFile(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server ID")
		return
	}
	coreType := c.Param("coreType")
	if coreType == "" {
		coreType = "xray"
	}

	// WAF rule files are read via the node operator (local disk or remote gRPC),
	// not through the core config generator. coreType format: "waf_rule:<name>".
	if strings.HasPrefix(coreType, "waf_rule:") {
		ruleName := strings.TrimPrefix(coreType, "waf_rule:")
		if h.svc == nil {
			response.InternalError(c, "NOT_CONFIGURED", "node service not configured")
			return
		}
		data, _, err := h.svc.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_rule:"+ruleName)
		if err != nil {
			response.NotFound(c, "WAF_RULE_NOT_FOUND", err.Error())
			return
		}
		c.JSON(200, gin.H{"path": "waf-rules/" + ruleName, "content": string(data)})
		return
	}

	if !isValidCoreType(coreType) {
		response.BadRequest(c, "INVALID_CORE_TYPE", "unsupported core type: "+coreType)
		return
	}

	// Try to read the actual config file from the node (local or remote via gRPC)
	data, _, err := h.svc.ReadConfigFile(c.Request.Context(), serverID, coreType)
	if err != nil {
		// If reading from node fails, fall back to generating config from database
		log.Printf("[nodeserver] GetConfigFile: ReadConfigFile failed for server %d (%s): %v, falling back to DB generation", serverID, coreType, err)

		if h.coreConfigSvc == nil {
			response.InternalError(c, "NOT_CONFIGURED", "core config service not configured")
			return
		}
		fullConfig, genErr := h.coreConfigSvc.GetFull(c.Request.Context(), serverID, coreType)
		if genErr != nil {
			response.NotFound(c, "PROTOCOL_NOT_FOUND", genErr.Error())
			return
		}
		data, err = coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
			Ctx:              c.Request.Context(),
			ServerID:         serverID,
			CoreType:         coreType,
			FullConfig:       fullConfig,
			InboundProvider:  h.inboundConfigSvc,
			OutboundProvider: h.outboundConfigSvc,
			EndpointProvider: h.endpointConfigSvc,
		})
		if err != nil {
			response.InternalError(c, "GENERATE_FAILED", err.Error())
			return
		}
	}
	c.Data(200, "application/json", data)
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
	// Include guardian settings from the panel DB for remote nodes.
	resp := transformProxyStatus(result, h.svc, serverID)
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
		audit.RecordNodeFromCtx(c, serverID, "control_proxy", fmt.Sprintf("%s %s", req.Action, req.ProxyType), false, err.Error())
		response.InternalError(c, "PROXY_CONTROL_FAILED", err.Error())
		return
	}
	audit.RecordNodeFromCtx(c, serverID, "control_proxy", fmt.Sprintf("%s %s", req.Action, req.ProxyType), true, "")
	response.OK(c, gin.H{"controlled": true, "server_id": serverID, "proxy_type": req.ProxyType, "action": req.Action})
}

// SyncProxyConfig handles POST /api/v1/node-servers/:id/proxy/sync
// Generates and syncs proxy config to a specific server.
// For remote servers, also triggers SyncData to land TLS certificates
// referenced by proxy sites (Caddy/Nginx tls directives) on the agent.
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
		audit.RecordNodeFromCtx(c, serverID, "sync_proxy", req.ProxyType, false, err.Error())
		response.InternalError(c, "PROXY_SYNC_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, serverID, "sync_proxy", fmt.Sprintf("synced %s config", req.ProxyType), true, "")

	// For remote servers, also trigger SyncData to land TLS certificates
	// that are referenced by the proxy config (Caddy/Nginx tls directives).
	// SyncData now includes certificates from proxy_sites/caddy_sites/nginx_sites
	// tables, ensuring cert files exist on the agent before the proxy starts.
	if !h.svc.IsLocalServer(c.Request.Context(), serverID) {
		go func() {
			ctx := context.Background()
			if _, err := h.svc.SyncServerData(ctx, serverID, false); err != nil {
				log.Printf("[nodeserver] SyncData for proxy certs on server %d: %v", serverID, err)
			} else {
				log.Printf("[nodeserver] SyncData for proxy certs on server %d: success", serverID)
			}
		}()
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

	// 2. Use the shared PrepareFullConfig to apply all standard steps
	data, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
		Ctx:              ctx,
		ServerID:         serverID,
		CoreType:         coreType,
		FullConfig:       fullConfig,
		InboundProvider:  h.inboundConfigSvc,
		OutboundProvider: h.outboundConfigSvc,
		EndpointProvider: h.endpointConfigSvc,
	})
	if err != nil {
		log.Printf("[nodeserver] syncConfigToServer: PrepareFullConfig failed for server %d: %v", serverID, err)
		return
	}

	// 3. Sync config file to target node (for persistence, no restart)
	if err := h.svc.SyncServerConfig(ctx, serverID, coreType, data, 0); err != nil {
		log.Printf("[nodeserver] sync config to server %d failed: %v", serverID, err)
	} else {
		log.Printf("[nodeserver] config file synced to server %d (%s)", serverID, coreType)
	}

	// 4. Sync to mirror nodes bound to this primary.
	// This ensures mirrors get the updated config at the same time as the
	// primary. SyncMirrorsForPrimary copies the primary's inbound configs
	// to each mirror's DB, then pushes the config to each mirror's Agent
	// and applies it immediately (hot-reload).
	h.svc.SyncMirrorsForPrimary(ctx, serverID)
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
// the frontend expects. It also includes guardian settings from the panel DB.
func transformProxyStatus(raw interface{}, svc proxyGuardianReader, serverID int64) gin.H {
	resp := gin.H{
		"active_proxy":   "",
		"caddy":          gin.H{"type": "caddy", "installed": false, "status": "unknown", "version": "", "path": "", "managed": false},
		"nginx":          gin.H{"type": "nginx", "installed": false, "status": "unknown", "version": "", "path": "", "managed": false},
		"panel_port":     "8080",
		"caddy_guardian": nil,
		"nginx_guardian": nil,
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

		// Use guardian status from agent if available
		for pt, statusJSON := range pb.GuardianStatus {
			var guardian map[string]interface{}
			if err := json.Unmarshal([]byte(statusJSON), &guardian); err == nil {
				if pt == "caddy" {
					resp["caddy_guardian"] = guardian
				} else if pt == "nginx" {
					resp["nginx_guardian"] = guardian
				}
			}
		}
	}

	// Fallback: if guardian status not available from agent, read from panel DB
	if resp["nginx_guardian"] == nil || resp["caddy_guardian"] == nil {
		if svc != nil {
			for _, pt := range []string{"caddy", "nginx"} {
				if (pt == "caddy" && resp["caddy_guardian"] != nil) || (pt == "nginx" && resp["nginx_guardian"] != nil) {
					continue
				}
				autoStart, autoRestart, maxRestarts := svc.GetProxyGuardianSettingsForServer(context.Background(), serverID, pt)
				guardian := gin.H{
					"proxy_type":       pt,
					"state":            "no_guardian",
					"auto_start":       autoStart,
					"auto_restart":     autoRestart,
					"max_restarts":     maxRestarts,
					"current_restarts": 0,
				}
				if pt == "caddy" {
					resp["caddy_guardian"] = guardian
				} else {
					resp["nginx_guardian"] = guardian
				}
			}
		}
	}

	return resp
}

// proxyGuardianReader is implemented by nodeserver.Service to read proxy
// guardian settings from the panel DB.
type proxyGuardianReader interface {
	GetProxyGuardianSettingsForServer(ctx context.Context, serverID int64, proxyType string) (autoStart, autoRestart bool, maxRestarts int)
}
