package main

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SystemMetrics holds comprehensive system resource metrics collected by the Agent.
type SystemMetrics struct {
	// CPU
	CPUUsage float64 // CPU usage percentage (0-100), from /proc/stat
	CPUCores int     // Number of logical CPU cores

	// Memory
	MemTotalBytes int64   // Total physical memory in bytes
	MemUsedBytes  int64   // Used memory in bytes
	MemUsage      float64 // Memory usage percentage (0-100)

	// Swap
	SwapTotalBytes int64   // Total swap in bytes
	SwapUsedBytes  int64   // Used swap in bytes
	SwapUsage      float64 // Swap usage percentage (0-100)

	// Disk
	DiskTotalBytes int64   // Total disk space in bytes (data partition)
	DiskUsedBytes  int64   // Used disk space in bytes
	DiskUsage      float64 // Disk usage percentage (0-100)

	// Disk I/O (rate since last sample)
	DiskReadBytesSec  int64 // Disk read rate in bytes/sec
	DiskWriteBytesSec int64 // Disk write rate in bytes/sec

	// Network (rate since last sample)
	NetRxBytesSec int64 // Network receive rate in bytes/sec
	NetTxBytesSec int64 // Network transmit rate in bytes/sec

	// Network (cumulative counters for delta calculation)
	NetRxBytesTotal int64 // Cumulative received bytes
	NetTxBytesTotal int64 // Cumulative transmitted bytes

	// Egress interface traffic (出口网卡累计流量，用于月度配额)
	EgressInterface string // 出口网卡名称
	EgressRxBytes   int64 // 出口网卡累计接收字节
	EgressTxBytes   int64 // 出口网卡累计发送字节

	// System
	Hostname       string  // System hostname
	KernelVersion  string  // Kernel version string
	UptimeSeconds  int64   // System uptime in seconds
	Load1m         float64 // 1-minute load average
	TCPConnections int     // Number of TCP ESTABLISHED connections

	// Timestamps for rate calculation
	lastNetStats   *netCounter
	lastDiskStats  *diskCounter
}

// egressInterface holds the configured egress interface name for traffic quota monitoring.
// Set by Panel via SetEgressInterface command.
var egressInterface string

// SetEgressInterface sets the egress interface name to monitor.
func SetEgressInterface(iface string) {
	egressInterface = iface
}

// GetEgressInterface returns the currently configured egress interface name.
func GetEgressInterface() string {
	return egressInterface
}

type netCounter struct {
	rxBytes   int64
	txBytes   int64
	timestamp time.Time
}

type diskCounter struct {
	readSectors  int64
	writeSectors int64
	timestamp   time.Time
}

// getSystemMetrics collects comprehensive system metrics.
// On non-Linux systems, only CPU cores and basic info are available.
func getSystemMetrics() *SystemMetrics {
	m := &SystemMetrics{
		CPUCores: runtime.NumCPU(),
	}

	if runtime.GOOS != "linux" {
		return m
	}

	m.readCPUUsage()
	m.readMemoryInfo()
	m.readDiskInfo()
	m.readNetworkCounters()
	m.readDiskIOCounters()
	m.readLoadAvg()
	m.readTCPConnections()
	m.readUptime()
	m.readHostname()

	return m
}

// ──────────────────────────────────────────────────────
// CPU Usage — read from /proc/stat (real utilization)
// ──────────────────────────────────────────────────────

var lastCPUStats *cpuStats

type cpuStats struct {
	user    int64
	nice    int64
	system  int64
	idle    int64
	iowait  int64
	steal   int64
	guest   int64
	guestNi int64
}

