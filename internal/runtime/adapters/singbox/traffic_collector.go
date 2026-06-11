package singbox

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

// TrafficCollector collects traffic stats from sing-box HTTP API.
type TrafficCollector struct {
	serviceID   int64
	apiAddr     string
	apiSecret   string
	httpClient  *http.Client
	subscribers []chan<- metric.TrafficDeltaEvent
	lastValues  map[string]int64
	mu          sync.RWMutex
}

func NewTrafficCollector(serviceID int64, apiAddr, apiSecret string) *TrafficCollector {
	return &TrafficCollector{
		serviceID:  serviceID,
		apiAddr:    apiAddr,
		apiSecret:  apiSecret,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		lastValues: make(map[string]int64),
	}
}

func (c *TrafficCollector) CoreType() domain.CoreType { return domain.CoreTypeSingBox }

func (c *TrafficCollector) Start(ctx context.Context, in metric.CollectorStartInput) error {
	c.apiAddr = in.RuntimeMeta["api_addr"]
	c.apiSecret = in.RuntimeMeta["api_secret"]
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

type singBoxTrafficResp struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
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
	// Query inbound traffic
	inbounds := []string{"inbound-main"} // Would be populated from config
	for _, tag := range inbounds {
		traffic, err := c.queryTraffic(ctx, tag)
		if err != nil {
			continue
		}

		c.mu.Lock()
		upKey := fmt.Sprintf("%s:up", tag)
		downKey := fmt.Sprintf("%s:down", tag)

		lastUp := c.lastValues[upKey]
		lastDown := c.lastValues[downKey]

		deltaUp := traffic.Upload - lastUp
		deltaDown := traffic.Download - lastDown

		if deltaUp > 0 || deltaDown > 0 {
			c.lastValues[upKey] = traffic.Upload
			c.lastValues[downKey] = traffic.Download
		}
		c.mu.Unlock()

		if deltaUp > 0 || deltaDown > 0 {
			event := metric.TrafficDeltaEvent{
				SessionID:      fmt.Sprintf("singbox-%s-%d", tag, time.Now().UnixNano()),
				ServiceID:      c.serviceID,
				CoreType:       domain.CoreTypeSingBox,
				UplinkBytes:    deltaUp,
				DownlinkBytes:  deltaDown,
				Sequence:       time.Now().UnixNano(),
				SourceCollector: "singbox_api",
				CollectedAt:    time.Now(),
			}
			c.emit(event)
		}
	}
}

func (c *TrafficCollector) queryTraffic(ctx context.Context, tag string) (*singBoxTrafficResp, error) {
	url := fmt.Sprintf("http://%s/traffic?name=%s", c.apiAddr, tag)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	if c.apiSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiSecret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var traffic singBoxTrafficResp
	if err := json.NewDecoder(resp.Body).Decode(&traffic); err != nil {
		return nil, err
	}
	return &traffic, nil
}

func (c *TrafficCollector) emit(event metric.TrafficDeltaEvent) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, ch := range c.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
