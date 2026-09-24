package auth

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/auth/password"
	"corefusion/internal/service/audit"
	"corefusion/internal/service/auth"
	"corefusion/internal/service/twofa"
	"corefusion/pkg/netutil"
)

// Handler handles auth HTTP requests.
type Handler struct {
	authSvc  *auth.Service
	twoFASvc *twofa.Service
}

// NewHandler creates a new auth handler.
func NewHandler(authSvc *auth.Service, twoFASvc *twofa.Service) *Handler {
	return &Handler{authSvc: authSvc, twoFASvc: twoFASvc}
}

// loginRequest 统一一步登录：`code` 既可以是静态密码（未启用 2FA），
// 也可以是 TOTP 动态码 / 恢复码（已启用 2FA）—— 由后端按用户状态识别。
// `password` 字段保留是为了兼容旧调用方（与 code 等价）。
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Code     string `json:"code"`
	Password string `json:"password"` // 兼容旧客户端：与 code 等价
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// loginFailed 统一处理登录失败：计入限流、记审计、返回 401。
func (h *Handler) loginFailed(c *gin.Context, username, reason string) {
	middleware.DefaultLoginLimiter().RecordFailure(netutil.GetRealIP(c.Request))
	audit.Record(audit.Event{
		Category: "auth", EventType: "admin_login", Result: "failure",
		RemoteIP: c.ClientIP(), Reason: reason,
		Metadata: map[string]any{"username": username},
	})
	response.Unauthorized(c, "INVALID_CREDENTIALS", "invalid username or credential")
}

// Login handles POST /api/v1/auth/login（一步登录）。
//
// 逻辑：按用户名查用户后自动识别凭据类型——
//   - 用户已启用 2FA：`code` 必须是 TOTP 动态码 / 恢复码（静态密码不再作为登录凭据）；
//   - 用户未启用 2FA：`code` 是静态密码。
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	code := req.Code
	if code == "" {
		code = req.Password // 兼容旧客户端
	}
	if strings.TrimSpace(code) == "" {
		response.BadRequest(c, "INVALID_REQUEST", "username and credential are required")
		return
	}
	ctx := c.Request.Context()

	user, err := h.authSvc.GetActiveUserByUsername(ctx, req.Username)
	if err != nil {
		h.loginFailed(c, req.Username, err.Error())
		return
	}

	if user.TOTPEnabled {
		// 2FA 用户：只验证 TOTP 动态码 / 恢复码，忽略静态密码。
		if verr := h.twoFASvc.VerifyLoginCode(ctx, user, code); verr != nil {
			h.loginFailed(c, req.Username, "2FA: "+verr.Error())
			return
		}
	} else if !password.Check(code, user.PasswordHash) {
		h.loginFailed(c, req.Username, auth.ErrInvalidCredentials.Error())
		return
	}

	accessToken, refreshToken, err := h.authSvc.IssueTokens(ctx, user)
	if err != nil {
		response.InternalError(c, "TOKEN_ISSUE_FAILED", err.Error())
		return
	}

	middleware.DefaultLoginLimiter().RecordSuccess(netutil.GetRealIP(c.Request))
	via := "password"
	if user.TOTPEnabled {
		via = "totp"
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "admin_login", Result: "success",
		UserID: user.ID, RemoteIP: c.ClientIP(),
		Metadata: map[string]any{"username": user.Username, "via": via},
	})

	response.OK(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// Login2FAStatus handles GET /api/v1/auth/login/2fa-status?username=xxx（公开）。
// 登录页据此把输入框切换为"密码"或"动态码/恢复码"模式。
// 用户不存在 / 任何异常均返回 enabled=false（不区分），避免用户名枚举。
func (h *Handler) Login2FAStatus(c *gin.Context) {
	username := strings.TrimSpace(c.Query("username"))
	if username == "" {
		response.BadRequest(c, "INVALID_REQUEST", "username is required")
		return
	}
	enabled := false
	if user, err := h.authSvc.GetActiveUserByUsername(c.Request.Context(), username); err == nil {
		enabled = user.TOTPEnabled
	}
	response.OK(c, gin.H{"enabled": enabled})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	accessToken, refreshToken, err := h.authSvc.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Unauthorized(c, "INVALID_TOKEN", err.Error())
		return
	}

	response.OK(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// Logout handles POST /api/v1/auth/logout.
//
// 会把该账号的令牌版本 +1 → **此前签发的所有 token（含 refresh）立即失效**。
// 也就是说登出等于"登出所有设备"；这是"token 被窃取后能作废"的最小实现（审计 v5 M8）。
func (h *Handler) Logout(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID > 0 {
		if err := h.authSvc.Logout(c.Request.Context(), userID); err != nil {
			log.Printf("[auth] logout: revoke tokens for user %d failed: %v", userID, err)
			response.InternalError(c, "LOGOUT_FAILED", "failed to revoke tokens")
			return
		}
	}
	response.OK(c, gin.H{"message": "logged out"})
}

type ticketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
	Scope     string `json:"scope"`
}

// IssueTicket 签发一张一次性短时票据（安全审计 v5 M10）。
//
// POST /api/v1/auth/ticket?scope=logs|backup_download（需已认证，走正常 Bearer 头）
//
// 供**无法自定义请求头**的浏览器场景使用：日志 SSE 流（EventSource）与文件下载
// （window.open）。前端拿到票据后拼 `?ticket=`，票据 60 秒过期且只能用一次 ——
// 即使 URL 泄漏（日志/历史/Referer），泄漏的也只是一张已作废的票据 ✗。
func (h *Handler) IssueTicket(c *gin.Context) {
	store := middleware.TicketStore()
	if store == nil {
		response.InternalError(c, "TICKET_NOT_CONFIGURED", "ticket auth not configured")
		return
	}

	scope := c.Query("scope")
	if scope == "" {
		scope = auth.TicketScopeLogs
	}
	switch scope {
	case auth.TicketScopeLogs, auth.TicketScopeBackupDownload, auth.TicketScopeCertDownload:
	default:
		response.BadRequest(c, "INVALID_SCOPE", "unsupported ticket scope")
		return
	}

	userID := c.GetInt64("user_id")
	tv, err := middleware.CurrentTokenVersion(c.Request.Context(), userID)
	if err != nil {
		response.Unauthorized(c, "INVALID_TOKEN", "token subject not found")
		return
	}

	ticket, expires, err := store.Issue(auth.TicketOptions{
		Scope:        scope,
		Kind:         auth.TicketKindAdmin,
		UserID:       userID,
		Username:     c.GetString("username"),
		Role:         c.GetString("role"),
		TokenVersion: tv,
	})
	if err != nil {
		response.InternalError(c, "TICKET_ISSUE_FAILED", "failed to issue ticket")
		return
	}

	c.Header("Cache-Control", "no-store")
	response.OK(c, ticketResponse{
		Ticket:    ticket,
		ExpiresIn: int(time.Until(expires).Seconds()),
		Scope:     scope,
	})
}

// Me handles GET /api/v1/auth/me.
func (h *Handler) Me(c *gin.Context) {
	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}

	response.OK(c, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"status":     user.Status,
		"created_at": user.CreatedAt,
	})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// ChangePassword handles POST /api/v1/auth/change-password.
func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	userID := c.GetInt64("user_id")
	if err := h.authSvc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		response.BadRequest(c, "CHANGE_PASSWORD_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "password changed"})
}
