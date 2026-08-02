package coredata

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"
)

func TestParseStatsResponse(t *testing.T) {
	resp := &statsService.QueryStatsResponse{
		Stat: []*statsService.Stat{
			{Name: "user>>>alice@example.com>>>traffic>>>uplink", Value: 100},
			{Name: "user>>>alice@example.com>>>traffic>>>downlink", Value: 240},
			{Name: "user>>>bob@example.com>>>traffic>>>uplink", Value: 10},
			{Name: "user>>>bob@example.com>>>traffic>>>downlink", Value: 20},
			{Name: "inbound>>>grpc>>>traffic>>>uplink", Value: 5},
			{Name: "inbound>>>grpc>>>traffic>>>downlink", Value: 7},
			{Name: "outbound>>>freedom>>>traffic>>>uplink", Value: 1}, // 非 inbound>>> 前缀，忽略
		},
	}

	sr, err := ParseStatsResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sr.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(sr.Users))
	}
	if len(sr.Inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %d", len(sr.Inbounds))
	}

	// 校验 alice 累计值
	var alice *UserTraffic
	for i := range sr.Users {
		if sr.Users[i].Email == "alice@example.com" {
			alice = &sr.Users[i]
		}
	}
	if alice == nil {
		t.Fatal("alice not found")
	}
	if alice.Uplink != 100 || alice.Downlink != 240 || alice.Total != 340 {
		t.Fatalf("alice traffic wrong: %+v", alice)
	}

	if sr.Inbounds[0].Tag != "grpc" || sr.Inbounds[0].Uplink != 5 || sr.Inbounds[0].Downlink != 7 || sr.Inbounds[0].Total != 12 {
		t.Fatalf("inbound traffic wrong: %+v", sr.Inbounds[0])
	}
}

// TestWaitForReady_Timeout 验证 NewClient 迁移后 fail-safe 语义保留：
// 连接一个确定不可达的地址时，应在 ctx 超时（5s）内返回 error，而非永久阻塞。
func TestWaitForReady_Timeout(t *testing.T) {
	conn, err := grpc.NewClient("127.0.0.1:1", // 端口 1 通常无监听，触发拨号失败
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- WaitForReady(ctx, conn)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error for unreachable address, got nil")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("WaitForReady blocked beyond timeout, fail-safe semantics broken")
	}
}
