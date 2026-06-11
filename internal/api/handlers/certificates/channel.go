package certificates

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	certService "corefusion/internal/service/certificate"
)

// ChannelHandler handles DNS channel HTTP requests.
type ChannelHandler struct {
	svc *certService.DNSChannelService
}

// NewChannelHandler creates a new channel handler.
func NewChannelHandler(svc *certService.DNSChannelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

// List handles GET /api/v1/cert-channels.
func (h *ChannelHandler) List(c *gin.Context) {
	channels, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if channels == nil {
		channels = []map[string]interface{}{}
	}
	response.OK(c, channels)
}

// Create handles POST /api/v1/cert-channels.
func (h *ChannelHandler) Create(c *gin.Context) {
	var req struct {
		Name     string            `json:"name" binding:"required"`
		Provider string            `json:"provider" binding:"required"`
		EnvVars  map[string]string `json:"env_vars" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if !certService.ValidateProvider(req.Provider) {
		response.BadRequest(c, "INVALID_PROVIDER", "unsupported provider: "+req.Provider)
		return
	}
	if err := certService.ValidateEnvVars(req.Provider, req.EnvVars); err != nil {
		response.BadRequest(c, "INVALID_ENV_VARS", err.Error())
		return
	}
	ch, err := h.svc.Create(c.Request.Context(), req.Name, req.Provider, req.EnvVars)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, ch)
}

// Update handles PUT /api/v1/cert-channels/:id.
func (h *ChannelHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Name     string            `json:"name" binding:"required"`
		Provider string            `json:"provider" binding:"required"`
		EnvVars  map[string]string `json:"env_vars"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	ch, err := h.svc.Update(c.Request.Context(), id, req.Name, req.Provider, req.EnvVars)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, ch)
}

// Delete handles DELETE /api/v1/cert-channels/:id.
func (h *ChannelHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
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

// Providers handles GET /api/v1/cert-channels/providers.
func (h *ChannelHandler) Providers(c *gin.Context) {
	providers := certService.SupportedDNSProviders()
	response.OK(c, providers)
}
