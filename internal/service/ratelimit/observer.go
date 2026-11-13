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

// LocalObserver 是**面板本机节点**的核心观测器。
//
// 远程节点的真实连接 IP 由 Agent 的 ONLINE_USERS 上报（→ SetOnlineUsersHook）；
// 但本地节点没有独立 Agent 进程（核心由面板进程内管理），所以本机必须自己
// 轮询 xray GetUsersStats，才能把「核心观测 IP」交给限速关联。
//
// 只在 serverID=1（本地节点）运行；核心未运行/未安装时直接跳过。
type LocalObserver struct {
	svc         *Service
	serverID    int64
	interval    time.Duration
	statsAddr   func(context.Context) string
	coreRunning func() bool

	mu          sync.Mutex
	client      *coredata.XrayClient
	clientAddr  string
	unsupported bool
	visState    string

	// 在线快照（面板在线订阅人/在线用户）：速率由两次采样差分得到。
	// trafficsync 会周期性 reset 计数器，出现回退时按新会话处理（同 Agent 侧口径）。
	rateMu  sync.Mutex
	samples map[string]*localOnlineSample
}

type localOnlineSample struct {
	down      int64
	up        int64
	ts        int64
	firstSeen int64
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
	}
	go o.loop(context.Background())
	log.Printf("[RateLimitObserved] local observer started (server=%d interval=%s)", serverID, interval)
	return o
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
	if o.statsAddr == nil || o.svc == nil || o.serverID <= 0 {
		return
	}
	o.mu.Lock()
	unsupported := o.unsupported
	o.mu.Unlock()
	if unsupported {
		return
	}
	if o.coreRunning != nil && !o.coreRunning() {
		return
	}

	addr := strings.TrimSpace(o.statsAddr(ctx))
	if addr == "" {
		return
	}
	client := o.clientFor(addr)
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
			o.unsupported = true
			o.mu.Unlock()
			log.Printf("[RateLimitObserved] xray GetUsersStats unsupported; local core-IP enforcement disabled")
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
		dlRate, ulRate, firstSeen := o.rateFor(u.GetEmail(), dl, ul, now)

		baseEmail := stripObservedEmailTag(u.GetEmail())
		subID, _ := o.svc.resolveObservedSub(ctx, ObservedUser{Email: u.GetEmail(), IPs: ips})
		entries = append(entries, &agentpb.OnlineUserEntry{
			SubscriptionId: subID,
			Email:          baseEmail,
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
	if len(users) == 0 {
		o.cleanupSamples(nil)
		// 空列表也要写快照：面板才能区分「节点未上报」与「节点 0 人在线」
		// （否则最后一批用户下线后列表会一直停在旧数据，直到 TTL 过期）。
		if o.svc.onlineSink != nil {
			o.svc.onlineSink(o.serverID, []*agentpb.OnlineUserEntry{})
		}
		return
	}
	if o.svc.onlineSink != nil {
		o.svc.onlineSink(o.serverID, entries)
	}
	o.cleanupSamples(entries)

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

	o.svc.HandleObservedUsers(ctx, o.serverID, users)
}

// rateFor 用两次采样的累计流量差分出 bps 速率（与 Agent 侧 online_users.go 同口径）。
// 计数器回退（trafficsync 周期性 reset / 核心重启）时按新会话处理，速率记 0。
func (o *LocalObserver) rateFor(email string, down, up, now int64) (dlRate, ulRate, firstSeen int64) {
	o.rateMu.Lock()
	defer o.rateMu.Unlock()
	if o.samples == nil {
		o.samples = make(map[string]*localOnlineSample)
	}
	prev := o.samples[email]
	if prev == nil {
		o.samples[email] = &localOnlineSample{down: down, up: up, ts: now, firstSeen: now}
		return 0, 0, now
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
	if down < prev.down || up < prev.up {
		firstSeen = now
	}
	prev.down, prev.up, prev.ts, prev.firstSeen = down, up, now, firstSeen
	return dlRate, ulRate, firstSeen
}

// cleanupSamples 清掉已下线的 email 采样，避免内存随历史用户增长。
func (o *LocalObserver) cleanupSamples(entries []*agentpb.OnlineUserEntry) {
	o.rateMu.Lock()
	defer o.rateMu.Unlock()
	if len(entries) == 0 {
		o.samples = make(map[string]*localOnlineSample)
		return
	}
	live := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		live[e.GetEmail()] = struct{}{}
	}
	// 采样 key 是完整 email（带 _tag），快照里存的是 base email：
	// 按"能匹配上任意存活 base email 的前缀"保留。
	for k := range o.samples {
		base := stripObservedEmailTag(k)
		if _, ok := live[base]; ok {
			continue
		}
		if _, ok := live[k]; ok {
			continue
		}
		delete(o.samples, k)
	}
}

// clientFor 按地址复用 stats 客户端（地址变化时重建）。
func (o *LocalObserver) clientFor(addr string) *coredata.XrayClient {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.client != nil && o.clientAddr == addr {
		return o.client
	}
	if o.client != nil {
		o.client.Close()
	}
	o.client = coredata.NewXrayClient(addr)
	o.clientAddr = addr
	return o.client
}

func (o *LocalObserver) close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.client != nil {
		o.client.Close()
		o.client = nil
	}
}
