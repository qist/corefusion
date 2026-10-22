//go:build !windows

package system

import (
	"os"
	"os/exec"
	"syscall"
)

// restartProcess restarts the current process (Unix).
//
// 用 syscall.Exec 原地替换进程镜像（PID 不变）：面板现在多由 systemd
// （Type=simple）托管，如果先 start 子进程再 exit(0)，systemd 会认为主进程
// 正常退出，进而把整个 cgroup（含刚起的子进程）回收 —— 表现为"自更新后
// 面板起不来"。exec 保持 PID，systemd 完全无感。
//
// 参数按数组传递，不经 shell，无注入面（安全审计 V4-S5 / v5 §3）。
func restartProcess() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}
	argv := append([]string{executable}, os.Args[1:]...)
	if err := syscall.Exec(executable, argv, os.Environ()); err != nil {
		// exec 失败的兜底：退回 spawn，至少尝试拉起新进程。
		cmd := exec.Command(executable, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Start()
		os.Exit(0)
	}
}
