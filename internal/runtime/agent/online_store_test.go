package agent

import (
	"testing"
	"time"

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

// 会话级时长 = last_seen - connected_since：缺省 0 会得到 498484 小时，
// 比 last_seen 晚则是负时长。快照入库时必须统一修正。
// 站点级「在线时长」必须跨短连接保持：Agent 每次上报的 first_seen 会因
// 差分基线清理 / Agent 重启而重置，面板要按「连续在线」自己记起点。
func TestUpdateKeepsOnlineSinceAcrossShortGaps(t *testing.T) {
	s := NewNodeOnlineStore(60)
	entry := func(firstSeen int64) []*agentpb.OnlineUserEntry {
		return []*agentpb.OnlineUserEntry{{
			SubscriptionId: 5, Email: "a@example.com", CoreType: "xray",
			FirstSeen: firstSeen, LastSeen: time.Now().Unix(),
			Sessions: []*agentpb.OnlineSession{{Ip: "1.2.3.4", ConnectedSince: firstSeen}},
		}}
	}

	s.Update(1, entry(time.Now().Unix()))
	snap, _ := s.GetFresh(1)
	first := snap.Users[0].FirstSeen
	sessFirst := snap.Users[0].Sessions[0].ConnectedSince
	if first <= 0 {
		t.Fatalf("first_seen 未设置: %d", first)
	}

	// 下一轮上报 Agent 把 first_seen 重置成「现在」（模拟基线重建），面板应保留原起点。
	s.Update(1, entry(time.Now().Unix()))
	snap, _ = s.GetFresh(1)
	if got := snap.Users[0].FirstSeen; got != first {
		t.Fatalf("短间隔重连不应重置订阅级起点: got %d want %d", got, first)
	}
	if got := snap.Users[0].Sessions[0].ConnectedSince; got != sessFirst {
		t.Fatalf("短间隔重连不应重置会话级起点: got %d want %d", got, sessFirst)
	}
}

// 超过 graceSec 没有上报则视为新的一次在线，起点重新计时（避免把「昨天的在线」算进来）。
func TestUpdateResetsOnlineSinceAfterLongGap(t *testing.T) {
	s := NewNodeOnlineStore(60)
	users := []*agentpb.OnlineUserEntry{{
		SubscriptionId: 5, Email: "a@example.com", CoreType: "xray",
		LastSeen: time.Now().Unix(),
		Sessions: []*agentpb.OnlineSession{{Ip: "1.2.3.4"}},
	}}
	s.Update(1, users)

	// 模拟 10 分钟没收到该节点的上报。
	s.mu.Lock()
	for _, st := range s.sessions[1] {
		st.since = time.Now().Unix() - 600
		st.lastSeen = time.Now().Unix() - 600
	}
	s.mu.Unlock()

	s.Update(1, []*agentpb.OnlineUserEntry{{
		SubscriptionId: 5, Email: "a@example.com", CoreType: "xray",
		LastSeen: time.Now().Unix(),
		Sessions: []*agentpb.OnlineSession{{Ip: "1.2.3.4"}},
	}})
	snap, _ := s.GetFresh(1)
	if got := snap.Users[0].FirstSeen; time.Now().Unix()-got > 5 {
		t.Fatalf("超过 grace 后应重新计时，实际 first_seen=%d", got)
	}
}

// 同一订阅换 IP：订阅级时长继续累加，新 IP 的会话时长从 0 开始。
func TestUpdateTracksSessionsPerIP(t *testing.T) {
	s := NewNodeOnlineStore(60)
	now := time.Now().Unix()
	s.Update(1, []*agentpb.OnlineUserEntry{{
		SubscriptionId: 5, Email: "a@example.com", CoreType: "xray", LastSeen: now,
		Sessions: []*agentpb.OnlineSession{{Ip: "1.2.3.4"}},
	}})
	first := s.mustGet(1).Users[0].FirstSeen

	s.Update(1, []*agentpb.OnlineUserEntry{{
		SubscriptionId: 5, Email: "a@example.com", CoreType: "xray", LastSeen: now,
		Sessions: []*agentpb.OnlineSession{{Ip: "5.6.7.8"}},
	}})
	u := s.mustGet(1).Users[0]
	if u.FirstSeen != first {
		t.Fatalf("订阅级起点应保持: got %d want %d", u.FirstSeen, first)
	}
	if got := u.Sessions[0].ConnectedSince; time.Now().Unix()-got > 5 {
		t.Fatalf("新 IP 的会话时长应从本次开始，实际 connected_since=%d", got)
	}
}

func (s *NodeOnlineStore) mustGet(serverID int64) *NodeOnlineSnapshot {
	snap, ok := s.GetFresh(serverID)
	if !ok || snap == nil {
		panic("missing snapshot")
	}
	return snap
}

// 会话级时长 = now - connected_since：缺省 0 会得到 498484 小时，
// 比采样时间还晚则是负时长。快照入库时必须统一修正。
func TestUpdateNormalizesSessionConnectedSince(t *testing.T) {
	s := NewNodeOnlineStore(60)
	now := time.Now().Unix()
	s.Update(1, []*agentpb.OnlineUserEntry{{
		SubscriptionId: 1, Email: "a@example.com", LastSeen: now,
		Sessions: []*agentpb.OnlineSession{
			{Ip: "1.1.1.1", LastSeen: now},                           // 缺 ConnectedSince
			{Ip: "2.2.2.2", LastSeen: now, ConnectedSince: now + 30}, // 倒挂
		},
	}})
	snap, ok := s.GetFresh(1)
	if !ok || snap == nil || len(snap.Users) != 1 {
		t.Fatalf("snapshot missing: %+v", snap)
	}
	for _, sess := range snap.Users[0].Sessions {
		// 起点由面板侧记录（面板时钟），只要求是「最近一次采样」附近且非 0/未来值。
		if sess.ConnectedSince <= 0 || sess.ConnectedSince > now+5 {
			t.Errorf("session %s 的 connected_since=%d 未归一（now=%d）", sess.Ip, sess.ConnectedSince, now)
		}
	}
}
