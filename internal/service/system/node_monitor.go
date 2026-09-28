package system

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"corefusion/internal/model/entity"

	"gorm.io/gorm"
)

// ============================================================
// 节点健康监控 —— 监控类通知的核心实现：
//   按「频率」持续检查节点心跳，**状态发生变化才发通知**（异常发告警、
//   恢复发通知），状态未变化的轮次全部静默跳过。
//
//   频率：设置键 monitor.node_interval_seconds（单位秒，默认 60，最小 30）
//   离线判定：AgentStatus != online 或心跳超时（monitor.node_offline_timeout_seconds，默认 120）
//   通知：按 name 查找**已启用**的 admin 模板「节点离线告警」/「节点恢复通知」，
//         模板未启用时 NotifyTemplate 静默 no-op —— 即"没配置就只监控不发"。
// ============================================================

const (
	MonitorIntervalSetting   = "monitor.node_interval_seconds"
	MonitorOfflineTimeoutKey = "monitor.node_offline_timeout_seconds"
	defaultMonitorInterval   = 60 * time.Second
	defaultOfflineTimeout    = 120 * time.Second
	minMonitorInterval       = 30 * time.Second
	tmplNameNodeDown         = "节点离线告警"
	tmplNameNodeRecovered    = "节点恢复通知"
)

type NodeHealthMonitor struct {
	db         *gorm.DB
	settingSvc *SettingService

	mu       sync.Mutex
	lastSeen map[int64]string // nodeID -> 上轮状态（online/offline）；首轮只初始化不发通知
}

// StartNodeHealthMonitor 启动节点健康监控循环（阻塞调用，请用 go 启动）。
func StartNodeHealthMonitor(ctx context.Context, db *gorm.DB, settingSvc *SettingService) {
	m := &NodeHealthMonitor{db: db, settingSvc: settingSvc, lastSeen: map[int64]string{}}
	run := func() {
		if err := m.scan(ctx); err != nil {
			log.Printf("[node-monitor] scan failed: %v", err)
		}
	}
	for {
		interval := m.currentInterval()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			run()
		}
	}
}

// currentInterval 读取监控频率设置（每轮重读，改设置无需重启；下限 30s 防误配打爆）。
func (m *NodeHealthMonitor) currentInterval() time.Duration {
	interval := defaultMonitorInterval
	if v, err := m.settingSvc.GetByKey(context.Background(), MonitorIntervalSetting); err == nil && v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}
	if interval < minMonitorInterval {
		interval = minMonitorInterval
	}
	return interval
}

func (m *NodeHealthMonitor) offlineTimeout() time.Duration {
	timeout := defaultOfflineTimeout
	if v, err := m.settingSvc.GetByKey(context.Background(), MonitorOfflineTimeoutKey); err == nil && v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	return timeout
}

// scan 一轮检查：对比上一轮状态，仅在变化时发通知，其余静默跳过。
func (m *NodeHealthMonitor) scan(ctx context.Context) error {
	var nodes []entity.NodeServer
	if err := m.db.Where("is_local = ?", false).Find(&nodes).Error; err != nil {
		return err
	}
	timeout := m.offlineTimeout()
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, node := range nodes {
		cur := "online"
		if node.AgentStatus != "online" || node.LastHeartbeatAt == nil || now.Sub(*node.LastHeartbeatAt) > timeout {
			cur = "offline"
		}

		prev, seen := m.lastSeen[node.ID]
		if !seen {
			// 首轮只建立基线，避免面板重启后重复告警
			m.lastSeen[node.ID] = cur
			continue
		}
		if cur == prev {
			continue // 状态未变化 —— 静默跳过
		}
		m.lastSeen[node.ID] = cur

		vars := map[string]string{
			"node": node.Name,
			"time": now.Format("2006-01-02 15:04:05"),
		}
		if cur == "offline" {
			if err := NotifyTemplate(ctx, tmplNameNodeDown, vars); err != nil {
				log.Printf("[node-monitor] offline alert for %s failed: %v", node.Name, err)
			} else {
				log.Printf("[node-monitor] node %s went OFFLINE — alert sent", node.Name)
			}
		} else {
			if err := NotifyTemplate(ctx, tmplNameNodeRecovered, vars); err != nil {
				log.Printf("[node-monitor] recovery notice for %s failed: %v", node.Name, err)
			} else {
				log.Printf("[node-monitor] node %s RECOVERED — notice sent", node.Name)
			}
		}
	}
	return nil
}
