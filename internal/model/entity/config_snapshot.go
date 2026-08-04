package entity

import "time"

// ConfigSnapshot stores a configuration version for a node. The agent persists a
// snapshot on every SyncConfig so it can restore the last known-good config locally
// when the panel is unreachable (data localization / node self-healing).
type ConfigSnapshot struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID  int64     `json:"server_id" gorm:"index;not null"`
	CoreType  string    `json:"core_type" gorm:"size:32;index;not null"`
	Version   int64     `json:"version"`
	Config    string    `json:"config" gorm:"type:longtext;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
}

func (ConfigSnapshot) TableName() string { return "config_snapshots" }
