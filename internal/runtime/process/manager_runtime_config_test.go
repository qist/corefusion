package process

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestManager 构造一个只带 dataDir 的 Manager。
// RuntimeConfigChanged / RuntimeConfigPath / WriteRuntimeConfig 都不依赖仓库与二进制解析器
// （传 nil 即可），所以这里无需任何桩。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(t.TempDir(), nil, nil, nil, nil)
}

// TestRuntimeConfigChanged 锁定 reload_strategy=none 的核心判断：
// "把 data 写下去是否会真的改变磁盘上的配置"。
//
// 这个判断决定"同步时要不要重启核心"：
//   - 判错成 false → **变更静默不生效** ✗（真机踩过：远程节点核心两天没重载、链式配置写了从未加载）
//   - 判错成 true  → 每次同步都白重启核心 ✗（抖动、断流）
func TestRuntimeConfigChanged(t *testing.T) {
	m := newTestManager(t)

	// ① 文件还不存在 → 视为变更（首次下发必须生效）
	if !m.RuntimeConfigChanged("xray", []byte(`{"a":1}`)) {
		t.Fatal("文件不存在时应视为变更")
	}

	// ② 写入后内容完全相同 → 不变更（避免每次同步都白重启）
	if err := m.WriteRuntimeConfig("xray", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("WriteRuntimeConfig: %v", err)
	}
	if m.RuntimeConfigChanged("xray", []byte(`{"a":1}`)) {
		t.Error("内容完全相同时不应视为变更")
	}

	// ③ 仅首尾空白差异 → 不变更（实现按 TrimSpace 比较；面板可能带尾随换行）
	if m.RuntimeConfigChanged("xray", []byte("\n  {\"a\":1}\t\n")) {
		t.Error("仅首尾空白差异不应视为变更")
	}

	// ④ 内容真的变了 → 变更
	if !m.RuntimeConfigChanged("xray", []byte(`{"a":2}`)) {
		t.Error("内容变化时必须视为变更")
	}

	// ⑤ 长度相同但内容不同也必须识别（防止将来改成"比长度"之类的取巧实现）
	if !m.RuntimeConfigChanged("xray", []byte(`{"a":3}`)) {
		t.Error("等长但内容不同的配置必须识别为变更")
	}
}

// TestRuntimeConfigChangedIsPerCore 不同核心各写各的文件（runtime/<core>.jsonc），
// 互不干扰：写 singbox 不该让 xray 的判断变成"没变"。
func TestRuntimeConfigChangedIsPerCore(t *testing.T) {
	m := newTestManager(t)

	if err := m.WriteRuntimeConfig("singbox", []byte(`{"sb":1}`)); err != nil {
		t.Fatalf("WriteRuntimeConfig: %v", err)
	}
	if !m.RuntimeConfigChanged("xray", []byte(`{"sb":1}`)) {
		t.Error("另一个核心的文件不该影响 xray 的判断（xray 还没有文件 → 应视为变更）")
	}
	if m.RuntimeConfigChanged("singbox", []byte(`{"sb":1}`)) {
		t.Error("singbox 内容相同不应视为变更")
	}
}

// TestRuntimeConfigChangedReflectsSyncFlow 走一遍真实的同步语义：
// 第一次同步 → 变更（核心需要启动/重启）→ 落盘 → 同样内容的第二次同步 → 不变更（不重启）。
func TestRuntimeConfigChangedReflectsSyncFlow(t *testing.T) {
	m := newTestManager(t)
	cfg := []byte(`{"inbounds":[{"port":10087}]}`)

	changed1 := m.RuntimeConfigChanged("xray", cfg)
	if err := m.WriteRuntimeConfig("xray", cfg); err != nil {
		t.Fatalf("WriteRuntimeConfig: %v", err)
	}
	changed2 := m.RuntimeConfigChanged("xray", cfg)

	if !changed1 {
		t.Error("第一次同步应判定为变更（核心需要启动/重启）")
	}
	if changed2 {
		t.Error("内容相同的重复同步不应判定为变更（否则每次同步都会白重启核心）")
	}

	// 顺带钉住落盘路径：dataDir/runtime/<core>.jsonc
	want := filepath.Join(m.dataDir, "runtime", "xray.jsonc")
	if got := m.RuntimeConfigPath("xray"); got != want {
		t.Errorf("RuntimeConfigPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("配置应已落盘到 %s：%v", want, err)
	}
}
