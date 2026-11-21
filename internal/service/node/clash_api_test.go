package node

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentsvc "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

type fakeOnlineProvider struct {
	connErr error
}

func (f *fakeOnlineProvider) GetNodeOnlineUsers(int64) ([]*agentpb.OnlineUserEntry, int64, bool) {
	return nil, 0, false
}

func (f *fakeOnlineProvider) GetAllOnlineSummary() map[int64]*agentsvc.NodeOnlineSummary {
	return nil
}

func (f *fakeOnlineProvider) GetAllOnlineUsers() map[int64]*agentsvc.NodeOnlineSnapshot {
	return nil
}

func (f *fakeOnlineProvider) GetOnlineTotals() (int64, int64) { return 0, 0 }

func (f *fakeOnlineProvider) FetchSingboxConnectionsOnServer(context.Context, int64) ([]byte, string, error) {
	return nil, "", f.connErr
}

// 未开启 clash_api 时必须直接返回「关闭」，不能回退默认 9090 去连：
// 否则用户关闭后页面一直显示 connection refused，把关闭误报成故障。
func TestGetSingboxConnectionsDisabledSkipsProbe(t *testing.T) {
	svc := &Service{repo: &repository.NodeRepository{}, clashAPIEnabledFn: func() bool { return false }}
	res := svc.GetSingboxConnections(context.Background(), 1, SingboxConnectionFilter{})
	if !res.Unsupported || res.Reason != "clash_api_disabled" {
		t.Fatalf("unsupported=%v reason=%q, want clash_api_disabled", res.Unsupported, res.Reason)
	}
	if len(res.Items) != 0 {
		t.Fatalf("未开启时不应返回连接，实际 %d 条", len(res.Items))
	}
}

// 开启时按运行中配置的地址读取（这里用本地测试服务模拟 /connections）。
func TestGetSingboxConnectionsEnabledReadsConfiguredAddr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"downloadTotal":10,"uploadTotal":5,"connections":[{
			"id":"c1","upload":3,"download":4,"start":"2026-11-21T10:00:00Z",
			"chains":["a","b"],"rule":"final",
			"metadata":{"network":"tcp","type":"anytls/inbound-anytls-49146","sourceIP":"1.2.3.4","sourcePort":"12345","destinationIP":"8.8.8.8","destinationPort":"443","host":"example.com"}
		}]}`))
	}))
	defer srv.Close()

	svc := &Service{
		repo:              &repository.NodeRepository{},
		clashAPIEnabledFn: func() bool { return true },
		clashAPIAddrFn:    func() string { return strings.TrimPrefix(srv.URL, "http://") },
	}
	res := svc.GetSingboxConnections(context.Background(), 1, SingboxConnectionFilter{})
	if res.Unsupported {
		t.Fatalf("开启时不应返回 unsupported: %s", res.Reason)
	}
	if res.Total != 1 || len(res.Items) != 1 {
		t.Fatalf("total=%d items=%d, want 1", res.Total, len(res.Items))
	}
	got := res.Items[0]
	if got.SourceIP != "1.2.3.4" || got.InboundTag != "inbound-anytls-49146" || got.Destination != "example.com:443" {
		t.Fatalf("unexpected connection: %+v", got)
	}
	if res.UploadTotal != 5 || res.DownloadTotal != 10 {
		t.Fatalf("totals = %d/%d, want 5/10", res.UploadTotal, res.DownloadTotal)
	}
}

// 远端 Agent 报告「未开启」时保持关闭语义，而不是拼成不可用错误。
func TestGetSingboxConnectionsRemoteDisabled(t *testing.T) {
	svc := &Service{
		repo: &repository.NodeRepository{},
		nodeOnline: &fakeOnlineProvider{
			connErr: errors.New("clash_api_disabled: 该节点未启用 sing-box clash_api"),
		},
	}
	res := svc.GetSingboxConnections(context.Background(), 4, SingboxConnectionFilter{})
	if !res.Unsupported || res.Reason != "clash_api_disabled" {
		t.Fatalf("unsupported=%v reason=%q, want clash_api_disabled", res.Unsupported, res.Reason)
	}
}
