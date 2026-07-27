package ratelimit

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager 是限速子系统的统一入口，组合 MarkRegistry 和 RateLimitBackend。
// 它提供高层 API，由 LocalAgent 调用。
//
// 使用流程：
//   1. NewManager() 创建
//   2. Init(iface) 初始化基础设施
//   3. AssociateIP() / ApplyLimits() 执行限速操作
//   4. StartCleanupLoop() 启动过期清理
//   5. Close() 清理
type Manager struct {
	registry *MarkRegistry
	backend  RateLimitBackend
	iface    string

	mu          sync.Mutex
	initialized bool

	// 清理循环
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}
	cleanupTTL    time.Duration // 归属有效期，0=不清理

	// 持久化回调：当 IP 映射变更时调用（Agent 侧注入 DB 持久化逻辑）
	persistFunc func(ip string, subID int64, maxDownMbps, maxUpMbps int32)
	removeFunc  func(ip string)
}

// NewManager 创建一个新的限速管理器。
func NewManager(iface string) *Manager {
	return &Manager{
		registry:  NewMarkRegistry(),
		backend:   NewNftTcBackend(),
		iface:     iface,
		cleanupTTL: 0, // 默认不自动清理
	}
}

// WithBackend 注入自定义后端（用于测试或 eBPF 升级）。
func (m *Manager) WithBackend(backend RateLimitBackend) *Manager {
	m.backend = backend
	return m
}

// WithCleanupTTL 设置归属自动清理的 TTL。
// 0 表示不自动清理。设置后需调用 StartCleanupLoop 才生效。
func (m *Manager) WithCleanupTTL(ttl time.Duration) *Manager {
	m.cleanupTTL = ttl
	return m
}

// WithPersistFunc sets the persistence callback.
// When set, AssociateIP will persist the IP mapping to DB,
// and RemoveIP will delete the persisted entry.
func (m *Manager) WithPersistFunc(persist func(ip string, subID int64, maxDownMbps, maxUpMbps int32), remove func(ip string)) *Manager {
	m.persistFunc = persist
	m.removeFunc = remove
	return m
}

// Init 初始化限速基础设施。
func (m *Manager) Init(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	if err := m.backend.Init(ctx, m.iface); err != nil {
		return fmt.Errorf("init rate limit backend: %w", err)
	}

	m.initialized = true
	log.Printf("[RateLimitManager] initialized on iface=%s", m.iface)
	return nil
}

// AssociateIP 将 IP 关联到订阅并应用限速规则。
// 这是 P0 触发源（Panel 推送）的 Agent 侧入口。
func (m *Manager) AssociateIP(ctx context.Context, ip string, subID int64, rate *RateConfig) error {
	if !m.initialized {
		return fmt.Errorf("rate limit manager not initialized")
	}

	// 1. 在 MarkRegistry 中记录映射
	mark := m.registry.AssociateIP(ip, subID, rate)

	// 2. 应用 nft + tc 规则
	if err := m.backend.ApplyRule(ctx, ip, mark, rate); err != nil {
		return fmt.Errorf("apply rate limit rule for ip=%s sub=%d mark=0x%x: %w", ip, subID, mark, err)
	}

	log.Printf("[RateLimitManager] associated ip=%s → sub=%d mark=0x%x rate=↓%dMbps/↑%dMbps",
		ip, subID, mark, rate.MaxDownMbps, rate.MaxUpMbps)

	// Persist to local DB for restart recovery
	if m.persistFunc != nil {
		go m.persistFunc(ip, subID, rate.MaxDownMbps, rate.MaxUpMbps)
	}

	return nil
}

