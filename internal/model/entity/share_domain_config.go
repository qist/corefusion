package entity

import "time"

// ShareDomainConfig stores share center domain configuration.
type ShareDomainConfig struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Type      string    `json:"type" gorm:"size:32;not null;index"` // direct, cf_domain, cf_ip
	Value     string    `json:"value" gorm:"size:255;not null"`     // domain or IP
	Port      int       `json:"port" gorm:"not null;default:443"`
	Ports     string    `json:"ports" gorm:"type:text"`             // JSON array of ports
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	SortOrder int       `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (ShareDomainConfig) TableName() string { return "share_domain_configs" }
