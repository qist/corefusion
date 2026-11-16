package router

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	agentrt "corefusion/internal/runtime/agent"
)

func TestXrayPendingOpOf(t *testing.T) {
	cases := []struct {
		name    string
		service string
		method  string
		payload string
		want    agentrt.XrayPendingOp
		ok      bool
	}{
		{
			name:    "add inbound",
			service: "proxyman",
			method:  "addInbound",
			payload: `{"config":{"tag":"inbound-443"}}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingInbound, Tag: "inbound-443"},
			ok:      true,
		},
		{
			name:    "remove inbound",
			service: "proxyman",
			method:  "removeInbound",
			payload: `{"tag":"inbound-443"}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingInbound, Tag: "inbound-443"},
			ok:      true,
		},
		{
			// 加用户也算该入站需要重新同步（入站配置里自带用户列表）
			name:    "add user",
			service: "proxyman",
			method:  "addUser",
			payload: `{"tag":"inbound-443","email":"a@b.c","protocol":"vless"}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingInbound, Tag: "inbound-443"},
			ok:      true,
		},
		{
			name:    "add outbound",
			service: "proxyman",
			method:  "addOutbound",
			payload: `{"config":{"tag":"warp-out"}}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingOutbound, Tag: "warp-out"},
			ok:      true,
		},
		{
			name:    "remove outbound",
			service: "proxyman",
			method:  "removeOutbound",
			payload: `{"tag":"warp-out"}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingOutbound, Tag: "warp-out"},
			ok:      true,
		},
		{
			name:    "apply routing",
			service: "router",
			method:  "applyRouting",
			payload: `{"routing":{"rules":[]}}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingRouting},
			ok:      true,
		},
		{
			name:    "balancer counts as routing",
			service: "router",
			method:  "alterBalancer",
			payload: `{}`,
			want:    agentrt.XrayPendingOp{Kind: agentrt.XrayPendingRouting},
			ok:      true,
		},
		{
			name:    "unknown method",
			service: "proxyman",
			method:  "getInboundUsers",
			payload: `{"tag":"inbound-443"}`,
			ok:      false,
		},
		{
			name:    "missing tag",
			service: "proxyman",
			method:  "addInbound",
			payload: `{"config":{}}`,
			ok:      false,
		},
	}
	for _, tc := range cases {
		got, ok := xrayPendingOpOf(tc.service, tc.method, []byte(tc.payload))
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("%s: op = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// fakeControl 记录补下发调用，可注入失败。
type fakeControl struct {
	addInbounds    []string
	removeInbounds []string
	addOutbounds   []string
	removeOutbound []string
	routings       int
	failOn         string
}

func (f *fakeControl) AddInbound(ctx context.Context, serverID int64, configJSON []byte) error {
	if f.failOn == "addInbound" {
		return errors.New("boom")
	}
	f.addInbounds = append(f.addInbounds, string(configJSON))
	return nil
}

func (f *fakeControl) RemoveInbound(ctx context.Context, serverID int64, tag string) error {
	f.removeInbounds = append(f.removeInbounds, tag)
	return nil
}

func (f *fakeControl) AddOutbound(ctx context.Context, serverID int64, configJSON []byte) error {
	if f.failOn == "addOutbound" {
		return errors.New("boom")
	}
	f.addOutbounds = append(f.addOutbounds, string(configJSON))
	return nil
}

func (f *fakeControl) RemoveOutbound(ctx context.Context, serverID int64, tag string) error {
	f.removeOutbound = append(f.removeOutbound, tag)
	return nil
}

func (f *fakeControl) ApplyRouting(ctx context.Context, serverID int64, routingJSON []byte) error {
	f.routings++
	return nil
}

func TestApplyXrayPendingSyncsToDesiredState(t *testing.T) {
	desired := &xrayDesiredRuntime{
		Inbounds: map[string]json.RawMessage{
			"inbound-443": json.RawMessage(`{"tag":"inbound-443"}`),
		},
		Outbounds: map[string]json.RawMessage{
			"proxy": json.RawMessage(`{"tag":"proxy"}`),
		},
		Routing: json.RawMessage(`{"rules":[]}`),
	}
	ops := []agentrt.XrayPendingOp{
		{Kind: agentrt.XrayPendingInbound, Tag: "inbound-443"},
		{Kind: agentrt.XrayPendingInbound, Tag: "inbound-gone"},
		{Kind: agentrt.XrayPendingOutbound, Tag: "proxy"},
		{Kind: agentrt.XrayPendingOutbound, Tag: "outbound-gone"},
		{Kind: agentrt.XrayPendingRouting},
		{Kind: agentrt.XrayPendingRouting},
	}

	ctl := &fakeControl{}
	if err := applyXrayPending(context.Background(), 4, ops, desired, ctl); err != nil {
		t.Fatalf("applyXrayPending: %v", err)
	}
	if len(ctl.addInbounds) != 1 || ctl.addInbounds[0] != `{"tag":"inbound-443"}` {
		t.Errorf("addInbounds = %v", ctl.addInbounds)
	}
	if len(ctl.removeInbounds) != 1 || ctl.removeInbounds[0] != "inbound-gone" {
		t.Errorf("removeInbounds = %v", ctl.removeInbounds)
	}
	if len(ctl.addOutbounds) != 1 || ctl.addOutbounds[0] != `{"tag":"proxy"}` {
		t.Errorf("addOutbounds = %v", ctl.addOutbounds)
	}
	if len(ctl.removeOutbound) != 1 || ctl.removeOutbound[0] != "outbound-gone" {
		t.Errorf("removeOutbound = %v", ctl.removeOutbound)
	}
	if ctl.routings != 2 {
		t.Errorf("routings = %d, want 2", ctl.routings)
	}
}

func TestApplyXrayPendingStopsOnError(t *testing.T) {
	desired := &xrayDesiredRuntime{
		Inbounds:  map[string]json.RawMessage{"inbound-443": json.RawMessage(`{"tag":"inbound-443"}`)},
		Outbounds: map[string]json.RawMessage{},
	}
	ops := []agentrt.XrayPendingOp{
		{Kind: agentrt.XrayPendingInbound, Tag: "inbound-443"},
		{Kind: agentrt.XrayPendingOutbound, Tag: "proxy"},
	}
	ctl := &fakeControl{failOn: "addInbound"}
	if err := applyXrayPending(context.Background(), 4, ops, desired, ctl); err == nil {
		t.Fatal("expected error")
	}
	if len(ctl.removeOutbound) != 0 {
		t.Errorf("should stop before later ops, got %v", ctl.removeOutbound)
	}
}

func TestBuildDesiredRuntimeIndexesByTag(t *testing.T) {
	raw := []byte(`{
		"inbounds": [
			{"tag": "api", "protocol": "dokodemo-door", "port": 10085},
			{"tag": "inbound-vless-31164", "protocol": "vless", "port": 31164},
			{"protocol": "vless", "port": 1}
		],
		"outbounds": [
			{"tag": "outbound-vmess-oquojq", "protocol": "vmess"},
			{"tag": "direct", "protocol": "freedom"}
		],
		"routing": {"rules": [{"type": "field", "inboundTag": ["api"], "outboundTag": "api"}]}
	}`)

	desired, err := buildDesiredRuntime(raw, 2)
	if err != nil {
		t.Fatalf("buildDesiredRuntime: %v", err)
	}
	if len(desired.Inbounds) != 2 {
		t.Errorf("inbounds = %v, want 2 (untagged skipped)", len(desired.Inbounds))
	}
	if _, ok := desired.Inbounds["inbound-vless-31164"]; !ok {
		t.Errorf("missing inbound tag, got %v", desired.Inbounds)
	}
	if len(desired.Outbounds) != 2 {
		t.Errorf("outbounds = %d, want 2", len(desired.Outbounds))
	}
	if tag := jsonTagOf(desired.Outbounds["outbound-vmess-oquojq"]); tag != "outbound-vmess-oquojq" {
		t.Errorf("outbound tag = %q", tag)
	}
	if len(desired.Routing) == 0 {
		t.Error("routing should be kept")
	}

	if _, err := buildDesiredRuntime([]byte("not json"), 2); err == nil {
		t.Error("expected parse error")
	}
}

func TestApplyXrayPendingRemovesUnknownRoutingWithoutConfig(t *testing.T) {
	// 生成配置里没有 routing 段时不应调用 ApplyRouting（否则会清空运行中的路由）
	ctl := &fakeControl{}
	ops := []agentrt.XrayPendingOp{{Kind: agentrt.XrayPendingRouting}}
	if err := applyXrayPending(context.Background(), 2, ops, &xrayDesiredRuntime{}, ctl); err != nil {
		t.Fatalf("applyXrayPending: %v", err)
	}
	if ctl.routings != 0 {
		t.Errorf("routings = %d, want 0", ctl.routings)
	}
}
