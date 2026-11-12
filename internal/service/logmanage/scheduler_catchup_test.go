package logmanage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// newSchedulerTestDB 建一个独立的测试库（临时文件，而非 :memory:）。
//
// 不能用 sqlite.Open(":memory:")：SQLite 的 :memory: 是「每个连接一个库」，
// 而 GORM 有连接池，catchUp 里的异步轮转 goroutine 可能拿到一条新连接，
// 新连接看到的是空库 → "no such table: log_rotation_policies"
// （CI 上偶发，本地因为连接被复用而通常能过）。
// 单连接是为了让主 goroutine 与补跑 goroutine 串行访问，避免 database is locked。
func newSchedulerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "logmanage.db")), &gorm.Config{})
	if err != nil {
		t.Skipf("SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.LogRotationPolicy{}, &entity.LogRotationJob{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接池失败：%v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// 生产回归：面板每天重启多次，启动时如果「上次执行之后已经错过了计划时间点」，
// 必须立即补跑一次，否则重启永远卡在计划点之前 → 日志一直不轮转。
func TestSchedulerCatchUpRunsMissedRotation(t *testing.T) {
	db := newSchedulerTestDB(t)
	policyRepo := repository.NewLogRotationPolicyRepository(db)
	jobRepo := repository.NewLogRotationJobRepository(db)

	logPath := filepath.Join(t.TempDir(), "access.log")
	if err := os.WriteFile(logPath, []byte(make([]byte, 4096)), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	p := &entity.LogRotationPolicy{
		ServerID: 1, Name: "access", SourceType: "nginx", LogPath: logPath,
		Enabled: true, RotateCron: "0 0 * * *", RetainDays: 10, MaxSize: 1024,
		CreatedAt: now.AddDate(0, 0, -5), UpdatedAt: now.AddDate(0, 0, -5),
	}
	if err := policyRepo.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	// 最近一次执行在 2 天前 → 中间的计划点（昨天 00:00）被重启吞掉
	if err := jobRepo.Create(context.Background(), &entity.LogRotationJob{
		PolicyID: p.ID, Status: "success",
		StartedAt: now.AddDate(0, 0, -2), FinishedAt: now.AddDate(0, 0, -2),
	}); err != nil {
		t.Fatal(err)
	}

	svc := &Service{policyRepo: policyRepo, jobRepo: jobRepo}
	s := NewScheduler(policyRepo, jobRepo, svc)
	s.catchUp(p)

	// 补跑是异步的，等它落一条新任务记录
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		latest, err := jobRepo.GetLatestByPolicy(context.Background(), p.ID)
		if err == nil && latest.FinishedAt.After(now.Add(-time.Minute)) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("错过计划点的轮转没有在启动时补跑")
}
