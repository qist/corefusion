package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"corefusion/pkg/netutil"
)

// lockOut 把某个 IP 打到锁定（多打几次，避免依赖内部阈值常量）。
func lockOut(limiter *LoginLimiter, ip string) {
	for i := 0; i < 20; i++ {
		limiter.RecordFailure(ip)
	}
}

// 安全审计 v5 S3：限流键如果取自 `c.ClientIP()`，调用方每次换一个 `X-Forwarded-For`
// 就能换掉限流键 —— 口令爆破没有实际限制。
// 断言：**未配置可信代理时，伪造 XFF 不能绕过限流**。
func TestLoginRateLimitIgnoresSpoofedXFFWhenNoTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer netutil.SetTrustedProxies(nil)
	netutil.SetTrustedProxies(nil) // 无可信代理：只信直连地址

	limiter := DefaultLoginLimiter()
	const peer = "198.51.100.7"
	lockOut(limiter, peer)
	defer limiter.RecordSuccess(peer)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	c.Request.RemoteAddr = peer + ":45678"
	// 攻击者尝试用伪造的 XFF "洗掉"自己的失败记录
	c.Request.Header.Set("X-Forwarded-For", "203.0.113.123")

	LoginRateLimit()(c)

	if !c.IsAborted() || w.Code != http.StatusTooManyRequests {
		t.Fatalf("已锁定的直连 IP 即使伪造 XFF 也必须被限流（code=%d aborted=%v）—— "+
			"否则换一个 XFF 就能无限次爆破口令", w.Code, c.IsAborted())
	}
}

// 反向确认：直连方是**已配置的可信代理**时，XFF 里的真实客户端 IP 才是限流对象
// （否则反向代理后面所有用户会共用一个限流键，互相连累）。
func TestLoginRateLimitUsesXFFWhenPeerIsTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer netutil.SetTrustedProxies(nil)
	const proxyIP = "198.51.100.7"
	const realClient = "203.0.113.123"
	netutil.SetTrustedProxies([]string{proxyIP})

	limiter := DefaultLoginLimiter()
	lockOut(limiter, realClient)
	defer limiter.RecordSuccess(realClient)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	c.Request.RemoteAddr = proxyIP + ":45678"
	c.Request.Header.Set("X-Forwarded-For", realClient)

	LoginRateLimit()(c)

	if !c.IsAborted() || w.Code != http.StatusTooManyRequests {
		t.Fatalf("可信代理转发的真实客户端 IP 已被锁定，必须 429（code=%d aborted=%v）—— "+
			"说明限流键没有取到 XFF 里的客户端地址", w.Code, c.IsAborted())
	}
}
