package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/pkg/netutil"
)

// 本文件提供两类与"公开端点"相关的保护（安全审计 v5 M4 / M9 / M10）：
//
//	M4 —— 公开订阅 / 注册 / public API 的**请求速率**限流：token 虽不可枚举，
//	      但已泄露的 token 可以被无限抓取；且订阅端点每次访问都会写库（access log /
//	      限速关联），可被放大成 DB 写压力。
//	M9 —— 请求体大小上限：多数 handler 直接 io.ReadAll，没有任何上限。
//	M10 —— 访问日志脱敏：管理端鉴权支持 `?token=`（前端下载/日志流确实在用），
//	      而 gin 默认日志会把 query 原样写进访问日志 → JWT 泄漏到日志里。

// ---------------------------------------------------------------------------
// 通用请求速率限流
// ---------------------------------------------------------------------------

// rateLimiter 按 key（客户端 IP）在滑动窗口内限制**请求次数**。
//
// 注意与 LoginLimiter 的区别：后者数的是"失败次数"（用于爆破防护，成功即清零），
// 这里数的是"请求次数"（用于防抓取/防写放大）。两者的语义与生命周期都不同，不能混用。
//
// 状态是**进程内**的：多副本部署时实际限额 = 限额 × 副本数。对"减速"够用；要精确需换 Redis。
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	lastGC time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	if limit <= 0 || window <= 0 {
		panic("newRateLimiter: limit and window must be > 0")
	}
	return &rateLimiter{hits: make(map[string][]time.Time), limit: limit, window: window, lastGC: time.Now()}
}

// Allow 记录一次请求并返回是否放行。
func (l *rateLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// 周期性清理过期 key，避免长期运行时 map 无限增长。
	if now.Sub(l.lastGC) > l.window {
		cutoff := now.Add(-l.window)
		for k, ts := range l.hits {
			if len(ts) == 0 || ts[len(ts)-1].Before(cutoff) {
				delete(l.hits, k)
			}
		}
		l.lastGC = now
	}

	cutoff := now.Add(-l.window)
	kept := l.hits[key][:0]
	for _, ts := range l.hits[key] {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// 公开端点的限额。取值都偏宽松：订阅会被客户端定时刷新、同一出口 IP 后面常有多台设备/NAT，
// 目的是"拦住抓取与放大"，而不是卡正常用户。
const (
	subscribeRateLimit  = 240
	subscribeRateWindow = time.Minute

	registerRateLimit  = 10
	registerRateWindow = 10 * time.Minute

	publicAPIRateLimit  = 120
	publicAPIRateWindow = time.Minute
)

var (
	subscribeLimiter = newRateLimiter(subscribeRateLimit, subscribeRateWindow)
	registerLimiter  = newRateLimiter(registerRateLimit, registerRateWindow)
	publicAPILimiter = newRateLimiter(publicAPIRateLimit, publicAPIRateWindow)
)

// rateLimitByIP 构造一个按客户端 IP 限流的中间件。
//
// ⚠ key 必须走 `netutil.GetRealIP` ✗ 不能用 `c.ClientIP()`（Gin 默认信任所有代理，
// 换个 X-Forwarded-For 就能换 key —— 与审计 v5 S3 同一类问题）。
func rateLimitByIP(l *rateLimiter, code, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(netutil.GetRealIP(c.Request)) {
			response.TooManyRequests(c, code, message)
			c.Abort()
			return
		}
		c.Next()
	}
}

// SubscribeRateLimit 挂在公开订阅端点上（`/sub/:token`、根路径 `/:token`）。
func SubscribeRateLimit() gin.HandlerFunc {
	return rateLimitByIP(subscribeLimiter, "TOO_MANY_REQUESTS",
		"too many subscription requests, please retry later")
}

// RegisterRateLimit 挂在 Portal 注册 / 刷新端点上。
func RegisterRateLimit() gin.HandlerFunc {
	return rateLimitByIP(registerLimiter, "TOO_MANY_REQUESTS",
		"too many requests, please retry later")
}

// PublicAPIRateLimit 挂在 `/api/v1/public/*` 这类无需登录的读接口上。
func PublicAPIRateLimit() gin.HandlerFunc {
	return rateLimitByIP(publicAPILimiter, "TOO_MANY_REQUESTS",
		"too many requests, please slow down")
}

// ---------------------------------------------------------------------------
// M9：请求体大小上限
// ---------------------------------------------------------------------------

// DefaultMaxBodyBytes 是普通（非 multipart）请求体的上限：32 MiB。
// 面板里的配置/规则文本远小于此；而备份还原、证书上传等走 multipart（见 BodyLimit 的跳过逻辑），
// 它们本身是流式落盘，不在这里限制。
const DefaultMaxBodyBytes int64 = 32 << 20

// BodyLimit 限制请求体大小，超限直接 413（而不是让 handler 把超大 body 读进内存）。
//
// 跳过 multipart/form-data：备份还原、证书上传等合法请求体可能很大，
// 且它们已经是流式处理；这些路径的超限保护应各自按业务需要单独设定。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	return func(c *gin.Context) {
		if c.Request == nil || c.Request.Body == nil {
			c.Next()
			return
		}
		if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/") {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			response.Error(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE",
				"request body too large")
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// ---------------------------------------------------------------------------
// M10：访问日志脱敏
// ---------------------------------------------------------------------------

// tokenQueryRe 匹配 URL 里的 token / access_token 查询参数值。
var tokenQueryRe = regexp.MustCompile(`(?i)([?&](?:token|access_token|ticket|bootstrap_token)=)[^&\s"]*`)

// RedactTokenQuery 把 URL 中的 token 值打码。
//
// 为什么需要它：SSE/下载这类场景浏览器无法自定义 Authorization 头，凭证只能进 URL
// （安全审计 v5 M10 后已改为**一次性短时票据** `?ticket=`，但仍不该把明文写进日志），
// 而 gin 默认访问日志会把完整 query 写进日志文件。
// 这里**不改变**鉴权行为，只让日志不再记录明文 token / ticket / bootstrap_token。
func RedactTokenQuery(s string) string {
	lower := strings.ToLower(s)
	if !strings.Contains(lower, "token=") && !strings.Contains(lower, "ticket=") {
		return s
	}
	return tokenQueryRe.ReplaceAllString(s, "${1}***")
}

// AccessLogFormatter 是替代 `gin.Logger()` 的访问日志格式：与默认格式基本一致，
// 但会把 token 查询参数打码。
func AccessLogFormatter(param gin.LogFormatterParams) string {
	path := RedactTokenQuery(param.Path)
	return "[" + param.TimeStamp.Format(time.RFC3339) + "] " +
		param.Method + " " + path + " " +
		"| " + strconv.Itoa(param.StatusCode) + " | " +
		param.Latency.String() + " | " + param.ClientIP + " | " +
		param.ErrorMessage + "\n"
}
