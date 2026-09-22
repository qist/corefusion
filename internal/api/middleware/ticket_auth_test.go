package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	authsvc "corefusion/internal/service/auth"
)

// 安全审计 v5 M10：SSE/下载这类无法自定义请求头的场景改用**一次性短时票据**，
// 并且**彻底不再接受** `?token=`（长期 JWT 一旦进 URL 就会残留在日志/历史/Referer 里）。
//
// 这里锁住三条关键性质：短时一次性、限作用域、限路径。
func TestTicketAuthRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	InitTicketStore(time.Minute)
	store := TicketStore()
	if store == nil {
		t.Fatal("票据仓库未初始化")
	}

	// 造票的小工具（TokenVersion = -1 表示跳过版本校验，便于单独测票据本身）
	issue := func(scope string) string {
		tk, _, err := store.Issue(authsvc.TicketOptions{
			Scope: scope, Kind: authsvc.TicketKindAdmin,
			UserID: 3, Username: "ops", Role: "admin", TokenVersion: -1,
		})
		if err != nil {
			t.Fatalf("签发票据失败：%v", err)
		}
		return tk
	}

	handler := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	r := gin.New()
	r.GET("/api/v1/log-manage/core/:type/stdout-stream", AuthRequired(), handler)
	r.GET("/api/v1/log-manage/core/:type/stdout-log", AuthRequired(), handler)
	r.GET("/api/v1/backups/:id/download", AuthRequired(), handler)
	r.GET("/api/v1/nodes", AuthRequired(), handler) // 不接受票据的普通接口

	get := func(url string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		r.ServeHTTP(w, req)
		return w.Code
	}

	// ① 票据在日志流路径上可用（EventSource 场景）
	if got := get("/api/v1/log-manage/core/xray/stdout-stream?ticket=" + issue(authsvc.TicketScopeLogs)); got != http.StatusOK {
		t.Errorf("日志流应接受 logs 票据，实际 %d", got)
	}

	// ② 一次性：同一张票据再用必须失败
	tk := issue(authsvc.TicketScopeLogs)
	if got := get("/api/v1/log-manage/core/xray/stdout-log?ticket=" + tk); got != http.StatusOK {
		t.Fatalf("首次使用应通过，实际 %d", got)
	}
	if got := get("/api/v1/log-manage/core/xray/stdout-log?ticket=" + tk); got != http.StatusUnauthorized {
		t.Errorf("票据必须一次性（重放应 401），实际 %d", got)
	}

	// ③ 作用域：日志票据不能拿去下载备份
	if got := get("/api/v1/backups/1/download?ticket=" + issue(authsvc.TicketScopeLogs)); got != http.StatusUnauthorized {
		t.Errorf("日志票据用于备份下载应 401，实际 %d", got)
	}
	// 正确作用域可以
	if got := get("/api/v1/backups/1/download?ticket=" + issue(authsvc.TicketScopeBackupDownload)); got != http.StatusOK {
		t.Errorf("备份票据应用于下载应通过，实际 %d", got)
	}

	// ④ 路径表之外：普通接口不接受票据（失败关闭）
	if got := get("/api/v1/nodes?ticket=" + issue(authsvc.TicketScopeLogs)); got != http.StatusUnauthorized {
		t.Errorf("票据不得用于未登记的路径，实际 %d", got)
	}

	// ⑤ `?token=` 已彻底移除：即使票据机制被绕过，长期 token 也不能再从 URL 进来
	if got := get("/api/v1/backups/1/download?token=some-long-lived-jwt"); got != http.StatusUnauthorized {
		t.Errorf("URL 里的长期 token 必须被拒绝（M10），实际 %d", got)
	}
	if got := get("/api/v1/log-manage/core/xray/stdout-stream?token=some-long-lived-jwt"); got != http.StatusUnauthorized {
		t.Errorf("日志流也不得接受 URL 里的长期 token，实际 %d", got)
	}

	// ⑥ 没有凭证 → 401
	if got := get("/api/v1/log-manage/core/xray/stdout-stream"); got != http.StatusUnauthorized {
		t.Errorf("无凭证应 401，实际 %d", got)
	}
}

// 路径 → 作用域 表本身的行为（票据只在登记的路径上被接受）。
func TestTicketScopeForPath(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/api/v1/log-manage/core/xray/stdout-stream", authsvc.TicketScopeLogs},
		{"/api/v1/log-manage/proxy/singbox/stdout-log", authsvc.TicketScopeLogs},
		{"/api/v1/backups/12/download", authsvc.TicketScopeBackupDownload},
		{"/api/v1/client-auth/logs/packages/9/download", authsvc.TicketScopeClientDownload},
		{"/api/v1/nodes", ""},
		{"/api/v1/backups/12", ""},
		{"/api/v1/backups", ""},
	}
	for _, c := range cases {
		if got := TicketScopeForPath(c.path); got != c.want {
			t.Errorf("TicketScopeForPath(%q) = %q，期望 %q", c.path, got, c.want)
		}
	}
}
