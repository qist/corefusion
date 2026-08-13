package logmanage

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/model/entity"
	trafficSvc "corefusion/internal/service/traffic"
)

// CleanupHandler handles database log cleanup rule management.
// It is registered alongside the main logmanage handler.
type CleanupHandler struct {
	cleanupSvc CleanupService
}

// CleanupService is the interface for the log cleanup service.
type CleanupService interface {
	ListRules() ([]entity.LogCleanupRule, error)
	GetRule(id int64) (*entity.LogCleanupRule, error)
	UpdateRule(rule *entity.LogCleanupRule) error
	CleanupNow(ctx context.Context) ([]trafficSvc.CleanupResult, int64, error)
	CleanupTableByName(ctx context.Context, tableName string) (int64, error)
}

// NewCleanupHandler creates a new cleanup handler.
func NewCleanupHandler(svc CleanupService) *CleanupHandler {
	return &CleanupHandler{cleanupSvc: svc}
}

// ListCleanupRules returns all log cleanup rules.
func (h *CleanupHandler) ListCleanupRules(c *gin.Context) {
	rules, err := h.cleanupSvc.ListRules()
	if err != nil {
		response.InternalError(c, "LIST_FAILED", err.Error())
		return
	}
	if rules == nil {
		rules = []entity.LogCleanupRule{}
	}
	response.OK(c, rules)
}

// UpdateCleanupRule updates a cleanup rule's retain days, enabled status, etc.
func (h *CleanupHandler) UpdateCleanupRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "invalid id")
		return
	}
	existing, err := h.cleanupSvc.GetRule(id)
	if err != nil {
		response.NotFound(c, "NOT_FOUND", "cleanup rule not found")
		return
	}

	var req struct {
		RetainDays  *int    `json:"retain_days"`
		Enabled     *bool   `json:"enabled"`
		CleanupCron *string `json:"cleanup_cron"`
		DisplayName *string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}

	if req.RetainDays != nil {
		existing.RetainDays = *req.RetainDays
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.CleanupCron != nil {
		existing.CleanupCron = *req.CleanupCron
	}
	if req.DisplayName != nil {
		existing.DisplayName = *req.DisplayName
	}

	if err := h.cleanupSvc.UpdateRule(existing); err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, existing)
}

// CleanupAll manually triggers cleanup for all enabled rules.
func (h *CleanupHandler) CleanupAll(c *gin.Context) {
	results, total, err := h.cleanupSvc.CleanupNow(c.Request.Context())
	if err != nil {
		response.InternalError(c, "CLEANUP_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"results": results, "total_deleted": total})
}

// CleanupTable manually triggers cleanup for a specific table.
func (h *CleanupHandler) CleanupTable(c *gin.Context) {
	tableName := c.Param("table")
	if tableName == "" {
		response.BadRequest(c, "INVALID_TABLE", "table name required")
		return
	}
	deleted, err := h.cleanupSvc.CleanupTableByName(c.Request.Context(), tableName)
	if err != nil {
		response.InternalError(c, "CLEANUP_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"table": tableName, "deleted": deleted})
}
