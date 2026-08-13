package system

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/proxy"
	agentpb "corefusion/internal/runtime/agent/proto"
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

// --- WAF Management ---

// GetWAFConfig handles GET /api/v1/node-servers/:id/nginx/waf/config.
func (h *NginxHandler) GetWAFConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_config")
		if err != nil {
			response.NotFound(c, "WAF_NOT_FOUND", "WAF config.lua not found: "+err.Error())
			return
		}
		response.OK(c, gin.H{"content": string(data)})
		return
	}
	content, err := h.nginxMgr.ReadWAFConfig()
	if err != nil {
		response.NotFound(c, "WAF_NOT_FOUND", "WAF config.lua not found: "+err.Error())
		return
	}
	response.OK(c, gin.H{"content": content})
}

// SaveWAFConfig handles PUT /api/v1/node-servers/:id/nginx/waf/config.
func (h *NginxHandler) SaveWAFConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if isLocal {
		if err := h.nginxMgr.SaveWAFConfig(req.Content); err != nil {
			response.InternalError(c, "SAVE_FAILED", err.Error())
			return
		}
	} else {
		// Remote: patch paths and send to agent only
		content := req.Content
		content = strings.Replace(content,
			"config_log_dir = \"/apps/nginx/log/\"",
			"config_log_dir = \"logs\"", 1)
		content = strings.Replace(content,
			"config_rule_dir = \"/apps/nginx/conf/waf/rule-config\"",
			"config_rule_dir = \"proxy/lua/waf/rule-config\"", 1)
		h.syncWAFFileRemote(c, serverID, "config.lua", []byte(content))
		response.OK(c, gin.H{"message": "saved"})
		return
	}
	h.syncWAFFiles(c, serverID)
	response.OK(c, gin.H{"message": "saved"})
}

// ListWAFRules handles GET /api/v1/node-servers/:id/nginx/waf/rules.
func (h *NginxHandler) ListWAFRules(c *gin.Context) {
	h.applyServerID(c)
	files := h.nginxMgr.ListWAFRules()
	if files == nil {
		files = []proxy.WAFRuleFile{}
	}
	response.OK(c, files)
}

// GetWAFRule handles GET /api/v1/node-servers/:id/nginx/waf/rules/:name.
func (h *NginxHandler) GetWAFRule(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	name := c.Param("name")
	if !isLocal && h.nodeOp != nil {
		data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_rule:"+name)
		if err != nil {
			response.NotFound(c, "RULE_NOT_FOUND", "rule file not found: "+err.Error())
			return
		}
		response.OK(c, gin.H{"name": name, "content": string(data)})
		return
	}
	content, err := h.nginxMgr.ReadWAFRule(name)
	if err != nil {
		response.NotFound(c, "RULE_NOT_FOUND", "rule file not found: "+err.Error())
		return
	}
	response.OK(c, gin.H{"name": name, "content": content})
}

// SaveWAFRule handles PUT /api/v1/node-servers/:id/nginx/waf/rules/:name.
func (h *NginxHandler) SaveWAFRule(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	name := c.Param("name")
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if isLocal {
		if err := h.nginxMgr.SaveWAFRule(name, req.Content); err != nil {
			response.InternalError(c, "SAVE_FAILED", err.Error())
			return
		}
	} else {
		h.syncWAFFileRemote(c, serverID, "rule-config/"+name, []byte(req.Content))
		response.OK(c, gin.H{"message": "saved"})
		return
	}
	h.syncWAFFiles(c, serverID)
	response.OK(c, gin.H{"message": "saved"})
}

// GetWAFDomainConfig handles GET /api/v1/node-servers/:id/nginx/waf/domain-config.
func (h *NginxHandler) GetWAFDomainConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_domain")
		if err != nil {
			response.NotFound(c, "DOMAIN_CONFIG_NOT_FOUND", "domain.json not found: "+err.Error())
			return
		}
		response.OK(c, gin.H{"content": string(data)})
		return
	}
	content, err := h.nginxMgr.ReadWAFDomainConfig()
	if err != nil {
		response.NotFound(c, "DOMAIN_CONFIG_NOT_FOUND", "domain.json not found: "+err.Error())
		return
	}
	response.OK(c, gin.H{"content": content})
}

// SaveWAFDomainConfig handles PUT /api/v1/node-servers/:id/nginx/waf/domain-config.
func (h *NginxHandler) SaveWAFDomainConfig(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if isLocal {
		if err := h.nginxMgr.SaveWAFDomainConfig(req.Content); err != nil {
			response.InternalError(c, "SAVE_FAILED", err.Error())
			return
		}
	} else {
		h.syncWAFFileRemote(c, serverID, "rule-config/domain.json", []byte(req.Content))
		response.OK(c, gin.H{"message": "saved"})
		return
	}
	h.syncWAFFiles(c, serverID)
	response.OK(c, gin.H{"message": "saved"})
}

