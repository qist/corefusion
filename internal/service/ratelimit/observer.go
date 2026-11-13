package ratelimitsvc

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

const (
	coreXray    = "xray"
	coreSingbox = "singbox"

	// singboxIdleTTL：sing-box 没有按用户的连接 IP，也不提供「在线」语义，
	// 只能靠「两次采样之间按用户流量是否增长」判断活跃。超过这个时长没有
	// 任何流量增量就视为离线（否则一次连接后会永远挂在在线列表里）。
	singboxIdleTTL = 120 * time.Second
	// singboxSampleTTL：活跃样本保留时长（比 idle TTL 长，避免频繁重建 first_seen）。
	singboxSampleTTL = 30 * time.Minute
)

// LocalObserver 是**面板本机节点**的核心观测器。
//
// 远程节点的真实连接 IP 由 Agent 的 ONLINE_USERS 上报（→ SetOnlineUsersHook）；
// 但本地节点没有独立 Agent 进程（核心由面板进程内管理），所以本机必须自己
// 轮询核心：
//   - xray：GetUsersStats → 按用户的在线 IP（可用于 IP 维度限速/设备数）；
//   - sing-box：v2ray_api QueryStats → 只有按用户流量，没有连接 IP。
//     sing-box 的「在线」只能按流量增量推断（见 singboxIdleTTL）。
//
// 两个核心各自产出一批 OnlineUserEntry（带 core_type），合并后写入同一份
// 节点在线快照，面板据此显示「这个套餐连的是哪个核心」。
type LocalObserver struct {
	svc      *Service
	serverID int64
	interval time.Duration

	// xray（有连接 IP）
	statsAddr   func(context.Context) string
	coreRunning func() bool

	mu            sync.Mutex
	sbStatsAddr   func(context.Context) string
	sbCoreRunning func() bool

	clients     map[string]*coredata.XrayClient
	clientAddrs map[string]string
	unsupported map[string]bool
	visState    string
	sbErrLogAt  int64

	// 每个核心最近一次的在线条目；合并后写入节点快照
	lastByCore map[string][]*agentpb.OnlineUserEntry

	// 在线快照（面板在线订阅人/在线用户）：速率由两次采样差分得到。
	// trafficsync 会周期性 reset 计数器，出现回退时按新会话处理（同 Agent 侧口径）。
	rateMu  sync.Mutex
	samples map[string]*localOnlineSample
}

type localOnlineSample struct {
	down       int64
	up         int64
	ts         int64
	firstSeen  int64
	lastActive int64
}

// StartLocalObserver 启动本机核心观测循环（幂等：多次调用只保留最后一个）。
// statsAddr 返回空串时本轮跳过；coreRunning 为空表示不设门禁。
func (s *Service) StartLocalObserver(
	serverID int64,
	interval time.Duration,
	statsAddr func(context.Context) string,
	coreRunning func() bool,
) *LocalObserver {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	o := &LocalObserver{
		svc:         s,
		serverID:    serverID,
		interval:    interval,
		statsAddr:   statsAddr,
		coreRunning: coreRunning,
		clients:     make(map[string]*coredata.XrayClient),
		clientAddrs: make(map[string]string),
		unsupported: make(map[string]bool),
		lastByCore:  make(map[string][]*agentpb.OnlineUserEntry),
		samples:     make(map[string]*localOnlineSample),
	}
	go o.loop(context.Background())
	log.Printf("[RateLimitObserved] local observer started (server=%d interval=%s)", serverID, interval)
	return o
}

// EnableSingbox 注入本机 sing-box 的观测依赖。
//
// 必须在 StartLocalObserver 之后、第一轮 tick 之前调用（路由器装配时同步调用即可）。
// statsAddr 返回空串或 coreRunning 为 false 时，该核心的在线条目会被清空。
func (o *LocalObserver) EnableSingbox(statsAddr func(context.Context) string, coreRunning func() bool) {
	o.mu.Lock()
	o.sbStatsAddr = statsAddr
	o.sbCoreRunning = coreRunning
	o.mu.Unlock()
	log.Printf("[RateLimitObserved] local sing-box observation enabled (server=%d)", o.serverID)
}

func (o *LocalObserver) loop(ctx context.Context) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	// 启动后先等一个周期：核心/配置就绪需要时间，避免开局刷错误日志。
	for {
		select {
		case <-ctx.Done():
			o.close()
			return
		case <-ticker.C:
			o.poll(ctx)
		}
	}
}

