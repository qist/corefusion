package system

import (
	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/proxy"
	"github.com/gin-gonic/gin"
	"strconv"
)

// CaddyHandler handles Caddy-specific HTTP requests.
type CaddyHandler struct {
	caddyMgr *proxy.CaddyManager
	proxyMgr *proxy.Manager
	nodeOp   NodeOperator
}

// NewCaddyHandler creates a new Caddy handler.
func NewCaddyHandler(caddyMgr *proxy.CaddyManager, proxyMgr *proxy.Manager) *CaddyHandler {
	return &CaddyHandler{caddyMgr: caddyMgr, proxyMgr: proxyMgr}
}

// SetNodeOperator sets the node operator for remote proxy operations.
func (h *CaddyHandler) SetNodeOperator(op NodeOperator) {
	h.nodeOp = op
}

// applyServerID extracts the server ID from the URL and sets it on the managers
// so that site queries filter by the correct node.
func (h *CaddyHandler) applyServerID(c *gin.Context) (int64, bool) {
	serverID := parseServerID(c)
	isLocal := serverID == localServerID || serverID == 0
	if !isLocal && h.nodeOp != nil {
		isLocal = h.nodeOp.IsLocalServer(c.Request.Context(), serverID)
	}
	sid := serverID
	if sid == 0 {
		sid = localServerID
	}
	h.caddyMgr.SetServerID(sid)
	h.proxyMgr.SetServerID(sid)
	h.caddyMgr.SetIsLocal(isLocal)
	h.proxyMgr.SetIsLocal(isLocal)
	return sid, isLocal
}

