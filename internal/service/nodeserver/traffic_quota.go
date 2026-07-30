package nodeserver

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/store/repository"
)

// NodeTrafficQuotaService manages monthly traffic quotas for node servers.
//
// Core logic:
//  1. Each heartbeat carries egress interface cumulative counters (rx/tx bytes).
//  2. We compute the delta since the last recorded counter → monthly accumulated traffic.
//  3. If accumulated traffic exceeds the quota, suspend the node (stop all cores).
//  4. On the billing reset day, reset the counter and re-enable the node.
type NodeTrafficQuotaService struct {
	db         *gorm.DB
	serverRepo *repository.NodeServerRepository
	nodeMgr    *agent.NodeServerManager

	mu             sync.Mutex
	lastEgressStat map[int64]*egressStat // serverID → last egress counters
}

type egressStat struct {
	rxBytes   int64
	txBytes   int64
	timestamp time.Time
	bootID    string // boot identifier to detect counter resets
}

// NewNodeTrafficQuotaService creates a new traffic quota service.
func NewNodeTrafficQuotaService(
	db *gorm.DB,
	serverRepo *repository.NodeServerRepository,
	nodeMgr *agent.NodeServerManager,
) *NodeTrafficQuotaService {
	return &NodeTrafficQuotaService{
		db:             db,
		serverRepo:     serverRepo,
		nodeMgr:        nodeMgr,
		lastEgressStat: make(map[int64]*egressStat),
	}
}

// Start begins the periodic quota check loop.
func (s *NodeTrafficQuotaService) Start(ctx context.Context) {
	// Initial check after 30 seconds (let heartbeats come in first)
	time.Sleep(30 * time.Second)
	s.checkAllQuotas(ctx)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAllQuotas(ctx)
		}
	}
}

// StartLocalCollector accounts NIC traffic for the local (panel) node directly
// from the host's /proc/net/dev. The LocalAgent does not send heartbeats with
// egress counters, so without this the local node's traffic_used_bytes stays at
// 0 even though /api/v1/system/stats already reports the host NIC counters.
// It feeds the same ProcessHeartbeat accumulation used by remote Agents, so the
// local node shows up in the traffic dashboard alongside remote nodes.
func (s *NodeTrafficQuotaService) StartLocalCollector(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	// Prime the baseline so the first cycle does not register a huge delta.
	s.collectLocal()
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.collectLocal()
			}
		}
	}()
}

// collectLocal builds an AgentReport from the host NIC counters and feeds it
// through ProcessHeartbeat for every local server.
func (s *NodeTrafficQuotaService) collectLocal() {
	ctx := context.Background()
	locals, err := s.serverRepo.ListLocal(ctx)
	if err != nil || len(locals) == 0 {
		return
	}
	rx, tx := readHostNetCounters()
	if rx == 0 && tx == 0 {
		return
	}
	report := &agentpb.AgentReport{
		EgressInterface: "local",
		EgressRxBytes:   rx,
		EgressTxBytes:   tx,
		Timestamp:       time.Now().Unix(),
	}
	for i := range locals {
		s.ProcessHeartbeat(locals[i].ID, report)
	}
}

// readHostNetCounters sums cumulative rx/tx bytes across all non-loopback
// interfaces from /proc/net/dev, matching the accounting in system stats
// readNetwork. Returns total recv and sent bytes since boot.
func readHostNetCounters() (rx, tx int64) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		iface := strings.TrimSpace(line[:idx])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 9 {
			continue
		}
		if v, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			rx += v
		}
		if v, err := strconv.ParseInt(fields[8], 10, 64); err == nil {
			tx += v
		}
	}
	return rx, tx
}

