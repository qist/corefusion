package clientauth

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/service/audit"
	"corefusion/internal/service/auth"
	"corefusion/internal/service/clientauth"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// RateLimitPusher pushes IP association to rate limit agents.
type RateLimitPusher interface {
	PushIPAssociation(ctx context.Context, serverID int64, ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) error
}

// Handler handles client authentication HTTP requests.
type Handler struct {
	svc             *clientauth.Service
	rateLimitPusher RateLimitPusher
	nodeServerRepo  *repository.NodeServerRepository
	clientRepo      *repository.ClientRepository
	planTmplRepo    *repository.PlanTemplateRepository
}

// NewHandler creates a new client auth handler.
func NewHandler(svc *clientauth.Service) *Handler {
	return &Handler{svc: svc}
}

// SetRateLimitPusher injects the rate limit pusher for IP association on login.
func (h *Handler) SetRateLimitPusher(pusher RateLimitPusher, nodeServerRepo *repository.NodeServerRepository, clientRepo *repository.ClientRepository, planTmplRepo *repository.PlanTemplateRepository) {
	h.rateLimitPusher = pusher
	h.nodeServerRepo = nodeServerRepo
	h.clientRepo = clientRepo
	h.planTmplRepo = planTmplRepo
}

// Register handles POST /api/v1/client-auth/register.
func (h *Handler) Register(c *gin.Context) {
	var input clientauth.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.svc.Register(c.Request.Context(), input)
	if err != nil {
		if err.Error() == "registration is closed" {
			response.Forbidden(c, "REGISTRATION_CLOSED", err.Error())
			return
		}
		if err.Error() == "email already registered" {
			response.Conflict(c, "EMAIL_EXISTS", err.Error())
			return
		}
		response.InternalError(c, "REGISTER_FAILED", err.Error())
		return
	}

	// Get client info
	clientInfo, _ := h.svc.GetMe(c.Request.Context(), result.Client.ID)
	result.Client = clientInfo

	response.Created(c, result)
}

// Login handles POST /api/v1/client-auth/login.
func (h *Handler) Login(c *gin.Context) {
	var input clientauth.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.svc.Login(c.Request.Context(), input)
	if err != nil {
		middleware.DefaultClientLoginLimiter().RecordFailure(netutil.GetRealIP(c.Request))
		audit.Record(audit.Event{
			Category: "auth", EventType: "client_login", Result: "failure",
			RemoteIP: netutil.GetRealIP(c.Request), Reason: err.Error(),
			Metadata: map[string]any{"email": input.Email},
		})
		response.Unauthorized(c, "INVALID_CREDENTIALS", err.Error())
		return
	}
	middleware.DefaultClientLoginLimiter().RecordSuccess(netutil.GetRealIP(c.Request))
	audit.Record(audit.Event{
		Category: "auth", EventType: "client_login", Result: "success",
		UserID: result.Client.ID, RemoteIP: netutil.GetRealIP(c.Request),
		Metadata: map[string]any{"email": result.Client.Email},
	})

	// Get client info
	clientInfo, _ := h.svc.GetMe(c.Request.Context(), result.Client.ID)
	result.Client = clientInfo

	// Push rate limit IP association on login (P0 trigger source)
	if h.rateLimitPusher != nil && result.Client.ID > 0 {
		clientIP := netutil.GetRealIP(c.Request)
		go h.pushRateLimitOnLogin(clientIP, result.Client.ID)
	}

	response.OK(c, result)
}

// Refresh handles POST /api/v1/client-auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := h.svc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, "INVALID_TOKEN", err.Error())
		return
	}

	clientInfo, _ := h.svc.GetMe(c.Request.Context(), result.Client.ID)
	result.Client = clientInfo

	response.OK(c, result)
}

// GetMe handles GET /api/v1/client-auth/me.
func (h *Handler) GetMe(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	info, err := h.svc.GetMe(c.Request.Context(), clientID)
	if err != nil {
		response.NotFound(c, "CLIENT_NOT_FOUND", err.Error())
		return
	}

	response.OK(c, info)
}

