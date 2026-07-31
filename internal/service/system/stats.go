package system

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"corefusion/internal/model/entity"
)

var (
	panelStartTime time.Time
	initOnce       sync.Once

	// 面板进程自身 CPU 采样状态（跨请求持久化，用于计算进程级 CPU 占用）
	procCPUMu       sync.Mutex
	procPrevCPUTick int64
	procPrevCPUTS   time.Time
)

// InitStartTime records the panel start time (call once at startup).
func InitStartTime() {
	initOnce.Do(func() {
		panelStartTime = time.Now()
	})
}

// SystemStats contains system resource information.
type SystemStats struct {
	// Version
	Version string `json:"version"`

	// CPU
	CPUUsage float64 `json:"cpu_usage"`
	CPUCores int     `json:"cpu_cores"`

	// Memory
	MemTotal int64   `json:"mem_total"`
	MemUsed  int64   `json:"mem_used"`
	MemFree  int64   `json:"mem_free"`
	MemUsage float64 `json:"mem_usage"`

	// Swap
	SwapTotal int64   `json:"swap_total"`
	SwapUsed  int64   `json:"swap_used"`
	SwapFree  int64   `json:"swap_free"`
	SwapUsage float64 `json:"swap_usage"`

	// Disk
	DiskTotal int64   `json:"disk_total"`
	DiskUsed  int64   `json:"disk_used"`
	DiskFree  int64   `json:"disk_free"`
	DiskUsage float64 `json:"disk_usage"`

	// System
	Hostname  string  `json:"hostname"`
	OS        string  `json:"os"`
	Arch      string  `json:"arch"`
	Uptime    int64   `json:"uptime"`     // system uptime in seconds
	PanelUptime int64 `json:"panel_uptime"` // panel uptime in seconds
	LoadAvg1  float64 `json:"load_avg_1"`
	LoadAvg5  float64 `json:"load_avg_5"`
	LoadAvg15 float64 `json:"load_avg_15"`

	// Network
	TCPConnections int   `json:"tcp_connections"`
	UDPConnections int   `json:"udp_connections"`
	NetBytesSent   int64 `json:"net_bytes_sent"`
	NetBytesRecv   int64 `json:"net_bytes_recv"`

	// Runtime
	Goroutines int `json:"goroutines"`

	// 面板进程自身资源占用（区别于上面的主机级指标）
	PanelCPUUsage float64 `json:"panel_cpu_usage"` // 面板进程 CPU 占用（基于 /proc/self/stat，相对单核，多核可超过 100%）
	PanelMemUsage float64 `json:"panel_mem_usage"` // 进程 RSS 占机器总内存百分比
	PanelMemBytes int64   `json:"panel_mem_bytes"` // 进程 RSS 字节数

	// Time
	CurrentTime time.Time `json:"current_time"`
}

// StatsService provides system statistics.
type StatsService struct {
	version string
}

// NewStatsService creates a new stats service.
func NewStatsService(version string) *StatsService {
	return &StatsService{version: version}
}

// GetStats returns current system statistics.
func (s *StatsService) GetStats() *SystemStats {
	stats := &SystemStats{
		Version:     s.version,
		CPUCores:    runtime.NumCPU(),
		Hostname:    getHostname(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CurrentTime: time.Now(),
	}

	// Panel uptime
	if !panelStartTime.IsZero() {
		stats.PanelUptime = int64(time.Since(panelStartTime).Seconds())
	}

	// Goroutines
	stats.Goroutines = runtime.NumGoroutine()

	s.readCPU(stats)
	s.readMemory(stats)
	s.readDisk(stats)
	s.readLoadAvg(stats)
	s.readNetwork(stats)
	s.readConnections(stats)
	s.readUptime(stats)
	s.readProcessCPU(stats)
	s.readProcessMemory(stats)

	return stats
}

// CollectServerMetric 将当前面板自身（本地节点）的主机资源映射为 *entity.ServerMetric，
// 供 MetricsCollector 落库。网络字段使用累计计数器（NetBytesRecv/Sent），
// 速率由 MetricsCollector 基于两次采样差分计算。
func (s *StatsService) CollectServerMetric(serverID int64) *entity.ServerMetric {
	st := s.GetStats()
	return &entity.ServerMetric{
		ServerID:        serverID,
		CPUUsage:        st.CPUUsage,
		CPUCores:        st.CPUCores,
		MemUsageBytes:   st.MemUsed,
		MemTotalBytes:   st.MemTotal,
		MemUsage:        st.MemUsage,
		SwapTotalBytes:  st.SwapTotal,
		SwapUsedBytes:   st.SwapUsed,
		SwapUsage:       st.SwapUsage,
		DiskUsageBytes:  st.DiskUsed,
		DiskTotalBytes:  st.DiskTotal,
		DiskUsage:       st.DiskUsage,
		Load1m:          st.LoadAvg1,
		TCPConnections:  st.TCPConnections,
		UptimeSeconds:   st.Uptime,
		Hostname:        st.Hostname,
		NetRxBytes:      st.NetBytesRecv,
		NetTxBytes:      st.NetBytesSent,
	}
}

func (s *StatsService) readCPU(stats *SystemStats) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				user, _ := strconv.ParseInt(fields[1], 10, 64)
				system, _ := strconv.ParseInt(fields[3], 10, 64)
				idle, _ := strconv.ParseInt(fields[4], 10, 64)
				total := user + system + idle
				if total > 0 {
					stats.CPUUsage = float64(user+system) / float64(total) * 100
				}
			}
			break
		}
	}
}

