package entity

import "time"

// CoreCrashLog is the GORM model for the core_crash_logs table.
type CoreCrashLog struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServiceID      int64     `json:"service_id" gorm:"index;not null"`
	CoreType       string    `json:"core_type" gorm:"size:32;not null"`
	ExitCode       int       `json:"exit_code"`
	Signal         string    `json:"signal" gorm:"size:32"`
	RuntimeSeconds int64     `json:"runtime_seconds" gorm:"not null;default:0"`
	StdoutTail     string    `json:"stdout_tail" gorm:"type:text"`
	StderrTail     string    `json:"stderr_tail" gorm:"type:text"`
	RestartAttempt int       `json:"restart_attempt" gorm:"not null;default:0"`
	AutoRestarted  bool      `json:"auto_restarted" gorm:"not null;default:false"`
	CrashAt        time.Time `json:"crash_at" gorm:"not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
}

func (CoreCrashLog) TableName() string { return "core_crash_logs" }
