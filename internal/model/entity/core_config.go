package entity

import "time"

// CoreConfig stores configuration per core type per section, per node server.
// Each row = one section of one proxy core's configuration for a specific server.
type CoreConfig struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID   int64     `json:"server_id" gorm:"not null;default:1;uniqueIndex:idx_core_section"`
	CoreType   string    `json:"core_type" gorm:"size:32;not null;uniqueIndex:idx_core_section"`
	Section    string    `json:"section" gorm:"size:64;not null;uniqueIndex:idx_core_section"`
	ConfigJSON string    `json:"config_json" gorm:"type:text;not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (CoreConfig) TableName() string { return "core_configs" }
