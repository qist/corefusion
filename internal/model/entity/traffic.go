package entity

import "time"

// TrafficQuota is the GORM model for the traffic_quotas table.
type TrafficQuota struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SubjectType string `json:"subject_type" gorm:"size:32;not null"`
	SubjectID int64 `json:"subject_id" gorm:"not null"`
	ResetCycle string `json:"reset_cycle" gorm:"size:32;not null;default:monthly"`
	QuotaBytes int64 `json:"quota_bytes" gorm:"not null"`
	HardLimit int `json:"hard_limit" gorm:"not null;default:1"`
	GraceBytes int64 `json:"grace_bytes" gorm:"not null;default:0"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (TrafficQuota) TableName() string { return "traffic_quotas" }

// TrafficUsageLedger is the GORM model for the traffic_usage_ledger table.
type TrafficUsageLedger struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SessionID string `json:"session_id" gorm:"size:128;not null;uniqueIndex:idx_ledger_session_seq"`
	Seq int64 `json:"seq" gorm:"not null;uniqueIndex:idx_ledger_session_seq"`
	UserID         int64
	SubscriptionID int64
	ServiceID      int64
	CoreType string `json:"core_type" gorm:"size:32;not null"`
	Protocol string `json:"protocol" gorm:"size:32;not null"`
	UplinkBytes int64 `json:"uplink_bytes" gorm:"not null;default:0"`
	DownlinkBytes int64 `json:"downlink_bytes" gorm:"not null;default:0"`
	BilledBytes int64 `json:"billed_bytes" gorm:"not null;default:0"`
	OccurredAt     time.Time `json:"created_at" gorm:"not null"`
}

func (TrafficUsageLedger) TableName() string { return "traffic_usage_ledger" }

// TrafficUsageSummary is the GORM model for the traffic_usage_summary table.
type TrafficUsageSummary struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SubjectType string `json:"subject_type" gorm:"size:32;not null;uniqueIndex:idx_summary_subject_period"`
	SubjectID int64 `json:"subject_id" gorm:"not null;uniqueIndex:idx_summary_subject_period"`
	PeriodStart time.Time `json:"period_start" gorm:"not null;uniqueIndex:idx_summary_subject_period"`
	PeriodEnd     time.Time `json:"created_at" gorm:"not null"`
	UplinkBytes int64 `json:"uplink_bytes" gorm:"not null;default:0"`
	DownlinkBytes int64 `json:"downlink_bytes" gorm:"not null;default:0"`
	BilledBytes int64 `json:"billed_bytes" gorm:"not null;default:0"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null"`
}

func (TrafficUsageSummary) TableName() string { return "traffic_usage_summary" }

// TrafficSession is the GORM model for the traffic_sessions table.
type TrafficSession struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SessionID string `json:"session_id" gorm:"size:128;uniqueIndex;not null"`
	UserID            int64
	SubscriptionID    int64
	ServiceID int64 `json:"service_id" gorm:"not null"`
	CoreType string `json:"core_type" gorm:"size:32;not null"`
	Protocol string `json:"protocol" gorm:"size:32;not null"`
	Status string `json:"status" gorm:"size:32;not null"`
	StartedAt         time.Time  `json:"created_at" gorm:"not null"`
	EndedAt *time.Time `json:"ended_at"`
	FinalUplinkBytes int64 `json:"final_uplink_bytes" gorm:"not null;default:0"`
	FinalDownlinkBytes int64 `json:"final_downlink_bytes" gorm:"not null;default:0"`
}

func (TrafficSession) TableName() string { return "traffic_sessions" }

// TrafficQuotaResetLog is the GORM model for the traffic_quota_reset_logs table.
type TrafficQuotaResetLog struct {
	ID int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	SubjectType string `json:"subject_type" gorm:"size:32;not null"`
	SubjectID int64 `json:"subject_id" gorm:"not null"`
	ResetCycle string `json:"reset_cycle" gorm:"size:32;not null"`
	PeriodStart            time.Time `json:"created_at" gorm:"not null"`
	PeriodEnd              time.Time `json:"created_at" gorm:"not null"`
	BilledBytesBeforeReset int64 `json:"billed_bytes_before_reset" gorm:"not null;default:0"`
	TriggerSource string `json:"trigger_source" gorm:"size:32;not null"`
	OperatorID             int64
	CreatedAt              time.Time `json:"created_at" gorm:"not null"`
}

func (TrafficQuotaResetLog) TableName() string { return "traffic_quota_reset_logs" }
