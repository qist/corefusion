//go:build !windows

package process

import (
	"os/exec"
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

// isProcessAlive checks if a process is still running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
