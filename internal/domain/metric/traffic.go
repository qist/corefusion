package metric

import (
	"time"

	"corefusion/internal/domain"
)

// TrafficDeltaEvent represents a traffic increment event from a core collector.
type TrafficDeltaEvent struct {
	SessionID       string              `json:"session_id"`
	UserID          int64               `json:"user_id"`
	SubscriptionID  int64               `json:"subscription_id"`
	ServiceID       int64               `json:"service_id"`
	EndpointID      int64               `json:"endpoint_id"`
	CoreType        domain.CoreType     `json:"core_type"`
	Protocol        domain.ProtocolType `json:"protocol"`
	UplinkBytes     int64               `json:"uplink_bytes"`
	DownlinkBytes   int64               `json:"downlink_bytes"`
	BillingBytes    int64               `json:"billing_bytes"`
	Sequence        int64               `json:"seq"`
	SourceCollector string              `json:"source_collector"`
	CollectedAt     time.Time           `json:"collected_at"`
}

// RawCollectorEvent is the raw event from a core collector, format varies by core.
type RawCollectorEvent struct {
	CoreType       domain.CoreType
	UserIdentifier string
	SessionID      string
	UplinkBytes    int64
	DownlinkBytes  int64
	Metadata       map[string]any
}

// TrafficOwnership maps a raw event to a user/subscription/service.
type TrafficOwnership struct {
	UserID         int64               `json:"user_id"`
	SubscriptionID int64               `json:"subscription_id"`
	ServiceID      int64               `json:"service_id"`
	EndpointID     int64               `json:"endpoint_id"`
	Protocol       domain.ProtocolType `json:"protocol"`
}

// QuotaSnapshot represents the current quota state for a subject.
type QuotaSnapshot struct {
	SubjectType string            `json:"subject_type"`
	SubjectID   int64             `json:"subject_id"`
	UsedBytes   int64             `json:"used_bytes"`
	QuotaBytes  int64             `json:"quota_bytes"`
	GraceBytes  int64             `json:"grace_bytes"`
	UsageRatio  float64           `json:"usage_ratio"`
	RejectMode  string            `json:"reject_mode"`
	State       domain.QuotaState `json:"state"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AccessContext represents the context for a connection access check.
type AccessContext struct {
	ServiceID         int64               `json:"service_id"`
	EndpointID        int64               `json:"endpoint_id"`
	UserID            *int64              `json:"user_id,omitempty"`
	SubscriptionID    *int64              `json:"subscription_id,omitempty"`
	RemoteIP          string              `json:"remote_ip"`
	Protocol          domain.ProtocolType `json:"protocol"`
	CurrentConcurrent int                 `json:"current_concurrent"`
	AttemptAt         time.Time           `json:"attempt_at"`
}

// CoreCrashLog represents a core process crash record.
type CoreCrashLog struct {
	ID             int64     `json:"id"`
	ServiceID      int64     `json:"service_id"`
	CoreType       string    `json:"core_type"`
	ExitCode       int       `json:"exit_code"`
	Signal         string    `json:"signal"`
	KilledByOOM    bool      `json:"killed_by_oom"`
	RuntimeSeconds int64     `json:"runtime_seconds"`
	StdoutTail     string    `json:"stdout_tail"`
	StderrTail     string    `json:"stderr_tail"`
	RestartAttempt int       `json:"restart_attempt"`
	AutoRestarted  bool      `json:"auto_restarted"`
	CrashAt        time.Time `json:"crash_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// ProcessStatus represents the current status of a core process.
type ProcessStatus struct {
	ServiceID          int64      `json:"service_id"`
	State              string     `json:"state"`
	PID                int        `json:"pid"`
	UptimeSeconds      int64      `json:"uptime_seconds"`
	CrashCount         int        `json:"crash_count"`
	MaxRestartAttempts int        `json:"max_restart_attempts"`
	NextRestartAt      *time.Time `json:"next_restart_at,omitempty"`
	LastCrashAt        *time.Time `json:"last_crash_at,omitempty"`
	LastExitCode       int        `json:"last_exit_code"`
	AutoRestartEnabled bool       `json:"auto_restart_enabled"`
}
