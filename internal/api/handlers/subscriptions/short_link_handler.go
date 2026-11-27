package subscriptions

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	domainuser "corefusion/internal/domain/user"
	"corefusion/internal/model/entity"
	shortlinkSvc "corefusion/internal/service/shortlink"
	subService "corefusion/internal/service/subscription"
)

// ShortLinkHandler 管理后台的订阅短链接。
//
// 短链是订阅凭证（拿到即可拉取节点），读接口对 viewer 只返回脱敏 code，
// 不返回可直接使用的 URL；operator 及以上才返回完整 URL。
type ShortLinkHandler struct {
	subSvc   *subService.Service
	shortSvc *shortlinkSvc.Service
}

func NewShortLinkHandler(subSvc *subService.Service, shortSvc *shortlinkSvc.Service) *ShortLinkHandler {
	return &ShortLinkHandler{subSvc: subSvc, shortSvc: shortSvc}
}

type shortLinkItem struct {
	ID                  int64      `json:"id"`
	SubscriptionID      int64      `json:"subscription_id"`
	Code                string     `json:"code"`
	CodeMasked          string     `json:"code_masked"`
	URL                 string     `json:"url"`
	Format              string     `json:"format"`
	MihomoConfigID      *int64     `json:"mihomo_config_id"`
	ConfigName          string     `json:"config_name"`
	Platform            string     `json:"platform"`
	Providers           bool       `json:"providers"`
	LockParams          bool       `json:"lock_params"`
	Enabled             bool       `json:"enabled"`
	ExpiresAt           *time.Time `json:"expires_at"`
	RevokedAt           *time.Time `json:"revoked_at"`
	FollowTokenRotation bool       `json:"follow_token_rotation"`
	LastAccessAt        *time.Time `json:"last_access_at"`
	AccessCount         int64      `json:"access_count"`
	CreatedAt           time.Time  `json:"created_at"`
}

func (h *ShortLinkHandler) item(c *gin.Context, link *entity.SubscriptionShortLink, full bool) shortLinkItem {
	item := shortLinkItem{
		ID:                  link.ID,
		SubscriptionID:      link.SubscriptionID,
		Format:              link.Format,
		MihomoConfigID:      link.MihomoConfigID,
		ConfigName:          link.ConfigName,
		Platform:            link.Platform,
		Providers:           link.Providers,
		LockParams:          link.LockParams,
		Enabled:             link.Enabled,
		ExpiresAt:           link.ExpiresAt,
		RevokedAt:           link.RevokedAt,
		FollowTokenRotation: link.FollowTokenRotation,
		LastAccessAt:        link.LastAccessAt,
		AccessCount:         link.AccessCount,
		CreatedAt:           link.CreatedAt,
	}
	if full {
		item.Code = link.Code
		item.URL = h.shortSvc.ManualLinkURL(c.Request.Context(), link, requestDomain(c))
	} else {
		item.CodeMasked = maskShortCode(link.Code)
	}
	return item
}

func maskShortCode(code string) string {
	if len(code) <= 6 {
		return "***"
	}
	return code[:6] + "***"
}

func canSeeCredential(c *gin.Context) bool {
	return middleware.RankOf(c.GetString("role")) >= middleware.RankOf(domainuser.RoleOperator)
}

