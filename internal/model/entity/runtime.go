package entity

import "time"

// RuntimeSnapshot is the GORM model for runtime config snapshots.
type RuntimeSnapshot struct {
	ID                  int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	SnapshotType        string    `json:"snapshot_type" gorm:"size:32;not null"`
	TriggerSource       string    `json:"trigger_source" gorm:"size:32"`
	Reason              string    `json:"reason" gorm:"type:longtext"`
	TargetServerIDsJSON string    `json:"target_server_ids_json" gorm:"type:longtext"` // intended target nodes (empty = all)
	ConfigJSON          string    `json:"config_json" gorm:"type:longtext"`
	FilePathsJSON       string    `json:"file_paths_json" gorm:"type:longtext"`
	ChecksumSHA256      string    `json:"checksum_sha256" gorm:"size:64"`
	CreatedAt           time.Time `json:"created_at" gorm:"not null"`
}

func (RuntimeSnapshot) TableName() string { return "runtime_snapshots" }

// RuntimeApplyJob is the GORM model for runtime apply jobs.
type RuntimeApplyJob struct {
	ID                  int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	TriggerSource       string     `json:"trigger_source" gorm:"size:32;index;not null"`
	Reason              string     `json:"reason" gorm:"type:longtext"`
	OperationKind       string     `json:"operation_kind" gorm:"size:32;not null;default:apply"`
	Status              string     `json:"status" gorm:"size:32;index;not null"`
	Phase               string     `json:"phase" gorm:"size:64"`
	SnapshotID          *int64     `json:"snapshot_id"`
	OriginalSnapshotID  *int64     `json:"original_snapshot_id"`
	TargetCount         int        `json:"target_count" gorm:"not null;default:0"`
	SuccessCount        int        `json:"success_count" gorm:"not null;default:0"`
	FailedCount         int        `json:"failed_count" gorm:"not null;default:0"`
	VerifyStatus        string     `json:"verify_status" gorm:"size:32;not null;default:''"` // pending|verifying|ok|partial|failed|skipped
	TargetServerIDsJSON string     `json:"target_server_ids_json" gorm:"type:longtext"`      // selected target nodes (empty = all)
	ErrorMessage        string     `json:"error_message" gorm:"type:longtext"`
	StartedAt           *time.Time `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at"`
	CreatedAt           time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt           time.Time  `json:"updated_at" gorm:"not null"`
}

func (RuntimeApplyJob) TableName() string { return "runtime_apply_jobs" }

// RuntimeApplyJobTarget is the GORM model for runtime apply job targets.
type RuntimeApplyJobTarget struct {
	ID              int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	JobID           int64      `json:"job_id" gorm:"index;not null"`
	TargetType      string     `json:"target_type" gorm:"size:32;not null"`
	TargetID        int64      `json:"target_id" gorm:"not null"`
	ApplyMode       string     `json:"apply_mode" gorm:"size:32;not null"`
	Status          string     `json:"status" gorm:"size:32;not null"`
	Phase           string     `json:"phase" gorm:"size:64"`
	VersionBefore   string     `json:"version_before" gorm:"size:64"`
	VersionAfter    string     `json:"version_after" gorm:"size:64"`
	RollbackVersion string     `json:"rollback_version" gorm:"size:64"`
	ErrorMessage    string     `json:"error_message" gorm:"type:longtext"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"not null"`
}

func (RuntimeApplyJobTarget) TableName() string { return "runtime_apply_job_targets" }

// RuntimeApplyJobEvent is the GORM model for runtime apply job events.
type RuntimeApplyJobEvent struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	JobID      int64     `json:"job_id" gorm:"index;not null"`
	TargetID   *int64    `json:"target_id"`
	EventLevel string    `json:"event_level" gorm:"size:16;not null"`
	EventType  string    `json:"event_type" gorm:"size:64;not null"`
	Phase      string    `json:"phase" gorm:"size:64"`
	Message    string    `json:"message" gorm:"type:longtext;not null"`
	DetailJSON string    `json:"detail_json" gorm:"type:longtext"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
}

func (RuntimeApplyJobEvent) TableName() string { return "runtime_apply_job_events" }
