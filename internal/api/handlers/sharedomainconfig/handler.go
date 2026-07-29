package sharedomainconfig

import (
	"strconv"
	"time"

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
// Returns all share domain configs (optionally filtered by type).
func (h *Handler) List(c *gin.Context) {
	pg := response.ParsePagination(c)
	configType := c.Query("type")
	configs, total, err := h.svc.ListPaged(c.Request.Context(), configType, pg.Limit, pg.Offset)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.Paginated(c, configs, total, pg.Limit, pg.Offset)
}

// ListByServer handles GET /api/v1/node-servers/:id/share-domains.
// Returns direct share domain configs effective for a specific server
// (both global server_id=0 and server-specific server_id=serverID).
func (h *Handler) ListByServer(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid server id")
		return
	}
	configs, err := h.svc.ListByServer(c.Request.Context(), serverID)
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	response.OK(c, configs)
}

// Create handles POST /api/v1/share-domains and
// POST /api/v1/node-servers/:id/share-domains.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Type     string `json:"type" binding:"required"`
		Value    string `json:"value"`
		Port     int    `json:"port"`
		Ports    string `json:"ports"`
		ServerID int64  `json:"server_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	// :id param takes priority over body server_id
	serverID := req.ServerID
	if idParam := c.Param("id"); idParam != "" {
		if parsed, err := strconv.ParseInt(idParam, 10, 64); err == nil {
			serverID = parsed
		}
	}

	config, err := h.svc.Create(c.Request.Context(), serverID, req.Type, req.Value, req.Port, req.Ports)
	if err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.OK(c, config)
}

// Update handles PUT /api/v1/share-domains/:id and
// PUT /api/v1/node-servers/:id/share-domains/:domainId.
func (h *Handler) Update(c *gin.Context) {
	idStr := c.Param("domainId")
	if idStr == "" {
		idStr = c.Param("id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	var req struct {
		Type     string `json:"type"`
		Value    string `json:"value"`
		Port     int    `json:"port"`
		Ports    string `json:"ports"`
		Enabled  *bool  `json:"enabled"`
		ServerID *int64 `json:"server_id"`
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
	// Update server_id binding if provided
	if req.ServerID != nil {
		config.ServerID = *req.ServerID
		config.UpdatedAt = time.Now()
		_ = h.svc.Save(c.Request.Context(), config)
	}
	response.OK(c, config)
}

// Delete handles DELETE /api/v1/share-domains/:id and
// DELETE /api/v1/node-servers/:id/share-domains/:domainId.
func (h *Handler) Delete(c *gin.Context) {
	idStr := c.Param("domainId")
	if idStr == "" {
		idStr = c.Param("id")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
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

// ClearAll handles DELETE /api/v1/share-domains/all and
// DELETE /api/v1/node-servers/:id/share-domains/all.
func (h *Handler) ClearAll(c *gin.Context) {
	configType := c.Query("type")

	if idParam := c.Param("id"); idParam != "" {
		if serverID, err := strconv.ParseInt(idParam, 10, 64); err == nil {
			if err := h.svc.ClearByServer(c.Request.Context(), serverID, configType); err != nil {
				response.InternalError(c, "CLEAR_FAILED", err.Error())
				return
			}
			response.OK(c, gin.H{"message": "cleared"})
			return
		}
	}

	if err := h.svc.ClearAll(c.Request.Context(), configType); err != nil {
		response.InternalError(c, "CLEAR_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "cleared"})
}

// SeedDefaults handles POST /api/v1/share-domains/seed.
// Seeds default CF domain configs globally (server_id=0).
func (h *Handler) SeedDefaults(c *gin.Context) {
	if err := h.svc.SeedDefaults(c.Request.Context()); err != nil {
		response.InternalError(c, "SEED_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "seeded"})
}
