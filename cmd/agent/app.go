package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agent "corefusion/internal/runtime/agent"
	"corefusion/internal/runtime/firewall"
	"corefusion/internal/runtime/process"
	ratelimit "corefusion/internal/runtime/agent/ratelimit"
	binariesSvc "corefusion/internal/service/binaries"
	logManageSvc "corefusion/internal/service/logmanage"
	proxySvc "corefusion/internal/service/proxy"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// AgentApp holds all runtime state for the Agent daemon.
// It reuses the Panel's process.Manager, proxy.Manager, and LocalAgent
// to avoid re-implementing process management.
type AgentApp struct {
	cfg      *AgentConfig
	dataDir  string
	certPath string
	keyPath  string
	caPath   string

	// Reused infrastructure (same as Panel)
	procMgr     *process.Manager
	proxyMgr    *proxySvc.Manager
	localAgent  *agent.LocalAgent
	svcRepo     *repository.ServiceRepository
	database    *gorm.DB
	settingRepo *repository.SystemSettingRepository

	// Config repos for SyncData (local config generation from synced data)
	coreConfigRepo     *repository.CoreConfigRepository
	inboundConfigRepo  *repository.InboundConfigRepository
	outboundConfigRepo *repository.OutboundConfigRepository

	// Caddy site repo for log discovery (shared with logMgr)
	caddySiteRepo *repository.CaddySiteRepository

	// Log management (same as Panel — provides LogWriter for stdout file persistence)
	logMgr *logManageSvc.Service

	// Traffic collector for periodic xray Stats API polling
	trafficCollector *trafficCollector

	// Rate limit manager (nft + tc) — lazily initialized when SyncData provides config
	rateLimitMgr *ratelimit.Manager

	// Online stats poller — polls xray GetUsersStats API for online user IPs
	onlineStatsPoller *ratelimit.OnlineStatsPoller

	// Online users reporter — collects real-time online subscribers for the Panel
	onlineUsersReporter *onlineUsersReporter

	// gRPC connection to Panel
	grpcConn *grpc.ClientConn

	// streamSendMu protects stream.Send calls from concurrent goroutines
	// (heartbeat ticker, status ticker, traffic ticker, recv loop, TaskQueue workers)
	streamSendMu sync.Mutex
}

