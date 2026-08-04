package entity

import "time"

// NotificationTemplate is the GORM model for the notification_templates table.
type NotificationTemplate struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string    `json:"name" gorm:"size:128;not null"`
	Type       string    `json:"type" gorm:"size:32;not null"` // telegram, email
	Recipients string    `json:"recipients" gorm:"type:longtext"`  // comma-separated emails or chat IDs
	Subject    string    `json:"subject" gorm:"size:255"`
	Content    string    `json:"content" gorm:"type:longtext;not null"`
	Cron       string    `json:"cron" gorm:"size:64"` // empty = manual only
	Enabled    bool      `json:"enabled" gorm:"not null;default:false"`
	LastSentAt time.Time `json:"last_sent_at"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (NotificationTemplate) TableName() string { return "notification_templates" }
