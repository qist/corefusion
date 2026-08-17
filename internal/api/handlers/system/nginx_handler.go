package system

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/proxy"
)

// NginxHandler handles Nginx-specific HTTP requests.
type NginxHandler struct {
	nginxMgr *proxy.NginxManager
	proxyMgr *proxy.Manager
	nodeOp   NodeOperator
}

// NewNginxHandler creates a new Nginx handler.
func NewNginxHandler(nginxMgr *proxy.NginxManager, proxyMgr *proxy.Manager) *NginxHandler {
	return &NginxHandler{nginxMgr: nginxMgr, proxyMgr: proxyMgr}
}

// SetNodeOperator sets the node operator for remote proxy operations.
func (h *NginxHandler) SetNodeOperator(op NodeOperator) {
	h.nodeOp = op
}

// applyServerID extracts the server ID from the URL and sets it on the managers
// so that site queries filter by the correct node.
func (h *NginxHandler) applyServerID(c *gin.Context) (int64, bool) {
	serverID := parseServerID(c)
	isLocal := serverID == localServerID || serverID == 0
	if !isLocal && h.nodeOp != nil {
		isLocal = h.nodeOp.IsLocalServer(c.Request.Context(), serverID)
	}
	sid := serverID
	if sid == 0 {
		sid = localServerID
	}
h.nginxMgr.SetServerID(sid)
h.proxyMgr.SetServerID(sid)
h.nginxMgr.SetIsLocal(isLocal)
h.proxyMgr.SetIsLocal(isLocal)
return sid, isLocal
}

// --- Site Management ---

func (h *NginxHandler) ListSites(c *gin.Context) {
	h.applyServerID(c)
	sites, err := h.nginxMgr.ListSites(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if sites == nil {
		sites = []entity.NginxSite{}
	}
	response.OK(c, sites)
}

func (h *NginxHandler) CreateSite(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		Domain          string `json:"domain" binding:"required"`
		Upstream        string `json:"upstream"`
		Template        string `json:"template"`
		Path            string `json:"path"`
		ListenPort      string `json:"listen_port"`
		ListenSocket    string `json:"listen_socket"`
		CertPath        string `json:"cert_path"`
		KeyPath         string `json:"key_path"`
		ServerConfig    string `json:"server_config"`
		LocationConfig  string `json:"location_config"`
		L4Protocol      string `json:"l4_protocol"`
		SNITarget       string `json:"sni_target"`
		LocalPort       string `json:"local_port"`
		ProxyProtocol   bool   `json:"proxy_protocol"`
		EnableHttp2     bool   `json:"enable_http2"`
		EnableIpv6      bool   `json:"enable_ipv6"`
		EnableQuic      bool   `json:"enable_quic"`
		EnableEarlyData bool   `json:"enable_early_data"`
		EnableStapling  bool   `json:"enable_stapling"`
		SslProtocols    string `json:"ssl_protocols"`
		SslCiphers      string `json:"ssl_ciphers"`
		SslEcdhCurve    string `json:"ssl_ecdh_curve"`
		WafEnabled      *bool  `json:"waf_enabled"`
		IsPanel         bool   `json:"is_panel"`
		Priority        int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	site := &entity.NginxSite{
		Domain:          req.Domain,
		Upstream:        req.Upstream,
		Template:        req.Template,
		Path:            req.Path,
		ListenPort:      req.ListenPort,
		ListenSocket:    req.ListenSocket,
		CertPath:        req.CertPath,
		KeyPath:         req.KeyPath,
		ServerConfig:    req.ServerConfig,
		LocationConfig:  req.LocationConfig,
		L4Protocol:      req.L4Protocol,
		SNITarget:       req.SNITarget,
		LocalPort:       req.LocalPort,
		ProxyProtocol:   req.ProxyProtocol,
		EnableHttp2:     req.EnableHttp2,
		EnableIpv6:      req.EnableIpv6,
		EnableQuic:      req.EnableQuic,
		EnableEarlyData: req.EnableEarlyData,
		EnableStapling:  req.EnableStapling,
		SslProtocols:    req.SslProtocols,
		SslCiphers:      req.SslCiphers,
		SslEcdhCurve:    req.SslEcdhCurve,
		WafEnabled:      req.WafEnabled,
		Enabled:         true,
		IsPanel:         req.IsPanel,
		Priority:        req.Priority,
	}
	if err := h.nginxMgr.CreateSite(c.Request.Context(), site); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, site)
}

