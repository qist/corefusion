package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/auth/jwt"
	authsvc "corefusion/internal/service/auth"
)

var jwtHandler *jwt.JWT

// TokenVersionProvider 提供"用户当前的令牌版本"，用于让**登出/踢下线立即生效**
// （JWT 本身无状态，只靠过期时间的话，被窃取的 token 在有效期内一直可用 —— 审计 v5 M8）。
type TokenVersionProvider interface {
	GetTokenVersion(ctx context.Context, userID int64) (int, error)
}

var tokenVersionProvider TokenVersionProvider

// SetTokenVersionProvider 注入令牌版本来源（router 装配时调用）。
func SetTokenVersionProvider(p TokenVersionProvider) {
	tokenVersionProvider = p
}

// ── 一次性短时票据（安全审计 v5 M10）──────────────────────────────
//
// 浏览器的 `EventSource`（日志 SSE 流）与 `window.open`（文件下载）**无法自定义
// Authorization 头** ✗ → 前端只能把凭证放进 URL ✗，而 URL 会进访问日志、Referer、
// 浏览器历史、反向代理日志 → 等于把**长期有效的 JWT** 到处散播 ✗。
//
// 修法（全自动，前端不需要用户做任何事）：前端先调 `POST /api/v1/auth/ticket`
// 拿一张 **60 秒、一次性、限路径** 的票据，再用 `?ticket=` 建流/下载。
// 票据的作用域由**路径**决定（见 ticketRouteScopes）→ 日志票据拿去下载备份也没用。
var ticketStore *authsvc.TicketStore

// InitTicketStore 创建票据仓库（router 装配时调用，与 SetTokenVersionProvider 并列）。
func InitTicketStore(ttl time.Duration) *authsvc.TicketStore {
	ticketStore = authsvc.NewTicketStore(ttl)
	return ticketStore
}

// TicketStore 返回票据仓库（供签发端点使用）。
func TicketStore() *authsvc.TicketStore {
	return ticketStore
}

// CurrentTokenVersion 读取用户当前令牌版本（签发票据时写进票据，登出后票据也随之失效）。
func CurrentTokenVersion(ctx context.Context, userID int64) (int, error) {
	if tokenVersionProvider == nil {
		return 0, nil
	}
	return tokenVersionProvider.GetTokenVersion(ctx, userID)
}

// ticketRouteScopes 是"路径 → 票据作用域"表：票据**只在这些路径上**被接受 ✓
// （不在表里的路径完全不认票据 → 失败关闭）。
var ticketRouteScopes = []struct {
	contains string
	suffix   string
	scope    string
}{
	{"/log-manage/", "/stdout-log", authsvc.TicketScopeLogs},
	{"/log-manage/", "/stdout-stream", authsvc.TicketScopeLogs},
	{"/backups/", "/download", authsvc.TicketScopeBackupDownload},
	// 证书下载 ✓：/api/v1/certificates/<id>/download 与 /api/v1/certificates/export ✓
	{"/certificates/", "/download", authsvc.TicketScopeCertDownload},
	{"/certificates/", "/export", authsvc.TicketScopeCertDownload},
	{"/client-auth/logs/", "/download", authsvc.TicketScopeClientDownload},
}

// TicketScopeForPath 返回该路径允许的票据作用域（空串 = 该路径不接受票据）。
func TicketScopeForPath(path string) string {
	for _, r := range ticketRouteScopes {
		if strings.Contains(path, r.contains) && strings.HasSuffix(path, r.suffix) {
			return r.scope
		}
	}
	return ""
}

// SetJWT sets the JWT handler for the auth middleware.
func SetJWT(j *jwt.JWT) {
	jwtHandler = j
}

