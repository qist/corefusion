package security

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
)

// Handler handles security HTTP requests.
type Handler struct {
	db *gorm.DB
}

// NewHandler creates a new security handler.
func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// ListAuditEvents handles GET /api/v1/security/audit-events.
func (h *Handler) ListAuditEvents(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	// TODO: query from security_audit_events table when implemented
	response.Paginated(c, []interface{}{}, 0, q.Limit, q.Offset)
}

// ListAlerts handles GET /api/v1/security/alerts.
func (h *Handler) ListAlerts(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	// TODO: query from security_alerts table when implemented
	response.Paginated(c, []interface{}{}, 0, q.Limit, q.Offset)
}

// ListAlertChannels handles GET /api/v1/security/alert-channels.
func (h *Handler) ListAlertChannels(c *gin.Context) {
	// TODO: query from alert_channels table when implemented
	response.OK(c, []interface{}{})
}

// CreateAlertChannel handles POST /api/v1/security/alert-channels.
func (h *Handler) CreateAlertChannel(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// UpdateAlertChannel handles PUT /api/v1/security/alert-channels/:id.
func (h *Handler) UpdateAlertChannel(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}

// ResolveAlert handles POST /api/v1/security/alerts/:id/resolve.
func (h *Handler) ResolveAlert(c *gin.Context) {
	// TODO: implement
	response.OK(c, gin.H{"code": "NOT_IMPLEMENTED"})
}
