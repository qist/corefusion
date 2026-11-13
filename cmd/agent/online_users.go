package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"corefusion/internal/coredata"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/pkg/netutil"
)

// onlineUsersReporter 定期查询核心的按用户统计，计算实时速率，
// 并产出 OnlineUserEntry 列表供 Agent 通过心跳流上报面板。
//
// 两种模式：
//   - xray（ipBased=true）：GetUsersStats 给出「在线 IP」，是真正的在线用户，
//     同时用于 IP 维度限速/设备数；
//   - sing-box（ipBased=false）：v2ray_api 只有 QueryStats 的按用户流量，
//     没有连接 IP、也没有在线会话接口 → 只能按「最近是否还有流量增量」推断在线
//     （singboxIdleTTL）。设备数不可知，sessions 为空。
//
// 它与限速解耦：即使节点未开启限速也能工作（不需要 nft 可用）。
// 流量是订阅级（核心只提供 user 级累计流量），速率由两次采样差分计算。
type onlineUsersReporter struct {
	statsAddr string
	client    *coredata.XrayClient
	// gate 核心门禁：核心未安装/未运行时直接跳过，不产生无效连接与报错
	gate *coreProbeGate

	coreType string // xray / singbox（上报给面板，用于显示「套餐走哪个核心」）
	ipBased  bool   // 是否能拿到连接 IP

	mu       sync.RWMutex
	emailMap map[string]int64 // 核心 client email (baseEmail) -> subscriptionID

	// 速率差分：email -> 上次采样（累计流量 + 时间 + 首次出现 + 最后活跃）
	prevMu      sync.Mutex
	prevTraffic map[string]*trafficSample

	unsupportedOnce sync.Once
}

type trafficSample struct {
	downlink   int64
	uplink     int64
	ts         int64 // unix 秒
	firstSeen  int64
	lastActive int64
}

// coreOnlineIdleTTL：sing-box 没有在线语义，超过这个时长没有流量增量即视为离线。
// 与面板 LocalObserver 的 singboxIdleTTL 保持一致。
const coreOnlineIdleTTL = 120

func newOnlineUsersReporter(statsAddr string, gate *coreProbeGate) *onlineUsersReporter {
	if statsAddr == "" {
		statsAddr = coredata.DefaultStatsAddr
	}
	return &onlineUsersReporter{
		statsAddr:   statsAddr,
		client:      coredata.NewXrayClient(statsAddr),
		gate:        gate,
		coreType:    "xray",
		ipBased:     true,
		emailMap:    make(map[string]int64),
		prevTraffic: make(map[string]*trafficSample),
	}
}

// newSingboxOnlineUsersReporter 创建 sing-box 的模式：v2ray_api 只有按用户流量，
// 没有连接 IP（因此 sessions 恒为空），在线状态按流量增量推断。
func newSingboxOnlineUsersReporter(statsAddr string, gate *coreProbeGate) *onlineUsersReporter {
	if statsAddr == "" {
		statsAddr = coredata.DefaultSingboxStatsAddr
	}
	return &onlineUsersReporter{
		statsAddr:   statsAddr,
		client:      coredata.NewV2RayClient(statsAddr),
		gate:        gate,
		coreType:    "singbox",
		ipBased:     false,
		emailMap:    make(map[string]int64),
		prevTraffic: make(map[string]*trafficSample),
	}
}

// SetEmailMapping replaces the email -> subscriptionID mapping.
//
// 同时登记「完整 email」与「去掉 _shortTag 的 base email」两种键：
// xray 返回的 email 是完整形式（user@example.com_vless12345），而聚合/归户
// 用的是 base email。此前只存完整键、查询却用 base email，导致映射永远查不到。
func (r *onlineUsersReporter) SetEmailMapping(m map[string]int64) {
	normalized := make(map[string]int64, len(m)*2)
	for email, subID := range m {
		if email == "" || subID <= 0 {
			continue
		}
		normalized[email] = subID
		if base := stripShortTag(email); base != email {
			if _, exists := normalized[base]; !exists {
				normalized[base] = subID
			}
		}
	}
	r.mu.Lock()
	r.emailMap = normalized
	r.mu.Unlock()
}

