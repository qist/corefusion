// Package audit provides a package-level, fire-and-forget writer for
// security audit events (security_audit_events table).
//
// Usage:
//
//	audit.Register(db)                          // once at startup (router)
//	audit.Record(audit.Event{ ... })            // anywhere, non-blocking
//
// If Register was never called (e.g. agent-only build), Record is a no-op.
package audit

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// Event describes a security audit event to record.
type Event struct {
	Category   string // e.g. "auth", "access", "admin"
	EventType  string // e.g. "admin_login", "client_login", "ip_ban_created"
	ServiceID  int64  // 0 = none
	EndpointID int64  // 0 = none
	UserID     int64  // 0 = none
	RemoteIP   string
	Result     string // "success" | "failure" | "denied"
	Reason     string
	Metadata   map[string]any // optional, serialized to metadata_json
}

var (
	mu sync.RWMutex
	db *gorm.DB
)

// Register sets the database used for audit writes. Call once at startup.
func Register(database *gorm.DB) {
	mu.Lock()
	defer mu.Unlock()
	db = database
}

// Record writes an audit event asynchronously. Never blocks, never fails the caller.
func Record(ev Event) {
	mu.RLock()
	database := db
	mu.RUnlock()
	if database == nil {
		return
	}

	row := &entity.SecurityAuditEvent{
		Category:  ev.Category,
		EventType: ev.EventType,
		RemoteIP:  ev.RemoteIP,
		Result:    ev.Result,
		Reason:    truncate(ev.Reason, 255),
		CreatedAt: time.Now(),
	}
	if ev.ServiceID != 0 {
		row.ServiceID = &ev.ServiceID
	}
	if ev.EndpointID != 0 {
		row.EndpointID = &ev.EndpointID
	}
	if ev.UserID != 0 {
		row.UserID = &ev.UserID
	}
	if len(ev.Metadata) > 0 {
		if b, err := json.Marshal(ev.Metadata); err == nil {
			row.MetadataJSON = string(b)
		}
	}

	go func() {
		if err := database.Create(row).Error; err != nil {
			log.Printf("[Audit] failed to record event %s/%s: %v", ev.Category, ev.EventType, err)
		}
	}()
}

// truncate ensures a string does not exceed n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// NodeEvent describes a node-level administrative audit event to record.
// These are written to the node_audit_logs table and displayed in the
// Node Servers → Audit tab in the admin UI.
type NodeEvent struct {
	ServerID  int64  // 0 = global/not node-specific
	AdminID   int64  // 0 = system/unknown
	Action    string // e.g. create, update, delete, control_core, save_protocol
	Detail    string
	SourceIP  string
	UserAgent string
	Success   bool
	ErrMsg    string
}

// RecordNode writes a node audit log entry asynchronously. Never blocks, never fails the caller.
func RecordNode(ev NodeEvent) {
	mu.RLock()
	database := db
	mu.RUnlock()
	if database == nil {
		return
	}

	row := &entity.NodeAuditLog{
		ServerID:     ev.ServerID,
		AdminID:      ev.AdminID,
		Action:       truncate(ev.Action, 64),
		Detail:       ev.Detail,
		SourceIP:     truncate(ev.SourceIP, 64),
		UserAgent:    truncate(ev.UserAgent, 256),
		Success:      ev.Success,
		ErrorMessage: truncate(ev.ErrMsg, 512),
		CreatedAt:    time.Now(),
	}

	go func() {
		if err := database.Create(row).Error; err != nil {
			log.Printf("[Audit] failed to record node event %s: %v", ev.Action, err)
		}
	}()
}

// RecordNodeFromCtx is a convenience wrapper that extracts admin ID, source IP,
// and user agent from a gin context, then calls RecordNode.
func RecordNodeFromCtx(c *gin.Context, serverID int64, action, detail string, success bool, errMsg string) {
	ev := NodeEvent{
		ServerID:  serverID,
		Action:    action,
		Detail:    detail,
		SourceIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Success:   success,
		ErrMsg:    errMsg,
	}
	if uid, ok := c.Get("user_id"); ok {
		if v, ok := uid.(int64); ok {
			ev.AdminID = v
		}
	}
	RecordNode(ev)
}
