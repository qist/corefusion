package traffic

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// LogCleanupService periodically removes old log/event records from the database
// to prevent unbounded growth.
type LogCleanupService struct {
	db             *gorm.DB
	logRetention   time.Duration // retention for general logs (90 days)
	ledgerRetention time.Duration // retention for traffic ledger (30 days)
	interval       time.Duration // how often to run cleanup
}

// NewLogCleanupService creates a new log cleanup service.
func NewLogCleanupService(db *gorm.DB) *LogCleanupService {
	return &LogCleanupService{
		db:              db,
		logRetention:    90 * 24 * time.Hour,
		ledgerRetention: 30 * 24 * time.Hour,
		interval:        6 * time.Hour,
	}
}

// Start starts the cleanup background worker.
func (s *LogCleanupService) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *LogCleanupService) loop(ctx context.Context) {
	// Run once shortly after startup
	time.Sleep(30 * time.Second)
	s.cleanup()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *LogCleanupService) cleanup() {
	totalDeleted := int64(0)

	// ── Traffic tables (30-day retention) ──

	trafficCutoff := time.Now().Add(-s.ledgerRetention)

	// 1. traffic_usage_ledger
	result := s.db.Where("occurred_at < ?", trafficCutoff).Delete(&entity.TrafficUsageLedger{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d traffic_usage_ledger records (older than %v)", result.RowsAffected, s.ledgerRetention)
		totalDeleted += result.RowsAffected
	}

	// 2. traffic_sessions — closed sessions older than retention
	result = s.db.Where("status = ? AND ended_at < ?", "ended", trafficCutoff).Delete(&entity.TrafficSession{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d closed traffic_sessions (older than %v)", result.RowsAffected, s.ledgerRetention)
		totalDeleted += result.RowsAffected
	}

	// 3. traffic_sessions — stale open sessions (no activity >24h)
	staleCutoff := time.Now().Add(-24 * time.Hour)
	result = s.db.Where("status = ? AND started_at < ?", "active", staleCutoff).Delete(&entity.TrafficSession{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d stale traffic_sessions (open >24h)", result.RowsAffected)
		totalDeleted += result.RowsAffected
	}

	// 4. traffic_quota_reset_logs
	result = s.db.Where("created_at < ?", trafficCutoff).Delete(&entity.TrafficQuotaResetLog{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d traffic_quota_reset_logs (older than %v)", result.RowsAffected, s.ledgerRetention)
		totalDeleted += result.RowsAffected
	}

	// ── General log/event tables (90-day retention) ──

	logCutoff := time.Now().Add(-s.logRetention)

	// 5. subscription_access_logs
	result = s.db.Where("created_at < ?", logCutoff).Delete(&entity.SubscriptionAccessLog{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d subscription_access_logs (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 6. security_audit_events
	result = s.db.Where("created_at < ?", logCutoff).Delete(&entity.SecurityAuditEvent{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d security_audit_events (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 7. security_alerts (resolved alerts older than retention)
	result = s.db.Where("status = ? AND resolved_at < ?", "resolved", logCutoff).Delete(&entity.SecurityAlert{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d resolved security_alerts (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 8. core_crash_logs
	result = s.db.Where("created_at < ?", logCutoff).Delete(&entity.CoreCrashLog{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d core_crash_logs (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 9. runtime_apply_job_events
	result = s.db.Where("created_at < ?", logCutoff).Delete(&entity.RuntimeApplyJobEvent{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d runtime_apply_job_events (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 10. runtime_snapshots
	result = s.db.Where("created_at < ?", logCutoff).Delete(&entity.RuntimeSnapshot{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d runtime_snapshots (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 11. runtime_apply_jobs (completed/failed jobs older than retention)
	result = s.db.Where("status IN ? AND finished_at < ?", []string{"completed", "failed", "cancelled"}, logCutoff).Delete(&entity.RuntimeApplyJob{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d old runtime_apply_jobs (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	// 12. update_jobs (completed/failed jobs older than retention)
	result = s.db.Where("status IN ? AND finished_at < ?", []string{"completed", "failed", "cancelled"}, logCutoff).Delete(&entity.UpdateJob{})
	if result.RowsAffected > 0 {
		log.Printf("[LogCleanup] deleted %d old update_jobs (older than %v)", result.RowsAffected, s.logRetention)
		totalDeleted += result.RowsAffected
	}

	if totalDeleted > 0 {
		log.Printf("[LogCleanup] total deleted %d records (traffic retention=%v, log retention=%v)",
			totalDeleted, s.ledgerRetention, s.logRetention)
	} else {
		log.Printf("[LogCleanup] nothing to clean (traffic retention=%v, log retention=%v)",
			s.ledgerRetention, s.logRetention)
	}
}
