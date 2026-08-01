package middleware

import (
	"testing"
	"time"
)

func TestLimiterAllowsUntilThreshold(t *testing.T) {
	l := NewLoginLimiter()

	for i := 0; i < loginMaxFailures-1; i++ {
		if allowed, _ := l.Allowed("1.2.3.4"); !allowed {
			t.Fatalf("locked out early after %d failures", i)
		}
		l.RecordFailure("1.2.3.4")
	}

	if allowed, _ := l.Allowed("1.2.3.4"); !allowed {
		t.Fatal("should still be allowed just below the threshold")
	}
}

func TestLimiterLocksOutAtThreshold(t *testing.T) {
	l := NewLoginLimiter()

	for i := 0; i < loginMaxFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}

	allowed, retryAfter := l.Allowed("1.2.3.4")
	if allowed {
		t.Fatal("expected lockout once the threshold is reached")
	}
	if retryAfter <= 0 || retryAfter > loginLockout {
		t.Fatalf("implausible retryAfter: %v", retryAfter)
	}
}

// A successful login must clear the history, so a user who mistypes a few
// times and then succeeds is not penalised on their next session.
func TestLimiterSuccessResetsFailures(t *testing.T) {
	l := NewLoginLimiter()

	for i := 0; i < loginMaxFailures-1; i++ {
		l.RecordFailure("1.2.3.4")
	}
	l.RecordSuccess("1.2.3.4")

	for i := 0; i < loginMaxFailures-1; i++ {
		if allowed, _ := l.Allowed("1.2.3.4"); !allowed {
			t.Fatalf("history was not cleared by success (i=%d)", i)
		}
		l.RecordFailure("1.2.3.4")
	}
}

// Lockout must be scoped per key, or one attacker could deny service to
// everyone else.
func TestLimiterIsolatesKeys(t *testing.T) {
	l := NewLoginLimiter()

	for i := 0; i < loginMaxFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}

	if allowed, _ := l.Allowed("5.6.7.8"); !allowed {
		t.Fatal("unrelated key must not be affected by another key's lockout")
	}
}

// Failures older than the window decay, so occasional typos spread over time
// never accumulate into a ban.
func TestLimiterFailuresDecay(t *testing.T) {
	l := NewLoginLimiter()

	stale := time.Now().Add(-loginFailureWindow - time.Minute)
	l.attempts["1.2.3.4"] = &loginAttempt{
		failures: []time.Time{stale, stale, stale, stale},
	}

	// One fresh failure brings the raw count to 5, but the four stale ones are
	// outside the window, so this must not trip the lockout.
	l.RecordFailure("1.2.3.4")

	if allowed, _ := l.Allowed("1.2.3.4"); !allowed {
		t.Fatal("stale failures should have decayed instead of causing a lockout")
	}
}
