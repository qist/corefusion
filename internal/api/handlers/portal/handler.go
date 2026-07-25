package portal

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	portalService "corefusion/internal/service/portal"
)

// Handler handles portal template HTTP requests.
type Handler struct {
	svc *portalService.Service
}

// NewHandler creates a new portal handler.
func NewHandler(svc *portalService.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/portal/templates.
func (h *Handler) List(c *gin.Context) {
	templates, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, templates)
}

// Get handles GET /api/v1/portal/templates/:id.
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	tmpl, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// Create handles POST /api/v1/portal/templates.
func (h *Handler) Create(c *gin.Context) {
	var input portalService.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tmpl, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, tmpl)
}

// Update handles PUT /api/v1/portal/templates/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var input portalService.UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tmpl, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// Delete handles DELETE /api/v1/portal/templates/:id.
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

// Activate handles POST /api/v1/portal/templates/:id/activate.
func (h *Handler) Activate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.Activate(c.Request.Context(), id); err != nil {
		response.InternalError(c, "ACTIVATE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "activated", "id": id})
}

// GetActive handles GET /api/v1/portal/active.
// Returns the currently active portal site configuration.
func (h *Handler) GetActive(c *gin.Context) {
	tmpl, err := h.svc.GetActiveConfig(c.Request.Context())
	if err != nil {
		response.NotFound(c, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// UpdateActive handles PUT /api/v1/portal/active.
// Updates the currently active portal site configuration.
func (h *Handler) UpdateActive(c *gin.Context) {
	var input portalService.UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	tmpl, err := h.svc.UpdateActiveConfig(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// ResetToDefault handles POST /api/v1/portal/reset.
// Resets the active portal to the built-in default template.
func (h *Handler) ResetToDefault(c *gin.Context) {
	tmpl, err := h.svc.ResetToDefault(c.Request.Context())
	if err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}
	response.OK(c, tmpl)
}

// CacheStatus handles GET /api/v1/portal/cache.
// Returns diagnostic information about the render cache.
func (h *Handler) CacheStatus(c *gin.Context) {
	stats := h.svc.GetCacheStats()
	response.OK(c, stats)
}

// CacheClear handles DELETE /api/v1/portal/cache.
// Invalidates the render cache, forcing a rebuild on next request.
func (h *Handler) CacheClear(c *gin.Context) {
	h.svc.InvalidateCache()
	response.OK(c, gin.H{"message": "cache cleared"})
}

// CacheRebuild handles POST /api/v1/portal/cache/rebuild.
// Forces a cache rebuild by rendering the page once immediately.
func (h *Handler) CacheRebuild(c *gin.Context) {
	if err := h.svc.RebuildCache(c.Request.Context()); err != nil {
		response.InternalError(c, "REBUILD_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "cache rebuilt", "stats": h.svc.GetCacheStats()})
}

// CacheDevMode handles PUT /api/v1/portal/cache/dev-mode.
// Toggles dev mode (cache bypass) at runtime.
// Body: {"dev_mode": true/false}
func (h *Handler) CacheDevMode(c *gin.Context) {
	var body struct {
		DevMode bool `json:"dev_mode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	h.svc.SetDevMode(body.DevMode)
	mode := "production"
	if body.DevMode {
		mode = "dev"
	}
	response.OK(c, gin.H{"message": "dev mode set to " + mode, "dev_mode": body.DevMode})
}
