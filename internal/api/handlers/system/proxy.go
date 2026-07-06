package system

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/proxy"
)

// ProxyHandler handles proxy management HTTP requests.
type ProxyHandler struct {
	proxyMgr *proxy.Manager
	caddyMgr *proxy.CaddyManager
	nginxMgr *proxy.NginxManager
	nodeOp   NodeOperator
}

// NewProxyHandler creates a new proxy handler.
func NewProxyHandler(proxyMgr *proxy.Manager, caddyMgr *proxy.CaddyManager, nginxMgr *proxy.NginxManager) *ProxyHandler {
	return &ProxyHandler{proxyMgr: proxyMgr, caddyMgr: caddyMgr, nginxMgr: nginxMgr}
}

// SetNodeOperator sets the node operator for remote proxy operations.
func (h *ProxyHandler) SetNodeOperator(op NodeOperator) {
	h.nodeOp = op
}

// applyServerID extracts the server ID from the URL and sets it on all managers
// so that site queries filter by the correct node. Returns the server ID and
// whether it is the local node.
func (h *ProxyHandler) applyServerID(c *gin.Context) (int64, bool) {
	serverID := parseServerID(c)
	isLocal := serverID == localServerID || serverID == 0
	if !isLocal && h.nodeOp != nil {
		isLocal = h.nodeOp.IsLocalServer(c.Request.Context(), serverID)
	}
	// Set serverID on all managers for per-node site filtering.
	// For local nodes this is always 1; for remote nodes it's the remote server's ID.
	sid := serverID
	if sid == 0 {
		sid = localServerID
	}
	h.proxyMgr.SetServerID(sid)
	h.caddyMgr.SetServerID(sid)
	h.nginxMgr.SetServerID(sid)
	return sid, isLocal
}

// parseServerID extracts the :id parameter from the URL and returns it.
// Returns 0 if the parameter is missing or invalid.
func parseServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// localServerID is the server ID of the local node.
const localServerID = int64(1)

// GetStatus handles GET /api/v1/proxy/status.
func (h *ProxyHandler) GetStatus(c *gin.Context) {
	nginx := h.proxyMgr.DetectProxy(proxy.ProxyNginx)
	caddy := h.proxyMgr.DetectProxy(proxy.ProxyCaddy)
	active := ""
	if nginx.Status == "running" {
		active = "nginx"
	} else if caddy.Status == "running" {
		active = "caddy"
	}
	// Read panel listen port from settings
	panelPort := "8080"
	if h.proxyMgr != nil {
		if port, err := h.proxyMgr.GetSetting(c.Request.Context(), "panel.listen_port"); err == nil && port != "" {
			panelPort = port
		}
	}
	// Guardian status
	nginxGuardian := h.proxyMgr.GetGuardianStatus(proxy.ProxyNginx)
	caddyGuardian := h.proxyMgr.GetGuardianStatus(proxy.ProxyCaddy)
	response.OK(c, gin.H{
		"active_proxy": active,
		"nginx":        nginx,
		"caddy":        caddy,
		"panel_port":   panelPort,
		"nginx_guardian": nginxGuardian,
		"caddy_guardian": caddyGuardian,
	})
}

// Install handles POST /api/v1/proxy/install.
func (h *ProxyHandler) Install(c *gin.Context) {
	var req struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	proxyType := proxy.ProxyType(req.Type)
	if err := h.proxyMgr.InstallProxy(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "INSTALL_FAILED", err.Error())
		return
	}
	nginx := h.proxyMgr.DetectProxy(proxy.ProxyNginx)
	caddy := h.proxyMgr.DetectProxy(proxy.ProxyCaddy)
	response.OK(c, gin.H{"nginx": nginx, "caddy": caddy})
}

