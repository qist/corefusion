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
		IsLocal: func(int64) bool { return true },
		AddrResolver: func(_ context.Context, _ int64) (string, error) {
			return "", nil
		},
	})
	if _, err := p.LocalConn(context.Background(), 1); err == nil {
		t.Fatal("expected error when addr is empty")
	}
}

// TestResolveStatsAddr 验证地址解析三级回退：DB xray.stats_addr → api.listen → 默认值。
func TestResolveStatsAddr(t *testing.T) {
	cases := []struct {
		name string
		deps StatsAddrDeps
		want string
	}{
		{
			name: "DB 命中",
			deps: StatsAddrDeps{
				SettingGetter: func(_ context.Context, key string) (string, error) {
					if key == "xray.stats_addr" {
						return "10.0.0.1:12345", nil
					}
					return "", nil
				},
			},
			want: "10.0.0.1:12345",
		},
		{
			name: "DB 缺失回退 api.listen",
			deps: StatsAddrDeps{
				SettingGetter: func(_ context.Context, _ string) (string, error) {
					return "", nil // 模拟 setting 不存在
				},
				APIListenGetter: func(_ context.Context) (string, error) {
					return "0.0.0.0:9999", nil
				},
			},
			want: "0.0.0.0:9999",
		},
		{
			name: "DB 报错回退 api.listen",
			deps: StatsAddrDeps{
				SettingGetter: func(_ context.Context, _ string) (string, error) {
					return "", errors.New("db down")
				},
				APIListenGetter: func(_ context.Context) (string, error) {
					return "127.0.0.1:8080", nil
				},
			},
			want: "127.0.0.1:8080",
		},
		{
			name: "全部缺省回退默认值",
			deps: StatsAddrDeps{
				SettingGetter: func(_ context.Context, _ string) (string, error) {
					return "", nil
				},
				APIListenGetter: func(_ context.Context) (string, error) {
					return "", nil
				},
			},
			want: DefaultStatsAddr,
		},
		{
			name: "无依赖直接默认值",
			deps: StatsAddrDeps{},
			want: DefaultStatsAddr,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveStatsAddr(context.Background(), tc.deps)
			if got != tc.want {
				t.Fatalf("ResolveStatsAddr = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestResolveStatsAddrSingbox 验证 sing-box 模式与 xray 的解析链完全隔离：
// 此前 sing-box 会回退到 "xray.stats_addr" 甚至 xray 默认端口，导致 sing-box 的
// 客户端实际连到 xray 上——因为 xray 用的服务名能应答，所以既不报错也永远拿不到
// sing-box 的流量（静默失效）。
func TestResolveStatsAddrSingbox(t *testing.T) {
	getter := func(_ context.Context, key string) (string, error) {
		if key == "singbox.stats_addr" {
			return "127.0.0.1:19086", nil
		}
		if key == "xray.stats_addr" {
			return "127.0.0.1:19085", nil
		}
		return "", nil
	}

	// 1) 只设了 xray 地址时，sing-box 必须走自己的默认地址
	got := ResolveStatsAddr(context.Background(), StatsAddrDeps{
		SettingGetter: func(_ context.Context, key string) (string, error) {
			if key == "xray.stats_addr" {
				return "127.0.0.1:19085", nil
			}
			return "", nil
		},
		V2RayAPIListenGetter: func(context.Context) (string, error) {
			return "", errors.New("no v2ray_api section")
		},
	})
	if got != DefaultSingboxStatsAddr {
		t.Fatalf("sing-box 地址 = %q, want %q（不能回退到 xray）", got, DefaultSingboxStatsAddr)
	}

	// 2) singbox.stats_addr 设置了就优先用它
	got = ResolveStatsAddr(context.Background(), StatsAddrDeps{
		SettingGetter: getter,
		V2RayAPIListenGetter: func(context.Context) (string, error) {
			return "127.0.0.1:10086", nil
		},
	})
	if got != "127.0.0.1:19086" {
		t.Fatalf("sing-box 地址 = %q, want singbox.stats_addr 的值", got)
	}

	// 3) 没设 singbox.stats_addr 时用 v2ray_api.listen
	got = ResolveStatsAddr(context.Background(), StatsAddrDeps{
		SettingGetter: func(context.Context, string) (string, error) { return "", nil },
		V2RayAPIListenGetter: func(context.Context) (string, error) {
			return "127.0.0.1:10086", nil
		},
	})
	if got != "127.0.0.1:10086" {
		t.Fatalf("sing-box 地址 = %q, want v2ray_api.listen", got)
	}
}

// TestStatsServiceNames 固定两个核心的 gRPC 服务名，避免再次写成同一个。
func TestStatsServiceNames(t *testing.T) {
	if XrayStatsServiceName != "xray.app.stats.command.StatsService" {
		t.Errorf("xray 服务名被改动: %s", XrayStatsServiceName)
	}
	if V2RayStatsServiceName != "v2ray.core.app.stats.command.StatsService" {
		t.Errorf("sing-box 服务名被改动: %s", V2RayStatsServiceName)
	}
	c := NewV2RayClient("127.0.0.1:10086")
	if !c.IsSingbox() || c.serviceName != V2RayStatsServiceName {
		t.Errorf("NewV2RayClient 未使用 v2ray 服务名: %+v", c)
	}
	if x := NewXrayClient(""); x.IsSingbox() || x.serviceName != XrayStatsServiceName || x.Addr() != DefaultStatsAddr {
		t.Errorf("NewXrayClient 行为异常: %+v", x)
	}
	if v := NewV2RayClient(""); v.Addr() != DefaultSingboxStatsAddr {
		t.Errorf("NewV2RayClient 默认地址 = %q, want %q", v.Addr(), DefaultSingboxStatsAddr)
	}
}

// TestProvider_RemoteRawStats 验证远程后端 GetUserTraffic 经 Agent RPC 路由并解析。
func TestProvider_RemoteRawStats(t *testing.T) {
	remote := &fakeRemote{
		resp: &agentpb.XrayAPIResponse{
			Success:  true,
			Response: []byte(`{"stat":[{"name":"user>>>a@vmess>>>traffic>>>downlink","value":123}]}`),
		},
	}
	b := &remoteBackend{caller: remote, serverID: 2}

	users, err := b.GetUserTraffic(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].Email != "a@vmess" || users[0].Downlink != 123 {
		t.Fatalf("unexpected users: %+v", users)
	}
	if remote.lastService != "stats" || remote.lastMethod != "QueryStats" {
		t.Fatalf("route mismatch: service=%s method=%s", remote.lastService, remote.lastMethod)
	}
}

// TestProvider_RemoteResetUserTraffic_Success 覆盖远程 reset 成功路径。
func TestProvider_RemoteResetUserTraffic_Success(t *testing.T) {
	remote := &fakeRemote{resp: &agentpb.XrayAPIResponse{Success: true}}
	b := &remoteBackend{caller: remote, serverID: 2}

	if err := b.ResetUserTraffic(context.Background(), 2, "a@vmess"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remote.lastService != "stats" || remote.lastMethod != "QueryStats" {
		t.Fatalf("route mismatch: service=%s method=%s", remote.lastService, remote.lastMethod)
	}
}

// TestProvider_RemoteResetUserTraffic_Error 覆盖远程 reset 失败返回 error（bugfix 回归）。
func TestProvider_RemoteResetUserTraffic_Error(t *testing.T) {
	remote := &fakeRemote{resp: &agentpb.XrayAPIResponse{Success: false, Error: "boom"}}
	b := &remoteBackend{caller: remote, serverID: 2}

	if err := b.ResetUserTraffic(context.Background(), 2, "a@vmess"); err == nil {
		t.Fatal("expected error when remote reset reports failure")
	}
}
