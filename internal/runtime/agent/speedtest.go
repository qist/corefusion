package agent

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"sort"
	"sync"
	"time"

	"corefusion/internal/store/repository"
)

// SpeedTestManager manages node latency testing and auto-sorting.
//
// 测速策略：
//   - 本地服务器：直接 TCP 连接到入站端口测延迟
//   - 远程服务器：通过 gRPC 调用 Agent.SpeedTest，由 Agent 在本地执行测速
//   - 结果存储在内存中，用于负载均衡排序
//   - 定时执行（默认 5 分钟），支持手动触发
type SpeedTestManager struct {
	mu         sync.RWMutex
	serverRepo *repository.NodeServerRepository
	nodeMgr    *NodeServerManager

	// 测速结果缓存: server_id → latest result
	results map[int64]*SpeedTestResult

	// 配置
	interval      time.Duration // 自动测速间隔
	timeout       time.Duration // 单次测速超时
	samples       int           // 每次测速采样次数
	testPort      int           // 远端测速（保留，Agent 已不再探测端口）
	localTestPort int           // 本地测速目标端口（面板自身 web 端口）
}

// SpeedTestResult 存储测速结果。
type SpeedTestResult struct {
	ServerID   int64     `json:"server_id"`
	ServerName string    `json:"server_name"` // 节点显示名，供面板表格渲染
	LatencyMs  int32     `json:"latency_ms"`  // 平均延迟（毫秒）
	JitterMs   int32     `json:"jitter_ms"`   // 抖动（毫秒）
	PacketLoss float64   `json:"packet_loss"` // 丢包率（0-1）
	TestedAt   time.Time `json:"tested_at"`
	Error      string    `json:"error"` // 如果测速失败，记录错误
}

// NewSpeedTestManager creates a new SpeedTestManager.
func NewSpeedTestManager(serverRepo *repository.NodeServerRepository, nodeMgr *NodeServerManager) *SpeedTestManager {
	return &SpeedTestManager{
		serverRepo: serverRepo,
		nodeMgr:    nodeMgr,
		results:    make(map[int64]*SpeedTestResult),
		interval:      5 * time.Minute,
		timeout:       5 * time.Second,
		samples:       3,
		testPort:      443,
		localTestPort: 8080,
	}
}

// WithInterval sets the auto test interval.
func (m *SpeedTestManager) WithInterval(d time.Duration) *SpeedTestManager {
	m.interval = d
	return m
}

// WithTimeout sets the test timeout.
func (m *SpeedTestManager) WithTimeout(d time.Duration) *SpeedTestManager {
	m.timeout = d
	return m
}

// WithSamples sets the number of samples per test.
func (m *SpeedTestManager) WithSamples(n int) *SpeedTestManager {
	m.samples = n
	return m
}

// WithTestPort is retained for compatibility; remote tests now measure the heartbeat
// stream round-trip and no longer probe an arbitrary port.
func (m *SpeedTestManager) WithTestPort(port int) *SpeedTestManager {
	m.testPort = port
	return m
}

// TestServer tests latency to a specific server.
// For local servers: direct TCP connect to the server's public IP.
// For remote servers: gRPC call to Agent.SpeedTest.
func (m *SpeedTestManager) TestServer(ctx context.Context, serverID int64) (*SpeedTestResult, error) {
	server, err := m.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	var result *SpeedTestResult

	if m.nodeMgr.isLocalServer(serverID) {
		// Local server: direct TCP connect test
		result = m.testLocalServer(ctx, server.ID, server.Hostname)
	} else {
		// Remote server: gRPC call to Agent
		result = m.testRemoteServer(ctx, server.ID, server.Hostname)
	}

	// Attach the human-readable name for the panel UI (frontend renders server_name).
	result.ServerName = server.Name

	// Cache result
	m.mu.Lock()
	m.results[serverID] = result
	m.mu.Unlock()

	return result, nil
}

// TestAllServers tests latency to all enabled servers.
func (m *SpeedTestManager) TestAllServers(ctx context.Context) ([]SpeedTestResult, error) {
	servers, err := m.serverRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}

	var wg sync.WaitGroup
	results := make([]SpeedTestResult, 0, len(servers))
	var resultsMu sync.Mutex

	for _, server := range servers {
		if !server.Enabled {
			continue
		}

		wg.Add(1)
		go func(srvID int64) {
			defer wg.Done()
			result, err := m.TestServer(ctx, srvID)
			if err != nil {
				log.Printf("[SpeedTest] server %d: %v", srvID, err)
				return
			}
			resultsMu.Lock()
			results = append(results, *result)
			resultsMu.Unlock()
		}(server.ID)
	}

	wg.Wait()

	log.Printf("[SpeedTest] tested %d servers", len(results))
	return results, nil
}

// GetResult returns the latest speed test result for a server.
func (m *SpeedTestManager) GetResult(serverID int64) *SpeedTestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.results[serverID]
}

// GetAllResults returns all cached speed test results.
func (m *SpeedTestManager) GetAllResults() []SpeedTestResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]SpeedTestResult, 0, len(m.results))
	for _, r := range m.results {
		results = append(results, *r)
	}
	return results
}