// ListReleases handles GET /api/v1/proxy/releases.
func (h *ProxyHandler) ListReleases(c *gin.Context) {
	proxyType := c.Query("type")
	var repo string
	switch proxyType {
	case "caddy":
		repo = "qist/caddy-binaries"
	case "nginx":
		repo = "qist/nginx-binaries"
	default:
		response.BadRequest(c, "INVALID_TYPE", "unsupported proxy type")
		return
	}

	// Determine target server platform for asset filtering
	serverID, isLocal := h.applyServerID(c)
	var osName, archName string
	var distro string
	if isLocal || h.nodeOp == nil {
		osName, archName = runtime.GOOS, runtime.GOARCH
	} else {
		osName, archName = h.nodeOp.GetServerPlatform(c.Request.Context(), serverID)
		distro = h.nodeOp.GetServerDistro(c.Request.Context(), serverID)
	}

	var releases interface{}
	var err error
	if proxyType == "nginx" {
		releases, err = h.proxyMgr.ListNginxReleasesForPlatform(repo, osName, archName, distro)
	} else {
		releases, err = h.proxyMgr.ListReleasesForPlatform(repo, osName, archName)
	}
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "403") {
			response.InternalError(c, "RATE_LIMITED", "GitHub API rate limit exceeded. Set GITHUB_TOKEN env var.")
		} else {
			response.InternalError(c, "FETCH_FAILED", errMsg)
		}
		return
	}
	response.OK(c, releases)
}

// DownloadVersion handles POST /api/v1/node-servers/:id/proxy/download.
// For remote nodes, delegates to NodeOperator.InstallProxy via gRPC.
// For local nodes, downloads the asset and restarts the proxy if it was running.
func (h *ProxyHandler) DownloadVersion(c *gin.Context) {
	var req struct {
		Type        string `json:"type" binding:"required"`
		DownloadURL string `json:"download_url" binding:"required"`
		TagName     string `json:"tag_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	serverID, isLocal := h.applyServerID(c)

	// Remote node: delegate via gRPC to the Agent
	if !isLocal && h.nodeOp != nil {
		result, err := h.nodeOp.InstallProxy(c.Request.Context(), serverID, req.Type, req.DownloadURL, req.TagName)
		if err != nil {
			response.InternalError(c, "INSTALL_FAILED", err.Error())
			return
		}
		response.OK(c, result)
		return
	}

	// Local node: download the asset
	proxyType := proxy.ProxyType(req.Type)
	wasRunning := h.proxyMgr.DetectProxy(proxyType).Status == "running"

	path, err := h.proxyMgr.DownloadAsset(c.Request.Context(), req.DownloadURL, req.Type)
	if err != nil {
		response.InternalError(c, "DOWNLOAD_FAILED", err.Error())
		return
	}

	// If proxy was running, restart it to pick up the new binary
	if wasRunning {
		if err := h.proxyMgr.RestartProxy(c.Request.Context(), proxyType); err != nil {
			response.InternalError(c, "RESTART_FAILED", err.Error())
			return
		}
	}

	response.OK(c, gin.H{"path": path, "version": req.TagName, "type": req.Type, "restarted": wasRunning})
}

// Start handles POST /api/v1/node-servers/:id/proxy/start.
func (h *ProxyHandler) Start(c *gin.Context) {
	proxyType := proxy.ProxyType(c.Query("type"))
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		if err := h.nodeOp.ControlProxy(c.Request.Context(), serverID, string(proxyType), "start"); err != nil {
			response.InternalError(c, "START_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"message": "started"})
		return
	}
	if err := h.proxyMgr.StartProxy(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "START_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "started"})
}

// Stop handles POST /api/v1/node-servers/:id/proxy/stop.
func (h *ProxyHandler) Stop(c *gin.Context) {
	proxyType := proxy.ProxyType(c.Query("type"))
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		if err := h.nodeOp.ControlProxy(c.Request.Context(), serverID, string(proxyType), "stop"); err != nil {
			response.InternalError(c, "STOP_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"message": "stopped"})
		return
	}
	if err := h.proxyMgr.StopProxy(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "STOP_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "stopped"})
}

// Restart handles POST /api/v1/node-servers/:id/proxy/restart.
func (h *ProxyHandler) Restart(c *gin.Context) {
	proxyType := proxy.ProxyType(c.Query("type"))
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		if err := h.nodeOp.ControlProxy(c.Request.Context(), serverID, string(proxyType), "restart"); err != nil {
			response.InternalError(c, "RESTART_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"message": "restarted"})
		return
	}
	if err := h.proxyMgr.RestartProxy(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "RESTART_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "restarted"})
}

// Reload handles POST /api/v1/node-servers/:id/proxy/reload.
func (h *ProxyHandler) Reload(c *gin.Context) {
	proxyType := proxy.ProxyType(c.Query("type"))
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		if err := h.nodeOp.ControlProxy(c.Request.Context(), serverID, string(proxyType), "reload"); err != nil {
			response.InternalError(c, "RELOAD_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"message": "reloaded"})
		return
	}
	if err := h.proxyMgr.ReloadProxy(c.Request.Context(), proxyType); err != nil {
		response.InternalError(c, "RELOAD_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "reloaded"})
}

// --- Site Management ---

// ListSites handles GET /api/v1/node-servers/:id/proxy/sites.
func (h *ProxyHandler) ListSites(c *gin.Context) {
	h.applyServerID(c)
	proxyType := c.Query("type")
	var sites []entity.ProxySite
	var err error
	if proxyType != "" {
		sites, err = h.proxyMgr.ListSitesByType(c.Request.Context(), proxyType)
	} else {
		sites, err = h.proxyMgr.ListSites(c.Request.Context())
	}
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if sites == nil {
		sites = []entity.ProxySite{}
	}
	response.OK(c, sites)
}

// CreateSite handles POST /api/v1/node-servers/:id/proxy/sites.
func (h *ProxyHandler) CreateSite(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		ProxyType     string `json:"proxy_type"`
		Domain        string `json:"domain" binding:"required"`
		Upstream      string `json:"upstream"`
		Template      string `json:"template"`
		Path          string `json:"path"`
		ListenPort    string `json:"listen_port"`
		ListenSocket  string `json:"listen_socket"`
		CertPath      string `json:"cert_path"`
		KeyPath       string `json:"key_path"`
		CustomConfig  string `json:"custom_config"`
		ServerConfig  string `json:"server_config"`
		LocationConfig string `json:"location_config"`
		L4Protocol    string `json:"l4_protocol"`
		SNITarget     string `json:"sni_target"`
		LocalPort     string `json:"local_port"`
		ProxyProtocol bool   `json:"proxy_protocol"`
		IsPanel       bool   `json:"is_panel"`
		Priority      int    `json:"priority"`
		// TLS options
		EnableHttp2    bool   `json:"enable_http2"`
		EnableIpv6     bool   `json:"enable_ipv6"`
		EnableQuic     bool   `json:"enable_quic"`
		EnableEarlyData bool  `json:"enable_early_data"`
		EnableStapling bool   `json:"enable_stapling"`
		SslProtocols   string `json:"ssl_protocols"`
		SslCiphers     string `json:"ssl_ciphers"`
		SslEcdhCurve   string `json:"ssl_ecdh_curve"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	proxyType := req.ProxyType
	if proxyType == "" {
		proxyType = "caddy"
	}
	site := &entity.ProxySite{
		ProxyType:     proxyType,
		Domain:        req.Domain,
		Upstream:      req.Upstream,
		Template:      req.Template,
		Path:          req.Path,
		ListenPort:    req.ListenPort,
		ListenSocket:  req.ListenSocket,
		CertPath:      req.CertPath,
		EnableHttp2:   req.EnableHttp2,
		EnableIpv6:   req.EnableIpv6,
		EnableQuic:    req.EnableQuic,
		EnableEarlyData: req.EnableEarlyData,
		EnableStapling: req.EnableStapling,
		SslProtocols:  req.SslProtocols,
		SslCiphers:    req.SslCiphers,
		SslEcdhCurve:  req.SslEcdhCurve,
		KeyPath:       req.KeyPath,
		CustomConfig:   req.CustomConfig,
		ServerConfig:   req.ServerConfig,
		LocationConfig: req.LocationConfig,
		L4Protocol:     req.L4Protocol,
		SNITarget:     req.SNITarget,
		LocalPort:     req.LocalPort,
		ProxyProtocol: req.ProxyProtocol,
		Enabled:       true,
		IsPanel:       req.IsPanel,
		Priority:      req.Priority,
	}
	if err := h.proxyMgr.CreateSite(c.Request.Context(), site); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, site)
}

