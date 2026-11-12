package traffic

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/service/cronutil"
)

// cleanupScanInterval 是调度扫描粒度：每轮读一次规则表，按各规则自己的
// cleanup_cron 判断是否到点。1 分钟足够（最小可供配置的周期就是几小时）。
//
// 以前是一个全局 ticker（scheduler.log_cleanup_interval，默认 6h）把所有规则一起跑，
// 每条规则上配的 cleanup_cron 根本没被使用 —— 界面能改、实际不生效。
const cleanupScanInterval = time.Minute

// LogCleanupService periodically removes old log/event records from the database
// to prevent unbounded growth. Cleanup rules are configurable via the log_cleanup_rules table.
//
// 调度语义：
//   - 每条规则按自己的 cleanup_cron 独立排期（支持 0 */6 * * * / 每天 / 每周 / @every 6h）；
//   - 规则改完 1 分钟内自动重新排期，无需重启面板；
//   - cron 非法/为空时回落到 fallbackInterval（scheduler.log_cleanup_interval）；
//   - 启动时仍会对所有启用规则补跑一次（与旧行为一致，避免长期停机后积压）。
type LogCleanupService struct {
	db               *gorm.DB
	fallbackInterval time.Duration // cron 非法/为空时的兜底周期

	mu       sync.Mutex
	nextRun  map[int64]time.Time // 规则 ID → 下一次执行时间
	cronSeen map[int64]string    // 规则 ID → 上次排期时的 cron（用于识别修改）
}

// NewLogCleanupService creates a new log cleanup service.
func NewLogCleanupService(db *gorm.DB, fallbackInterval time.Duration) *LogCleanupService {
	if fallbackInterval <= 0 {
		fallbackInterval = 6 * time.Hour
	}
	return &LogCleanupService{
		db:               db,
		fallbackInterval: fallbackInterval,
		nextRun:          map[int64]time.Time{},
		cronSeen:         map[int64]string{},
	}
}

// Start starts the cleanup background worker.
func (s *LogCleanupService) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *LogCleanupService) loop(ctx context.Context) {
	// Run once shortly after startup（先补跑一轮，再按 cron 排期）
	time.Sleep(30 * time.Second)
	s.RunScheduledCleanup(ctx)
	s.reschedule(time.Now())

	ticker := time.NewTicker(cleanupScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.runDue(now)
		}
	}
}

// reschedule 按当前规则重新计算所有下次执行时间（不执行清理）。
func (s *LogCleanupService) reschedule(now time.Time) {
	rules, err := s.ListRules()
	if err != nil {
		log.Printf("[LogCleanup] failed to load rules: %v", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	active := make(map[int64]bool, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		active[rule.ID] = true
		cron := strings.TrimSpace(rule.CleanupCron)
		s.cronSeen[rule.ID] = cron
		s.nextRun[rule.ID] = s.nextFor(cron, now)
	}
	for id := range s.nextRun {
		if !active[id] {
			delete(s.nextRun, id)
			delete(s.cronSeen, id)
		}
	}
}

// nextFor 计算下一次执行时间：cron 合法按 cron，否则按兜底周期。
func (s *LogCleanupService) nextFor(cron string, now time.Time) time.Time {
	if next, ok := cronutil.NextRotation(cron, now); ok {
		return next
	}
	return now.Add(s.fallbackInterval)
}

// runDue 执行所有「已到点」的规则，并为每条规则重新排期。
//
// 每轮都会重新读规则表 → 改 cron / 启停 / 保留天数都无需重启面板，
// 最多 1 分钟内生效。
func (s *LogCleanupService) runDue(now time.Time) {
	rules, err := s.ListRules()
	if err != nil {
		log.Printf("[LogCleanup] failed to load rules: %v", err)
		return
	}

	due := make([]entity.LogCleanupRule, 0, 4)

	s.mu.Lock()
	active := make(map[int64]bool, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		active[rule.ID] = true
		cron := strings.TrimSpace(rule.CleanupCron)

		// cron 变了（或第一次见到这条规则）→ 重新排期，本轮不执行。
		if seen, ok := s.cronSeen[rule.ID]; !ok || seen != cron {
			s.cronSeen[rule.ID] = cron
			s.nextRun[rule.ID] = s.nextFor(cron, now)
			continue
		}

		next, ok := s.nextRun[rule.ID]
		if !ok {
			s.nextRun[rule.ID] = s.nextFor(cron, now)
			continue
		}
		if now.Before(next) {
			continue
		}

		due = append(due, rule)
		s.nextRun[rule.ID] = s.nextFor(cron, now)
	}

	for id := range s.nextRun {
		if !active[id] {
			delete(s.nextRun, id)
			delete(s.cronSeen, id)
		}
	}
	s.mu.Unlock()

	// 实际清理放到锁外：删大量历史行可能耗时，不能卡住 /cleanup-rules 的查询。
	for _, rule := range due {
		s.cleanupTable(rule)
	}
}

// NextRunAt 返回某条规则的下次执行时间（供管理端展示；未启用/未排期返回 false）。
func (s *LogCleanupService) NextRunAt(id int64) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, ok := s.nextRun[id]
	return next, ok
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

	case "runtime_apply_jobs":
		// Only completed/failed/cancelled jobs
		r := s.db.Table(table).Where("status IN ? AND finished_at < ?", []string{"completed", "failed", "cancelled", "success"}, cutoff).Delete(nil)
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
