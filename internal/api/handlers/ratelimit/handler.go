package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/service/audit"
	ratelimitsvc "corefusion/internal/service/ratelimit"
)

// Handler handles rate limit HTTP requests.
type Handler struct {
	svc *ratelimitsvc.Service
}

// NewHandler creates a new rate limit handler.
func NewHandler(svc *ratelimitsvc.Service) *Handler {
	return &Handler{svc: svc}
}

// GetConfig handles GET /api/v1/node-servers/:id/rate-limit.
func (h *Handler) GetConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}

	config, err := h.svc.GetConfig(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "GET_RATE_LIMIT_CONFIG_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, config)
}

// UpdateConfig handles PUT /api/v1/node-servers/:id/rate-limit.
func (h *Handler) UpdateConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}

	var req dto.RateLimitConfigUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	config, err := h.svc.UpdateConfig(c.Request.Context(), serverID, &req)
	if err != nil {
		audit.RecordNodeFromCtx(c, serverID, "update_ratelimit", "", false, err.Error())
		response.InternalError(c, "UPDATE_RATE_LIMIT_CONFIG_FAILED", err.Error())
		return
	}

	audit.RecordNodeFromCtx(c, serverID, "update_ratelimit", fmt.Sprintf("enabled=%v", config.Enabled), true, "")

	c.JSON(http.StatusOK, config)
}

// GetStatus handles GET /api/v1/node-servers/:id/rate-limit/status.
func (h *Handler) GetStatus(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}

	status, err := h.svc.GetStatus(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "GET_RATE_LIMIT_STATUS_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, status)
}
