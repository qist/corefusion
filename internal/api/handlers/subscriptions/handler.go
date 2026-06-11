package subscriptions

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	subService "corefusion/internal/service/subscription"
)

// Handler handles subscription HTTP requests.
type Handler struct {
	subSvc *subService.Service
}

// NewHandler creates a new subscription handler.
func NewHandler(subSvc *subService.Service) *Handler {
	return &Handler{subSvc: subSvc}
}

// List handles GET /api/v1/subscriptions.
func (h *Handler) List(c *gin.Context) {
	var q struct {
		dto.ListQuery
		Status string `form:"status"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	subs, total, err := h.subSvc.List(c.Request.Context(), q.Status, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_SUBSCRIPTIONS_FAILED", err.Error())
		return
	}

	// Mask tokens in list view
	items := make([]gin.H, len(subs))
	for i, s := range subs {
		token := s.Token
		if len(token) > 8 {
			token = token[:8] + "***"
		}
		items[i] = gin.H{
			"id":              s.ID,
			"client_id":       s.ClientID,
			"format":          s.Format,
			"token_masked":    token,
			"status":          s.Status,
			"security_mode":   s.SecurityMode,
			"quota_bytes":     s.QuotaBytes,
			"used_bytes_cache": s.UsedBytesCache,
			"quota_state":     s.QuotaState,
			"access_count":    s.AccessCount,
			"last_access_at":  s.LastAccessAt,
			"expires_at":      s.ExpiresAt,
			"created_at":      s.CreatedAt,
			"updated_at":      s.UpdatedAt,
		}
	}

	response.Paginated(c, items, total, q.Limit, q.Offset)
}

// Create handles POST /api/v1/subscriptions.
func (h *Handler) Create(c *gin.Context) {
	var input subService.CreateSubscriptionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	sub, err := h.subSvc.Create(c.Request.Context(), input)
	if err != nil {
		response.InternalError(c, "CREATE_SUBSCRIPTION_FAILED", err.Error())
		return
	}

	response.Created(c, gin.H{
		"id":           sub.ID,
		"client_id":    sub.ClientID,
		"format":       sub.Format,
		"token":        sub.Token,
		"status":       sub.Status,
		"security_mode": sub.SecurityMode,
		"quota_bytes":  sub.QuotaBytes,
		"expires_at":   sub.ExpiresAt,
		"created_at":   sub.CreatedAt,
	})
}

// GetByID handles GET /api/v1/subscriptions/:id.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	sub, err := h.subSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}

	response.OK(c, sub)
}

// Update handles PUT /api/v1/subscriptions/:id.
func (h *Handler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	var input subService.UpdateSubscriptionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	sub, err := h.subSvc.Update(c.Request.Context(), id, input)
	if err != nil {
		response.InternalError(c, "UPDATE_SUBSCRIPTION_FAILED", err.Error())
		return
	}

	response.OK(c, sub)
}

// Delete handles DELETE /api/v1/subscriptions/:id.
func (h *Handler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	if err := h.subSvc.Delete(c.Request.Context(), id); err != nil {
		response.InternalError(c, "DELETE_SUBSCRIPTION_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "subscription deleted"})
}

// Rotate handles POST /api/v1/subscriptions/:id/rotate.
func (h *Handler) Rotate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	userID := c.GetInt64("user_id")
	sub, err := h.subSvc.Rotate(c.Request.Context(), id, userID, req.Reason)
	if err != nil {
		response.InternalError(c, "ROTATE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":            sub.ID,
		"token":         sub.Token,
		"token_version": sub.TokenVersion,
		"updated_at":    sub.UpdatedAt,
	})
}

// Revoke handles POST /api/v1/subscriptions/:id/revoke.
func (h *Handler) Revoke(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	sub, err := h.subSvc.Revoke(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "REVOKE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":         sub.ID,
		"status":     sub.Status,
		"revoked_at": sub.RevokedAt,
	})
}

// Restore handles POST /api/v1/subscriptions/:id/restore.
func (h *Handler) Restore(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	sub, err := h.subSvc.Restore(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "RESTORE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":         sub.ID,
		"status":     sub.Status,
		"quota_state": sub.QuotaState,
	})
}

// Enable handles POST /api/v1/subscriptions/:id/enable.
func (h *Handler) Enable(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	sub, err := h.subSvc.Enable(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "ENABLE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":     sub.ID,
		"status": sub.Status,
	})
}

// ResetUsage handles POST /api/v1/subscriptions/:id/reset-usage.
func (h *Handler) ResetUsage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	sub, err := h.subSvc.ResetUsage(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "RESET_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{
		"id":              sub.ID,
		"used_bytes_cache": sub.UsedBytesCache,
		"quota_state":     sub.QuotaState,
		"last_reset_at":   sub.LastResetAt,
	})
}

// ListAccessLogs handles GET /api/v1/subscriptions/:id/access-logs.
func (h *Handler) ListAccessLogs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	logs, total, err := h.subSvc.ListAccessLogs(c.Request.Context(), id, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_ACCESS_LOGS_FAILED", err.Error())
		return
	}

	response.Paginated(c, logs, total, q.Limit, q.Offset)
}

// ListRotations handles GET /api/v1/subscriptions/:id/rotations.
func (h *Handler) ListRotations(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid subscription id")
		return
	}

	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	rotations, total, err := h.subSvc.ListRotations(c.Request.Context(), id, q.Limit, q.Offset)
	if err != nil {
		response.InternalError(c, "LIST_ROTATIONS_FAILED", err.Error())
		return
	}

	response.Paginated(c, rotations, total, q.Limit, q.Offset)
}
