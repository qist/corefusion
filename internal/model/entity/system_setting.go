package entity

import "time"

// SystemSetting is the GORM model for the system_settings table.
type SystemSetting struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key       string    `json:"key" gorm:"size:128;uniqueIndex;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (SystemSetting) TableName() string { return "system_settings" }
