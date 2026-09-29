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
