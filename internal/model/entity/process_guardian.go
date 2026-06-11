package entity

import "time"

// ProcessGuardian is the GORM model for the process_guardians table.
type ProcessGuardian struct {
	ID                 int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServiceID          int64     `json:"service_id" gorm:"uniqueIndex;not null"`
	AutoStart          bool      `json:"auto_start" gorm:"not null;default:false"`
	AutoRestart        bool      `json:"auto_restart" gorm:"not null;default:false"`
	MaxRestartCount    int       `json:"max_restart_count" gorm:"not null;default:5"`
	RestartWindowSec   int       `json:"restart_window_sec" gorm:"not null;default:600"`
	StableThresholdSec int       `json:"stable_threshold_sec" gorm:"not null;default:300"`
	CurrentRestarts    int       `json:"current_restarts" gorm:"not null;default:0"`
	WindowStartAt      time.Time `json:"window_start_at"`
	LastRestartAt      time.Time `json:"last_restart_at"`
	LastCrashAt        time.Time `json:"last_crash_at"`
	LastExitCode       int       `json:"last_exit_code"`
	ManuallyStopped    bool      `json:"manually_stopped" gorm:"not null;default:false"`
	PID                int       `json:"pid"`
	ProcessStatus      string    `json:"process_status" gorm:"size:32;not null;default:'stopped'"`
	StartedAt          time.Time `json:"started_at"`
	CreatedAt          time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt          time.Time `json:"updated_at" gorm:"not null"`
}

func (ProcessGuardian) TableName() string { return "process_guardians" }
