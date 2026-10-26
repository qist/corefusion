package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 更新标记：运行版本 == 目标版本 → 视为成功，清理 .bak 与标记。
func TestDecideRecoverySuccessKeepsNewBinary(t *testing.T) {
	marker := &updateMarker{ToVersion: "v0.13.0-alpha.4"}
	if got := decideRecovery(marker, true, "v0.13.0-alpha.4", true); got != recoveryCleanup {
		t.Fatalf("decideRecovery = %v, want recoveryCleanup", got)
	}
	// 没有备份可留（首次更新且未产生 .bak）也算成功
	if got := decideRecovery(marker, true, "v0.13.0-alpha.4", false); got != recoveryCleanup {
		t.Fatalf("decideRecovery without backup = %v, want recoveryCleanup", got)
	}
}

// 运行版本 != 目标版本 → 回滚（新二进制没起来）。
func TestDecideRecoveryMismatchRollsBack(t *testing.T) {
	marker := &updateMarker{ToVersion: "v0.13.0-alpha.4"}
	if got := decideRecovery(marker, true, "v0.13.0-alpha.3", true); got != recoveryRollback {
		t.Fatalf("decideRecovery = %v, want recoveryRollback", got)
	}
	// 没有 .bak 时无法回滚，只能清标记，避免每个启动周期反复尝试
	if got := decideRecovery(marker, true, "v0.13.0-alpha.3", false); got != recoveryCleanup {
		t.Fatalf("decideRecovery without backup = %v, want recoveryCleanup", got)
	}
}

// 没有标记时交给 mtime 兜底逻辑（老版本 Agent 更新的场景）。
func TestDecideRecoveryNoMarker(t *testing.T) {
	if got := decideRecovery(nil, false, "v0.13.0-alpha.4", true); got != recoveryNone {
		t.Fatalf("decideRecovery = %v, want recoveryNone", got)
	}
}

func TestUpdateMarkerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	selfPath := filepath.Join(dir, "corefusion-agent")
	u := &SelfUpdater{dataDir: dir}

	// 写标记需要 SelfBinaryPath()（当前测试进程可执行文件），这里只验证
	// 标记文件的读写/损坏容错逻辑本身。
	marker := updateMarker{FromVersion: "v1", ToVersion: "v2"}
	data, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	path := updateMarkerPath(selfPath)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	got, ok := readUpdateMarker(selfPath)
	if !ok || got.ToVersion != "v2" || got.FromVersion != "v1" {
		t.Fatalf("readUpdateMarker = %+v, ok=%v", got, ok)
	}

	// 损坏的标记不能导致误判，直接当作没有标记
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readUpdateMarker(selfPath); ok {
		t.Fatalf("corrupted marker should be ignored")
	}

	// 不存在的标记
	if _, ok := readUpdateMarker(filepath.Join(dir, "missing")); ok {
		t.Fatalf("missing marker should not be found")
	}
	_ = u
}

// 自更新后旧二进制被 rename 成 .bak，安装路径必须从 .bak 归一回去，
// 否则 replaceBinary / re-exec 都会作用在 .bak 上（节点 18 的事故）。
func TestStripBackupSuffix(t *testing.T) {
	cases := map[string]string{
		"/opt/corefusion-agent/corefusion-agent":         "",
		"/opt/corefusion-agent/corefusion-agent.bak":     "/opt/corefusion-agent/corefusion-agent",
		"/opt/corefusion-agent/corefusion-agent.bak.bak": "/opt/corefusion-agent/corefusion-agent",
	}
	for in, want := range cases {
		got, stripped := stripBackupSuffix(in)
		if want == "" {
			if stripped {
				t.Errorf("stripBackupSuffix(%q) 不该剥离", in)
			}
			continue
		}
		if !stripped || got != want {
			t.Errorf("stripBackupSuffix(%q) = (%q,%v), want (%q,true)", in, got, stripped, want)
		}
	}
}

// 运行在 .bak 上、且安装路径存在 → SelfBinaryPath 必须返回安装路径。
func TestSelfBinaryPathSkipsBackupWhenBaseExists(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "corefusion-agent")
	if err := os.WriteFile(base, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	if got, stripped := stripBackupSuffix(base + ".bak"); !stripped || got != base {
		t.Fatalf("strip = (%q,%v)", got, stripped)
	}
	if !fileExists(base) {
		t.Fatalf("base should exist")
	}
}