// UpdateSite handles PUT /api/v1/node-servers/:id/proxy/sites/:siteId.
func (h *ProxyHandler) UpdateSite(c *gin.Context) {
	h.applyServerID(c)
	id, err := strconv.ParseInt(c.Param("siteId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Domain        string  `json:"domain"`
		Upstream      string  `json:"upstream"`
		Template      string  `json:"template"`
		Path          string  `json:"path"`
		ListenPort    string  `json:"listen_port"`
		ListenSocket  string  `json:"listen_socket"`
		CertPath      *string `json:"cert_path"`
		KeyPath       *string `json:"key_path"`
		CustomConfig   *string `json:"custom_config"`
		ServerConfig   *string `json:"server_config"`
		LocationConfig *string `json:"location_config"`
		L4Protocol     string  `json:"l4_protocol"`
		SNITarget     string  `json:"sni_target"`
		LocalPort     string  `json:"local_port"`
		ProxyProtocol *bool   `json:"proxy_protocol"`
		Enabled       *bool   `json:"enabled"`
		IsPanel       *bool   `json:"is_panel"`
		Priority      *int    `json:"priority"`
		// TLS options
		EnableHttp2    *bool   `json:"enable_http2"`
		EnableIpv6     *bool   `json:"enable_ipv6"`
		EnableQuic     *bool   `json:"enable_quic"`
		EnableEarlyData *bool  `json:"enable_early_data"`
		EnableStapling *bool   `json:"enable_stapling"`
		SslProtocols   string  `json:"ssl_protocols"`
		SslCiphers     string  `json:"ssl_ciphers"`
		SslEcdhCurve   string  `json:"ssl_ecdh_curve"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// Get existing site
	existing, err := h.proxyMgr.GetSite(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "site not found")
		return
	}

	// Update fields
	if req.Domain != "" {
		existing.Domain = req.Domain
	}
	if req.Upstream != "" {
		existing.Upstream = req.Upstream
	}
	if req.Template != "" {
		existing.Template = req.Template
	}
	if req.Path != "" {
		existing.Path = req.Path
	}
	if req.ListenPort != "" {
		existing.ListenPort = req.ListenPort
	}
	if req.ListenSocket != "" {
		existing.ListenSocket = req.ListenSocket
	}
	if req.CertPath != nil {
		existing.CertPath = *req.CertPath
	}
	if req.KeyPath != nil {
		existing.KeyPath = *req.KeyPath
	}
	if req.CustomConfig != nil {
		existing.CustomConfig = *req.CustomConfig
	}
	if req.ServerConfig != nil {
		existing.ServerConfig = *req.ServerConfig
	}
	if req.LocationConfig != nil {
		existing.LocationConfig = *req.LocationConfig
	}
	if req.L4Protocol != "" {
		existing.L4Protocol = req.L4Protocol
	}
	if req.SNITarget != "" {
		existing.SNITarget = req.SNITarget
	}
	if req.LocalPort != "" {
		existing.LocalPort = req.LocalPort
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
	// TLS options
	if req.EnableHttp2 != nil {
		existing.EnableHttp2 = *req.EnableHttp2
	}
	if req.EnableIpv6 != nil {
		existing.EnableIpv6 = *req.EnableIpv6
	}
	if req.EnableQuic != nil {
		existing.EnableQuic = *req.EnableQuic
	}
	if req.EnableEarlyData != nil {
		existing.EnableEarlyData = *req.EnableEarlyData
	}
	if req.EnableStapling != nil {
		existing.EnableStapling = *req.EnableStapling
	}
	if req.SslProtocols != "" {
		existing.SslProtocols = req.SslProtocols
	}
	if req.SslCiphers != "" {
		existing.SslCiphers = req.SslCiphers
	}
	if req.SslEcdhCurve != "" {
		existing.SslEcdhCurve = req.SslEcdhCurve
	}

	if err := h.proxyMgr.UpdateSite(c.Request.Context(), existing); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, existing)
}

// DeleteSite handles DELETE /api/v1/node-servers/:id/proxy/sites/:siteId.
func (h *ProxyHandler) DeleteSite(c *gin.Context) {
	h.applyServerID(c)
	id, err := strconv.ParseInt(c.Param("siteId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.proxyMgr.DeleteSite(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// GenerateConfig handles POST /api/v1/node-servers/:id/proxy/generate-config.
// For local nodes, generates config and writes to local disk.
// For remote nodes, generates config and syncs to the remote Agent via gRPC.
func (h *ProxyHandler) GenerateConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	var req struct {
		Type string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	proxyType := proxy.ProxyType(req.Type)
	var config string
	var genErr error
	switch proxyType {
	case proxy.ProxyCaddy:
		config, genErr = h.caddyMgr.GenerateConfig(c.Request.Context())
	case proxy.ProxyNginx:
		config, genErr = h.nginxMgr.GenerateConfig(c.Request.Context())
	default:
		response.BadRequest(c, "UNSUPPORTED", "unsupported proxy type")
		return
	}
	if genErr != nil {
		response.InternalError(c, "GENERATE_FAILED", genErr.Error())
		return
	}

	if isLocal {
		h.proxyMgr.WriteConfigFile(proxyType, config)
	} else if h.nodeOp != nil {
		if err := h.nodeOp.SyncProxyConfig(c.Request.Context(), serverID, string(proxyType), []byte(config)); err != nil {
			response.InternalError(c, "SYNC_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"config": config})
}

// GetConfig handles GET /api/v1/proxy/config.
func (h *ProxyHandler) GetConfig(c *gin.Context) {
	proxyType := proxy.ProxyType(c.Query("type"))
	config, _ := h.proxyMgr.ReadConfigFile(proxyType)
	response.OK(c, gin.H{"config": config, "path": h.proxyMgr.GetConfigPath(proxyType)})
}

// UpdateConfig handles PUT /api/v1/node-servers/:id/proxy/config.
func (h *ProxyHandler) UpdateConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	var req struct {
		Type   string `json:"type" binding:"required"`
		Config string `json:"config" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	proxyType := proxy.ProxyType(req.Type)

	// Validate config
	if err := h.proxyMgr.ValidateConfig(proxyType, req.Config); err != nil {
		response.OK(c, gin.H{"error": err.Error()})
		return
	}

	// Save / sync config
	if isLocal {
		if err := h.proxyMgr.WriteConfigFile(proxyType, req.Config); err != nil {
			response.InternalError(c, "WRITE_FAILED", err.Error())
			return
		}
		// Auto-reload if proxy is running
		proxyInfo := h.proxyMgr.DetectProxy(proxyType)
		if proxyInfo.Status == "running" {
			if err := h.proxyMgr.ReloadProxy(c.Request.Context(), proxyType); err != nil {
				response.OK(c, gin.H{"message": "config saved", "reload_error": err.Error()})
				return
			}
			response.OK(c, gin.H{"message": "config saved and reloaded"})
			return
		}
	} else if h.nodeOp != nil {
		if err := h.nodeOp.SyncProxyConfig(c.Request.Context(), serverID, string(proxyType), []byte(req.Config)); err != nil {
			response.InternalError(c, "SYNC_FAILED", err.Error())
			return
		}
		// Reload remote proxy
		if err := h.nodeOp.ControlProxy(c.Request.Context(), serverID, string(proxyType), "reload"); err != nil {
			response.OK(c, gin.H{"message": "config synced", "reload_error": err.Error()})
			return
		}
		response.OK(c, gin.H{"message": "config synced and reloaded"})
		return
	}

	response.OK(c, gin.H{"message": "config saved"})
}

// GetSNISettings handles GET /api/v1/proxy/sni-settings.
func (h *ProxyHandler) GetSNISettings(c *gin.Context) {
	settings, err := h.proxyMgr.GetSNISettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}

// SaveSNISettings handles POST /api/v1/proxy/sni-settings.
func (h *ProxyHandler) SaveSNISettings(c *gin.Context) {
	var req struct {
		Enabled    bool   `json:"enabled"`
		ListenPort string `json:"listen_port"`
		LocalPort  string `json:"local_port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	settings := &proxy.SNISettings{
		Enabled:    req.Enabled,
		ListenPort: req.ListenPort,
		LocalPort:  req.LocalPort,
	}
	if err := h.proxyMgr.SaveSNISettings(c.Request.Context(), settings); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}

// GetLogSettings handles GET /api/v1/proxy/log-settings.
func (h *ProxyHandler) GetLogSettings(c *gin.Context) {
	output := "stdout"
	path := ""
	if v, err := h.proxyMgr.GetSetting(c.Request.Context(), "proxy.log_output"); err == nil && v != "" {
		output = v
	}
	if v, err := h.proxyMgr.GetSetting(c.Request.Context(), "proxy.log_path"); err == nil && v != "" {
		path = v
	}
	response.OK(c, gin.H{"output": output, "path": path})
}

// GetNginxSettings handles GET /api/v1/proxy/nginx-settings.
func (h *ProxyHandler) GetNginxSettings(c *gin.Context) {
	settings, err := h.nginxMgr.GetNginxSettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}

// SaveNginxSettings handles POST /api/v1/proxy/nginx-settings.
func (h *ProxyHandler) SaveNginxSettings(c *gin.Context) {
	var settings proxy.NginxSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.nginxMgr.SaveNginxSettings(c.Request.Context(), &settings); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}

// SaveLogSettings handles POST /api/v1/proxy/log-settings.
func (h *ProxyHandler) SaveLogSettings(c *gin.Context) {
	var req struct {
		Output string `json:"output"`
		Path   string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	h.proxyMgr.SetSetting(c.Request.Context(), "proxy.log_output", req.Output)
	h.proxyMgr.SetSetting(c.Request.Context(), "proxy.log_path", req.Path)
	response.OK(c, gin.H{"output": req.Output, "path": req.Path})
}

// BatchUpdatePorts handles POST /api/v1/node-servers/:id/proxy/batch-update-ports.
func (h *ProxyHandler) BatchUpdatePorts(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		Port   string `json:"port" binding:"required"`
		Enable bool   `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.proxyMgr.BatchUpdatePorts(c.Request.Context(), req.Port, req.Enable)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"updated": count})
}

// DeleteAllSNI handles DELETE /api/v1/node-servers/:id/proxy/sni-rules.
func (h *ProxyHandler) DeleteAllSNI(c *gin.Context) {
	h.applyServerID(c)
	count, err := h.proxyMgr.DeleteAllSNI(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": count})
}

// ToggleSNI handles POST /api/v1/node-servers/:id/proxy/sni-rules/toggle.
func (h *ProxyHandler) ToggleSNI(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		Enabled   bool   `json:"enabled"`
		ProxyType string `json:"proxy_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.proxyMgr.ToggleSNIEnabled(c.Request.Context(), req.ProxyType, req.Enabled)
	if err != nil {
		response.InternalError(c, "TOGGLE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"updated": count})
}

// GetL4Settings handles GET /api/v1/proxy/l4-settings.
func (h *ProxyHandler) GetL4Settings(c *gin.Context) {
	enabled := h.proxyMgr.GetL4Enabled(c.Request.Context())
	response.OK(c, gin.H{"enabled": enabled})
}

// SaveL4Settings handles POST /api/v1/proxy/l4-settings.
func (h *ProxyHandler) SaveL4Settings(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.proxyMgr.SetL4Enabled(c.Request.Context(), req.Enabled); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"enabled": req.Enabled})
}

// DeleteAllL4 handles DELETE /api/v1/node-servers/:id/proxy/l4-rules.
func (h *ProxyHandler) DeleteAllL4(c *gin.Context) {
	h.applyServerID(c)
	count, err := h.proxyMgr.DeleteAllL4(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": count})
}

// UpdateGuardian handles POST /api/v1/node-servers/:id/proxy/guardian.
// Persists guardian settings (auto_start, auto_restart, max_restarts) to DB
// so they survive process restarts. Also updates the in-memory guardian if
// the proxy is currently running. Works for both local and remote nodes.
func (h *ProxyHandler) UpdateGuardian(c *gin.Context) {
	proxyType := proxy.ProxyType(c.Query("type"))
	if proxyType == "" {
		response.BadRequest(c, "INVALID_TYPE", "type is required")
		return
	}
	var req struct {
		AutoStart   *bool `json:"auto_start"`
		AutoRestart *bool `json:"auto_restart"`
		MaxRestarts *int  `json:"max_restarts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	serverID, isLocal := h.applyServerID(c)

	// For remote nodes, delegate via NodeOperator
	if !isLocal && h.nodeOp != nil {
		if err := h.nodeOp.UpdateProxyGuardian(c.Request.Context(), serverID, string(proxyType), req.AutoStart, req.AutoRestart, req.MaxRestarts); err != nil {
			response.InternalError(c, "UPDATE_FAILED", err.Error())
			return
		}
		response.OK(c, gin.H{"controlled": true})
		return
	}

	// Local: read current persisted settings
	ctx := c.Request.Context()
	autoStart, autoRestart, maxRestarts := h.proxyMgr.GetProxyGuardianSettings(ctx, proxyType)
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}
	if req.AutoRestart != nil {
		autoRestart = *req.AutoRestart
	}
	if req.MaxRestarts != nil && *req.MaxRestarts > 0 {
		maxRestarts = *req.MaxRestarts
	}

	// Persist to DB (works even when proxy is stopped)
	if err := h.proxyMgr.SetProxyGuardianSettings(ctx, proxyType, autoStart, autoRestart, maxRestarts); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	// Also update in-memory guardian if running
	if g, ok := h.proxyMgr.GetGuardian(proxyType); ok {
		g.SetMaxRestarts(maxRestarts)
		g.SetAutoRestart(autoRestart)
	}

	response.OK(c, gin.H{
		"auto_start":   autoStart,
		"auto_restart": autoRestart,
		"max_restarts": maxRestarts,
	})
}