// markUnsupported logs the "stats API unsupported" error exactly once.
func (r *onlineUsersReporter) markUnsupported() {
	r.unsupportedOnce.Do(func() {
		log.Printf("[Agent] %s stats API does not provide per-user data; online-users reporting disabled for this core", r.coreType)
	})
}

// Collect 按核心类型采集在线用户（速率已差分）。
// Returns (nil, nil) when there are no online users; (nil, err) on transport error.
func (r *onlineUsersReporter) Collect(ctx context.Context) ([]*agentpb.OnlineUserEntry, error) {
	// 核心未安装/未运行时不必探测：直接返回空（面板会收到空列表，快照时间戳照常刷新）。
	if r.gate != nil && !r.gate.Allowed(r.coreType) {
		return nil, nil
	}
	if !r.ipBased {
		return r.collectSingbox(ctx)
	}
	return r.collectXray(ctx)
}

// collectXray 走 GetUsersStats：给出每个用户的在线 IP（权威在线状态）。
func (r *onlineUsersReporter) collectXray(ctx context.Context) ([]*agentpb.OnlineUserEntry, error) {
	resp, err := r.client.GetUsersStats(ctx, true)
	if r.gate != nil {
		r.gate.Report(r.coreType, err == nil)
	}
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") {
			r.markUnsupported()
			return nil, nil
		}
		return nil, err
	}
	users := resp.GetUsers()
	if len(users) == 0 {
		// 无在线用户，清理差分基线，避免内存泄漏
		r.cleanupStale(nil)
		return nil, nil
	}

	emailMap := r.mappingSnapshot()

	now := time.Now().Unix()
	seen := make(map[string]struct{}, len(users))
	entries := make([]*agentpb.OnlineUserEntry, 0, len(users))

	for _, user := range users {
		email := user.GetEmail()
		if email == "" || len(user.GetIps()) == 0 {
			continue
		}

		// 解析订阅 ID：完整 email → base email → 旧格式解析。
		// 解析不出来也照样上报（SubscriptionId=0）：面板有 clients/subscriptions 表，
		// 可以按 email 反查并驱动限速；在 Agent 侧直接 continue 会让管理端创建的
		// 账号（email 形如 user@example.com_vless12345）永远拿不到核心观测 IP。
		baseEmail := stripShortTag(email)
		subID := resolveSubID(emailMap, baseEmail)
		seen[baseEmail] = struct{}{}

		// 速率差分（订阅级累计流量）
		var dl, ul int64
		if t := user.GetTraffic(); t != nil {
			dl = t.GetDownlink()
			ul = t.GetUplink()
		}
		dlRate, ulRate, firstSeen, _ := r.diffRate(email, dl, ul, now, true)

		// 组装 session（每个 IP 一行）
		sessions := make([]*agentpb.OnlineSession, 0, len(user.GetIps()))
		var lastSeen int64
		for _, ip := range user.GetIps() {
			ipStr := ip.GetIp()
			if ipStr == "" || !netutil.IsPublicClientIP(ipStr) {
				continue
			}
			ls := ip.GetLastSeen()
			if ls > lastSeen {
				lastSeen = ls
			}
			sessions = append(sessions, &agentpb.OnlineSession{
				Ip:             ipStr,
				ConnectedSince: firstSeen, // 近似：xray 不返回接入时间
				LastSeen:       ls,
			})
		}
		if len(sessions) == 0 {
			continue
		}
		if lastSeen == 0 {
			lastSeen = now
		}
		// xray 的 lastSeen 是「最后一次活动」，可能早于我们第一次采样到它的时间；
		// 倒挂会让前端算出负时长，统一以较早者为准。
		if firstSeen > lastSeen {
			firstSeen = lastSeen
		}
		for _, s := range sessions {
			since := firstSeen
			if s.LastSeen > 0 && since > s.LastSeen {
				since = s.LastSeen
			}
			s.ConnectedSince = since
		}

		entries = append(entries, &agentpb.OnlineUserEntry{
			SubscriptionId: subID,
			Email:          baseEmail,
			AccountEmail:   email,
			CoreType:       r.coreType,
			FirstSeen:      firstSeen,
			LastSeen:       lastSeen,
			DownlinkBytes:  dl,
			UplinkBytes:    ul,
			DownlinkRate:   dlRate,
			UplinkRate:     ulRate,
			Sessions:       sessions,
		})
	}

	r.cleanupStale(seen)

	if len(entries) == 0 {
		return nil, nil
	}
	return entries, nil
}

