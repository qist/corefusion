package agent

import (
	"strconv"
	"sync"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// NodeOnlineSnapshot 保存某节点的在线订阅人快照与刷新时间。
type NodeOnlineSnapshot struct {
	Users     []*agentpb.OnlineUserEntry
	UpdatedAt int64 // unix 秒
}

// NodeOnlineStore 在内存中按 serverID 缓存在线订阅人快照。
// 节点掉线（超过 TTL 未刷新）的快照会被视为过期，GetFresh 返回 false，
// 避免面板显示僵尸数据。
type NodeOnlineStore struct {
	mu     sync.RWMutex
	data   map[int64]*NodeOnlineSnapshot
	ttlSec int64
	// graceSec：「连续在线」容忍的断开秒数。短连接客户端会频繁上下线
	// （xray 的在线 IP 也有自己的保留窗口），只要在 graceSec 内重新出现，
	// 就视为同一次在线，在线时长继续累加；超过则重新计时。
	graceSec int64
	// sessions: serverID -> 会话键 -> 连续在线状态。
	// 会话键分两级：订阅级（core|s:<subID>）与会话级（订阅键|ip:<ip>）。
	sessions map[int64]map[string]*onlineSession
}

// onlineSession 记录面板侧观测到的一次「连续在线」。
type onlineSession struct {
	since    int64 // 本次连续在线的起点（面板时钟，unix 秒）
	lastSeen int64 // 面板最后一次看到该会话的时间
	// 流量累加：核心只提供「用户级累计计数器」，而且核心重启/流量复位会归零，
	// 直接展示计数器就等于展示「这次核心跑起来之后的流量」（和在线时长对不上）。
	// 这里按连续在线区间做增量累加：up/down 是上一次采样到的计数器值，
	// cumUp/cumDown 是本次连续在线区间内的累计流量。
	up      int64
	down    int64
	cumUp   int64
	cumDown int64
}

// NewNodeOnlineStore 创建在线存储；ttlSec 为快照过期秒数（默认 60s）。
func NewNodeOnlineStore(ttlSec int64) *NodeOnlineStore {
	if ttlSec <= 0 {
		ttlSec = 60
	}
	return &NodeOnlineStore{
		data:     make(map[int64]*NodeOnlineSnapshot),
		ttlSec:   ttlSec,
		graceSec: ttlSec,
		sessions: make(map[int64]map[string]*onlineSession),
	}
}

// Update 刷新某节点的在线快照（同时重置刷新时间）。
//
// 在线时长不能直接用 Agent 上报的 first_seen / connected_since：
// Agent 重启、差分基线被清理、核心计数器归零、以及短连接客户端频繁上下线，
// 都会让这个值反复归零，页面上「在线时长」就永远只有几秒。
// 这里在面板侧按「连续在线」跟踪：同一订阅（或同一 IP 会话）在 graceSec 内
// 重新出现就保留原起点，超过 graceSec 没出现才视为新的一次在线。
//
// 同时归一 last_seen：旧版 Agent 或异常数据里可能为 0 或超前，
// 前端按「现在 - first_seen」或「last_seen - connected_since」算时长会得到
// 50 万小时这种离谱值或负值。
func (s *NodeOnlineStore) Update(serverID int64, users []*agentpb.OnlineUserEntry) {
	now := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	states := s.sessions[serverID]
	if states == nil {
		states = make(map[string]*onlineSession)
		s.sessions[serverID] = states
	}
	seen := make(map[string]struct{}, len(users)*2)

	for _, u := range users {
		if u == nil {
			continue
		}
		if u.LastSeen <= 0 || u.LastSeen > now+60 {
			u.LastSeen = now
		}

		userKey := onlineUserKey(u)
		if userKey != "" {
			u.FirstSeen = touchOnlineSession(states, userKey, now, s.graceSec)
			seen[userKey] = struct{}{}
		} else if u.FirstSeen <= 0 || u.FirstSeen > u.LastSeen {
			// 无法归户的匿名条目：只做基础归一，不参与连续在线跟踪。
			u.FirstSeen = u.LastSeen
		}

		// 流量同样按「连续在线区间」累加，口径与在线时长一致：
		// 短连接重连、计数器复位都不清零，超过 graceSec 没出现才重新开始。
		// 累加结果直接写回上报条目，前端无需改字段。
		if trafficKey := onlineTrafficKey(u); trafficKey != "" {
			up, down := accumulateOnlineTraffic(states, trafficKey, now, s.graceSec,
				u.GetUplinkBytes(), u.GetDownlinkBytes())
			u.UplinkBytes = up
			u.DownlinkBytes = down
			seen[trafficKey] = struct{}{}
		}

		for _, sess := range u.Sessions {
			if sess == nil {
				continue
			}
			if sess.LastSeen <= 0 || sess.LastSeen > now+60 {
				sess.LastSeen = u.LastSeen
			}
			if userKey == "" || sess.GetIp() == "" {
				// 没有 IP 或无法归户：按最后活跃时间兜底，避免 0/负时长。
				if sess.ConnectedSince <= 0 || sess.ConnectedSince > now {
					sess.ConnectedSince = sess.LastSeen
				}
				continue
			}
			sessKey := userKey + "|ip:" + sess.GetIp()
			sess.ConnectedSince = touchOnlineSession(states, sessKey, now, s.graceSec)
			seen[sessKey] = struct{}{}
		}
	}

	// 清理超时未再出现的会话状态，避免长期运行时无限增长。
	for key, st := range states {
		if _, ok := seen[key]; !ok && now-st.lastSeen > s.graceSec {
			delete(states, key)
		}
	}

	s.data[serverID] = &NodeOnlineSnapshot{
		Users:     users,
		UpdatedAt: now,
	}
}

// touchOnlineSession 返回该会话本次「连续在线」的起点（面板时钟）。
// 在 graceSec 内出现过就沿用旧起点，否则以 now 重新开始。
func touchOnlineSession(states map[string]*onlineSession, key string, now, graceSec int64) int64 {
	st := states[key]
	if st == nil || now-st.lastSeen > graceSec {
		states[key] = &onlineSession{since: now, lastSeen: now}
		return now
	}
	st.lastSeen = now
	return st.since
}

// onlineUserKey 生成订阅级的会话键：同一订阅在多个入站各有账号时共用一条，
// xray / sing-box 分开计数。无法归户时返回空串（不参与跟踪）。
func onlineUserKey(u *agentpb.OnlineUserEntry) string {
	core := u.GetCoreType()
	if u.GetSubscriptionId() > 0 {
		return core + "|s:" + strconv.FormatInt(u.GetSubscriptionId(), 10)
	}
	if email := u.GetEmail(); email != "" {
		return core + "|e:" + email
	}
	return ""
}

// onlineTrafficKey 生成流量累加的键。
//
// 不能复用 onlineUserKey：一个订阅在多个入站各有一个账号，每个账号的
// 流量计数器是独立的；用订阅级键会把不同账号的计数器混在一起差分。
// 账号名（AccountEmail，带 _shortTag）是核心里的唯一计数主体，优先用它。
func onlineTrafficKey(u *agentpb.OnlineUserEntry) string {
	core := u.GetCoreType()
	account := u.GetAccountEmail()
	if account == "" {
		account = u.GetEmail()
	}
	if account == "" {
		if u.GetSubscriptionId() <= 0 {
			return ""
		}
		account = "s:" + strconv.FormatInt(u.GetSubscriptionId(), 10)
	}
	return core + "|t:" + account
}

// accumulateOnlineTraffic 把核心计数器换算成「本次连续在线区间内的累计流量」。
//
//   - 首次出现：以当前计数器为基线，累计从 0 开始（之前那些流量属于更早的在线）；
//   - graceSec 内再次出现：累加本次增量（短连接重连不清零）；
//   - 计数器回退（核心重启 / 流量复位）：按新计数器从头累加，不丢当前值；
//   - 超过 graceSec 没出现：视为新的一次在线，重新取基线。
//
// 返回值写回上报条目的 uplink_bytes / downlink_bytes，前端可直接展示。
func accumulateOnlineTraffic(states map[string]*onlineSession, key string, now, graceSec, up, down int64) (int64, int64) {
	up = max(up, 0)
	down = max(down, 0)

	st := states[key]
	if st == nil || now-st.lastSeen > graceSec {
		states[key] = &onlineSession{since: now, lastSeen: now, up: up, down: down}
		return 0, 0
	}
	st.lastSeen = now
	st.cumUp += counterDelta(up, st.up)
	st.cumDown += counterDelta(down, st.down)
	st.up, st.down = up, down
	return st.cumUp, st.cumDown
}

// counterDelta 计算单调计数器的增量；回退说明计数器被复位，从 0 重新算。
func counterDelta(cur, prev int64) int64 {
	if cur >= prev {
		return cur - prev
	}
	return cur
}

// GetFresh 返回未过期的在线快照；已过期视为无数据。
func (s *NodeOnlineStore) GetFresh(serverID int64) (*NodeOnlineSnapshot, bool) {
	s.mu.RLock()
	snap, ok := s.data[serverID]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().Unix()-snap.UpdatedAt > s.ttlSec {
		return nil, false
	}
	return snap, true
}

// Remove 删除某节点的快照（节点离线时调用）。
func (s *NodeOnlineStore) Remove(serverID int64) {
	s.mu.Lock()
	delete(s.data, serverID)
	delete(s.sessions, serverID)
	s.mu.Unlock()
}

// NodeOnlineSummary 是单节点的实时在线汇总。
type NodeOnlineSummary struct {
	OnlineSubscriptions int64 `json:"online_subscriptions"`
	OnlineDevices       int64 `json:"online_devices"`
}

// GetAllSummary 返回所有未过期节点的在线汇总（serverID → 汇总）。
// 用于节点列表徽章与 Dashboard 总卡，避免前端逐节点轮询（N+1）。
func (s *NodeOnlineStore) GetAllSummary() map[int64]*NodeOnlineSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[int64]*NodeOnlineSummary)
	now := time.Now().Unix()
	for serverID, snap := range s.data {
		if now-snap.UpdatedAt > s.ttlSec {
			continue
		}
		// 同一订阅会在多个入站各有账号（email 带不同 _tag），按订阅 ID 去重；
		// 解析不出订阅 ID 的（旧版 Agent）退回 base email 去重。
		// 设备数同理按 IP 去重（同 IP 从两个入站接入只算一台设备）。
		subs := make(map[string]struct{}, len(snap.Users))
		devs := make(map[string]struct{})
		for _, u := range snap.Users {
			if u == nil {
				continue
			}
			if u.GetSubscriptionId() > 0 {
				subs["s:"+strconv.FormatInt(u.GetSubscriptionId(), 10)] = struct{}{}
			} else if u.GetEmail() != "" {
				subs["e:"+u.GetEmail()] = struct{}{}
			}
			for _, sess := range u.GetSessions() {
				if sess != nil && sess.GetIp() != "" {
					devs[sess.GetIp()] = struct{}{}
				}
			}
		}
		result[serverID] = &NodeOnlineSummary{
			OnlineSubscriptions: int64(len(subs)),
			OnlineDevices:       int64(len(devs)),
		}
	}
	return result
}

// GetAllDetail 返回所有未过期节点的完整在线快照（serverID → 快照）。
// 用于全局在线用户页，把所有节点的在线订阅人合并成一张表。
func (s *NodeOnlineStore) GetAllDetail() map[int64]*NodeOnlineSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[int64]*NodeOnlineSnapshot)
	now := time.Now().Unix()
	for serverID, snap := range s.data {
		if now-snap.UpdatedAt > s.ttlSec {
			continue
		}
		result[serverID] = snap
	}
	return result
}
