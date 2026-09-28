package entity

import "time"

// ShareDomainConfig stores share center domain configuration.
type ShareDomainConfig struct {
	ID        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Type      string `json:"type" gorm:"size:32;not null;index"` // direct, cf_domain, cf_ip
	Value     string `json:"value" gorm:"size:255;not null"`     // domain or IP
	Port      int    `json:"port" gorm:"not null;default:443"`
	Ports     string `json:"ports" gorm:"type:longtext"` // JSON array of ports
	Enabled   bool   `json:"enabled" gorm:"not null;default:true"`
	SortOrder int    `json:"sort_order" gorm:"not null;default:0"`

	// 多区域：0=全局共享域名（所有服务器通用），>0=仅该服务器使用
	ServerID int64 `json:"server_id" gorm:"index;not null;default:0"`
	// 区域过滤：为空=所有区域，指定区域=仅该区域节点使用
	Region string `json:"region" gorm:"size:64;index;not null;default:''"`

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (ShareDomainConfig) TableName() string { return "share_domain_configs" }