// collectSingbox 走 sing-box v2ray_api 的 QueryStats：只有按用户累计流量，
// 没有连接 IP、也没有在线会话接口。
//
// 在线状态是**推断值**：coreOnlineIdleTTL 内有过流量增量即算在线。
// 因此面板能显示「哪个套餐在使用 sing-box + 用了多少流量」，
// 但 sessions 恒为空（设备数不可知）。
func (r *onlineUsersReporter) collectSingbox(ctx context.Context) ([]*agentpb.OnlineUserEntry, error) {
	usersTraffic, err := r.client.QueryUserTraffic(ctx, false)
	if r.gate != nil {
		r.gate.Report(r.coreType, err == nil)
	}
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") {
			r.markUnsupported()
			return nil, nil
		}
		return nil, err
	}
	if len(usersTraffic) == 0 {
		r.cleanupStale(nil)
		return nil, nil
	}

	emailMap := r.mappingSnapshot()
	now := time.Now().Unix()
	seen := make(map[string]struct{}, len(usersTraffic))
	entries := make([]*agentpb.OnlineUserEntry, 0, len(usersTraffic))

	for _, ut := range usersTraffic {
		email := strings.TrimSpace(ut.Email)
		if email == "" {
			continue
		}
		dlRate, ulRate, firstSeen, active := r.diffRate(email, ut.Downlink, ut.Uplink, now, false)
		baseEmail := stripShortTag(email)
		// 样本按全部用户保留（用户可能只是暂时没流量），条目只上报活跃的。
		seen[baseEmail] = struct{}{}
		if !active {
			continue
		}
		entries = append(entries, &agentpb.OnlineUserEntry{
			SubscriptionId: resolveSubID(emailMap, baseEmail),
			Email:          baseEmail,
			AccountEmail:   email,
			CoreType:       r.coreType,
			FirstSeen:      firstSeen,
			LastSeen:       now,
			DownlinkBytes:  ut.Downlink,
			UplinkBytes:    ut.Uplink,
			DownlinkRate:   dlRate,
			UplinkRate:     ulRate,
		})
	}

	r.cleanupStale(seen)
	if len(entries) == 0 {
		return nil, nil
	}
	return entries, nil
}

// diffRate computes bps rates from cumulative traffic using the last sample.
//
// alwaysActive：xray 条目有在线 IP，每次采样都算活跃；sing-box 没有在线语义，
// 只有累计流量增长才算活跃，返回 active=false 时调用方应跳过该用户
// （超过 coreOnlineIdleTTL 没有任何流量增量即视为离线）。
func (r *onlineUsersReporter) diffRate(email string, dl, ul, now int64, alwaysActive bool) (dlRate, ulRate, firstSeen int64, active bool) {
	r.prevMu.Lock()
	defer r.prevMu.Unlock()
	prev, ok := r.prevTraffic[email]
	total := dl + ul
	if !ok {
		// 首次采样：速率未知，仅记录基线
		r.prevTraffic[email] = &trafficSample{downlink: dl, uplink: ul, ts: now, firstSeen: now, lastActive: now}
		return 0, 0, now, alwaysActive || total > 0
	}
	dt := now - prev.ts
	if dt > 0 {
		if dl >= prev.downlink {
			dlRate = (dl - prev.downlink) / dt
		}
		if ul >= prev.uplink {
			ulRate = (ul - prev.uplink) / dt
		}
	}
	firstSeen = prev.firstSeen
	prevTotal := prev.downlink + prev.uplink
	// 核心重启会导致计数器归零（dl/ul < prev），按新会话处理
	if dl < prev.downlink || ul < prev.uplink {
		firstSeen = now
		prev.lastActive = now
	} else if total > prevTotal {
		prev.lastActive = now
	}
	if alwaysActive {
		prev.lastActive = now
	}
	prev.downlink, prev.uplink, prev.ts, prev.firstSeen = dl, ul, now, firstSeen
	active = now-prev.lastActive <= coreOnlineIdleTTL
	return dlRate, ulRate, firstSeen, active
}

