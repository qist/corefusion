package main

import (
	"context"
	"fmt"
	"log"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/coredata"
)

// trafficCollector periodically queries the local xray gRPC Stats API
// and returns traffic deltas for upstream reporting.
//
// Strategy (reset_=true, no lastValues needed):
//   - First query: QueryStats(reset_=true) — resets xray counters to 0.
//     The returned values are OLD accumulated traffic, so we discard them.
//     This loses at most one polling interval (30s) of traffic, once on startup.
//   - Subsequent queries: QueryStats(reset_=true) — each value IS the delta
//     since last query. No baseline needed, no first-sample skip per user.
//
// This approach:
//   - Never double-counts (reset ensures each byte is returned exactly once)
//   - Never loses traffic after the initial reset (no per-user baseline skip)
//   - Works correctly across xray restarts (counter starts at 0)
type trafficCollector struct {
	client       *coredata.XrayClient
	pollInterval time.Duration
	initialized  bool // false until first reset query completes
}

// newTrafficCollector creates a traffic collector for the local xray Stats API.
func newTrafficCollector(statsAddr string) *trafficCollector {
	if statsAddr == "" {
		statsAddr = coredata.DefaultStatsAddr
	}
	return &trafficCollector{
		client:       coredata.NewXrayClient(statsAddr),
		pollInterval: 30 * time.Second,
	}
}

// collect queries xray Stats API and returns traffic deltas as TrafficStat list.
// Returns nil stats (no error) when there are no deltas to report.
func (tc *trafficCollector) collect(ctx context.Context) ([]*agentpb.TrafficStat, error) {
	// First call: reset xray counters to 0.
	// Discard the returned values (they are old accumulated traffic).
	// This ensures subsequent queries return only new traffic.
	if !tc.initialized {
		if _, err := tc.client.RawStats(ctx, true); err != nil {
			return nil, fmt.Errorf("initial reset query stats: %w", err)
		}
		tc.initialized = true
		return nil, nil // baseline reset done, next call gets real deltas
	}

	// Subsequent calls: reset_=true so xray returns deltas and resets counters.
	resp, err := tc.client.RawStats(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	// 复用公共解析，仅取 user 方向；增量语义（value<=0 跳过）由调用方保留。
	sr, err := coredata.ParseStatsResponse(resp)
	if err != nil {
		return nil, err
	}

	stats := make([]*agentpb.TrafficStat, 0, len(sr.Users))
	for _, u := range sr.Users {
		// 增量语义：任一方向 <=0 视为复位/异常，跳过（与原行为一致）。
		if u.Uplink <= 0 || u.Downlink <= 0 {
			continue
		}
		stats = append(stats, &agentpb.TrafficStat{
			UserEmail:     u.Email,
			UplinkBytes:   u.Uplink,
			DownlinkBytes: u.Downlink,
			CoreType:      "xray",
		})
	}

	return stats, nil
}

// resolveStatsAddr reads the xray stats API address from local settings,
// falling back to the default.
func resolveStatsAddr(ctx context.Context, settingRepo *agentSettingStore) string {
	if settingRepo != nil {
		val, err := settingRepo.GetByKey(ctx, "xray.stats_addr")
		if err == nil && val != "" {
			return val
		}
	}
	return "127.0.0.1:10085"
}

// sendTrafficReport collects traffic deltas from the local xray Stats API
// and sends them to the Panel via the heartbeat stream as a TRAFFIC report.
func (a *AgentApp) sendTrafficReport(stream agentpb.HeartbeatService_StreamClient) error {
	if a.trafficCollector == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats, err := a.trafficCollector.collect(ctx)
	if err != nil {
		return err
	}

	if len(stats) == 0 {
		return nil // nothing to report
	}

	a.streamSendMu.Lock()
	defer a.streamSendMu.Unlock()

	err = stream.Send(&agentpb.AgentReport{
		Type:         agentpb.AgentReport_TRAFFIC,
		ServerId:     a.cfg.Panel.ServerID,
		Timestamp:    time.Now().Unix(),
		TrafficStats: stats,
	})
	if err != nil {
		return fmt.Errorf("send traffic report: %w", err)
	}

	log.Printf("[Agent] traffic report sent: %d users", len(stats))
	return nil
}
