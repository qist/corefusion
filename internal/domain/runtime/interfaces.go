package runtime

import (
	"context"
	"time"
)

// ManagedProcessController manages core process lifecycle.
type ManagedProcessController interface {
	Start(ctx context.Context, target RuntimeTarget) error
	Stop(ctx context.Context, target RuntimeTarget) error
	Restart(ctx context.Context, target RuntimeTarget) error
	Reload(ctx context.Context, target RuntimeTarget) error
	HealthCheck(ctx context.Context, target RuntimeTarget) error
	Capability(ctx context.Context, target RuntimeTarget) (*ProcessCapability, error)
	GetStatus(ctx context.Context, target RuntimeTarget) (*ProcessStatusResult, error)
}

// RuntimeTarget identifies a runtime target.
type RuntimeTarget struct {
	TargetType string
	TargetID   int64
	ApplyMode  string
}

// ProcessCapability describes what a core process supports.
type ProcessCapability struct {
	SupportsReload      bool
	SupportsHotReload   bool
	RequiresFullRestart bool
}

// ProcessStatusResult is the current process status.
type ProcessStatusResult struct {
	ServiceID     int64
	State         string
	PID           int
	UptimeSeconds int64
	CrashCount    int
	LastCrashAt   *time.Time
	LastExitCode  int
}

// GuardianConfig configures the process guardian.
type GuardianConfig struct {
	MaxRestartAttempts  int
	RestartWindow       time.Duration
	StableThreshold     time.Duration
	LongStableThreshold time.Duration
	BaseBackoff         time.Duration
	MaxBackoff          time.Duration
	MaxStdoutLines      int
	MaxStderrLines      int
	CrashLogRetention   int
}

// DefaultGuardianConfig returns the default guardian config.
func DefaultGuardianConfig() GuardianConfig {
	return GuardianConfig{
		MaxRestartAttempts:  5,
		RestartWindow:       10 * time.Minute,
		StableThreshold:     5 * time.Minute,
		LongStableThreshold: 1 * time.Hour,
		BaseBackoff:         5 * time.Second,
		MaxBackoff:          80 * time.Second,
		MaxStdoutLines:      200,
		MaxStderrLines:      200,
		CrashLogRetention:   50,
	}
}

// RuntimeApplyService manages runtime apply jobs.
type RuntimeApplyService interface {
	DryRun(ctx context.Context, in SingleNodeApplyInput) (*RuntimeDryRunResult, error)
	CreateJob(ctx context.Context, in SingleNodeApplyInput) (*RuntimeApplyJob, error)
	ExecuteJob(ctx context.Context, jobID int64) error
	DisableProxy(ctx context.Context, targets []RuntimeTarget, reason string) (*RuntimeApplyJob, error)
	RetryJob(ctx context.Context, jobID int64, reason string) error
	CancelJob(ctx context.Context, jobID int64, reason string) error
}

// SingleNodeApplyInput is the input for a runtime apply operation.
type SingleNodeApplyInput struct {
	ServiceIDs      []int64
	ReverseProxyIDs []int64
	CertificateIDs  []int64
	TriggerSource   string
	Reason          string
}

// RuntimeDryRunResult is the result of a dry run.
type RuntimeDryRunResult struct {
	OK               bool
	SnapshotRequired bool
	AffectedTargets  []RuntimeTarget
	PortConflicts    []string
	MissingCerts     []int64
	ReloadPlan       []RuntimeTarget
}

// RuntimeApplyJob represents a runtime apply job.
type RuntimeApplyJob struct {
	ID                 int64
	TriggerSource      string
	Reason             string
	OperationKind      string
	State              string
	SnapshotID         *int64
	OriginalSnapshotID *int64
	TargetCount        int
	SuccessCount       int
	FailedCount        int
	ErrorMessage       string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
