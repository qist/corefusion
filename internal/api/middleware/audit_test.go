package middleware

import (
	"net/http"
	"strings"
	"testing"
)

// 管理写操作审计：分类与动作名从路由模板推导，必须是可读的 `分类_动作`。
func TestAuditEventFor(t *testing.T) {
	cases := []struct {
		method string
		route  string
		cat    string
		event  string
	}{
		{http.MethodPost, "/api/v1/clients", "client", "client_create"},
		{http.MethodPut, "/api/v1/clients/:id", "client", "client_update"},
		{http.MethodDelete, "/api/v1/clients/:id", "client", "client_delete"},
		{http.MethodPost, "/api/v1/clients/:id/suspend", "client", "client_suspend"},
		{http.MethodPost, "/api/v1/clients/:id/reset-usage", "client", "client_reset_usage"},
		{http.MethodPost, "/api/v1/subscriptions/:id/rotate", "subscription", "subscription_rotate"},
		{http.MethodPost, "/api/v1/plan-templates", "plan", "plan_create"},
		{http.MethodDelete, "/api/v1/plan-variants/:id", "plan", "plan_delete"},
		{http.MethodPut, "/api/v1/system/settings", "system", "system_update"},
		{http.MethodPost, "/api/v1/backups/upload-restore", "system", "system_upload_restore"},
		{http.MethodDelete, "/api/v1/certificates/:id", "system", "system_delete"},
		{http.MethodPost, "/api/v1/users", "admin", "admin_create"},
		// 未知前缀 → admin 兜底
		{http.MethodPost, "/api/v1/whatever", "admin", "admin_create"},
	}
	for _, tc := range cases {
		cat, event := auditEventFor(tc.method, tc.route)
		if cat != tc.cat || event != tc.event {
			t.Errorf("%s %s => %s/%s, want %s/%s", tc.method, tc.route, cat, event, tc.cat, tc.event)
		}
	}
}

// 已有专门日志的路径不能再写进安全审计，避免两套表重复/被高频上报写爆。
func TestAuditWriteExempt(t *testing.T) {
	exempt := []string{
		"/api/v1/auth/login",
		"/api/v1/client-auth/login",
		"/api/v1/agent/report",
		"/api/v1/node-servers/:id/sync",
		"/api/v1/security/audit-events",
	}
	for _, route := range exempt {
		found := false
		for _, p := range auditWriteExemptPrefixes {
			if strings.HasPrefix(route, p) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s 应被排除在管理写审计之外", route)
		}
	}
	// 正常业务写操作不能被误排除
	for _, route := range []string{"/api/v1/clients", "/api/v1/plan-templates", "/api/v1/backups/:id/restore"} {
		for _, p := range auditWriteExemptPrefixes {
			if strings.HasPrefix(route, p) {
				t.Errorf("%s 不应被排除（命中 %s）", route, p)
			}
		}
	}
}
