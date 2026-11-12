package security

import (
	"context"
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
	// 审计只存 user_id：列表补用户名，前端「用户」列才有内容（原先直接透传 entity，前端取
	// row.user 永远为空）。
	names := h.auditUserNames(c.Request.Context(), events)
	items := make([]gin.H, 0, len(events))
	for _, e := range events {
		items = append(items, gin.H{
			"id":            e.ID,
			"category":      e.Category,
			"event_type":    e.EventType,
			"service_id":    e.ServiceID,
			"endpoint_id":   e.EndpointID,
			"user_id":       e.UserID,
			"user_name":     names[keyOfPtr(e.UserID)],
			"remote_ip":     e.RemoteIP,
			"result":        e.Result,
			"reason":        e.Reason,
			"metadata_json": e.MetadataJSON,
			"created_at":    e.CreatedAt,
		})
	}
	response.Paginated(c, items, total, q.Limit, q.Offset)
}

// keyOfPtr 把可空 ID 归一成 map key（nil = 0）。
func keyOfPtr(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// auditUserNames 批量查 user_id → 用户名。
func (h *Handler) auditUserNames(ctx context.Context, events []entity.SecurityAuditEvent) map[int64]string {
	out := map[int64]string{}
	ids := make([]int64, 0, len(events))
	seen := map[int64]bool{}
	for _, e := range events {
		if e.UserID == nil || *e.UserID == 0 || seen[*e.UserID] {
			continue
		}
		seen[*e.UserID] = true
		ids = append(ids, *e.UserID)
	}
	if len(ids) == 0 {
		return out
	}
	var users []entity.User
	if err := h.db.WithContext(ctx).Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return out
	}
	for _, u := range users {
		out[u.ID] = u.Username
	}
	return out
}

// ListSubscriptionAccessLogs handles GET /api/v1/security/subscription-access-logs.
//
// 订阅拉取日志（subscription_access_logs）：此前只有 /subscriptions/:id/access-logs，
// 管理端没有全局入口，导致「curl 拉过订阅但在日志里看不到」。这里补一个审计页入口，
// 可按订阅 ID / IP 过滤，并带出客户邮箱便于排查。
func (h *Handler) ListSubscriptionAccessLogs(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}

	tx := h.db.Model(&entity.SubscriptionAccessLog{})
	if v := strings.TrimSpace(c.Query("subscription_id")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			tx = tx.Where("subscription_id = ?", id)
		}
	}
	if v := strings.TrimSpace(c.Query("ip")); v != "" {
		tx = tx.Where("ip = ? OR real_ip = ?", v, v)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}
	var logs []entity.SubscriptionAccessLog
	if err := tx.Order("id DESC").Limit(q.Limit).Offset(q.Offset).Find(&logs).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", err.Error())
		return
	}

	emails := h.subscriptionClientEmails(c.Request.Context(), logs)
	items := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		items = append(items, gin.H{
			"id":              l.ID,
			"subscription_id": l.SubscriptionID,
			"client_email":    emails[l.SubscriptionID],
			"ip":              l.IP,
			"real_ip":         l.RealIP,
			"cdn_ip":          l.CDNIP,
			"cdn_provider":    l.CDNProvider,
			"user_agent":      l.UserAgent,
			"status_code":     l.StatusCode,
			"hit_result":      l.HitResult,
			"created_at":      l.CreatedAt,
		})
	}
	response.Paginated(c, items, total, q.Limit, q.Offset)
}

// subscriptionClientEmails 批量把 subscription_id 映射成客户邮箱。
func (h *Handler) subscriptionClientEmails(ctx context.Context, logs []entity.SubscriptionAccessLog) map[int64]string {
	out := map[int64]string{}
	ids := make([]int64, 0, len(logs))
	seen := map[int64]bool{}
	for _, l := range logs {
		if l.SubscriptionID == 0 || seen[l.SubscriptionID] {
			continue
		}
		seen[l.SubscriptionID] = true
		ids = append(ids, l.SubscriptionID)
	}
	if len(ids) == 0 {
		return out
	}
	var subs []entity.Subscription
	if err := h.db.WithContext(ctx).Select("id", "client_id").Where("id IN ?", ids).Find(&subs).Error; err != nil {
		return out
	}
	clientIDs := make([]int64, 0, len(subs))
	cseen := map[int64]bool{}
	for _, s := range subs {
		if s.ClientID == 0 || cseen[s.ClientID] {
			continue
		}
		cseen[s.ClientID] = true
		clientIDs = append(clientIDs, s.ClientID)
	}
	emails := map[int64]string{}
	if len(clientIDs) > 0 {
		var clients []entity.Client
		if err := h.db.WithContext(ctx).Select("id", "email").Where("id IN ?", clientIDs).Find(&clients).Error; err == nil {
			for _, cl := range clients {
				emails[cl.ID] = cl.Email
			}
		}
	}
	for _, s := range subs {
		out[s.ID] = emails[s.ClientID]
	}
	return out
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
