package entity

import "time"

// 模板受众：面板使用者（运维通知，发到管理员渠道）/ 订阅者（发给客户本人邮箱）
const (
	TemplateAudienceAdmin      = "admin"
	TemplateAudienceSubscriber = "subscriber"
)

// NotificationTemplate is the GORM model for the notification_templates table.
type NotificationTemplate struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string    `json:"name" gorm:"size:128;not null"`
	Type       string    `json:"type" gorm:"size:32;not null"`    // telegram, email
	Audience   string    `json:"audience" gorm:"size:16;not null;default:admin"` // admin / subscriber
	EventKey   string    `json:"event_key" gorm:"size:64;index"`  // 业务事件键（自动触发；空 = 仅手动）
	Recipients string    `json:"recipients" gorm:"type:longtext"` // admin: comma-separated emails; subscriber 模板忽略（发给客户本人）
	Subject    string    `json:"subject" gorm:"size:255"`
	Content    string    `json:"content" gorm:"type:longtext;not null"`
	Cron       string    `json:"cron" gorm:"size:64"` // empty = manual only
	Enabled    bool      `json:"enabled" gorm:"not null;default:false"`
	LastSentAt time.Time `json:"last_sent_at"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (NotificationTemplate) TableName() string { return "notification_templates" }

// NotificationSendLog 记录订阅者通知的发送（按 天+事件+客户 去重，防止每日扫描重复轰炸）。
type NotificationSendLog struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	TemplateID int64     `json:"template_id" gorm:"index;not null"`
	EventKey   string    `json:"event_key" gorm:"size:64;uniqueIndex:idx_notify_dedup,priority:1"`
	ClientKey  string    `json:"client_key" gorm:"size:255;uniqueIndex:idx_notify_dedup,priority:2"` // 客户邮箱
	Day        string    `json:"day" gorm:"size:10;uniqueIndex:idx_notify_dedup,priority:3"`         // YYYY-MM-DD
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
}

func (NotificationSendLog) TableName() string { return "notification_send_logs" }
