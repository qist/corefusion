package entity

import "time"

// RuntimeSnapshot is the GORM model for runtime config snapshots.
type RuntimeSnapshot struct {
	ID              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	SnapshotType    string    `json:"snapshot_type" gorm:"size:32;not null"`
	TriggerSource   string    `json:"trigger_source" gorm:"size:32"`
	Reason          string    `json:"reason" gorm:"type:text"`
	ConfigJSON      string    `json:"config_json" gorm:"type:text"`
	FilePathsJSON   string    `json:"file_paths_json" gorm:"type:text"`
	ChecksumSHA256  string    `json:"checksum_sha256" gorm:"size:64"`
	CreatedAt       time.Time `json:"created_at" gorm:"not null"`
}

func (RuntimeSnapshot) TableName() string { return "runtime_snapshots" }

// RuntimeApplyJob is the GORM model for runtime apply jobs.
type RuntimeApplyJob struct {
	ID                int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	TriggerSource     string     `json:"trigger_source" gorm:"size:32;index;not null"`
	Reason            string     `json:"reason" gorm:"type:text"`
	OperationKind     string     `json:"operation_kind" gorm:"size:32;not null;default:apply"`
	Status            string     `json:"status" gorm:"size:32;index;not null"`
	Phase             string     `json:"phase" gorm:"size:64"`
	SnapshotID        *int64     `json:"snapshot_id"`
	OriginalSnapshotID *int64   `json:"original_snapshot_id"`
	TargetCount       int        `json:"target_count" gorm:"not null;default:0"`
	SuccessCount      int        `json:"success_count" gorm:"not null;default:0"`
	FailedCount       int        `json:"failed_count" gorm:"not null;default:0"`
	ErrorMessage      string     `json:"error_message" gorm:"type:text"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	CreatedAt         time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"not null"`
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
	ErrorMessage    string     `json:"error_message" gorm:"type:text"`
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
	Message    string    `json:"message" gorm:"type:text;not null"`
	DetailJSON string    `json:"detail_json" gorm:"type:text"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null"`
}

func (RuntimeApplyJobEvent) TableName() string { return "runtime_apply_job_events" }

// ReverseProxyInstance is the GORM model for reverse proxy instances.
type ReverseProxyInstance struct {
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name          string    `json:"name" gorm:"size:128;not null"`
	ProxyType     string    `json:"proxy_type" gorm:"size:32;not null"`
	ConfigPath    string    `json:"config_path" gorm:"size:512"`
	ListenAddr    string    `json:"listen_addr" gorm:"size:255"`
	Status        string    `json:"status" gorm:"size:32;not null;default:stopped"`
	LastReloadAt  *time.Time `json:"last_renewed_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"not null"`
}

func (ReverseProxyInstance) TableName() string { return "reverse_proxy_instances" }

// BackupRetentionPolicy is the GORM model for backup retention policies.
type BackupRetentionPolicy struct {
	ID                        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name                      string    `json:"name" gorm:"size:128;not null"`
	TargetScope               string    `json:"target_scope" gorm:"size:32;not null"`
	BackupKind                string    `json:"backup_kind" gorm:"size:32;not null"`
	RetainLatestCount         int       `json:"retain_latest_count" gorm:"not null;default:5"`
	RetainDays                int       `json:"retain_days" gorm:"not null;default:30"`
	ProtectOriginalBaseline   int       `json:"protect_original_baseline" gorm:"not null;default:1"`
	ProtectLatestSuccessCount int       `json:"protect_latest_success_count" gorm:"not null;default:3"`
	CleanupEnabled            int       `json:"cleanup_enabled" gorm:"not null;default:1"`
	CreatedAt                 time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt                 time.Time `json:"updated_at" gorm:"not null"`
}

func (BackupRetentionPolicy) TableName() string { return "backup_retention_policies" }

