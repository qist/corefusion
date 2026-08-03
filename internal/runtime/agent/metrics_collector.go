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

	// 上次 xray 核心用户流量累计计数（用于计算实时速率）
	lastTrafficStats map[int64]*trafficCounter

	// 本地节点（面板自身）主机指标采集器；若设置，本地节点走此路径（StatsService 直接读 /proc），
	// 而非 gRPC GetStatus（独立 agent 接口）。
	localStatsFn func(int64) *entity.ServerMetric

	// 本地节点 xray 核心用户流量采集器；若设置，查询 xray Stats API 返回累计 (uplink, downlink)。
	// 速率由 collectFromLocalStats 基于两次采样差分计算。
	// 仅统计 user>>> 流量，自动排除 inbound>>>api>>> 等 API 流量。
	localTrafficFn func() (uplink int64, downlink int64)

	// 远程节点 xray 核心用户流量采集器；若设置，按 serverID 查询远程节点 xray 累计流量。
	// 通过 coredata.CoreDataProvider 在 router.go 中注入实现。
	remoteTrafficFn func(ctx context.Context, serverID int64) (uplink int64, downlink int64)
}

// netCounter tracks network byte counters for rate calculation.
type netCounter struct {
	rxBytes   int64
	txBytes   int64
	timestamp time.Time
}

// trafficCounter tracks xray core user traffic cumulative counters for rate calculation.
type trafficCounter struct {
	uplink    int64
	downlink  int64
	timestamp time.Time
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
		lastNetStats:     make(map[int64]*netCounter),
		lastTrafficStats: make(map[int64]*trafficCounter),
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
		ServerID:         serverID,
		CPUUsage:         report.CpuUsage,
		CPUCores:         int(report.CpuCores),
		MemUsageBytes:    report.MemUsageBytes,
		MemTotalBytes:    report.MemTotalBytes,
		MemUsage:         report.MemUsage,
		SwapTotalBytes:   report.SwapTotalBytes,
		SwapUsedBytes:    report.SwapUsedBytes,
		SwapUsage:        report.SwapUsage,
		DiskUsageBytes:   report.DiskUsageBytes,
		DiskTotalBytes:   report.DiskTotalBytes,
		DiskUsage:        report.DiskUsage,
		DiskReadBytesSec:  report.DiskReadBytesSec,
		DiskWriteBytesSec: report.DiskWriteBytesSec,
		Load1m:           report.Load_1M,
		TCPConnections:    int(report.TcpConnections),
		UptimeSeconds:     report.UptimeSeconds,
		TrafficUplink:    report.RateLimitTotalUplink,
		TrafficDownlink:  report.RateLimitTotalDownlink,
		AgentVersion:     report.AgentVersion,
		Hostname:         report.Hostname,

		// Egress interface traffic (出口网卡累计流量)
		EgressInterface: report.EgressInterface,
		EgressRxBytes:   report.EgressRxBytes,
		EgressTxBytes:   report.EgressTxBytes,

		CreatedAt:        now,
	}

	// Use Agent-reported network rates if available (field 80/81),
	// otherwise fall back to delta calculation from cumulative counters.
	if report.NetRxBytesSec > 0 || report.NetTxBytesSec > 0 {
		metric.NetRxBytes = report.NetRxBytesSec
		metric.NetTxBytes = report.NetTxBytesSec
	} else {
		// Legacy: calculate rate from cumulative counters
		if last, ok := mc.lastNetStats[serverID]; ok {
			elapsed := now.Sub(last.timestamp).Seconds()
			if elapsed > 0 {
				metric.NetRxBytes = int64(float64(report.NetRxBytes-last.rxBytes) / elapsed)
				metric.NetTxBytes = int64(float64(report.NetTxBytes-last.txBytes) / elapsed)
			}
		}
	}

	// Update last counters (for legacy delta calculation fallback)
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

// SetLocalStatsProvider 注入本地节点（面板自身）的主机指标采集函数。
// 本地节点不走独立 agent 的 gRPC 接口，直接使用 StatsService 读取本机 /proc 资源。
func (mc *MetricsCollector) SetLocalStatsProvider(fn func(int64) *entity.ServerMetric) {
	mc.localStatsFn = fn
}

