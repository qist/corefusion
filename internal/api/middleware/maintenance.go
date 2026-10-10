package middleware

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
)

// MaintenanceConfig holds the in-memory maintenance state.
// The authoritative source is the database (system_settings table,
// key "maintenance.*"), which is loaded on startup and kept in sync
// by SetMaintenance. This allows the state to survive restarts and
// be shared across all Gin engines (main panel, subscription, portal).
type maintenanceState struct {
	mu      sync.RWMutex
	active  bool
	message string
	// source tracks who enabled it: "manual" or "restore"
	source string
}

var maintState = &maintenanceState{
	message: "System is under maintenance. Please try again later.",
}

// SetMaintenance enables or disables maintenance mode.
// source is "manual" for admin toggle, "restore" for auto during restore.
func SetMaintenance(active bool, message, source string) {
	maintState.mu.Lock()
	defer maintState.mu.Unlock()
	maintState.active = active
	if message != "" {
		maintState.message = message
	}
	if source != "" {
		maintState.source = source
	}
	if !active {
		maintState.source = ""
	}
}

// IsMaintenance returns whether maintenance mode is currently active.
func IsMaintenance() bool {
	maintState.mu.RLock()
	defer maintState.mu.RUnlock()
	return maintState.active
}

// GetMaintenanceInfo returns the full maintenance state.
func GetMaintenanceInfo() (active bool, message, source string) {
	maintState.mu.RLock()
	defer maintState.mu.RUnlock()
	return maintState.active, maintState.message, maintState.source
}

// MaintenanceGuard blocks all requests when maintenance mode is active.
// Requests to maintenance-status, auth/login, and health endpoints are
// exempted so the frontend can poll status and users can re-login.
//
// This middleware is safe to mount on ALL engines (main panel, subscription,
// portal) because maintenance state is process-global.
func MaintenanceGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsMaintenance() {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// Always-allowed paths during maintenance
		allowedPrefixes := []string{
			"/api/v1/system/maintenance",     // maintenance status + toggle
			"/api/v1/backups/restore-status", // restore progress polling
			"/api/v1/auth/login",             // admin re-login
			"/api/v1/client-auth/login",      // portal client re-login
			"/api/v1/client-auth/register",   // portal client register
			// 充值站点服务端回调：必须在维护期也放行，否则维护期间已支付订单回调被 503
			// 丢掉 → 充值不到账。主引擎本就未挂 MaintenanceGuard，此处保持一致。
			"/api/v1/charge/callback",
		}
		for _, prefix := range allowedPrefixes {
			if path == prefix || len(path) > len(prefix) && path[:len(prefix)] == prefix {
				c.Next()
				return
			}
		}

		// Allow health check endpoints
		if path == "/health" || path == "/healthz" || path == "/ping" {
			c.Next()
			return
		}

		msg := maintState.message
		maintState.mu.RLock()
		if maintState.message != "" {
			msg = maintState.message
		}
		maintState.mu.RUnlock()

		// Return HTML for browser requests, JSON for API requests
		accept := c.GetHeader("Accept")
		if strings.Contains(accept, "text/html") {
			c.Data(503, "text/html; charset=utf-8", []byte(maintenanceHTML(msg)))
			c.Abort()
			return
		}

		response.Error(c, 503, "MAINTENANCE_MODE", msg)
		c.Abort()
	}
}

// maintenanceHTML returns a user-friendly maintenance page.
// The message is provided by the admin and can be in any language.
func maintenanceHTML(msg string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Maintenance</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);min-height:100vh;display:flex;align-items:center;justify-content:center}
.card{background:#fff;border-radius:16px;padding:48px 64px;text-align:center;box-shadow:0 20px 60px rgba(0,0,0,.3);max-width:500px}
.icon{font-size:64px;margin-bottom:24px}
h1{font-size:24px;color:#303030;margin-bottom:12px}
</style>
</head>
<body>
<div class="card">
<div class="icon">🔧</div>
<h1>` + msg + `</h1>
</div>
</body>
</html>`
}