func (m *SystemMetrics) readCPUUsage() {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			break
		}
		current := &cpuStats{}
		current.user, _ = strconv.ParseInt(fields[1], 10, 64)
		current.nice, _ = strconv.ParseInt(fields[2], 10, 64)
		current.system, _ = strconv.ParseInt(fields[3], 10, 64)
		current.idle, _ = strconv.ParseInt(fields[4], 10, 64)
		if len(fields) > 5 {
			current.iowait, _ = strconv.ParseInt(fields[5], 10, 64)
		}
		if len(fields) > 6 {
			current.steal, _ = strconv.ParseInt(fields[6], 10, 64)
		}
		if len(fields) > 7 {
			current.guest, _ = strconv.ParseInt(fields[7], 10, 64)
		}
		if len(fields) > 8 {
			current.guestNi, _ = strconv.ParseInt(fields[8], 10, 64)
		}

		if lastCPUStats != nil {
			// Delta calculation
			dUser := current.user - lastCPUStats.user
			dNice := current.nice - lastCPUStats.nice
			dSystem := current.system - lastCPUStats.system
			dIdle := current.idle - lastCPUStats.idle
			dIowait := current.iowait - lastCPUStats.iowait
			dSteal := current.steal - lastCPUStats.steal
			dGuest := current.guest - lastCPUStats.guest
			dGuestNi := current.guestNi - lastCPUStats.guestNi

			// guest/guest-nice are already accounted in user/nice
			dUser -= dGuest
			dNice -= dGuestNi

			total := dUser + dNice + dSystem + dIdle + dIowait + dSteal
			if total > 0 {
				m.CPUUsage = float64(total-dIdle-dIowait) / float64(total) * 100
			}
		}
		lastCPUStats = current
		break
	}
}

// ──────────────────────────────────────────────────────
// Memory — read from /proc/meminfo
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readMemoryInfo() {
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
			info[key] = val * 1024 // kB → bytes
		}
	}

	m.MemTotalBytes = info["MemTotal"]
	memAvailable := info["MemAvailable"]
	m.MemUsedBytes = m.MemTotalBytes - memAvailable
	if m.MemTotalBytes > 0 {
		m.MemUsage = float64(m.MemUsedBytes) / float64(m.MemTotalBytes) * 100
	}

	m.SwapTotalBytes = info["SwapTotal"]
	swapFree := info["SwapFree"]
	m.SwapUsedBytes = m.SwapTotalBytes - swapFree
	if m.SwapTotalBytes > 0 {
		m.SwapUsage = float64(m.SwapUsedBytes) / float64(m.SwapTotalBytes) * 100
	}
}

// ──────────────────────────────────────────────────────
// Disk — read from statfs
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readDiskInfo() {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return
	}
	m.DiskTotalBytes = int64(stat.Blocks) * int64(stat.Bsize)
	m.DiskUsedBytes = (int64(stat.Blocks) - int64(stat.Bavail)) * int64(stat.Bsize)
	if m.DiskTotalBytes > 0 {
		m.DiskUsage = float64(m.DiskUsedBytes) / float64(m.DiskTotalBytes) * 100
	}
}

// ──────────────────────────────────────────────────────
// Network — cumulative counters + rate calculation
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readNetworkCounters() {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return
	}
	var totalRx, totalTx int64
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		// Skip loopback
		if strings.Contains(line, "lo:") {
			continue
		}
		parts := strings.Fields(strings.Replace(line, ":", " ", 1))
		if len(parts) >= 10 {
			iface := strings.TrimSpace(parts[0])
			recv, _ := strconv.ParseInt(parts[1], 10, 64)
			sent, _ := strconv.ParseInt(parts[9], 10, 64)
			totalRx += recv
			totalTx += sent

			// If this is the egress interface, record its counters
			if egressInterface != "" && iface == egressInterface {
				m.EgressRxBytes = recv
				m.EgressTxBytes = sent
			}
		}
	}

	m.NetRxBytesTotal = totalRx
	m.NetTxBytesTotal = totalTx

	// Calculate rate
	now := time.Now()
	if m.lastNetStats != nil {
		elapsed := now.Sub(m.lastNetStats.timestamp).Seconds()
		if elapsed > 0 {
			m.NetRxBytesSec = int64(float64(totalRx-m.lastNetStats.rxBytes) / elapsed)
			m.NetTxBytesSec = int64(float64(totalTx-m.lastNetStats.txBytes) / elapsed)
		}
	}
	m.lastNetStats = &netCounter{
		rxBytes:   totalRx,
		txBytes:   totalTx,
		timestamp: now,
	}

	// Record egress interface name
	m.EgressInterface = egressInterface
}

