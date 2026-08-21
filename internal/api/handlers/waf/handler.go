package waf

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

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
}

// Handler handles unified WAF HTTP requests (both global rules and per-node configs).
type Handler struct {
	svc    *waf.Service
	nodeOp NodeOperator
}

// NewHandler creates a new WAF handler.
func NewHandler(svc *waf.Service) *Handler {
	return &Handler{svc: svc}
}

// SetNodeOperator sets the node operator for remote WAF operations.
func (h *Handler) SetNodeOperator(op NodeOperator) {
	h.nodeOp = op
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
	// Land to local disk
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		log.Printf("[WAF] land rules to disk: %v", err)
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
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		log.Printf("[WAF] land rules to disk: %v", err)
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// SyncFromGitHub handles POST /api/v1/waf/sync-github.
func (h *Handler) SyncFromGitHub(c *gin.Context) {
	if err := h.svc.SyncFromGitHub(c.Request.Context()); err != nil {
		response.InternalError(c, "SYNC_FAILED", err.Error())
		return
	}
	if err := h.svc.LandAllRules(c.Request.Context()); err != nil {
		log.Printf("[WAF] land rules to disk: %v", err)
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
	response.OK(c, gin.H{"message": "synced"})
}

// --- Per-Node WAF Rules ---

// ListNodeRules handles GET /api/v1/node-servers/:id/waf/rules.
// Returns both global rules and node-level overrides for this server.
func (h *Handler) ListNodeRules(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	rules, err := h.svc.ListNodeRules(c.Request.Context(), serverID)
	if err != nil {
		response.OK(c, []interface{}{})
		return
	}
	response.OK(c, rules)
}

// GetNodeRule handles GET /api/v1/node-servers/:id/waf/rules/:name.
func (h *Handler) GetNodeRule(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	name := c.Param("name")
	rule, err := h.svc.GetNodeRule(c.Request.Context(), name, serverID)
	if err != nil {
		response.OK(c, gin.H{"name": name, "content": "", "exists": false})
		return
	}
	response.OK(c, gin.H{"name": rule.Name, "content": rule.Content, "exists": true})
}

// SaveNodeRule handles PUT /api/v1/node-servers/:id/waf/rules/:name.
// Creates or updates a node-level override for the given rule.
// Body: { "content": "...", "merge_mode": "append|replace" }
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
	response.OK(c, gin.H{"message": "saved"})
}

// DeleteNodeRule handles DELETE /api/v1/node-servers/:id/waf/rules/:name.
// Removes the node-level override, falling back to the global rule.
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
	// Land updated config to local disk
	if err := h.svc.LandRulesForServer(c.Request.Context(), serverID); err != nil {
		log.Printf("[WAF] failed to land config for server %d: %v", serverID, err)
	}
	response.OK(c, gin.H{"message": "saved"})
}

// --- Per-Node Domain Config ---

// ListDomainConfigs handles GET /api/v1/node-servers/:id/waf/domain-config.
func (h *Handler) ListDomainConfigs(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	configs, err := h.svc.ListDomainConfigs(c.Request.Context(), serverID)
	if err != nil {
		response.OK(c, []interface{}{})
		return
	}
	response.OK(c, configs)
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
	// Land updated domain config to local disk
	if err := h.svc.LandRulesForServer(c.Request.Context(), serverID); err != nil {
		log.Printf("[WAF] failed to land domain config for server %d: %v", serverID, err)
	}
	response.OK(c, gin.H{"message": "saved"})
}

// EnableDomain handles POST /api/v1/node-servers/:id/waf/enable-domain.
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
	// Save domain config with waf_enable=on
	config := fmt.Sprintf(`{"waf_enable":"on","rule_dir":"domains/%s"}`, req.Domain)
	if err := h.svc.SaveDomainConfig(c.Request.Context(), serverID, req.Domain, config); err != nil {
		response.InternalError(c, "ENABLE_FAILED", err.Error())
		return
	}
	// Land updated domain config to local disk
	if err := h.svc.LandRulesForServer(c.Request.Context(), serverID); err != nil {
		log.Printf("[WAF] failed to land domain config for server %d: %v", serverID, err)
	}
	response.OK(c, gin.H{"domain": req.Domain, "enabled": true})
}

// DisableDomain handles POST /api/v1/node-servers/:id/waf/disable-domain.
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
	// Save domain config with waf_enable=off
	config := `{"waf_enable":"off"}`
	if err := h.svc.SaveDomainConfig(c.Request.Context(), serverID, req.Domain, config); err != nil {
		response.InternalError(c, "DISABLE_FAILED", err.Error())
		return
	}
	// Land updated domain config to local disk
	if err := h.svc.LandRulesForServer(c.Request.Context(), serverID); err != nil {
		log.Printf("[WAF] failed to land domain config for server %d: %v", serverID, err)
	}
	response.OK(c, gin.H{"domain": req.Domain, "enabled": false})
}

// GetDomainStatus handles GET /api/v1/node-servers/:id/waf/domain-status.
func (h *Handler) GetDomainStatus(c *gin.Context) {
	serverID := parseServerID(c)
	if serverID == 0 {
		serverID = localServerID
	}
	domain := c.Query("domain")
	config, err := h.svc.GetDomainConfig(c.Request.Context(), serverID, domain)
	if err != nil {
		response.OK(c, gin.H{"domain": domain, "enabled": false})
		return
	}
	// Parse config JSON to check waf_enable
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		response.OK(c, gin.H{"domain": domain, "enabled": false})
		return
	}
	if wafEnable, ok := cfg["waf_enable"].(string); ok && wafEnable == "off" {
		response.OK(c, gin.H{"domain": domain, "enabled": false})
		return
	}
	response.OK(c, gin.H{"domain": domain, "enabled": true})
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
	response.OK(c, gin.H{"enabled": req.Enabled})
}
