package xray

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"corefusion/internal/coredata"
	"corefusion/internal/domain"
	"corefusion/internal/domain/metric"
)

// TrafficCollector collects traffic stats from Xray-core gRPC Stats API.
type TrafficCollector struct {
	serviceID   int64
	statsAddr   string
	client      *coredata.XrayClient
	subscribers []chan<- metric.TrafficDeltaEvent
	lastValues  map[string]int64
	mu          sync.RWMutex
	pollInterval time.Duration
}

// NewTrafficCollector creates a new Xray traffic collector.
func NewTrafficCollector(serviceID int64, statsAddr string) *TrafficCollector {
	return &TrafficCollector{
		serviceID:    serviceID,
		statsAddr:    statsAddr,
		lastValues:   make(map[string]int64),
		pollInterval: 30 * time.Second,
	}
}

// CoreType returns the core type.
func (c *TrafficCollector) CoreType() domain.CoreType {
	return domain.CoreTypeXray
}

// Start starts the traffic collector.
func (c *TrafficCollector) Start(ctx context.Context, in metric.CollectorStartInput) error {
	c.statsAddr = in.RuntimeMeta["stats_addr"]
	if c.statsAddr == "" {
		c.statsAddr = coredata.DefaultStatsAddr
	}
	c.client = coredata.NewXrayClient(c.statsAddr)
	go c.pollLoop(ctx)
	return nil
}

// Stop stops the traffic collector.
func (c *TrafficCollector) Stop(ctx context.Context) error {
	if c.client != nil {
		c.client.Close()
	}
	return nil
}

// Snapshot returns a point-in-time traffic snapshot.
func (c *TrafficCollector) Snapshot(ctx context.Context) (*metric.CollectorSnapshot, error) {
	return &metric.CollectorSnapshot{
		ServiceID:   c.serviceID,
		CollectedAt: time.Now(),
	}, nil
}

// Subscribe subscribes to traffic delta events.
func (c *TrafficCollector) Subscribe(ctx context.Context, ch chan<- metric.TrafficDeltaEvent) error {
	c.mu.Lock()
	c.subscribers = append(c.subscribers, ch)
	c.mu.Unlock()
	return nil
}

func (c *TrafficCollector) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *TrafficCollector) collect(ctx context.Context) {
	// In real implementation, this would call Xray gRPC Stats API:
	// client.QueryStats(ctx, &stats.QueryStatsRequest{
	//     Pattern: "user>>>*>>>traffic>>>*",
	//     Reset_:  true,
	// })

	// Parse stats format: "user>>>email>>>traffic>>>uplink"
	// Calculate delta from lastValues
	// Emit events to subscribers

	// Example parsing logic:
	stats := c.queryStats(ctx)
	for key, value := range stats {
		parts := strings.Split(key, ">>>")
		if len(parts) != 4 {
			continue
		}

		email := parts[1]
		direction := parts[3]

		c.mu.RLock()
		lastValue := c.lastValues[key]
		c.mu.RUnlock()

		delta := value - lastValue
		if delta <= 0 {
			continue
		}

		c.mu.Lock()
		c.lastValues[key] = value
		c.mu.Unlock()

		event := metric.TrafficDeltaEvent{
			SessionID:       fmt.Sprintf("xray-%s-%d", email, time.Now().UnixNano()),
			UserID:          0, // Resolved by ownership resolver
			SubscriptionID:  0,
			ServiceID:       c.serviceID,
			CoreType:        domain.CoreTypeXray,
			SourceCollector: "xray_stats_api",
			Sequence:        time.Now().UnixNano(),
			CollectedAt:     time.Now(),
		}

		switch direction {
		case "uplink":
			event.UplinkBytes = delta
		case "downlink":
			event.DownlinkBytes = delta
		}

		c.emit(event)
	}
}

func (c *TrafficCollector) queryStats(ctx context.Context) map[string]int64 {
	if c.client == nil {
		return nil
	}
	// 增量采集：不 reset，依靠 collect 维护的 lastValues 计算 delta。
	resp, err := c.client.RawStats(ctx, false)
	if err != nil {
		return nil
	}
	stats := make(map[string]int64, len(resp.GetStat()))
	for _, stat := range resp.GetStat() {
		stats[stat.GetName()] = stat.GetValue()
	}
	return stats
}

func (c *TrafficCollector) emit(event metric.TrafficDeltaEvent) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, ch := range c.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, drop event
		}
	}
}

// UserTrafficDelta represents traffic delta for a single user.
type UserTrafficDelta struct {
	UserIdentifier string
	UplinkBytes    int64
	DownlinkBytes  int64
}