// GetSortedServers returns server IDs sorted by latency (lowest first).
// Servers without test results or with errors are placed at the end.
func (m *SpeedTestManager) GetSortedServers() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type srvScore struct {
		id       int64
		latency  int32
		hasError bool
	}

	scores := make([]srvScore, 0, len(m.results))
	for id, r := range m.results {
		scores = append(scores, srvScore{
			id:       id,
			latency:  r.LatencyMs,
			hasError: r.Error != "",
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		// Servers with errors go last
		if scores[i].hasError != scores[j].hasError {
			return !scores[i].hasError
		}
		return scores[i].latency < scores[j].latency
	})

	ids := make([]int64, len(scores))
	for i, s := range scores {
		ids[i] = s.id
	}
	return ids
}

// RunLoop starts the periodic speed test loop. Blocks until ctx is cancelled.
func (m *SpeedTestManager) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	log.Printf("[SpeedTest] loop started, interval=%v", m.interval)

	// Run once shortly after startup
	time.AfterFunc(10*time.Second, func() {
		if _, err := m.TestAllServers(ctx); err != nil {
			log.Printf("[SpeedTest] initial test error: %v", err)
		}
	})

	for {
		select {
		case <-ctx.Done():
			log.Println("[SpeedTest] stopped")
			return
		case <-ticker.C:
			if _, err := m.TestAllServers(ctx); err != nil {
				log.Printf("[SpeedTest] test error: %v", err)
			}
		}
	}
}

// testLocalServer measures the loopback latency of the local (panel) node. The local
// node is the panel itself, so we probe 127.0.0.1 on the panel's own web port (always
// listening) instead of the stored hostname (which may be an unresolvable container name
// like "local-panel"). This avoids the broken :443 probe and yields a realistic ~0ms
// loopback latency for the local node.
func (m *SpeedTestManager) testLocalServer(ctx context.Context, serverID int64, hostname string) *SpeedTestResult {
	result := &SpeedTestResult{
		ServerID: serverID,
		TestedAt: time.Now(),
	}

	latencies := make([]int32, 0, m.samples)
	// 127.0.0.1 on the panel's own web port — guaranteed to be listening locally.
	target := fmt.Sprintf("127.0.0.1:%d", m.localTestPort)

	for i := 0; i < m.samples; i++ {
		start := time.Now()
		dialer := net.Dialer{Timeout: m.timeout}
		conn, err := dialer.DialContext(ctx, "tcp", target)
		if err != nil {
			result.Error = fmt.Sprintf("tcp connect %s: %v", target, err)
			return result
		}
		latency := time.Since(start).Milliseconds()
		conn.Close()
		latencies = append(latencies, int32(latency))

		if i < m.samples-1 {
			time.Sleep(200 * time.Millisecond) // 间隔 200ms
		}
	}

	result.LatencyMs = averageLatency(latencies)
	result.JitterMs = jitter(latencies)
	result.PacketLoss = 0

	log.Printf("[SpeedTest] server %d (%s, local): latency=%dms jitter=%dms",
		serverID, hostname, result.LatencyMs, result.JitterMs)

	return result
}

// testRemoteServer calls Agent.SpeedTest via stream.
func (m *SpeedTestManager) testRemoteServer(ctx context.Context, serverID int64, hostname string) *SpeedTestResult {
	result := &SpeedTestResult{
		ServerID: serverID,
		TestedAt: time.Now(),
	}

	ac := m.nodeMgr.GetConnection(serverID)
	if ac == nil {
		result.Error = "server not connected"
		return result
	}

	testCtx, cancel := context.WithTimeout(ctx, m.timeout*time.Duration(m.samples+2))
	defer cancel()

	resp, err := m.nodeMgr.SpeedTestOnServer(testCtx, serverID, hostname, int32(m.testPort), int32(m.timeout/time.Millisecond), int32(m.samples))
	if err != nil {
		result.Error = fmt.Sprintf("grpc SpeedTest: %v", err)
		return result
	}

	if !resp.Success {
		result.Error = resp.Error
		return result
	}

	result.LatencyMs = resp.LatencyMs
	result.JitterMs = resp.JitterMs
	result.PacketLoss = resp.PacketLoss

	log.Printf("[SpeedTest] server %d (%s): latency=%dms jitter=%dms loss=%.1f%%",
		serverID, hostname, result.LatencyMs, result.JitterMs, result.PacketLoss*100)

	return result
}

// averageLatency calculates the average latency from samples, excluding the first sample (warmup).
func averageLatency(latencies []int32) int32 {
	if len(latencies) <= 1 {
		return latencies[0]
	}
	// Skip first sample (TCP warmup)
	sum := int64(0)
	count := 0
	for i, l := range latencies {
		if i == 0 {
			continue
		}
		sum += int64(l)
		count++
	}
	return int32(sum / int64(count))
}

// jitter calculates the standard deviation of latencies.
func jitter(latencies []int32) int32 {
	if len(latencies) <= 2 {
		return 0
	}
	avg := averageLatency(latencies)
	sumSq := float64(0)
	count := 0
	for i, l := range latencies {
		if i == 0 {
			continue
		}
		diff := float64(l) - float64(avg)
		sumSq += diff * diff
		count++
	}
	return int32(math.Sqrt(sumSq / float64(count)))
}
