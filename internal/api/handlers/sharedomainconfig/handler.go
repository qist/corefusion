package sharedomainconfig

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	certservice "corefusion/internal/service/certificate"
	"corefusion/internal/service/dnsrecord"
	"corefusion/internal/service/sharedomainconfig"
)

// Handler handles share domain config HTTP requests.
type Handler struct {
	svc *sharedomainconfig.Service
}

// NewHandler creates a new share domain config handler.
func NewHandler(svc *sharedomainconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// Providers 返回各 DNS 服务商的「自动解析写入」能力，前端据此决定下拉能否选。
//
// 说明：证书 DNS-01 支持 ≠ 这里支持 —— lego 只提供 TXT 的 Present/CleanUp，
// 没有通用记录 CRUD，所以每家都要单独实现（见 dnsrecord 注册表）。
func (h *Handler) Providers(c *gin.Context) {
	known := make([]string, 0, 8)
	for p := range certservice.SupportedDNSProviders() {
		known = append(known, p)
	}
	response.OK(c, dnsrecord.Capabilities(known))
}

// List handles GET /api/v1/share-domains.
// Returns all share domain configs (optionally filtered by type).
func (h *Handler) List(c *gin.Context) {
	configType := c.Query("type")
	// 分享域名是配置类数据（几十条），必须全量返回：
	// 旧实现走默认分页（DefaultPageSize=20）+ ORDER BY sort_order ASC，
	// 历史 cf_ip 行 sort_order=0 会排在前面占满首页，前端过滤掉 cf_ip 后
	// 一条 cf_domain 都看不到（116 上“重置后域名不全”的根因）。
	configs, err := h.svc.List(c.Request.Context(), configType)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, configs, int64(len(configs)), len(configs), 0)
}

// ListByServer handles GET /api/v1/node-servers/:id/share-domains.
// Returns direct share domain configs effective for a specific server
// (both global server_id=0 and server-specific server_id=serverID).
func (h *Handler) ListByServer(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}
	configs, err := h.svc.ListByServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// Create handles POST /api/v1/share-domains and