func (h *NginxHandler) UpdateSite(c *gin.Context) {
	h.applyServerID(c)
	id, err := strconv.ParseInt(c.Param("siteId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	existing, err := h.nginxMgr.GetSite(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "site not found")
		return
	}
	var req struct {
		Domain          *string `json:"domain"`
		Upstream        *string `json:"upstream"`
		Template        *string `json:"template"`
		Path            *string `json:"path"`
		ListenPort      *string `json:"listen_port"`
		ListenSocket    *string `json:"listen_socket"`
		CertPath        *string `json:"cert_path"`
		KeyPath         *string `json:"key_path"`
		ServerConfig    *string `json:"server_config"`
		LocationConfig  *string `json:"location_config"`
		L4Protocol      *string `json:"l4_protocol"`
		SNITarget       *string `json:"sni_target"`
		LocalPort       *string `json:"local_port"`
		ProxyProtocol   *bool   `json:"proxy_protocol"`
		Enabled         *bool   `json:"enabled"`
		EnableHttp2     *bool   `json:"enable_http2"`
		EnableIpv6      *bool   `json:"enable_ipv6"`
		EnableQuic      *bool   `json:"enable_quic"`
		EnableEarlyData *bool   `json:"enable_early_data"`
		EnableStapling  *bool   `json:"enable_stapling"`
		SslProtocols    *string `json:"ssl_protocols"`
		SslCiphers      *string `json:"ssl_ciphers"`
		SslEcdhCurve    *string `json:"ssl_ecdh_curve"`
		WafEnabled      *bool  `json:"waf_enabled"`
		IsPanel         *bool   `json:"is_panel"`
		Priority        *int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Domain != nil { existing.Domain = *req.Domain }
	if req.Upstream != nil { existing.Upstream = *req.Upstream }
	if req.Template != nil { existing.Template = *req.Template }
	if req.Path != nil { existing.Path = *req.Path }
	if req.ListenPort != nil { existing.ListenPort = *req.ListenPort }
	if req.ListenSocket != nil { existing.ListenSocket = *req.ListenSocket }
	if req.CertPath != nil { existing.CertPath = *req.CertPath }
	if req.KeyPath != nil { existing.KeyPath = *req.KeyPath }
	if req.ServerConfig != nil { existing.ServerConfig = *req.ServerConfig }
	if req.LocationConfig != nil { existing.LocationConfig = *req.LocationConfig }
	if req.L4Protocol != nil { existing.L4Protocol = *req.L4Protocol }
	if req.SNITarget != nil { existing.SNITarget = *req.SNITarget }
	if req.LocalPort != nil { existing.LocalPort = *req.LocalPort }
	if req.ProxyProtocol != nil { existing.ProxyProtocol = *req.ProxyProtocol }
	if req.Enabled != nil { existing.Enabled = *req.Enabled }
	if req.EnableHttp2 != nil { existing.EnableHttp2 = *req.EnableHttp2 }
	if req.EnableIpv6 != nil { existing.EnableIpv6 = *req.EnableIpv6 }
	if req.EnableQuic != nil { existing.EnableQuic = *req.EnableQuic }
	if req.EnableEarlyData != nil { existing.EnableEarlyData = *req.EnableEarlyData }
	if req.EnableStapling != nil { existing.EnableStapling = *req.EnableStapling }
	if req.SslProtocols != nil { existing.SslProtocols = *req.SslProtocols }
	if req.SslCiphers != nil { existing.SslCiphers = *req.SslCiphers }
	if req.SslEcdhCurve != nil { existing.SslEcdhCurve = *req.SslEcdhCurve }
	if req.WafEnabled != nil { existing.WafEnabled = req.WafEnabled }
	if req.IsPanel != nil { existing.IsPanel = *req.IsPanel }
	if req.Priority != nil { existing.Priority = *req.Priority }
	if err := h.nginxMgr.UpdateSite(c.Request.Context(), existing); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, existing)
}

func (h *NginxHandler) DeleteSite(c *gin.Context) {
	h.applyServerID(c)
	id, err := strconv.ParseInt(c.Param("siteId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.nginxMgr.DeleteSite(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// --- Config ---

func (h *NginxHandler) GenerateConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	config, err := h.nginxMgr.GenerateConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	if isLocal {
		h.proxyMgr.WriteConfigFile(proxy.ProxyNginx, config)
		// Push config to mirror nodes so they get the updated nginx.conf.
		if h.nodeOp != nil {
			h.nodeOp.SyncMirrorsProxyConfig(c.Request.Context(), serverID, "nginx", []byte(config))
		}
	} else if h.nodeOp != nil {
		if err := h.nodeOp.SyncProxyConfig(c.Request.Context(), serverID, "nginx", []byte(config)); err != nil {
			response.InternalError(c, "SYNC_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"config": config})
}

func (h *NginxHandler) SaveAndApply(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	config, err := h.nginxMgr.SaveAndApply(c.Request.Context())
	if err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if isLocal {
		// Push config to mirror nodes so they get the updated nginx.conf.
		if h.nodeOp != nil {
			h.nodeOp.SyncMirrorsProxyConfig(c.Request.Context(), serverID, "nginx", []byte(config))
		}
	} else if h.nodeOp != nil {
		if err := h.nodeOp.SyncProxyConfig(c.Request.Context(), serverID, "nginx", []byte(config)); err != nil {
			response.InternalError(c, "SYNC_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"config": config})
}

// --- Settings ---

func (h *NginxHandler) GetSettings(c *gin.Context) {
	h.applyServerID(c)
	settings, err := h.nginxMgr.GetNginxSettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, settings)
}

func (h *NginxHandler) SaveSettings(c *gin.Context) {
	h.applyServerID(c)
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

// --- SNI ---

func (h *NginxHandler) ToggleSNI(c *gin.Context) {
	serverID, _ := h.applyServerID(c)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.nginxMgr.ToggleSNIEnabled(c.Request.Context(), req.Enabled)
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

func (h *NginxHandler) DeleteAllSNI(c *gin.Context) {
	h.applyServerID(c)
	count, err := h.nginxMgr.DeleteAllSNI(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": count})
}

func (h *NginxHandler) BatchUpdatePorts(c *gin.Context) {
	serverID, _ := h.applyServerID(c)
	var req struct {
		Port   string `json:"port" binding:"required"`
		Enable bool   `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	count, err := h.nginxMgr.BatchUpdatePorts(c.Request.Context(), req.Port, req.Enable)
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

// --- L4 ---

func (h *NginxHandler) DeleteAllL4(c *gin.Context) {
	h.applyServerID(c)
	count, err := h.nginxMgr.DeleteAllL4(c.Request.Context())
	if err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": count})
}
