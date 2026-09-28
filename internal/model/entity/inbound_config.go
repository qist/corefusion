package entity

import "time"

// InboundConfig stores inbound configuration separately.
type InboundConfig struct {
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Remark     string `json:"remark" gorm:"size:128"`
	CoreType   string `json:"core_type" gorm:"size:32;not null;index"`
	Protocol   string `json:"protocol" gorm:"size:32;not null"`
	Tag        string `json:"tag" gorm:"size:128;uniqueIndex:idx_inbound_tag_server"`
	Listen     string `json:"listen" gorm:"size:256"`
	Port       int    `json:"port"`
	ConfigJSON string `json:"config_json" gorm:"type:longtext;not null"`
	Enabled    bool   `json:"enabled" gorm:"not null"`

	// 多区域：关联的 NodeServer ID，0=本地节点（兼容旧数据）
	ServerID int64  `json:"server_id" gorm:"index;not null;default:0;uniqueIndex:idx_inbound_tag_server"`
	Region   string `json:"region" gorm:"size:64;index;not null;default:''"`

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (InboundConfig) TableName() string { return "inbound_configs" }