// ListBySubscription handles GET /api/v1/subscriptions/:id/short-links.
func (h *ShortLinkHandler) ListBySubscription(c *gin.Context) {
	subID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}
	if _, err := h.subSvc.GetByID(c.Request.Context(), subID); err != nil {
		response.NotFound(c, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	full := canSeeCredential(c)
	// GET 不应成为只读角色的写入口：清理无配置 mihomo 链接会删除/撤销凭证，
	// ?expand=1 会新建凭证，两者都只对 operator 及以上生效（安全审计 v8 M-1）。
	if full {
		// 无配置的 mihomo_yaml 短链打开只有 proxies（没有规则），先清理掉。
		h.shortSvc.PurgeUnconfiguredMihomo(c.Request.Context(), subID)
	}
	// expand=1：按「格式 × 配置」幂等补齐并返回完整组合，
	// 客户端详情直接用 url，不再自己拼 ?format=&config=。
	var links []entity.SubscriptionShortLink
	var listErr error
	if full && c.Query("expand") == "1" {
		links, listErr = h.shortSvc.EnsureLinks(c.Request.Context(), subID)
	} else {
		links, listErr = h.shortSvc.ListBySubscription(c.Request.Context(), subID)
	}
	if listErr != nil {
		response.InternalError(c, "LIST_SHORT_LINKS_FAILED", listErr.Error())
		return
	}
	items := make([]shortLinkItem, 0, len(links))
	for i := range links {
		items = append(items, h.item(c, &links[i], full))
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// Create handles POST /api/v1/subscriptions/:id/short-links.
func (h *ShortLinkHandler) Create(c *gin.Context) {
	subID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}
	var req struct {
		Format              string     `json:"format"`
		MihomoConfigID      *int64     `json:"mihomo_config_id"`
		ConfigName          string     `json:"config_name"`
		Platform            string     `json:"platform"`
		Providers           bool       `json:"providers"`
		LockParams          bool       `json:"lock_params"`
		FollowTokenRotation *bool      `json:"follow_token_rotation"`
		ExpiresAt           *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if _, err := h.subSvc.GetByID(c.Request.Context(), subID); err != nil {
		response.NotFound(c, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	follow := h.shortSvc.GetSettings(c.Request.Context()).FollowTokenRotation
	if req.FollowTokenRotation != nil {
		follow = *req.FollowTokenRotation
	}
	link, err := h.shortSvc.Create(c.Request.Context(), shortlinkSvc.CreateInput{
		SubscriptionID:      subID,
		Format:              req.Format,
		MihomoConfigID:      req.MihomoConfigID,
		ConfigName:          req.ConfigName,
		Platform:            req.Platform,
		Providers:           req.Providers,
		LockParams:          req.LockParams,
		FollowTokenRotation: follow,
		ExpiresAt:           req.ExpiresAt,
		CreatedBy:           c.GetInt64("user_id"),
	})
	if err != nil {
		response.BadRequest(c, "CREATE_SHORT_LINK_FAILED", err.Error())
		return
	}
	response.Created(c, h.item(c, link, true))
}

// Update handles PUT /api/v1/subscription-short-links/:id.
func (h *ShortLinkHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid short link id")
		return
	}
	link, err := h.shortSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "SHORT_LINK_NOT_FOUND", "short link not found")
		return
	}
	var req struct {
		Enabled             *bool      `json:"enabled"`
		Format              *string    `json:"format"`
		MihomoConfigID      *int64     `json:"mihomo_config_id"`
		ConfigName          *string    `json:"config_name"`
		Platform            *string    `json:"platform"`
		Providers           *bool      `json:"providers"`
		LockParams          *bool      `json:"lock_params"`
		FollowTokenRotation *bool      `json:"follow_token_rotation"`
		ExpiresAt           *time.Time `json:"expires_at"`
		ClearExpiry         bool       `json:"clear_expiry"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Enabled != nil {
		link.Enabled = *req.Enabled
	}
	if req.Format != nil {
		format, ferr := shortlinkSvc.NormalizeFormat(*req.Format)
		if ferr != nil {
			response.BadRequest(c, "INVALID_FORMAT", ferr.Error())
			return
		}
		link.Format = format
	}
	if req.MihomoConfigID != nil {
		if *req.MihomoConfigID <= 0 {
			link.MihomoConfigID = nil
		} else {
			v := *req.MihomoConfigID
			link.MihomoConfigID = &v
			// 改了 ID 就以 ID 为准；旧的名称快照会让服务端继续按旧名字解析。
			link.ConfigName = ""
		}
	}
	if req.ConfigName != nil {
		link.ConfigName = strings.TrimSpace(*req.ConfigName)
	}
	if req.Platform != nil {
		link.Platform = strings.TrimSpace(*req.Platform)
	}
	if req.Providers != nil {
		link.Providers = *req.Providers
	}
	if req.LockParams != nil {
		link.LockParams = *req.LockParams
	}
	if req.FollowTokenRotation != nil {
		link.FollowTokenRotation = *req.FollowTokenRotation
		// 从“不跟随”切到“跟随”时，把快照对齐到当前版本；
		// 否则这条链接会因为历史版本不匹配而永远 404。
		if link.FollowTokenRotation {
			if sub, serr := h.subSvc.GetByID(c.Request.Context(), link.SubscriptionID); serr == nil && sub != nil {
				link.TokenVersion = sub.TokenVersion
			}
		}
	}
	if req.ClearExpiry {
		link.ExpiresAt = nil
	} else if req.ExpiresAt != nil {
		link.ExpiresAt = req.ExpiresAt
	}
	if err := h.shortSvc.ValidateLinkConfig(c.Request.Context(), link); err != nil {
		response.BadRequest(c, "INVALID_CONFIG_TARGET", err.Error())
		return
	}
	if err := h.shortSvc.Update(c.Request.Context(), link); err != nil {
		response.InternalError(c, "UPDATE_SHORT_LINK_FAILED", err.Error())
		return
	}
	response.OK(c, h.item(c, link, true))
}

// Delete handles DELETE /api/v1/subscription-short-links/:id.
func (h *ShortLinkHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid short link id")
		return
	}
	if err := h.shortSvc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_SHORT_LINK_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// Rotate handles POST /api/v1/subscription-short-links/:id/rotate.
func (h *ShortLinkHandler) Rotate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid short link id")
		return
	}
	link, err := h.shortSvc.RotateCode(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "ROTATE_SHORT_LINK_FAILED", err.Error())
		return
	}
	response.OK(c, h.item(c, link, true))
}

// Backfill handles POST /api/v1/subscriptions/short-links/backfill.
func (h *ShortLinkHandler) Backfill(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}
	_ = c.ShouldBindJSON(&req)
	created, skipped, failed, err := h.shortSvc.Backfill(c.Request.Context(), req.DryRun)
	if err != nil {
		response.InternalError(c, "BACKFILL_SHORT_LINKS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{
		"dry_run": req.DryRun,
		"created": created,
		"skipped": skipped,
		"failed":  failed,
	})
}

// GetSettings handles GET /api/v1/system/subscription-short-link.
func (h *ShortLinkHandler) GetSettings(c *gin.Context) {
	response.OK(c, h.shortSvc.GetSettings(c.Request.Context()))
}

// UpdateSettings handles PUT /api/v1/system/subscription-short-link.
func (h *ShortLinkHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		Enabled             *bool `json:"enabled"`
		AutoCreate          *bool `json:"auto_create"`
		FollowTokenRotation *bool `json:"follow_token_rotation"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cfg := h.shortSvc.GetSettings(c.Request.Context())
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.AutoCreate != nil {
		cfg.AutoCreate = *req.AutoCreate
	}
	if req.FollowTokenRotation != nil {
		cfg.FollowTokenRotation = *req.FollowTokenRotation
	}
	if err := h.shortSvc.SaveSettings(c.Request.Context(), cfg); err != nil {
		response.InternalError(c, "SAVE_SHORT_LINK_SETTINGS_FAILED", err.Error())
		return
	}
	response.OK(c, cfg)
}