// NewAgentApp initialises the Agent application:
//  1. Local SQLite database + migrations
//  2. Repositories (service, guardian, crash-log, settings, proxy-site, config)
//  3. Binary resolver + process.Manager (with config generator)
//  4. Binary installer (for remote core binary installation)
//  5. proxy.Manager
//  6. LocalAgent (wraps process.Manager + proxy.Manager + binary installer)
func NewAgentApp(cfg *AgentConfig) (*AgentApp, error) {
	dataDir := cfg.Agent.DataDir

	// Phase 1: Initialize local SQLite database (same as Panel)
	dbPath := filepath.Join(dataDir, "sqlite", "agent.db")
	dbCfg := db.DefaultConfig()
	dbCfg.DSN = dbPath
	if err := db.Init(dbCfg); err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	// Run migrations for process_guardians, inbound_services, etc.
	if err := migrateAgentTables(db.DB); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Phase 2: Create repositories (same as Panel)
	svcRepo := repository.NewServiceRepository(db.DB)
	guardianRepo := repository.NewProcessGuardianRepository(db.DB)
	crashLogRepo := repository.NewCrashLogRepository(db.DB)
	settingRepo := repository.NewSystemSettingRepository(db.DB)
	proxySiteRepo := repository.NewProxySiteRepository(db.DB)

	// Create xray kernel config repositories for local config generation
	coreConfigRepo := repository.NewCoreConfigRepository(db.DB)
	inboundConfigRepo := repository.NewInboundConfigRepository(db.DB)
	outboundConfigRepo := repository.NewOutboundConfigRepository(db.DB)

	// Phase 3: Create binary resolver + process.Manager (same as Panel)
	binaryResolver := binariesSvc.NewResolver(dataDir)
	procMgr := process.NewManager(dataDir, svcRepo, guardianRepo, crashLogRepo, binaryResolver)

	// Set config generator: generates config from local database (populated by SyncData).
	// If the local database has no data (e.g., first boot before panel sync),
	// falls back to generating a minimal default config.
	// After a successful SyncData, the local database contains the full config
	// from the panel, and this generator produces the complete runtime config.
	agentServerID := cfg.Panel.ServerID // Agent's own server ID in the panel
	procMgr.SetConfigGenerator(func(ctx context.Context, coreType string, configPath string) error {
		// Try to generate config from local database first
		data, err := generateConfigFromLocalDB(ctx, agentServerID, coreType)
		if err == nil && len(data) > 0 {
			log.Printf("[Agent] generated %s config from local database (%d bytes)", coreType, len(data))
			return os.WriteFile(configPath, data, 0640)
		}
		// Fallback: generate minimal default config
		log.Printf("[Agent] no local DB data for %s, using default config: %v", coreType, err)
		config := generateDefaultConfig(coreType, dataDir)
		if config == nil {
			return fmt.Errorf("no default config for core type: %s", coreType)
		}
		data, _ = json.MarshalIndent(config, "", "  ")
		return os.WriteFile(configPath, data, 0640)
	})

	// Phase 3b: Create binary installer (for remote core binary installation)
	binaryInstaller := binariesSvc.NewInstaller(dataDir, binaryResolver, &agentSettingStore{repo: settingRepo})
	binaryInstaller.SetProcessController(procMgr) // Enable stop/start during binary updates

	// Phase 4: Create proxy.Manager (same as Panel)
	proxyMgr := proxySvc.NewManager(dataDir, proxySiteRepo, &agentSettingStore{repo: settingRepo})

	// Phase 5: Create LocalAgent — wraps process.Manager + proxy.Manager
	// This is the SAME LocalAgent used by the Panel for is_local=1 servers.
	// It implements NodeOperator by delegating to process.Manager / proxy.Manager.
	localAgent := agent.NewLocalAgent(dataDir, procMgr, svcRepo)
	localAgent = localAgent.WithDB(db.DB)
	localAgent = localAgent.WithServerID(agentServerID)
	localAgent.RestoreFromSnapshot(context.Background())
	localAgent = localAgent.WithProxyManager(agent.NewProxyManagerAdapter(proxyMgr))
	localAgent = localAgent.WithBinaryInstaller(agent.NewBinaryInstallerAdapter(binaryInstaller))

	// Phase 5b: Create rate limit manager (lazy init — iface comes from SyncData/settings).
	// If the Agent has a previously synced rate_limit_configs row, init immediately.
	rateLimitMgr := ratelimit.NewManager("") // iface="" means auto-detect
	localAgent = localAgent.WithRateLimitManager(rateLimitMgr)

	// Phase 6: Create log management service (same as Panel)
	// This provides LogWriter which subscribes to proxy/core LogBroker
	// and persists captured stdout to logs/{type}-stdout.log files.
	logPolicyRepo := repository.NewLogRotationPolicyRepository(db.DB)
	logJobRepo := repository.NewLogRotationJobRepository(db.DB)
	caddySiteRepo := repository.NewCaddySiteRepository(db.DB)

	proxyBrokerAdapter := &logManageSvc.ProxyBrokerAdapter{
		GetBrokerFn: func(proxyType string) *logManageSvc.LogBrokerWrapper {
			broker := proxyMgr.GetLogBroker(proxySvc.ProxyType(proxyType))
			if broker == nil {
				return nil
			}
			return &logManageSvc.LogBrokerWrapper{
				SnapshotFn:  broker.Snapshot,
				SubscribeFn: broker.Subscribe,
			}
		},
	}
	logMgr := logManageSvc.NewService(logPolicyRepo, logJobRepo, &agentSettingStore{repo: settingRepo}, caddySiteRepo, dataDir, proxyBrokerAdapter)

	// Wire log mode provider so process.Manager knows the log output mode for each core
	procMgr.SetLogModeProvider(logMgr)

	// Wire core log broker provider so logmanage can read core process stdout
	logMgr.SetCoreLogBrokerProvider(&agentCoreLogBrokerAdapter{processMgr: procMgr})

	// Wire core config section getter so logmanage can derive output mode from
	// the actual xray/singbox log section (same as Panel)
	logMgr.SetCoreConfigSectionGetter(&agentCoreConfigGetter{})

	// Resolve xray stats API address from local settings (or default)
	statsAddr := resolveStatsAddr(context.Background(), &agentSettingStore{repo: settingRepo})

	app := &AgentApp{
		cfg:                cfg,
		dataDir:            dataDir,
		certPath:           filepath.Join(dataDir, "certs", "agent-cert.pem"),
		keyPath:            filepath.Join(dataDir, "certs", "agent-key.pem"),
		caPath:             filepath.Join(dataDir, "certs", "ca-cert.pem"),
		procMgr:            procMgr,
		proxyMgr:           proxyMgr,
		localAgent:         localAgent,
		svcRepo:            svcRepo,
		database:           db.DB,
		settingRepo:        settingRepo,
		coreConfigRepo:     coreConfigRepo,
		inboundConfigRepo:  inboundConfigRepo,
		outboundConfigRepo: outboundConfigRepo,
		caddySiteRepo:      caddySiteRepo,
		logMgr:             logMgr,
		trafficCollector:   newTrafficCollector(statsAddr),
		rateLimitMgr:       rateLimitMgr,
	}

	return app, nil
}

