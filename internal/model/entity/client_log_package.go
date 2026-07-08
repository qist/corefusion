package entity

import "time"

// ClientLogPackage represents a log packaging task for subscription users.
// These tasks are created when a client requests to download logs from
// their associated entry nodes. The packaged logs are desensitized and
// expire after 1 hour.
type ClientLogPackage struct {
	ID             string    `json:"id" gorm:"primaryKey;type:text"`
	ClientID       int64     `json:"client_id" gorm:"index"`
	SubscriptionID int64     `json:"subscription_id" gorm:"index"`
	ServerID       int64     `json:"server_id" gorm:"index"`
	LogType        string    `json:"log_type"`
	Hours          int       `json:"hours"`
	Status         string    `json:"status"` // processing / ready / failed / expired
	FilePath       string    `json:"file_path"`
	FileSize       int64     `json:"file_size"`
	Error          string    `json:"error,omitempty"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName returns the database table name.
func (ClientLogPackage) TableName() string {
	return "client_log_packages"
}
