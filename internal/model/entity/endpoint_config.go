package entity

import "time"

// EndpointConfig stores VPN endpoint configurations (WireGuard/Tailscale/OpenConnect/OpenVPN).
type EndpointConfig struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Remark       string    `json:"remark" gorm:"size:128"`
	CoreType     string    `json:"core_type" gorm:"size:32;not null;index"`
	EndpointType string    `json:"endpoint_type" gorm:"size:32;not null"`
	Tag          string    `json:"tag" gorm:"size:128;uniqueIndex:idx_endpoint_tag_server"`
	ConfigJSON   string    `json:"config_json" gorm:"type:longtext;not null"`
	Enabled      bool      `json:"enabled" gorm:"not null;default:true"`
	ServerID     int64     `json:"server_id" gorm:"index;not null;default:0;uniqueIndex:idx_endpoint_tag_server"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}

func (EndpointConfig) TableName() string { return "endpoint_configs" }
