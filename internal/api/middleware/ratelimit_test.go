package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// 安全审计 v5 M4：公开订阅/注册端点原先**完全没有限流** —— token 虽不可枚举，
// 但已泄露的 token 可被无限抓取，且每次访问都会写库（可放大成写压力）。
func TestRateLimiterAllowsThenBlocks(t *testing.T) {
	l := newRateLimiter(3, 50*time.Millisecond)

	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("第 %d 次应放行（限额=3）", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Error("超过限额必须被拦")
	}
	// 不同 key 互不影响
	if !l.Allow("5.6.7.8") {
		t.Error("另一个 IP 不应受影响")
	}
	// 窗口滑过后恢复
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("1.2.3.4") {
		t.Error("窗口滑过后应恢复放行")
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := newRateLimiter(2, time.Minute)
	mw := rateLimitByIP(l, "TOO_MANY_REQUESTS", "slow down")

	call := func() int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/sub/abc", nil)
		c.Request.RemoteAddr = "9.9.9.9:1234"
		mw(c)
		if !c.IsAborted() {
			c.Next()
		}
		return w.Code
	}
	if got := call(); got != http.StatusOK {
		t.Fatalf("前两次应放行，得到 %d", got)
	}
	_ = call()
	if got := call(); got != http.StatusTooManyRequests {
		t.Errorf("超限应 429，得到 %d", got)
	}
}

// 安全审计 v5 M9：多数 handler 直接 io.ReadAll，原先没有请求体上限。
func TestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := BodyLimit(1024)

	run := func(contentType, body string, contentLength int64) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/x", strings.NewReader(body))
		if contentType != "" {
			c.Request.Header.Set("Content-Type", contentType)
		}
		if contentLength >= 0 {
			c.Request.ContentLength = contentLength
		}
		mw(c)
		if c.IsAborted() {
			return w.Code
		}
		// handler 读到底：只有 `*http.MaxBytesError` 才代表"被限额拦住" ✗
		// （不能把所有 err 都当成被拦 —— 空 body 会返回 io.EOF，那是正常结束）
		buf := make([]byte, 4096)
		_, err := c.Request.Body.Read(buf)
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return 0
		}
		c.Next()
		return w.Code
	}

	if got := run("application/json", "small", -1); got != http.StatusOK {
		t.Errorf("小请求体应放行，得到 %d", got)
	}
	if got := run("application/json", "", 2048); got != http.StatusRequestEntityTooLarge {
		t.Errorf("Content-Length 超限应 413，得到 %d", got)
	}
	// multipart 必须跳过（备份还原/证书上传合法地很大）
	if got := run("multipart/form-data; boundary=x", "", 1<<20); got != http.StatusOK {
		t.Errorf("multipart 不应被 BodyLimit 拦截，得到 %d", got)
	}
}

// 安全审计 v5 M10：管理端支持 `?token=`（前端下载/日志流在用），
// 而 gin 默认访问日志会把完整 query 写进日志 → JWT 明文落盘。
func TestRedactTokenQuery(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/api/v1/backups/1/download?token=SECRET", "/api/v1/backups/1/download?token=***"},
		{"/api/v1/logs?server_id=2&token=SECRET&lines=100", "/api/v1/logs?server_id=2&token=***&lines=100"},
		{"/api/v1/logs?access_token=SECRET", "/api/v1/logs?access_token=***"},
		{"/api/v1/nodes", "/api/v1/nodes"}, // 没有 token 不动
	}
	for _, c := range cases {
		if got := RedactTokenQuery(c.in); got != c.want {
			t.Errorf("RedactTokenQuery(%q)=%q，期望 %q", c.in, got, c.want)
		}
		if strings.Contains(RedactTokenQuery(c.in), "SECRET") {
			t.Errorf("token 明文泄漏：%q", RedactTokenQuery(c.in))
		}
	}
}

func TestAccessLogFormatterRedactsToken(t *testing.T) {
	line := AccessLogFormatter(gin.LogFormatterParams{
		TimeStamp:  time.Now(),
		Method:     "GET",
		Path:       "/api/v1/backups/9/download?token=JWT-SECRET&x=1",
		StatusCode: 200,
		Latency:    time.Millisecond,
		ClientIP:   "10.0.0.1",
	})
	if strings.Contains(line, "JWT-SECRET") {
		t.Fatalf("访问日志里出现了 token 明文：%s", line)
	}
	if !strings.Contains(line, "token=***") || !strings.Contains(line, "x=1") {
		t.Errorf("应只打码 token、保留其它参数：%s", line)
	}
}