// ListWAFDomainDirs handles GET /api/v1/node-servers/:id/nginx/waf/domain-rules.
func (h *NginxHandler) ListWAFDomainDirs(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	if !isLocal && h.nodeOp != nil {
		// Remote: derive domain list from remote domain.json entries that have rule_dir set
		data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_domain")
		if err != nil {
			response.OK(c, []string{})
			return
		}
		var obj map[string]map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			response.OK(c, []string{})
			return
		}
		var dirs []string
		for domain, cfg := range obj {
			if _, ok := cfg["rule_dir"]; ok {
				dirs = append(dirs, domain)
			}
		}
		if dirs == nil {
			dirs = []string{}
		}
		response.OK(c, dirs)
		return
	}
	dirs := h.nginxMgr.ListWAFDomainRuleDirs()
	if dirs == nil {
		dirs = []string{}
	}
	response.OK(c, dirs)
}

// GetWAFDomainRule handles GET /api/v1/node-servers/:id/nginx/waf/domain-rules/:domain/:rule.
func (h *NginxHandler) GetWAFDomainRule(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	domain := c.Param("domain")
	ruleName := c.Param("rule")
	if strings.Contains(domain, "..") || strings.Contains(ruleName, "..") {
		response.BadRequest(c, "INVALID_PATH", "invalid domain or rule name")
		return
	}
	if !isLocal && h.nodeOp != nil {
		// Remote: read domain rule file from agent via waf_rule:domains/<domain>/<rule>
		data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID,
			fmt.Sprintf("waf_rule:domains/%s/%s", domain, ruleName))
		if err != nil {
			response.NotFound(c, "RULE_NOT_FOUND", "domain rule not found: "+err.Error())
			return
		}
		response.OK(c, gin.H{"domain": domain, "rule": ruleName, "content": string(data)})
		return
	}
	content, err := h.nginxMgr.ReadWAFDomainRule(domain, ruleName)
	if err != nil {
		response.NotFound(c, "RULE_NOT_FOUND", "domain rule not found: "+err.Error())
		return
	}
	response.OK(c, gin.H{"domain": domain, "rule": ruleName, "content": content})
}

