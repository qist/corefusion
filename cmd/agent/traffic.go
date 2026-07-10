package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"

	agentpb "corefusion/internal/runtime/agent/proto"
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
	statsAddr    string
	pollInterval time.Duration
	initialized  bool // false until first reset query completes

	conn   *grpc.ClientConn
	connMu sync.Mutex
}

// newTrafficCollector creates a traffic collector for the local xray Stats API.
func newTrafficCollector(statsAddr string) *trafficCollector {
	if statsAddr == "" {
		statsAddr = "127.0.0.1:10085"
	}
	return &trafficCollector{
		statsAddr:    statsAddr,
		pollInterval: 30 * time.Second,
	}
}

// collect queries xray Stats API and returns traffic deltas as TrafficStat list.
// Returns nil stats (no error) when there are no deltas to report.
func (tc *trafficCollector) collect(ctx context.Context) ([]*agentpb.TrafficStat, error) {
	conn, err := tc.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to xray stats: %w", err)
	}

	client := statsService.NewStatsServiceClient(conn)

	// First call: reset xray counters to 0.
	// Discard the returned values (they are old accumulated traffic).
	// This ensures subsequent queries return only new traffic.
	if !tc.initialized {
		_, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: true})
		if err != nil {
			return nil, fmt.Errorf("initial reset query stats: %w", err)
		}
		tc.initialized = true
		return nil, nil // baseline reset done, next call gets real deltas
	}

	// Subsequent calls: reset_=true so xray returns deltas and resets counters.
	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: true})
	if err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	// Parse user traffic deltas, keyed by email
	type userDelta struct {
		up   int64
		down int64
	}
	deltas := make(map[string]*userDelta)

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		value := stat.GetValue()

		// Only interested in user traffic stats: user>>>email>>>traffic>>>uplink/downlink
		if !strings.Contains(name, "user>>>") || !strings.Contains(name, ">>>traffic>>>") {
			continue
		}

		parts := strings.Split(name, ">>>")
		if len(parts) != 4 {
			continue
		}

		email := parts[1]
		direction := parts[3]

		// With reset_=true, value IS the delta since last query.
		if value <= 0 {
			continue
		}

		if deltas[email] == nil {
			deltas[email] = &userDelta{}
		}

		switch direction {
		case "uplink":
			deltas[email].up += value
		case "downlink":
			deltas[email].down += value
		}
	}

	if len(deltas) == 0 {
		return nil, nil
	}

	stats := make([]*agentpb.TrafficStat, 0, len(deltas))
	for email, d := range deltas {
		if d.up == 0 && d.down == 0 {
			continue
		}
		stats = append(stats, &agentpb.TrafficStat{
			UserEmail:     email,
			UplinkBytes:   d.up,
			DownlinkBytes: d.down,
			CoreType:      "xray",
		})
	}

	return stats, nil
}

// connect returns a shared gRPC connection to the local xray Stats API.
func (tc *trafficCollector) connect(ctx context.Context) (*grpc.ClientConn, error) {
	tc.connMu.Lock()
	defer tc.connMu.Unlock()

	if tc.conn != nil {
		// Reuse if still ready
		if tc.conn.GetState().String() != "SHUTDOWN" {
			return tc.conn, nil
		}
		tc.conn.Close()
		tc.conn = nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, tc.statsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	tc.conn = conn
	return conn, nil
}

// close closes the underlying gRPC connection.
func (tc *trafficCollector) close() {
	tc.connMu.Lock()
	defer tc.connMu.Unlock()
	if tc.conn != nil {
		tc.conn.Close()
		tc.conn = nil
	}
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
