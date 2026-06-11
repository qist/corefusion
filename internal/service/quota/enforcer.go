package quota

import (
	"context"
	"log"
	"os/exec"
	"time"

	"corefusion/internal/domain"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// Enforcer enforces quota decisions by blocking/unblocking users.
type Enforcer struct {
	coreManagers map[domain.CoreType]CoreUserManager
	subRepo      *repository.SubscriptionRepository
	ipTracker    IPTracker
}

// CoreUserManager manages users in a core.
type CoreUserManager interface {
	RemoveUser(ctx context.Context, serviceID int64, userIdentifier string) error
	RestoreUser(ctx context.Context, serviceID int64, userIdentifier string) error
}

// IPTracker tracks subscription IPs.
type IPTracker interface {
	GetIPs(subID int64) []string
	BlockSubscription(subID int64)
	UnblockSubscription(subID int64)
}

// NewEnforcer creates a new quota enforcer.
func NewEnforcer(subRepo *repository.SubscriptionRepository, ipTracker IPTracker) *Enforcer {
	return &Enforcer{
		coreManagers: make(map[domain.CoreType]CoreUserManager),
		subRepo:      subRepo,
		ipTracker:    ipTracker,
	}
}

// RegisterCoreManager registers a core user manager.
func (e *Enforcer) RegisterCoreManager(coreType domain.CoreType, manager CoreUserManager) {
	e.coreManagers[coreType] = manager
}

// OnQuotaStateChanged handles a quota state change for a subscription.
func (e *Enforcer) OnQuotaStateChanged(ctx context.Context, subID int64, oldState, newState domain.QuotaState) {
	log.Printf("subscription %d quota state: %s -> %s", subID, oldState, newState)

	switch newState {
	case domain.QuotaStateOverLimitSoft:
		// Soft reject: block via nftables (new connections blocked, existing continue)
		if e.ipTracker != nil {
			e.ipTracker.BlockSubscription(subID)
		}

	case domain.QuotaStateOverLimitHard:
		// Hard reject: block via nftables + flush conntrack
		if e.ipTracker != nil {
			e.ipTracker.BlockSubscription(subID)
		}
		e.flushConntrack(subID)

	case domain.QuotaStateSuspended:
		// Admin suspended: same as hard reject
		if e.ipTracker != nil {
			e.ipTracker.BlockSubscription(subID)
		}
		e.flushConntrack(subID)

	case domain.QuotaStateExpiredDays:
		// Max active days expired: block
		if e.ipTracker != nil {
			e.ipTracker.BlockSubscription(subID)
		}
		e.flushConntrack(subID)

	case domain.QuotaStateNormal, domain.QuotaStateRestored:
		// Restored: unblock
		if e.ipTracker != nil {
			e.ipTracker.UnblockSubscription(subID)
		}
	}
}

// flushConntrack clears conntrack entries for a subscription's IPs.
func (e *Enforcer) flushConntrack(subID int64) {
	if e.ipTracker == nil {
		return
	}

	ips := e.ipTracker.GetIPs(subID)
	for _, ip := range ips {
		// conntrack -D -s <ip>
		cmd := exec.Command("conntrack", "-D", "-s", ip)
		if err := cmd.Run(); err != nil {
			// conntrack may return non-zero if no entries found, not fatal
			log.Printf("conntrack flush for %s: %v", ip, err)
		}
	}
	log.Printf("flushed conntrack for subscription %d (%d IPs)", subID, len(ips))
}

// SyncAllStates checks all subscriptions and enforces quota state.
func (e *Enforcer) SyncAllStates(ctx context.Context) {
	// Find subscriptions that need blocking
	subs := e.findSubscriptionsToBlock(ctx)
	for _, sub := range subs {
		e.OnQuotaStateChanged(ctx, sub.ID, domain.QuotaStateNormal, domain.QuotaState(sub.QuotaState))
	}

	// Find subscriptions that need unblocking
	restored := e.findSubscriptionsToRestore(ctx)
	for _, sub := range restored {
		e.OnQuotaStateChanged(ctx, sub.ID, domain.QuotaState(sub.QuotaState), domain.QuotaStateNormal)
	}
}

func (e *Enforcer) findSubscriptionsToBlock(ctx context.Context) []entity.Subscription {
	subs, _ := e.subRepo.ListByQuotaStates(ctx, []string{
		string(domain.QuotaStateOverLimitSoft),
		string(domain.QuotaStateOverLimitHard),
		string(domain.QuotaStateSuspended),
		string(domain.QuotaStateExpiredDays),
	})
	return subs
}

func (e *Enforcer) findSubscriptionsToRestore(ctx context.Context) []entity.Subscription {
	subs, _ := e.subRepo.ListByQuotaStates(ctx, []string{
		string(domain.QuotaStateRestored),
	})
	return subs
}

// RunSyncLoop runs the sync loop periodically.
func (e *Enforcer) RunSyncLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.SyncAllStates(ctx)
		}
	}
}
