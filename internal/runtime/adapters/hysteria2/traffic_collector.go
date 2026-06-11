package hysteria2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"corefusion/internal/domain"
	"corefusion/internal/domain/metric"
)

// TrafficCollector collects traffic stats from Hysteria 2 control API.
type TrafficCollector struct {
	serviceID   int64
	controlAddr string
	secret      string
	httpClient  *http.Client
	subscribers []chan<- metric.TrafficDeltaEvent
	mu          sync.RWMutex
}

func NewTrafficCollector(serviceID int64, controlAddr, secret string) *TrafficCollector {
	return &TrafficCollector{
		serviceID:   serviceID,
		controlAddr: controlAddr,
		secret:      secret,
		httpClient:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *TrafficCollector) CoreType() domain.CoreType { return domain.CoreTypeHysteria2 }

func (c *TrafficCollector) Start(ctx context.Context, in metric.CollectorStartInput) error {
	c.controlAddr = in.RuntimeMeta["control_addr"]
	c.secret = in.RuntimeMeta["control_secret"]
	go c.pollLoop(ctx)
	return nil
}

func (c *TrafficCollector) Stop(ctx context.Context) error { return nil }

func (c *TrafficCollector) Snapshot(ctx context.Context) (*metric.CollectorSnapshot, error) {
	return &metric.CollectorSnapshot{ServiceID: c.serviceID, CollectedAt: time.Now()}, nil
}

func (c *TrafficCollector) Subscribe(ctx context.Context, ch chan<- metric.TrafficDeltaEvent) error {
	c.mu.Lock()
	c.subscribers = append(c.subscribers, ch)
	c.mu.Unlock()
	return nil
}

type hy2UserStats struct {
	Auth string `json:"auth"`
	Tx   int64  `json:"tx"`
	Rx   int64  `json:"rx"`
	Conns int   `json:"conns"`
}

func (c *TrafficCollector) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *TrafficCollector) collect(ctx context.Context) {
	// clear=true returns delta since last query
	users, err := c.queryAllUsers(ctx)
	if err != nil {
		return
	}

	for _, user := range users {
		if user.Tx == 0 && user.Rx == 0 {
			continue
		}

		event := metric.TrafficDeltaEvent{
			SessionID:      fmt.Sprintf("hy2-%s-%d", user.Auth, time.Now().UnixNano()),
			ServiceID:      c.serviceID,
			CoreType:       domain.CoreTypeHysteria2,
			Protocol:       domain.ProtocolHysteria2,
			UplinkBytes:    user.Tx,
			DownlinkBytes:  user.Rx,
			Sequence:       time.Now().UnixNano(),
			SourceCollector: "hy2_control_api",
			CollectedAt:    time.Now(),
		}

		c.mu.RLock()
		for _, ch := range c.subscribers {
			select {
			case ch <- event:
			default:
			}
		}
		c.mu.RUnlock()
	}
}

func (c *TrafficCollector) queryAllUsers(ctx context.Context) ([]hy2UserStats, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("http://%s/traffic?clear=true", c.controlAddr), nil)
	if c.secret != "" {
		req.Header.Set("Authorization", c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var users []hy2UserStats
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
}
