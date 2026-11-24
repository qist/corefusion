package healthcheck

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
)

type fakeStatusChecker struct{ online bool }

func (f fakeStatusChecker) IsOnline(int64) bool { return f.online }

func (f fakeStatusChecker) GetServerStatus(context.Context, int64) (*agentpb.GetStatusResponse, error) {
	return &agentpb.GetStatusResponse{}, nil
}

// 面板重启后内存连接表是空的，但节点心跳可能就在十几秒前：不能在宽限期内判离线，
// 否则 Dashboard 会在面板重启瞬间把所有节点闪成"离线"。
func TestDetermineStatusKeepsRecentHeartbeatOnline(t *testing.T) {
	c := &MultiServerHealthChecker{serverMgr: fakeStatusChecker{online: false}}

	recent := time.Now().Add(-10 * time.Second)
	srv := &entity.NodeServer{ID: 12, AgentStatus: entity.AgentStatusOnline, LastHeartbeatAt: &recent}
	if got := c.determineStatus(context.Background(), srv); got != entity.AgentStatusOnline {
		t.Fatalf("心跳 10s 前（宽限期内）应沿用 online，实际 %s", got)
	}

	degraded := &entity.NodeServer{ID: 12, AgentStatus: entity.AgentStatusDegraded, LastHeartbeatAt: &recent}
	if got := c.determineStatus(context.Background(), degraded); got != entity.AgentStatusDegraded {
		t.Fatalf("宽限期内应沿用库里的 degraded，实际 %s", got)
	}

	old := time.Now().Add(-5 * time.Minute)
	stale := &entity.NodeServer{ID: 12, AgentStatus: entity.AgentStatusOnline, LastHeartbeatAt: &old}
	if got := c.determineStatus(context.Background(), stale); got != entity.AgentStatusOffline {
		t.Fatalf("心跳超出宽限期应判 offline，实际 %s", got)
	}
}

// last_heartbeat_age_seconds 必须是秒：time.Duration 直接 JSON 化是纳秒，
// 前端按秒显示会变成 “10892651586s”。
func TestServerHealthInfoHeartbeatAgeIsSeconds(t *testing.T) {
	raw, err := json.Marshal(ServerHealthInfo{ID: 12, LastHeartbeatAge: 11})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"last_heartbeat_age_seconds":11`) {
		t.Fatalf("心跳秒数序列化不正确: %s", raw)
	}
}