// --- Site Management ---
func (h *CaddyHandler) ListSites(c *gin.Context) {
	h.applyServerID(c)
	sites, err := h.caddyMgr.ListSites(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if sites == nil {
		sites = []entity.CaddySite{}
	}
	response.OK(c, sites)
}
func (h *CaddyHandler) CreateSite(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		Domain        string `json:"domain" binding:"required"`
		Upstream      string `json:"upstream"`
		Template      string `json:"template"`
		Path          string `json:"path"`
		CertPath      string `json:"cert_path"`
		KeyPath       string `json:"key_path"`
		ProxyConfig   string `json:"proxy_config"`
		CustomConfig  string `json:"custom_config"`
		WafEnabled    *bool  `json:"waf_enabled"`
		LbPolicy      string `json:"lb_policy"`
		LogEnabled    bool   `json:"log_enabled"`
		LogPath       string `json:"log_path"`
		LogFormat     string `json:"log_format"`
		L4Protocol    string `json:"l4_protocol"`
		SNITarget     string `json:"sni_target"`
		UpstreamTLS   bool   `json:"upstream_tls"`
		LocalPort     string `json:"local_port"`
		ProxyProtocol bool   `json:"proxy_protocol"`
		IsPanel       bool   `json:"is_panel"`
		Priority      int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// Auto-generate log path from domain (without port) if not provided
	logPath := req.LogPath
	if logPath == "" && req.LogEnabled {
		logPath = proxy.SanitizeLogName(proxy.StripPort(req.Domain))
		if logPath == "default" {
			logPath = "logs/default.log"
		} else {
			logPath = "logs/" + logPath + ".log"
		}
	}
	site := &entity.CaddySite{
		Domain:        req.Domain,
		Upstream:      req.Upstream,
		Template:      req.Template,
		Path:          req.Path,
		CertPath:      req.CertPath,
		KeyPath:       req.KeyPath,
		ProxyConfig:   req.ProxyConfig,
		CustomConfig:  req.CustomConfig,
		WafEnabled:    req.WafEnabled,
		LbPolicy:      req.LbPolicy,
		LogEnabled:    req.LogEnabled,
		LogPath:       logPath,
		LogFormat:     req.LogFormat,
		L4Protocol:    req.L4Protocol,
		SNITarget:     req.SNITarget,
		UpstreamTLS:   req.UpstreamTLS,
		LocalPort:     req.LocalPort,
		ProxyProtocol: req.ProxyProtocol,
		Enabled:       true,
		IsPanel:       req.IsPanel,
		Priority:      req.Priority,
	}
	if err := h.caddyMgr.CreateSite(c.Request.Context(), site); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, site)
}
func (h *CaddyHandler) UpdateSite(c *gin.Context) {
	h.applyServerID(c)
	id, err := strconv.ParseInt(c.Param("siteId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	existing, err := h.caddyMgr.GetSite(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "site not found")
		return
	}
	var req struct {
		Domain        *string `json:"domain"`
		Upstream      *string `json:"upstream"`
		Template      *string `json:"template"`
		Path          *string `json:"path"`
		CertPath      *string `json:"cert_path"`
		KeyPath       *string `json:"key_path"`
		ProxyConfig   *string `json:"proxy_config"`
		CustomConfig  *string `json:"custom_config"`
		WafEnabled    *bool   `json:"waf_enabled"`
		LbPolicy      *string `json:"lb_policy"`
		LogEnabled    *bool   `json:"log_enabled"`
		LogPath       *string `json:"log_path"`
		LogFormat     *string `json:"log_format"`
		L4Protocol    *string `json:"l4_protocol"`
		SNITarget     *string `json:"sni_target"`
		UpstreamTLS   *bool   `json:"upstream_tls"`
		LocalPort     *string `json:"local_port"`
		ProxyProtocol *bool   `json:"proxy_protocol"`
		Enabled       *bool   `json:"enabled"`
		IsPanel       *bool   `json:"is_panel"`
		Priority      *int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Domain != nil {
		existing.Domain = *req.Domain
	}
	if req.Upstream != nil {
		existing.Upstream = *req.Upstream
	}
	if req.Template != nil {
		existing.Template = *req.Template
	}
	if req.Path != nil {
		existing.Path = *req.Path
	}
	if req.CertPath != nil {
		existing.CertPath = *req.CertPath
	}
	if req.KeyPath != nil {
		existing.KeyPath = *req.KeyPath
	}
	if req.ProxyConfig != nil {
		existing.ProxyConfig = *req.ProxyConfig
	}
	if req.CustomConfig != nil {
		existing.CustomConfig = *req.CustomConfig
	}
	if req.WafEnabled != nil {
		existing.WafEnabled = req.WafEnabled
	}
	if req.LbPolicy != nil {
		existing.LbPolicy = *req.LbPolicy
	}
	if req.LogEnabled != nil {
		existing.LogEnabled = *req.LogEnabled
	}
	if req.LogPath != nil {
		existing.LogPath = *req.LogPath
	} else if req.Domain != nil && existing.LogPath == "" && existing.LogEnabled {
		// Auto-generate log path from domain (without port) if not provided
		lp := proxy.SanitizeLogName(proxy.StripPort(*req.Domain))
		if lp == "default" {
			existing.LogPath = "logs/default.log"
		} else {
			existing.LogPath = "logs/" + lp + ".log"
		}
	}
	if req.LogFormat != nil {
		existing.LogFormat = *req.LogFormat
	}
	if req.L4Protocol != nil {
		existing.L4Protocol = *req.L4Protocol
	}
	if req.SNITarget != nil {
		existing.SNITarget = *req.SNITarget
	}
	// 回源 TLS ✓（grpc_pass grpc:// ↔ grpcs:// ✓）：显式传了才改 ✓，避免把已有设置冲掉 ✗
	if req.UpstreamTLS != nil {
		existing.UpstreamTLS = *req.UpstreamTLS
	}
	if req.LocalPort != nil {
		existing.LocalPort = *req.LocalPort
	}
	if req.ProxyProtocol != nil {
		existing.ProxyProtocol = *req.ProxyProtocol
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.IsPanel != nil {
		existing.IsPanel = *req.IsPanel
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if err := h.caddyMgr.UpdateSite(c.Request.Context(), existing); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, existing)
}
func (h *CaddyHandler) DeleteSite(c *gin.Context) {
	h.applyServerID(c)
	id, err := strconv.ParseInt(c.Param("siteId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.caddyMgr.DeleteSite(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// --- Config ---
func (h *CaddyHandler) GenerateConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	config, err := h.caddyMgr.GenerateConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	if isLocal {
		h.proxyMgr.WriteConfigFile(proxy.ProxyCaddy, config)
		// Push config to mirror nodes so they get the updated Caddyfile.
		// For local primary, the handler writes config directly to disk,
		// bypassing nodeOp — so we explicitly sync to mirrors here.
		if h.nodeOp != nil {
			h.nodeOp.SyncMirrorsProxyConfig(c.Request.Context(), serverID, "caddy", []byte(config))
		}
	} else if h.nodeOp != nil {
		if err := h.nodeOp.SyncProxyConfig(c.Request.Context(), serverID, "caddy", []byte(config)); err != nil {
			response.InternalError(c, "SYNC_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"config": config})
}
func (h *CaddyHandler) SaveAndApply(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	config, err := h.caddyMgr.SaveAndApply(c.Request.Context())
	if err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if isLocal {
		// Push config to mirror nodes so they get the updated Caddyfile.
		if h.nodeOp != nil {
			h.nodeOp.SyncMirrorsProxyConfig(c.Request.Context(), serverID, "caddy", []byte(config))
		}
	} else if h.nodeOp != nil {
		if err := h.nodeOp.SyncProxyConfig(c.Request.Context(), serverID, "caddy", []byte(config)); err != nil {
			response.InternalError(c, "SYNC_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"config": config})
}

// --- SNI ---
func (h *CaddyHandler) GetSNISettings(c *gin.Context) {
	h.applyServerID(c)
	settings, err := h.caddyMgr.GetSNISettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}
func (h *CaddyHandler) SaveSNISettings(c *gin.Context) {
	serverID, _ := h.applyServerID(c)
	var settings proxy.SNISettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.caddyMgr.SaveSNISettings(c.Request.Context(), &settings); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	// Sync DB changes to mirror nodes so their local DBs match the primary's SNI state
	if h.nodeOp != nil {
		h.nodeOp.SyncMirrorsData(c.Request.Context(), serverID)
	}
	response.OK(c, settings)
}
func (h *CaddyHandler) ToggleSNI(c *gin.Context) {
	serverID, _ := h.applyServerID(c)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.caddyMgr.ToggleSNIEnabled(c.Request.Context(), req.Enabled)
	if err != nil {
		response.InternalError(c, "TOGGLE_FAILED", err.Error())
		return
	}
	// Sync DB changes to mirror nodes so their local DBs match the primary's SNI state
	if h.nodeOp != nil {
		h.nodeOp.SyncMirrorsData(c.Request.Context(), serverID)
	}
	response.OK(c, gin.H{"updated": count})
}
func (h *CaddyHandler) DeleteAllSNI(c *gin.Context) {
	h.applyServerID(c)
	count, err := h.caddyMgr.DeleteAllSNI(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": count})
}
func (h *CaddyHandler) BatchUpdatePorts(c *gin.Context) {
	serverID, _ := h.applyServerID(c)
	var req struct {
		Port   string `json:"port" binding:"required"`
		Enable bool   `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.caddyMgr.BatchUpdatePorts(c.Request.Context(), req.Port, req.Enable)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	// Sync DB changes to mirror nodes so their local DBs match the primary's site ports
	if h.nodeOp != nil {
		h.nodeOp.SyncMirrorsData(c.Request.Context(), serverID)
	}
	response.OK(c, gin.H{"updated": count})
}

// BatchUpdateLogEnabled updates the log_enabled flag for all HTTP Caddy sites.
func (h *CaddyHandler) BatchUpdateLogEnabled(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.caddyMgr.BatchUpdateLogEnabled(c.Request.Context(), req.Enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"updated": count})
}

// --- L4 ---
func (h *CaddyHandler) DeleteAllL4(c *gin.Context) {
	h.applyServerID(c)
	count, err := h.caddyMgr.DeleteAllL4(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": count})
}