// BackupCleanupJob is the GORM model for backup cleanup jobs.
type BackupCleanupJob struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	PolicyID       *int64     `json:"policy_id"`
	JobScope       string     `json:"job_scope" gorm:"size:32;not null"`
	Status         string     `json:"status" gorm:"size:32;not null"`
	Phase          string     `json:"phase" gorm:"size:64"`
	ScannedCount   int        `json:"scanned_count" gorm:"not null;default:0"`
	ExpiredCount   int        `json:"expired_count" gorm:"not null;default:0"`
	DeletedCount   int        `json:"deleted_count" gorm:"not null;default:0"`
	ProtectedCount int        `json:"protected_count" gorm:"not null;default:0"`
	ErrorMessage   string     `json:"error_message" gorm:"type:text"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

func (BackupCleanupJob) TableName() string { return "backup_cleanup_jobs" }

// BinaryRelease is the GORM model for binary releases.
type BinaryRelease struct {
	ID              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ComponentName   string    `json:"component_name" gorm:"size:64;not null"`
	ComponentType   string    `json:"component_type" gorm:"size:32;not null"`
	Version         string    `json:"version" gorm:"size:64;not null"`
	OS              string    `json:"os" gorm:"size:16;not null"`
	Arch            string    `json:"arch" gorm:"size:16;not null"`
	DownloadURL     string    `json:"download_url" gorm:"size:512;not null"`
	Checksum        string    `json:"checksum" gorm:"size:128"`
	Signature       string    `json:"signature" gorm:"size:512"`
	ReleaseChannel  string    `json:"release_channel" gorm:"size:32;not null;default:stable"`
	MinUpgradeable  string    `json:"min_upgradeable" gorm:"size:64"`
	Notes           string    `json:"notes" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at" gorm:"not null"`
}

func (BinaryRelease) TableName() string { return "binary_releases" }

// UpdateJob is the GORM model for system/core update jobs.
type UpdateJob struct {
	ID                int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	TargetType        string     `json:"target_type" gorm:"size:32;not null"`
	TargetID          *int64     `json:"target_id"`
	ComponentName     string     `json:"component_name" gorm:"size:64;not null"`
	FromVersion       string     `json:"from_version" gorm:"size:64"`
	ToVersion         string     `json:"to_version" gorm:"size:64;not null"`
	ReleaseChannel    string     `json:"release_channel" gorm:"size:32;not null;default:stable"`
	Status            string     `json:"status" gorm:"size:32;not null;default:pending"`
	Phase             string     `json:"phase" gorm:"size:64"`
	ErrorMessage      string     `json:"error_message" gorm:"type:text"`
	RollbackAvailable int        `json:"rollback_available" gorm:"not null;default:0"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	CreatedAt         time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt         time.Time  `json:"updated_at" gorm:"not null"`
}

func (UpdateJob) TableName() string { return "update_jobs" }

// RoutingRegion is the GORM model for routing regions.
type RoutingRegion struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"size:64;not null;uniqueIndex"`
	Label       string    `json:"label" gorm:"size:128;not null"`
	Description string    `json:"description" gorm:"type:text"`
	Countries   string    `json:"countries" gorm:"type:text"`
	CIDRs       string    `json:"cidrs" gorm:"type:text"`
	Status      int       `json:"status" gorm:"not null;default:1"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"not null"`
}

func (RoutingRegion) TableName() string { return "routing_regions" }

// ACMEAccount is the GORM model for ACME accounts.
type ACMEAccount struct {
	ID              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Email           string    `json:"email" gorm:"size:255;not null"`
	CAProvider      string    `json:"ca_provider" gorm:"size:32;not null"`
	CAUrl           string    `json:"ca_url" gorm:"size:512;not null"`
	AccountKeyPEM   string    `json:"account_key_pem" gorm:"type:text;not null"`
	RegistrationURI string    `json:"registration_uri" gorm:"size:512"`
	EABKeyID        string    `json:"eab_key_id" gorm:"size:255"`
	EABMACKey       string    `json:"eab_mac_key" gorm:"size:255"`
	Status          string    `json:"status" gorm:"size:32;not null;default:active"`
	CreatedAt       time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"not null"`
}

func (ACMEAccount) TableName() string { return "acme_accounts" }

// CertificateJob is the GORM model for certificate issuance/renewal jobs.
type CertificateJob struct {
	ID            int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	CertificateID *int64     `json:"certificate_id"`
	JobType       string     `json:"job_type" gorm:"size:32;not null"`
	CAProvider    string     `json:"ca_provider" gorm:"size:32"`
	ChallengeType string     `json:"challenge_type" gorm:"size:32"`
	Domains       string     `json:"domains" gorm:"type:text;not null"`
	Status        string     `json:"status" gorm:"size:32;not null;default:pending"`
	Phase         string     `json:"phase" gorm:"size:64"`
	ErrorMessage  string     `json:"error_message" gorm:"type:text"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time  `json:"updated_at" gorm:"not null"`
}

func (CertificateJob) TableName() string { return "certificate_jobs" }