// agentSettingStore adapts SystemSettingRepository to proxy.SettingStore interface.
type agentSettingStore struct {
	repo *repository.SystemSettingRepository
}

func (s *agentSettingStore) GetByKey(ctx context.Context, key string) (string, error) {
	return s.repo.GetByKey(ctx, key)
}

func (s *agentSettingStore) Set(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
}

// agentCoreLogBrokerAdapter adapts process.Manager to logmanage.CoreLogBrokerProvider.
// Same as the panel's coreLogBrokerAdapter in router.go.
type agentCoreLogBrokerAdapter struct {
	processMgr *process.Manager
}

func (a *agentCoreLogBrokerAdapter) GetCoreLogBroker(coreType string) logManageSvc.LogBrokerReader {
	if a == nil || a.processMgr == nil {
		return nil
	}
	broker := a.processMgr.GetLogBroker(coreType)
	if broker == nil {
		return nil
	}
	return broker // *process.LogBroker implements LogBrokerReader (Snapshot + Subscribe)
}

// agentCoreConfigGetter adapts the local DB to logmanage.CoreConfigSectionGetter.
// It reads core_config sections from the Agent's local SQLite database, allowing
// the log management service to derive the actual log output mode from the
// xray/singbox log section (same as the Panel).
type agentCoreConfigGetter struct{}

func (g *agentCoreConfigGetter) GetSection(ctx context.Context, serverID int64, coreType, section string) (interface{}, error) {
	var cfg entity.CoreConfig
	err := db.DB.WithContext(ctx).
		Where("server_id = ? AND core_type = ? AND section = ?", serverID, coreType, section).
		First(&cfg).Error
	if err != nil {
		return nil, err
	}
	var val interface{}
	if err := json.Unmarshal([]byte(cfg.ConfigJSON), &val); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s/%s: %w", coreType, section, err)
	}
	return val, nil
}

// syncGithubAccelSettings writes GitHub acceleration config to the Agent's local
// settings DB so that installer and proxyMgr can use it for downloads.
func (a *AgentApp) syncGithubAccelSettings(ctx context.Context, accelDomain, proxyURL string) {
	if a.settingRepo == nil {
		return
	}
	if accelDomain == "" && proxyURL == "" {
		return
	}
	// Build config JSON matching the panel's github.acceleration setting format
	cfg := map[string]interface{}{
		"enabled":   true,
		"proxy_url": proxyURL,
		"domains":   []interface{}{},
	}
	if accelDomain != "" {
		cfg["domains"] = []map[string]interface{}{
			{"name": "panel-synced", "url": accelDomain, "enabled": true},
		}
	}
	data, _ := json.Marshal(cfg)
	a.settingRepo.Set(ctx, "github.acceleration", string(data))
	log.Printf("[Agent] synced github.acceleration settings (accel=%s, proxy=%s)", accelDomain, proxyURL)
}

