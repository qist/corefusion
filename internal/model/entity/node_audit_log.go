package entity

import "time"

// NodeAuditLog records an administrative operation on a node server.
type NodeAuditLog struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID     int64     `json:"server_id" gorm:"not null;index"`
	AdminID      int64     `json:"admin_id" gorm:"not null;index"`
	Action       string    `json:"action" gorm:"size:64;not null;index"` // e.g. create, update, delete, control, sync, revoke, distribute
	Detail       string    `json:"detail" gorm:"type:longtext"`
	SourceIP     string    `json:"source_ip" gorm:"size:64"`
	UserAgent    string    `json:"user_agent" gorm:"size:256"`
	Success      bool      `json:"success" gorm:"not null;default:true"`
	ErrorMessage string    `json:"error_message" gorm:"size:512"`
	CreatedAt    time.Time `json:"created_at" gorm:"index;not null"`
}

func (NodeAuditLog) TableName() string { return "node_audit_logs" }
