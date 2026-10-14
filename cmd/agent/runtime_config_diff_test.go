package main

import "testing"

// TestRuntimeConfigDiffers 回归：变更判定必须相对「运行态基准」，而不是磁盘文件。
//
// 真机事故：面板的 PanelCommand.SyncConfig 只写盘、不动进程，且与 SyncData 并发下发；
// 拿磁盘当基准会把"文件已更新、进程仍跑旧配置"误判成"未变"→ 创建链式代理后入口核心
// 不重载、链式路由静默不生效（流量落到 direct 直连）。
func TestRuntimeConfigDiffers(t *testing.T) {
	cases := []struct {
		name   string
		basis  string
		config string
		want   bool
	}{
		{"相同 → 未变", `{"a":1}`, `{"a":1}`, false},
		{"仅空白差异 → 未变", `{"a":1}`, "  {\"a\":1}\n", false},
		{"内容不同 → 变", `{"a":1}`, `{"a":2}`, true},
		{"基准为空（首次）→ 视为变", ``, `{"a":1}`, true},
		{"本次配置为空 → 不算变更", `{"a":1}`, ``, false},
	}
	for _, c := range cases {
		if got := runtimeConfigDiffers([]byte(c.basis), []byte(c.config)); got != c.want {
			t.Errorf("%s: runtimeConfigDiffers=%v, want %v", c.name, got, c.want)
		}
	}
}
