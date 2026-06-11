//go:build !windows

package system

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// restartProcess restarts the current process (Unix).
func restartProcess() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}

	args := strings.Join(os.Args[1:], " ")
	cmd := fmt.Sprintf("sleep 1 && exec %s %s", executable, args)
	exec.Command("sh", "-c", cmd).Start()
	os.Exit(0)
}
