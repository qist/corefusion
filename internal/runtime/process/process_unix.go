//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
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
