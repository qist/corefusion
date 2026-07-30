package access

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"corefusion/internal/domain/networkpolicy"
	"corefusion/internal/service/audit"
)

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	rate      float64
	maxTokens float64
	tokens    float64
	lastTime  time.Time
	mu        sync.Mutex
}

// NewTokenBucket creates a new token bucket.
func NewTokenBucket(ratePerSec, maxTokens float64) *TokenBucket {
	return &TokenBucket{
		rate:      ratePerSec,
		maxTokens: maxTokens,
		tokens:    maxTokens,
		lastTime:  time.Now(),
	}
}

// Allow checks if a request is allowed.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.tokens += now.Sub(b.lastTime).Seconds() * b.rate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastTime = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimitManager manages per-IP rate limiting.
type RateLimitManager struct {
	buckets sync.Map // key: "svc:ep:ip" -> *TokenBucket
}

// NewRateLimitManager creates a new rate limit manager.
func NewRateLimitManager() *RateLimitManager {
	return &RateLimitManager{}
}

// Check checks if a new connection is allowed by rate limit.
func (m *RateLimitManager) Check(serviceID, endpointID int64, ip string, maxPerMin int) bool {
	if maxPerMin <= 0 {
		return true
	}

	key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)
	rate := float64(maxPerMin) / 60.0

	v, loaded := m.buckets.LoadOrStore(key, NewTokenBucket(rate, rate*10))
	bucket := v.(*TokenBucket)

	if !loaded {
		// New bucket, allow first request
		return true
	}

	return bucket.Allow()
}

// ConcurrentTracker tracks concurrent connections per IP.
type ConcurrentTracker struct {
	counters sync.Map // key: "svc:ep:ip" -> *atomic.Int64
}

// NewConcurrentTracker creates a new concurrent tracker.
func NewConcurrentTracker() *ConcurrentTracker {
	return &ConcurrentTracker{}
}

// TryConnect tries to increment the concurrent counter.
func (t *ConcurrentTracker) TryConnect(serviceID, endpointID int64, ip string, maxConcurrent int) bool {
	if maxConcurrent <= 0 {
		return true
	}

	key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)
	v, _ := t.counters.LoadOrStore(key, &atomic.Int64{})
	counter := v.(*atomic.Int64)

	for {
		current := counter.Load()
		if current >= int64(maxConcurrent) {
			return false
		}
		if counter.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// OnDisconnect decrements the concurrent counter.
func (t *ConcurrentTracker) OnDisconnect(serviceID, endpointID int64, ip string) {
	key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)
	if v, ok := t.counters.Load(key); ok {
		v.(*atomic.Int64).Add(-1)
	}
}

// AutoBanManager manages automatic IP banning.
type AutoBanManager struct {
	violations sync.Map // key: "svc:ep:ip" -> *ViolationCounter
	banRepo    BanRepository
}

// ViolationCounter tracks violation count for an IP.
type ViolationCounter struct {
	Count    int
	FirstAt  time.Time
	LastAt   time.Time
	BannedAt *time.Time
}

// NewAutoBanManager creates a new auto ban manager.
func NewAutoBanManager(banRepo BanRepository) *AutoBanManager {
	return &AutoBanManager{banRepo: banRepo}
}

// OnRateLimitHit records a rate limit violation.
func (m *AutoBanManager) OnRateLimitHit(ctx context.Context, serviceID, endpointID int64, ip string, autoBanEnabled bool, banDuration time.Duration, maxViolations int) {
	if !autoBanEnabled {
		return
	}

	key := fmt.Sprintf("svc:%d:ep:%d:ip:%s", serviceID, endpointID, ip)
	now := time.Now()

	v, loaded := m.violations.LoadOrStore(key, &ViolationCounter{
		Count:   1,
		FirstAt: now,
		LastAt:  now,
	})

	vc := v.(*ViolationCounter)
	if loaded {
		vc.Count++
		vc.LastAt = now
	}

	// Check if should ban
	if vc.Count >= maxViolations && now.Sub(vc.FirstAt) < 5*time.Minute {
		m.banRepo.Create(ctx, &BanRecord{
			ServiceID:  serviceID,
			EndpointID: endpointID,
			IPCIDR:     ip + "/32",
			Reason:     fmt.Sprintf("auto_ban: rate limit exceeded %d times in 5 minutes", vc.Count),
			Source:     "auto_rate_limit",
			ExpiresAt:  now.Add(banDuration),
		})

		audit.Record(audit.Event{
			Category: "access", EventType: "ip_auto_banned", Result: "success",
			ServiceID: serviceID, EndpointID: endpointID, RemoteIP: ip,
			Reason: fmt.Sprintf("rate limit exceeded %d times in 5 minutes, banned for %s", vc.Count, banDuration),
		})

		bannedAt := now
		vc.BannedAt = &bannedAt
		vc.Count = 0
	}
}

// IsBanned checks if an IP is currently banned.
func (m *AutoBanManager) IsBanned(ctx context.Context, serviceID, endpointID int64, ip string) (bool, string) {
	ban := m.banRepo.FindActive(ctx, serviceID, endpointID, ip)
	if ban == nil {
		return false, ""
	}
	return true, ban.Reason
}

