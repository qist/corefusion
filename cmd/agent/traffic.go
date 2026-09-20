package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"corefusion/internal/coredata"
	agentpb "corefusion/internal/runtime/agent/proto"
	coreconfigSvc "corefusion/internal/service/coreconfig"
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
	client        *coredata.XrayClient
	singboxClient *coredata.XrayClient // nil = 该节点没有 sing-box v2ray_api
	pollInterval  time.Duration
	initialized   bool // false until first reset query completes
	initializedSb bool // sing-box 基线状态
}

// newTrafficCollector creates a traffic collector for the local cores' stats API.
// singboxStatsAddr 为空表示该节点未启用 sing-box v2ray_api（不采集 sing-box）。
func newTrafficCollector(statsAddr, singboxStatsAddr string) *trafficCollector {
	if statsAddr == "" {
		statsAddr = coredata.DefaultStatsAddr
	}
	tc := &trafficCollector{
		client:       coredata.NewXrayClient(statsAddr),
		pollInterval: 30 * time.Second,
	}
	if singboxStatsAddr != "" {
		// sing-box 的 stats protobuf 与 xray 一致，但 gRPC 服务名不同，
		// 必须用 NewV2RayClient，否则查询一直 Unimplemented（流量静默漏采）。
		tc.singboxClient = coredata.NewV2RayClient(singboxStatsAddr)
	}
	return tc
}

// collect queries both cores' stats APIs and returns traffic deltas as TrafficStat list.
// Returns nil stats (no error) when there are no deltas to report.
func (tc *trafficCollector) collect(ctx context.Context) ([]*agentpb.TrafficStat, error) {
	// First call: reset counters to 0.
	// Discard the returned values (they are old accumulated traffic).
	// This ensures subsequent queries return only new traffic.
	if !tc.initialized || (tc.singboxClient != nil && !tc.initializedSb) {
		baseline := false
		if !tc.initialized {
			if _, err := tc.client.RawStats(ctx, true); err != nil {
				return nil, fmt.Errorf("initial reset query xray stats: %w", err)
			}
			tc.initialized = true
			baseline = true
		}
		if tc.singboxClient != nil && !tc.initializedSb {
			if _, err := tc.singboxClient.RawStats(ctx, true); err != nil {
				// sing-box 未启动/v2ray_api 未监听时不阻断 xray 上报，下轮重试。
				log.Printf("traffic: initial reset sing-box stats failed: %v", err)
			} else {
				tc.initializedSb = true
				baseline = true
			}
		}
		if baseline {
			return nil, nil
		}
	}

	var stats []*agentpb.TrafficStat

	// Subsequent calls: reset_=true so the core returns deltas and resets counters.
	if tc.initialized {
		resp, err := tc.client.RawStats(ctx, true)
		if err != nil {
			return nil, fmt.Errorf("query xray stats: %w", err)
		}
		sr, err := coredata.ParseStatsResponse(resp)
		if err != nil {
			return nil, err
		}
		stats = append(stats, trafficStatsFromUsers(sr.Users, "xray")...)
	}

	if tc.singboxClient != nil && tc.initializedSb {
		resp, err := tc.singboxClient.RawStats(ctx, true)
		if err != nil {
			// 单核心失败不影响另一个核心的上报（例如 sing-box 滚动重启中）。
			log.Printf("traffic: query sing-box stats failed: %v", err)
		} else if sr, err := coredata.ParseStatsResponse(resp); err == nil {
			stats = append(stats, trafficStatsFromUsers(sr.Users, "singbox")...)
		} else {
			log.Printf("traffic: parse sing-box stats failed: %v", err)
		}
	}

	return stats, nil
}

// trafficStatsFromUsers 复用公共解析结果，仅取 user 方向；
// 增量语义（任一方向 <=0 跳过）与原行为一致。
func trafficStatsFromUsers(users []coredata.UserTraffic, coreType string) []*agentpb.TrafficStat {
	stats := make([]*agentpb.TrafficStat, 0, len(users))
	for _, u := range users {
		// 增量语义：任一方向 <=0 视为复位/异常，跳过（与原行为一致）。
		if u.Uplink <= 0 || u.Downlink <= 0 {
			continue
		}
		stats = append(stats, &agentpb.TrafficStat{
			UserEmail:     u.Email,
			UplinkBytes:   u.Uplink,
			DownlinkBytes: u.Downlink,
			CoreType:      coreType,
		})
	}
	return stats
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

// resolveSingboxStatsAddr 解析本机 sing-box V2Ray API 地址：
//  1. 本地设置 singbox.stats_addr
//  2. 运行时配置 runtime/singbox.jsonc 的 experimental.v2ray_api.listen
//     （面板生成配置时会注入，是真正生效的地址）
//
// 返回空字符串表示该节点没有 sing-box v2ray_api，此时不采集 sing-box 流量。
func resolveSingboxStatsAddr(ctx context.Context, dataDir string, settingRepo *agentSettingStore) string {
	if settingRepo != nil {
		if val, err := settingRepo.GetByKey(ctx, "singbox.stats_addr"); err == nil && val != "" {
			return val
		}
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, "runtime", "singbox.jsonc"))
	if err != nil {
		return ""
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(coreconfigSvc.StripJSONC(raw), &cfg); err != nil {
		log.Printf("traffic: parse runtime singbox config failed: %v", err)
		return ""
	}
	exp, _ := cfg["experimental"].(map[string]interface{})
	if exp == nil {
		return ""
	}
	v2, _ := exp["v2ray_api"].(map[string]interface{})
	if v2 == nil {
		return ""
	}
	listen, _ := v2["listen"].(string)
	return listen
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