func (s *StatsService) readMemory(stats *SystemStats) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	info := make(map[string]int64)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			val, _ := strconv.ParseInt(parts[1], 10, 64)
			info[key] = val * 1024
		}
	}
	stats.MemTotal = info["MemTotal"]
	stats.MemFree = info["MemAvailable"]
	stats.MemUsed = stats.MemTotal - stats.MemFree
	if stats.MemTotal > 0 {
		stats.MemUsage = float64(stats.MemUsed) / float64(stats.MemTotal) * 100
	}
	stats.SwapTotal = info["SwapTotal"]
	stats.SwapFree = info["SwapFree"]
	stats.SwapUsed = stats.SwapTotal - stats.SwapFree
	if stats.SwapTotal > 0 {
		stats.SwapUsage = float64(stats.SwapUsed) / float64(stats.SwapTotal) * 100
	}
}

// readProcessCPU 计算面板进程自身的 CPU 占用（基于 /proc/self/stat 的 utime+stime 两次采样差分）。
// 注意：这是进程级指标，与 readCPU 的主机级指标不同，前端面板卡片应使用本结果而非 CPUUsage。
func (s *StatsService) readProcessCPU(stats *SystemStats) {
	// /proc/self/stat：第2字段为 comm（括号包裹，可能含空格），其后第14=utime、第15=stime（时钟滴答）
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return
	}
	line := string(data)
	idx := strings.LastIndex(line, ")")
	if idx < 0 {
		return
	}
	fields := strings.Fields(line[idx+1:])
	if len(fields) < 13 {
		return
	}
	utime, _ := strconv.ParseInt(fields[11], 10, 64)
	stime, _ := strconv.ParseInt(fields[12], 10, 64)
	cur := utime + stime

	// Linux 用户态时钟频率（USER_HZ）固定为 100，等价于 sysconf(_SC_CLK_TCK)
	const clkTck = 100.0
	now := time.Now()

	procCPUMu.Lock()
	defer procCPUMu.Unlock()
	if !procPrevCPUTS.IsZero() {
		elapsed := now.Sub(procPrevCPUTS).Seconds()
		if elapsed > 0 {
			dt := float64(cur - procPrevCPUTick)
			if dt >= 0 {
				stats.PanelCPUUsage = dt / (elapsed * clkTck) * 100
			}
		}
	}
	procPrevCPUTick = cur
	procPrevCPUTS = now
}

// readProcessMemory 读取面板进程自身的常驻内存（RSS），来自 /proc/self/statm 第二个字段。
func (s *StatsService) readProcessMemory(stats *SystemStats) {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return
	}
	pageSize := int64(syscall.Getpagesize())
	if pageSize <= 0 {
		pageSize = 4096
	}
	rssPages, _ := strconv.ParseInt(fields[1], 10, 64)
	rss := rssPages * pageSize
	stats.PanelMemBytes = rss
	if stats.MemTotal > 0 {
		stats.PanelMemUsage = float64(rss) / float64(stats.MemTotal) * 100
	}
}

func (s *StatsService) readDisk(stats *SystemStats) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return
	}
	stats.DiskTotal = int64(stat.Blocks) * int64(stat.Bsize)
	stats.DiskFree = int64(stat.Bavail) * int64(stat.Bsize)
	stats.DiskUsed = stats.DiskTotal - stats.DiskFree
	if stats.DiskTotal > 0 {
		stats.DiskUsage = float64(stats.DiskUsed) / float64(stats.DiskTotal) * 100
	}
}

func (s *StatsService) readLoadAvg(stats *SystemStats) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		stats.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
		stats.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
		stats.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
	}
}

func (s *StatsService) readNetwork(stats *SystemStats) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, ":") && !strings.Contains(line, "lo:") {
			parts := strings.Fields(strings.Replace(line, ":", " ", 1))
			if len(parts) >= 10 {
				recv, _ := strconv.ParseInt(parts[1], 10, 64)
				sent, _ := strconv.ParseInt(parts[9], 10, 64)
				stats.NetBytesRecv += recv
				stats.NetBytesSent += sent
			}
		}
	}
}

func (s *StatsService) readConnections(stats *SystemStats) {
	data, err := os.ReadFile("/proc/net/tcp")
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		first := true
		count := 0
		for scanner.Scan() {
			if first {
				first = false
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 && fields[3] == "01" {
				count++
			}
		}
		stats.TCPConnections = count
	}
	data, err = os.ReadFile("/proc/net/tcp6")
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		first := true
		for scanner.Scan() {
			if first {
				first = false
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 && fields[3] == "01" {
				stats.TCPConnections++
			}
		}
	}
	data, err = os.ReadFile("/proc/net/udp")
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		first := true
		count := 0
		for scanner.Scan() {
			if first {
				first = false
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 && fields[3] == "07" {
				count++
			}
		}
		stats.UDPConnections = count
	}
}

func (s *StatsService) readUptime(stats *SystemStats) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		uptime, _ := strconv.ParseFloat(fields[0], 64)
		stats.Uptime = int64(uptime)
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
