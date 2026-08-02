package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"corefusion/internal/coredata"
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

// SetEmailMapping sets the email → subscriptionID mapping.
// This is called by the Panel when syncing data to the Agent.
func (p *OnlineStatsPoller) SetEmailMapping(mapping map[string]int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.emailToSubID = mapping
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

	// Call GetUsersStats to get online IPs per user
	resp, err := p.client.GetUsersStats(ctx, false)
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
			continue // no subscription mapping for this email
		}

		// For each IP, ensure rate limit rules are applied
		for _, ipEntry := range user.Ips {
			if ipEntry.Ip == "" {
				continue
			}
			// Skip localhost and private IPs
			if isPrivateIP(ipEntry.Ip) {
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

// ParseOnlineStatsFromXrayAPI parses the JSON response from the xray API proxy
// and extracts online user IPs. This is used when the poller runs through the
// Panel's XrayAPIProxy instead of direct gRPC connection.
func ParseOnlineStatsFromXrayAPI(responseJSON []byte) (map[string][]string, error) {
	var resp struct {
		Users []struct {
			Email string `json:"email"`
			IPs   []struct {
				IP       string `json:"ip"`
				LastSeen int64  `json:"last_seen"`
			} `json:"ips"`
		} `json:"users"`
	}

	if err := json.Unmarshal(responseJSON, &resp); err != nil {
		return nil, fmt.Errorf("parse online stats response: %w", err)
	}

	result := make(map[string][]string)
	for _, user := range resp.Users {
		for _, ip := range user.IPs {
			if ip.IP != "" && !isPrivateIP(ip.IP) {
				result[user.Email] = append(result[user.Email], ip.IP)
			}
		}
	}
	return result, nil
}

// isPrivateIP checks if an IP is a private/loopback address.
func isPrivateIP(ip string) bool {
	return strings.HasPrefix(ip, "127.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "192.168.") ||
		ip == "::1" ||
		ip == "0.0.0.0" ||
		ip == "::"
}
