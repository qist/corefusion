package security

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/response"
	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
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
// Optional filters: category, event_type, result, remote_ip.
func (h *Handler) ListAuditEvents(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	tx := h.db.Model(&entity.SecurityAuditEvent{})
	if v := c.Query("category"); v != "" {
		tx = tx.Where("category = ?", v)
	}
	if v := c.Query("event_type"); v != "" {
		tx = tx.Where("event_type = ?", v)
	}
	if v := c.Query("result"); v != "" {
		tx = tx.Where("result = ?", v)
	}
	if v := c.Query("remote_ip"); v != "" {
		tx = tx.Where("remote_ip = ?", v)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	var events []entity.SecurityAuditEvent
	if err := tx.Order("id DESC").Limit(q.Limit).Offset(q.Offset).Find(&events).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	response.Paginated(c, events, total, q.Limit, q.Offset)
}

// ListAlerts handles GET /api/v1/security/alerts.
// Optional filters: status, severity, alert_type.
func (h *Handler) ListAlerts(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	tx := h.db.Model(&entity.SecurityAlert{})
	if v := c.Query("status"); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := c.Query("severity"); v != "" {
		tx = tx.Where("severity = ?", v)
	}
	if v := c.Query("alert_type"); v != "" {
		tx = tx.Where("alert_type = ?", v)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	var alerts []entity.SecurityAlert
	if err := tx.Order("id DESC").Limit(q.Limit).Offset(q.Offset).Find(&alerts).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	response.Paginated(c, alerts, total, q.Limit, q.Offset)
}

// alertChannelInput is the request body for alert channel create/update.
type alertChannelInput struct {
	Name        string `json:"name" binding:"required"`
	ChannelType string `json:"channel_type" binding:"required"`
	ConfigJSON  string `json:"config_json"`
	Status      *int   `json:"status"`
}

// ListAlertChannels handles GET /api/v1/security/alert-channels.
func (h *Handler) ListAlertChannels(c *gin.Context) {
	var channels []entity.AlertChannel
	if err := h.db.Order("id DESC").Find(&channels).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	response.OK(c, channels)
}

// CreateAlertChannel handles POST /api/v1/security/alert-channels.
func (h *Handler) CreateAlertChannel(c *gin.Context) {
	var in alertChannelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	now := time.Now()
	ch := entity.AlertChannel{
		Name:        in.Name,
		ChannelType: in.ChannelType,
		ConfigJSON:  in.ConfigJSON,
		Status:      1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if in.ConfigJSON == "" {
		ch.ConfigJSON = "{}"
	}
	if in.Status != nil {
		ch.Status = *in.Status
	}
	if err := h.db.Create(&ch).Error; err != nil {
		response.InternalError(c, "CREATE_FAILED", err.Error())
		return
	}
	response.Created(c, ch)
}

// UpdateAlertChannel handles PUT /api/v1/security/alert-channels/:id.
func (h *Handler) UpdateAlertChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var ch entity.AlertChannel
	if err := h.db.First(&ch, id).Error; err != nil {
		response.NotFound(c, "CHANNEL_NOT_FOUND", "alert channel not found")
		return
	}
	var in alertChannelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	updates := map[string]interface{}{
		"name":         in.Name,
		"channel_type": in.ChannelType,
		"updated_at":   time.Now(),
	}
	if in.ConfigJSON != "" {
		updates["config_json"] = in.ConfigJSON
	}
	if in.Status != nil {
		updates["status"] = *in.Status
	}
	if err := h.db.Model(&ch).Updates(updates).Error; err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, ch)
}

// DeleteAlertChannel handles DELETE /api/v1/security/alert-channels/:id.
func (h *Handler) DeleteAlertChannel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.db.Delete(&entity.AlertChannel{}, id).Error; err != nil {
		response.InternalError(c, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"success": true})
}

// ResolveAlert handles POST /api/v1/security/alerts/:id/resolve.
func (h *Handler) ResolveAlert(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var alert entity.SecurityAlert
	if err := h.db.First(&alert, id).Error; err != nil {
		response.NotFound(c, "ALERT_NOT_FOUND", "alert not found")
		return
	}
	now := time.Now()
	if err := h.db.Model(&alert).Updates(map[string]interface{}{
		"status":      "resolved",
		"resolved_at": now,
	}).Error; err != nil {
		response.InternalError(c, "UPDATE_FAILED", err.Error())
		return
	}
	alert.Status = "resolved"
	alert.ResolvedAt = &now
	response.OK(c, alert)
}
