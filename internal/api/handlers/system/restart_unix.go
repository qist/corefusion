//go:build !windows

package system

import (
	"os"
	"os/exec"
)

// restartProcess restarts the current process (Unix).
// Uses exec.Command with argument array (not sh -c) to eliminate the
// command injection surface area entirely.
func restartProcess() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}

	// Pass args as an array — no shell interpretation needed.
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	os.Exit(0)
}