// StartLocalDaemons starts process.Manager's seed + reconcile, which provides
// independent local crash-recovery (Guardian) even if the panel is unreachable.
// After reconcile, it also restores firewall rules for hysteria port hopping.
func (a *AgentApp) StartLocalDaemons(ctx context.Context) {
	a.procMgr.SeedCores(ctx)

	// Start log management service (same as Panel)
	// This restores previously enabled LogWriters that subscribe to proxy/core
	// LogBrokers and persist captured stdout to logs/{type}-stdout.log files.
	if a.logMgr != nil {
		a.logMgr.StartAutoWritersForServer(a.cfg.Panel.ServerID)
		a.logMgr.Start()
		log.Println("[Agent] log management service started (LogWriter + rotation scheduler)")
	}

	// 实时在线订阅人上报（与限速完全解耦，不依赖 nft 与限速配置，必须独立启动）
	a.startOnlineUsersReporter(ctx)

	// Initialize rate limit manager from previously synced config (if any)
	a.initRateLimitFromDB(ctx)

	go func() {
		a.procMgr.Reconcile(ctx)
		// Restore firewall rules for hysteria port hopping after process reconcile.
		// Rules are lost on machine reboot and need to be re-created from local DB config.
		time.Sleep(2 * time.Second) // wait for cores to start
		a.restoreFirewallRules(ctx)
	}()
	log.Println("[Agent] process.Manager started (Guardian auto-restart active)")

	// Start proxy processes (nginx/caddy) if they were previously running.
	// This ensures proxy processes survive agent restarts.
	go func() {
		time.Sleep(3 * time.Second) // wait for cores to start first
		a.startProxyProcesses(ctx)
	}()
}

// restoreFirewallRules scans local inbound configs and re-creates firewall rules
// for hysteria port hopping. Called on Agent startup after process reconcile.
func (a *AgentApp) restoreFirewallRules(ctx context.Context) {
	if a.inboundConfigRepo == nil {
		return
	}
	// Read all enabled hysteria inbounds for this server from local DB
	configs, err := a.inboundConfigRepo.List(ctx, a.cfg.Panel.ServerID, "xray")
	if err != nil {
		log.Printf("[Agent] firewall restore: failed to list inbound configs: %v", err)
		return
	}

	var infos []firewall.InboundConfigInfo
	for _, c := range configs {
		if !c.Enabled || c.Protocol != "hysteria" {
			continue
		}
		infos = append(infos, firewall.InboundConfigInfo{
			Tag:        c.Tag,
			Protocol:   c.Protocol,
			Port:       c.Port,
			ConfigJSON: c.ConfigJSON,
			Enabled:    c.Enabled,
		})
	}

	firewall.RestoreRules(ctx, firewall.LocalApplier{}, func() ([]firewall.InboundConfigInfo, error) {
		return infos, nil
	})
}

// startProxyProcesses starts proxy processes (nginx/caddy) if not manually stopped.
// This is called on agent startup to ensure proxy processes survive agent restarts.
func (a *AgentApp) startProxyProcesses(ctx context.Context) {
	if a.proxyMgr == nil {
		return
	}

	for _, proxyType := range []proxySvc.ProxyType{proxySvc.ProxyNginx, proxySvc.ProxyCaddy} {
		proxyTypeStr := string(proxyType)
		
		// Check if manually stopped (the actual control flag)
		manuallyStopped := a.proxyMgr.IsManuallyStopped(ctx, proxyType)
		
		// Also check if the process is already running
		info := a.proxyMgr.DetectProxy(proxyType)
		isRunning := info.Status == "running"

		if !manuallyStopped && !isRunning {
			log.Printf("[Agent] auto-starting %s (manually_stopped=false, current_status=%s)", proxyTypeStr, info.Status)
			if err := a.proxyMgr.StartProxy(ctx, proxyType); err != nil {
				log.Printf("[Agent] failed to auto-start %s: %v", proxyTypeStr, err)
			} else {
				log.Printf("[Agent] %s auto-started successfully", proxyTypeStr)
			}
		} else if isRunning {
			log.Printf("[Agent] %s is already running, skipping auto-start", proxyTypeStr)
		} else {
			log.Printf("[Agent] %s manually stopped, skipping auto-start", proxyTypeStr)
		}
	}
}