func (o *LocalObserver) poll(ctx context.Context) {
	if o.svc == nil || o.serverID <= 0 {
		return
	}
	o.pollXray(ctx)
	o.pollSingbox(ctx)
	o.flush()
}

// flush 把两个核心的在线条目合并写入节点快照。
// 空列表也要写：面板才能区分「节点未上报」与「节点 0 人在线」。
func (o *LocalObserver) flush() {
	o.mu.Lock()
	merged := make([]*agentpb.OnlineUserEntry, 0,
		len(o.lastByCore[coreXray])+len(o.lastByCore[coreSingbox]))
	merged = append(merged, o.lastByCore[coreXray]...)
	merged = append(merged, o.lastByCore[coreSingbox]...)
	sink := o.svc.onlineSink
	o.mu.Unlock()
	if sink != nil {
		sink(o.serverID, merged)
	}
}

func (o *LocalObserver) setCoreEntries(core string, entries []*agentpb.OnlineUserEntry) {
	o.mu.Lock()
	o.lastByCore[core] = entries
	o.mu.Unlock()
}

// pollXray 采集 xray 的在线用户与连接 IP（同时驱动 IP 维度限速）。
func (o *LocalObserver) pollXray(ctx context.Context) {
	if o.statsAddr == nil {
		return
	}
	o.mu.Lock()
	unsupported := o.unsupported[coreXray]
	o.mu.Unlock()
	if unsupported {
		o.setCoreEntries(coreXray, nil)
		return
	}
	if o.coreRunning != nil && !o.coreRunning() {
		o.setCoreEntries(coreXray, nil)
		o.cleanupSamples(coreXray, nil)
		return
	}

	addr := strings.TrimSpace(o.statsAddr(ctx))
	if addr == "" {
		return
	}
	client := o.clientFor(coreXray, addr, false)
	if client == nil {
		return
	}

	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// includeTraffic=true：面板的「在线订阅人」要显示累计流量与实时速率。
	resp, err := client.GetUsersStats(pollCtx, true)
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") {
			o.mu.Lock()
			o.unsupported[coreXray] = true
			o.mu.Unlock()
			log.Printf("[RateLimitObserved] xray GetUsersStats unsupported; local core-IP enforcement disabled")
			o.setCoreEntries(coreXray, nil)
			return
		}
		log.Printf("[RateLimitObserved] local GetUsersStats failed: %v", err)
		return
	}

	now := time.Now().Unix()
	users := make([]ObservedUser, 0, len(resp.GetUsers()))
	entries := make([]*agentpb.OnlineUserEntry, 0, len(resp.GetUsers()))
	publicIPs := 0
	for _, u := range resp.GetUsers() {
		if u == nil || u.GetEmail() == "" || len(u.GetIps()) == 0 {
			continue
		}
		ips := make([]string, 0, len(u.GetIps()))
		sessions := make([]*agentpb.OnlineSession, 0, len(u.GetIps()))
		var lastSeen int64
		for _, entry := range u.GetIps() {
			if entry == nil || entry.GetIp() == "" {
				continue
			}
			if netutil.IsPublicClientIP(entry.GetIp()) {
				publicIPs++
			}
			ips = append(ips, entry.GetIp())
			sessions = append(sessions, &agentpb.OnlineSession{
				Ip:       entry.GetIp(),
				LastSeen: entry.GetLastSeen(),
			})
			if entry.GetLastSeen() > lastSeen {
				lastSeen = entry.GetLastSeen()
			}
		}
		if len(ips) == 0 {
			continue
		}
		if lastSeen == 0 {
			lastSeen = now
		}

		var dl, ul int64
		if t := u.GetTraffic(); t != nil {
			dl, ul = t.GetDownlink(), t.GetUplink()
		}
		baseEmail := stripObservedEmailTag(u.GetEmail())
		// 采样 key 用 base email：条目里存的也是 base email，cleanupSamples
		// 才能对得上（否则每轮都被当成「已下线」删掉 → first_seen 归零）。
		dlRate, ulRate, firstSeen, _ := o.rateFor(coreXray, baseEmail, dl, ul, now, true)
		// xray 的 lastSeen 是「最后一次活动」，可能早于我们第一次采样到它的时间；
		// 这种倒挂会让前端算出负时长。统一以较早者为准。
		if firstSeen > lastSeen {
			firstSeen = lastSeen
		}
		// session 级「连接时长」用 last_seen - connected_since：缺省 0 会让前端
		// 算出 498484 小时（last_seen 本身是 unix 秒）。以首次观测时间为接入时间，
		// 且不能晚于该 IP 的最后活跃时间（否则是负时长）。
		for _, s := range sessions {
			since := firstSeen
			if s.LastSeen > 0 && since > s.LastSeen {
				since = s.LastSeen
			}
			s.ConnectedSince = since
		}

		subID, _ := o.svc.resolveObservedSub(ctx, ObservedUser{Email: u.GetEmail(), IPs: ips})
		entries = append(entries, &agentpb.OnlineUserEntry{
			SubscriptionId: subID,
			Email:          baseEmail,
			AccountEmail:   u.GetEmail(),
			CoreType:       coreXray,
			FirstSeen:      firstSeen,
			LastSeen:       lastSeen,
			DownlinkBytes:  dl,
			UplinkBytes:    ul,
			DownlinkRate:   dlRate,
			UplinkRate:     ulRate,
			Sessions:       sessions,
		})
		users = append(users, ObservedUser{Email: u.GetEmail(), IPs: ips})
	}

	o.setCoreEntries(coreXray, entries)
	o.cleanupSamples(coreXray, entries)

	// 可见性提示：核心只看到回环/私网 → 入站前面有代理且没透传源 IP。
	// 只在状态变化时打一行，避免每 15s 刷屏。
	state := "visible"
	if publicIPs == 0 {
		state = "hidden"
	}
	o.mu.Lock()
	changed := o.visState != state
	o.visState = state
	o.mu.Unlock()
	if changed {
		if state == "hidden" {
			log.Printf("[RateLimitObserved] local xray source IPs are all private/loopback; IP-based limits are skipped " +
				"(enable PROXY protocol or connect inbounds directly)")
		} else {
			log.Printf("[RateLimitObserved] local xray source IPs are visible again; IP-based limits resumed")
		}
	}

	if len(users) > 0 {
		o.svc.HandleObservedUsers(ctx, o.serverID, users)
	}
}

