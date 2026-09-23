package waf

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	"corefusion/internal/service/waf"
)

// NodeOperator provides node-level operations for remote WAF management.
type NodeOperator interface {
	IsLocalServer(ctx context.Context, serverID int64) bool
	ReadConfigFileOnServer(ctx context.Context, serverID int64, coreType string) ([]byte, string, error)
	SyncWAFFiles(ctx context.Context, serverID int64, files []*waf.WAFFile) error
	// ControlProxy sends a control command (e.g. "reload") to the proxy on the node.
	ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error
	// GetInstalledProxyTypes returns the set of proxy types that are installed
	// on the node. Only installed proxies get their WAF config files pushed.
	GetInstalledProxyTypes(ctx context.Context, serverID int64) ([]string, error)
}

// Handler handles unified WAF HTTP requests (both global rules and per-node configs).
type Handler struct {
	svc     *waf.Service
	nodeOp  NodeOperator
	proxyMgr ProxyReloader
}

// ProxyReloader provides proxy detection and reload capability for local nodes.
// Implemented by *proxy.Manager via an adapter.
type ProxyReloader interface {
	DetectProxy(proxyType string) ProxyInfo
	ReloadProxy(ctx context.Context, proxyType string) error
}

// ProxyInfo describes a proxy's installation and running status.
type ProxyInfo struct {
	Installed bool
	Status    string
}

// NewHandler creates a new WAF handler.
func NewHandler(svc *waf.Service) *Handler {
	return &Handler{svc: svc}
}

// SetNodeOperator sets the node operator for remote WAF operations.
func (h *Handler) SetNodeOperator(op NodeOperator) {
	h.nodeOp = op
}

// SetProxyManager sets the proxy manager for local proxy reload.
func (h *Handler) SetProxyManager(mgr ProxyReloader) {
	h.proxyMgr = mgr
}

// localServerID is the server ID of the local node.
const localServerID = int64(1)

// parseServerID extracts the :id parameter and returns the server ID.
func parseServerID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// --- Global Rule Library ---

// ListRules handles GET /api/v1/waf/rules.
// Query param: scope=global|node (optional, default: all)
func (h *Handler) ListRules(c *gin.Context) {
	scope := c.Query("scope")
	if scope == "global" {
		rules, err := h.svc.ListGlobalRules(c.Request.Context())
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		if rules == nil {
			rules = []entity.WafRule{}
		}
		response.OK(c, rules)
		return
	}

	rules, err := h.svc.ListRules(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if rules == nil {
		rules = []entity.WafRule{}
	}
	response.OK(c, rules)
}

// GetRule handles GET /api/v1/waf/rules/:name.
func (h *Handler) GetRule(c *gin.Context) {
	name := c.Param("name")
	rule, err := h.svc.GetRule(c.Request.Context(), name)
	if err != nil {
		response.OK(c, gin.H{"name": name, "content": ""})
		return
	}
	response.OK(c, gin.H{"name": rule.Name, "content": rule.Content})
}

// SaveRule handles PUT /api/v1/waf/rules/:name.
// Body: { "content": "...", "merge_mode": "append|replace" }
func (h *Handler) SaveRule(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Content   string `json:"content"`
		MergeMode string `json:"merge_mode"` // append | replace, default: replace
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveRule(c.Request.Context(), name, req.Content, req.MergeMode); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	// Land to local disk AND push the updated (global + node-merged) rules to
	// every remote node. Global changes must reach all online nodes immediately,
	// otherwise each node's merged rule file stays stale (it cannot read global
	// directly — it only has its own merged copy).
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		log.Printf("[WAF] land rules to disk: %v", err)
	}
	if err := h.svc.SyncToAllNodesFromDB(c.Request.Context()); err != nil {
		log.Printf("[WAF] push global rule %q to all nodes: %v", name, err)
	}
	response.OK(c, gin.H{"message": "saved"})
}

