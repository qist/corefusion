package entity

import "time"

// OutboundConfig stores outbound configuration separately.
type OutboundConfig struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Remark     string    `json:"remark" gorm:"size:128"`
	CoreType   string    `json:"core_type" gorm:"size:32;not null;index"`
	Protocol   string    `json:"protocol" gorm:"size:32;not null"`
	Tag        string    `json:"tag" gorm:"size:128;uniqueIndex"`
	ConfigJSON string    `json:"config_json" gorm:"type:text;not null"`
	Enabled    bool      `json:"enabled" gorm:"not null;default:true"`
	IsDefault  bool      `json:"is_default" gorm:"not null;default:false"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (OutboundConfig) TableName() string { return "outbound_configs" }
