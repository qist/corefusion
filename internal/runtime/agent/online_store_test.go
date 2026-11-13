package agent

import (
	"testing"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// ONLINE_USERS 上报除了刷新快照，还必须触发注入的钩子，
// 面板靠它把「核心观测 IP」驱动到限速（替代订阅 HTTP IP）。
func TestUpdateOnlineUsersInvokesHook(t *testing.T) {
	m := &NodeServerManager{onlineStore: NewNodeOnlineStore(60)}

	gotServer := make(chan int64, 1)
	m.SetOnlineUsersHook(func(serverID int64, users []*agentpb.OnlineUserEntry) {
		gotServer <- serverID
	})

	users := []*agentpb.OnlineUserEntry{{
		SubscriptionId: 3,
		Email:          "a@example.com",
		Sessions:       []*agentpb.OnlineSession{{Ip: "1.2.3.4"}},
	}}
	m.UpdateOnlineUsers(9, users)

	if sid := <-gotServer; sid != 9 {
		t.Fatalf("hook serverID = %d, want 9", sid)
	}
	if got, _, ok := m.GetNodeOnlineUsers(9); !ok || len(got) != 1 || got[0].GetEmail() != "a@example.com" {
		t.Fatalf("online snapshot not refreshed: %+v ok=%v", got, ok)
	}
}

// 同一订阅在多个入站各有一个账号（email 带不同 _tag）时，「在线订阅人」与
// 「在线设备」必须去重：否则一个客户端挂 3 个入站会显示 3 个订阅人。
func TestGetAllSummaryDedupesSubscriptionsAndDevices(t *testing.T) {
	s := NewNodeOnlineStore(60)
	s.Update(1, []*agentpb.OnlineUserEntry{
		{SubscriptionId: 7, Email: "a@example.com", Sessions: []*agentpb.OnlineSession{{Ip: "1.2.3.4"}, {Ip: "5.6.7.8"}}},
		{SubscriptionId: 7, Email: "a@example.com", Sessions: []*agentpb.OnlineSession{{Ip: "1.2.3.4"}}},
		{SubscriptionId: 0, Email: "b@example.com", Sessions: []*agentpb.OnlineSession{{Ip: "9.9.9.9"}}},
	})
	got := s.GetAllSummary()[1]
	if got == nil {
		t.Fatal("missing summary for server 1")
	}
	if got.OnlineSubscriptions != 2 {
		t.Fatalf("在线订阅人应为 2（订阅 7 + 未归户的 b），实际 %d", got.OnlineSubscriptions)
	}
	if got.OnlineDevices != 3 {
		t.Fatalf("在线设备应为 3（去重后的 IP），实际 %d", got.OnlineDevices)
	}
}