// DeleteRule handles DELETE /api/v1/waf/rules/:name.
func (h *Handler) DeleteRule(c *gin.Context) {
	name := c.Param("name")
	if err := h.svc.DeleteRule(c.Request.Context(), name); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	// Land locally and push the updated merged rules to every remote node.
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		log.Printf("[WAF] land rules to disk: %v", err)
	}
	if err := h.svc.SyncToAllNodesFromDB(c.Request.Context()); err != nil {
		log.Printf("[WAF] push global rule deletion %q to all nodes: %v", name, err)
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// SyncFromGitHub handles POST /api/v1/waf/sync-github.
// Downloads rules from the configured source URL, then pushes to all nodes.
func (h *Handler) SyncFromGitHub(c *gin.Context) {
	// Get configured source URL and file mappings
	source, _ := h.svc.GetGitHubSource(c.Request.Context())
	sourceURL := ""
	var fileMappings []waf.SourceFileMapping
	if source != nil {
		sourceURL = source.URL
		fileMappings = source.FileMappings
	}
	if err := h.svc.SyncFromGitHubWithSource(c.Request.Context(), sourceURL, source.Files, fileMappings); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		log.Printf("[WAF] land rules to disk: %v", err)
	}
	// Push to all nodes after sync
	if err := h.svc.SyncToAllNodesFromDB(c.Request.Context()); err != nil {
		log.Printf("[WAF] push to all nodes after sync: %v", err)
	}
	response.OK(c, gin.H{"message": "synced"})
}

// SyncAll handles POST /api/v1/waf/sync-all.
// Pushes all WAF rules and configs to all online nodes.
func (h *Handler) SyncAll(c *gin.Context) {
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		response.InternalError(c, "LAND_FAILED", err.Error())
		return
	}
	// Push to all online nodes
	if err := h.svc.SyncToAllNodesFromDB(c.Request.Context()); err != nil {
		response.OK(c, gin.H{"message": "synced locally, remote sync had errors", "error": err.Error()})
		return
	}
	response.OK(c, gin.H{"message": "synced"})
}

// GetGitHubSource handles GET /api/v1/waf/github-source.
// Returns the configured GitHub source URL and sync schedule.
func (h *Handler) GetGitHubSource(c *gin.Context) {
	source, err := h.svc.GetGitHubSource(c.Request.Context())
	if err != nil {
		response.OK(c, gin.H{"url": "", "sync_cron": "", "enabled": false})
		return
	}
	response.OK(c, source)
}

