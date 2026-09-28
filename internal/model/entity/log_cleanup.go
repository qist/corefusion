package entity

import "time"

// LogCleanupRule defines a cleanup rule for a database log/event table.
// Users can configure retention days, enable/disable, and cleanup interval per table.
type LogCleanupRule struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Category    string    `json:"category" gorm:"size:32;not null;index"`                           // traffic, audit, runtime, etc.
	Table       string    `json:"table_name" gorm:"column:table_name;size:64;not null;uniqueIndex"` // DB table name
	DisplayName string    `json:"display_name" gorm:"size:128;not null"`                            // human-friendly name
	RetainDays  int       `json:"retain_days" gorm:"not null;default:30"`                           // delete records older than N days
	Enabled     bool      `json:"enabled" gorm:"not null;default:true"`
	CleanupCron string    `json:"cleanup_cron" gorm:"size:64;not null;default:'0 */6 * * *'"` // cron-like interval (currently parsed as hours)
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"not null"`
}

func (LogCleanupRule) TableName() string { return "log_cleanup_rules" }
