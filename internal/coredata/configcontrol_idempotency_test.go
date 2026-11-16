package coredata

import (
	"context"
	"strings"
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// scriptedRemote 按方法返回预设的 xray 错误（队列为空即成功），并记录调用顺序。
type scriptedRemote struct {
	calls []string
	errs  map[string][]string
}

func (f *scriptedRemote) XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	f.calls = append(f.calls, req.GetMethod())
	if q := f.errs[req.GetMethod()]; len(q) > 0 {
		f.errs[req.GetMethod()] = q[1:]
		return &agentpb.XrayAPIResponse{Success: false, Error: q[0]}, nil
	}
	return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"ok":true}`)}, nil
}

func scriptedProvider(r *scriptedRemote) *ControlProvider {
	return &ControlProvider{isLocal: func(int64) bool { return false }, remote: r}
}

// 关闭入口时，Agent 的热重载可能已经先删过同一个 tag；此时 xray 返回
// common.ErrNoClue，必须按「已删除」成功处理，否则前端会显示假的「接口不可用」。
func TestRemoveInboundTreatsMissingAsSuccess(t *testing.T) {
	f := &scriptedRemote{errs: map[string][]string{
		"removeInbound": {"rpc error: code = Unknown desc = common: not enough information for making a decision"},
	}}
	p := scriptedProvider(f)
	if err := p.RemoveInbound(context.Background(), 4, "inbound-vless-32780"); err != nil {
		t.Fatalf("missing inbound should be treated as success, got %v", err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("calls = %v", f.calls)
	}
}

// 真正的控制面错误仍要上报（不能把断连当成删除成功，否则用户看不到问题）。
func TestRemoveInboundPropagatesRealErrors(t *testing.T) {
	f := &scriptedRemote{errs: map[string][]string{
		"removeInbound": {"rpc error: code = Unavailable desc = transport is closing"},
	}}
	p := scriptedProvider(f)
	if err := p.RemoveInbound(context.Background(), 4, "inbound-x"); err == nil {
		t.Fatal("expected error for transport failure")
	}
}

func TestRemoveOutboundTreatsMissingAsSuccess(t *testing.T) {
	f := &scriptedRemote{errs: map[string][]string{
		"removeOutbound": {"rpc error: code = Unknown desc = common: not enough information for making a decision"},
	}}
	p := scriptedProvider(f)
	if err := p.RemoveOutbound(context.Background(), 4, "outbound-x"); err != nil {
		t.Fatalf("missing outbound should be treated as success, got %v", err)
	}
}

// 面板热加用户与 Agent 热重载交叉时 AddUser 会报「用户已存在」；
// 应自动「先删该 email 再加一次」，而不是把错误抛给用户。
func TestAddUserRetriesWhenUserAlreadyExists(t *testing.T) {
	f := &scriptedRemote{errs: map[string][]string{
		"addUser": {"rpc error: code = Unknown desc = User 4wo1rz@xray.com already exists"},
	}}
	p := scriptedProvider(f)
	err := p.AddUser(context.Background(), 4, "inbound-vmess-1", "vmess", "4wo1rz@xray.com", "uuid", "", "", "auto")
	if err != nil {
		t.Fatalf("AddUser should recover from existing user, got %v", err)
	}
	want := []string{"addUser", "removeUser", "addUser"}
	if strings.Join(f.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}

func TestRemoveUserTreatsMissingAsSuccess(t *testing.T) {
	f := &scriptedRemote{errs: map[string][]string{
		"removeUser": {"rpc error: code = Unknown desc = User foo@bar not found"},
	}}
	p := scriptedProvider(f)
	if err := p.RemoveUser(context.Background(), 4, "inbound-1", "foo@bar"); err != nil {
		t.Fatalf("missing user should be treated as success, got %v", err)
	}
}

func TestIsMissingHandlerErr(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"common: not enough information for making a decision", true},
		{"User foo@bar not found", true},
		{"handler not found", true},
		{"rpc error: code = Unavailable desc = transport is closing", false},
		{"failed to build inbound: invalid settings", false},
	}
	for _, tc := range cases {
		if got := IsMissingHandlerErr(errString(tc.msg)); got != tc.want {
			t.Errorf("IsMissingHandlerErr(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}