// initRateLimitFromDB reads the previously synced rate_limit_configs from the
// local SQLite database and, if enabled, initializes the RateLimitManager
// with the correct interface and starts its cleanup loop.
func (a *AgentApp) initRateLimitFromDB(ctx context.Context) {
	if a.rateLimitMgr == nil || a.database == nil {
		return
	}

	var config entity.RateLimitConfig
	if err := a.database.WithContext(ctx).
		Where("server_id = ?", a.cfg.Panel.ServerID).
		First(&config).Error; err != nil {
		// No config synced yet — rate limiting stays uninitialized
		return
	}

	if !config.Enabled {
		log.Printf("[Agent] rate limiting disabled for server %d", config.ServerID)
		return
	}

	// Determine interface: explicit config > auto-detect
	iface := config.Iface
	if iface == "" {
		iface = detectDefaultInterface()
	}
	if iface == "" {
		log.Printf("[Agent] rate limit: no network interface found, skipping init")
		return
	}

	// Update the manager's interface
	a.rateLimitMgr = ratelimit.NewManager(iface)
	a.localAgent.WithRateLimitManager(a.rateLimitMgr)

	// Inject persistence callbacks so IP mappings survive Agent restarts
	a.rateLimitMgr.WithPersistFunc(a.persistIPEntry, a.removeIPEntry)

	// Initialize the nft + tc backend
	if err := a.rateLimitMgr.Init(ctx); err != nil {
		log.Printf("[Agent] rate limit init failed on iface=%s: %v", iface, err)
		return
	}

	// Restore previously persisted IP entries from local DB
	a.restoreIPEntries(ctx)

	// Start cleanup loop if TTL configured
	if config.IdleTTLSeconds > 0 && config.CleanupInterval > 0 {
		a.rateLimitMgr.WithCleanupTTL(time.Duration(config.IdleTTLSeconds) * time.Second)
		a.rateLimitMgr.StartCleanupLoop(time.Duration(config.CleanupInterval) * time.Second)
	}

	log.Printf("[Agent] rate limit initialized: iface=%s, default=↓%dMbps/↑%dMbps, cleanup_ttl=%ds",
		iface, config.DefaultMaxDownMbps, config.DefaultMaxUpMbps, config.IdleTTLSeconds)

	// Start online stats poller for P1 trigger source
	a.startOnlineStatsPoller(ctx)
}

// startOnlineStatsPoller starts the online stats poller that queries the xray
// Stats API for online user IPs and keeps their rate limit rules active.
// startOnlineUsersReporter initializes the online-users reporter that feeds
// real-time online subscriber data to the Panel via the heartbeat stream.
// Unlike startOnlineStatsPoller, it does NOT depend on nft availability.
func (a *AgentApp) startOnlineUsersReporter(ctx context.Context) {
	statsAddr := a.getStatsAddr()
	if statsAddr == "" {
		return
	}
	a.onlineUsersReporter = newOnlineUsersReporter(statsAddr)
	if a.database != nil {
		a.onlineUsersReporter.SetEmailMapping(a.buildEmailToSubIDMapping(ctx))
	}
}

func (a *AgentApp) startOnlineStatsPoller(ctx context.Context) {
	if a.rateLimitMgr == nil || !a.rateLimitMgr.IsAvailable().NftAvailable {
		return
	}

	statsAddr := a.getStatsAddr()
	if statsAddr == "" {
		return
	}

	a.onlineStatsPoller = ratelimit.NewOnlineStatsPoller(a.rateLimitMgr, statsAddr)

	// Build email → subscriptionID mapping from local DB
	if a.database != nil {
		mapping := a.buildEmailToSubIDMapping(ctx)
		a.onlineStatsPoller.SetEmailMapping(mapping)
	}

	a.onlineStatsPoller.Start(ctx)
}

// buildEmailToSubIDMapping builds a mapping from xray user email to subscription ID
// by scanning inbound_configs in the local database.
func (a *AgentApp) buildEmailToSubIDMapping(ctx context.Context) map[string]int64 {
	mapping := make(map[string]int64)
	if a.database == nil || a.inboundConfigRepo == nil {
		return mapping
	}

	// Query all inbound configs to extract user emails
	configs, err := a.inboundConfigRepo.ListAll(ctx, "xray")
	if err != nil {
		return mapping
	}

	for _, cfg := range configs {
		if cfg.ConfigJSON == "" {
			continue
		}
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(cfg.ConfigJSON), &config); err != nil {
			continue
		}
		settings, ok := config["settings"].(map[string]interface{})
		if !ok {
			continue
		}
		clients, ok := settings["clients"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range clients {
			client, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			email, _ := client["email"].(string)
			if email == "" {
				continue
			}
			// Email format: sub_<subscriptionID>@domain or just the raw email
			// Try to extract subscription ID from email
			subID := extractSubIDFromEmail(email)
			if subID > 0 {
				mapping[email] = subID
			}
		}
	}

	log.Printf("[Agent] built email→subID mapping: %d entries", len(mapping))
	return mapping
}

