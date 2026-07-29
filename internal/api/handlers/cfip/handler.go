package cfip

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/cfip"
)

// Handler handles CF IP HTTP requests.
type Handler struct {
	svc *cfip.Service
}

// NewHandler creates a new CF IP handler.
func NewHandler(svc *cfip.Service) *Handler {
	return &Handler{svc: svc}
}

// ListSources handles GET /api/v1/cfip/sources.
func (h *Handler) ListSources(c *gin.Context) {
	pg := response.ParsePagination(c)
	sources, total, err := h.svc.ListSourcesPaged(c.Request.Context(), pg.Limit, pg.Offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, sources, total, pg.Limit, pg.Offset)
}

// CreateSource handles POST /api/v1/cfip/sources.
func (h *Handler) CreateSource(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		URL      string `json:"url" binding:"required"`
		Type     string `json:"type"`
		Ports    string `json:"ports"`
		Interval int    `json:"interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.Type == "" {
		req.Type = "bestcf"
	}
	source, err := h.svc.CreateSource(c.Request.Context(), req.Name, req.URL, req.Type, req.Ports, req.Interval)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.OK(c, source)
}

// UpdateSource handles PUT /api/v1/cfip/sources/:id.
func (h *Handler) UpdateSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Name     string `json:"name"`
		URL      string `json:"url"`
		Type     string `json:"type"`
		Ports    string `json:"ports"`
		Interval int    `json:"interval"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	source, err := h.svc.UpdateSource(c.Request.Context(), id, req.Name, req.URL, req.Type, req.Ports, req.Interval, enabled)
	if err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, source)
}

// DeleteSource handles DELETE /api/v1/cfip/sources/:id.
func (h *Handler) DeleteSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteSource(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// FetchSource handles POST /api/v1/cfip/sources/:id/fetch.
func (h *Handler) FetchSource(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.FetchFromSource(c.Request.Context(), id); err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "fetched"})
}

// FetchAll handles POST /api/v1/cfip/fetch-all.
func (h *Handler) FetchAll(c *gin.Context) {
	if err := h.svc.FetchAllEnabled(c.Request.Context()); err != nil {
		response.InternalError(c, "FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "fetched all"})
}

// ListEntries handles GET /api/v1/cfip/entries.
func (h *Handler) ListEntries(c *gin.Context) {
	pg := response.ParsePagination(c)
	sourceID, _ := strconv.ParseInt(c.Query("source_id"), 10, 64)
	entries, total, err := h.svc.ListEntriesPaged(c.Request.Context(), sourceID, pg.Limit, pg.Offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, entries, total, pg.Limit, pg.Offset)
}

// AddEntry handles POST /api/v1/cfip/entries.
func (h *Handler) AddEntry(c *gin.Context) {
	var req struct {
		IP    string `json:"ip" binding:"required"`
		Ports string `json:"ports"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	entry, err := h.svc.AddEntry(c.Request.Context(), req.IP, req.Ports)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.OK(c, entry)
}

// DeleteEntry handles DELETE /api/v1/cfip/entries/:id.
func (h *Handler) DeleteEntry(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	if err := h.svc.DeleteEntry(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ClearEntries handles DELETE /api/v1/cfip/entries.
func (h *Handler) ClearEntries(c *gin.Context) {
	if err := h.svc.ClearEntries(c.Request.Context()); err != nil {
		response.InternalError(c, "CLEAR_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "cleared"})
}

// SeedDefaults handles POST /api/v1/cfip/seed.
func (h *Handler) SeedDefaults(c *gin.Context) {
	if err := h.svc.SeedDefaultSources(c.Request.Context()); err != nil {
		response.InternalError(c, "SEED_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "seeded"})
}
