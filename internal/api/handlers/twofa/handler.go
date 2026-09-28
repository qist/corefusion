package twofa

import (
	"errors"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/auth/password"
	"corefusion/internal/security/crypto"
	"corefusion/internal/service/audit"
	"corefusion/internal/service/auth"
	"corefusion/internal/service/twofa"
	"corefusion/pkg/netutil"
)

// Handler 处理 2FA（TOTP）的 HTTP 请求。
type Handler struct {
	svc     *twofa.Service
	authSvc *auth.Service
}

// NewHandler 创建 2FA 处理器。
func NewHandler(svc *twofa.Service, authSvc *auth.Service) *Handler {
	return &Handler{svc: svc, authSvc: authSvc}
}

type login2FARequest struct {
	LoginToken string `json:"login_token" binding:"required"`
	Code       string `json:"code" binding:"required"`
}

// Login2FA handles POST /api/v1/auth/login/2fa（公开）。
// 密码验证通过且用户已启用 2FA 时，前端凭登录票据提交 TOTP 动态码/恢复码换取正式 token。
func (h *Handler) Login2FA(c *gin.Context) {
	var req login2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// 先 peek（不消费）：动态码输错时票据保留，用户可直接重试，无需重新输密码。
	pending, ok := h.svc.PeekPendingLogin(req.LoginToken)
	if !ok {
		// 票据不存在/过期 —— 需重新走密码登录
		middleware.DefaultLoginLimiter().RecordFailure(netutil.GetRealIP(c.Request))
		response.Unauthorized(c, "PENDING_LOGIN_EXPIRED", "login session expired, sign in again")
		return
	}
	user, err := h.authSvc.GetByID(c.Request.Context(), pending.UserID)
	if err != nil || user.Status != 1 {
		response.Unauthorized(c, "USER_NOT_FOUND", "user not found")
		return
	}
	if err := h.svc.VerifyLoginCode(c.Request.Context(), user, req.Code); err != nil {
		middleware.DefaultLoginLimiter().RecordFailure(netutil.GetRealIP(c.Request))
		audit.Record(audit.Event{
			Category: "auth", EventType: "twofa_login", Result: "failure",
			UserID: user.ID, RemoteIP: c.ClientIP(),
			Reason: err.Error(),
		})
		response.Unauthorized(c, "TWOFA_CODE_INVALID", "invalid verification code")
		return
	}
	// 验证通过才消费票据（并发下两个请求同时验证成功，只有一人能取走）。
	if _, ok := h.svc.TakePendingLogin(req.LoginToken); !ok {
		response.Unauthorized(c, "PENDING_LOGIN_EXPIRED", "login session expired, sign in again")
		return
	}
	accessToken, refreshToken, err := h.authSvc.IssueTokens(c.Request.Context(), user)
	if err != nil {
		response.InternalError(c, "TOKEN_ISSUE_FAILED", err.Error())
		return
	}
	middleware.DefaultLoginLimiter().RecordSuccess(netutil.GetRealIP(c.Request))
	audit.Record(audit.Event{
		Category: "auth", EventType: "twofa_login", Result: "success",
		UserID: user.ID, RemoteIP: c.ClientIP(),
		Metadata: map[string]any{"username": user.Username},
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

// Status handles GET /api/v1/auth/2fa/status（需已登录）。
func (h *Handler) Status(c *gin.Context) {
	userID := c.GetInt64("user_id")
	enabled, unused, err := h.svc.Status(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"enabled": enabled, "recovery_codes": unused})
}

type setupRequest struct {
	Password string `json:"password" binding:"required"` // 必须输入当前密码确认身份
}

// Setup handles POST /api/v1/auth/2fa/setup（需已登录）。
// 生成新的 TOTP 密钥并**加密落库（待启用）**，返回 otpauth URI 与一次性明文 secret。
func (h *Handler) Setup(c *gin.Context) {
	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}
	if !password.Check(req.Password, user.PasswordHash) {
		response.Unauthorized(c, "PASSWORD_MISMATCH", "current password is incorrect")
		return
	}
	secret, otpauthURI, err := h.svc.BeginSetup(c.Request.Context(), user)
	if err != nil {
		response.BadRequest(c, "SETUP_FAILED", err.Error())
		return
	}
	// 加密落库：Enable 时从库里解密校验动态码（中途刷新/掉线也能继续绑定）
	enc, err := crypto.EncryptSecret(secret)
	if err != nil {
		response.InternalError(c, "ENCRYPT_FAILED", err.Error())
		return
	}
	if err := h.authSvc.SetTOTPSecret(c.Request.Context(), userID, enc); err != nil {
		response.InternalError(c, "SAVE_FAILED", err.Error())
		return
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "twofa_setup", Result: "success",
		UserID: userID, RemoteIP: c.ClientIP(),
	})
	response.OK(c, gin.H{"secret": secret, "otpauth_url": otpauthURI})
}

type codeRequest struct {
	Code string `json:"code" binding:"required"`
}

// Enable handles POST /api/v1/auth/2fa/enable（需已登录）。
// 用 TOTP 动态码完成绑定，启用 2FA，返回一次性恢复码明文。
func (h *Handler) Enable(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}
	recovery, err := h.svc.Enable(c.Request.Context(), user, req.Code)
	if err != nil {
		if errors.Is(err, twofa.ErrCodeInvalid) {
			response.BadRequest(c, "TWOFA_CODE_INVALID", err.Error())
			return
		}
		response.BadRequest(c, "ENABLE_FAILED", err.Error())
		return
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "twofa_enable", Result: "success",
		UserID: userID, RemoteIP: c.ClientIP(),
	})
	response.OK(c, gin.H{"recovery_codes": recovery})
}

// Disable handles POST /api/v1/auth/2fa/disable（需已登录）。
// 必须再次验证 TOTP 动态码（防已登录会话被越权关闭 2FA）。
func (h *Handler) Disable(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}
	if err := h.svc.VerifyManagedCode(c.Request.Context(), user, req.Code); err != nil {
		response.BadRequest(c, "TWOFA_CODE_INVALID", err.Error())
		return
	}
	if err := h.svc.Disable(c.Request.Context(), user); err != nil {
		response.BadRequest(c, "DISABLE_FAILED", err.Error())
		return
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "twofa_disable", Result: "success",
		UserID: userID, RemoteIP: c.ClientIP(),
	})
	response.OK(c, gin.H{"message": "2FA disabled"})
}

// RegenerateRecovery handles POST /api/v1/auth/2fa/recovery-codes（需已登录）。
// 重新生成恢复码（旧码全部作废）；必须验证 TOTP 动态码。
func (h *Handler) RegenerateRecovery(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	userID := c.GetInt64("user_id")
	user, err := h.authSvc.GetByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "user not found")
		return
	}
	if err := h.svc.VerifyManagedCode(c.Request.Context(), user, req.Code); err != nil {
		response.BadRequest(c, "TWOFA_CODE_INVALID", err.Error())
		return
	}
	recovery, err := h.svc.RegenerateRecoveryCodes(c.Request.Context(), user)
	if err != nil {
		response.BadRequest(c, "REGENERATE_FAILED", err.Error())
		return
	}
	audit.Record(audit.Event{
		Category: "auth", EventType: "twofa_recovery_regenerate", Result: "success",
		UserID: userID, RemoteIP: c.ClientIP(),
	})
	response.OK(c, gin.H{"recovery_codes": recovery})
}