// extractSubIDFromEmail tries to extract a subscription ID from an xray user email.
// Email formats in CoreFusion: "sub_<subID>@<tag>" or "<subID>@<domain>".
func extractSubIDFromEmail(email string) int64 {
	// Try "sub_<id>" prefix format
	if strings.HasPrefix(email, "sub_") {
		rest := email[4:]
		if idx := strings.Index(rest, "@"); idx > 0 {
			var id int64
			if _, err := fmt.Sscanf(rest[:idx], "%d", &id); err == nil && id > 0 {
				return id
			}
		}
	}
	// Try "<id>@" format
	if idx := strings.Index(email, "@"); idx > 0 {
		var id int64
		if _, err := fmt.Sscanf(email[:idx], "%d", &id); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

// getStatsAddr returns the xray stats API address from local settings or default.
func (a *AgentApp) getStatsAddr() string {
	if a.settingRepo != nil {
		if addr, err := a.settingRepo.GetByKey(context.Background(), "xray.stats_addr"); err == nil && addr != "" {
			return addr
		}
	}
	return "127.0.0.1:10085" // default xray stats API address
}

// detectDefaultInterface returns the default outbound network interface name
// (e.g. "eth0"). It uses `ip route` to find the interface used for the
// default route.
func detectDefaultInterface() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	// Output format: "default via X.X.X.X dev eth0 ..."
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// persistIPEntry saves an IP → subscription mapping to the local DB for restart recovery.
func (a *AgentApp) persistIPEntry(ip string, subID int64, maxDownMbps, maxUpMbps int32) {
	if a.database == nil {
		return
	}
	ctx := context.Background()
	entry := entity.RateLimitIPEntry{
		ServerID:       a.cfg.Panel.ServerID,
		IP:             ip,
		SubscriptionID: subID,
		MaxDownMbps:    maxDownMbps,
		MaxUpMbps:      maxUpMbps,
		LastSeen:       time.Now(),
		CreatedAt:      time.Now(),
	}
	// Upsert: update if IP already exists
	if err := a.database.WithContext(ctx).
		Where("ip = ? AND server_id = ?", ip, a.cfg.Panel.ServerID).
		Assign(entry).
		FirstOrCreate(&entry).Error; err != nil {
		log.Printf("[Agent] persist rate limit IP entry failed: %v", err)
	}
}

// removeIPEntry deletes a persisted IP entry from the local DB.
func (a *AgentApp) removeIPEntry(ip string) {
	if a.database == nil {
		return
	}
	ctx := context.Background()
	a.database.WithContext(ctx).
		Where("ip = ? AND server_id = ?", ip, a.cfg.Panel.ServerID).
		Delete(&entity.RateLimitIPEntry{})
}

// restoreIPEntries loads previously persisted IP entries from the local DB
// and re-applies their nft + tc rate limit rules.
func (a *AgentApp) restoreIPEntries(ctx context.Context) {
	if a.database == nil || a.rateLimitMgr == nil {
		return
	}

	var entries []entity.RateLimitIPEntry
	if err := a.database.WithContext(ctx).
		Where("server_id = ?", a.cfg.Panel.ServerID).
		Find(&entries).Error; err != nil {
		log.Printf("[Agent] failed to load rate limit IP entries: %v", err)
		return
	}

	if len(entries) == 0 {
		return
	}

	restored := 0
	for _, entry := range entries {
		rate := &ratelimit.RateConfig{
			MaxDownMbps: entry.MaxDownMbps,
			MaxUpMbps:   entry.MaxUpMbps,
		}
		if err := a.rateLimitMgr.RestoreFromDB(ctx, entry.IP, entry.SubscriptionID, rate); err != nil {
			log.Printf("[Agent] restore rate limit for ip=%s failed: %v", entry.IP, err)
			continue
		}
		restored++
	}

	log.Printf("[Agent] restored %d/%d rate limit IP entries from local DB", restored, len(entries))
}