// SaveWAFDomainRule handles PUT /api/v1/node-servers/:id/nginx/waf/domain-rules/:domain/:rule.
func (h *NginxHandler) SaveWAFDomainRule(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	domain := c.Param("domain")
	ruleName := c.Param("rule")
	if strings.Contains(domain, "..") || strings.Contains(ruleName, "..") {
		response.BadRequest(c, "INVALID_PATH", "invalid domain or rule name")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if isLocal {
		if err := h.nginxMgr.SaveWAFDomainRule(domain, ruleName, req.Content); err != nil {
			response.InternalError(c, "SAVE_FAILED", err.Error())
			return
		}
		h.syncWAFFiles(c, serverID)
	} else {
		// Remote: send the single domain rule file to agent only
		h.syncWAFFileRemote(c, serverID,
			fmt.Sprintf("rule-config/domains/%s/%s", domain, ruleName),
			[]byte(req.Content))
	}
	response.OK(c, gin.H{"message": "saved"})
}

// ToggleWAFDomain handles POST /api/v1/node-servers/:id/nginx/waf/toggle-domain.
// Enables or disables WAF for a specific domain by adding/removing it from domain.json.
// When enabling, copies global rule files to domains/<domain>/ for independent editing.
func (h *NginxHandler) ToggleWAFDomain(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	var req struct {
		Domain  string `json:"domain" binding:"required"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if isLocal {
		if req.Enabled {
			if err := h.nginxMgr.EnableWAFDomain(req.Domain); err != nil {
				response.InternalError(c, "ENABLE_FAILED", err.Error())
				return
			}
		} else {
			if err := h.nginxMgr.DisableWAFDomain(req.Domain); err != nil {
				response.InternalError(c, "DISABLE_FAILED", err.Error())
				return
			}
		}
		h.syncWAFFiles(c, serverID)
	} else {
		// Remote: modify domain.json in memory and send to agent only
		if err := h.toggleWAFDomainRemote(c, serverID, req.Domain, req.Enabled); err != nil {
			response.InternalError(c, "TOGGLE_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"domain": req.Domain, "enabled": req.Enabled})
}

// toggleWAFDomainRemote toggles WAF for a domain on a remote node without
// modifying the local domain.json. It reads the remote domain.json, modifies
// it in memory, and sends the updated file (plus rule files if enabling) to
// the remote Agent via SyncWAFFiles.
func (h *NginxHandler) toggleWAFDomainRemote(c *gin.Context, serverID int64, domain string, enabled bool) error {
	if h.nodeOp == nil {
		return fmt.Errorf("node operator not available")
	}
	if strings.Contains(domain, "..") || domain == "" {
		return fmt.Errorf("invalid domain")
	}
	// Read remote domain.json
	data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_domain")
	if err != nil {
		data = []byte("{}")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		obj = map[string]interface{}{}
	}
	if obj == nil {
		obj = map[string]interface{}{}
	}

	var files []*agentpb.WAFFile

	if enabled {
		// Add domain entry with rule_dir
		obj[domain] = map[string]interface{}{
			"rule_dir": fmt.Sprintf("domains/%s", domain),
		}
		// Copy global rule files to domains/<domain>/ on remote
		ruleFiles := []string{"args.rule", "blackip.rule", "cookie.rule", "post.rule",
			"url.rule", "useragent.rule", "whiteip.rule", "whiteurl.rule",
			"whiteua.rule", "referer.rule", "fileext.rule"}
		for _, rf := range ruleFiles {
			content, err := h.nginxMgr.ReadWAFRule(rf)
			if err != nil {
				continue // skip missing files
			}
			files = append(files, &agentpb.WAFFile{
				Path:    fmt.Sprintf("rule-config/domains/%s/%s", domain, rf),
				Content: []byte(content),
			})
		}
	} else {
		delete(obj, domain)
	}

	// Send updated domain.json
	out, err := json.MarshalIndent(obj, "", "    ")
	if err != nil {
		return err
	}
	files = append(files, &agentpb.WAFFile{
		Path:    "rule-config/domain.json",
		Content: out,
	})

	if err := h.nodeOp.SyncWAFFiles(c.Request.Context(), serverID, files); err != nil {
		log.Printf("sync WAF files to server %d: %v", serverID, err)
	}
	return nil
}

// CheckWAFDomain handles GET /api/v1/node-servers/:id/nginx/waf/domain-status.
func (h *NginxHandler) CheckWAFDomain(c *gin.Context) {
	serverID, isLocal := h.applyServerID(c)
	domain := c.Query("domain")
	if !isLocal && h.nodeOp != nil {
		// Remote: read remote domain.json and check
		data, _, err := h.nodeOp.ReadConfigFileOnServer(c.Request.Context(), serverID, "waf_domain")
		if err != nil {
			response.OK(c, gin.H{"domain": domain, "enabled": false})
			return
		}
		var obj map[string]map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			response.OK(c, gin.H{"domain": domain, "enabled": false})
			return
		}
		cfg, ok := obj[domain]
		if !ok {
			response.OK(c, gin.H{"domain": domain, "enabled": false})
			return
		}
		if wafEnable, ok := cfg["waf_enable"].(string); ok && wafEnable == "off" {
			response.OK(c, gin.H{"domain": domain, "enabled": false})
			return
		}
		response.OK(c, gin.H{"domain": domain, "enabled": true})
		return
	}
	enabled := h.nginxMgr.IsWAFDomainEnabled(domain)
	response.OK(c, gin.H{"domain": domain, "enabled": enabled})
}

// syncWAFFileRemote sends a single WAF file to a remote agent (non-blocking).
func (h *NginxHandler) syncWAFFileRemote(c *gin.Context, serverID int64, path string, content []byte) {
	if h.nodeOp == nil {
		return
	}
	files := []*agentpb.WAFFile{{Path: path, Content: content}}
	if err := h.nodeOp.SyncWAFFiles(c.Request.Context(), serverID, files); err != nil {
		log.Printf("sync WAF file %s to server %d: %v", path, serverID, err)
	}
}

// syncWAFFiles collects all WAF files from disk and pushes them to the remote node
// (fire-and-forget, non-blocking). For local nodes this is a no-op.
func (h *NginxHandler) syncWAFFiles(c *gin.Context, serverID int64) {
	if h.nodeOp == nil {
		return
	}
	files := h.nginxMgr.CollectWAFFiles()
	if len(files) == 0 {
		return
	}
	pbFiles := make([]*agentpb.WAFFile, 0, len(files))
	for _, f := range files {
		pbFiles = append(pbFiles, &agentpb.WAFFile{
			Path:    f.Path,
			Content: f.Content,
		})
	}
	if err := h.nodeOp.SyncWAFFiles(c.Request.Context(), serverID, pbFiles); err != nil {
		log.Printf("sync WAF files to server %d: %v", serverID, err)
	}
}