// RemoveIP 移除一个 IP 的限速规则。
func (m *Manager) RemoveIP(ctx context.Context, ip string) error {
	if !m.initialized {
		return nil
	}

	mark, entry := m.registry.LookupByIP(ip)
	if entry == nil {
		return nil // 不存在，忽略
	}

	// 从 MarkRegistry 移除
	m.registry.RemoveIP(ip)

	// 移除 nft + tc 规则
	if err := m.backend.RemoveRule(ctx, ip, mark); err != nil {
		return fmt.Errorf("remove rate limit rule for ip=%s mark=0x%x: %w", ip, mark, err)
	}

	log.Printf("[RateLimitManager] removed ip=%s (was sub=%d mark=0x%x)", ip, entry.SubscriptionID, mark)

	// Remove from persisted DB
	if m.removeFunc != nil {
		go m.removeFunc(ip)
	}

	return nil
}

// RemoveSubscription removes all rate limit rules for a given subscription ID.
// This is used by the quota block/unblock linkage to clean up all IPs
// associated with a subscription when it's blocked.
func (m *Manager) RemoveSubscription(ctx context.Context, subID int64) error {
	if !m.initialized {
		return nil
	}

	ips := m.registry.GetSubscriptionIPs(subID)
	for _, ip := range ips {
		mark, entry := m.registry.LookupByIP(ip)
		if entry == nil {
			continue
		}

		// Remove from registry
		m.registry.RemoveIP(ip)

		// Remove nft + tc rules
		if err := m.backend.RemoveRule(ctx, ip, mark); err != nil {
			log.Printf("[RateLimitManager] remove rule for ip=%s mark=0x%x on subscription removal: %v", ip, mark, err)
		}

		// Remove from persisted DB
		if m.removeFunc != nil {
			go m.removeFunc(ip)
		}
	}

	log.Printf("[RateLimitManager] removed subscription %d (%d IPs)", subID, len(ips))
	return nil
}

// TouchIP updates the last-seen timestamp for an IP if it's already associated.
// If the IP is not in the registry, it's a no-op (new IPs need full AssociateIP with rate config).
// This is used by the online-stats poller to keep active IPs from being cleaned up.
func (m *Manager) TouchIP(ctx context.Context, ip string, subID int64) error {
	if !m.initialized {
		return nil
	}

	_, entry := m.registry.LookupByIP(ip)
	if entry == nil {
		// IP not in registry — can't apply rate limit without a rate config.
		// The P0 trigger source (Panel push) should have already associated this IP.
		return nil
	}

	// Update last seen
	m.registry.TouchIP(ip)
	return nil
}

// UpdateRate 更新一个订阅的限速速率。
// 适用于用户修改限速配置后，不需要重建 nft 规则，只需更新 tc class。
func (m *Manager) UpdateRate(ctx context.Context, subID int64, rate *RateConfig) error {
	if !m.initialized {
		return fmt.Errorf("rate limit manager not initialized")
	}

	// 更新 MarkRegistry 中的限速配置
	m.registry.SetRateConfig(subID, rate)

	// 获取 mark
	mark := m.registry.GetOrCreateMark(subID)

	// 更新 tc class 速率
	if err := m.backend.UpdateRate(ctx, mark, rate); err != nil {
		return fmt.Errorf("update rate for sub=%d mark=0x%x: %w", subID, mark, err)
	}

	log.Printf("[RateLimitManager] updated rate for sub=%d mark=0x%x rate=↓%dMbps/↑%dMbps",
		subID, mark, rate.MaxDownMbps, rate.MaxUpMbps)
	return nil
}

// RemoveSubscription 移除一个订阅的所有限速规则。
// FlushAll 清除所有限速规则并重置 MarkRegistry。
func (m *Manager) FlushAll(ctx context.Context) error {
	if err := m.backend.FlushAll(ctx); err != nil {
		return fmt.Errorf("flush all rate limit rules: %w", err)
	}
	m.registry = NewMarkRegistry()
	m.initialized = false
	log.Printf("[RateLimitManager] flushed all rules and reset registry")
	return nil
}

