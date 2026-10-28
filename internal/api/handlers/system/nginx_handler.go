package system

import (
	"context"
	"fmt"
	"strconv"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/proxy"

	"github.com/gin-gonic/gin"
)

// NginxHandler handles Nginx-specific HTTP requests.
type NginxHandler struct {
	nginxMgr *proxy.NginxManager
	proxyMgr *proxy.Manager
	nodeOp   NodeOperator
	// wafProvisioner 为「转发」类站点（面板/门户/订阅）生成站点级 WAF 配置（由 router 注入）。
	wafProvisioner func(ctx context.Context, serverID int64, template, domain string)
}

// NewNginxHandler creates a new Nginx handler.
func NewNginxHandler(nginxMgr *proxy.NginxManager, proxyMgr *proxy.Manager) *NginxHandler {
	return &NginxHandler{nginxMgr: nginxMgr, proxyMgr: proxyMgr}
}

// SetWAFProvisioner 注入转发站点 WAF 配置生成器（面板=独立 URL 白名单；门户/订阅=独立站点级配置）。
func (h *NginxHandler) SetWAFProvisioner(fn func(ctx context.Context, serverID int64, template, domain string)) {
	h.wafProvisioner = fn
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

// httpTemplateHasLocation 判断模板是否生成 HTTP location/route
// （sni/l4/reality/stream 不生成，不参与同域名 location 冲突判定）。
func httpTemplateHasLocation(template string) bool {
	switch template {
	case "sni", "l4", "reality", "stream":
		return false
	}
	return true
}

// nginxLocationConflict 检查是否已存在「同域名 + 同 location」的启用站点，返回其 id。
func nginxLocationConflict(ctx context.Context, mgr *proxy.NginxManager, domain, path, template string, excludeID int64) (int64, bool) {
	if !httpTemplateHasLocation(template) {
		return 0, false
	}
	sites, err := mgr.ListSites(ctx)
	if err != nil {
		return 0, false
	}
	nd := proxy.NormalizeGroupDomain(domain)
	np := proxy.NormalizeLocationPath(path)
	for _, s := range sites {
		if !s.Enabled || s.ID == excludeID {
			continue
		}
		if proxy.NormalizeGroupDomain(s.Domain) == nd && proxy.NormalizeLocationPath(s.Path) == np {
			return s.ID, true
		}
	}
	return 0, false
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
	sid, _ := h.applyServerID(c)
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
		UpstreamTLS     bool   `json:"upstream_tls"`
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
		RedirectHTTPS   *bool  `json:"redirect_https"` // HTTP→HTTPS 跳转开关 ✓（nil=开 ✓；生成器还会判 80 端口是否开启 ✓）
		IsPanel         bool   `json:"is_panel"`
		DualCert        *bool  `json:"dual_cert"` // 是否挂同域名的多张证书（ECC+RSA）；nil=是
		Priority        int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if id, dup := nginxLocationConflict(c.Request.Context(), h.nginxMgr, req.Domain, req.Path, req.Template, 0); dup {
		response.BadRequest(c, "LOCATION_CONFLICT", fmt.Sprintf("域名 %s 已存在相同 location（站点 id=%d）；同一域名下 location 不能重复", proxy.NormalizeGroupDomain(req.Domain), id))
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
		UpstreamTLS:     req.UpstreamTLS,
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
		RedirectHTTPS:   req.RedirectHTTPS,
		Enabled:         true,
		IsPanel:         req.IsPanel,
		DualCert:        req.DualCert,
		Priority:        req.Priority,
	}
	if err := h.nginxMgr.CreateSite(c.Request.Context(), site); err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	switch site.Template {
	case "panel_forward", "portal_forward", "subscription_forward":
		if h.wafProvisioner != nil {
			h.wafProvisioner(c.Request.Context(), sid, site.Template, site.Domain)
		}
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
		UpstreamTLS     *bool   `json:"upstream_tls"`
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
		WafEnabled      *bool   `json:"waf_enabled"`
		RedirectHTTPS   *bool   `json:"redirect_https"` // HTTP→HTTPS 跳转开关 ✓（nil=不改 ✓）
		IsPanel         *bool   `json:"is_panel"`
		DualCert        *bool   `json:"dual_cert"`
		Priority        *int    `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Domain != nil || req.Path != nil || req.Template != nil {
		nd, np, nt := existing.Domain, existing.Path, existing.Template
		if req.Domain != nil {
			nd = *req.Domain
		}
		if req.Path != nil {
			np = *req.Path
		}
		if req.Template != nil {
			nt = *req.Template
		}
		if id, dup := nginxLocationConflict(c.Request.Context(), h.nginxMgr, nd, np, nt, existing.ID); dup {
			response.BadRequest(c, "LOCATION_CONFLICT", fmt.Sprintf("域名 %s 已存在相同 location（站点 id=%d）；同一域名下 location 不能重复", proxy.NormalizeGroupDomain(nd), id))
			return
		}
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
	if req.ListenPort != nil {
		existing.ListenPort = *req.ListenPort
	}
	if req.ListenSocket != nil {
		existing.ListenSocket = *req.ListenSocket
	}
	if req.CertPath != nil {
		existing.CertPath = *req.CertPath
	}
	if req.KeyPath != nil {
		existing.KeyPath = *req.KeyPath
	}
	if req.ServerConfig != nil {
		existing.ServerConfig = *req.ServerConfig
	}
	if req.LocationConfig != nil {
		existing.LocationConfig = *req.LocationConfig
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
	if req.SslProtocols != nil {
		existing.SslProtocols = *req.SslProtocols
	}
	if req.SslCiphers != nil {
		existing.SslCiphers = *req.SslCiphers
	}
	if req.SslEcdhCurve != nil {
		existing.SslEcdhCurve = *req.SslEcdhCurve
	}
	if req.WafEnabled != nil {
		existing.WafEnabled = req.WafEnabled
	}
	if req.RedirectHTTPS != nil {
		existing.RedirectHTTPS = req.RedirectHTTPS
	}
	if req.IsPanel != nil {
		existing.IsPanel = *req.IsPanel
	}
	if req.DualCert != nil {
		existing.DualCert = req.DualCert
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
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

// ImportSites handles POST /api/v1/node-servers/:id/nginx/import.
//
// 把现网 nginx 配置文本解析成站点；dry_run=true 只返回预览（不落库）。
// 用途：从旧服务器迁移到面板托管生成 nginx 配置时，避免手抄每个 conf。
func (h *NginxHandler) ImportSites(c *gin.Context) {
	serverID, _ := h.applyServerID(c)
	var req struct {
		Content string `json:"content" binding:"required"`
		DryRun  bool   `json:"dry_run"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	res := proxy.ParseNginxSites(req.Content)
	if req.DryRun {
		response.OK(c, gin.H{
			"dry_run":  true,
			"sites":    res.Sites,
			"warnings": res.Warnings,
		})
		return
	}

	created := 0
	skipped := make([]string, 0)
	for i := range res.Sites {
		site := res.Sites[i]
		site.ServerID = serverID
		if id, dup := nginxLocationConflict(c.Request.Context(), h.nginxMgr, site.Domain, site.Path, site.Template, 0); dup {
			skipped = append(skipped, fmt.Sprintf("%s %s 已存在（站点 id=%d）", site.Domain, site.Path, id))
			continue
		}
		if err := h.nginxMgr.CreateSite(c.Request.Context(), &site); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s %s 创建失败：%v", site.Domain, site.Path, err))
			continue
		}
		created++
	}
	response.OK(c, gin.H{
		"created":  created,
		"skipped":  skipped,
		"warnings": res.Warnings,
	})
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
	// 开启全局 TLS 时自动补齐素材（base64 存库），并把最新的完整设置返回给前端。
	if s, err := h.nginxMgr.EnsureGlobalSSLMaterial(c.Request.Context()); err == nil {
		response.OK(c, s)
		return
	}
	response.OK(c, settings)
}

// GenerateSSLFiles 重新生成全局 TLS 素材（dhparam / 会话票据密钥），存库并还原到本机磁盘。
// 供前端「生成/替换」按钮调用；target: dhparam | session_ticket。
func (h *NginxHandler) GenerateSSLFiles(c *gin.Context) {
	h.applyServerID(c)
	var req struct {
		Target string `json:"target"`
	}
	_ = c.ShouldBindJSON(&req)
	settings, err := h.nginxMgr.RegenerateGlobalSSLMaterial(c.Request.Context(), req.Target)
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
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
