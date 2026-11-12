package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 订阅被限流（429）时必须回调记录钩子：429 在 handler 之前返回，
// 不带钩子的话管理端「订阅访问日志」永远看不到限流记录。
func TestSubscribeRateLimitRecordsDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var gotStatus int
	var gotHit string
	SetSubscribeDeniedRecorder(func(_ *gin.Context, status int, hit string) {
		gotStatus, gotHit = status, hit
	})
	t.Cleanup(func() { SetSubscribeDeniedRecorder(nil) })

	const ip = "203.0.113.77"
	// 直接耗尽该 IP 的配额，避免发几百个真实请求。
	for i := 0; i < subscribeRateLimit; i++ {
		subscribeLimiter.Allow(ip)
	}

	r := gin.New()
	r.GET("/sub/:token", SubscribeRateLimit(), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/sub/some-token", nil)
	req.RemoteAddr = ip + ":34567"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if gotStatus != http.StatusTooManyRequests || gotHit != "rate_limited" {
		t.Fatalf("钩子收到 %d/%q, want 429/rate_limited", gotStatus, gotHit)
	}
}

// 未被限流时不能触发记录钩子（否则会把正常请求重复记一遍）。
func TestSubscribeRateLimitAllowsNormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	called := false
	SetSubscribeDeniedRecorder(func(_ *gin.Context, _ int, _ string) { called = true })
	t.Cleanup(func() { SetSubscribeDeniedRecorder(nil) })

	r := gin.New()
	r.GET("/sub/:token", SubscribeRateLimit(), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/sub/another-token", nil)
	req.RemoteAddr = "203.0.113.88:34567"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if called {
		t.Fatal("正常请求不应触发限流记录钩子")
	}
}