// AuthRequired returns a middleware that requires authentication.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 注意：jwtHandler 的空值检查放在 **token 校验之前**（见下）——票据分支不需要
		// JWT 配置，放在这里会把票据路径一起挡掉 ✗。

		// ① Authorization 头（常规路径）
		tokenStr := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				response.Unauthorized(c, "INVALID_TOKEN_FORMAT", "authorization header must start with Bearer")
				c.Abort()
				return
			}
		} else if ticket := c.Query("ticket"); ticket != "" {
			// ② 一次性短时票据（仅 SSE/下载这类无法自定义 Header 的场景；见 ticketRouteScopes）
			// ⚠ 这里**不再接受** `?token=` ✗（安全审计 v5 M10）：长期 JWT 一旦进 URL 就会残留在
			// 日志/历史/Referer 里。旧前端若还在用 ?token= 会直接 401（失败关闭，而不是继续泄漏）。
			if ticketStore == nil {
				response.Unauthorized(c, "TICKET_NOT_CONFIGURED", "ticket auth not configured")
				c.Abort()
				return
			}
			scope := TicketScopeForPath(c.Request.URL.Path)
			if scope == "" {
				response.Unauthorized(c, "TICKET_NOT_ALLOWED",
					"one-time tickets are not accepted on this endpoint")
				c.Abort()
				return
			}
			t, ok := ticketStore.Consume(ticket, scope)
			if !ok {
				response.Unauthorized(c, "TICKET_INVALID",
					"ticket is invalid, already used or expired; request a new one")
				c.Abort()
				return
			}
			// 票据里带的角色同样要过管理端角色检查（与 token 路径一致）
			if RankOf(t.Role) <= 0 {
				response.Forbidden(c, "ROLE_NOT_ALLOWED", "this ticket is not allowed to access the admin API")
				c.Abort()
				return
			}
			// 登出（令牌版本 +1）后，之前签发的票据也一并失效
			if tokenVersionProvider != nil && t.TokenVersion != -1 {
				tv, terr := tokenVersionProvider.GetTokenVersion(c.Request.Context(), t.UserID)
				if terr != nil || tv != t.TokenVersion {
					response.Unauthorized(c, "TOKEN_REVOKED", "token has been revoked; please sign in again")
					c.Abort()
					return
				}
			}
			c.Set("user_id", t.UserID)
			c.Set("username", t.Username)
			c.Set("role", t.Role)
			c.Next()
			return
		} else {
			response.Unauthorized(c, "MISSING_TOKEN", "missing authorization header")
			c.Abort()
			return
		}
		if tokenStr == authHeader {
			response.Unauthorized(c, "INVALID_TOKEN_FORMAT", "authorization header must start with Bearer")
			c.Abort()
			return
		}

		if jwtHandler == nil {
			response.InternalError(c, "AUTH_NOT_CONFIGURED", "jwt not configured")
			c.Abort()
			return
		}

		claims, err := jwtHandler.ValidateTyped(tokenStr, jwt.TokenTypeAccess)
		if err != nil {
			response.Unauthorized(c, "INVALID_TOKEN", err.Error())
			c.Abort()
			return
		}

		// ⚠ 管理端必须拒绝**非管理端角色**的 token ✗（安全审计 v5 S2）：
		// Portal 客户端 token 与管理端 token 用**同一把密钥**、结构完全一致，只有 role 不同
		// （客户端是 "client"）；而大量管理端路由只挂了 AuthRequired、没挂 RequireRole ——
		// 结果是任一登录的 Portal 用户都能调 `/api/v1/nodes`、`/clients`、`/subscriptions` 等。
		// 这里按 rbac 的 RankOf 拒绝（未收录的角色返回 0，天然"失败关闭"）：
		// 不仅是 client，将来任何新 token 类型忘了配角色也不会被放进管理端。
		if RankOf(claims.Role) <= 0 {
			response.Forbidden(c, "ROLE_NOT_ALLOWED",
				"this token is not allowed to access the admin API")
			c.Abort()
			return
		}

		// 令牌版本校验：登出/踢下线后版本 +1，旧 token 立刻失效 ✗ 否则"登出"只是前端丢 token，
		// 被窃取的 token 在有效期内（access 24h / refresh 7d）仍然可用（审计 v5 M8）。
		if tokenVersionProvider != nil {
			tv, terr := tokenVersionProvider.GetTokenVersion(c.Request.Context(), claims.UserID)
			if terr != nil {
				response.Unauthorized(c, "INVALID_TOKEN", "token subject not found")
				c.Abort()
				return
			}
			if tv != claims.TokenVersion {
				response.Unauthorized(c, "TOKEN_REVOKED",
					"token has been revoked; please sign in again")
				c.Abort()
				return
			}
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}
