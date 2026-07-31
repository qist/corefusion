package agent

import (
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
func (s *NodeOnlineStore) Update(serverID int64, users []*agentpb.OnlineUserEntry) {
	s.mu.Lock()
	s.data[serverID] = &NodeOnlineSnapshot{
		Users:     users,
		UpdatedAt: time.Now().Unix(),
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
		var subs, devs int64
		for _, u := range snap.Users {
			subs++
			devs += int64(len(u.Sessions))
		}
		result[serverID] = &NodeOnlineSummary{
			OnlineSubscriptions: subs,
			OnlineDevices:       devs,
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
