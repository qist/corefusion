package trafficsync

import (
	"context"
	"log"
	"sync"
	"time"

	"corefusion/internal/coredata"
)

// InboundConfigService provides access to inbound configurations.
type InboundConfigService interface {
	UpdateTraffic(ctx context.Context, tag string, email string, upDelta int64, downDelta int64) error
}

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// Service handles periodic traffic synchronization to database.
//
// Strategy (reset_=true, no lastValues needed):
//   - First query: QueryStats(reset_=true) — resets xray counters to 0.
//     Discard returned values (old accumulated traffic).
//   - Subsequent queries: QueryStats(reset_=true) — each value IS the delta
//     since last query. No baseline, no per-user first-sample skip.
type Service struct {
	settingSvc  SettingStore
	inboundSvc  InboundConfigService
	interval    time.Duration
	mu          sync.RWMutex
	running     bool
	stopCh      chan struct{}
	initialized bool // false until first reset query completes
	client      *coredata.XrayClient
}

// NewService creates a new traffic sync service.
func NewService(settingSvc SettingStore, inboundSvc InboundConfigService, interval time.Duration) *Service {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Service{
		settingSvc: settingSvc,
		inboundSvc: inboundSvc,
		interval:   interval,
		stopCh:     make(chan struct{}),
		client:     coredata.NewXrayClient(getStatsAddrDefault(settingSvc)),
	}
}

// getStatsAddrDefault resolves the xray stats address from settings or default.
func getStatsAddrDefault(settingSvc SettingStore) string {
	if settingSvc != nil {
		if v, err := settingSvc.GetByKey(context.Background(), "xray.stats_addr"); err == nil && v != "" {
			return v
		}
	}
	return coredata.DefaultStatsAddr
}

// Start starts the traffic sync service.
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.loop()
	log.Printf("trafficsync: started with interval %v", s.interval)
}

// Stop stops the traffic sync service.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)

	log.Println("trafficsync: stopped")
}

func (s *Service) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.sync()
		}
	}
}

func (s *Service) sync() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First call: reset xray counters to 0.
	// Discard returned values (old accumulated traffic).
	if !s.initialized {
		if _, err := s.client.RawStats(ctx, true); err != nil {
			log.Printf("trafficsync: initial reset failed: %v", err)
			return
		}
		s.initialized = true
		return // counters reset, next call gets real deltas
	}

	// Subsequent calls: reset_=true so xray returns deltas and resets counters.
	resp, err := s.client.RawStats(ctx, true)
	if err != nil {
		log.Printf("trafficsync: query stats failed: %v", err)
		return
	}

	// 复用公共解析，仅取 user 方向；增量语义（value<=0 跳过）由调用方保留。
	sr, err := coredata.ParseStatsResponse(resp)
	if err != nil {
		log.Printf("trafficsync: parse stats failed: %v", err)
		return
	}

	// Update database
	if len(sr.Users) > 0 {
		log.Printf("trafficsync: updating %d users", len(sr.Users))
		for _, u := range sr.Users {
			if u.Uplink <= 0 && u.Downlink <= 0 {
				continue
			}
			log.Printf("trafficsync: %s up=%d down=%d", u.Email, u.Uplink, u.Downlink)
			if err := s.inboundSvc.UpdateTraffic(ctx, "", u.Email, u.Uplink, u.Downlink); err != nil {
				log.Printf("trafficsync: failed to update traffic for %s: %v", u.Email, err)
			}
		}
	}
}
