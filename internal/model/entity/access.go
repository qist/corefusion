package entity

import "time"

// ServiceAccessPolicy is the GORM model for service access policies.
type ServiceAccessPolicy struct {
	ID                       int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServiceID                int64     `json:"service_id" gorm:"index;not null"`
	EndpointID               *int64    `json:"endpoint_id"`
	PolicyScope              string    `json:"policy_scope" gorm:"size:32;not null"`
	Mode                     string    `json:"mode" gorm:"size:32;not null"`
	IPCIDR                   string    `json:"ip_cidr" gorm:"size:64"`
	UserID                   *int64    `json:"user_id"`
	SubscriptionID           *int64    `json:"subscription_id"`
	MaxNewConnectionsPerMin  int       `json:"max_new_connections_per_min" gorm:"not null;default:0"`
	MaxConcurrentConnections int       `json:"max_concurrent_connections" gorm:"not null;default:0"`
	BanDurationSeconds       int       `json:"ban_duration_seconds" gorm:"not null;default:600"`
	AutoBanEnabled           int       `json:"auto_ban_enabled" gorm:"not null;default:0"`
	Status                   int       `json:"status" gorm:"not null;default:1"`
	CreatedAt                time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt                time.Time `json:"updated_at" gorm:"not null"`
}

func (ServiceAccessPolicy) TableName() string { return "service_access_policies" }

// ServiceIPBan is the GORM model for IP bans.
type ServiceIPBan struct {
	ID         int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	ServiceID  int64      `json:"service_id" gorm:"index;not null"`
	EndpointID *int64     `json:"endpoint_id"`
	IPCIDR     string     `json:"ip_cidr" gorm:"size:64;not null"`
	Reason     string     `json:"reason" gorm:"size:255"`
	Source     string     `json:"source" gorm:"size:32;not null"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt  time.Time  `json:"updated_at" gorm:"not null"`
}

func (ServiceIPBan) TableName() string { return "service_ip_bans" }

// SecurityAuditEvent is the GORM model for audit events.
type SecurityAuditEvent struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Category     string    `json:"category" gorm:"size:32;not null"`
	EventType    string    `json:"event_type" gorm:"size:64;not null"`
	ServiceID    *int64    `json:"service_id"`
	EndpointID   *int64    `json:"endpoint_id"`
	UserID       *int64    `json:"user_id"`
	RemoteIP     string    `json:"remote_ip" gorm:"size:64"`
	Result       string    `json:"result" gorm:"size:32"`
	Reason       string    `json:"reason" gorm:"size:255"`
	MetadataJSON string    `json:"metadata_json" gorm:"type:longtext"`
	CreatedAt    time.Time `json:"created_at" gorm:"index;not null"`
}

func (SecurityAuditEvent) TableName() string { return "security_audit_events" }
