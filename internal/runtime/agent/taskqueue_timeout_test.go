package agent

import (
	"testing"
	"time"
)

func TestTaskTimeout(t *testing.T) {
	cases := []struct {
		taskType string
		want     time.Duration
	}{
		{"install_binary", taskTimeoutInstall},
		{"install_proxy", taskTimeoutInstall},
		{"control_core", taskTimeoutDefault},
		{"sync_config", taskTimeoutDefault},
		{"", taskTimeoutDefault},
	}
	for _, c := range cases {
		if got := taskTimeout(c.taskType); got != c.want {
			t.Errorf("taskTimeout(%q) = %v, want %v", c.taskType, got, c.want)
		}
	}
	if taskTimeoutInstall <= taskTimeoutDefault {
		t.Fatalf("install timeout %v must be longer than default %v", taskTimeoutInstall, taskTimeoutDefault)
	}
}
