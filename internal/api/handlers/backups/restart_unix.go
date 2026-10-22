//go:build !windows

package backups

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// restartProcess restarts the current process (Unix).
//
// 保留原来 `sh -c "sleep 1 && exec …"` 的**延迟 1 秒**语义 ✓：调用点会先回响应、
// 再重启 ✓，立刻 exec 会与新进程抢同一端口 ✗。
//
// 用 syscall.Exec 原地替换镜像（PID 不变）：systemd（Type=simple）下 spawn 子进程
// 再 exit 会被当成服务停止，整个 cgroup 被回收，新进程随之被杀。
// 参数按数组传递，不经 shell，无注入面（安全审计 V4-S5 / v5 §3）。
func restartProcess() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}

	// 等响应落盘 / 旧连接排空后再换镜像（原 sh 的 sleep 1 ✓）。
	time.Sleep(1 * time.Second)

	argv := append([]string{executable}, os.Args[1:]...)
	if err := syscall.Exec(executable, argv, os.Environ()); err != nil {
		// exec 失败的兜底：退回 spawn。
		cmd := exec.Command(executable, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
		os.Exit(0)
	}
}
