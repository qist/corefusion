package coredata

import (
	"context"
	"strings"
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// fakeXrayRemote 模拟远端 Agent 的 proxyman 调用；addFailures 控制 addInbound 前 N 次失败。
type fakeXrayRemote struct {
	calls       []string
	addFailures int
	lastAddErr  string
}

func (f *fakeXrayRemote) XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	f.calls = append(f.calls, req.GetMethod())
	if req.GetMethod() == "addInbound" && f.addFailures > 0 {
		f.addFailures--
		return &agentpb.XrayAPIResponse{Success: false, Error: f.lastAddErr}, nil
	}
	return &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"message":"ok"}`)}, nil
}

func remoteOnlyProvider(r *fakeXrayRemote) *ControlProvider {
	return &ControlProvider{
		isLocal: func(int64) bool { return false },
		remote:  r,
	}
}

// 面板保存与 Agent 热重载并发时，addInbound 会撞上 existing tag；
// ControlProvider 应「先删后加」并重试一次，让保存不再报错。
func TestAddInboundRetriesOnExistingTag(t *testing.T) {
	f := &fakeXrayRemote{
		addFailures: 1,
		lastAddErr:  "rpc error: code = Unknown desc = app/proxyman/inbound: existing tag found: inbound-vless-32780",
	}
	p := remoteOnlyProvider(f)
	cfg := []byte(`{"tag":"inbound-vless-32780","protocol":"vless","port":32780,"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}],"decryption":"none"}}`)

	if err := p.AddInbound(context.Background(), 4, cfg); err != nil {
		t.Fatalf("AddInbound should recover from existing tag, got %v", err)
	}
	want := []string{"removeInbound", "addInbound", "removeInbound", "addInbound"}
	if strings.Join(f.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v, want %v", f.calls, want)
	}
}

// 非 tag 冲突的业务错误不重试，直接返回（由调用方提示原因）。
func TestAddInboundDoesNotRetryOtherErrors(t *testing.T) {
	f := &fakeXrayRemote{
		addFailures: 1,
		lastAddErr:  "failed to build inbound: invalid settings",
	}
	p := remoteOnlyProvider(f)
	cfg := []byte(`{"tag":"inbound-vless-1","protocol":"vless","port":1,"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}],"decryption":"none"}}`)

	err := p.AddInbound(context.Background(), 4, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(f.calls) != 2 {
		t.Fatalf("calls = %v, want one remove+add attempt", f.calls)
	}
}

func TestIsExistingTagErr(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"remote error: rpc error: code = Unknown desc = app/proxyman/inbound: existing tag found: x", true},
		{"existing handler: foo", true},
		{"failed to connect to xray: connection refused", false},
		{"failed to build inbound: invalid", false},
	}
	for _, tc := range cases {
		if got := IsExistingTagErr(errString(tc.msg)); got != tc.want {
			t.Errorf("IsExistingTagErr(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

type errString string

func (e errString) Error() string { return string(e) }
