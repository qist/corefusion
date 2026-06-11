package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// IPRecord represents a subscription IP mapping.
type IPRecord struct {
	IP             string
	SubscriptionID int64
	UserID         int64
	FirstSeen      time.Time
	LastSeen       time.Time
}

// SubscriptionIPTracker tracks subscription IPs and manages nftables rules.
type SubscriptionIPTracker struct {
	tableName string
	ipMap     sync.Map // key: "sub:<id>" -> sync.Map[string]*IPRecord
	eventCh   chan<- TrafficDeltaEvent
}

// NewSubscriptionIPTracker creates a new IP tracker.
func NewSubscriptionIPTracker(tableName string) *SubscriptionIPTracker {
	return &SubscriptionIPTracker{
		tableName: tableName,
	}
}

// SetEventChannel sets the channel for emitting traffic events.
func (t *SubscriptionIPTracker) SetEventChannel(ch chan<- TrafficDeltaEvent) {
	t.eventCh = ch
}

// RecordAccess records a subscription access from an IP.
func (t *SubscriptionIPTracker) RecordAccess(subID, userID int64, ip string) {
	key := fmt.Sprintf("sub:%d", subID)

	v, _ := t.ipMap.LoadOrStore(key, &sync.Map{})
	ipMap := v.(*sync.Map)

	now := time.Now()
	existing, loaded := ipMap.LoadOrStore(ip, &IPRecord{
		IP:             ip,
		SubscriptionID: subID,
		UserID:         userID,
		FirstSeen:      now,
		LastSeen:       now,
	})

	if loaded {
		existing.(*IPRecord).LastSeen = now
	}

	// Add nftables rule
	t.addNftRule(subID, ip)
}

// GetIPs returns all IPs for a subscription.
func (t *SubscriptionIPTracker) GetIPs(subID int64) []string {
	key := fmt.Sprintf("sub:%d", subID)
	v, ok := t.ipMap.Load(key)
	if !ok {
		return nil
	}

	ipMap := v.(*sync.Map)
	var ips []string
	ipMap.Range(func(ip, _ interface{}) bool {
		ips = append(ips, ip.(string))
		return true
	})
	return ips
}

// ReadSubscriptionTraffic reads total traffic for a subscription from nftables.
func (t *SubscriptionIPTracker) ReadSubscriptionTraffic(subID int64) int64 {
	key := fmt.Sprintf("sub:%d", subID)
	v, ok := t.ipMap.Load(key)
	if !ok {
		return 0
	}

	ipMap := v.(*sync.Map)
	var totalBytes int64

	ipMap.Range(func(ip, _ interface{}) bool {
		bytes := t.readIPCounter(subID, ip.(string))
		totalBytes += bytes
		return true
	})

	return totalBytes
}

// ActiveDeviceCount returns the number of active devices for a subscription.
func (t *SubscriptionIPTracker) ActiveDeviceCount(subID int64) int {
	key := fmt.Sprintf("sub:%d", subID)
	v, ok := t.ipMap.Load(key)
	if !ok {
		return 0
	}

	ipMap := v.(*sync.Map)
	count := 0
	now := time.Now()

	ipMap.Range(func(_, record interface{}) bool {
		r := record.(*IPRecord)
		if now.Sub(r.LastSeen) < 24*time.Hour {
			count++
		}
		return true
	})

	return count
}

// BlockSubscription blocks all traffic for a subscription.
func (t *SubscriptionIPTracker) BlockSubscription(subID int64) {
	chainName := fmt.Sprintf("sub_%d", subID)

	// Flush the chain (remove all rules)
	exec.Command("nft", "flush", "chain", "inet", t.tableName, chainName).Run()

	// Add default drop rule
	exec.Command("nft", "add", "rule", "inet", t.tableName, chainName, "drop").Run()

	log.Printf("blocked subscription %d via nftables", subID)
}

// UnblockSubscription unblocks traffic for a subscription.
func (t *SubscriptionIPTracker) UnblockSubscription(subID int64) {
	chainName := fmt.Sprintf("sub_%d", subID)

	// Flush all rules
	exec.Command("nft", "flush", "chain", "inet", t.tableName, chainName).Run()

	// Restore IP counter rules from cache
	key := fmt.Sprintf("sub:%d", subID)
	if v, ok := t.ipMap.Load(key); ok {
		ipMap := v.(*sync.Map)
		ipMap.Range(func(ip, _ interface{}) bool {
			t.addNftRule(subID, ip.(string))
			return true
		})
	}

	log.Printf("unblocked subscription %d via nftables", subID)
}

// PollAll polls all subscriptions' traffic (periodic).
func (t *SubscriptionIPTracker) PollAll(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.pollAllOnce()
		}
	}
}

func (t *SubscriptionIPTracker) pollAllOnce() {
	t.ipMap.Range(func(key, value interface{}) bool {
		subID := parseSubID(key.(string))
		if subID <= 0 {
			return true
		}

		bytes := t.ReadSubscriptionTraffic(subID)
		if bytes > 0 && t.eventCh != nil {
			t.eventCh <- TrafficDeltaEvent{
				SessionID:  fmt.Sprintf("nft-%d-%d", subID, time.Now().UnixNano()),
				SubID:      subID,
				TotalBytes: bytes,
				Source:     "nftables",
				Timestamp:  time.Now(),
			}
		}

		return true
	})
}

func (t *SubscriptionIPTracker) addNftRule(subID int64, ip string) {
	chainName := fmt.Sprintf("sub_%d", subID)

	// Ensure chain exists
	exec.Command("nft", "add", "chain", "inet", t.tableName, chainName).Run()

	// Add counter rule
	exec.Command("nft", "add", "rule", "inet", t.tableName, chainName,
		"ip", "saddr", ip, "counter", "accept").Run()
}

func (t *SubscriptionIPTracker) readIPCounter(subID int64, ip string) int64 {
	chainName := fmt.Sprintf("sub_%d", subID)

	out, err := exec.Command("nft", "-j", "list", "chain", "inet", t.tableName, chainName).Output()
	if err != nil {
		return 0
	}

	var result struct {
		Nftables []struct {
			Rule struct {
				Expr []struct {
					Counter *struct {
						Bytes int64 `json:"bytes"`
					} `json:"counter,omitempty"`
				} `json:"expr"`
			} `json:"rule,omitempty"`
		} `json:"nftables"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return 0
	}

	// Sum all counters in the chain (simplified - in production would match by IP)
	var total int64
	for _, item := range result.Nftables {
		for _, expr := range item.Rule.Expr {
			if expr.Counter != nil {
				total += expr.Counter.Bytes
			}
		}
	}

	return total
}

func parseSubID(key string) int64 {
	// key format: "sub:<id>"
	parts := strings.Split(key, ":")
	if len(parts) != 2 {
		return 0
	}
	var id int64
	fmt.Sscanf(parts[1], "%d", &id)
	return id
}

// TrafficDeltaEvent is a traffic event from nftables counting.
type TrafficDeltaEvent struct {
	SessionID  string
	SubID      int64
	TotalBytes int64
	Source     string
	Timestamp  time.Time
}