// POST /api/v1/node-servers/:id/share-domains.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Type        string  `json:"type" binding:"required"`
		Value       string  `json:"value"`
		Port        int     `json:"port"`
		Ports       string  `json:"ports"`
		ServerID    int64   `json:"server_id"`
		ServerIDs   []int64 `json:"server_ids"` // 多节点绑定；空 = 全局
		TLSMode     *string `json:"tls_mode"`
		SNI         *string `json:"sni"`
		ALPN        *string `json:"alpn"`
		Fingerprint *string `json:"fingerprint"`
		// 主备入口漂移（P2）：A 记录指向当前 primary
		DNSChannelID  *int64  `json:"dns_channel_id"`
		DNSRecordName *string `json:"dns_record_name"`
		DNSProxied    *bool   `json:"dns_proxied"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// :id param takes priority over body server_id
	serverID := req.ServerID
	if idParam := c.Param("id"); idParam != "" {
		if parsed, err := strconv.ParseInt(idParam, 10, 64); err == nil {
			serverID = parsed
		}
	}

	config, err := h.svc.Create(c.Request.Context(), serverID, req.Type, req.Value, req.Port, req.Ports)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	if req.ServerIDs != nil {
		applyServerIDs(config, req.ServerIDs)
		if err := h.svc.Save(c.Request.Context(), config); err != nil {
			response.InternalError(c, "CREATE_FAILED", err.Error())
			return
		}
	}
	if applyTLSFields(config, req.TLSMode, req.SNI, req.ALPN, req.Fingerprint) {
		config.UpdatedAt = time.Now()
		if err := h.svc.Save(c.Request.Context(), config); err != nil {
			response.InternalError(c, "CREATE_FAILED", err.Error())
			return
		}
	}
	if applyDNSFields(config, req.DNSChannelID, req.DNSRecordName, req.DNSProxied) {
		config.UpdatedAt = time.Now()
		if err := h.svc.Save(c.Request.Context(), config); err != nil {
			response.InternalError(c, "CREATE_FAILED", err.Error())
			return
		}
	}
	response.OK(c, config)
}

// applyTLSFields 写入「前置 TLS 终结」的旧版兜底配置（分享域名条目级）。
//
// 新版本已把 TLS 配置移到「入口节点 → 入站 → 访问域名」，这里只为兼容历史数据：
// 只有显式传了字段（非 nil）才修改，避免普通编辑（不带这些字段）把已有值清空。
func applyTLSFields(config *entity.ShareDomainConfig, mode, sni, alpn, fp *string) bool {
	if config == nil {
		return false
	}
	changed := false
	if mode != nil && config.TLSMode != *mode {
		config.TLSMode = *mode
		changed = true
	}
	if sni != nil && config.SNI != *sni {
		config.SNI = *sni
		changed = true
	}
	if alpn != nil && config.ALPN != *alpn {
		config.ALPN = *alpn
		changed = true
	}
	if fp != nil && config.Fingerprint != *fp {
		config.Fingerprint = *fp
		changed = true
	}
	return changed
}

// applyDNSFields 保存主备入口漂移配置（仅 direct 域名有意义）。
func applyDNSFields(config *entity.ShareDomainConfig, channelID *int64, recordName *string, proxied *bool) bool {
	if config == nil {
		return false
	}
	changed := false
	if channelID != nil && config.DNSChannelID != *channelID {
		config.DNSChannelID = *channelID
		changed = true
	}
	if recordName != nil && config.DNSRecordName != *recordName {
		config.DNSRecordName = *recordName
		changed = true
	}
	if proxied != nil && config.DNSProxied != *proxied {
		config.DNSProxied = *proxied
		changed = true
	}
	return changed
}

// applyServerIDs 把多节点绑定列表写入实体（JSON 文本）；同时把 ServerID 同步为
// 第一个节点（旧语义字段兼容：单节点时非 0，多节点/全局时为 0）。
func applyServerIDs(config *entity.ShareDomainConfig, ids []int64) {
	if len(ids) == 0 {
		config.ServerIDs = ""
		return
	}
	if b, err := json.Marshal(ids); err == nil {
		config.ServerIDs = string(b)
	}
	config.ServerID = 0
	if len(ids) == 1 {
		config.ServerID = ids[0]
	}
}

// Update handles PUT /api/v1/share-domains/:id and
// PUT /api/v1/node-servers/:id/share-domains/:domainId.
func (h *Handler) Update(c *gin.Context) {
	idStr := c.Param("domainId")
	if idStr == "" {
		idStr = c.Param("id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Type        string  `json:"type"`
		Value       string  `json:"value"`
		Port        int     `json:"port"`
		Ports       string  `json:"ports"`
		Enabled     *bool   `json:"enabled"`
		ServerID    *int64  `json:"server_id"`
		ServerIDs   []int64 `json:"server_ids"` // 多节点绑定；空数组 = 清除多选（回退 server_id 语义）
		TLSMode     *string `json:"tls_mode"`
		SNI         *string `json:"sni"`
		ALPN        *string `json:"alpn"`
		Fingerprint *string `json:"fingerprint"`
		// 主备入口漂移（P2）：A 记录指向当前 primary
		DNSChannelID  *int64  `json:"dns_channel_id"`
		DNSRecordName *string `json:"dns_record_name"`
		DNSProxied    *bool   `json:"dns_proxied"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	config, err := h.svc.Update(c.Request.Context(), id, req.Type, req.Value, req.Port, req.Ports, enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	// Update server_id binding if provided
	if req.ServerID != nil {
		config.ServerID = *req.ServerID
		config.UpdatedAt = time.Now()
		_ = h.svc.Save(c.Request.Context(), config)
	}
	// 多节点绑定：显式传 server_ids 字段时以它为准
	if req.ServerIDs != nil {
		applyServerIDs(config, req.ServerIDs)
		config.UpdatedAt = time.Now()
		_ = h.svc.Save(c.Request.Context(), config)
	}
	// 前置 TLS 终结配置（CF 优选 / nginx / caddy）
	if applyTLSFields(config, req.TLSMode, req.SNI, req.ALPN, req.Fingerprint) {
		config.UpdatedAt = time.Now()
		_ = h.svc.Save(c.Request.Context(), config)
	}
	// 主备入口漂移配置（只有 direct 域名会用到）
	if applyDNSFields(config, req.DNSChannelID, req.DNSRecordName, req.DNSProxied) {
		config.UpdatedAt = time.Now()
		_ = h.svc.Save(c.Request.Context(), config)
	}
	response.OK(c, config)
}

// Delete handles DELETE /api/v1/share-domains/:id and
// DELETE /api/v1/node-servers/:id/share-domains/:domainId.
func (h *Handler) Delete(c *gin.Context) {
	idStr := c.Param("domainId")
	if idStr == "" {
		idStr = c.Param("id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ClearAll handles DELETE /api/v1/share-domains/all and
// DELETE /api/v1/node-servers/:id/share-domains/all.
func (h *Handler) ClearAll(c *gin.Context) {
	configType := c.Query("type")

	if idParam := c.Param("id"); idParam != "" {
		if serverID, err := strconv.ParseInt(idParam, 10, 64); err == nil {
			if err := h.svc.ClearByServer(c.Request.Context(), serverID, configType); err != nil {
				response.InternalError(c, "CLEAR_FAILED", err.Error())
				return
			}
			response.OK(c, gin.H{"message": "cleared"})
			return
		}
	}

	if err := h.svc.ClearAll(c.Request.Context(), configType); err != nil {
		response.InternalError(c, "CLEAR_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "cleared"})
}

// SeedDefaults handles POST /api/v1/share-domains/seed.
// Seeds default CF domain configs globally (server_id=0).
func (h *Handler) SeedDefaults(c *gin.Context) {
	if err := h.svc.SeedDefaults(c.Request.Context()); err != nil {
		response.InternalError(c, "SEED_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "seeded"})
}
