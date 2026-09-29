package traffic

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// LogCleanupService periodically removes old log/event records from the database
// to prevent unbounded growth. Cleanup rules are configurable via the log_cleanup_rules table.
type LogCleanupService struct {
	db       *gorm.DB
	interval time.Duration // how often to run scheduled cleanup
}

// NewLogCleanupService creates a new log cleanup service.
func NewLogCleanupService(db *gorm.DB, cleanupInterval time.Duration) *LogCleanupService {
	if cleanupInterval <= 0 {
		cleanupInterval = 6 * time.Hour
	}
	return &LogCleanupService{
		db:       db,
		interval: cleanupInterval,
	}
}

// Start starts the cleanup background worker.
func (s *LogCleanupService) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *LogCleanupService) loop(ctx context.Context) {
	// Run once shortly after startup
	time.Sleep(30 * time.Second)
	s.RunScheduledCleanup(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunScheduledCleanup(ctx)
		}
	}
}

// RunScheduledCleanup executes cleanup for all enabled rules.
// Called by the background scheduler.
func (s *LogCleanupService) RunScheduledCleanup(ctx context.Context) {
	rules, err := s.ListRules()
	if err != nil {
		log.Printf("[LogCleanup] failed to load rules: %v", err)
		return
	}

	totalDeleted := int64(0)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		deleted := s.cleanupTable(rule)
		totalDeleted += deleted
	}

	if totalDeleted > 0 {
		log.Printf("[LogCleanup] total deleted %d records", totalDeleted)
	} else {
		log.Printf("[LogCleanup] nothing to clean")
	}
}

// CleanupResult records the result of a single table cleanup.
type CleanupResult struct {
	Table   string `json:"table"`
	Deleted int64  `json:"deleted"`
}

// CleanupNow manually triggers cleanup for all enabled rules.
// Returns per-table results.
func (s *LogCleanupService) CleanupNow(ctx context.Context) ([]CleanupResult, int64, error) {
	rules, err := s.ListRules()
	if err != nil {
		return nil, 0, err
	}

	var results []CleanupResult
	var totalDeleted int64

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		deleted := s.cleanupTable(rule)
		if deleted > 0 {
			results = append(results, CleanupResult{Table: rule.Table, Deleted: deleted})
			totalDeleted += deleted
		}
	}

	return results, totalDeleted, nil
}

// CleanupTableByName manually triggers cleanup for a specific table rule.
func (s *LogCleanupService) CleanupTableByName(ctx context.Context, tableName string) (int64, error) {
	var rule entity.LogCleanupRule
	if err := s.db.Where("table_name = ?", tableName).First(&rule).Error; err != nil {
		return 0, err
	}
	deleted := s.cleanupTable(rule)
	log.Printf("[LogCleanup] manual cleanup for %s: deleted %d records", tableName, deleted)
	return deleted, nil
}

// cleanupTable executes cleanup for a single rule and returns the number of deleted rows.
func (s *LogCleanupService) cleanupTable(rule entity.LogCleanupRule) int64 {
	cutoff := time.Now().AddDate(0, 0, -rule.RetainDays)
	table := rule.Table

	// Special handling for tables with non-standard time columns or conditions
	// ⚠ 表不存在就直接跳过 ✗ —— 有些清理规则是给**节点本地库**用的
	//（例如 config_snapshots 只存在于 Agent 的 DB ✓），面板里没有这张表 ✓，
	// 原来会一直刷 `no such table: config_snapshots` ✗（真机日志噪音 ✓）。
	if !s.db.Migrator().HasTable(table) {
		return 0
	}

	switch table {
	case "traffic_sessions":
		// Closed sessions older than retention + stale open sessions (>24h)
		var total int64
		r := s.db.Table(table).Where("status = ? AND ended_at < ?", "ended", cutoff).Delete(nil)
		total += r.RowsAffected
		r = s.db.Table(table).Where("status = ? AND started_at < ?", "active", time.Now().Add(-24*time.Hour)).Delete(nil)
		total += r.RowsAffected
		if total > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", total, table, rule.RetainDays)
		}
		return total

	case "runtime_apply_jobs", "update_jobs", "certificate_jobs", "backup_cleanup_jobs":
		// Only completed/failed/cancelled jobs
		timeCol := "finished_at"
		if table == "backup_cleanup_jobs" {
			timeCol = "finished_at"
		}
		r := s.db.Table(table).Where("status IN ? AND "+timeCol+" < ?", []string{"completed", "failed", "cancelled", "success"}, cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected

	case "runtime_apply_job_targets":
		// Delete targets whose parent job is completed and old
		r := s.db.Table(table).
			Where("job_id IN (SELECT id FROM runtime_apply_jobs WHERE status IN ? AND finished_at < ?)",
				[]string{"completed", "failed", "cancelled"}, cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected

	case "charge_callback_retries":
		// Only resolved/success/failed retries
		r := s.db.Table(table).
			Where("status IN ? AND created_at < ?", []string{"resolved", "success", "failed", "abandoned"}, cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected

	case "log_rotation_jobs":
		// Uses finished_at
		r := s.db.Table(table).Where("finished_at < ?", cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected

	case "client_log_packages":
		// Delete by expires_at (already expired) or created_at (old)
		r := s.db.Table(table).Where("expires_at < ? OR created_at < ?", time.Now(), cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected

	case "shared_account_repair_logs":
		// Delete completed repair logs older than retention
		r := s.db.Table(table).Where("status = ? AND completed_at < ?", "completed", cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected

	default:
		// Standard: delete by created_at (or occurred_at for traffic_usage_ledger)
		timeCol := "created_at"
		if table == "traffic_usage_ledger" {
			timeCol = "occurred_at"
		}
		r := s.db.Table(table).Where(timeCol+" < ?", cutoff).Delete(nil)
		if r.RowsAffected > 0 {
			log.Printf("[LogCleanup] deleted %d %s (retention=%d days)", r.RowsAffected, table, rule.RetainDays)
		}
		return r.RowsAffected
	}
}

// ── Rule CRUD ──

// ListRules returns all cleanup rules.
func (s *LogCleanupService) ListRules() ([]entity.LogCleanupRule, error) {
	var rules []entity.LogCleanupRule
	err := s.db.Order("category ASC, id ASC").Find(&rules).Error
	return rules, err
}

// GetRule returns a cleanup rule by ID.
func (s *LogCleanupService) GetRule(id int64) (*entity.LogCleanupRule, error) {
	var rule entity.LogCleanupRule
	err := s.db.First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateRule updates a cleanup rule.
func (s *LogCleanupService) UpdateRule(rule *entity.LogCleanupRule) error {
	return s.db.Save(rule).Error
}
