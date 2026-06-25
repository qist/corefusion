package clientauth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/clientauth"
)

// Handler handles client authentication HTTP requests.
type Handler struct {
	svc *clientauth.Service
}

// NewHandler creates a new client auth handler.
func NewHandler(svc *clientauth.Service) *Handler {
	return &Handler{svc: svc}
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
		response.Unauthorized(c, "INVALID_CREDENTIALS", err.Error())
		return
	}

	// Get client info
	clientInfo, _ := h.svc.GetMe(c.Request.Context(), result.Client.ID)
	result.Client = clientInfo

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
		NewPassword string `json:"new_password" binding:"required,min=6"`
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

// getClientID extracts client ID from Gin context (set by middleware).
func getClientID(c *gin.Context) int64 {
	if id, exists := c.Get("client_id"); exists {
		if cid, ok := id.(int64); ok {
			return cid
		}
	}
	return 0
}

// ClientAuthMiddleware creates a middleware for client JWT authentication.
func ClientAuthMiddleware(svc *clientauth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
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

		clientID, err := svc.ValidateAccessToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("client_id", clientID)
		c.Next()
	}
}
