package entity

import "time"

// Plan represents a subscription plan/package.
type Plan struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	QuotaBytes  int64     `gorm:"not null;default:0" json:"quota_bytes"`
	Duration    int       `gorm:"not null;default:30" json:"duration"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Plan) TableName() string { return "plans" }

// PlanNode maps a plan to inbound configs (nodes).
type PlanNode struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PlanID          int64     `gorm:"uniqueIndex:idx_plan_inbound;not null" json:"plan_id"`
	InboundConfigID int64     `gorm:"uniqueIndex:idx_plan_inbound;not null" json:"inbound_config_id"`
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PlanNode) TableName() string { return "plan_nodes" }

// Client represents a subscription client/user.
type Client struct {
	ID             int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Email          string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password       string     `gorm:"size:255" json:"password"`
	PlanID         int64      `gorm:"index" json:"plan_id"`
	SubscriptionID *int64     `gorm:"index" json:"subscription_id"`
	Status         string     `gorm:"size:32;not null;default:active" json:"status"`
	QuotaBytes     int64      `gorm:"not null;default:0" json:"quota_bytes"`
	UsedBytes      int64      `gorm:"not null;default:0" json:"used_bytes"`
	ExpiresAt      *time.Time `json:"expires_at"`
	SuspendedAt    *time.Time `json:"suspended_at"`
	Remark         string     `gorm:"size:255" json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (Client) TableName() string { return "clients" }
