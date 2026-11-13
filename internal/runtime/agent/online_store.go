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
}

// NewNodeOnlineStore 创建在线存储；ttlSec 为快照过期秒数（默认 60s）。
func NewNodeOnlineStore(ttlSec int64) *NodeOnlineStore {
	if ttlSec <= 0 {
		ttlSec = 60
	}
	return &NodeOnlineStore{
		data:   make(map[int64]*NodeOnlineSnapshot),
		ttlSec: ttlSec,
	}
}

// Update 刷新某节点的在线快照（同时重置刷新时间）。
//
// 顺手归一 first_seen/last_seen：旧版 Agent 或异常数据里 first_seen 可能为 0，
// 前端用 last_seen - first_seen 算「连接时长」会得到 50 万小时这种离谱值。
// first_seen 缺失/晚于 last_seen 时按「本次采样」处理。
func (s *NodeOnlineStore) Update(serverID int64, users []*agentpb.OnlineUserEntry) {
	now := time.Now().Unix()
	for _, u := range users {
		if u == nil {
			continue
		}
		if u.LastSeen <= 0 || u.LastSeen > now+60 {
			u.LastSeen = now
		}
		if u.FirstSeen <= 0 || u.FirstSeen > u.LastSeen {
			u.FirstSeen = u.LastSeen
		}
		for _, sess := range u.Sessions {
			if sess != nil && (sess.LastSeen <= 0 || sess.LastSeen > now+60) {
				sess.LastSeen = u.LastSeen
			}
		}
	}
	s.mu.Lock()
	s.data[serverID] = &NodeOnlineSnapshot{
		Users:     users,
		UpdatedAt: now,
	}
	s.mu.Unlock()
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
