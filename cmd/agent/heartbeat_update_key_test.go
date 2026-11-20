package main

import (
	"context"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	agentpb "corefusion/internal/runtime/agent/proto"
)

type fakeRuntimeConfigServer struct {
	agentpb.UnimplementedHeartbeatServiceServer
	pem   string
	calls int32
}

func (f *fakeRuntimeConfigServer) GetRuntimeConfig(context.Context, *agentpb.GetRuntimeConfigRequest) (*agentpb.GetRuntimeConfigResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	return &agentpb.GetRuntimeConfigResponse{UpdatePublicKey: f.pem}, nil
}

func dialBufconn(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// 老节点升级后的自愈链路：连上面板 → 拉公钥 → 回写 config.yaml → 内存生效。
func TestEnsureUpdatePublicKeyWritesConfig(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	fake := &fakeRuntimeConfigServer{pem: testUpdateKeyPEM}
	agentpb.RegisterHeartbeatServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn := dialBufconn(t, lis)
	defer conn.Close()

	path := writeAgentConfig(t, "panel:\n  host: \"h\"\n\nagent:\n  data_dir: \"/tmp/x\"\n")
	app := &AgentApp{cfg: &AgentConfig{}, configPath: path}
	app.ensureUpdatePublicKey(context.Background(), agentpb.NewHeartbeatServiceClient(conn))

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// YAML 块内容整体缩进 4 格，逐行比对去缩进后的 PEM
	var gotLines []string
	for _, ln := range strings.Split(string(raw), "\n") {
		gotLines = append(gotLines, strings.TrimSpace(ln))
	}
	for _, want := range strings.Split(testUpdateKeyPEM, "\n") {
		if !strings.Contains(strings.Join(gotLines, "\n"), strings.TrimSpace(want)) {
			t.Fatalf("config.yaml 缺少公钥行 %q：\n%s", want, raw)
		}
	}
	if strings.TrimSpace(app.cfg.Update.PublicKey) != testUpdateKeyPEM {
		t.Fatal("内存配置未同步，后续 UpdateAgent 仍不会校验签名")
	}

	// 已经配置过的节点不应每次重连都写盘/请求
	app2 := &AgentApp{cfg: &AgentConfig{}, configPath: path}
	app2.cfg.Update.PublicKey = testUpdateKeyPEM
	before := atomic.LoadInt32(&fake.calls)
	app2.ensureUpdatePublicKey(context.Background(), agentpb.NewHeartbeatServiceClient(conn))
	if atomic.LoadInt32(&fake.calls) != before {
		t.Fatal("已配置公钥时不应再请求面板")
	}
}

// 面板没注入签名器（返回空）时不能把空值写进配置，也不能清掉内存状态。
func TestEnsureUpdatePublicKeyIgnoresEmptyResponse(t *testing.T) {
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	agentpb.RegisterHeartbeatServiceServer(srv, &fakeRuntimeConfigServer{})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn := dialBufconn(t, lis)
	defer conn.Close()

	body := "panel:\n  host: \"h\"\n"
	path := writeAgentConfig(t, body)
	app := &AgentApp{cfg: &AgentConfig{}, configPath: path}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	app.ensureUpdatePublicKey(ctx, agentpb.NewHeartbeatServiceClient(conn))

	raw, _ := os.ReadFile(path)
	if string(raw) != body {
		t.Fatalf("空公钥不应改动配置：\n%s", raw)
	}
}
