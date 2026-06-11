package tuic

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"corefusion/internal/domain"
	"corefusion/internal/domain/metric"
)

// TrafficCollector collects traffic stats for TUIC.
// TUIC has limited management API, so this uses /proc/<pid>/net/dev as fallback.
type TrafficCollector struct {
	serviceID  int64
	pid        int
	lastBytes  int64
	subscribers []chan<- metric.TrafficDeltaEvent
	mu         sync.RWMutex
}

func NewTrafficCollector(serviceID int64, pid int) *TrafficCollector {
	return &TrafficCollector{
		serviceID: serviceID,
		pid:       pid,
	}
}

func (c *TrafficCollector) CoreType() domain.CoreType { return domain.CoreTypeTUIC }

func (c *TrafficCollector) Start(ctx context.Context, in metric.CollectorStartInput) error {
	c.pid = 0 // Will be set when process starts
	go c.pollLoop(ctx)
	return nil
}

func (c *TrafficCollector) Stop(ctx context.Context) error { return nil }

func (c *TrafficCollector) Snapshot(ctx context.Context) (*metric.CollectorSnapshot, error) {
	return &metric.CollectorSnapshot{ServiceID: c.serviceID, CollectedAt: time.Now()}, nil
}

func (c *TrafficCollector) Subscribe(ctx context.Context, ch chan<- metric.TrafficDeltaEvent) error {
	c.mu.Lock()
	c.subscribers = append(c.subscribers, ch)
	c.mu.Unlock()
	return nil
}

func (c *TrafficCollector) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

func (c *TrafficCollector) collect() {
	if c.pid <= 0 {
		return
	}

	totalBytes := c.readProcNetDev()
	if totalBytes == 0 {
		return
	}

	c.mu.Lock()
	delta := totalBytes - c.lastBytes
	c.lastBytes = totalBytes
	c.mu.Unlock()

	if delta <= 0 {
		return
	}

	// TUIC is process-level only, can't distinguish users
	event := metric.TrafficDeltaEvent{
		SessionID:      fmt.Sprintf("tuic-proc-%d-%d", c.serviceID, time.Now().UnixNano()),
		ServiceID:      c.serviceID,
		CoreType:       domain.CoreTypeTUIC,
		Protocol:       domain.ProtocolTUIC,
		UplinkBytes:    delta / 2, // Can't distinguish up/down, split evenly
		DownlinkBytes:  delta / 2,
		Sequence:       time.Now().UnixNano(),
		SourceCollector: "tuic_proc_net_dev",
		CollectedAt:    time.Now(),
	}

	c.mu.RLock()
	for _, ch := range c.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	c.mu.RUnlock()
}

func (c *TrafficCollector) readProcNetDev() int64 {
	path := fmt.Sprintf("/proc/%d/net/dev", c.pid)
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	var totalBytes int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 || !strings.Contains(fields[0], ":") {
			continue
		}
		// Skip loopback
		if strings.HasPrefix(fields[0], "lo:") {
			continue
		}
		// fields[1] = rx bytes, fields[9] = tx bytes
		rx, _ := strconv.ParseInt(fields[1], 10, 64)
		tx, _ := strconv.ParseInt(fields[9], 10, 64)
		totalBytes += rx + tx
	}
	return totalBytes
}
