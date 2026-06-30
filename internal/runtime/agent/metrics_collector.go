package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// MetricsCollector collects and stores server resource metrics for monitoring charts.
//
// 数据来源：
//   1. Agent 心跳上报（AgentReport 中的 CPU、内存、磁盘、网络指标）
//   2. gRPC GetStatus 主动查询（补充心跳间隙）
//   3. gRPC GetMetrics 查询历史指标（Agent 侧缓存的高频数据点）
//
// 存储策略：
//   - 高频数据（10s 间隔）保留 1 小时
//   - 中频数据（1min 间隔）保留 7 天
//   - 低频数据（5min 间隔）保留 90 天
//   - 超过保留期的数据自动清理
type MetricsCollector struct {
	mu         sync.Mutex
	db         *gorm.DB
	serverRepo *repository.NodeServerRepository
	nodeMgr    *NodeServerManager

	// 配置
	collectInterval time.Duration // 采集间隔
	retentionShort  time.Duration // 高频数据保留
	retentionMedium time.Duration // 中频数据保留
	retentionLong   time.Duration // 低频数据保留

	// 上次网络计数（用于计算速率）
	lastNetStats map[int64]*netCounter
}

// netCounter tracks network byte counters for rate calculation.
type netCounter struct {
	rxBytes    int64
	txBytes    int64
	timestamp  time.Time
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(db *gorm.DB, serverRepo *repository.NodeServerRepository, nodeMgr *NodeServerManager) *MetricsCollector {
	return &MetricsCollector{
		db:              db,
		serverRepo:      serverRepo,
		nodeMgr:         nodeMgr,
		collectInterval: 30 * time.Second,
		retentionShort:  1 * time.Hour,
		retentionMedium: 7 * 24 * time.Hour,
		retentionLong:   90 * 24 * time.Hour,
		lastNetStats:    make(map[int64]*netCounter),
	}
}

// WithCollectInterval sets the collection interval.
func (mc *MetricsCollector) WithCollectInterval(d time.Duration) *MetricsCollector {
	mc.collectInterval = d
	return mc
}

// CollectFromHeartbeat processes a heartbeat report and stores metrics.
// Called by NodeServerManager when a HEARTBEAT report is received.
func (mc *MetricsCollector) CollectFromHeartbeat(serverID int64, report *agentpb.AgentReport) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	metric := &entity.ServerMetric{
		ServerID:        serverID,
		CPUUsage:        report.CpuUsage,
		MemUsageBytes:   report.MemUsageBytes,
		DiskUsageBytes:  report.DiskUsageBytes,
		NetRxBytes:      report.NetRxBytes,
		NetTxBytes:      report.NetTxBytes,
		AgentVersion:    report.AgentVersion,
		CreatedAt:       now,
	}

	// Calculate network rate
	if last, ok := mc.lastNetStats[serverID]; ok {
		elapsed := now.Sub(last.timestamp).Seconds()
		if elapsed > 0 {
			// Store rate (bytes/sec) instead of cumulative
			metric.NetRxBytes = (report.NetRxBytes - last.rxBytes) / int64(elapsed)
			metric.NetTxBytes = (report.NetTxBytes - last.txBytes) / int64(elapsed)
		}
	}

	// Update last counters
	mc.lastNetStats[serverID] = &netCounter{
		rxBytes:   report.NetRxBytes,
		txBytes:   report.NetTxBytes,
		timestamp: now,
	}

	// Store in database
	if err := mc.db.Create(metric).Error; err != nil {
		log.Printf("[MetricsCollector] failed to store metric for server %d: %v", serverID, err)
	}
}

// CollectFromStatus collects metrics from a GetStatus response.
// Used for local servers that don't send heartbeats.
func (mc *MetricsCollector) CollectFromStatus(serverID int64, resp *agentpb.GetStatusResponse) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metric := &entity.ServerMetric{
		ServerID:      serverID,
		CPUUsage:      resp.CpuUsage,
		MemUsageBytes: resp.MemUsageBytes,
		CreatedAt:     time.Now(),
	}

	// Sum up core metrics
	for _, core := range resp.Cores {
		metric.CPUUsage += core.CpuUsage
	}

	if err := mc.db.Create(metric).Error; err != nil {
		log.Printf("[MetricsCollector] failed to store status metric for server %d: %v", serverID, err)
	}
}

