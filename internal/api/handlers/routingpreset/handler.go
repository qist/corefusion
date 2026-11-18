package routingpreset

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/warp"
)

// CoreConfigService 负责把地区规则写入核心路由段。
type CoreConfigService interface {
	ImportRegionRoute(ctx context.Context, serverID int64, coreType, region, outboundTag string) (int, error)
}

// GeoRuleService 负责确保地区规则文件已下载并推送到节点。
type GeoRuleService interface {
	EnsureRegionFiles(ctx context.Context, serverID int64, coreType, region string) error
}

// WarpService 负责按需创建 WARP/MASQUE 出站。
type WarpService interface {
	EnsureOutbound(ctx context.Context, serverID int64, coreType, mode string) (*warp.EnsureResult, error)
}

// ConfigSyncHook triggers config sync to a specific server after data changes.
type ConfigSyncHook interface {
	SyncServerConfig(ctx context.Context, serverID int64, coreType string)
}

// Handler 处理「路由地区预设导入」请求。
type Handler struct {
	coreCfg CoreConfigService
	geoRule GeoRuleService
	warp    WarpService
	sync    ConfigSyncHook
}

// NewHandler creates a new routing preset handler.
func NewHandler(coreCfg CoreConfigService, geoRule GeoRuleService, warpSvc WarpService) *Handler {
	return &Handler{coreCfg: coreCfg, geoRule: geoRule, warp: warpSvc}
}

// SetSyncHook 注入配置同步钩子：导入规则后让节点立即生成并下发新配置。
func (h *Handler) SetSyncHook(hook ConfigSyncHook) {
	h.sync = hook
}

// Import handles POST /api/v1/node-servers/:id/routing-presets/import
// body: {"core_type":"xray|singbox","region":"cn|ru|ir","outbound":"direct|warp|masque"}
func (h *Handler) Import(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || serverID <= 0 {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}
	var req struct {
		CoreType string `json:"core_type"`
		Region   string `json:"region"`
		Outbound string `json:"outbound"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	coreType := strings.ToLower(strings.TrimSpace(req.CoreType))
	if coreType != "xray" && coreType != "singbox" {
		response.BadRequest(c, "INVALID_CORE", "unsupported core type")
		return
	}
	region := strings.ToLower(strings.TrimSpace(req.Region))
	if region != "cn" && region != "ru" && region != "ir" {
		response.BadRequest(c, "INVALID_REGION", "unsupported region")
		return
	}
	outbound := strings.ToLower(strings.TrimSpace(req.Outbound))
	if outbound == "" {
		outbound = "direct"
	}
	if outbound != "direct" && outbound != "warp" && outbound != "masque" {
		response.BadRequest(c, "INVALID_OUTBOUND", "unsupported outbound")
		return
	}
	if outbound == "masque" && coreType != "xray" {
		response.BadRequest(c, "INVALID_OUTBOUND", "sing-box only supports WARP")
		return
	}

	// 1. 规则文件必须真实可用，否则写入 ext:/local rule_set 后核心会起不来。
	if h.geoRule != nil {
		if err := h.geoRule.EnsureRegionFiles(c.Request.Context(), serverID, coreType, region); err != nil {
			response.InternalError(c, "RULE_FILE_UNAVAILABLE", err.Error())
			return
		}
	}

	// 2. 按需创建 WARP/MASQUE 出站，拿到可供路由引用的 tag。
	tag := outbound
	if outbound == "warp" || outbound == "masque" {
		if h.warp == nil {
			response.InternalError(c, "NOT_CONFIGURED", "warp service not configured")
			return
		}
		res, err := h.warp.EnsureOutbound(c.Request.Context(), serverID, coreType, outbound)
		if err != nil {
			response.InternalError(c, "OUTBOUND_CREATE_FAILED", err.Error())
			return
		}
		tag = res.Tag
	}

	// 3. 写入路由段（幂等）。
	added, err := h.coreCfg.ImportRegionRoute(c.Request.Context(), serverID, coreType, region, tag)
	if err != nil {
		response.InternalError(c, "IMPORT_FAILED", err.Error())
		return
	}

	// 4. 让节点生成并应用新配置。
	if h.sync != nil {
		go h.sync.SyncServerConfig(context.Background(), serverID, coreType)
	}

	response.OK(c, gin.H{
		"region":       region,
		"core_type":    coreType,
		"outbound_tag": tag,
		"rules_added":  added,
	})
}
