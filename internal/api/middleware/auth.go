package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/auth/jwt"
)

var jwtHandler *jwt.JWT

// SetJWT sets the JWT handler for the auth middleware.
func SetJWT(j *jwt.JWT) {
	jwtHandler = j
}

// AuthRequired returns a middleware that requires authentication.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if jwtHandler == nil {
			response.InternalError(c, "AUTH_NOT_CONFIGURED", "jwt not configured")
			c.Abort()
			return
		}

		// Check Authorization header first, then query parameter
		tokenStr := ""
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				response.Unauthorized(c, "INVALID_TOKEN_FORMAT", "authorization header must start with Bearer")
				c.Abort()
				return
			}
		} else {
			// Check query parameter for file downloads
			tokenStr = c.Query("token")
			if tokenStr == "" {
				response.Unauthorized(c, "MISSING_TOKEN", "missing authorization header")
				c.Abort()
				return
			}
		}
		if tokenStr == authHeader {
			response.Unauthorized(c, "INVALID_TOKEN_FORMAT", "authorization header must start with Bearer")
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

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}