// RotateToken handles POST /api/v1/client-auth/rotate-token.
func (h *Handler) RotateToken(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	newToken, err := h.svc.RotateToken(c.Request.Context(), clientID)
	if err != nil {
		response.InternalError(c, "ROTATE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"token": newToken, "message": "token rotated"})
}

// UpdateSettings handles PUT /api/v1/client-auth/settings.
func (h *Handler) UpdateSettings(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	var req struct {
		IPWhitelist string `json:"ip_whitelist"`
		DeviceLimit int    `json:"device_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.UpdateSettings(c.Request.Context(), clientID, req.IPWhitelist, req.DeviceLimit); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "settings updated"})
}

// ChangePassword handles POST /api/v1/client-auth/change-password.
func (h *Handler) ChangePassword(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=72"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), clientID, req.OldPassword, req.NewPassword); err != nil {
		response.BadRequest(c, "CHANGE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "password changed"})
}

// GetTrafficHistory handles GET /api/v1/client-auth/traffic-history.
// Returns the authenticated client's monthly traffic usage trend, aggregated across
// the billing nodes of their subscription. Optional ?months=N (1-24, default 6).
func (h *Handler) GetTrafficHistory(c *gin.Context) {
	clientID := getClientID(c)
	if clientID == 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "client not authenticated")
		return
	}
	months := 6
	if m := c.Query("months"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 && v <= 24 {
			months = v
		}
	}
	history, err := h.svc.GetTrafficHistory(c.Request.Context(), clientID, months)
	if err != nil {
		response.InternalError(c, "TRAFFIC_HISTORY_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"months": months, "history": history})
}

// getClientID extracts client ID from Gin context (set by middleware).
func getClientID(c *gin.Context) int64 {
	if id, exists := c.Get("client_id"); exists {
		if cid, ok := id.(int64); ok {
			return cid
		}
	}
	return 0
}

// pushRateLimitOnLogin pushes IP association to all nodes when a client logs in.
func (h *Handler) pushRateLimitOnLogin(clientIP string, clientID int64) {
	if clientIP == "" || h.clientRepo == nil || h.nodeServerRepo == nil {
		return
	}
	ctx := context.Background()

	cl, err := h.clientRepo.GetByID(ctx, clientID)
	if err != nil || cl.SubscriptionID == nil || *cl.SubscriptionID == 0 {
		return
	}
	subID := *cl.SubscriptionID

	// Resolve limits (speed, concurrent connections, device count)
	var maxDownMbps, maxUpMbps, connLimit, deviceLimit int32
	if cl.SpeedLimitMbps > 0 {
		maxDownMbps = int32(cl.SpeedLimitMbps)
	}
	if cl.SpeedLimitUpMbps > 0 {
		maxUpMbps = int32(cl.SpeedLimitUpMbps)
	}
	if cl.ConcurrentLimit > 0 {
		connLimit = int32(cl.ConcurrentLimit)
	}
	if cl.DeviceLimit > 0 {
		deviceLimit = int32(cl.DeviceLimit)
	}
	if maxDownMbps == 0 || maxUpMbps == 0 || connLimit == 0 || deviceLimit == 0 {
		if cl.PlanID > 0 && h.planTmplRepo != nil {
			if tmpl, err := h.planTmplRepo.GetByID(ctx, cl.PlanID); err == nil {
				if maxDownMbps == 0 && tmpl.SpeedLimitMbps > 0 {
					maxDownMbps = int32(tmpl.SpeedLimitMbps)
				}
				if maxUpMbps == 0 && tmpl.SpeedLimitUpMbps > 0 {
					maxUpMbps = int32(tmpl.SpeedLimitUpMbps)
				}
				if connLimit == 0 && tmpl.ConcurrentLimit > 0 {
					connLimit = int32(tmpl.ConcurrentLimit)
				}
				if deviceLimit == 0 && tmpl.DeviceLimit > 0 {
					deviceLimit = int32(tmpl.DeviceLimit)
				}
			}
		}
	}

	servers, err := h.nodeServerRepo.ListEnabled(ctx)
	if err != nil {
		return
	}
	for _, server := range servers {
		if server.ID <= 0 {
			continue
		}
		if err := h.rateLimitPusher.PushIPAssociation(ctx, server.ID, clientIP, subID, maxDownMbps, maxUpMbps, connLimit, deviceLimit); err != nil {
			log.Printf("clientauth: rate limit push failed for server %d: %v", server.ID, err)
		}
	}
}

// Logout 让当前客户端的全部 token 立即失效（令牌版本 +1）。
//
// POST /api/v1/client-auth/logout（需已认证）
// ⚠ 客户端侧以前**没有**吊销机制 ✗：门户"退出登录"只清前端本地存储，服务端照旧认那个 token
// （被窃取的 access/refresh 在有效期内仍可用）——审计 v5 M8 只补了管理端，本轮补客户端侧。
func (h *Handler) Logout(c *gin.Context) {
	clientID := c.GetInt64("client_id")
	if clientID <= 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "invalid client session")
		return
	}
	if err := h.svc.Logout(c.Request.Context(), clientID); err != nil {
		response.InternalError(c, "LOGOUT_FAILED", "failed to revoke tokens")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.OK(c, gin.H{"message": "logged out"})
}

// IssueTicket 签发客户端一次性短时票据（安全审计 v5 M10）。
//
// POST /api/v1/client-auth/ticket?scope=client_download（需已认证）
//
// 供门户里的"下载日志包"（`window.open`，无法自定义请求头）使用：票据 60 秒过期、
// 一次性、且只能用在客户端日志包下载路径上 ✓。
func (h *Handler) IssueTicket(c *gin.Context) {
	store := middleware.TicketStore()
	if store == nil {
		response.InternalError(c, "TICKET_NOT_CONFIGURED", "ticket auth not configured")
		return
	}

	scope := c.DefaultQuery("scope", auth.TicketScopeClientDownload)
	if scope != auth.TicketScopeClientDownload {
		response.BadRequest(c, "INVALID_SCOPE", "unsupported ticket scope")
		return
	}

	clientID := c.GetInt64("client_id")
	if clientID <= 0 {
		response.Unauthorized(c, "UNAUTHORIZED", "invalid client session")
		return
	}

	ticket, expires, err := store.Issue(auth.TicketOptions{
		Scope:    scope,
		Kind:     auth.TicketKindClient,
		ClientID: clientID,
	})
	if err != nil {
		response.InternalError(c, "TICKET_ISSUE_FAILED", "failed to issue ticket")
		return
	}

	c.Header("Cache-Control", "no-store")
	response.OK(c, gin.H{
		"ticket":     ticket,
		"expires_in": int(time.Until(expires).Seconds()),
		"scope":      scope,
	})
}

// ClientAuthMiddleware creates a middleware for client JWT authentication.
//
// 支持两种凭证（安全审计 v5 M10）：
//  1. `Authorization: Bearer <access token>` —— 常规请求；
//  2. `?ticket=<一次性短时票据>` —— **仅**日志包下载这种浏览器无法自定义请求头的场景
//     （`window.open` 跳转）。作用域与路径都在 middleware 的路径表里限定 ✓。
//
// ⚠ 以前客户端下载是直接 `window.open(...?token=<长期 JWT>)` ✗：URL 会残留在访问日志、
// 浏览器历史、Referer 里，等于把客户端凭证散播出去 ✗；而且客户端中间件**只读 Header** ✗，
// 所以那个下载入口实际上一直是 401（顺带修好 ✓）。
func ClientAuthMiddleware(svc *clientauth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			if ticket := c.Query("ticket"); ticket != "" {
				store := middleware.TicketStore()
				scope := middleware.TicketScopeForPath(c.Request.URL.Path)
				if store == nil || scope != auth.TicketScopeClientDownload {
					c.JSON(http.StatusUnauthorized, gin.H{"code": "TICKET_NOT_ALLOWED",
						"message": "one-time tickets are not accepted on this endpoint"})
					c.Abort()
					return
				}
				t, ok := store.Consume(ticket, scope)
				if !ok || t.Kind != auth.TicketKindClient || t.ClientID <= 0 {
					c.JSON(http.StatusUnauthorized, gin.H{"code": "TICKET_INVALID",
						"message": "ticket is invalid, already used or expired; request a new one"})
					c.Abort()
					return
				}
				c.Set("client_id", t.ClientID)
				c.Next()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "invalid authorization format"})
			c.Abort()
			return
		}

		// ⚠ 必须用**严格**校验：客户端与管理端共用签名密钥，普通 ValidateAccessToken 会
		// 放行管理端 token（其 user_id 被当成 client_id）✗ → 管理端身份就能冒充任意客户端。
		clientID, err := svc.ValidateClientAccessToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("client_id", clientID)
		c.Next()
	}
}
