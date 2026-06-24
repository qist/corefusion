package entity

import "time"

// SubscriptionEndpoint represents a public subscription endpoint with domain/path routing.
type SubscriptionEndpoint struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Domain      string    `gorm:"size:255;not null;index" json:"domain"`
	Path        string    `gorm:"size:255;default:'/'" json:"path"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	Description string    `gorm:"size:500" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SubscriptionEndpoint) TableName() string { return "subscription_endpoints" }
