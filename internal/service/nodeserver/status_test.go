package nodeserver

import (
	"errors"
	"fmt"
	"testing"

	"corefusion/internal/runtime/agent"
)

// 状态轮询的过渡态错误应降级为空状态（否则 Agent 重启时前端每 5s 报一次 500）。
func TestIsTransientServerStatusError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"agent 未连接", &agent.NotConnectedError{ServerID: 20}, true},
		{"包装后的未连接", fmt.Errorf("get status: %w", &agent.NotConnectedError{ServerID: 20}), true},
		{"尚未上报", errors.New("server 20 status not yet reported"), true},
		{"真正的内部错误", errors.New("database is locked"), false},
	}
	for _, tc := range cases {
		if got := isTransientServerStatusError(tc.err); got != tc.want {
			t.Errorf("%s: isTransientServerStatusError(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
}
