package warp

import (
	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/warp"
)

// Handler handles WARP HTTP requests.
type Handler struct {
	svc *warp.Service
}

// NewHandler creates a new WARP handler.
func NewHandler(svc *warp.Service) *Handler {
	return &Handler{svc: svc}
}

// Status handles GET /api/v1/warp/status.
func (h *Handler) Status(c *gin.Context) {
	status, err := h.svc.GetStatus()
	if err != nil {
		response.InternalError(c, "STATUS_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// Register handles POST /api/v1/warp/register.
func (h *Handler) Register(c *gin.Context) {
	status, err := h.svc.Register()
	if err != nil {
		response.InternalError(c, "REGISTER_FAILED", err.Error())
		return
	}
	response.OK(c, status)
}

// GenerateConfig handles POST /api/v1/warp/generate-config.
func (h *Handler) GenerateConfig(c *gin.Context) {
	config, err := h.svc.GenerateConfig()
	if err != nil {
		response.InternalError(c, "GENERATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"config": config})
}

// GetConfig handles GET /api/v1/warp/config.
func (h *Handler) GetConfig(c *gin.Context) {
	config, _ := h.svc.GetConfig()
	response.OK(c, gin.H{"config": config})
}

// Reset handles DELETE /api/v1/warp.
func (h *Handler) Reset(c *gin.Context) {
	if err := h.svc.Reset(); err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "reset"})
}