// CollectAllServers queries all servers and collects current metrics.
func (mc *MetricsCollector) CollectAllServers(ctx context.Context) error {
	servers, err := mc.serverRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list servers: %w", err)
	}

	for _, server := range servers {
		if !server.Enabled {
			continue
		}

		if mc.nodeMgr.isLocalServer(server.ID) {
			// Local server: use GetStatusForServer
			resp, err := mc.nodeMgr.GetStatusForServer(ctx, server.ID)
			if err != nil {
				log.Printf("[MetricsCollector] local server %d: %v", server.ID, err)
				continue
			}
			mc.CollectFromStatus(server.ID, resp)
		} else {
			// Remote server: try GetMetrics for detailed data
			mc.collectRemoteMetrics(ctx, server.ID)
		}
	}

	return nil
}

// collectRemoteMetrics queries detailed metrics from a remote Agent.
func (mc *MetricsCollector) collectRemoteMetrics(ctx context.Context, serverID int64) {
	ac := mc.nodeMgr.GetConnection(serverID)
	if ac == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := ac.agentClient.GetMetrics(ctx, &agentpb.GetMetricsRequest{
		LastNPoints: 1,
	})
	if err != nil {
		// Fallback to GetStatus
		statusResp, err2 := mc.nodeMgr.GetServerStatus(ctx, serverID)
		if err2 != nil {
			log.Printf("[MetricsCollector] remote server %d: GetMetrics: %v, GetStatus: %v", serverID, err, err2)
			return
		}
		mc.CollectFromStatus(serverID, statusResp)
		return
	}

	if len(resp.Points) == 0 {
		return
	}

	p := resp.Points[0]
	metric := &entity.ServerMetric{
		ServerID:        serverID,
		CPUUsage:        p.CpuUsage,
		MemUsageBytes:   p.MemUsageBytes,
		DiskUsageBytes:  p.DiskUsageBytes,
		NetRxBytes:      p.NetRxBytes,
		NetTxBytes:      p.NetTxBytes,
		Load1m:          p.Load_1M,
		TCPConnections:  int(p.TcpConnections),
		TrafficUplink:   p.TrafficUplink,
		TrafficDownlink: p.TrafficDownlink,
		CreatedAt:       time.Unix(p.Timestamp, 0),
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if err := mc.db.Create(metric).Error; err != nil {
		log.Printf("[MetricsCollector] failed to store remote metric for server %d: %v", serverID, err)
	}
}

// GetMetricsHistory retrieves historical metrics for a server.
//
// Parameters:
//   - serverID: the server to query
//   - duration: how far back to look (e.g., 1h, 24h, 7d)
//   - interval: data point interval (e.g., 1m, 5m)
func (mc *MetricsCollector) GetMetricsHistory(serverID int64, duration, interval time.Duration) ([]entity.ServerMetric, error) {
	since := time.Now().Add(-duration)

	var metrics []entity.ServerMetric
	err := mc.db.Where("server_id = ? AND created_at >= ?", serverID, since).
		Order("created_at ASC").
		Find(&metrics).Error
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}

	// Downsample if needed
	if interval > mc.collectInterval {
		metrics = downsample(metrics, interval)
	}

	return metrics, nil
}

// GetAggregatedMetrics returns aggregated metrics for all servers.
func (mc *MetricsCollector) GetAggregatedMetrics(ctx context.Context) (map[int64]*ServerMetricSummary, error) {
	servers, err := mc.serverRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*ServerMetricSummary)
	since := time.Now().Add(-5 * time.Minute)

	for _, server := range servers {
		var metrics []entity.ServerMetric
		if err := mc.db.Where("server_id = ? AND created_at >= ?", server.ID, since).
			Order("created_at DESC").
			Limit(10).
			Find(&metrics).Error; err != nil {
			continue
		}

		if len(metrics) == 0 {
			result[server.ID] = &ServerMetricSummary{
				ServerID:  server.ID,
				Name:      server.Name,
				Region:    server.Region,
				Online:    mc.nodeMgr.IsOnlineForServer(server.ID),
			}
			continue
		}

		// Calculate averages
		var cpuSum, memSum float64
		var netRxSum, netTxSum int64
		latest := metrics[0]

		for _, m := range metrics {
			cpuSum += m.CPUUsage
			memSum += float64(m.MemUsageBytes)
			netRxSum += m.NetRxBytes
			netTxSum += m.NetTxBytes
		}

		result[server.ID] = &ServerMetricSummary{
			ServerID:        server.ID,
			Name:            server.Name,
			Region:          server.Region,
			Online:          mc.nodeMgr.IsOnlineForServer(server.ID),
			CPUUsage:        cpuSum / float64(len(metrics)),
			MemUsageBytes:   int64(memSum / float64(len(metrics))),
			NetRxBytes:      netRxSum / int64(len(metrics)),
			NetTxBytes:      netTxSum / int64(len(metrics)),
			TCPConnections:  latest.TCPConnections,
			Load1m:          latest.Load1m,
			TrafficUplink:   latest.TrafficUplink,
			TrafficDownlink: latest.TrafficDownlink,
			LastUpdated:     latest.CreatedAt,
		}
	}

	return result, nil
}