// mappingSnapshot 复制一份 email→订阅映射（避免长时间持锁）。
func (r *onlineUsersReporter) mappingSnapshot() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]int64, len(r.emailMap))
	for k, v := range r.emailMap {
		out[k] = v
	}
	return out
}

// resolveSubID 解析订阅 ID：完整 email → base email → 旧格式（sub_<id>@ / <id>@）。
func resolveSubID(emailMap map[string]int64, baseEmail string) int64 {
	if subID := emailMap[baseEmail]; subID > 0 {
		return subID
	}
	// 映射里可能只登记了带 _tag 的完整邮箱：反向找一个 base 相同的键。
	for k, v := range emailMap {
		if v > 0 && stripShortTag(k) == baseEmail {
			return v
		}
	}
	return extractSubIDFromEmail(baseEmail)
}

// cleanupStale removes differential baselines for emails no longer reported.
func (r *onlineUsersReporter) cleanupStale(seen map[string]struct{}) {
	if len(seen) == 0 {
		r.prevMu.Lock()
		r.prevTraffic = make(map[string]*trafficSample)
		r.prevMu.Unlock()
		return
	}
	r.prevMu.Lock()
	for email := range r.prevTraffic {
		// 样本 key 可能是完整 email，而 seen 存的是 base email。
		if _, ok := seen[stripShortTag(email)]; !ok {
			delete(r.prevTraffic, email)
		}
	}
	r.prevMu.Unlock()
}

// stripShortTag removes the trailing "_shortTag" suffix appended by
// CoreFusion's generateInboundEmail, returning the base email used as the
// mapping key. Example: "sub_123@example.com_abc123" -> "sub_123@example.com".
func stripShortTag(email string) string {
	if idx := strings.LastIndex(email, "_"); idx > 0 {
		tail := email[idx+1:]
		// shortTag 段不含 '@'，而 baseEmail 中下划线只出现在 '@' 之前
		if !strings.Contains(tail, "@") {
			return email[:idx]
		}
	}
	return email
}

// sendOnlineUsersReport collects online users from every running core and sends
// them to the Panel via the heartbeat stream as an ONLINE_USERS report.
// 每条记录带 core_type，面板据此显示「这个套餐走的是 xray 还是 sing-box」。
func (a *AgentApp) sendOnlineUsersReport(stream agentpb.HeartbeatService_StreamClient) error {
	reporters := []*onlineUsersReporter{a.onlineUsersReporter, a.singboxOnlineReporter}
	any := false
	for _, r := range reporters {
		if r != nil {
			any = true
			break
		}
	}
	if !any {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// 空列表也发送：让面板刷新"节点在线但当前无订阅人"的快照时间戳，
	// 否则面板无法区分"节点未上报"与"节点 0 人在线"。
	entries := make([]*agentpb.OnlineUserEntry, 0, 8)
	for _, reporter := range reporters {
		if reporter == nil {
			continue
		}
		got, err := reporter.Collect(ctx)
		if err != nil {
			// 传输错误不应中断心跳流；另一个核心的结果照常上报
			log.Printf("[Agent] collect %s online users failed: %v", reporter.coreType, err)
			continue
		}
		entries = append(entries, got...)
	}

	a.streamSendMu.Lock()
	defer a.streamSendMu.Unlock()
	return stream.Send(&agentpb.AgentReport{
		Type:        agentpb.AgentReport_ONLINE_USERS,
		ServerId:    a.cfg.Panel.ServerID,
		Timestamp:   time.Now().Unix(),
		OnlineUsers: entries,
	})
}
