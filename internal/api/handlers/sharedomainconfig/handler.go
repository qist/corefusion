package sharedomainconfig

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/sharedomainconfig"
)

// Handler handles share domain config HTTP requests.
type Handler struct {
	svc *sharedomainconfig.Service
}

// NewHandler creates a new share domain config handler.
func NewHandler(svc *sharedomainconfig.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/share-domains.
func (h *Handler) List(c *gin.Context) {
	configType := c.Query("type")
	configs, err := h.svc.List(c.Request.Context(), configType)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// Create handles POST /api/v1/share-domains.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Type  string `json:"type" binding:"required"`
		Value string `json:"value"`
		Port  int    `json:"port"`
		Ports string `json:"ports"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	config, err := h.svc.Create(c.Request.Context(), req.Type, req.Value, req.Port, req.Ports)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

// Update handles PUT /api/v1/share-domains/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Type    string `json:"type"`
		Value   string `json:"value"`
		Port    int    `json:"port"`
		Ports   string `json:"ports"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	config, err := h.svc.Update(c.Request.Context(), id, req.Type, req.Value, req.Port, req.Ports, enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

// Delete handles DELETE /api/v1/share-domains/:id.
func (h *Handler) Delete(c *gin.Context) {
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

// ClearAll handles DELETE /api/v1/share-domains/all.
func (h *Handler) ClearAll(c *gin.Context) {
	configType := c.Query("type")
	if err := h.svc.ClearAll(c.Request.Context(), configType); err != nil {
		response.InternalError(c, "CLEAR_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "cleared"})
}

// SeedDefaults handles POST /api/v1/share-domains/seed.
func (h *Handler) SeedDefaults(c *gin.Context) {
	if err := h.svc.SeedDefaults(c.Request.Context()); err != nil {
		response.InternalError(c, "SEED_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "seeded"})
}
