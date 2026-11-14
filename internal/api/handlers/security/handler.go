package security

import (
	"context"
	"encoding/json"
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
	// 按操作人/客户邮箱模糊筛选：审计只存 user_id（客户端事件存 clients.id，
	// 管理端事件存 users.id），所以两张表都要查；登录失败这类还没落到
	// user_id 的事件，邮箱只在 metadata_json 里，也一并匹配。
	if v := strings.TrimSpace(c.Query("email")); v != "" {
		like := "%" + v + "%"
		adminSub := h.db.Model(&entity.User{}).Select("id").Where("email LIKE ? OR username LIKE ?", like, like)
		clientSub := h.db.Model(&entity.Client{}).Select("id").Where("email LIKE ? OR nickname LIKE ?", like, like)
		tx = tx.Where("(user_id IN (?) OR user_id IN (?) OR metadata_json LIKE ?)", adminSub, clientSub, like)
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
	// 审计只存 user_id：列表补用户名/邮箱，前端「用户」列才有内容（原先直接透传 entity，前端取
	// row.user 永远为空）。客户端事件（client_*）的 user_id 是 clients.id，管理端事件是 users.id。
	users, clients := h.auditAccounts(c.Request.Context(), events)
	items := make([]gin.H, 0, len(events))
	for _, e := range events {
		actorName, actorEmail := "", ""
		uid := keyOfPtr(e.UserID)
		if isClientAuditEvent(e.EventType) {
			if cl, ok := clients[uid]; ok {
				actorName, actorEmail = cl.Nickname, cl.Email
			}
		} else if u, ok := users[uid]; ok {
			actorName, actorEmail = u.Username, u.Email
		}
		// 兜底：ID 落在另一张表，或事件没写 user_id（登录失败只存了 metadata.email）。
		if actorName == "" && actorEmail == "" {
			if u, ok := users[uid]; ok {
				actorName, actorEmail = u.Username, u.Email
			} else if cl, ok := clients[uid]; ok {
				actorName, actorEmail = cl.Nickname, cl.Email
			}
		}
		if actorName == "" && actorEmail == "" {
			actorName = auditMetadataEmail(e.MetadataJSON)
			actorEmail = actorName
		}
		items = append(items, gin.H{
			"id":            e.ID,
			"category":      e.Category,
			"event_type":    e.EventType,
			"service_id":    e.ServiceID,
			"endpoint_id":   e.EndpointID,
			"user_id":       e.UserID,
			"user_name":     actorName,
			"user_email":    actorEmail,
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

// isClientAuditEvent 判断审计事件的操作主体是否为订阅客户（clients 表）。
func isClientAuditEvent(eventType string) bool {
	return strings.HasPrefix(eventType, "client_")
}

// auditMetadataEmail 取出 metadata_json 里的 email（登录失败等事件只记在这里）。
func auditMetadataEmail(meta string) string {
	if meta == "" {
		return ""
	}
	var m struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal([]byte(meta), &m); err != nil {
		return ""
	}
	return strings.TrimSpace(m.Email)
}

// auditAccounts 批量查 user_id → 管理端用户 / 客户（两张表都可能命中）。
func (h *Handler) auditAccounts(ctx context.Context, events []entity.SecurityAuditEvent) (map[int64]entity.User, map[int64]entity.Client) {
	users := map[int64]entity.User{}
	clients := map[int64]entity.Client{}
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
		return users, clients
	}
	var us []entity.User
	if err := h.db.WithContext(ctx).Select("id", "username", "email").Where("id IN ?", ids).Find(&us).Error; err == nil {
		for _, u := range us {
			users[u.ID] = u
		}
	}
	var cs []entity.Client
	if err := h.db.WithContext(ctx).Select("id", "email", "nickname").Where("id IN ?", ids).Find(&cs).Error; err == nil {
		for _, cl := range cs {
			clients[cl.ID] = cl
		}
	}
	return users, clients
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
	// 按客户邮箱模糊筛选：日志只存 subscription_id，先反查「客户 → 订阅」。
	if v := strings.TrimSpace(c.Query("email")); v != "" {
		like := "%" + v + "%"
		clientIDs := h.db.Model(&entity.Client{}).Select("id").Where("email LIKE ?", like)
		subIDs := h.db.Model(&entity.Subscription{}).Select("id").Where("client_id IN (?)", clientIDs)
		tx = tx.Where("subscription_id IN (?)", subIDs)
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
			"risk_level":      l.RiskLevel,
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
	// 与列表一致：支持按操作人/客户邮箱模糊筛选（管理端 users + 客户端 clients + metadata）。
	if v := strings.TrimSpace(c.Query("email")); v != "" {
		like := "%" + v + "%"
		adminSub := h.db.Model(&entity.User{}).Select("id").Where("email LIKE ? OR username LIKE ?", like, like)
		clientSub := h.db.Model(&entity.Client{}).Select("id").Where("email LIKE ? OR nickname LIKE ?", like, like)
		tx = tx.Where("(user_id IN (?) OR user_id IN (?) OR metadata_json LIKE ?)", adminSub, clientSub, like)
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