// ProcessHeartbeat processes egress interface traffic data from a heartbeat report.
// This should be called by NodeServerManager when a HEARTBEAT report is received.
func (s *NodeTrafficQuotaService) ProcessHeartbeat(serverID int64, report *agentpb.AgentReport) {
	if report.EgressInterface == "" {
		return // No egress interface configured for this node
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rxBytes := report.EgressRxBytes
	txBytes := report.EgressTxBytes

	last, exists := s.lastEgressStat[serverID]

	// Detect counter reset (e.g. node reboot) — if counters went down, reset baseline
	if exists && (rxBytes < last.rxBytes || txBytes < last.txBytes) {
		// Counter reset detected — just update baseline, don't compute delta
		s.lastEgressStat[serverID] = &egressStat{
			rxBytes:   rxBytes,
			txBytes:   txBytes,
			timestamp: time.Now(),
		}
		return
	}

	if !exists {
		// First heartbeat with egress data — just store baseline
		s.lastEgressStat[serverID] = &egressStat{
			rxBytes:   rxBytes,
			txBytes:   txBytes,
			timestamp: time.Now(),
		}
		return
	}

	// Compute delta
	dRx := rxBytes - last.rxBytes
	dTx := txBytes - last.txBytes
	total := dRx + dTx

	// Update baseline
	s.lastEgressStat[serverID] = &egressStat{
		rxBytes:   rxBytes,
		txBytes:   txBytes,
		timestamp: time.Now(),
	}

	if total <= 0 {
		return
	}

	// Add delta to the current period's usage
	s.addTrafficDelta(serverID, report.EgressInterface, dRx, dTx)
}

// addTrafficDelta adds a traffic delta to the current billing period.
func (s *NodeTrafficQuotaService) addTrafficDelta(serverID int64, iface string, dRx, dTx int64) {
	now := time.Now()

	// Get the server to determine the billing period
	server, err := s.serverRepo.GetByID(context.Background(), serverID)
	if err != nil {
		return
	}

	// Traffic accounting is decoupled from quota enforcement: any node with a
	// configured egress interface is accounted (NIC rx/tx deltas), while only
	// nodes that also carry a quota can be suspended for exceeding it.
	//
	// if !server.HasTrafficQuota() {
	// 	return
	// }

	periodStart := s.computePeriodStart(now, server.TrafficQuotaResetDay)
	periodEnd := s.computePeriodEnd(now, server.TrafficQuotaResetDay)

	// Update or create the usage record for this period
	usage := &entity.NodeTrafficUsage{}
	result := s.db.Where("server_id = ? AND period_start = ?", serverID, periodStart).First(usage)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new period record
		usage = &entity.NodeTrafficUsage{
			ServerID:   serverID,
			Interface:  iface,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
			RxBytes:    dRx,
			TxBytes:    dTx,
			TotalBytes: dRx + dTx,
			QuotaBytes: server.TrafficQuotaBytes,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := s.db.Create(usage).Error; err != nil {
			log.Printf("[TrafficQuota] failed to create usage record for server %d: %v", serverID, err)
		}
	} else if result.Error == nil {
		// Update existing record
		s.db.Model(usage).Updates(map[string]interface{}{
			"rx_bytes":    gorm.Expr("rx_bytes + ?", dRx),
			"tx_bytes":    gorm.Expr("tx_bytes + ?", dTx),
			"total_bytes": gorm.Expr("total_bytes + ?", dRx+dTx),
			"updated_at":  now,
		})
	}

	// Update the cached used_bytes on the server entity
	s.db.Model(&entity.NodeServer{}).Where("id = ?", serverID).Updates(map[string]interface{}{
		"traffic_used_bytes":   gorm.Expr("traffic_used_bytes + ?", dRx+dTx),
		"traffic_period_start": periodStart,
	})
}

// checkAllQuotas checks all servers with traffic quotas and takes action.
func (s *NodeTrafficQuotaService) checkAllQuotas(ctx context.Context) {
	servers, err := s.serverRepo.ListAll(ctx)
	if err != nil {
		log.Printf("[TrafficQuota] failed to list servers: %v", err)
		return
	}

	now := time.Now()

	for _, server := range servers {
		if !server.HasTrafficQuota() {
			continue
		}

		// ── Check if we need to reset the billing period ──
		periodStart := s.computePeriodStart(now, server.TrafficQuotaResetDay)
		needsReset := server.TrafficPeriodStart == nil || server.TrafficPeriodStart.Before(periodStart)

		if needsReset {
			s.resetPeriod(ctx, &server, periodStart)
			continue // After reset, skip quota check for this cycle
		}

		// ── Check quota ──
		if server.TrafficUsedBytes >= server.TrafficQuotaBytes && !server.TrafficSuspended {
			// Quota exceeded — suspend the node
			s.suspendNode(ctx, &server)
		} else if server.TrafficUsedBytes < server.TrafficQuotaBytes && server.TrafficSuspended {
			// Under quota but still suspended — this can happen after manual reset
			// Do NOT auto-resume; admin must explicitly resume
			// Exception: if a new billing period started, the reset handler above will re-enable
		}
	}
}

// resetPeriod resets the billing period for a server.
// This happens on the billing reset day.
func (s *NodeTrafficQuotaService) resetPeriod(ctx context.Context, server *entity.NodeServer, periodStart time.Time) {
	log.Printf("[TrafficQuota] resetting billing period for server %d (%s)", server.ID, server.Name)

	now := time.Now()
	periodEnd := s.computePeriodEnd(now, server.TrafficQuotaResetDay)

	// Get the old period's total for the reset log
	var oldUsage entity.NodeTrafficUsage
	s.db.Where("server_id = ? AND period_start = ?", server.ID, server.TrafficPeriodStart).First(&oldUsage)

	// Create reset log
	if server.TrafficPeriodStart != nil {
		s.db.Create(&entity.TrafficQuotaResetLog{
			SubjectType:           "node_server",
			SubjectID:             server.ID,
			ResetCycle:            "monthly",
			PeriodStart:           *server.TrafficPeriodStart,
			PeriodEnd:             periodEnd,
			BilledBytesBeforeReset: server.TrafficUsedBytes,
			TriggerSource:         "auto_periodic",
			CreatedAt:             now,
		})
	}

	// Reset the server's traffic counters
	s.db.Model(&entity.NodeServer{}).Where("id = ?", server.ID).Updates(map[string]interface{}{
		"traffic_used_bytes":    0,
		"traffic_period_start":  periodStart,
		"traffic_suspended":     false,
		"traffic_suspended_at":  nil,
	})

	// If the node was suspended, re-enable it and start cores
	if server.TrafficSuspended {
		log.Printf("[TrafficQuota] re-enabling server %d after billing period reset", server.ID)
		s.resumeNode(ctx, server)
	}

	// Reset the egress counter baseline so we don't double-count
	s.mu.Lock()
	delete(s.lastEgressStat, server.ID)
	s.mu.Unlock()
}

// suspendNode stops all cores on a node and marks it as traffic-suspended.
func (s *NodeTrafficQuotaService) suspendNode(ctx context.Context, server *entity.NodeServer) {
	log.Printf("[TrafficQuota] SUSPENDING server %d (%s) — traffic quota exceeded: used=%d quota=%d",
		server.ID, server.Name, server.TrafficUsedBytes, server.TrafficQuotaBytes)

	now := time.Now()

	// Mark as suspended in DB
	s.db.Model(&entity.NodeServer{}).Where("id = ?", server.ID).Updates(map[string]interface{}{
		"traffic_suspended":    true,
		"traffic_suspended_at": now,
	})

	// Stop all cores on the server
	if s.nodeMgr != nil {
		// Get the server's supported cores
		supportedCores := parseSupportedCores(server.SupportedCores)
		for _, coreType := range supportedCores {
			if err := s.nodeMgr.ControlCore(ctx, server.ID, coreType, "stop"); err != nil {
				log.Printf("[TrafficQuota] failed to stop %s on server %d: %v", coreType, server.ID, err)
			}
		}
	}

	// Also mark the usage record as suspended
	s.db.Model(&entity.NodeTrafficUsage{}).
		Where("server_id = ? AND suspended = false", server.ID).
		Updates(map[string]interface{}{
			"suspended":    true,
			"suspended_at": now,
		})
}

// resumeNode re-enables a suspended node by restarting its cores.
func (s *NodeTrafficQuotaService) resumeNode(ctx context.Context, server *entity.NodeServer) {
	log.Printf("[TrafficQuota] RESUMING server %d (%s)", server.ID, server.Name)

	// Sync data and start cores
	if s.nodeMgr != nil {
		// Trigger a full data sync + start
		_, err := s.nodeMgr.SyncDataToServer(ctx, server.ID, true)
		if err != nil {
			log.Printf("[TrafficQuota] failed to sync+start server %d: %v", server.ID, err)
		}
	}
}

// ResumeServer manually resumes a traffic-suspended server.
// This is called from the API handler when an admin clicks "resume".
func (s *NodeTrafficQuotaService) ResumeServer(ctx context.Context, serverID int64) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	if !server.TrafficSuspended {
		return fmt.Errorf("server is not traffic-suspended")
	}

	// Re-enable the server
	s.db.Model(&entity.NodeServer{}).Where("id = ?", serverID).Updates(map[string]interface{}{
		"traffic_suspended":    false,
		"traffic_suspended_at": nil,
	})

	// Resume the node (start cores)
	s.resumeNode(ctx, server)

	return nil
}

