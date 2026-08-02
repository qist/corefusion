package coredata

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// fakeRemote 记录最后一次调用并返回预设响应。
type fakeRemote struct {
	lastServerID int64
	lastService  string
	lastMethod   string
	resp         *agentpb.XrayAPIResponse
	err          error
}

func (f *fakeRemote) XrayAPIOnServer(_ context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	f.lastServerID = serverID
	f.lastService = req.Service
	f.lastMethod = req.Method
	return f.resp, f.err
}

func TestControlProvider_RemoteCall(t *testing.T) {
	remote := &fakeRemote{
		resp: &agentpb.XrayAPIResponse{Success: true, Response: []byte(`{"ok":true}`)},
	}
	p := NewControlProvider(ControlOptions{
		IsLocal: func(int64) bool { return false },
		Remote:  remote,
	})

	got, err := p.RemoteCall(context.Background(), 2, "proxyman", "addInbound", map[string]any{"tag": "vmess"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]bool
	if err := json.Unmarshal(got, &m); err != nil || !m["ok"] {
		t.Fatalf("unexpected response: %s", got)
	}
	if remote.lastServerID != 2 || remote.lastService != "proxyman" || remote.lastMethod != "addInbound" {
		t.Fatalf("route mismatch: %+v", remote)
	}
}

func TestControlProvider_RemoteCallError(t *testing.T) {
	remote := &fakeRemote{err: errors.New("boom")}
	p := NewControlProvider(ControlOptions{
		IsLocal: func(int64) bool { return false },
		Remote:  remote,
	})
	if _, err := p.RemoteCall(context.Background(), 2, "proxyman", "addInbound", nil); err == nil {
		t.Fatal("expected error from remote failure")
	}
}

func TestControlProvider_LocalConnRejectsRemote(t *testing.T) {
	p := NewControlProvider(ControlOptions{
		IsLocal: func(int64) bool { return false },
		AddrResolver: func(_ context.Context, _ int64) (string, error) {
			return "127.0.0.1:10085", nil
		},
	})
	if _, err := p.LocalConn(context.Background(), 2); err == nil {
		t.Fatal("expected error calling LocalConn on a remote server")
	}
}

func TestControlProvider_LocalConnNoAddr(t *testing.T) {
	p := NewControlProvider(ControlOptions{
		IsLocal:     func(int64) bool { return true },
		AddrResolver: func(_ context.Context, _ int64) (string, error) {
			return "", nil
		},
	})
	if _, err := p.LocalConn(context.Background(), 1); err == nil {
		t.Fatal("expected error when addr is empty")
	}
}
