package nodes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/service/node"
)

// Handler handles node HTTP requests.
type Handler struct {
	nodeSvc *node.Service
}

// NewHandler creates a new node handler.
func NewHandler(nodeSvc *node.Service) *Handler {
	return &Handler{nodeSvc: nodeSvc}
}

// List handles GET /api/v1/nodes.
func (h *Handler) List(c *gin.Context) {
	var q struct {
		dto.ListQuery
		CoreType string `form:"core_type"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	nodes, total, err := h.nodeSvc.List(c.Request.Context(), q.CoreType, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_NODES_FAILED", err.Error())
		return
	}

	response.Paginated(c, nodes, total, q.Limit, q.Offset)
}

// Create handles POST /api/v1/nodes.
func (h *Handler) Create(c *gin.Context) {
	var input node.CreateNodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	n, err := h.nodeSvc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_NODE_FAILED", err.Error())
		return
	}

	response.Created(c, n)
}

// GetByID handles GET /api/v1/nodes/:id.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node id")
		return
	}

	n, err := h.nodeSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "NODE_NOT_FOUND", "node not found")
		return
	}

	response.OK(c, n)
}

// Update handles PUT /api/v1/nodes/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node id")
		return
	}

	var input node.UpdateNodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	n, err := h.nodeSvc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_NODE_FAILED", err.Error())
		return
	}

	response.OK(c, n)
}

// Delete handles DELETE /api/v1/nodes/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node id")
		return
	}

	if err := h.nodeSvc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_NODE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "node deleted"})
}