// RestoreFromDB restores IP → subscription associations from persisted data.
// Unlike AssociateIP, this method does NOT trigger persistFunc (since data comes from DB).
// It also does NOT call backend.ApplyRule if skipRules is true (useful when the
// rules will be applied in bulk via RestoreRules).
func (m *Manager) RestoreFromDB(ctx context.Context, ip string, subID int64, rate *RateConfig) error {
	if !m.initialized {
		return fmt.Errorf("rate limit manager not initialized")
	}

	// Record in MarkRegistry (same as AssociateIP)
	mark := m.registry.AssociateIP(ip, subID, rate)

	// Apply nft + tc rules
	if err := m.backend.ApplyRule(ctx, ip, mark, rate); err != nil {
		return fmt.Errorf("restore rate limit rule for ip=%s sub=%d mark=0x%x: %w", ip, subID, mark, err)
	}

	// NOTE: no persistFunc call — data already in DB
	return nil
}

// IsAvailable 检查限速后端是否可用。
func (m *Manager) IsAvailable() Availability {
	return m.backend.IsAvailable()
}

// GetStatus 获取当前限速状态（用于上报 Panel）。
func (m *Manager) GetStatus(subID int64) []RateLimitEntry {
	entries := m.registry.GetAllEntries()
	var result []RateLimitEntry
	for _, entry := range entries {
		if subID > 0 && entry.SubscriptionID != subID {
			continue
		}
		rate := m.registry.GetRateConfig(entry.SubscriptionID)
		var maxDown, maxUp int32
		if rate != nil {
			maxDown = rate.MaxDownMbps
			maxUp = rate.MaxUpMbps
		}
		result = append(result, RateLimitEntry{
			SubscriptionID: entry.SubscriptionID,
			IP:             entry.IP,
			Mark:           entry.Mark,
			MaxDownMbps:    maxDown,
			MaxUpMbps:     maxUp,
			LastSeen:      entry.LastSeen.Unix(),
		})
	}
	return result
}

// StartCleanupLoop 启动后台清理协程。
// 定期清理超过 TTL 未活跃的 IP 映射和规则。
func (m *Manager) StartCleanupLoop(interval time.Duration) {
	if m.cleanupTTL <= 0 {
		return
	}
	m.cleanupTicker = time.NewTicker(interval)
	m.cleanupDone = make(chan struct{})
	go m.cleanupLoop()
	log.Printf("[RateLimitManager] cleanup loop started (interval=%s, ttl=%s)", interval, m.cleanupTTL)
}

// StopCleanupLoop 停止后台清理协程。
func (m *Manager) StopCleanupLoop() {
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
	if m.cleanupDone != nil {
		close(m.cleanupDone)
	}
}

func (m *Manager) cleanupLoop() {
	for {
		select {
		case <-m.cleanupDone:
			return
		case <-m.cleanupTicker.C:
			m.doCleanup()
		}
	}
}

func (m *Manager) doCleanup() {
	removed := m.registry.CleanupStale(m.cleanupTTL)
	if len(removed) == 0 {
		return
	}
	ctx := context.Background()
	for _, r := range removed {
		if err := m.backend.RemoveRule(ctx, r.IP, r.Mark); err != nil {
			log.Printf("[RateLimitManager] cleanup: failed to remove rule for ip=%s mark=0x%x: %v", r.IP, r.Mark, err)
		}
		// Also remove from persisted DB
		if m.removeFunc != nil {
			m.removeFunc(r.IP)
		}
	}
	log.Printf("[RateLimitManager] cleanup: removed %d stale entries", len(removed))
}

// Close 清理资源。
func (m *Manager) Close() error {
	m.StopCleanupLoop()
	return m.backend.Close()
}

// RateLimitEntry 是限速状态的导出结构（与 proto 定义对齐）。
type RateLimitEntry struct {
	SubscriptionID int64
	IP             string
	Mark           uint32
	MaxDownMbps    int32
	MaxUpMbps     int32
	DownlinkBytes  int64
	UplinkBytes    int64
	LastSeen       int64
}