// GetTrafficUsage returns the current period's traffic usage for a server.
func (s *NodeTrafficQuotaService) GetTrafficUsage(ctx context.Context, serverID int64) (*entity.NodeTrafficUsage, error) {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	now := time.Now()
	periodStart := s.computePeriodStart(now, server.TrafficQuotaResetDay)

	var usage entity.NodeTrafficUsage
	result := s.db.Where("server_id = ? AND period_start = ?", serverID, periodStart).First(&usage)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No usage record yet
		}
		return nil, result.Error
	}
	return &usage, nil
}

// GetTrafficHistory returns traffic usage history for a server (last N months).
func (s *NodeTrafficQuotaService) GetTrafficHistory(ctx context.Context, serverID int64, months int) ([]entity.NodeTrafficUsage, error) {
	if months <= 0 {
		months = 6
	}
	cutoff := time.Now().AddDate(0, -months, 0)
	var usage []entity.NodeTrafficUsage
	result := s.db.Where("server_id = ? AND period_start >= ?", serverID, cutoff).
		Order("period_start DESC").
		Find(&usage)
	if result.Error != nil {
		return nil, result.Error
	}
	return usage, nil
}

// ──────────────────────────────────────────────────────
// Period calculation helpers
// ──────────────────────────────────────────────────────

