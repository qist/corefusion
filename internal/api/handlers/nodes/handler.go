package nodes

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
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

// GetOnlineUsers handles GET /api/v1/nodes/:id/online-users.
// 返回该节点当前实时在线的订阅人（含 IP、速率、累计流量、会话明细）与聚合统计。
// 支持服务端筛选（keyword：邮箱/客户端名/IP）与分页（limit/offset 或 page/page_size，默认20封顶100）。
func (h *Handler) GetOnlineUsers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid node id")
		return
	}

	p := response.ParsePagination(c)
	res := h.nodeSvc.GetOnlineUsers(id, node.NodeOnlineFilter{
		Keyword: c.Query("keyword"),
		Limit:   p.Limit,
		Offset:  p.Offset,
	})
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
// 支持服务端筛选（server_id / keyword）与分页（limit/offset 或 page/page_size，默认20封顶100）。
func (h *Handler) GetAllOnlineUsers(c *gin.Context) {
	p := response.ParsePagination(c)
	serverID, _ := strconv.ParseInt(c.Query("server_id"), 10, 64)
	res := h.nodeSvc.GetAllOnlineUsersGlobal(node.GlobalOnlineFilter{
		ServerID: serverID,
		Keyword:  c.Query("keyword"),
		Limit:    p.Limit,
		Offset:   p.Offset,
	})
	// 用统一分页响应包裹：items=本页用户，total=筛选后总数；同时附带全局汇总。
	response.OK(c, gin.H{
		"items":               res.Users,
		"total":               res.FilteredTotal,
		"limit":               p.Limit,
		"offset":              p.Offset,
		"total_subscriptions": res.TotalSubscriptions,
		"total_devices":       res.TotalDevices,
	})
}
