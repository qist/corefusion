package clientauth

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/pkg/netutil"
	"corefusion/internal/service/audit"
	"corefusion/internal/service/clientauth"
	"corefusion/internal/store/repository"
)

// RateLimitPusher pushes IP association to rate limit agents.
type RateLimitPusher interface {
	PushIPAssociation(ctx context.Context, serverID int64, ip string, subID int64, maxDownMbps, maxUpMbps, connLimit, deviceLimit int32) error
}

// Handler handles client authentication HTTP requests.
type Handler struct {
	svc              *clientauth.Service
	rateLimitPusher  RateLimitPusher
	nodeServerRepo   *repository.NodeServerRepository
	clientRepo       *repository.ClientRepository
	planTmplRepo     *repository.PlanTemplateRepository
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
		audit.Record(audit.Event{
			Category: "auth", EventType: "client_login", Result: "failure",
			RemoteIP: netutil.GetRealIP(c.Request), Reason: err.Error(),
			Metadata: map[string]any{"email": input.Email},
		})
		response.Unauthorized(c, "INVALID_CREDENTIALS", err.Error())
		return
	}
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
