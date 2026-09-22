package auth

import (
	"github.com/gin-gonic/gin"

	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/service/audit"
	"corefusion/internal/service/auth"
	"corefusion/pkg/netutil"
)

// Handler handles auth HTTP requests.
type Handler struct {
	authSvc *auth.Service
}

// NewHandler creates a new auth handler.
func NewHandler(authSvc *auth.Service) *Handler {
	return &Handler{authSvc: authSvc}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	accessToken, refreshToken, user, err := h.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		middleware.DefaultLoginLimiter().RecordFailure(netutil.GetRealIP(c.Request))
		audit.Record(audit.Event{
			Category: "auth", EventType: "admin_login", Result: "failure",
			RemoteIP: c.ClientIP(), Reason: err.Error(),
			Metadata: map[string]any{"username": req.Username},
		})
		response.Unauthorized(c, "INVALID_CREDENTIALS", err.Error())
		return
	}
	middleware.DefaultLoginLimiter().RecordSuccess(netutil.GetRealIP(c.Request))
	audit.Record(audit.Event{
		Category: "auth", EventType: "admin_login", Result: "success",
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
func (h *Handler) Logout(c *gin.Context) {
	// TODO: implement token blacklisting
	response.OK(c, gin.H{"message": "logged out"})
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
	NewPassword string `json:"new_password" binding:"required,min=8"`
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
