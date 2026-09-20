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
	singboxClient *coredata.XrayClient // nil = 尚未发现 sing-box v2ray_api
	singboxAddr   string
	// singboxAddrResolver 懒解析 sing-box V2Ray API 地址（返回空表示未启用）。
	// 每次采集都会调用，因为核可能在 Agent 启动后才安装/启用 v2ray_api。
	singboxAddrResolver func() string
	// gate 核心门禁：未安装/未运行的核心不探测（避免无效连接与日志刷屏）。
	gate          *coreProbeGate
	pollInterval  time.Duration
	initialized   bool // false until first reset query completes
	initializedSb bool // sing-box 基线状态
}

// newTrafficCollector creates a traffic collector for the local cores' stats API.
// singboxAddrResolver 返回该节点的 sing-box V2Ray API 地址（空 = 未启用，不采集）。
// gate 为 nil 时不做门禁（仅测试用），生产路径应始终传入。
func newTrafficCollector(statsAddr string, singboxAddrResolver func() string, gate *coreProbeGate) *trafficCollector {
	if statsAddr == "" {
		statsAddr = coredata.DefaultStatsAddr
	}
	return &trafficCollector{
		client:              coredata.NewXrayClient(statsAddr),
		singboxAddrResolver: singboxAddrResolver,
		gate:                gate,
		pollInterval:        30 * time.Second,
	}
}

// probeAllowed 报告该核心当前是否允许探测（gate 为空时按允许处理）。
func (tc *trafficCollector) probeAllowed(coreType string) bool {
	if tc.gate == nil {
		return true
	}
	return tc.gate.Allowed(coreType)
}

// collect queries both cores' stats APIs and returns traffic deltas as TrafficStat list.
// Returns nil stats (no error) when there are no deltas to report.
func (tc *trafficCollector) collect(ctx context.Context) ([]*agentpb.TrafficStat, error) {
	// 懒解析 sing-box 地址：核可能是在 Agent 启动之后才安装/配置的
	// （例如刚给节点装了 sing-box 并启用 v2ray_api）。
	tc.ensureSingboxClient()

	// 门禁：核心没装/没跑就不探测。同时把基线标志复位，
	// 等核心起来后重新建立基线（否则会把启动前的累计值当成增量上报）。
	xrayOK := tc.probeAllowed("xray")
	if !xrayOK {
		tc.initialized = false
	}
	// sing-box 需要「门禁通过 + 有 v2ray_api 地址」两个条件
	sbOK := tc.singboxClient != nil && tc.probeAllowed("singbox")
	if !sbOK {
		tc.initializedSb = false
	}

	// First call: reset counters to 0.
	// Discard the returned values (they are old accumulated traffic).
	// This ensures subsequent queries return only new traffic.
	baseline := false
	if xrayOK && !tc.initialized {
		if _, err := tc.client.RawStats(ctx, true); err != nil {
			// 单核心不可用时不能中断另一核心的上报：节点可能只跑 sing-box
			// （xray 未启用 api/stats 时 10085 连不上），失败则下轮重试。
			tc.report("xray", false)
			log.Printf("traffic: initial reset xray stats failed (will retry): %v", err)
		} else {
			tc.report("xray", true)
			tc.initialized = true
			baseline = true
		}
	}
	if sbOK && !tc.initializedSb {
		if _, err := tc.singboxClient.RawStats(ctx, true); err != nil {
			tc.report("singbox", false)
			log.Printf("traffic: initial reset sing-box stats failed (will retry): %v", err)
		} else {
			tc.report("singbox", true)
			tc.initializedSb = true
			baseline = true
		}
	}
	if baseline {
		return nil, nil
	}

	var stats []*agentpb.TrafficStat

	// Subsequent calls: reset_=true so the core returns deltas and resets counters。
	// 两个核心各自独立处理失败，互不影响。
	if tc.initialized && xrayOK {
		if resp, err := tc.client.RawStats(ctx, true); err != nil {
			tc.report("xray", false)
			log.Printf("traffic: query xray stats failed: %v", err)
		} else if sr, err := coredata.ParseStatsResponse(resp); err != nil {
			log.Printf("traffic: parse xray stats failed: %v", err)
		} else {
			tc.report("xray", true)
			stats = append(stats, trafficStatsFromUsers(sr.Users, "xray")...)
		}
	}

	if sbOK && tc.initializedSb {
		if resp, err := tc.singboxClient.RawStats(ctx, true); err != nil {
			tc.report("singbox", false)
			// 单核心失败不影响另一个核心的上报（例如 sing-box 滚动重启中）。
			log.Printf("traffic: query sing-box stats failed: %v", err)
		} else if sr, err := coredata.ParseStatsResponse(resp); err == nil {
			tc.report("singbox", true)
			stats = append(stats, trafficStatsFromUsers(sr.Users, "singbox")...)
		} else {
			log.Printf("traffic: parse sing-box stats failed: %v", err)
		}
	}

	return stats, nil
}

// report 回填探测结果给门禁（连续失败会触发退避暂停）。
func (tc *trafficCollector) report(coreType string, ok bool) {
	if tc.gate != nil {
		tc.gate.Report(coreType, ok)
	}
}

// ensureSingboxClient 懒创建/更新 sing-box stats 客户端。
// sing-box 可能在 Agent 启动之后才安装或启用 v2ray_api，因此地址不能只在启动时解析一次。
func (tc *trafficCollector) ensureSingboxClient() {
	if tc.singboxAddrResolver == nil {
		return
	}
	addr := tc.singboxAddrResolver()
	if addr == "" {
		return
	}
	if tc.singboxClient != nil && addr == tc.singboxAddr {
		return
	}
	tc.singboxClient = coredata.NewV2RayClient(addr)
	tc.singboxAddr = addr
	tc.initializedSb = false // 地址变化后需要重新建立基线
	log.Printf("traffic: sing-box stats client initialized at %s (service=%s)",
		addr, coredata.V2RayStatsServiceName)
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
