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
	UpdateTraffic(ctx context.Context, serverID int64, tag string, email string, upDelta int64, downDelta int64) error
}

// NodeTrafficAccumulator is an optional callback to accumulate xray user
// traffic deltas into a node-level monthly traffic record. Implemented by
// nodeserver.NodeTrafficQuotaService.AddUserTrafficDelta.
type NodeTrafficAccumulator interface {
	AddUserTrafficDelta(serverID int64, upDelta, downDelta int64)
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
	settingSvc       SettingStore
	inboundSvc       InboundConfigService
	nodeAccum        NodeTrafficAccumulator
	localSID         int64 // local node server ID (always 1)
	interval         time.Duration
	mu               sync.RWMutex
	running          bool
	stopCh           chan struct{}
	initialized      bool                 // false until first reset query completes
	client           *coredata.XrayClient // xray stats client
	singboxClient    *coredata.XrayClient // sing-box V2Ray API client (nil if not configured)
	initializedSb    bool                 // sing-box init state
	warnedNilSingbox bool                 // sing-box 客户端未配置时只提示一次
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
		localSID:   1, // local node server ID is always 1
	}
}

// getStatsAddrDefault resolves the xray stats address from settings or default.
func getStatsAddrDefault(settingSvc SettingStore) string {
	var getter func(ctx context.Context, key string) (string, error)
	if settingSvc != nil {
		getter = settingSvc.GetByKey
	}
	return coredata.ResolveStatsAddr(context.Background(), coredata.StatsAddrDeps{
		SettingGetter: getter,
		// 无核心配置兜底（trafficsync 不持有 coreConfigSvc），缺失时回退默认地址。
	})
}

// SetNodeAccumulator injects a node traffic accumulator to also update
// node-level monthly traffic records when syncing user traffic.
func (s *Service) SetNodeAccumulator(acc NodeTrafficAccumulator) {
	s.nodeAccum = acc
}

// SetSingboxStatsAddr configures the sing-box V2Ray API address.
// When set, the traffic sync service will also query sing-box's V2Ray API
// (which is protocol-compatible with xray's StatsService) and merge
// the traffic deltas.
//
// 必须用 coredata.NewV2RayClient：sing-box 注册的 gRPC 服务名是
// v2ray.core.app.stats.command.StatsService，与 xray 的
// xray.app.stats.command.StatsService 不同；用 xray 的客户端查 sing-box 会一直
// Unimplemented，流量采集静默失效。
func (s *Service) SetSingboxStatsAddr(addr string) {
	if addr == "" {
		log.Printf("trafficsync: sing-box stats address is empty, sing-box traffic will NOT be collected")
		return
	}
	if s.client != nil && addr == s.client.Addr() {
		log.Printf("trafficsync: WARNING sing-box stats address (%s) equals xray stats address; "+
			"sing-box traffic collection is likely misconfigured", addr)
	}
	s.singboxClient = coredata.NewV2RayClient(addr)
	log.Printf("trafficsync: sing-box stats client configured at %s (service=%s)",
		addr, coredata.V2RayStatsServiceName)
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

	// First call: reset counters to 0 for xray.
	if !s.initialized {
		if _, err := s.client.RawStats(ctx, true); err != nil {
			log.Printf("trafficsync: xray initial reset failed: %v", err)
		} else {
			s.initialized = true
		}
	}
	// First call: reset counters for sing-box (if configured).
	if s.singboxClient != nil && !s.initializedSb {
		if _, err := s.singboxClient.RawStats(ctx, true); err != nil {
			log.Printf("trafficsync: singbox initial reset failed: %v", err)
		} else {
			s.initializedSb = true
		}
	}
	// If neither initialized, wait for next cycle
	if !s.initialized && (s.singboxClient == nil || !s.initializedSb) {
		return
	}
	if !s.warnedNilSingbox && s.singboxClient == nil {
		// 只提示一次：sing-box 客户端未配置时，sing-box 的流量不会被采集，
		// 但 xray 侧仍正常，很容易被误认为「采集正常」。
		s.warnedNilSingbox = true
		log.Printf("trafficsync: sing-box stats client not configured (sing-box traffic collection disabled)")
	}

	// Collect user traffic from xray
	var allUsers []coredata.UserTraffic
	if s.initialized {
		resp, err := s.client.RawStats(ctx, true)
		if err != nil {
			log.Printf("trafficsync: xray query stats failed: %v", err)
		} else {
			sr, err := coredata.ParseStatsResponse(resp)
			if err != nil {
				log.Printf("trafficsync: xray parse stats failed: %v", err)
			} else {
				allUsers = append(allUsers, sr.Users...)
			}
		}
	}

	// Collect user traffic from sing-box (if configured)
	if s.singboxClient != nil && s.initializedSb {
		resp, err := s.singboxClient.RawStats(ctx, true)
		if err != nil {
			log.Printf("trafficsync: singbox query stats failed: %v", err)
		} else {
			sr, err := coredata.ParseStatsResponse(resp)
			if err != nil {
				log.Printf("trafficsync: singbox parse stats failed: %v", err)
			} else {
				// Merge: add sing-box user traffic to xray user traffic
				allUsers = mergeUserTraffic(allUsers, sr.Users)
			}
		}
	}

	// Update database
	if len(allUsers) > 0 {
		log.Printf("trafficsync: updating %d users", len(allUsers))
		var nodeUp, nodeDown int64
		for _, u := range allUsers {
			if u.Uplink <= 0 && u.Downlink <= 0 {
				continue
			}
			log.Printf("trafficsync: %s up=%d down=%d", u.Email, u.Uplink, u.Downlink)
			if err := s.inboundSvc.UpdateTraffic(ctx, s.localSID, "", u.Email, u.Uplink, u.Downlink); err != nil {
				log.Printf("trafficsync: failed to update traffic for %s: %v", u.Email, err)
			}
			nodeUp += u.Uplink
			nodeDown += u.Downlink
		}
		// Also accumulate into node-level monthly traffic record
		if s.nodeAccum != nil && (nodeUp > 0 || nodeDown > 0) {
			s.nodeAccum.AddUserTrafficDelta(s.localSID, nodeUp, nodeDown)
		}
	}
}

// mergeUserTraffic merges two user traffic slices, summing uplink/downlink
// for users that appear in both.
func mergeUserTraffic(a, b []coredata.UserTraffic) []coredata.UserTraffic {
	if len(b) == 0 {
		return a
	}
	merged := make(map[string]*coredata.UserTraffic, len(a))
	for i := range a {
		u := a[i]
		merged[u.Email] = &u
	}
	for _, u := range b {
		if existing, ok := merged[u.Email]; ok {
			existing.Uplink += u.Uplink
			existing.Downlink += u.Downlink
			existing.Total += u.Total
		} else {
			tmp := u
			merged[u.Email] = &tmp
		}
	}
	result := make([]coredata.UserTraffic, 0, len(merged))
	for _, u := range merged {
		result = append(result, *u)
	}
	return result
}
