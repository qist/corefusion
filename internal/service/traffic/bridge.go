package traffic

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"corefusion/internal/store/repository"
)

// NetworkTrafficBridge consumes nftables traffic delta events and applies them to the
// subscription traffic cache. It ONLY accounts for subscriptions whose traffic is NOT
// already counted by xray itself (see isXrayAccounted), so xray and nftables accounting
// never double-count the same subscription.
type NetworkTrafficBridge struct {
	tracker     *SubscriptionIPTracker
	subRepo     *repository.SubscriptionRepository
	clientRepo  *repository.ClientRepository
	inboundRepo *repository.InboundConfigRepository
	lastTotal   sync.Map // subID -> last cumulative total bytes seen from nftables
}

// NewNetworkTrafficBridge creates a bridge wiring the IP tracker to the subscription
// traffic repository, skipping subscriptions already accounted by xray.
func NewNetworkTrafficBridge(
	tracker *SubscriptionIPTracker,
	subRepo *repository.SubscriptionRepository,
	clientRepo *repository.ClientRepository,
	inboundRepo *repository.InboundConfigRepository,
) *NetworkTrafficBridge {
	return &NetworkTrafficBridge{
		tracker:     tracker,
		subRepo:     subRepo,
		clientRepo:  clientRepo,
		inboundRepo: inboundRepo,
	}
}

// Run starts the nftables polling loop and consumes events until ctx is cancelled.
func (b *NetworkTrafficBridge) Run(ctx context.Context, interval time.Duration) {
	ch := make(chan TrafficDeltaEvent, 1024)
	b.tracker.SetEventChannel(ch)
	go b.tracker.PollAll(ctx, interval)

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			b.handleEvent(ctx, ev)
		}
	}
}

func (b *NetworkTrafficBridge) handleEvent(ctx context.Context, ev TrafficDeltaEvent) {
	// TotalBytes is the cumulative nftables counter; compute the delta since last poll.
	last, _ := b.lastTotal.LoadOrStore(ev.SubID, ev.TotalBytes)
	prev := last.(int64)
	delta := ev.TotalBytes - prev
	b.lastTotal.Store(ev.SubID, ev.TotalBytes)
	if delta <= 0 {
		return
	}

	// Skip subscriptions whose traffic is already accounted by xray itself.
	if b.isXrayAccounted(ctx, ev.SubID) {
		return
	}

	if err := b.subRepo.AddUsedBytes(ctx, ev.SubID, delta); err != nil {
		log.Printf("[nft-bridge] add used bytes for sub %d failed: %v", ev.SubID, err)
	}
}

// isXrayAccounted reports whether the subscription's client is configured inside an
// xray inbound config (i.e. xray already counts its traffic via QueryStats).
func (b *NetworkTrafficBridge) isXrayAccounted(ctx context.Context, subID int64) bool {
	sub, err := b.subRepo.GetByID(ctx, subID)
	if err != nil || sub == nil {
		return false
	}
	if sub.ClientID == 0 {
		return false
	}
	client, err := b.clientRepo.GetByID(ctx, sub.ClientID)
	if err != nil || client == nil || client.Email == "" {
		return false
	}
	configs, err := b.inboundRepo.ListAll(ctx, "xray")
	if err != nil {
		return false
	}
	for _, cfg := range configs {
		if strings.Contains(cfg.ConfigJSON, client.Email) {
			return true
		}
	}
	return false
}
