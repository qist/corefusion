package subendpoint

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/service/subendpoint"
)

// Handler handles subscription endpoint HTTP requests.
type Handler struct {
	svc *subendpoint.Service
}

// NewHandler creates a new subscription endpoint handler.
func NewHandler(svc *subendpoint.Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /api/v1/subscription-endpoints.
func (h *Handler) List(c *gin.Context) {
	pg := response.ParsePagination(c)
	endpoints, total, err := h.svc.ListPaged(c.Request.Context(), pg.Limit, pg.Offset)
	if err != nil {
		response.InternalError(c, "LIST_ENDPOINTS_FAILED", err.Error())
		return
	}
	response.Paginated(c, endpoints, total, pg.Limit, pg.Offset)
}

// GetByID handles GET /api/v1/subscription-endpoints/:id.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid endpoint id")
		return
	}

	endpoint, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "ENDPOINT_NOT_FOUND", "endpoint not found")
		return
	}

	response.OK(c, endpoint)
}

// Create handles POST /api/v1/subscription-endpoints.
func (h *Handler) Create(c *gin.Context) {
	var input subendpoint.CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	endpoint, err := h.svc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_ENDPOINT_FAILED", err.Error())
		return
	}

	response.Created(c, endpoint)
}

// Update handles PUT /api/v1/subscription-endpoints/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid endpoint id")
		return
	}

	var input subendpoint.UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	endpoint, err := h.svc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_ENDPOINT_FAILED", err.Error())
		return
	}

	response.OK(c, endpoint)
}

// Delete handles DELETE /api/v1/subscription-endpoints/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid endpoint id")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_ENDPOINT_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "endpoint deleted"})
}
