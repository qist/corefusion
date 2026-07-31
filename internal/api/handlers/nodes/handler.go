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

// GetOnlineUsers handles GET /api/v1/nodes/:id/online-users.
// 返回该节点当前实时在线的订阅人（含 IP、速率、累计流量、会话明细）与聚合统计。
func (h *Handler) GetOnlineUsers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node id")
		return
	}

	res := h.nodeSvc.GetOnlineUsers(id)
	response.OK(c, res)
}

// GetOnlineSummary handles GET /api/v1/node-servers/online-summary.
// 返回所有节点当前实时在线订阅人汇总（避免前端逐节点轮询）。
func (h *Handler) GetOnlineSummary(c *gin.Context) {
	res := h.nodeSvc.GetAllOnlineSummary()
	response.OK(c, res)
}

// GetAllOnlineUsers handles GET /api/v1/node-servers/online-users.
// 返回所有节点在线订阅人的全局合并视图（含节点名与富化字段），供全局在线用户页使用。
func (h *Handler) GetAllOnlineUsers(c *gin.Context) {
	res := h.nodeSvc.GetAllOnlineUsersGlobal()
	response.OK(c, res)
}
