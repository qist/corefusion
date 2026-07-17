package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	agent "corefusion/internal/runtime/agent"
	"corefusion/internal/runtime/firewall"
	"corefusion/internal/runtime/process"
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
	localAgent = localAgent.WithProxyManager(agent.NewProxyManagerAdapter(proxyMgr))
	localAgent = localAgent.WithBinaryInstaller(agent.NewBinaryInstallerAdapter(binaryInstaller))

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
