package entity

import "time"

// CoreConfig stores configuration per core type per section.
// Each row = one section of one proxy core's configuration.
type CoreConfig struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	CoreType   string    `json:"core_type" gorm:"size:32;not null;uniqueIndex:idx_core_section"`
	Section    string    `json:"section" gorm:"size:64;not null;uniqueIndex:idx_core_section"`
	ConfigJSON string    `json:"config_json" gorm:"type:text;not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (CoreConfig) TableName() string { return "core_configs" }