// SaveGitHubSource handles PUT /api/v1/waf/github-source.
// Body: { "url": "...", "file_mappings": [{"name":"args.rule","url":"https://..."}], "sync_cron": "...", "enabled": true/false }
func (h *Handler) SaveGitHubSource(c *gin.Context) {
	var req waf.GitHubSourceConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveGitHubSource(c.Request.Context(), &req); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// PushToAllNodes handles POST /api/v1/waf/push-all.
// Collects all WAF files from DB and pushes to all online nodes.
func (h *Handler) PushToAllNodes(c *gin.Context) {
	if err := h.svc.SyncToAllNodesFromDB(c.Request.Context()); err != nil {
		response.InternalError(c, "PUSH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "pushed"})
}

// PushToNode handles POST /api/v1/node-servers/:id/waf/push.
// Pushes all WAF files for this specific node.
func (h *Handler) PushToNode(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	if err := h.svc.SyncToNode(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "PUSH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "pushed"})
}

// --- Per-Node WAF Rules ---

// ListNodeRules handles GET /api/v1/node-servers/:id/waf/rules.
// Returns merged rules (global + node override) for this server.
// Each rule includes a "source" field: "global" | "node" | "merged".
func (h *Handler) ListNodeRules(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	rules, err := h.svc.ListNodeRules(c.Request.Context(), serverID)
	if err != nil {
	// ⚠ 原来这里返回 200 + 空数组 ✗：把"查询失败"伪装成"没有规则"，
	// 前端/管理员完全无感（WAF 规则看着像被清空了，却一条报错都没有）。
	response.InternalError(c, "LIST_FAILED", err.Error())
	return
	}
	response.OK(c, rules)
}

// GetNodeRule handles GET /api/v1/node-servers/:id/waf/rules/:name.
// Returns the merged content (global + node override) for a specific rule.
// The "source" field indicates: "global" (from global only), "node" (node only), "merged" (both combined).
func (h *Handler) GetNodeRule(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	name := c.Param("name")
	rule, err := h.svc.GetNodeRule(c.Request.Context(), name, serverID)
	if err != nil {
		response.OK(c, gin.H{"name": name, "content": "", "exists": false, "source": "none"})
		return
	}
	resp := gin.H{
		"name":       rule.Name,
		"content":    rule.Content,
		"exists":     true,
		"source":     rule.Source,
		"merge_mode": rule.MergeMode,
		"hash":       rule.Hash,
	}
	if rule.Source == "merged" {
		resp["global_content"] = rule.GlobalContent
		resp["node_content"] = rule.NodeContent
	}
	response.OK(c, resp)
}

// SaveNodeRule handles PUT /api/v1/node-servers/:id/waf/rules/:name.
// Creates or updates a node-level override for the given rule.
// Body: { "content": "...", "merge_mode": "append|replace" }
// After saving, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) SaveNodeRule(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	name := c.Param("name")
	var req struct {
		Content   string `json:"content"`
		MergeMode string `json:"merge_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveNodeRule(c.Request.Context(), name, serverID, req.Content, req.MergeMode); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// DeleteNodeRule handles DELETE /api/v1/node-servers/:id/waf/rules/:name.
// Removes the node-level override, falling back to the global rule.
// After deleting, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) DeleteNodeRule(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	name := c.Param("name")
	if err := h.svc.DeleteNodeRule(c.Request.Context(), name, serverID); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted, falling back to global rule"})
}

// --- Per-Node WAF Config ---

// GetConfig handles GET /api/v1/node-servers/:id/waf/config.
// Query param: proxy_type=caddy|nginx
func (h *Handler) GetConfig(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	proxyType := c.DefaultQuery("proxy_type", "caddy")
	content, err := h.svc.GetConfig(c.Request.Context(), serverID, proxyType)
	if err != nil {
		response.OK(c, gin.H{"content": ""})
		return
	}
	response.OK(c, gin.H{"content": content})
}

// SaveConfig handles PUT /api/v1/node-servers/:id/waf/config.
// Body: { "proxy_type": "caddy|nginx", "content": "..." }
// After saving, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) SaveConfig(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	var req struct {
		ProxyType string `json:"proxy_type"`
		Content   string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.ProxyType == "" {
		req.ProxyType = "caddy"
	}
	if err := h.svc.SaveConfig(c.Request.Context(), serverID, req.ProxyType, req.Content); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// --- Per-Node Domain Config ---

// ListDomainConfigs handles GET /api/v1/node-servers/:id/waf/domain-config.
// Returns the entire domain.json content as {content: "..."}.
func (h *Handler) ListDomainConfigs(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	content, err := h.svc.GetDomainJSONContent(c.Request.Context(), serverID)
	if err != nil {
		response.OK(c, gin.H{"content": "{}"})
		return
	}
	response.OK(c, gin.H{"content": content})
}

// SaveDomainConfigsBulk handles PUT /api/v1/node-servers/:id/waf/domain-config.
// Replaces all domain configs from a JSON body {content: "<domain.json content>"}.
// After saving, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) SaveDomainConfigsBulk(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveDomainConfigsBulk(c.Request.Context(), serverID, req.Content); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// GetDomainConfig handles GET /api/v1/node-servers/:id/waf/domain-config/:domain.
func (h *Handler) GetDomainConfig(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	domain := c.Param("domain")
	config, err := h.svc.GetDomainConfig(c.Request.Context(), serverID, domain)
	if err != nil {
		response.OK(c, gin.H{"domain": domain, "config": ""})
		return
	}
	response.OK(c, gin.H{"domain": domain, "config": config})
}

// SaveDomainConfig handles PUT /api/v1/node-servers/:id/waf/domain-config/:domain.
// After saving, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) SaveDomainConfig(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	domain := c.Param("domain")
	var req struct {
		Config string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SaveDomainConfig(c.Request.Context(), serverID, domain, req.Config); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}

// DeleteDomainConfig handles DELETE /api/v1/node-servers/:id/waf/domain-config/:domain.
// Removes the domain entry from domain.json entirely (fall back to global rules).
// After deleting, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) DeleteDomainConfig(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	domain := waf.StripPort(c.Param("domain"))
	if err := h.svc.DeleteDomainConfig(c.Request.Context(), serverID, domain); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// EnableDomain handles POST /api/v1/node-servers/:id/waf/enable-domain.
// After enabling, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) EnableDomain(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	var req struct {
		Domain string `json:"domain" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// Strip port from domain (caddyguard matches without port)
	domain := waf.StripPort(req.Domain)
	// Save domain config with waf_enable=on
	config := fmt.Sprintf(`{"waf_enable":"on","rule_dir":"domains/%s"}`, domain)
	if err := h.svc.SaveDomainConfig(c.Request.Context(), serverID, domain, config); err != nil {
		response.InternalError(c, "ENABLE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	// Copy global rule files to domain directory (skip existing files).
	// Only for local node — remote nodes get their domain rules via gRPC push.
	if h.nodeOp == nil || h.nodeOp.IsLocalServer(c.Request.Context(), serverID) {
		if err := h.svc.InitDomainRules(c.Request.Context(), domain); err != nil {
			log.Printf("[WAF] failed to init domain rules for %s: %v", domain, err)
		}
	}
	response.OK(c, gin.H{"domain": domain, "enabled": true})
}

// DisableDomain handles POST /api/v1/node-servers/:id/waf/disable-domain.
// After disabling, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) DisableDomain(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	var req struct {
		Domain string `json:"domain" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// Strip port from domain (caddyguard matches without port)
	domain := waf.StripPort(req.Domain)
	// Save domain config with waf_enable=off
	config := `{"waf_enable":"off"}`
	if err := h.svc.SaveDomainConfig(c.Request.Context(), serverID, domain, config); err != nil {
		response.InternalError(c, "DISABLE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"domain": domain, "enabled": false})
}

// GetDomainStatus handles GET /api/v1/node-servers/:id/waf/domain-status.
// Returns the WAF status for a domain:
// - status="global": no domain config, uses global rules
// - status="site": domain config exists with waf_enable=on (site-specific rules)
// - status="off": domain config exists with waf_enable=off (WAF disabled for this domain)
func (h *Handler) GetDomainStatus(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	domain := waf.StripPort(c.Query("domain"))
	config, err := h.svc.GetDomainConfig(c.Request.Context(), serverID, domain)
	if err != nil {
		response.OK(c, gin.H{"domain": domain, "enabled": false, "status": "global"})
		return
	}
	// Parse config JSON to check waf_enable
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		response.OK(c, gin.H{"domain": domain, "enabled": false, "status": "global"})
		return
	}
	if wafEnable, ok := cfg["waf_enable"].(string); ok && wafEnable == "off" {
		response.OK(c, gin.H{"domain": domain, "enabled": false, "status": "off"})
		return
	}
	response.OK(c, gin.H{"domain": domain, "enabled": true, "status": "site"})
}

// SyncToNode handles POST /api/v1/node-servers/:id/waf/sync.
// Pushes all WAF files from DB to the specified node.
func (h *Handler) SyncToNode(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	if err := h.svc.SyncToNode(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "synced"})
}

// --- WAF Master Switch ---

// GetWAFStatus handles GET /api/v1/node-servers/:id/waf/status.
// Returns whether WAF is enabled for this node.
func (h *Handler) GetWAFStatus(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	enabled := h.svc.IsWAFEnabled(c.Request.Context(), serverID)
	response.OK(c, gin.H{"enabled": enabled})
}

// SetWAFStatus handles PUT /api/v1/node-servers/:id/waf/status.
// Body: { "enabled": true/false }
// When disabled, no WAF config is generated in Caddyfile/nginx.conf.
// After toggling, lands to local disk (local node) or pushes to remote Agent.
func (h *Handler) SetWAFStatus(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.svc.SetWAFEnabled(c.Request.Context(), serverID, req.Enabled); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"enabled": req.Enabled})
}

// --- Domain Rule Files ---

// ListDomainRuleDirs handles GET /api/v1/node-servers/:id/waf/domain-rules.
// Returns a list of domain names that have rule directories on disk.
func (h *Handler) ListDomainRuleDirs(c *gin.Context) {
	dirs, err := h.svc.ListDomainRuleDirs()
	if err != nil {
	// 同上：错误不再伪装成 200 空列表 ✓
	response.InternalError(c, "LIST_FAILED", err.Error())
	return
	}
	response.OK(c, dirs)
}

// GetDomainRuleFile handles GET /api/v1/node-servers/:id/waf/domain-rules/:domain/:name.
// Returns the domain-specific rule content plus the global rule content (read-only reference).
// If the domain doesn't have its own file, domain_content is empty — the WAF engine
// falls back to the global rule at runtime.
func (h *Handler) GetDomainRuleFile(c *gin.Context) {
	domain := waf.StripPort(c.Param("domain"))
	name := c.Param("name")
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	globalContent, domainContent, mergeMode, _ := h.svc.GetDomainRuleWithGlobal(c.Request.Context(), domain, name, serverID)
	response.OK(c, gin.H{
		"name":            name,
		"content":         domainContent,  // domain-specific content (editable)
		"global_content":  globalContent,  // global rule content (read-only reference)
		"has_domain_file": domainContent != "",
		"merge_mode":      mergeMode,
	})
}

// landAndSync is the common post-save hook for node-level WAF operations.
// For local node: writes rules/configs to local disk (LandRulesForServer) and
// reloads the running proxy so changes take effect immediately.
// For remote node: collects all WAF files from DB and pushes them via SyncWAFFiles,
// then sends a reload command so the remote Agent reloads its proxy.
// A remote node's rules are NEVER written to the panel's local disk.
func (h *Handler) landAndSync(ctx context.Context, serverID int64) error {
	isLocal := h.nodeOp == nil || h.nodeOp.IsLocalServer(ctx, serverID)
	if isLocal {
		if err := h.svc.LandRulesForServer(ctx, serverID); err != nil {
			return fmt.Errorf("land rules for local server %d: %w", serverID, err)
		}
		// Reload local proxies so WAF changes take effect.
		// caddyguard hot-reloads config.json within ~2s, but we reload
		// anyway for nginx (Lua WAF needs reload to re-read config.lua).
		h.reloadLocalProxies(ctx)
	} else {
		if err := h.pushToRemote(ctx, serverID); err != nil {
			return err
		}
		// Send reload command to remote Agent for both nginx and caddy.
		h.reloadRemoteProxies(ctx, serverID)
	}

	// Fan-out: if this server has mirror nodes, sync WAF to them too.
	// This ensures WAF changes on the primary node are immediately
	// propagated to all its mirrors.
	h.fanOutMirrorSync(ctx, serverID)

	return nil
}

// ReconcileMirror 对**单个镜像**做一次 WAF 对账：把主节点的规则推过去并 reload ✓。
//
// 走的路径与事件驱动的 fanOutMirrorSync **完全一致**（pushToRemote + reloadRemoteProxies ✓，
// 且 SyncToNode 内部对镜像会取主节点的规则 ✓），区别只是由定时循环兜底触发 ✓。
func (h *Handler) ReconcileMirror(ctx context.Context, mirrorServerID int64) error {
	if err := h.pushToRemote(ctx, mirrorServerID); err != nil {
		return err
	}
	h.reloadRemoteProxies(ctx, mirrorServerID)
	return nil
}

// RunMirrorReconcileLoop 周期性对所有镜像做 WAF 对账 ✓（启动即先跑一次 ✓）。
//
// ⚠ 为什么必须有它 ✗：主节点改 WAF 时会 fanOutMirrorSync 推给镜像 ✓，但**若那一刻镜像离线**，
// 推送只在日志里留一行错误 ✓ —— **不重试、也没有定时兜底** ✗ → 该镜像会一直用旧规则，
// 直到下次有人再推一次 WAF（这就是"跨区 / 镜像 WAF 不一致"的成因 ✓）。
// 对照：inbound 有 MirrorSyncManager.RunLoop（每 10 分钟）兜底 ✓，WAF 原先什么都没有 ✓。
//
// isOnline 为 nil 时不做在线过滤（离线节点推送失败会被记进日志 ✓）。
// ⚠ 幂等 ✓：内容没变时 Agent 侧不会白重启代理 ✓（这是可以每 10 分钟跑的前提 ✓）。
func (h *Handler) RunMirrorReconcileLoop(ctx context.Context, interval time.Duration, isOnline func(int64) bool) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}

	reconcile := func() {
		ids, err := h.svc.ListMirrorServerIDs(ctx)
		if err != nil {
			log.Printf("[WAF] mirror reconcile: list mirrors failed: %v", err)
			return
		}
		if len(ids) == 0 {
			return
		}
		ok, skipped, failed := 0, 0, 0
		for _, id := range ids {
			if isOnline != nil && !isOnline(id) {
				skipped++
				continue
			}
			if err := h.ReconcileMirror(ctx, id); err != nil {
				failed++
				log.Printf("[WAF] mirror reconcile: mirror %d failed: %v", id, err)
				continue
			}
			ok++
		}
		log.Printf("[WAF] mirror reconcile: %d mirror(s) — ok=%d skipped(offline)=%d failed=%d",
			len(ids), ok, skipped, failed)
	}

	// 启动即跑一次 ✓：与仓库里其它循环（quota enforcer / cert renewer / mirror sync）一致 ✓，
	// 也保证"面板重启后能立刻补上离线期间漏掉的 WAF" ✓。
	reconcile()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("[WAF] mirror reconcile loop stopped")
			return
		case <-ticker.C:
			reconcile()
		}
	}
}

// fanOutMirrorSync finds all mirror nodes that reference serverID as their
// primary (partner_server_id) and pushes WAF files to them.
// SyncToNode internally detects mirror nodes and collects rules from the
// primary's server_id, so the mirror gets the correct WAF configuration.
func (h *Handler) fanOutMirrorSync(ctx context.Context, primaryServerID int64) {
	mirrorIDs, err := h.svc.GetMirrorServerIDs(ctx, primaryServerID)
	if err != nil || len(mirrorIDs) == 0 {
		return
	}
	for _, mirrorID := range mirrorIDs {
		if err := h.pushToRemote(ctx, mirrorID); err != nil {
			log.Printf("[WAF] fan-out to mirror %d: %v", mirrorID, err)
			continue
		}
		h.reloadRemoteProxies(ctx, mirrorID)
		log.Printf("[WAF] fan-out to mirror %d: success", mirrorID)
	}
}

// reloadLocalProxies reloads installed and running proxies on the local node.
// Proxies that are not installed or not running are silently skipped —
// WAF rules are already on disk and will take effect on next start.
func (h *Handler) reloadLocalProxies(ctx context.Context) {
	if h.proxyMgr == nil {
		return
	}
	for _, pt := range []string{"nginx", "caddy"} {
		info := h.proxyMgr.DetectProxy(pt)
		if !info.Installed || info.Status != "running" {
			continue
		}
		if err := h.proxyMgr.ReloadProxy(ctx, pt); err != nil {
			log.Printf("[WAF] local reload %s: %v", pt, err)
		}
	}
}

// reloadRemoteProxies sends reload commands to the remote Agent.
// Only reloads proxies that are installed and running on the remote node.
func (h *Handler) reloadRemoteProxies(ctx context.Context, serverID int64) {
	if h.nodeOp == nil {
		return
	}
	// Query which proxies are installed on the remote node.
	proxyTypes, err := h.nodeOp.GetInstalledProxyTypes(ctx, serverID)
	if err != nil {
		// Fallback: try both nginx and caddy — ReloadProxy on the
		// Agent side will silently skip uninstalled/stopped proxies.
		proxyTypes = []string{"nginx", "caddy"}
	}
	for _, pt := range proxyTypes {
		if err := h.nodeOp.ControlProxy(ctx, serverID, pt, "reload"); err != nil {
			log.Printf("[WAF] remote reload %s on server %d: %v", pt, serverID, err)
		}
	}
}

// pushToRemote pushes all WAF files for serverID to the remote Agent synchronously.
// Only pushes files when at least one proxy is installed on the remote node.
// config.lua is only pushed if nginx is installed; config.json only if caddy is.
// If neither proxy is installed, the entire push is skipped — WAF rules have no
// effect on a node without a proxy.
func (h *Handler) pushToRemote(ctx context.Context, serverID int64) error {
	if h.nodeOp == nil {
		return fmt.Errorf("node operator not configured for remote server %d", serverID)
	}
	// Query which proxies are installed on the remote node.
	proxyTypes, err := h.nodeOp.GetInstalledProxyTypes(ctx, serverID)
	if err != nil {
		log.Printf("[WAF] failed to query installed proxy types for server %d: %v, pushing all files", serverID, err)
		proxyTypes = []string{"nginx", "caddy"} // fallback: push everything
	}
	// If no proxy is installed, skip the push entirely — WAF rules are useless
	// without a proxy to enforce them.
	if len(proxyTypes) == 0 {
		log.Printf("[WAF] no proxy installed on server %d, skipping WAF file push", serverID)
		return nil
	}
	files, err := h.svc.CollectAllFilesForProxyTypes(ctx, serverID, proxyTypes)
	if err != nil {
		return fmt.Errorf("collect files for server %d: %w", serverID, err)
	}
	if len(files) == 0 {
		return nil
	}
	if err := h.nodeOp.SyncWAFFiles(ctx, serverID, files); err != nil {
		return fmt.Errorf("sync to server %d: %w", serverID, err)
	}
	log.Printf("[WAF] synced %d files to server %d", len(files), serverID)
	return nil
}

// SaveDomainRuleFile handles PUT /api/v1/node-servers/:id/waf/domain-rules/:domain/:name.
// Saves the content of a rule file for a specific domain.
func (h *Handler) SaveDomainRuleFile(c *gin.Context) {
	domain := waf.StripPort(c.Param("domain"))
	name := c.Param("name")
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	var req struct {
		Content   string `json:"content"`
		MergeMode string `json:"merge_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.MergeMode == "" {
		req.MergeMode = "append"
	}
	if err := h.svc.SaveDomainRuleFileWithMode(c.Request.Context(), domain, name, req.Content, req.MergeMode, serverID); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	// Land and sync
	if err := h.landAndSync(c.Request.Context(), serverID); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "saved"})
}
