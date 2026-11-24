package agent

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeCoreConfigSyncer 记录按核心下发的调用。
type fakeCoreConfigSyncer struct {
	mu    sync.Mutex
	calls []string
	done  chan struct{}
}

func (f *fakeCoreConfigSyncer) SyncServerCore(_ context.Context, _ int64, coreType string) error {
	f.mu.Lock()
	f.calls = append(f.calls, coreType)
	f.mu.Unlock()
	select {
	case f.done <- struct{}{}:
	default:
	}
	return nil
}

func (f *fakeCoreConfigSyncer) waitCalls(t *testing.T, n int) []string {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		f.mu.Lock()
		calls := append([]string(nil), f.calls...)
		f.mu.Unlock()
		if len(calls) >= n {
			return calls
		}
		select {
		case <-f.done:
		case <-deadline:
			t.Fatalf("等待 %d 次按核心下发超时，实际 %v", n, calls)
		}
	}
}

// SyncData 只带一个核心的 config_content；其余核心必须按核心补推，
// 否则 sing-box 的入站在主备/镜像节点上永远不生效。
func TestSyncSecondaryCoresPushesNonPrimaryCore(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	fake := &fakeCoreConfigSyncer{done: make(chan struct{}, 4)}
	m.SetCoreConfigSyncer(fake)

	m.syncSecondaryCores(13, "xray")

	calls := fake.waitCalls(t, 1)
	if len(calls) != 1 || calls[0] != "singbox" {
		t.Fatalf("应只补推 singbox，实际 %v", calls)
	}
}

// 主配置核心是 singbox 时就补推 xray（不重复推同一个核心）。
func TestSyncSecondaryCoresSkipsCurrentCore(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	fake := &fakeCoreConfigSyncer{done: make(chan struct{}, 4)}
	m.SetCoreConfigSyncer(fake)

	m.syncSecondaryCores(13, "singbox")

	calls := fake.waitCalls(t, 1)
	if len(calls) != 1 || calls[0] != "xray" {
		t.Fatalf("应只补推 xray，实际 %v", calls)
	}
}

// 本地节点（server 1）不跑 SyncData，也不应触发补推。
func TestSyncSecondaryCoresSkipsLocalServer(t *testing.T) {
	m := &NodeServerManager{localServerID: 1}
	fake := &fakeCoreConfigSyncer{done: make(chan struct{}, 4)}
	m.SetCoreConfigSyncer(fake)

	m.syncSecondaryCores(1, "xray")

	time.Sleep(80 * time.Millisecond)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.calls) != 0 {
		t.Fatalf("本地节点不应补推，实际 %v", fake.calls)
	}
}