// computePeriodStart returns the start of the current billing period.
// The period starts on the configured reset day of each month.
func (s *NodeTrafficQuotaService) computePeriodStart(now time.Time, resetDay int) time.Time {
	if resetDay < 1 {
		resetDay = 1
	}
	if resetDay > 28 {
		resetDay = 28
	}

	// If today >= resetDay, period starts on resetDay of this month
	// If today < resetDay, period starts on resetDay of last month
	day := now.Day()
	year := now.Year()
	month := now.Month()

	if day >= resetDay {
		// Current period starts on resetDay of this month
		return time.Date(year, month, resetDay, 0, 0, 0, 0, now.Location())
	}

	// Current period starts on resetDay of previous month
	if month == 1 {
		return time.Date(year-1, 12, resetDay, 0, 0, 0, 0, now.Location())
	}
	return time.Date(year, month-1, resetDay, 0, 0, 0, 0, now.Location())
}

// computePeriodEnd returns the end of the current billing period.
func (s *NodeTrafficQuotaService) computePeriodEnd(now time.Time, resetDay int) time.Time {
	start := s.computePeriodStart(now, resetDay)
	return start.AddDate(0, 1, 0)
}

// parseSupportedCores parses the supported_cores JSON string into a slice.
func parseSupportedCores(jsonStr string) []string {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	// Simple parsing: remove brackets and quotes
	s := jsonStr
	if len(s) > 2 && s[0] == '[' && s[len(s)-1] == ']' {
		s = s[1 : len(s)-1]
	}
	var cores []string
	for _, part := range splitByComma(s) {
		core := trimQuotes(part)
		if core != "" {
			cores = append(cores, core)
		}
	}
	return cores
}

func splitByComma(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func trimQuotes(s string) string {
	s = trimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
