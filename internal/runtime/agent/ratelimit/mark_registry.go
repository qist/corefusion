package ratelimit

import (
	"sync"
	"time"
)

// MarkRegistry 维护 IP → Mark → 订阅 的映射关系。
// 它是限速系统的核心数据结构：nft 打标记需要知道"IP 属于哪个订阅"，
// tc 分流需要知道"这个订阅限速多少"。
//
// Mark 分配策略：每个订阅分配一个唯一 mark，范围 0x1001 ~ 0xFFFF
// （最多 0xF000 = 61440 个并发订阅）。
//
// mark = 0x1000 + (subscriptionID % 0xF000)
//
// 线程安全：所有方法均可并发调用。
type MarkRegistry struct {
	mu        sync.RWMutex
	ipToMark  map[string]*MarkEntry  // key: "1.2.3.4" → mark entry
	subToMark map[int64]uint32       // key: subscriptionID → mark value
	subToRate map[int64]*RateConfig  // key: subscriptionID → 限速配置
}

// MarkEntry 记录一个 IP 的完整归属信息。
type MarkEntry struct {
	IP             string
	SubscriptionID int64
	Mark           uint32
	FirstSeen      time.Time
	LastSeen       time.Time
}

// RateConfig 记录一个订阅的限速配置。
type RateConfig struct {
	MaxDownMbps int32 // 下载限速 Mbps（0=不限）
	MaxUpMbps   int32 // 上传限速 Mbps（0=不限）
}

// NewMarkRegistry 创建一个新的 MarkRegistry。
func NewMarkRegistry() *MarkRegistry {
	return &MarkRegistry{
		ipToMark:  make(map[string]*MarkEntry),
		subToMark: make(map[int64]uint32),
		subToRate: make(map[int64]*RateConfig),
	}
}

// GetOrCreateMark 为订阅分配或复用 mark 值。
// mark 范围：0x1001 ~ 0xFFFF
func (r *MarkRegistry) GetOrCreateMark(subID int64) uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mark, ok := r.subToMark[subID]; ok {
		return mark
	}
	mark := uint32(0x1000 + (uint64(subID) % 0xF000))
	// 防止 mark 为 0x1000（保留给默认 class）
	if mark == 0x1000 {
		mark = 0x1001
	}
	r.subToMark[subID] = mark
	return mark
}

// AssociateIP 将 IP 关联到订阅的 mark，并返回 mark 值。
// 如果该订阅已有限速配置，会一并记录。
// 这是所有触发源（Panel 推送、Hysteria 2 回调、Stats API 等）的统一入口。
func (r *MarkRegistry) AssociateIP(ip string, subID int64, rate *RateConfig) uint32 {
	mark := r.GetOrCreateMark(subID)

	r.mu.Lock()
	defer r.mu.Unlock()

	// 更新或创建 IP 映射
	if entry, exists := r.ipToMark[ip]; exists {
		entry.LastSeen = time.Now()
		// 如果 IP 从一个订阅迁移到另一个（如 IP 重分配）
		if entry.SubscriptionID != subID {
			entry.SubscriptionID = subID
			entry.Mark = mark
		}
	} else {
		r.ipToMark[ip] = &MarkEntry{
			IP:             ip,
			SubscriptionID: subID,
			Mark:           mark,
			FirstSeen:      time.Now(),
			LastSeen:       time.Now(),
		}
	}

	// 更新限速配置
	if rate != nil {
		r.subToRate[subID] = rate
	}

	return mark
}

// RemoveIP 移除一个 IP 的映射（用户下线时调用）。
func (r *MarkRegistry) RemoveIP(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.ipToMark, ip)
}

// TouchIP updates the last-seen timestamp for an IP if it exists in the registry.
// This is used by the online-stats poller to keep active IPs from being cleaned up.
func (r *MarkRegistry) TouchIP(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.ipToMark[ip]; ok {
		entry.LastSeen = time.Now()
	}
}

// RemoveSubscription 移除一个订阅的所有映射（用户被封断时调用）。
// 返回被移除的 IP 列表和 mark 值。
func (r *MarkRegistry) RemoveSubscription(subID int64) (ips []string, mark uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	mark, hasMark := r.subToMark[subID]
	if !hasMark {
		return nil, 0
	}

	// 收集该订阅的所有 IP
	for ip, entry := range r.ipToMark {
		if entry.SubscriptionID == subID {
			ips = append(ips, ip)
		}
	}

	// 删除 IP 映射
	for _, ip := range ips {
		delete(r.ipToMark, ip)
	}

	// 删除订阅映射和限速配置
	delete(r.subToMark, subID)
	delete(r.subToRate, subID)

	return ips, mark
}

// LookupByIP 根据 IP 查询 mark 和订阅信息。
// 如果 IP 不在注册表中，返回 0, nil。
func (r *MarkRegistry) LookupByIP(ip string) (uint32, *MarkEntry) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.ipToMark[ip]
	if !ok {
		return 0, nil
	}
	return entry.Mark, entry
}

// GetRateConfig 获取订阅的限速配置。
func (r *MarkRegistry) GetRateConfig(subID int64) *RateConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.subToRate[subID]
}

// SetRateConfig 设置订阅的限速配置。
func (r *MarkRegistry) SetRateConfig(subID int64, rate *RateConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subToRate[subID] = rate
}

// GetAllEntries 返回所有 IP 映射条目（用于状态查询和规则重建）。
func (r *MarkRegistry) GetAllEntries() []*MarkEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]*MarkEntry, 0, len(r.ipToMark))
	for _, entry := range r.ipToMark {
		entries = append(entries, entry)
	}
	return entries
}

// GetSubscriptionIPs 返回指定订阅的所有 IP。
func (r *MarkRegistry) GetSubscriptionIPs(subID int64) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var ips []string
	for ip, entry := range r.ipToMark {
		if entry.SubscriptionID == subID {
			ips = append(ips, ip)
		}
	}
	return ips
}

// CleanupStale 清理超过 ttl 未活跃的条目。
// ttl=0 表示不清理。
// 返回被清理的 (ip, mark) 列表。
func (r *MarkRegistry) CleanupStale(ttl time.Duration) []struct {
	IP   string
	Mark uint32
} {
	if ttl <= 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	var removed []struct {
		IP   string
		Mark uint32
	}

	for ip, entry := range r.ipToMark {
		if entry.LastSeen.Before(cutoff) {
			removed = append(removed, struct {
				IP   string
				Mark uint32
			}{ip, entry.Mark})
			delete(r.ipToMark, ip)
		}
	}

	// 清理无 IP 引用的订阅映射
	for subID := range r.subToMark {
		hasIPs := false
		for _, entry := range r.ipToMark {
			if entry.SubscriptionID == subID {
				hasIPs = true
				break
			}
		}
		if !hasIPs {
			delete(r.subToMark, subID)
			delete(r.subToRate, subID)
		}
	}

	return removed
}

// Size 返回当前注册的 IP 数量和订阅数量。
func (r *MarkRegistry) Size() (ipCount, subCount int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.ipToMark), len(r.subToMark)
}
