//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// setProcessGroup sets the process to run in its own process group.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcess kills a process and its children.
func killProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// forceKillProcess force kills a process and its children.
func forceKillProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

// isProcessAlive checks if a process is still running (not zombie).
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// First check if process exists
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false
	}
	// Check if process is zombie
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		// In restricted environments with PID namespaces, kill(pid, 0) may resolve
		// the namespace PID while /proc only exposes host PIDs. If the signal probe
		// already succeeded, treat the process as alive when /proc status is
		// unavailable instead of incorrectly marking it crashed.
		return true
	}
	return !strings.Contains(string(data), "State:\tZ")
}

// processRunsBinary 判断 pid 当前的可执行文件是否仍是 binaryPath 指向的那个 inode。
// 二进制被更新/替换后 inode 会变化 → 返回 false（说明进程跑的是旧二进制，需要重启）。
func processRunsBinary(pid int, binaryPath string) bool {
	if pid <= 0 || binaryPath == "" {
		return false
	}
	exe, err := os.Stat(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(binaryPath)
	if err != nil {
		abs = binaryPath
	}
	want, err := os.Stat(abs)
	if err != nil {
		return false
	}
	return os.SameFile(exe, want)
}

// processHasArg 判断进程命令行里是否带有 arg（用于确认跑的是本核心的运行时配置）。
func processHasArg(pid int, arg string) bool {
	if pid <= 0 || arg == "" {
		return false
	}
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	for _, f := range strings.Split(string(raw), "\x00") {
		if f == arg {
			return true
		}
	}
	return false
}
