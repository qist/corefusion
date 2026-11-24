package agent

import (
	"context"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// fakeFailoverNodeMgr 让状态机可在纯内存里跑：id → 是否在线 / 核心状态。
type fakeFailoverNodeMgr struct {
	online map[int64]bool
	cores  map[int64]string
}

func (f *fakeFailoverNodeMgr) IsOnline(id int64) bool { return f.online[id] }

func (f *fakeFailoverNodeMgr) GetServerStatus(_ context.Context, id int64) (*agentpb.GetStatusResponse, error) {
	st, ok := f.cores[id]
	if !ok {
		st = "running"
	}
	return &agentpb.GetStatusResponse{Cores: []*agentpb.CoreStatus{{CoreType: "xray", State: st}}}, nil
}

func (f *fakeFailoverNodeMgr) SyncDataToServer(context.Context, int64, bool) (*agentpb.SyncDataResponse, error) {
	return &agentpb.SyncDataResponse{Success: true}, nil
}

func newFailoverTestPair(t *testing.T, mode string) (*FailoverManager, *repository.NodeServerRepository, *fakeFailoverNodeMgr, int64, int64) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.NodeServer{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	repo := repository.NewNodeServerRepository(db)
	ctx := context.Background()

	primary := &entity.NodeServer{
		Name: "p-primary", Hostname: "p-primary", NodeType: entity.NodeTypePrimaryBackup,
		FailoverRole: entity.FailoverRolePrimary, RecoverMode: mode, Enabled: true, BillingEnabled: true,
	}
	if err := repo.Create(ctx, primary); err != nil {
		t.Fatalf("创建主节点失败：%v", err)
	}
	backup := &entity.NodeServer{
		Name: "b-backup", Hostname: "b-backup", NodeType: entity.NodeTypePrimaryBackup,
		FailoverRole: entity.FailoverRoleBackup, RecoverMode: mode, Enabled: true, BillingEnabled: false,
	}
	if err := repo.Create(ctx, backup); err != nil {
		t.Fatalf("创建备节点失败：%v", err)
	}
	primary.PartnerServerID = backup.ID
	backup.PartnerServerID = primary.ID
	if err := repo.Update(ctx, primary); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, backup); err != nil {
		t.Fatal(err)
	}

	mgr := &fakeFailoverNodeMgr{
		online: map[int64]bool{primary.ID: true, backup.ID: true},
		cores:  map[int64]string{primary.ID: "running", backup.ID: "running"},
	}
	fm := NewFailoverManager(repo, mgr).WithRecoverGrace(40 * time.Millisecond)
	return fm, repo, mgr, primary.ID, backup.ID
}

// 手动切换：角色互换 + billing 翻转，且走与自动切换相同的校验。
func TestForceFailoverSwapsRolesAndBilling(t *testing.T) {
	fm, repo, _, primaryID, backupID := newFailoverTestPair(t, string(RecoverModeAutoSwitchBack))
	ctx := context.Background()

	if err := fm.ForceFailover(ctx, primaryID); err != nil {
		t.Fatalf("手动切换失败：%v", err)
	}

	p, _ := repo.GetByID(ctx, primaryID)
	b, _ := repo.GetByID(ctx, backupID)
	if p.FailoverRole != entity.FailoverRoleBackupRecovering {
		t.Errorf("原主角色 = %q, want backup_recovering", p.FailoverRole)
	}
	if p.BillingEnabled {
		t.Error("原主 billing_enabled 应为 false")
	}
	if b.FailoverRole != entity.FailoverRolePrimary {
		t.Errorf("备节点角色 = %q, want primary", b.FailoverRole)
	}
	if !b.BillingEnabled {
		t.Error("新主 billing_enabled 应为 true")
	}
}

// 备节点离线时手动切换必须失败（不能切到一个起不来的节点）。
func TestForceFailoverRejectsOfflinePartner(t *testing.T) {
	fm, repo, mgr, primaryID, backupID := newFailoverTestPair(t, string(RecoverModeAutoSwitchBack))
	ctx := context.Background()
	mgr.online[backupID] = false

	if err := fm.ForceFailover(ctx, primaryID); err == nil {
		t.Fatal("伙伴离线时手动切换应报错")
	}
	p, _ := repo.GetByID(ctx, primaryID)
	if p.FailoverRole != entity.FailoverRolePrimary {
		t.Errorf("切换失败后原主角色不应变化，实际 %q", p.FailoverRole)
	}
}

// 回切必须「连续健康 recoverGrace」：首次只记时，满时长后才切回。
func TestRecoverRequiresContinuousHealthyWindow(t *testing.T) {
	fm, repo, _, primaryID, backupID := newFailoverTestPair(t, string(RecoverModeAutoSwitchBack))
	ctx := context.Background()

	// 先做一次切换，让原主进入 backup_recovering。
	if err := fm.ForceFailover(ctx, primaryID); err != nil {
		t.Fatalf("准备切换失败：%v", err)
	}
	recovering, _ := repo.GetByID(ctx, primaryID)
	if recovering.FailoverRole != entity.FailoverRoleBackupRecovering {
		t.Fatalf("原主应处于 backup_recovering，实际 %q", recovering.FailoverRole)
	}

	// 第一次检查：只记录起点，不切。
	fm.checkRecovering(ctx, recovering)
	if p, _ := repo.GetByID(ctx, primaryID); p.FailoverRole != entity.FailoverRoleBackupRecovering {
		t.Fatal("刚恢复就回切了，稳定期未生效")
	}

	// 等过稳定期后再检查：应回切。
	time.Sleep(60 * time.Millisecond)
	fm.checkRecovering(ctx, recovering)
	p, _ := repo.GetByID(ctx, primaryID)
	b, _ := repo.GetByID(ctx, backupID)
	if p.FailoverRole != entity.FailoverRolePrimary || !p.BillingEnabled {
		t.Errorf("稳定期后应回切到原主：role=%q billing=%v", p.FailoverRole, p.BillingEnabled)
	}
	if b.FailoverRole != entity.FailoverRoleBackup || b.BillingEnabled {
		t.Errorf("鸭提升节点应降回 backup：role=%q billing=%v", b.FailoverRole, b.BillingEnabled)
	}
}

// 健康状态中断（核心全 down）要清零计时，恢复后必须重新计满稳定期。
func TestRecoverTimerResetsWhenUnhealthy(t *testing.T) {
	fm, repo, mgr, primaryID, _ := newFailoverTestPair(t, string(RecoverModeAutoSwitchBack))
	ctx := context.Background()
	if err := fm.ForceFailover(ctx, primaryID); err != nil {
		t.Fatalf("准备切换失败：%v", err)
	}
	recovering, _ := repo.GetByID(ctx, primaryID)

	fm.checkRecovering(ctx, recovering) // 记录起点
	mgr.cores[primaryID] = "crashed"    // 中途不健康
	fm.checkRecovering(ctx, recovering) // 清零
	time.Sleep(60 * time.Millisecond)
	mgr.cores[primaryID] = "running" // 恢复，但计时已清零
	fm.checkRecovering(ctx, recovering)
	if p, _ := repo.GetByID(ctx, primaryID); p.FailoverRole != entity.FailoverRoleBackupRecovering {
		t.Fatal("计时清零后不应立即回切")
	}
}
