package entity

import "time"

// CertDistributionQueueItem represents a pending certificate distribution task.
// When a server is offline, distribution is queued and retried on reconnect.
type CertDistributionQueueItem struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID      int64     `json:"server_id" gorm:"not null;index"`
	CertificateID int64     `json:"certificate_id" gorm:"not null"`
	CertType      string    `json:"cert_type" gorm:"size:32;not null;default:'tls'"`        // tls or agent
	Payload       string    `json:"payload" gorm:"type:longtext"`                           // JSON payload
	Status        string    `json:"status" gorm:"size:16;not null;default:'pending';index"` // pending, in_progress, done, failed
	Attempts      int       `json:"attempts" gorm:"not null;default:0"`
	LastError     string    `json:"last_error" gorm:"size:512"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (CertDistributionQueueItem) TableName() string { return "cert_distribution_queue" }
