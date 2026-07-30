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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
