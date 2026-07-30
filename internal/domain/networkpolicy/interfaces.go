package networkpolicy

import (
	"context"
	"errors"
	"time"
)

// ErrAccessDeniedByPolicy is returned when access is denied by a policy rule.
var ErrAccessDeniedByPolicy = errors.New("access denied by policy")

// ErrAccessDeniedByBan is returned when access is denied due to an active ban.
var ErrAccessDeniedByBan = errors.New("access denied by ban")

// ErrAccessDeniedByRate is returned when access is denied due to rate/concurrency limits.
var ErrAccessDeniedByRate = errors.New("access denied by rate limit")

// AccessGate checks if a connection should be allowed based on access policies.
type AccessGate interface {
	Check(ctx context.Context, ac AccessContext) error
	OnAccepted(ctx context.Context, ac AccessContext) error
	OnClosed(ctx context.Context, ac AccessContext) error
}

// AccessContext is the context for an access check.
type AccessContext struct {
	ServiceID         int64
	EndpointID        int64
	UserID            *int64
	SubscriptionID    *int64
	RemoteIP          string
	Protocol          string
	CurrentConcurrent int
	AttemptAt         time.Time
}

// AccessStateMachine evaluates access control state.
type AccessStateMachine interface {
	Evaluate(ctx context.Context, ac AccessContext, snapshot AccessRuntimeSnapshot) AccessControlDecision
}

// AccessRuntimeSnapshot is the current access state for an IP.
type AccessRuntimeSnapshot struct {
	CurrentState          string
	NewConnectionsPerMin  int
	ConcurrentConnections int
	ViolationCount        int
	RiskScore             int
	BanExpiresAt          *time.Time
}

// AccessControlDecision is the result of an access evaluation.
type AccessControlDecision struct {
	NextState   string
	Allowed     bool
	Reason      string
	AutoBan     bool
	BanDuration time.Duration
}
