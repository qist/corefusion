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
