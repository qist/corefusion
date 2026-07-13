package client

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/service/client"
)

// Handler handles client HTTP requests.
type Handler struct {
	svc *client.Service
}

func NewHandler(svc *client.Service) *Handler {
	return &Handler{svc: svc}
}

// --- Client handlers ---

func (h *Handler) ListClients(c *gin.Context) {
	var q struct {
		dto.ListQuery
		PlanID int64  `form:"plan_id"`
		Status string `form:"status"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	clients, total, err := h.svc.ListClients(c.Request.Context(), q.PlanID, q.Status, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_CLIENTS_FAILED", err.Error())
		return
	}
	response.Paginated(c, clients, total, q.Limit, q.Offset)
}

func (h *Handler) GetClient(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	client, err := h.svc.GetClient(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "CLIENT_NOT_FOUND", "client not found")
		return
	}
	response.OK(c, client)
}

func (h *Handler) CreateClient(c *gin.Context) {
	var input client.CreateClientInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cl, err := h.svc.CreateClient(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_CLIENT_FAILED", err.Error())
		return
	}
	response.Created(c, cl)
}

func (h *Handler) UpdateClient(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	var input struct {
		Password        *string `json:"password"`
		PlanID          *int64  `json:"plan_id"`
		PlanVariantID   *int64  `json:"plan_variant_id"`
		QuotaBytes      *int64  `json:"quota_bytes"`
		SpeedLimitMbps  *int    `json:"speed_limit_mbps"`
		ConcurrentLimit *int    `json:"concurrent_limit"`
		ExpiresAt       *string `json:"expires_at"`
		Remark          *string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cl, err := h.svc.UpdateClient(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_CLIENT_FAILED", err.Error())
		return
	}
	response.OK(c, cl)
}

func (h *Handler) DeleteClient(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	if err := h.svc.DeleteClient(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_CLIENT_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "client deleted"})
}

func (h *Handler) SuspendClient(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	if err := h.svc.SuspendClient(c.Request.Context(), id); err != nil {
		response.InternalError(c, "SUSPEND_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "client suspended"})
}

func (h *Handler) RestoreClient(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	if err := h.svc.RestoreClient(c.Request.Context(), id); err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "client restored"})
}

func (h *Handler) ResetUsage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid client id")
		return
	}
	if err := h.svc.ResetUsage(c.Request.Context(), id); err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"message": "usage reset"})
}
