package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/pkg/netutil"
)

const (
	// loginMaxFailures is how many consecutive failures a key may accumulate
	// before it is locked out.
	loginMaxFailures = 5
	// loginLockout is how long a key stays locked once it trips the limit.
	loginLockout = 15 * time.Minute
	// loginFailureWindow is how long a single failure is remembered. Failures
	// older than this decay, so occasional typos never accumulate into a ban.
	loginFailureWindow = 15 * time.Minute
)

type loginAttempt struct {
	failures    []time.Time
	lockedUntil time.Time
}

// LoginLimiter tracks failed login attempts per key (client IP) and blocks
// further attempts once the threshold is exceeded.
//
// State is in-process and therefore per-instance: behind multiple replicas the
// effective limit is the threshold times the replica count. That is acceptable
// for slowing down credential stuffing; it is not a distributed lock. Moving
// this to Redis would make it exact, and is the natural upgrade path if the
// admin API is ever run multi-replica.
type LoginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
	lastGC   time.Time
}

// NewLoginLimiter creates a login rate limiter.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		attempts: make(map[string]*loginAttempt),
		lastGC:   time.Now(),
	}
}

// prune drops failures that have aged out of the window. Callers must hold mu.
func (l *LoginLimiter) prune(a *loginAttempt, now time.Time) {
	cutoff := now.Add(-loginFailureWindow)
	kept := a.failures[:0]
	for _, ts := range a.failures {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	a.failures = kept
}

// gc removes entries that are fully expired so the map cannot grow without
// bound under a rotating-IP attack. Callers must hold mu.
func (l *LoginLimiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < time.Minute {
		return
	}
	l.lastGC = now
	for key, a := range l.attempts {
		l.prune(a, now)
		if len(a.failures) == 0 && now.After(a.lockedUntil) {
			delete(l.attempts, key)
		}
	}
}

// Allowed reports whether key may attempt a login, and if not, how long until
// it may try again.
func (l *LoginLimiter) Allowed(key string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.gc(now)

	a, ok := l.attempts[key]
	if !ok {
		return true, 0
	}
	if now.Before(a.lockedUntil) {
		return false, time.Until(a.lockedUntil)
	}
	return true, 0
}

// RecordFailure registers a failed login for key, locking it out if the
// threshold is reached.
func (l *LoginLimiter) RecordFailure(key string) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[key]
	if !ok {
		a = &loginAttempt{}
		l.attempts[key] = a
	}

	l.prune(a, now)
	a.failures = append(a.failures, now)

	if len(a.failures) >= loginMaxFailures {
		a.lockedUntil = now.Add(loginLockout)
		a.failures = nil
	}
}

// RecordSuccess clears the failure history for key.
func (l *LoginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

var defaultLoginLimiter = NewLoginLimiter()

// DefaultLoginLimiter returns the process-wide login limiter, so handlers can
// report success/failure against the same state the middleware enforces.
func DefaultLoginLimiter() *LoginLimiter {
	return defaultLoginLimiter
}

// LoginRateLimit rejects login attempts from a client that has recently failed
// too many times. Mount it on the login route only.
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ⚠ 不能用 `c.ClientIP()` ✗：Gin 默认信任所有代理（trustedProxies 为 0.0.0.0/0），
		// 调用方每次换一个 `X-Forwarded-For` 就能换掉限流键 → 口令爆破无实际限制
		// （安全审计 v5 S3）。GetRealIP 只在"直连方是可信代理"时才采信 XFF。
		key := netutil.GetRealIP(c.Request)
		if allowed, retryAfter := defaultLoginLimiter.Allowed(key); !allowed {
			response.TooManyRequests(c, "TOO_MANY_ATTEMPTS",
				"too many failed login attempts, retry in "+retryAfter.Round(time.Second).String())
			c.Abort()
			return
		}
		c.Next()
	}
}

// --- Client (portal) login rate limiter ---
//
// Portal users have a larger user base than admins, so we use more lenient
// thresholds: 10 failures in 30 minutes → 30 minute lockout. This still
// slows down credential stuffing without overly penalizing legitimate users
// who mistype their password.

const (
	clientLoginMaxFailures   = 10
	clientLoginLockout       = 30 * time.Minute
	clientLoginFailureWindow = 30 * time.Minute
)

type clientLoginLimiterType struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
	lastGC   time.Time
}

func newClientLoginLimiter() *clientLoginLimiterType {
	return &clientLoginLimiterType{
		attempts: make(map[string]*loginAttempt),
		lastGC:   time.Now(),
	}
}

func (l *clientLoginLimiterType) prune(a *loginAttempt, now time.Time) {
	cutoff := now.Add(-clientLoginFailureWindow)
	kept := a.failures[:0]
	for _, ts := range a.failures {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	a.failures = kept
}

func (l *clientLoginLimiterType) gc(now time.Time) {
	if now.Sub(l.lastGC) < time.Minute {
		return
	}
	l.lastGC = now
	for key, a := range l.attempts {
		l.prune(a, now)
		if len(a.failures) == 0 && now.After(a.lockedUntil) {
			delete(l.attempts, key)
		}
	}
}

func (l *clientLoginLimiterType) Allowed(key string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gc(now)

	a, ok := l.attempts[key]
	if !ok {
		return true, 0
	}
	if now.Before(a.lockedUntil) {
		return false, time.Until(a.lockedUntil)
	}
	return true, 0
}

func (l *clientLoginLimiterType) RecordFailure(key string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.attempts[key]
	if !ok {
		a = &loginAttempt{}
		l.attempts[key] = a
	}

	l.prune(a, now)
	a.failures = append(a.failures, now)

	if len(a.failures) >= clientLoginMaxFailures {
		a.lockedUntil = now.Add(clientLoginLockout)
		a.failures = nil
	}
}

func (l *clientLoginLimiterType) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

var defaultClientLoginLimiter = newClientLoginLimiter()

// DefaultClientLoginLimiter returns the process-wide client login limiter.
func DefaultClientLoginLimiter() *clientLoginLimiterType {
	return defaultClientLoginLimiter
}

// ClientLoginRateLimit rejects client login attempts from an IP that has
// recently failed too many times. More lenient than admin limiter.
func ClientLoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 同上：必须与 GetRealIP 口径一致，否则限流键可被 XFF 伪造绕过。
		key := netutil.GetRealIP(c.Request)
		if allowed, retryAfter := defaultClientLoginLimiter.Allowed(key); !allowed {
			response.TooManyRequests(c, "TOO_MANY_ATTEMPTS",
				"too many failed login attempts, retry in "+retryAfter.Round(time.Second).String())
			c.Abort()
			return
		}
		c.Next()
	}
}
