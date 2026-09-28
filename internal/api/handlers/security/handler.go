package security

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/handlers/exportutil"
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

// ── 2026-09-23 补齐：前端在调、后端却不存在的两个接口 ──────────────────
//
// 背景（代码完成度体检）：`web/admin/src/api/security.js` 与 `Security.vue` 一直在调
//   GET  /security/audit-events/export   （导出 CSV，responseType: blob）
//   POST /security/alerts/resolve        （全部标记已解决）
// 但 router.go 里**只有** `/audit-events` 和 `/alerts/:id/resolve` ✗ → 界面上点"导出"和
// "全部已解决"永远 404 ✓。这里是补上后端实现（不改前端：前端那套调用方式本身是合理的）。

// ExportAuditEvents handles GET /api/v1/security/audit-events/export.
//
// 过滤条件与 ListAuditEvents **完全一致**（同一套 query 参数），但导出就是全量，
// 所以不分页；只设一个上限，避免一次导出把内存/响应体撑爆。
func (h *Handler) ExportAuditEvents(c *gin.Context) {
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

	// 上限走**设置项**（设置页 → 面板设置 → 导出上限）✓，原来是写死的 50000 ✗。
	// 前端可能传更大的 limit，但这里绝不越过配置上限 ✓。
	limit := exportutil.Limit(c, exportutil.MaxRows(c.Request.Context(), h.db))

	var events []entity.SecurityAuditEvent
	if err := tx.Order("id DESC").Limit(limit).Find(&events).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition",
		`attachment; filename="audit-events-`+time.Now().Format("20060102-150405")+`.csv"`)

	var b strings.Builder
	// ⚠ BOM：没有它 Excel 打开中文（reason / metadata 里常有中文）会乱码 ✗
	b.WriteString(exportutil.BOM)
	b.WriteString("id,created_at,category,event_type,result,remote_ip,reason,metadata_json\n")
	for i := range events {
		e := &events[i]
		fields := []string{
			strconv.FormatInt(e.ID, 10),
			e.CreatedAt.Format(time.RFC3339),
			e.Category,
			e.EventType,
			e.Result,
			e.RemoteIP,
			e.Reason,
			e.MetadataJSON,
		}
		for j, v := range fields {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString(exportutil.Escape(v))
		}
		b.WriteString("\n")
	}
	c.String(http.StatusOK, b.String())
}

// ResolveAllAlerts handles POST /api/v1/security/alerts/resolve.
//
// 把**所有尚未解决**的告警一次性标记为 resolved，返回处理条数。
// 注意：只更新 `status <> 'resolved'` 的行（不是全表 UPDATE），否则已解决的告警会被
// 重新写一遍 resolved_at，把人工处理时间冲掉 ✗。
func (h *Handler) ResolveAllAlerts(c *gin.Context) {
	now := time.Now()
	res := h.db.Model(&entity.SecurityAlert{}).
		Where("status IS NULL OR status <> ?", "resolved").
		Updates(map[string]interface{}{
			"status":      "resolved",
			"resolved_at": now,
		})
	if res.Error != nil {
		response.InternalError(c, "UPDATE_FAILED", res.Error.Error())
		return
	}
	response.OK(c, gin.H{"success": true, "resolved": res.RowsAffected})
}
