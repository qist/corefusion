package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/coredata"
)

// onlineUsersReporter 定期查询 xray GetUsersStats，计算实时速率，
// 并产出 OnlineUserEntry 列表供 Agent 通过心跳流上报面板。
//
// 它与限速解耦：即使节点未开启限速也能工作（不需要 nft 可用）。
// 流量是订阅级（xray 只提供 user 级累计流量），速率由两次采样差分计算。
type onlineUsersReporter struct {
	statsAddr string
	client    *coredata.XrayClient

	mu       sync.RWMutex
	emailMap map[string]int64 // xray client email (baseEmail) -> subscriptionID

	// 速率差分：email -> 上次采样（累计流量 + 时间 + 首次出现）
	prevMu     sync.Mutex
	prevTraffic map[string]*trafficSample

	unsupportedOnce sync.Once
}

type trafficSample struct {
	downlink  int64
	uplink    int64
	ts        int64 // unix 秒
	firstSeen int64
}

func newOnlineUsersReporter(statsAddr string) *onlineUsersReporter {
	if statsAddr == "" {
		statsAddr = coredata.DefaultStatsAddr
	}
	return &onlineUsersReporter{
		statsAddr:   statsAddr,
		client:      coredata.NewXrayClient(statsAddr),
		emailMap:    make(map[string]int64),
		prevTraffic: make(map[string]*trafficSample),
	}
}

// SetEmailMapping replaces the email -> subscriptionID mapping.
func (r *onlineUsersReporter) SetEmailMapping(m map[string]int64) {
	r.mu.Lock()
	r.emailMap = m
	r.mu.Unlock()
}

// markUnsupported logs the "GetUsersStats unsupported" error exactly once.
func (r *onlineUsersReporter) markUnsupported() {
	r.unsupportedOnce.Do(func() {
		log.Printf("[Agent] xray GetUsersStats unsupported by this xray version; online-users reporting disabled")
	})
}

// Collect queries xray and returns the current online users (rate-differentiated).
// Returns (nil, nil) when there are no online users; (nil, err) on transport error.
func (r *onlineUsersReporter) Collect(ctx context.Context) ([]*agentpb.OnlineUserEntry, error) {
	resp, err := r.client.GetUsersStats(ctx, true)
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

	r.mu.RLock()
	emailMap := make(map[string]int64, len(r.emailMap))
	for k, v := range r.emailMap {
		emailMap[k] = v
	}
	r.mu.RUnlock()

	now := time.Now().Unix()
	seen := make(map[string]struct{}, len(users))
	entries := make([]*agentpb.OnlineUserEntry, 0, len(users))

	for _, user := range users {
		email := user.GetEmail()
		if email == "" || len(user.GetIps()) == 0 {
			continue
		}

		// 解析订阅 ID：先剥离 shortTag 匹配映射，再尝试从 email 提取数字
		baseEmail := stripShortTag(email)
		subID := emailMap[baseEmail]
		if subID == 0 {
			subID = extractSubIDFromEmail(baseEmail)
		}
		if subID == 0 {
			continue
		}
		seen[baseEmail] = struct{}{}

		// 速率差分（订阅级累计流量）
		var dl, ul int64
		if t := user.GetTraffic(); t != nil {
			dl = t.GetDownlink()
			ul = t.GetUplink()
		}
		dlRate, ulRate, firstSeen := r.diffRate(baseEmail, dl, ul, now)

		// 组装 session（每个 IP 一行）
		sessions := make([]*agentpb.OnlineSession, 0, len(user.GetIps()))
		var lastSeen int64
		for _, ip := range user.GetIps() {
			ipStr := ip.GetIp()
			if ipStr == "" || isPrivateOrLoopback(ipStr) {
				continue
			}
			ls := ip.GetLastSeen()
			if ls > lastSeen {
				lastSeen = ls
			}
			sessions = append(sessions, &agentpb.OnlineSession{
				Ip:            ipStr,
				ConnectedSince: firstSeen, // 近似：xray 不返回接入时间
				LastSeen:      ls,
			})
		}
		if len(sessions) == 0 {
			continue
		}
		if lastSeen == 0 {
			lastSeen = now
		}

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
	}

	r.cleanupStale(seen)

	if len(entries) == 0 {
		return nil, nil
	}
	return entries, nil
}

// diffRate computes bps rates from cumulative traffic using the last sample.
func (r *onlineUsersReporter) diffRate(email string, dl, ul, now int64) (dlRate, ulRate, firstSeen int64) {
	r.prevMu.Lock()
	defer r.prevMu.Unlock()
	prev, ok := r.prevTraffic[email]
	if !ok {
		// 首次采样：速率未知，仅记录基线
		r.prevTraffic[email] = &trafficSample{downlink: dl, uplink: ul, ts: now, firstSeen: now}
		return 0, 0, now
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
	// xray 重启会导致计数器归零（dl/ul < prev），按新会话处理
	if dl < prev.downlink || ul < prev.uplink {
		firstSeen = now
	}
	r.prevTraffic[email] = &trafficSample{downlink: dl, uplink: ul, ts: now, firstSeen: firstSeen}
	return dlRate, ulRate, firstSeen
}

// cleanupStale removes differential baselines for emails no longer online.
func (r *onlineUsersReporter) cleanupStale(seen map[string]struct{}) {
	if len(seen) == 0 {
		r.prevMu.Lock()
		r.prevTraffic = make(map[string]*trafficSample)
		r.prevMu.Unlock()
		return
	}
	r.prevMu.Lock()
	for email := range r.prevTraffic {
		if _, ok := seen[email]; !ok {
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

// isPrivateOrLoopback reports whether an IP is private/loopback/unspecified.
func isPrivateOrLoopback(ip string) bool {
	return strings.HasPrefix(ip, "127.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "169.254.") ||
		ip == "::1" || ip == "0.0.0.0" || ip == "::" ||
		strings.HasPrefix(ip, "fe80:")
}

// sendOnlineUsersReport collects online users and sends them to the Panel
// via the heartbeat stream as an ONLINE_USERS report.
func (a *AgentApp) sendOnlineUsersReport(stream agentpb.HeartbeatService_StreamClient) error {
	if a.onlineUsersReporter == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	entries, err := a.onlineUsersReporter.Collect(ctx)
	if err != nil {
		// 传输错误不应中断心跳流
		return err
	}

	// 空列表也发送：让面板刷新"节点在线但当前无订阅人"的快照时间戳，
	// 否则面板无法区分"节点未上报"与"节点 0 人在线"。
	if entries == nil {
		entries = []*agentpb.OnlineUserEntry{}
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
