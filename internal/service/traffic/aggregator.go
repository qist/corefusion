package traffic

import (
	"context"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/domain/metric"
	"corefusion/internal/model/entity"
)

// Aggregator aggregates traffic delta events into the database.
type Aggregator struct {
	db           *gorm.DB
	eventCh      chan metric.TrafficDeltaEvent
	batchSize    int
	flushInterval time.Duration
	mu           sync.Mutex
}

// NewAggregator creates a new traffic aggregator.
func NewAggregator(db *gorm.DB) *Aggregator {
	return &Aggregator{
		db:            db,
		eventCh:       make(chan metric.TrafficDeltaEvent, 10000),
		batchSize:     100,
		flushInterval: 5 * time.Second,
	}
}

// Start starts the aggregator background worker.
func (a *Aggregator) Start(ctx context.Context) {
	go a.consumeLoop(ctx)
}

// Submit submits a traffic delta event (non-blocking).
func (a *Aggregator) Submit(event metric.TrafficDeltaEvent) {
	select {
	case a.eventCh <- event:
	default:
		// Channel full, drop event
	}
}

func (a *Aggregator) consumeLoop(ctx context.Context) {
	batch := make([]metric.TrafficDeltaEvent, 0, a.batchSize)
	ticker := time.NewTicker(a.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.flush(batch)
			return
		case event := <-a.eventCh:
			batch = append(batch, event)
			if len(batch) >= a.batchSize {
				a.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				a.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (a *Aggregator) flush(events []metric.TrafficDeltaEvent) {
	if len(events) == 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	err := a.db.Transaction(func(tx *gorm.DB) error {
		// 1. Write to ledger (idempotent, skip on duplicate)
		for _, e := range events {
			ledger := entity.TrafficUsageLedger{
				SessionID:      e.SessionID,
				Seq:            e.Sequence,
				UserID:         e.UserID,
				SubscriptionID: e.SubscriptionID,
				ServiceID:      e.ServiceID,
				CoreType:       string(e.CoreType),
				Protocol:       string(e.Protocol),
				UplinkBytes:    e.UplinkBytes,
				DownlinkBytes:  e.DownlinkBytes,
				BilledBytes:    e.UplinkBytes + e.DownlinkBytes,
				OccurredAt:     e.OccurredAt,
			}
			// Use FirstOrCreate for idempotency
			tx.Where("session_id = ? AND seq = ?", e.SessionID, e.Sequence).
				FirstOrCreate(&ledger)
		}

		// 2. Update summary tables
		subDeltas := aggregateBySubscription(events)
		for subID, delta := range subDeltas {
			now := time.Now()
			periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			periodEnd := periodStart.AddDate(0, 1, 0)

			tx.Where("subject_type = ? AND subject_id = ? AND period_start = ?",
				"subscription", subID, periodStart).
				Assign(map[string]interface{}{
					"uplink_bytes":   gorm.Expr("uplink_bytes + ?", delta.Uplink),
					"downlink_bytes": gorm.Expr("downlink_bytes + ?", delta.Downlink),
					"billed_bytes":   gorm.Expr("billed_bytes + ?", delta.Total),
					"updated_at":     now,
				}).
				FirstOrCreate(&entity.TrafficUsageSummary{
					SubjectType:   "subscription",
					SubjectID:     subID,
					PeriodStart:   periodStart,
					PeriodEnd:     periodEnd,
					UplinkBytes:   delta.Uplink,
					DownlinkBytes: delta.Downlink,
					BilledBytes:   delta.Total,
				})
		}

		// 3. Update subscription used_bytes_cache
		for subID, delta := range subDeltas {
			tx.Model(&entity.Subscription{}).
				Where("id = ?", subID).
				Update("used_bytes_cache", gorm.Expr("used_bytes_cache + ?", delta.Total))
		}

		return nil
	})

	if err != nil {
		log.Printf("traffic aggregator flush error: %v", err)
	}
}

type subscriptionDelta struct {
	Uplink   int64
	Downlink int64
	Total    int64
}

func aggregateBySubscription(events []metric.TrafficDeltaEvent) map[int64]*subscriptionDelta {
	deltas := make(map[int64]*subscriptionDelta)
	for _, e := range events {
		if e.SubscriptionID <= 0 {
			continue
		}
		if deltas[e.SubscriptionID] == nil {
			deltas[e.SubscriptionID] = &subscriptionDelta{}
		}
		deltas[e.SubscriptionID].Uplink += e.UplinkBytes
		deltas[e.SubscriptionID].Downlink += e.DownlinkBytes
		deltas[e.SubscriptionID].Total += e.UplinkBytes + e.DownlinkBytes
	}
	return deltas
}
