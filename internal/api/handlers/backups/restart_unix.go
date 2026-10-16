//go:build !windows

package backups

import (
	"os"
	"os/exec"
	"time"
)

// restartProcess restarts the current process (Unix).
// Uses exec.Command with an argument array (not `sh -c`) to eliminate the
// command injection surface area entirely — same approach as the system
// handler's restartProcess (安全审计 V4-S5 / v5 §3).
//
// 保留原来 `sh -c "sleep 1 && exec …"` 的**延迟 1 秒**语义 ✓：调用点会先回响应、
// 再重启 ✓，立刻 exec 会与新进程抢同一端口 ✗。
func restartProcess() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}

	// 等响应落盘 / 旧连接排空后再拉起新进程（原 sh 的 sleep 1 ✓）。
	time.Sleep(1 * time.Second)

	// 参数按数组传递 —— 不经 shell 解释，无注入面。
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	os.Exit(0)
}
