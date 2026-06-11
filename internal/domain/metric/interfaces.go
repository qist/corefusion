package metric

import (
	"context"
	"time"

	"corefusion/internal/domain"
)

// TrafficCollectorAdapter is the interface for core-specific traffic collectors.
type TrafficCollectorAdapter interface {
	CoreType() domain.CoreType
	Start(ctx context.Context, in CollectorStartInput) error
	Stop(ctx context.Context) error
	Snapshot(ctx context.Context) (*CollectorSnapshot, error)
	Subscribe(ctx context.Context, ch chan<- TrafficDeltaEvent) error
}

// CollectorStartInput is the input for starting a collector.
type CollectorStartInput struct {
	ServiceID   int64
	HostID      int64
	CoreType    domain.CoreType
	RuntimeMeta map[string]string
}

// CollectorSnapshot is a point-in-time traffic snapshot.
type CollectorSnapshot struct {
	ServiceID      int64
	ActiveSessions int64
	UplinkBytes    int64
	DownlinkBytes  int64
	CollectedAt    time.Time
}

// TrafficCollectorRegistry manages traffic collector adapters.
type TrafficCollectorRegistry interface {
	Register(adapter TrafficCollectorAdapter) error
	Get(coreType domain.CoreType) (TrafficCollectorAdapter, error)
}

// TrafficAggregator aggregates traffic delta events into the database.
type TrafficAggregator interface {
	ApplyDelta(ctx context.Context, event TrafficDeltaEvent) error
	Flush(ctx context.Context) error
}

// SessionOwnershipResolver resolves raw collector events to user/subscription ownership.
type SessionOwnershipResolver interface {
	Resolve(ctx context.Context, raw RawCollectorEvent) (*TrafficOwnership, error)
}

// QuotaStateMachine evaluates quota state from a snapshot.
type QuotaStateMachine interface {
	Evaluate(snapshot QuotaSnapshot) domain.QuotaState
	AllowNewConnection(snapshot QuotaSnapshot) error
	ShouldTerminateActiveSession(snapshot QuotaSnapshot) bool
}

// QuotaService manages quota checking and state.
type QuotaService interface {
	Evaluate(ctx context.Context, subjectType string, subjectID int64) (*QuotaSnapshot, error)
	CheckConnection(ctx context.Context, subjectType string, subjectID int64) error
	ResetCycle(ctx context.Context, subjectType string, subjectID int64) error
	RefreshSnapshot(ctx context.Context, subjectType string, subjectID int64)
}

// ConnectionGate checks if a new connection should be allowed.
type ConnectionGate interface {
	Check(ctx context.Context, subjectType string, subjectID int64) error
}
