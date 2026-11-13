package ratelimit

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"corefusion/internal/coredata"
	"corefusion/pkg/netutil"
)

// OnlineStatsPoller polls the Xray Stats API for online user IPs
// and automatically associates them with rate limit rules.
//
// This is the P1 trigger source: when Xray's statsUserOnline policy is enabled,
// the poller periodically queries GetUsersStats to discover which IPs are
// currently active, then ensures their rate limit rules are applied.
type OnlineStatsPoller struct {
	mgr       *Manager
	statsAddr string
	client    *coredata.XrayClient

	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc

	// email → subscriptionID mapping, populated by the Panel via SyncData
	emailToSubID map[string]int64

	// How often to poll the Stats API
	pollInterval time.Duration

	// shouldPoll 可选门禁：返回 false 时跳过本轮探测。
	// 用于「xray 未安装/未运行就不去连 10085」——否则每 30s 刷一次连接失败日志。
	shouldPoll func() bool
	// onResult 可选：回填本轮探测结果，供门禁做连续失败退避。
	onResult func(ok bool)
}

// NewOnlineStatsPoller creates a new online stats poller.
func NewOnlineStatsPoller(mgr *Manager, statsAddr string) *OnlineStatsPoller {
	if statsAddr == "" {
		statsAddr = coredata.DefaultStatsAddr
	}
	return &OnlineStatsPoller{
		mgr:          mgr,
		statsAddr:    statsAddr,
		client:       coredata.NewXrayClient(statsAddr),
		emailToSubID: make(map[string]int64),
		pollInterval: 30 * time.Second,
	}
}

// WithPollInterval sets the poll interval.
func (p *OnlineStatsPoller) WithPollInterval(d time.Duration) *OnlineStatsPoller {
	p.pollInterval = d
	return p
}

// WithShouldPoll sets a gate called before each poll; returning false skips the round.
func (p *OnlineStatsPoller) WithShouldPoll(fn func() bool) *OnlineStatsPoller {
	p.shouldPoll = fn
	return p
}

// WithResultCallback sets a callback receiving each poll result (for backoff).
func (p *OnlineStatsPoller) WithResultCallback(fn func(ok bool)) *OnlineStatsPoller {
	p.onResult = fn
	return p
}

// SetEmailMapping sets the email → subscriptionID mapping.
// This is called by the Panel when syncing data to the Agent.
// 同时登记完整 email 与去掉 _tag 后缀的 base email（xray 返回完整 form，
// 而 base email 才是面向客户的邮箱）。
func (p *OnlineStatsPoller) SetEmailMapping(mapping map[string]int64) {
	normalized := make(map[string]int64, len(mapping)*2)
	for email, subID := range mapping {
		if email == "" || subID <= 0 {
			continue
		}
		normalized[email] = subID
		if base := stripTagSuffix(email); base != email {
			if _, exists := normalized[base]; !exists {
				normalized[base] = subID
			}
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.emailToSubID = normalized
}

// UpdateEmailMapping adds or updates a single email → subscriptionID mapping.
func (p *OnlineStatsPoller) UpdateEmailMapping(email string, subID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.emailToSubID[email] = subID
}

// Start starts the polling loop.
func (p *OnlineStatsPoller) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	p.cancelFn = cancel

	go p.pollLoop(ctx)
	log.Printf("[OnlineStatsPoller] started (interval=%s, stats_addr=%s)", p.pollInterval, p.statsAddr)
}

// Stop stops the polling loop.
func (p *OnlineStatsPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancelFn != nil {
		p.cancelFn()
		p.cancelFn = nil
	}
	p.running = false
	if p.client != nil {
		p.client.Close()
	}
}

func (p *OnlineStatsPoller) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Initial poll after a short delay
	time.Sleep(5 * time.Second)
	p.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *OnlineStatsPoller) poll(ctx context.Context) {
	if p.mgr == nil || !p.mgr.IsAvailable().NftAvailable {
		return
	}
	// 门禁：核未安装/未运行时不探测（避免无意义连接与日志刷屏）
	if p.shouldPoll != nil && !p.shouldPoll() {
		return
	}

	// Call GetUsersStats to get online IPs per user
	resp, err := p.client.GetUsersStats(ctx, false)
	if p.onResult != nil {
		p.onResult(err == nil)
	}
	if err != nil {
		// Unimplemented means the xray version doesn't support this API
		if strings.Contains(err.Error(), "Unimplemented") {
			log.Printf("[OnlineStatsPoller] GetUsersStats not supported by this xray version, disabling poller")
			p.Stop()
			return
		}
		log.Printf("[OnlineStatsPoller] GetUsersStats failed: %v", err)
		return
	}

	if len(resp.Users) == 0 {
		return
	}

	// Get current email mapping
	p.mu.Lock()
	emailMap := make(map[string]int64, len(p.emailToSubID))
	for k, v := range p.emailToSubID {
		emailMap[k] = v
	}
	p.mu.Unlock()

	// Process each online user
	applied := 0
	for _, user := range resp.Users {
		if user.Email == "" || len(user.Ips) == 0 {
			continue
		}

		subID, ok := emailMap[user.Email]
		if !ok {
			subID, ok = emailMap[stripTagSuffix(user.Email)]
		}
		if !ok {
			continue // no subscription mapping for this email
		}

		// For each IP, ensure rate limit rules are applied
		for _, ipEntry := range user.Ips {
			if ipEntry.Ip == "" {
				continue
			}
			// 只处理公网客户端地址：回环/私网/链路本地/ULA/CGNAT 不能作为限速键。
			if !netutil.IsPublicClientIP(ipEntry.Ip) {
				continue
			}

			// Touch the IP in the manager (update last seen, apply rules if not already)
			if err := p.mgr.TouchIP(ctx, ipEntry.Ip, subID); err != nil {
				log.Printf("[OnlineStatsPoller] touch ip=%s sub=%d failed: %v", ipEntry.Ip, subID, err)
				continue
			}
			applied++
		}
	}

	if applied > 0 {
		log.Printf("[OnlineStatsPoller] processed %d online IPs from %d users", applied, len(resp.Users))
	}
}

// stripTagSuffix 去掉面板账号 email 末尾的 _<tagSuffix>
// （user@example.com_vless12345 → user@example.com）。tag 段不含 '@'。
func stripTagSuffix(email string) string {
	if idx := strings.LastIndex(email, "_"); idx > 0 {
		tail := email[idx+1:]
		if !strings.Contains(tail, "@") {
			return email[:idx]
		}
	}
	return email
}