// pollSingbox 采集 sing-box 的按用户流量，并按「最近是否还有流量增长」推断在线。
//
// sing-box 的 v2ray_api 只有 `user>>>email>>>traffic>>>uplink|downlink`：
// 拿不到连接 IP、也没有在线会话接口。所以这里的在线是**推断值**：
// 一个用户只要在 singboxIdleTTL 内有过流量增量，就认为还在线；
// 因此可以显示「哪个套餐在使用 sing-box 核心 + 用了多少流量」，
// 但 sessions 恒为空（前端显示为「无 IP」）。
func (o *LocalObserver) pollSingbox(ctx context.Context) {
	o.mu.Lock()
	addrFn := o.sbStatsAddr
	gate := o.sbCoreRunning
	unsupported := o.unsupported[coreSingbox]
	o.mu.Unlock()
	if addrFn == nil || unsupported {
		return
	}
	if gate != nil && !gate() {
		o.setCoreEntries(coreSingbox, nil)
		o.cleanupSamples(coreSingbox, nil)
		return
	}
	addr := strings.TrimSpace(addrFn(ctx))
	if addr == "" {
		return
	}
	client := o.clientFor(coreSingbox, addr, true)
	if client == nil {
		return
	}

	pollCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	usersTraffic, err := client.QueryUserTraffic(pollCtx, false)
	if err != nil {
		if strings.Contains(err.Error(), "Unimplemented") {
			o.mu.Lock()
			o.unsupported[coreSingbox] = true
			o.mu.Unlock()
			log.Printf("[RateLimitObserved] sing-box v2ray_api QueryStats unsupported; sing-box usage disabled")
			o.setCoreEntries(coreSingbox, nil)
			return
		}
		// 核心刚装好/还没重启加载 v2ray_api 时这里会 connection refused；
		// 15s 一报太吵，同一地址 5 分钟内只打一行。
		o.mu.Lock()
		canLog := time.Now().Unix()-o.sbErrLogAt > 300
		if canLog {
			o.sbErrLogAt = time.Now().Unix()
		}
		o.mu.Unlock()
		if canLog {
			log.Printf("[RateLimitObserved] local sing-box QueryStats failed (addr=%s): %v", addr, err)
		}
		return
	}
	now := time.Now().Unix()
	entries := make([]*agentpb.OnlineUserEntry, 0, len(usersTraffic))
	for _, ut := range usersTraffic {
		if strings.TrimSpace(ut.Email) == "" {
			continue
		}
		baseEmail := stripObservedEmailTag(ut.Email)
		dlRate, ulRate, firstSeen, active := o.rateFor(coreSingbox, baseEmail, ut.Downlink, ut.Uplink, now, false)
		if !active {
			continue
		}
		subID, _ := o.svc.resolveObservedSub(ctx, ObservedUser{Email: ut.Email})
		entries = append(entries, &agentpb.OnlineUserEntry{
			SubscriptionId: subID,
			Email:          baseEmail,
			AccountEmail:   ut.Email,
			CoreType:       coreSingbox,
			FirstSeen:      firstSeen,
			LastSeen:       now,
			DownlinkBytes:  ut.Downlink,
			UplinkBytes:    ut.Uplink,
			DownlinkRate:   dlRate,
			UplinkRate:     ulRate,
		})
	}
	o.setCoreEntries(coreSingbox, entries)
	o.cleanupSamples(coreSingbox, entries)
}

