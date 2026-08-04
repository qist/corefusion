package entity

import "time"

// LogRotationPolicy defines a log rotation policy for a specific log file.
type LogRotationPolicy struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID   int64     `json:"server_id" gorm:"not null;default:1;index:idx_log_policy_server"` // 1 = local node
	Name       string    `json:"name" gorm:"size:128;not null"`
	SourceType string    `json:"source_type" gorm:"size:16;not null;default:custom"` // nginx, caddy, custom
	LogPath    string    `json:"log_path" gorm:"size:512;not null"`
	Enabled    bool      `json:"enabled" gorm:"not null;default:true"`
	RotateCron string    `json:"rotate_cron" gorm:"size:64;not null;default:'0 0 * * *'"` // daily midnight
	RetainDays int       `json:"retain_days" gorm:"not null;default:30"`
	MaxSize    int64     `json:"max_size" gorm:"not null;default:0"` // 0 = no limit, bytes
	Compress   bool      `json:"compress" gorm:"not null;default:false"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (LogRotationPolicy) TableName() string { return "log_rotation_policies" }

// LogRotationJob records a single log rotation execution.
type LogRotationJob struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	PolicyID     int64     `json:"policy_id" gorm:"not null;index"`
	Status       string    `json:"status" gorm:"size:16;not null"` // success, failed
	RotatedFiles int       `json:"rotated_files" gorm:"not null;default:0"`
	CleanedFiles int       `json:"cleaned_files" gorm:"not null;default:0"`
	FreedBytes   int64     `json:"freed_bytes" gorm:"not null;default:0"`
	ErrorMessage string    `json:"error_message" gorm:"type:longtext"`
	StartedAt    time.Time `json:"started_at" gorm:"not null"`
	FinishedAt   time.Time `json:"finished_at" gorm:"not null"`
}

func (LogRotationJob) TableName() string { return "log_rotation_jobs" }
