//go:build !windows

package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// processRunsBinary 必须能识别“二进制被更新替换”：inode 变了就不能接管，
// 否则核心升级后进程还在跑旧版本，且永远不会被重启。
func TestProcessRunsBinaryDetectsReplacement(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable: %v", err)
	}
	if !processRunsBinary(os.Getpid(), exe) {
		t.Fatal("自身进程应该匹配自己的可执行文件")
	}

	// 复制一份到临时路径：内容相同但 inode 不同 → 不是“同一个文件”。
	copyPath := filepath.Join(t.TempDir(), "copy.bin")
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Skipf("read exe: %v", err)
	}
	if err := os.WriteFile(copyPath, data, 0o755); err != nil {
		t.Skipf("write copy: %v", err)
	}
	if processRunsBinary(os.Getpid(), copyPath) {
		t.Fatal("不同 inode 的副本不应被当成正在运行的二进制")
	}
}

// processHasArg 用于确认 PID 跑的确实是本核心的运行时配置，避免 PID 复用误接管。
func TestProcessHasArgMatchesCommandLine(t *testing.T) {
	raw, err := os.ReadFile("/proc/self/cmdline")
	if err != nil {
		t.Skipf("read cmdline: %v", err)
	}
	args := strings.Split(string(raw), "\x00")
	var last string
	for _, a := range args {
		if a != "" {
			last = a
		}
	}
	if last == "" {
		t.Skip("cmdline 中没有可用参数")
	}
	if !processHasArg(os.Getpid(), last) {
		t.Fatalf("processHasArg 未匹配真实参数 %q", last)
	}
	if processHasArg(os.Getpid(), "definitely-not-an-arg-"+last) {
		t.Fatal("不存在的参数不应匹配")
	}
}

// TestReconcileAdoptsRunningCore 锁定核心接管语义：
// 面板/Agent 重启后，guardian 里记录的 PID 仍存活且二进制/配置都对得上时必须**接管**，
// 不能 kill 掉再拉起（否则所有入站断流数秒）。
func TestReconcileAdoptsRunningCore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("内存 SQLite 不可用（跳过）：%v", err)
	}
	if err := db.AutoMigrate(&entity.InboundService{}, &entity.ProcessGuardian{}, &entity.CoreCrashLog{}); err != nil {
		t.Skipf("迁移失败（跳过）：%v", err)
	}
	svcRepo := repository.NewServiceRepository(db)
	guardRepo := repository.NewProcessGuardianRepository(db)
	crashRepo := repository.NewCrashLogRepository(db)

	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "runtime"), 0o750); err != nil {
		t.Fatal(err)
	}
	relConfig := filepath.Join("runtime", "xray.jsonc")
	if err := os.WriteFile(filepath.Join(dataDir, relConfig), []byte(`{"log":{}}`), 0o640); err != nil {
		t.Fatal(err)
	}

	svc := &entity.InboundService{ServerID: 1, Name: "Xray", CoreType: "xray", Protocol: "-"}
	if err := svcRepo.Create(context.Background(), svc); err != nil {
		t.Fatal(err)
	}

	// 用一个真实子进程模拟“正在运行的核心”：可执行文件就是测试二进制本身，
	// 命令行里带上运行时配置路径（adoptRunning 的两项校验都成立）。
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable: %v", err)
	}
	child := exec.Command(exe, "-test.run=TestAdoptHelperProcess", "--", relConfig)
	child.Dir = dataDir
	child.Env = append(os.Environ(), "GO_WANT_ADOPT_HELPER=1")
	if err := child.Start(); err != nil {
		t.Skipf("启动 helper 进程失败（跳过）：%v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	}()

	// 等 /proc/<pid>/exe 可用
	deadline := time.Now().Add(5 * time.Second)
	for !processRunsBinary(child.Process.Pid, exe) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !processRunsBinary(child.Process.Pid, exe) {
		t.Skip("helper 进程未就绪")
	}

	startedAt := time.Now().Add(-time.Minute)
	g := &entity.ProcessGuardian{
		ServiceID:       svc.ID,
		ProcessStatus:   "running",
		PID:             child.Process.Pid,
		StartedAt:       startedAt,
		MaxRestartCount: 5,
	}
	if err := guardRepo.Create(context.Background(), g); err != nil {
		t.Fatal(err)
	}

	m := NewManager(dataDir, svcRepo, guardRepo, crashRepo, stubResolver{path: exe})
	m.Reconcile(context.Background())
	defer m.DetachAll(context.Background())

	m.mu.RLock()
	p, ok := m.processes[svc.ID]
	var pid int
	var state string
	if ok {
		pid, state = p.pid, p.state
	}
	m.mu.RUnlock()

	if !ok {
		t.Fatal("Reconcile 未接管仍在运行的核心")
	}
	if pid != child.Process.Pid {
		t.Fatalf("接管后 PID = %d, want 原进程 %d（不能重启）", pid, child.Process.Pid)
	}
	if state != "running" {
		t.Fatalf("接管后状态 = %q, want running", state)
	}

	// 原进程必须还活着（没有被 kill 再拉起）
	if !isProcessAlive(child.Process.Pid) {
		t.Fatal("接管不应杀掉原核心进程")
	}

	// DB 里保留原 PID 与启动时间
	got, err := guardRepo.GetByServiceID(context.Background(), svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != child.Process.Pid || got.ProcessStatus != "running" {
		t.Fatalf("guardian 记录 = %+v, want pid=%d running", got, child.Process.Pid)
	}
	if !got.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt 被改写：%v, want %v", got.StartedAt, startedAt)
	}
}

type stubResolver struct {
	path string
}

func (s stubResolver) Resolve(coreType string) (string, string, error) {
	return s.path, "test", nil
}

// TestAdoptHelperProcess 不是真正的测试：被 TestReconcileAdoptsRunningCore
// 拉起后保持存活，充当“正在运行的核心进程”。
func TestAdoptHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_ADOPT_HELPER") != "1" {
		return
	}
	for {
		time.Sleep(time.Hour)
	}
}
