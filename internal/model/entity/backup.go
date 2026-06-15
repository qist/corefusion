package entity

import "time"

// BackupRecord is the GORM model for the backup_records table.
type BackupRecord struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string    `json:"name" gorm:"size:255;not null"`
	Type       string    `json:"type" gorm:"size:32;not null"` // manual, scheduled
	FilePath   string    `json:"file_path" gorm:"size:512;not null"`
	FileSize   int64     `json:"file_size"`
	Status     string    `json:"status" gorm:"size:32;not null;default:'completed'"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
}

func (BackupRecord) TableName() string { return "backup_records" }

// BackupSchedule is the GORM model for the backup_schedules table.
type BackupSchedule struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string    `json:"name" gorm:"size:128;not null"`
	Cron       string    `json:"cron" gorm:"size:64;not null"`
	RetainDays int       `json:"retain_days" gorm:"not null;default:30"`
	Enabled    bool      `json:"enabled" gorm:"not null;default:true"`
	LastRunAt  time.Time `json:"last_run_at"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"not null"`
}

func (BackupSchedule) TableName() string { return "backup_schedules" }

// BackupStorage is the GORM model for the backup_storages table.
type BackupStorage struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"size:128;not null"`
	Type      string    `json:"type" gorm:"size:32;not null"` // local, s3, webdav, ftp
	Config    string    `json:"config" gorm:"type:text"`      // JSON config
	Enabled   bool      `json:"enabled" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (BackupStorage) TableName() string { return "backup_storages" }
