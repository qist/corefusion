package quota

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"corefusion/internal/domain"
	"corefusion/internal/domain/metric"
	"corefusion/internal/model/entity"
)

var (
	ErrQuotaSoftRejected = errors.New("quota soft rejected: over limit, new connections denied")
	ErrQuotaHardRejected = errors.New("quota hard rejected: over limit, all connections terminated")
	ErrQuotaSuspended    = errors.New("quota suspended by administrator")
	ErrQuotaExpiredDays  = errors.New("subscription max active days expired")
)

// DefaultStateMachine evaluates quota state from a snapshot.
type DefaultStateMachine struct {
	nearLimitThreshold float64
}

// NewDefaultStateMachine creates a new quota state machine.
func NewDefaultStateMachine() *DefaultStateMachine {
	return &DefaultStateMachine{
		nearLimitThreshold: 0.8, // 80%
	}
}

// Evaluate evaluates the quota state from a snapshot.
func (m *DefaultStateMachine) Evaluate(snapshot metric.QuotaSnapshot) domain.QuotaState {
	totalLimit := snapshot.QuotaBytes + snapshot.GraceBytes

	// Suspended state is sticky
	if snapshot.State == domain.QuotaStateSuspended {
		return domain.QuotaStateSuspended
	}

	// No quota limit
	if totalLimit <= 0 {
		return domain.QuotaStateNormal
	}

	used := snapshot.UsedBytes

	if used >= totalLimit {
		if snapshot.RejectMode == "hard" {
			return domain.QuotaStateOverLimitHard
		}
		return domain.QuotaStateOverLimitSoft
	}

	if float64(used) >= float64(totalLimit)*m.nearLimitThreshold {
		return domain.QuotaStateNearLimit
	}

	return domain.QuotaStateNormal
}

// AllowNewConnection checks if a new connection should be allowed.
func (m *DefaultStateMachine) AllowNewConnection(snapshot metric.QuotaSnapshot) error {
	state := m.Evaluate(snapshot)
	switch state {
	case domain.QuotaStateOverLimitSoft:
		return ErrQuotaSoftRejected
	case domain.QuotaStateOverLimitHard:
		return ErrQuotaHardRejected
	case domain.QuotaStateSuspended:
		return ErrQuotaSuspended
	case domain.QuotaStateExpiredDays:
		return ErrQuotaExpiredDays
	default:
		return nil
	}
}

// ShouldTerminateActiveSession checks if active sessions should be terminated.
func (m *DefaultStateMachine) ShouldTerminateActiveSession(snapshot metric.QuotaSnapshot) bool {
	state := m.Evaluate(snapshot)
	return state == domain.QuotaStateOverLimitHard || state == domain.QuotaStateSuspended
}

// Service manages quota checking and state.
type Service struct {
	quotaRepo   *QuotaRepository
	summaryRepo *SummaryRepository
	machine     *DefaultStateMachine
	cache       sync.Map // key: "type:id" -> *metric.QuotaSnapshot
}

// NewService creates a new quota service.
func NewService(db interface{}) *Service {
	return &Service{
		machine: NewDefaultStateMachine(),
	}
}

// CheckConnection checks if a new connection should be allowed for a subject.
func (s *Service) CheckConnection(ctx context.Context, subjectType string, subjectID int64) error {
	snapshot := s.getSnapshot(subjectType, subjectID)
	if snapshot == nil {
		// No quota configured, allow
		return nil
	}

	return s.machine.AllowNewConnection(*snapshot)
}

// RefreshSnapshot refreshes the quota snapshot in cache.
func (s *Service) RefreshSnapshot(ctx context.Context, subjectType string, subjectID int64) {
	// Load from database
	quota := s.quotaRepo.GetBySubject(ctx, subjectType, subjectID)
	summary := s.summaryRepo.GetCurrent(ctx, subjectType, subjectID)

	snapshot := metric.QuotaSnapshot{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		UpdatedAt:   time.Now(),
	}

	if summary != nil {
		snapshot.UsedBytes = summary.BilledBytes
	}

	if quota != nil {
		snapshot.QuotaBytes = quota.QuotaBytes
		snapshot.GraceBytes = quota.GraceBytes
		if quota.HardLimit == 1 {
			snapshot.RejectMode = "hard"
		} else {
			snapshot.RejectMode = "soft"
		}

		totalLimit := quota.QuotaBytes + quota.GraceBytes
		if totalLimit > 0 {
			snapshot.UsageRatio = float64(snapshot.UsedBytes) / float64(totalLimit)
		}
	}

	snapshot.State = s.machine.Evaluate(snapshot)

	key := subjectType + ":" + intToStr(subjectID)
	s.cache.Store(key, &snapshot)
}

func (s *Service) getSnapshot(subjectType string, subjectID int64) *metric.QuotaSnapshot {
	key := subjectType + ":" + intToStr(subjectID)
	if v, ok := s.cache.Load(key); ok {
		return v.(*metric.QuotaSnapshot)
	}
	return nil
}

func intToStr(n int64) string {
	return fmt.Sprintf("%d", n)
}

// QuotaRepository handles quota database operations.
type QuotaRepository struct {
	// Uses GORM DB
}

// GetBySubject retrieves quota config for a subject.
func (r *QuotaRepository) GetBySubject(ctx context.Context, subjectType string, subjectID int64) *entity.TrafficQuota {
	// TODO: implement with GORM
	return nil
}

// SummaryRepository handles traffic summary database operations.
type SummaryRepository struct {
	// Uses GORM DB
}

// GetCurrent retrieves current period summary for a subject.
func (r *SummaryRepository) GetCurrent(ctx context.Context, subjectType string, subjectID int64) *entity.TrafficUsageSummary {
	// TODO: implement with GORM
	return nil
}
