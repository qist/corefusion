package main

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// getSystemMetrics returns the current CPU usage percentage and memory usage in bytes.
func getSystemMetrics() (cpuUsage float64, memUsageBytes int64) {
	if runtime.GOOS == "linux" {
		memUsageBytes = getLinuxMemUsage()
	}
	cpuUsage = getLinuxCPUUsage()
	return
}

// getLinuxMemUsage reads /proc/meminfo and returns used memory in bytes.
func getLinuxMemUsage() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var memTotal, memAvailable int64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMemInfoLine(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = parseMemInfoLine(line)
		}
	}
	if memTotal > 0 {
		return (memTotal - memAvailable) * 1024
	}
	return 0
}

// parseMemInfoLine extracts the numeric value (in kB) from a /proc/meminfo line.
func parseMemInfoLine(line string) int64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	val, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// getLinuxCPUUsage reads /proc/loadavg and returns an approximate CPU usage percentage.
func getLinuxCPUUsage() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return 0
	}
	load1, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	ncpu := runtime.NumCPU()
	if ncpu == 0 {
		ncpu = 1
	}
	usage := (load1 / float64(ncpu)) * 100
	if usage > 100 {
		usage = 100
	}
	return usage
}
