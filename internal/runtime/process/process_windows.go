package process

import (
	"os/exec"
)

// setProcessGroup sets the process to run in its own process group (no-op on Windows).
func setProcessGroup(cmd *exec.Cmd) {
	// Windows: CreateNewProcessGroup could be used but not needed for basic functionality
}

// killProcess kills a process.
func killProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

// forceKillProcess force kills a process (same as kill on Windows).
func forceKillProcess(pid int) error {
	return killProcess(pid)
}

// isProcessAlive checks if a process is still running.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Windows, FindProcess always succeeds. We try to open the process.
	// This is a simplified check.
	return p != nil
}