// BanRepository handles ban database operations.
type BanRepository interface {
	Create(ctx context.Context, ban *BanRecord) error
	FindActive(ctx context.Context, serviceID, endpointID int64, ip string) *BanRecord
	DeleteExpired(ctx context.Context) (int64, error)
}

// BanRecord represents an IP ban.
type BanRecord struct {
	ID         int64
	ServiceID  int64
	EndpointID int64
	IPCIDR     string
	Reason     string
	Source     string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// Service handles access control business logic.
type Service struct {
	rateLimit  *RateLimitManager
	concurrent *ConcurrentTracker
	autoBan    *AutoBanManager
	policyRepo PolicyRepository
	whitelists sync.Map // key: "svc:ep" -> map[string]bool
	blacklists sync.Map // key: "svc:ep" -> map[string]bool
}

// NewService creates a new access control service.
func NewService(banRepo BanRepository, policyRepo PolicyRepository) *Service {
	return &Service{
		rateLimit:  NewRateLimitManager(),
		concurrent: NewConcurrentTracker(),
		autoBan:    NewAutoBanManager(banRepo),
		policyRepo: policyRepo,
	}
}

// CheckConnection checks if a connection should be allowed.
func (s *Service) CheckConnection(ctx context.Context, ac networkpolicy.AccessContext) error {
	// 1. Check whitelist
	if s.isWhitelisted(ac.ServiceID, ac.EndpointID, ac.RemoteIP) {
		return nil
	}

	// 2. Check blacklist
	if s.isBlacklisted(ac.ServiceID, ac.EndpointID, ac.RemoteIP) {
		audit.Record(audit.Event{
			Category: "access", EventType: "connection_denied", Result: "denied",
			ServiceID: ac.ServiceID, EndpointID: ac.EndpointID, RemoteIP: ac.RemoteIP,
			Reason: "blacklisted",
		})
		return networkpolicy.ErrAccessDeniedByPolicy
	}

	// 3. Check ban
	if banned, reason := s.autoBan.IsBanned(ctx, ac.ServiceID, ac.EndpointID, ac.RemoteIP); banned {
		audit.Record(audit.Event{
			Category: "access", EventType: "connection_denied", Result: "denied",
			ServiceID: ac.ServiceID, EndpointID: ac.EndpointID, RemoteIP: ac.RemoteIP,
			Reason: "banned: " + reason,
		})
		return networkpolicy.ErrAccessDeniedByBan
	}

	// 4 & 5. Evaluate access policies: concurrent and rate limits.
	if s.policyRepo != nil {
		policies, err := s.policyRepo.ListActive(ctx, ac.ServiceID, ac.EndpointID)
		if err == nil {
			for i := range policies {
				p := &policies[i]
				if p.Status != 1 {
					continue
				}
				if p.MaxNewConnectionsPerMin > 0 {
					if !s.rateLimit.Check(ac.ServiceID, ac.EndpointID, ac.RemoteIP, p.MaxNewConnectionsPerMin) {
						if p.AutoBanEnabled == 1 {
							s.autoBan.OnRateLimitHit(ctx, ac.ServiceID, ac.EndpointID, ac.RemoteIP,
								true, time.Duration(p.BanDurationSeconds)*time.Second, 5)
						}
						audit.Record(audit.Event{
							Category: "access", EventType: "rate_limit_exceeded", Result: "denied",
							ServiceID: ac.ServiceID, EndpointID: ac.EndpointID, RemoteIP: ac.RemoteIP,
							Reason: fmt.Sprintf("max_new_connections_per_min=%d exceeded", p.MaxNewConnectionsPerMin),
						})
						return networkpolicy.ErrAccessDeniedByRate
					}
				}
				if p.MaxConcurrentConnections > 0 {
					if !s.concurrent.TryConnect(ac.ServiceID, ac.EndpointID, ac.RemoteIP, p.MaxConcurrentConnections) {
						audit.Record(audit.Event{
							Category: "access", EventType: "concurrent_limit_exceeded", Result: "denied",
							ServiceID: ac.ServiceID, EndpointID: ac.EndpointID, RemoteIP: ac.RemoteIP,
							Reason: fmt.Sprintf("max_concurrent_connections=%d exceeded", p.MaxConcurrentConnections),
						})
						return networkpolicy.ErrAccessDeniedByRate
					}
				}
			}
		}
	}

	return nil
}

func (s *Service) isWhitelisted(serviceID, endpointID int64, ip string) bool {
	key := fmt.Sprintf("svc:%d:ep:%d", serviceID, endpointID)
	if v, ok := s.whitelists.Load(key); ok {
		return v.(map[string]bool)[ip]
	}
	return false
}

func (s *Service) isBlacklisted(serviceID, endpointID int64, ip string) bool {
	key := fmt.Sprintf("svc:%d:ep:%d", serviceID, endpointID)
	if v, ok := s.blacklists.Load(key); ok {
		return v.(map[string]bool)[ip]
	}
	return false
}
