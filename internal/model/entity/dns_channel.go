package entity

import "time"

// DNSChannel is the GORM model for the dns_channels table.
type DNSChannel struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"size:128;not null"`
	Provider  string    `json:"provider" gorm:"size:32;not null"`
	EnvVars   string    `json:"env_vars" gorm:"type:longtext"` // JSON: {"KEY":"value"}
	Status    string    `json:"status" gorm:"size:32;not null;default:'active'"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (DNSChannel) TableName() string { return "dns_channels" }