// RunLoop starts the periodic collection loop. Blocks until ctx is cancelled.
func (mc *MetricsCollector) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(mc.collectInterval)
	defer ticker.Stop()

	// Cleanup ticker (runs every hour)
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	log.Printf("[MetricsCollector] loop started, interval=%v", mc.collectInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[MetricsCollector] stopped")
			return
		case <-ticker.C:
			if err := mc.CollectAllServers(ctx); err != nil {
				log.Printf("[MetricsCollector] collect error: %v", err)
			}
		case <-cleanupTicker.C:
			mc.cleanupOldData()
		}
	}
}

// cleanupOldData removes metrics older than the retention period.
func (mc *MetricsCollector) cleanupOldData() {
	cutoff := time.Now().Add(-mc.retentionLong)
	result := mc.db.Where("created_at < ?", cutoff).Delete(&entity.ServerMetric{})
	if result.RowsAffected > 0 {
		log.Printf("[MetricsCollector] cleaned up %d old metric records (older than %v)",
			result.RowsAffected, mc.retentionLong)
	}
}

// downsample reduces the number of data points by averaging over intervals.
func downsample(metrics []entity.ServerMetric, interval time.Duration) []entity.ServerMetric {
	if len(metrics) <= 1 {
		return metrics
	}

	result := make([]entity.ServerMetric, 0, len(metrics)/int(interval/time.Second))
	var bucket []entity.ServerMetric
	var bucketStart time.Time

	for _, m := range metrics {
		if bucketStart.IsZero() {
			bucketStart = m.CreatedAt
		}

		if m.CreatedAt.Sub(bucketStart) >= interval {
			// Process bucket
			if len(bucket) > 0 {
				avg := averageMetrics(bucket)
				result = append(result, avg)
			}
			bucket = bucket[:0]
			bucketStart = m.CreatedAt
		}

		bucket = append(bucket, m)
	}

	// Process remaining
	if len(bucket) > 0 {
		avg := averageMetrics(bucket)
		result = append(result, avg)
	}

	return result
}

// averageMetrics averages a slice of metrics into a single point.
func averageMetrics(metrics []entity.ServerMetric) entity.ServerMetric {
	if len(metrics) == 0 {
		return entity.ServerMetric{}
	}

	var cpuSum, loadSum float64
	var memSum, diskSum, netRxSum, netTxSum, uplinkSum, downlinkSum int64
	var tcpSum int

	for _, m := range metrics {
		cpuSum += m.CPUUsage
		memSum += m.MemUsageBytes
		diskSum += m.DiskUsageBytes
		netRxSum += m.NetRxBytes
		netTxSum += m.NetTxBytes
		loadSum += m.Load1m
		tcpSum += m.TCPConnections
		uplinkSum += m.TrafficUplink
		downlinkSum += m.TrafficDownlink
	}

	n := float64(len(metrics))
	return entity.ServerMetric{
		ServerID:        metrics[0].ServerID,
		CPUUsage:        cpuSum / n,
		MemUsageBytes:   memSum / int64(n),
		DiskUsageBytes:  diskSum / int64(n),
		NetRxBytes:      netRxSum / int64(n),
		NetTxBytes:      netTxSum / int64(n),
		Load1m:          loadSum / n,
		TCPConnections:  tcpSum / int(n),
		TrafficUplink:   uplinkSum / int64(n),
		TrafficDownlink: downlinkSum / int64(n),
		CreatedAt:       metrics[len(metrics)-1].CreatedAt,
	}
}

// ServerMetricSummary is an aggregated view of a server's current metrics.
type ServerMetricSummary struct {
	ServerID        int64     `json:"server_id"`
	Name            string    `json:"name"`
	Region          string    `json:"region"`
	Online          bool      `json:"online"`
	CPUUsage        float64   `json:"cpu_usage"`
	MemUsageBytes   int64     `json:"mem_usage_bytes"`
	NetRxBytes      int64     `json:"net_rx_bytes"`
	NetTxBytes      int64     `json:"net_tx_bytes"`
	TCPConnections  int       `json:"tcp_connections"`
	Load1m          float64   `json:"load_1m"`
	TrafficUplink   int64     `json:"traffic_uplink"`
	TrafficDownlink int64     `json:"traffic_downlink"`
	LastUpdated     time.Time `json:"last_updated"`
}
