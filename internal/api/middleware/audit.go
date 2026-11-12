package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/service/audit"
)

// auditWriteExemptPrefixes 不写「管理操作审计」的路径：
//   - /auth：登录/登出/2FA 已有更细的语义化事件（admin_login 等）
//   - /client-auth：客户端自助行为，不属于管理端审计
//   - /agent：节点上报/心跳，高频写会把审计表写爆
//   - /node-servers：写操作走 node_audit_logs（节点操作日志），避免两张表重复
//   - /security、/runtime、/metrics、/node-audit-logs：审计/任务查询本身
//   - /charge-callback*、/public：外部回调
var auditWriteExemptPrefixes = []string{
	"/api/v1/auth",
	"/api/v1/client-auth",
	"/api/v1/agent",
	"/api/v1/node-servers",
	"/api/v1/security",
	"/api/v1/runtime",
	"/api/v1/metrics",
	"/api/v1/node-audit-logs",
	"/api/v1/charge-callback",
	"/api/v1/public",
}

// auditCategoryRules 路径前缀 → 分类（审计页「类别」筛选用）。
// 顺序敏感：更具体的前缀放前面。
var auditCategoryRules = []struct{ prefix, category string }{
	{"/api/v1/plan-templates", "plan"},
	{"/api/v1/plan-variants", "plan"},
	{"/api/v1/node-groups", "plan"},
	{"/api/v1/promotions", "plan"},
	{"/api/v1/traffic-packs", "plan"},
	{"/api/v1/plan-orders", "plan"},
	{"/api/v1/clients", "client"},
	{"/api/v1/client-balances", "client"},
	{"/api/v1/client-gifts", "client"},
	{"/api/v1/subscriptions", "subscription"},
	{"/api/v1/subscription-endpoints", "subscription"},
	{"/api/v1/mihomo", "subscription"},
	{"/api/v1/users", "admin"},
	{"/api/v1/roles", "admin"},
	{"/api/v1/system", "system"},
	{"/api/v1/settings", "system"},
	{"/api/v1/waf", "system"},
	{"/api/v1/backups", "system"},
	{"/api/v1/certificates", "system"},
	{"/api/v1/ca", "system"},
	{"/api/v1/acme-accounts", "system"},
	{"/api/v1/cert-channels", "system"},
	{"/api/v1/eab-configs", "system"},
	{"/api/v1/portal", "system"},
	{"/api/v1/log-manage", "system"},
	{"/api/v1/geo-rules", "system"},
	{"/api/v1/cfip", "system"},
	{"/api/v1/warp", "system"},
	{"/api/v1/chainlink", "system"},
	{"/api/v1/services", "system"},
	{"/api/v1/traffic", "system"},
	{"/api/v1/share-domains", "system"},
	{"/api/v1/endpoint-configs", "system"},
	{"/api/v1/ip-whitelists", "system"},
	{"/api/v1/regions", "system"},
	{"/api/v1/referral", "client"},
	{"/api/v1/shared-accounts", "client"},
	{"/api/v1/articles", "system"},
	{"/api/v1/charge-sites", "system"},
	{"/api/v1/charge-cards", "system"},
	{"/api/v1/charge-orders", "system"},
}

// auditActionVerbs POST 子路径里视为「动作」的词（其余 POST 一律 create）。
var auditActionVerbs = map[string]bool{
	"suspend": true, "restore": true, "reset-usage": true, "reset-password": true,
	"rotate": true, "renew": true, "upgrade": true, "downgrade": true,
	"refund": true, "approve": true, "reject": true, "cancel": true,
	"revoke": true, "resolve": true, "reply": true, "close": true,
	"import": true, "sync": true, "sync-all": true, "push-all": true,
	"sync-github": true, "test": true, "install": true, "generate": true,
	"upload-restore": true, "remote-restore": true, "apply": true,
	"start": true, "stop": true, "restart": true, "enable": true, "disable": true,
	"activate": true, "deactivate": true, "assign": true, "unassign": true,
}

// AuditWrites 记录管理端写操作（POST/PUT/PATCH/DELETE）到 security_audit_events。
//
// 之前只有认证类事件有审计，客户/套餐/订阅/系统设置等管理写操作完全没记录。
// 用中间件统一接线：新增接口自动覆盖，不需要每个 handler 各写一遍；
// 具体动作名从路由模板推导（如 client_update / plan_delete / client_suspend），
// 实际路径与状态码写进 reason/metadata 便于追查。
func AuditWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodPatch && method != http.MethodDelete {
			c.Next()
			return
		}

		c.Next()

		route := c.FullPath()
		if route == "" {
			return
		}
		for _, p := range auditWriteExemptPrefixes {
			if strings.HasPrefix(route, p) {
				return
			}
		}

		category, event := auditEventFor(method, route)
		status := c.Writer.Status()
		result := "success"
		if status >= 400 {
			result = "failure"
		}
		audit.Record(audit.Event{
			Category:  category,
			EventType: event,
			UserID:    auditUserID(c),
			RemoteIP:  c.ClientIP(),
			Result:    result,
			Reason:    truncateReason(fmt.Sprintf("%s %s -> %d", method, c.Request.URL.Path, status)),
			Metadata: map[string]any{
				"method": method,
				"route":  route,
				"path":   c.Request.URL.Path,
				"status": status,
			},
		})
	}
}

func auditUserID(c *gin.Context) int64 {
	v, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case string:
		id, _ := strconv.ParseInt(n, 10, 64)
		return id
	}
	return 0
}

// auditEventFor 推导 category 与 event_type（如 client_update、plan_delete、client_suspend）。
func auditEventFor(method, route string) (string, string) {
	category := "admin"
	best := 0
	for _, r := range auditCategoryRules {
		if strings.HasPrefix(route, r.prefix) && len(r.prefix) > best {
			category = r.category
			best = len(r.prefix)
		}
	}

	segments := strings.Split(strings.Trim(route, "/"), "/")
	last := ""
	if len(segments) > 0 {
		last = segments[len(segments)-1]
	}
	verb := ""
	switch method {
	case http.MethodPost:
		if !strings.HasPrefix(last, ":") && auditActionVerbs[last] {
			verb = last
		} else {
			verb = "create"
		}
	case http.MethodPut, http.MethodPatch:
		verb = "update"
	case http.MethodDelete:
		verb = "delete"
	}
	verb = strings.NewReplacer("-", "_", "/", "_").Replace(verb)
	if verb == "" {
		verb = "write"
	}
	return category, category + "_" + verb
}

func truncateReason(s string) string {
	const max = 250
	if len(s) <= max {
		return s
	}
	return s[:max]
}
