package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestXrayPendingQueueDedupesByTarget(t *testing.T) {
	var q xrayPendingQueue
	q.add(4, XrayPendingOp{Kind: XrayPendingInbound, Tag: "inbound-443"})
	q.add(4, XrayPendingOp{Kind: XrayPendingOutbound, Tag: "proxy"})
	// 同 tag 的入站重复记录只保留一条
	q.add(4, XrayPendingOp{Kind: XrayPendingInbound, Tag: "inbound-443"})
	q.add(4, XrayPendingOp{Kind: XrayPendingRouting})
	q.add(4, XrayPendingOp{Kind: XrayPendingRouting})

	if got := q.len(4); got != 3 {
		t.Fatalf("pending count = %d, want 3", got)
	}
	// 不同节点互不影响
	q.add(5, XrayPendingOp{Kind: XrayPendingInbound, Tag: "inbound-443"})
	if got := q.len(5); got != 1 {
		t.Fatalf("server 5 pending count = %d, want 1", got)
	}
}

func TestXrayPendingQueueTakeAndRestore(t *testing.T) {
	var q xrayPendingQueue
	q.add(4, XrayPendingOp{Kind: XrayPendingInbound, Tag: "a"})
	q.add(4, XrayPendingOp{Kind: XrayPendingInbound, Tag: "b"})

	ops := q.take(4)
	if len(ops) != 2 {
		t.Fatalf("taken = %d, want 2", len(ops))
	}
	if got := q.len(4); got != 0 {
		t.Fatalf("after take pending count = %d, want 0", got)
	}

	// 补下发失败放回；期间新增的同目标记录不应被旧记录覆盖
	q.add(4, XrayPendingOp{Kind: XrayPendingInbound, Tag: "c"})
	q.restore(4, ops)
	if got := q.len(4); got != 3 {
		t.Fatalf("after restore pending count = %d, want 3", got)
	}
	// 同一目标重复 restore 不会翻倍
	q.restore(4, ops)
	if got := q.len(4); got != 3 {
		t.Fatalf("after second restore pending count = %d, want 3", got)
	}
}

func TestXrayPendingQueueSkipsInvalid(t *testing.T) {
	var q xrayPendingQueue
	q.add(4, XrayPendingOp{Kind: XrayPendingInbound})
	q.add(0, XrayPendingOp{Kind: XrayPendingRouting})
	q.add(4, XrayPendingOp{})
	if got := q.len(4); got != 0 {
		t.Fatalf("invalid ops recorded: %d", got)
	}
}

func TestNodeServerManagerMarkSkipsLocal(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	m.MarkXrayPending(1, XrayPendingOp{Kind: XrayPendingRouting})
	if got := m.XrayPendingCount(1); got != 0 {
		t.Fatalf("local node should not queue pending ops, got %d", got)
	}
	m.MarkXrayPending(4, XrayPendingOp{Kind: XrayPendingRouting})
	if got := m.XrayPendingCount(4); got != 1 {
		t.Fatalf("remote pending count = %d, want 1", got)
	}
}

func TestIsXrayControlUnavailable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"not connected", &NotConnectedError{ServerID: 4}, true},
		{"wrapped not connected", fmt.Errorf("send command: %w", &NotConnectedError{ServerID: 4}), true},
		{"grpc unavailable", status.Error(codes.Unavailable, "transport is closing"), true},
		{"grpc deadline", status.Error(codes.DeadlineExceeded, "context deadline exceeded"), true},
		// xray 业务错误：指令已送达，重放无意义
		{"business error", errors.New("failed to add user: handler not found"), false},
		{"parse error", errors.New("parse inbound config: unexpected end of JSON input"), false},
	}
	for _, tc := range cases {
		if got := IsXrayControlUnavailable(tc.err); got != tc.want {
			t.Errorf("%s: IsXrayControlUnavailable = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// fakeReplayer 记录补下发调用，可注入失败。
type fakeReplayer struct {
	calls int
	err   error
}

func (f *fakeReplayer) ReplayXrayPending(ctx context.Context, serverID int64, ops []XrayPendingOp) error {
	f.calls++
	return f.err
}

func TestReplayPendingXrayConsumesQueue(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	f := &fakeReplayer{}
	m.SetXrayPendingReplayer(f)
	m.MarkXrayPending(4, XrayPendingOp{Kind: XrayPendingInbound, Tag: "inbound-443"})
	m.MarkXrayPending(4, XrayPendingOp{Kind: XrayPendingRouting})

	m.replayPendingXray(4)
	if f.calls != 1 {
		t.Fatalf("replayer calls = %d, want 1", f.calls)
	}
	if got := m.XrayPendingCount(4); got != 0 {
		t.Fatalf("pending after success = %d, want 0", got)
	}
}

func TestReplayPendingXrayKeepsQueueOnFailure(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	f := &fakeReplayer{err: errors.New("boom")}
	m.SetXrayPendingReplayer(f)
	m.MarkXrayPending(4, XrayPendingOp{Kind: XrayPendingOutbound, Tag: "proxy"})

	m.replayPendingXray(4)
	if f.calls != 1 {
		t.Fatalf("replayer calls = %d, want 1", f.calls)
	}
	if got := m.XrayPendingCount(4); got != 1 {
		t.Fatalf("pending after failure = %d, want 1 (kept for next reconnect)", got)
	}
}

func TestReplayPendingXrayNoopWithoutPending(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	f := &fakeReplayer{}
	m.SetXrayPendingReplayer(f)
	m.replayPendingXray(4)
	if f.calls != 0 {
		t.Fatalf("replayer should not be called without pending ops, calls=%d", f.calls)
	}
}
