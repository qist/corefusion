package system

import (
	"os"
	"os/exec"
	"time"
)

// restartProcess restarts the current process (Windows).
func restartProcess() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}

	// On Windows, start a new process and exit the current one
	go func() {
		time.Sleep(1 * time.Second)
		cmd := exec.Command(executable, os.Args[1:]...)
		cmd.Start()
	}()
	os.Exit(0)
}