// SetLocalTrafficProvider 注入本地节点 xray 核心用户流量采集函数。
// 该函数应查询 xray Stats API（reset_=false，累计值）并返回所有用户的
// 总 uplink/downlink 字节。仅统计 user>>> 条目，自动排除 API inbound 流量。
// 速率由 collectFromLocalStats 基于两次采样差分计算。
func (mc *MetricsCollector) SetLocalTrafficProvider(fn func() (int64, int64)) {
	mc.localTrafficFn = fn
}

// SetRemoteTrafficProvider 注入远程节点 xray 核心用户流量采集函数。
// 该函数应通过 coredata.CoreDataProvider 查询指定 serverID 的 xray 累计用户流量。
// 速率由 collectRemoteMetrics 基于两次采样差分计算。
func (mc *MetricsCollector) SetRemoteTrafficProvider(fn func(ctx context.Context, serverID int64) (int64, int64)) {
	mc.remoteTrafficFn = fn
}

// collectFromLocalStats 采集本地节点（面板自身）的主机指标并落库。
// 本地节点不发送心跳，且不应走独立 agent 的 GetStatus gRPC 接口，
// 因此直接调用注入的 StatsService（读本机 /proc）。
// 同时查询 xray Stats API 获取核心用户流量，计算实时速率。
func (mc *MetricsCollector) collectFromLocalStats(serverID int64) {
	if mc.localStatsFn == nil {
		return
	}
	// 在锁外采集本机 /proc 和 xray stats（可能执行外部命令/gRPC），避免阻塞其他指标写入
	metric := mc.localStatsFn(serverID)
	if metric == nil {
		return
	}

	// 查询 xray 核心用户流量（累计值，不含 API inbound）
	var trafficUp, trafficDown int64
	if mc.localTrafficFn != nil {
		trafficUp, trafficDown = mc.localTrafficFn()
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	metric.CreatedAt = now

	// --- 网卡速率：基于累计计数器差分计算 ---
	// StatsService 返回累计 NetBytesRecv/Sent，存入 rawRx/rawRx 用于下次差分
	rawRx := metric.NetRxBytes
	rawTx := metric.NetTxBytes
	if last, ok := mc.lastNetStats[serverID]; ok {
		elapsed := now.Sub(last.timestamp).Seconds()
		if elapsed > 0 {
			metric.NetRxBytes = int64(float64(rawRx-last.rxBytes) / elapsed)
			metric.NetTxBytes = int64(float64(rawTx-last.txBytes) / elapsed)
		}
	} else {
		// 无基准时首条记录不展示速率，避免把累计值误当作速率
		metric.NetRxBytes = 0
		metric.NetTxBytes = 0
	}
	// 保存原始累计值（而非速率），供下次差分使用
	mc.lastNetStats[serverID] = &netCounter{
		rxBytes:   rawRx,
		txBytes:   rawTx,
		timestamp: now,
	}

	// --- xray 核心用户流量速率：基于累计计数器差分计算 ---
	if last, ok := mc.lastTrafficStats[serverID]; ok {
		elapsed := now.Sub(last.timestamp).Seconds()
		if elapsed > 0 {
			metric.TrafficUplink = int64(float64(trafficUp-last.uplink) / elapsed)
			metric.TrafficDownlink = int64(float64(trafficDown-last.downlink) / elapsed)
		}
	} else {
		metric.TrafficUplink = 0
		metric.TrafficDownlink = 0
	}
	mc.lastTrafficStats[serverID] = &trafficCounter{
		uplink:    trafficUp,
		downlink:  trafficDown,
		timestamp: now,
	}

	if err := mc.db.Create(metric).Error; err != nil {
		log.Printf("[MetricsCollector] failed to store local stats metric for server %d: %v", serverID, err)
		return
	}
	log.Printf("[MetricsCollector] stored local stats metric for server %d: cpu=%.1f%% mem=%d/%d bytes disk=%.1f%% traffic_up=%d/s traffic_down=%d/s",
		serverID, metric.CPUUsage, metric.MemUsageBytes, metric.MemTotalBytes, metric.DiskUsage,
		metric.TrafficUplink, metric.TrafficDownlink)
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
			// 本地节点（面板自身）：直接走 StatsService 采集，不走独立 agent 的 gRPC 接口
			if mc.localStatsFn != nil {
				mc.collectFromLocalStats(server.ID)
			}
			continue
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

	resp, err := mc.nodeMgr.GetMetricsFromServer(ctx, serverID, 1, "")
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

	// 查询远程节点 xray 用户流量（通过注入的 remoteTrafficFn，底层走 coredata.Provider）
	var trafficUp, trafficDown int64
	if mc.remoteTrafficFn != nil {
		trafficUp, trafficDown = mc.remoteTrafficFn(ctx, serverID)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Unix(p.Timestamp, 0)

	metric := &entity.ServerMetric{
		ServerID:          serverID,
		CPUUsage:          p.CpuUsage,
		CPUCores:          int(p.CpuCores),
		MemUsageBytes:     p.MemUsageBytes,
		MemTotalBytes:     p.MemTotalBytes,
		MemUsage:          p.MemUsage,
		SwapTotalBytes:    p.SwapTotalBytes,
		SwapUsedBytes:     p.SwapUsedBytes,
		SwapUsage:         p.SwapUsage,
		DiskUsageBytes:    p.DiskUsageBytes,
		DiskTotalBytes:    p.DiskTotalBytes,
		DiskUsage:         p.DiskUsage,
		DiskReadBytesSec:  p.DiskReadBytesSec,
		DiskWriteBytesSec: p.DiskWriteBytesSec,
		NetRxBytes:        p.NetRxBytesSec,
		NetTxBytes:        p.NetTxBytesSec,
		Load1m:            p.Load_1M,
		TCPConnections:    int(p.TcpConnections),
		UptimeSeconds:     p.UptimeSeconds,
		CreatedAt:         now,
	}

	// 计算 xray 用户流量速率（差分）
	if last, ok := mc.lastTrafficStats[serverID]; ok {
		elapsed := now.Sub(last.timestamp).Seconds()
		if elapsed > 0 {
			metric.TrafficUplink = int64(float64(trafficUp-last.uplink) / elapsed)
			metric.TrafficDownlink = int64(float64(trafficDown-last.downlink) / elapsed)
		}
	}
	mc.lastTrafficStats[serverID] = &trafficCounter{
		uplink:    trafficUp,
		downlink:  trafficDown,
		timestamp: now,
	}

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
		var cpuSum float64
		latest := metrics[0]

		for _, m := range metrics {
			cpuSum += m.CPUUsage
		}

		result[server.ID] = &ServerMetricSummary{
			ServerID:         server.ID,
			Name:             server.Name,
			Region:           server.Region,
			Online:           mc.nodeMgr.IsOnlineForServer(server.ID),
			CPUUsage:         cpuSum / float64(len(metrics)),
			CPUCores:         latest.CPUCores,
			MemUsageBytes:    latest.MemUsageBytes,
			MemTotalBytes:    latest.MemTotalBytes,
			MemUsage:         latest.MemUsage,
			SwapTotalBytes:   latest.SwapTotalBytes,
			SwapUsedBytes:    latest.SwapUsedBytes,
			SwapUsage:        latest.SwapUsage,
			DiskUsageBytes:   latest.DiskUsageBytes,
			DiskTotalBytes:   latest.DiskTotalBytes,
			DiskUsage:        latest.DiskUsage,
			DiskReadBytesSec: latest.DiskReadBytesSec,
			DiskWriteBytesSec: latest.DiskWriteBytesSec,
			NetRxBytes:       latest.NetRxBytes,
			NetTxBytes:       latest.NetTxBytes,
			TCPConnections:   latest.TCPConnections,
			Load1m:           latest.Load1m,
			UptimeSeconds:    latest.UptimeSeconds,
			Hostname:         latest.Hostname,
			TrafficUplink:    latest.TrafficUplink,
			TrafficDownlink:  latest.TrafficDownlink,
			LastUpdated:      latest.CreatedAt,
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

	// 启动立即采集一次，避免本地节点（不发送心跳）在首个 tick 前长期无数据
	if err := mc.CollectAllServers(ctx); err != nil {
		log.Printf("[MetricsCollector] initial collect error: %v", err)
	}

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

	var cpuSum, loadSum, memUsageSum, swapUsageSum, diskUsageSum float64
	var memSum, diskSum, netRxSum, netTxSum, uplinkSum, downlinkSum int64
	var memTotalSum, diskTotalSum int64
	var diskReadSum, diskWriteSum int64
	var tcpSum int
	var uptimeSum int64

	for _, m := range metrics {
		cpuSum += m.CPUUsage
		memSum += m.MemUsageBytes
		memTotalSum += m.MemTotalBytes
		memUsageSum += m.MemUsage
		diskSum += m.DiskUsageBytes
		diskTotalSum += m.DiskTotalBytes
		diskUsageSum += m.DiskUsage
		diskReadSum += m.DiskReadBytesSec
		diskWriteSum += m.DiskWriteBytesSec
		netRxSum += m.NetRxBytes
		netTxSum += m.NetTxBytes
		loadSum += m.Load1m
		tcpSum += m.TCPConnections
		uplinkSum += m.TrafficUplink
		downlinkSum += m.TrafficDownlink
		uptimeSum += m.UptimeSeconds
		swapUsageSum += m.SwapUsage
	}

	n := float64(len(metrics))
	return entity.ServerMetric{
		ServerID:          metrics[0].ServerID,
		CPUUsage:          cpuSum / n,
		CPUCores:          metrics[0].CPUCores,
		MemUsageBytes:     memSum / int64(n),
		MemTotalBytes:     memTotalSum / int64(n),
		MemUsage:          memUsageSum / n,
		SwapTotalBytes:    metrics[0].SwapTotalBytes,
		SwapUsedBytes:     metrics[0].SwapUsedBytes,
		SwapUsage:         swapUsageSum / n,
		DiskUsageBytes:    diskSum / int64(n),
		DiskTotalBytes:    diskTotalSum / int64(n),
		DiskUsage:         diskUsageSum / n,
		DiskReadBytesSec:  diskReadSum / int64(n),
		DiskWriteBytesSec: diskWriteSum / int64(n),
		NetRxBytes:        netRxSum / int64(n),
		NetTxBytes:        netTxSum / int64(n),
		Load1m:            loadSum / n,
		TCPConnections:    tcpSum / int(n),
		UptimeSeconds:     uptimeSum / int64(n),
		TrafficUplink:     uplinkSum / int64(n),
		TrafficDownlink:   downlinkSum / int64(n),
		Hostname:          metrics[0].Hostname,
		CreatedAt:         metrics[len(metrics)-1].CreatedAt,
	}
}

// ServerMetricSummary is an aggregated view of a server's current metrics.
type ServerMetricSummary struct {
	ServerID        int64     `json:"server_id"`
	Name            string    `json:"name"`
	Region          string    `json:"region"`
	Online          bool      `json:"online"`

	// CPU
	CPUUsage        float64   `json:"cpu_usage"`
	CPUCores        int       `json:"cpu_cores"`

	// Memory
	MemUsageBytes   int64     `json:"mem_usage_bytes"`
	MemTotalBytes   int64     `json:"mem_total_bytes"`
	MemUsage        float64   `json:"mem_usage"`

	// Swap
	SwapTotalBytes  int64     `json:"swap_total_bytes"`
	SwapUsedBytes   int64     `json:"swap_used_bytes"`
	SwapUsage       float64   `json:"swap_usage"`

	// Disk
	DiskUsageBytes  int64     `json:"disk_usage_bytes"`
	DiskTotalBytes  int64     `json:"disk_total_bytes"`
	DiskUsage       float64   `json:"disk_usage"`

	// Disk I/O
	DiskReadBytesSec  int64   `json:"disk_read_bytes_sec"`
	DiskWriteBytesSec int64   `json:"disk_write_bytes_sec"`

	// Network
	NetRxBytes      int64     `json:"net_rx_bytes"`
	NetTxBytes      int64     `json:"net_tx_bytes"`

	// System
	TCPConnections  int       `json:"tcp_connections"`
	Load1m          float64   `json:"load_1m"`
	UptimeSeconds   int64     `json:"uptime_seconds"`
	Hostname        string    `json:"hostname"`

	// Traffic
	TrafficUplink   int64     `json:"traffic_uplink"`
	TrafficDownlink int64     `json:"traffic_downlink"`
	LastUpdated     time.Time `json:"last_updated"`
}

// GetLatestMetricsBatch returns the most recent metric for each server.
// Used for embedding resource usage in node server list responses.
func (mc *MetricsCollector) GetLatestMetricsBatch(ctx context.Context) (map[int64]*entity.ServerMetric, error) {
	if mc.db == nil {
		return nil, nil
	}

	servers, err := mc.serverRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[int64]*entity.ServerMetric, len(servers))
	for _, server := range servers {
		var metric entity.ServerMetric
		if err := mc.db.Where("server_id = ?", server.ID).
			Order("created_at DESC").
			Limit(1).
			Find(&metric).Error; err != nil || metric.ID == 0 {
			continue
		}
		result[server.ID] = &metric
	}

	return result, nil
}