// ──────────────────────────────────────────────────────
// Disk I/O — from /proc/diskstats
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readDiskIOCounters() {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return
	}

	// Find the primary disk (first non-partition, non-loop entry)
	var readSectors, writeSectors int64
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		deviceName := fields[2]

		// Skip partitions (e.g., sda1, nvme0n1p1) and loop devices
		if strings.HasPrefix(deviceName, "loop") {
			continue
		}
		// Skip ram devices
		if strings.HasPrefix(deviceName, "ram") {
			continue
		}
		// Skip dm- devices (LVM)
		if strings.HasPrefix(deviceName, "dm-") {
			continue
		}

		// Check if this looks like a partition
		// Partitions of sdX have names like sda1, sda2
		// Partitions of nvme have names like nvme0n1p1
		if isPartition(deviceName) {
			continue
		}

		// This is a primary disk — use its stats
		readSec, _ := strconv.ParseInt(fields[5], 10, 64)  // sectors read
		writeSec, _ := strconv.ParseInt(fields[9], 10, 64)  // sectors written
		readSectors += readSec
		writeSectors += writeSec
	}

	// Calculate rate
	now := time.Now()
	if m.lastDiskStats != nil {
		elapsed := now.Sub(m.lastDiskStats.timestamp).Seconds()
		if elapsed > 0 {
			sectorSize := int64(512) // standard sector size
			dReadSec := readSectors - m.lastDiskStats.readSectors
			dWriteSec := writeSectors - m.lastDiskStats.writeSectors
			m.DiskReadBytesSec = int64(float64(dReadSec*sectorSize) / elapsed)
			m.DiskWriteBytesSec = int64(float64(dWriteSec*sectorSize) / elapsed)
		}
	}
	m.lastDiskStats = &diskCounter{
		readSectors:  readSectors,
		writeSectors: writeSectors,
		timestamp:    now,
	}
}

// isPartition returns true if the device name looks like a partition
func isPartition(name string) bool {
	// sdX partitions: sda1, sdb2, etc.
	if len(name) > 3 && name[0] == 's' && name[1] == 'd' {
		if name[2] >= 'a' && name[2] <= 'z' && len(name) > 3 {
			_, err := strconv.Atoi(name[3:])
			return err == nil
		}
	}
	// nvme partitions: nvme0n1p1, nvme1n2p3, etc.
	if strings.HasPrefix(name, "nvme") && strings.Contains(name, "p") {
		pIdx := strings.LastIndex(name, "p")
		if pIdx > 0 {
			_, err := strconv.Atoi(name[pIdx+1:])
			return err == nil
		}
	}
	// virtio partitions: vda1, vdb2, etc.
	if len(name) > 3 && name[0] == 'v' && name[1] == 'd' {
		if name[2] >= 'a' && name[2] <= 'z' && len(name) > 3 {
			_, err := strconv.Atoi(name[3:])
			return err == nil
		}
	}
	return false
}

// ──────────────────────────────────────────────────────
// Load Average — from /proc/loadavg
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readLoadAvg() {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		m.Load1m, _ = strconv.ParseFloat(fields[0], 64)
	}
}

// ──────────────────────────────────────────────────────
// TCP Connections — count ESTABLISHED from /proc/net/tcp
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readTCPConnections() {
	count := 0
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		first := true
		for scanner.Scan() {
			if first {
				first = false
				continue // skip header
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 && fields[3] == "01" { // 01 = ESTABLISHED
				count++
			}
		}
	}
	m.TCPConnections = count
}

// ──────────────────────────────────────────────────────
// Uptime — from /proc/uptime
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readUptime() {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 1 {
		uptime, _ := strconv.ParseFloat(fields[0], 64)
		m.UptimeSeconds = int64(uptime)
	}
}

// ──────────────────────────────────────────────────────
// Hostname
// ──────────────────────────────────────────────────────

func (m *SystemMetrics) readHostname() {
	hostname, err := os.Hostname()
	if err != nil {
		m.Hostname = "unknown"
	} else {
		m.Hostname = hostname
	}
}