// rateFor 用两次采样的累计流量差分出 bps 速率（与 Agent 侧 online_users.go 同口径）。
// 计数器回退（trafficsync 周期性 reset / 核心重启）时按新会话处理，速率记 0。
//
// xray 条目每次采样都算「在线」（alwaysActive=true），最后活跃时间随采样推进；
// sing-box 条目没有连接 IP，只能靠流量增量更新 lastActive，返回它是否仍在 TTL 内。
func (o *LocalObserver) rateFor(core, email string, down, up, now int64, alwaysActive bool) (dlRate, ulRate, firstSeen int64, active bool) {
	o.rateMu.Lock()
	defer o.rateMu.Unlock()
	if o.samples == nil {
		o.samples = make(map[string]*localOnlineSample)
	}
	key := core + "|" + email
	prev := o.samples[key]
	total := down + up
	if prev == nil {
		o.samples[key] = &localOnlineSample{down: down, up: up, ts: now, firstSeen: now, lastActive: now}
		return 0, 0, now, alwaysActive || total > 0
	}
	if dt := now - prev.ts; dt > 0 {
		if down >= prev.down {
			dlRate = (down - prev.down) / dt
		}
		if up >= prev.up {
			ulRate = (up - prev.up) / dt
		}
	}
	firstSeen = prev.firstSeen
	prevTotal := prev.down + prev.up
	if down < prev.down || up < prev.up {
		// 计数器回退：核心重启或 trafficsync 复位 → 按新会话处理
		firstSeen = now
		prev.lastActive = now
	} else if total > prevTotal {
		prev.lastActive = now
	}
	if alwaysActive {
		prev.lastActive = now
	}
	prev.down, prev.up, prev.ts, prev.firstSeen = down, up, now, firstSeen
	active = now-prev.lastActive <= int64(singboxIdleTTL/time.Second)
	return dlRate, ulRate, firstSeen, active
}

// cleanupSamples 清掉已下线/过期的采样，避免内存随历史用户增长。
func (o *LocalObserver) cleanupSamples(core string, entries []*agentpb.OnlineUserEntry) {
	o.rateMu.Lock()
	defer o.rateMu.Unlock()
	now := time.Now().Unix()
	live := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		live[core+"|"+e.GetEmail()] = struct{}{}
	}
	for k, v := range o.samples {
		if !strings.HasPrefix(k, core+"|") {
			continue
		}
		if _, ok := live[k]; ok {
			continue
		}
		// sing-box 的样本要留够 TTL（用户可能只是暂时没流量），
		// xray 的在线条目就是权威结果，不在列表里即可删除。
		if core == coreSingbox && now-v.lastActive < int64(singboxSampleTTL/time.Second) {
			continue
		}
		delete(o.samples, k)
	}
}

// clientFor 按「核心 + 地址」复用 stats 客户端（地址变化时重建）。
// sing-box 必须用 V2Ray API 的服务名，否则一定 Unimplemented。
func (o *LocalObserver) clientFor(core, addr string, singbox bool) *coredata.XrayClient {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.clients == nil {
		o.clients = make(map[string]*coredata.XrayClient)
	}
	if o.clientAddrs == nil {
		o.clientAddrs = make(map[string]string)
	}
	if c := o.clients[core]; c != nil && o.clientAddrs[core] == addr {
		return c
	}
	if c := o.clients[core]; c != nil {
		c.Close()
	}
	var c *coredata.XrayClient
	if singbox {
		c = coredata.NewV2RayClient(addr)
	} else {
		c = coredata.NewXrayClient(addr)
	}
	o.clients[core] = c
	o.clientAddrs[core] = addr
	return c
}

func (o *LocalObserver) close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for core, c := range o.clients {
		if c != nil {
			c.Close()
		}
		delete(o.clients, core)
	}
}
