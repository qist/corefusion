package trafficsync

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	statsService "github.com/xtls/xray-core/app/stats/command"
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
	conn        *grpc.ClientConn
	connMu      sync.Mutex
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
	}
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

	s.connMu.Lock()
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	s.connMu.Unlock()

	log.Println("trafficsync: stopped")
}

// getStatsAddr returns the xray stats API address.
func (s *Service) getStatsAddr(ctx context.Context) string {
	if s.settingSvc != nil {
		val, err := s.settingSvc.GetByKey(ctx, "xray.stats_addr")
		if err == nil && val != "" {
			return val
		}
	}
	return "127.0.0.1:10085"
}

// connect returns a shared gRPC connection.
func (s *Service) connect(ctx context.Context) (*grpc.ClientConn, error) {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if s.conn != nil && s.conn.GetState().String() != "SHUTDOWN" {
		return s.conn, nil
	}

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	addr := s.getStatsAddr(ctx)
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	s.conn = conn
	return conn, nil
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

	conn, err := s.connect(ctx)
	if err != nil {
		log.Printf("trafficsync: connect failed: %v", err)
		return
	}

	client := statsService.NewStatsServiceClient(conn)

	// First call: reset xray counters to 0.
	// Discard returned values (old accumulated traffic).
	if !s.initialized {
		_, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: true})
		if err != nil {
			log.Printf("trafficsync: initial reset failed: %v", err)
			return
		}
		s.initialized = true
		return // counters reset, next call gets real deltas
	}

	// Subsequent calls: reset_=true so xray returns deltas and resets counters.
	resp, err := client.QueryStats(ctx, &statsService.QueryStatsRequest{Reset_: true})
	if err != nil {
		log.Printf("trafficsync: query stats failed: %v", err)
		return
	}

	// Parse user traffic deltas
	type userTraffic struct {
		email string
		up    int64
		down  int64
	}
	userDeltas := make(map[string]*userTraffic)

	for _, stat := range resp.GetStat() {
		name := stat.GetName()
		value := stat.GetValue()

		if !strings.Contains(name, "user>>>") || !strings.Contains(name, ">>>traffic>>>") {
			continue
		}

		parts := strings.Split(name, ">>>")
		if len(parts) != 4 {
			continue
		}

		email := parts[1]
		direction := parts[3]

		// With reset_=true, value IS the delta since last query.
		if value <= 0 {
			continue
		}

		if userDeltas[email] == nil {
			userDeltas[email] = &userTraffic{email: email}
		}

		switch direction {
		case "uplink":
			userDeltas[email].up += value
		case "downlink":
			userDeltas[email].down += value
		}
	}

	// Update database
	if len(userDeltas) > 0 {
		log.Printf("trafficsync: updating %d users", len(userDeltas))
		for _, ut := range userDeltas {
			if ut.up > 0 || ut.down > 0 {
				log.Printf("trafficsync: %s up=%d down=%d", ut.email, ut.up, ut.down)
				if err := s.inboundSvc.UpdateTraffic(ctx, "", ut.email, ut.up, ut.down); err != nil {
					log.Printf("trafficsync: failed to update traffic for %s: %v", ut.email, err)
				}
			}
		}
	}
}
