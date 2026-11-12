package traffic

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
)

// 清理规则上的 cleanup_cron 必须真的驱动调度：以前是一个全局 ticker 把所有规则
// 一起跑，界面改「清理周期」只是存库、不生效。
func TestLogCleanupPerRuleCronDrivesSchedule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cleanup.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.LogCleanupRule{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	rule := &entity.LogCleanupRule{
		Category: "audit", Table: "demo_logs", DisplayName: "Demo Logs",
		RetainDays: 7, Enabled: true, CleanupCron: "0 */6 * * *",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(rule).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewLogCleanupService(db, 6*time.Hour)
	now := time.Date(2026, 11, 12, 10, 30, 0, 0, time.Local)
	svc.reschedule(now)

	next, ok := svc.NextRunAt(rule.ID)
	if !ok {
		t.Fatal("启用规则应有下次执行时间")
	}
	// 0 */6 * * * → 12:00
	if want := time.Date(2026, 11, 12, 12, 0, 0, 0, time.Local); !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}

	// 未到点：不应重排
	svc.runDue(now.Add(time.Minute))
	if got, _ := svc.NextRunAt(rule.ID); !got.Equal(next) {
		t.Fatalf("未到点不应重排：%s -> %s", next, got)
	}

	// 到点：执行后推进到下一个 6 小时点
	svc.runDue(next)
	after, _ := svc.NextRunAt(rule.ID)
	if want := time.Date(2026, 11, 12, 18, 0, 0, 0, time.Local); !after.Equal(want) {
		t.Fatalf("执行后 next = %s, want %s", after, want)
	}

	// 修改 cron → 下一轮立即按新表达式重排（最多 1 分钟内生效）
	rule.CleanupCron = "0 0 * * *"
	if err := svc.UpdateRule(rule); err != nil {
		t.Fatal(err)
	}
	svc.runDue(now.Add(2 * time.Minute))
	rescheduled, _ := svc.NextRunAt(rule.ID)
	if want := time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local); !rescheduled.Equal(want) {
		t.Fatalf("改 cron 后 next = %s, want %s", rescheduled, want)
	}

	// 禁用后不再排期
	rule.Enabled = false
	if err := svc.UpdateRule(rule); err != nil {
		t.Fatal(err)
	}
	svc.runDue(now.Add(3 * time.Minute))
	if _, ok := svc.NextRunAt(rule.ID); ok {
		t.Fatal("禁用规则不应有下次执行时间")
	}
}

// 到点执行时真的会删掉超过保留天数的行（调度 → cleanupTable 的完整链路）。
func TestLogCleanupRunDueDeletesOldRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cleanup2.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.LogCleanupRule{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Exec("CREATE TABLE demo_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, created_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, at := range []time.Time{now.AddDate(0, 0, -30), now} {
		if err := db.Exec("INSERT INTO demo_logs (created_at) VALUES (?)", at).Error; err != nil {
			t.Fatal(err)
		}
	}
	rule := &entity.LogCleanupRule{
		Category: "audit", Table: "demo_logs", DisplayName: "Demo Logs",
		RetainDays: 7, Enabled: true, CleanupCron: "@every 1h",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(rule).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewLogCleanupService(db, 6*time.Hour)
	svc.reschedule(now)
	svc.runDue(now.Add(2 * time.Hour)) // 到点

	var left int64
	if err := db.Table("demo_logs").Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("应只保留 1 条新记录，实际 %d", left)
	}
}

// cron 非法时回落到兜底间隔，而不是永不执行。
func TestLogCleanupInvalidCronFallsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cleanup3.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.LogCleanupRule{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	rule := &entity.LogCleanupRule{
		Category: "audit", Table: "demo_logs", DisplayName: "Demo",
		RetainDays: 7, Enabled: true, CleanupCron: "not a cron",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(rule).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewLogCleanupService(db, 90*time.Minute)
	now := time.Now()
	svc.reschedule(now)
	next, ok := svc.NextRunAt(rule.ID)
	if !ok {
		t.Fatal("非法 cron 也应排期（兜底）")
	}
	if d := next.Sub(now); d < 89*time.Minute || d > 91*time.Minute {
		t.Fatalf("兜底间隔 = %s, want ~90m", d)
	}
}
