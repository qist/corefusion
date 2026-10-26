package nodeserver

import (
	"errors"
	"testing"
	"time"

	"corefusion/internal/model/entity"
)

// 重新引导令牌的签发门：只有证书不可用（从未签发 / 已过期）时才允许。
func TestEnsureBootstrapAllowed(t *testing.T) {
	svc := &Service{}
	now := time.Now()
	future := now.Add(5 * 24 * time.Hour)
	past := now.Add(-1 * time.Hour)

	cases := []struct {
		name string
		srv  *entity.NodeServer
		want error
	}{
		{
			name: "本地面板节点不允许",
			srv:  &entity.NodeServer{ID: 1, IsLocal: true},
			want: ErrBootstrapLocalNode,
		},
		{
			name: "证书仍有效不允许（即使在续签窗口内）",
			srv:  &entity.NodeServer{ID: 2, AgentCertExpiresAt: &future},
			want: ErrAgentCertStillValid,
		},
		{
			name: "证书已过期允许",
			srv:  &entity.NodeServer{ID: 3, AgentCertExpiresAt: &past},
			want: nil,
		},
		{
			name: "从未签发（新建节点/已吊销）允许",
			srv:  &entity.NodeServer{ID: 4},
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := svc.ensureBootstrapAllowed(c.srv)
			if c.want == nil {
				if err != nil {
					t.Fatalf("expected allow, got %v", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}
