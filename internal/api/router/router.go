package router

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"runtime"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	extNodeHandler "corefusion/internal/api/handlers"
	agentHandler "corefusion/internal/api/handlers/agent"
	authHandler "corefusion/internal/api/handlers/auth"
	backupsHandler "corefusion/internal/api/handlers/backups"

	certsHandler "corefusion/internal/api/handlers/certificates"
	cfipHandler "corefusion/internal/api/handlers/cfip"
	chargeHandler "corefusion/internal/api/handlers/charge"
	clientHandler "corefusion/internal/api/handlers/client"
	clientauthHandler "corefusion/internal/api/handlers/clientauth"
	clientextHandler "corefusion/internal/api/handlers/clientext"
	coreconfigHandler "corefusion/internal/api/handlers/coreconfig"
	geoRuleHandler "corefusion/internal/api/handlers/georule"
	inboundconfigHandler "corefusion/internal/api/handlers/inboundconfig"
	logManageHandler "corefusion/internal/api/handlers/logmanage"
	metricsHandler "corefusion/internal/api/handlers/metrics"
	mihomoHandler "corefusion/internal/api/handlers/mihomo"
	nodesHandler "corefusion/internal/api/handlers/nodes"
	nodeServerHandler "corefusion/internal/api/handlers/nodeserver"
	oauthHandler "corefusion/internal/api/handlers/oauth"
	outboundconfigHandler "corefusion/internal/api/handlers/outboundconfig"
	planHandler "corefusion/internal/api/handlers/plan"
	portalHandler "corefusion/internal/api/handlers/portal"
	rateLimitHandler "corefusion/internal/api/handlers/ratelimit"
	runtimecfgHandler "corefusion/internal/api/handlers/runtimecfg"
	securityHandler "corefusion/internal/api/handlers/security"
	servicesHandler "corefusion/internal/api/handlers/services"
	sharedomainconfigHandler "corefusion/internal/api/handlers/sharedomainconfig"
	subendHandler "corefusion/internal/api/handlers/subendpoint"
	subfmtHandler "corefusion/internal/api/handlers/subformat"
	subsHandler "corefusion/internal/api/handlers/subscriptions"
	systemHandler "corefusion/internal/api/handlers/system"
	trafficHandler "corefusion/internal/api/handlers/traffic"
	usersHandler "corefusion/internal/api/handlers/users"
	warpHandler "corefusion/internal/api/handlers/warp"
	xrayControlHandler "corefusion/internal/api/handlers/xraycontrol"
	xrayStatsHandler "corefusion/internal/api/handlers/xraystats"
	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/app/embed"
	domainuser "corefusion/internal/domain/user"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/runtime/cert"
	"corefusion/internal/runtime/firewall"
	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/runtime/process"
	authService "corefusion/internal/service/auth"
	backupService "corefusion/internal/service/backup"
	binariesSvc "corefusion/internal/service/binaries"
	certService "corefusion/internal/service/certificate"
	cfipSvc "corefusion/internal/service/cfip"
	clientSvc "corefusion/internal/service/client"
	clientauthSvc "corefusion/internal/service/clientauth"
	configsyncSvc "corefusion/internal/service/configsync"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	geoRuleSvc "corefusion/internal/service/georule"
	inboundconfigSvc "corefusion/internal/service/inboundconfig"
	logManageSvc "corefusion/internal/service/logmanage"
	runtimeapplySvc "corefusion/internal/service/runtimeapply"
	mihomoSvc "corefusion/internal/service/mihomo"
	nodeService "corefusion/internal/service/node"
	nodeServerSvc "corefusion/internal/service/nodeserver"
	oauthService "corefusion/internal/service/oauth"
	outboundconfigSvc "corefusion/internal/service/outboundconfig"
	planSvc "corefusion/internal/service/plan"
	portalSvc "corefusion/internal/service/portal"
	proxySvc "corefusion/internal/service/proxy"
	rateLimitSvc "corefusion/internal/service/ratelimit"
	svcService "corefusion/internal/service/service"
	sharedomainconfigSvc "corefusion/internal/service/sharedomainconfig"
	subendSvc "corefusion/internal/service/subendpoint"
	subfmtSvc "corefusion/internal/service/subformat"
	subRegion "corefusion/internal/service/subscription"
	subService "corefusion/internal/service/subscription"
	systemSvc "corefusion/internal/service/system"
	trafficSvc "corefusion/internal/service/traffic"
	trafficsyncSvc "corefusion/internal/service/trafficsync"
	userService "corefusion/internal/service/user"
	warpSvc "corefusion/internal/service/warp"
	accessPkg "corefusion/internal/service/access"
	"corefusion/internal/coredata"
	"corefusion/internal/service/audit"
	"corefusion/internal/store/repository"
)

// --- Network-layer (nftables) traffic accounting ----------------------------------
// Shares one IP tracker + bridge across the whole process. The bridge only accounts
// for subscriptions whose traffic is NOT already counted by xray itself.
var (
	networkTrafficTracker *trafficSvc.SubscriptionIPTracker
	networkBridgeOnce     sync.Once
)

// initNetworkTrafficAccounting returns the shared nftables tracker (nil if disabled)
// and starts the accounting bridge exactly once.
func initNetworkTrafficAccounting(subRepo *repository.SubscriptionRepository, clientRepo *repository.ClientRepository, inboundRepo *repository.InboundConfigRepository) *trafficSvc.SubscriptionIPTracker {
	if os.Getenv("CF_NETWORK_TRAFFIC_ACCOUNTING") == "0" {
		return nil
	}
	if networkTrafficTracker == nil {
		table := os.Getenv("CF_NETWORK_TRAFFIC_TABLE")
		if table == "" {
			table = "cf_traffic"
		}
		networkTrafficTracker = trafficSvc.NewSubscriptionIPTracker(table)
	}
	tracker := networkTrafficTracker
	networkBridgeOnce.Do(func() {
		bridge := trafficSvc.NewNetworkTrafficBridge(tracker, subRepo, clientRepo, inboundRepo)
		go bridge.Run(context.Background(), 30*time.Second)
	})
	return tracker
}

// --- Access control service (service/endpoint level) --------------------------
// Shares one access control service across the whole process; its ban + policy
// repositories are GORM-backed (service_ip_bans / service_access_policies).
var (
	sharedAccessSvc *accessPkg.Service
	accessSvcOnce   sync.Once
)

// getAccessService returns the shared access control service, instantiating it once.
func getAccessService(db *gorm.DB) *accessPkg.Service {
	accessSvcOnce.Do(func() {
		sharedAccessSvc = accessPkg.NewService(
			accessPkg.NewBanRepository(db),
			accessPkg.NewPolicyRepository(db),
		)
	})
	return sharedAccessSvc
}

// binaryListerAdapter adapts binaries.Installer to nodeserver.BinaryLister interface.
type binaryListerAdapter struct {
	installer *binariesSvc.Installer
}

func (a *binaryListerAdapter) ListReleases(ctx context.Context, coreType string) ([]struct {
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease"`
}, error) {
	releases, err := a.installer.ListReleases(ctx, coreType)
	if err != nil {
		return nil, err
	}
	var result []struct {
		Version    string `json:"version"`
		Prerelease bool   `json:"prerelease"`
	}
	for _, r := range releases {
		result = append(result, struct {
			Version    string `json:"version"`
			Prerelease bool   `json:"prerelease"`
		}{
			Version:    r.Version,
			Prerelease: r.Prerelease,
		})
	}
	return result, nil
}

func (a *binaryListerAdapter) ListReleasesForPlatform(ctx context.Context, coreType, osName, archName string) ([]struct {
	Version    string `json:"version"`
	Prerelease bool   `json:"prerelease"`
}, error) {
	releases, err := a.installer.ListReleasesForPlatform(ctx, coreType, osName, archName)
	if err != nil {
		return nil, err
	}
	var result []struct {
		Version    string `json:"version"`
		Prerelease bool   `json:"prerelease"`
	}
	for _, r := range releases {
		result = append(result, struct {
			Version    string `json:"version"`
			Prerelease bool   `json:"prerelease"`
		}{
			Version:    r.Version,
			Prerelease: r.Prerelease,
		})
	}
	return result, nil
}

// New creates a new Gin engine with all routes registered.
func New(database *gorm.DB) *gin.Engine {
	return NewWithSecret(database, "corefusion-dev-secret")
}

// NewWithSecret creates a new Gin engine with a custom JWT secret.
// webPath is the URL prefix for the frontend (e.g., "/apps"). Empty or "/" means root.
func NewWithSecret(database *gorm.DB, jwtSecret string) *gin.Engine {
	return NewWithConfig(database, jwtSecret, "")
}

// NewWithConfig creates a new Gin engine with config.
func NewWithConfig(database *gorm.DB, jwtSecret string, webPath string) *gin.Engine {
	return NewWithFullConfig(database, jwtSecret, webPath, "data", "unknown")
}

// processMgr is the global process manager for graceful shutdown.
var processMgr *process.Manager

// proxyMgr is the global proxy manager for graceful shutdown.
var proxyMgr *proxySvc.Manager

// caService is the global CA service for Agent mTLS certificate management.
var caService *agent.CertificateAuthority

// sharedNodeMgr is the global NodeServerManager shared between main.go and router.
// This is set from main.go to ensure the gRPC server (Agent connections) and
// the HTTP API (status queries, commands) use the same instance.
var sharedNodeMgr *agent.NodeServerManager

// SetCAService sets the CA service for the router (called from main.go before NewWithFullConfig).
func SetCAService(ca *agent.CertificateAuthority) {
	caService = ca
}

// SetNodeServerManager sets the shared NodeServerManager (called from main.go before NewWithFullConfig).
// This ensures that the gRPC server (which receives Agent connections) and the HTTP API
// (which queries Agent status) share the same instance and connection map.
func SetNodeServerManager(mgr *agent.NodeServerManager) {
	sharedNodeMgr = mgr
}

// SetProcessManager sets the process manager (called from main.go before NewWithFullConfig).
// If set, the router will reuse this instance instead of creating a new one.
// This ensures that LocalAgent and HTTP API handlers share the same process.Manager.
func SetProcessManager(pm *process.Manager) {
	processMgr = pm
}

// sharedLocalAgent is the global LocalAgent shared between main.go and router.
// Set from main.go to ensure there is only one LocalAgent instance with all
// dependencies (ProxyManager, BinaryInstaller) properly injected.
var sharedLocalAgent *agent.LocalAgent

// SetLocalAgent sets the shared LocalAgent (called from main.go before NewWithFullConfig).
// This ensures that both the gRPC server and HTTP API use the same LocalAgent
// with all dependencies (ProxyManager, BinaryInstaller) properly injected,
// instead of creating a second incomplete instance.
func SetLocalAgent(la *agent.LocalAgent) {
	sharedLocalAgent = la
}

// sharedUserSyncer is the global UserSyncer shared between main.go and router.
// Set from main.go to ensure that CommerceService can sync user credentials
// to all nodes (local + remote) when a plan is activated or refunded.
var sharedUserSyncer clientauthSvc.UserSyncer

// sharedAccountSyncer is the global AccountSyncer shared between NewWithFullConfig
// and NewPortalEngine. Set in NewWithFullConfig after planSvcInst is created,
// so that the portal engine's CommerceService can also trigger account sync.
var sharedAccountSyncer clientauthSvc.AccountSyncer

// SetUserSyncer sets the shared UserSyncer (called from main.go before NewWithFullConfig).
// This enables CommerceService to sync user accounts to nodes after plan activation
// and remove them on refund.
func SetUserSyncer(syncer clientauthSvc.UserSyncer) {
	sharedUserSyncer = syncer
}

// configSyncHookAdapter adapts configsync.Service to the ConfigSyncHook interface
// used by inboundconfig and outboundconfig handlers.
// When triggerMirrorSync is true (inbound changes), it also triggers mirror node
// sync so that mirrors take effect at the same time as the primary.
// Outbound changes do NOT trigger mirror sync — mirrors only replicate inbounds
// and caddy/nginx proxy configs.
type configSyncHookAdapter struct {
	syncSvc           *configsyncSvc.Service
	nodeServerSvc     *nodeServerSvc.Service
	triggerMirrorSync bool
}

func (a *configSyncHookAdapter) SyncServerConfig(_ context.Context, serverID int64, coreType string) {
	if a.syncSvc != nil {
		if err := a.syncSvc.SyncNow(context.Background()); err != nil {
			log.Printf("[router] configSyncHook: SyncNow failed for server %d: %v", serverID, err)
		}
	}
	// Only trigger mirror sync for inbound changes (not outbound).
	// Mirrors only replicate inbounds and caddy/nginx proxy configs.
	if a.triggerMirrorSync && a.nodeServerSvc != nil {
		a.nodeServerSvc.SyncMirrorsForPrimary(context.Background(), serverID)
	}
}

// trafficResetHookAdapter combines inboundconfig.Service (for resetting user traffic)
// with configsync.Service (for triggering remote sync) and nodeserver.Service
// (for SyncData) to implement the TrafficResetHook interface.
type trafficResetHookAdapter struct {
	inboundSvc    *inboundconfigSvc.Service
	syncSvc       *configsyncSvc.Service
	nodeServerSvc *nodeServerSvc.Service
}

func (a *trafficResetHookAdapter) ResetUserTrafficByEmail(ctx context.Context, email string) ([]int64, error) {
	return a.inboundSvc.ResetUserTrafficByEmail(ctx, email)
}

func (a *trafficResetHookAdapter) SyncServerConfig(_ context.Context, serverID int64, coreType string) {
	if a.syncSvc != nil {
		if err := a.syncSvc.SyncNow(context.Background()); err != nil {
			log.Printf("[router] trafficResetHook: SyncNow failed for server %d: %v", serverID, err)
		}
	}
	// Also trigger SyncData to push the updated inbound_configs (with reset
	// _usedUp/_usedDown) to the remote Agent's local SQLite DB.
	if a.nodeServerSvc != nil && serverID > 1 {
		go func() {
			if _, err := a.nodeServerSvc.SyncServerData(context.Background(), serverID, true); err != nil {
				log.Printf("[router] trafficResetHook: SyncData failed for server %d: %v", serverID, err)
			} else {
				log.Printf("[router] trafficResetHook: SyncData success for server %d", serverID)
			}
		}()
	}
}

// syncDataHookAdapter adapts nodeserver.Service to the SyncDataHook interface.
type syncDataHookAdapter struct {
	nodeServerSvc *nodeServerSvc.Service
}

func (a *syncDataHookAdapter) SyncServerData(_ context.Context, serverID int64) {
	if a.nodeServerSvc != nil && serverID > 1 {
		go func() {
			if _, err := a.nodeServerSvc.SyncServerData(context.Background(), serverID, true); err != nil {
				log.Printf("[router] syncDataHook: SyncData failed for server %d: %v", serverID, err)
			} else {
				log.Printf("[router] syncDataHook: SyncData success for server %d", serverID)
			}
		}()
	}
}

// Shutdown stops all managed processes. Call this when the panel exits.
func Shutdown() {
	ctx := context.Background()
	// Stop core processes
	if processMgr != nil {
		processMgr.StopAll(ctx)
	}
	// Stop managed proxies without marking as manually stopped
	if proxyMgr != nil {
		log.Println("stopping managed proxies...")
		proxyMgr.StopProxyForShutdown(ctx, proxySvc.ProxyNginx)
		proxyMgr.StopProxyForShutdown(ctx, proxySvc.ProxyCaddy)
	}
}

// NewWithFullConfig creates a new Gin engine with full config including data directory and version.
func NewWithFullConfig(database *gorm.DB, jwtSecret string, webPath string, dataDir string, version string, listenPort ...string) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	// Note: MaintenanceGuard is NOT mounted on the admin panel engine.
	// It is only mounted on the subscription and portal engines so that
	// admins can always access the panel to toggle maintenance on/off.

	if dataDir == "" {
		dataDir = "data"
	}

	// Determine listen port for internal API calls
	apiPort := "8080"
	if len(listenPort) > 0 && listenPort[0] != "" {
		apiPort = listenPort[0]
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(database)
	nodeRepo := repository.NewNodeRepository(database)
	svcRepo := repository.NewServiceRepository(database)
	endpointRepo := repository.NewEndpointRepository(database)
	subRepo := repository.NewSubscriptionRepository(database)
	accessLogRepo := repository.NewAccessLogRepository(database)
	rotationRepo := repository.NewRotationRepository(database)
	certRepo := repository.NewCertificateRepository(database)
	ruleSourceRepo := repository.NewRuleSourceRepository(database)
	geoRuleRepo := repository.NewGeoRuleRepository(database)
	warpRepo := repository.NewWarpRepository(database)
	guardianRepo := repository.NewProcessGuardianRepository(database)
	crashLogRepo := repository.NewCrashLogRepository(database)
	dnsChannelRepo := repository.NewDNSChannelRepository(database)
	acmeAccountRepo := repository.NewACMEAccountConfigRepository(database)
	systemSettingRepo := repository.NewSystemSettingRepository(database)
	notifTmplRepo := repository.NewNotificationTemplateRepository(database)
	backupRepo := repository.NewBackupRepository(database)
	subfmtRepo := repository.NewSubscriptionFormatRepository(database)
	subfmtRuleRepo := repository.NewSubscriptionRuleRepository(database)
	subendRepo := repository.NewSubscriptionEndpointRepository(database)
	shareDomainRepo := repository.NewShareDomainConfigRepository(database)
	clientRepo := repository.NewClientRepository(database)

	coreConfigRepo := repository.NewCoreConfigRepository(database)
	coreConfigSvc := coreconfigSvc.NewService(coreConfigRepo)
	if err := coreConfigSvc.SeedDefaults(context.Background(), 1); err != nil {
		log.Printf("WARNING: seed core configs failed: %v", err)
	}

	// Inbound config service (separate table for inbound configurations)
	inboundConfigRepo := repository.NewInboundConfigRepository(database)

	// Plan v2: templates, variants, node groups, promotions, orders (created early, many services depend on planSvc)
	planTmplRepo := repository.NewPlanTemplateRepository(database)
	planVariantRepo := repository.NewPlanVariantRepository(database)
	nodeGroupRepo := repository.NewNodeGroupRepository(database)
	nodeGroupMemberRepo := repository.NewNodeGroupMemberRepository(database)
	promoRepo := repository.NewPromotionRepository(database)
	planOrderRepo := repository.NewPlanOrderRepository(database)
	extNodeRepo := repository.NewExternalNodeRepository(database)
	planSvcInst := planSvc.NewService(database, planTmplRepo, planVariantRepo, nodeGroupRepo, nodeGroupMemberRepo, promoRepo, planOrderRepo, clientRepo, inboundConfigRepo, extNodeRepo)
	// Trial plan auto-convert/expiry scheduler (hourly)
	planSvcInst.StartTrialScheduler(context.Background(), time.Hour)
	// Pending downgrade auto-execution scheduler (every 5 minutes)
	planSvcInst.StartDowngradeScheduler(context.Background(), 5*time.Minute)

	inboundConfigSvc := inboundconfigSvc.NewService(inboundConfigRepo, database)
	outboundConfigRepo := repository.NewOutboundConfigRepository(database)
	outboundConfigSvc := outboundconfigSvc.NewService(outboundConfigRepo)

	// Config sync service (periodically syncs DB configs to disk)
	configSyncSvc := configsyncSvc.NewService(coreConfigSvc, inboundConfigSvc, outboundConfigSvc, dataDir, 5*time.Minute)

	// Inject process.Manager as the config writer so configsync uses the same
	// mutex-protected write path as LocalAgent.SyncConfig, avoiding race conditions.
	if processMgr != nil {
		configSyncSvc.SetConfigWriter(processMgr)
	}

	// Inject NodeServerRepository as the server lister so configsync can
	// discover all registered servers (local + remote) for multi-node sync.
	nodeServerRepoForSync := repository.NewNodeServerRepository(database)
	configSyncSvc.SetServerLister(nodeServerRepoForSync)

	// Runtime config workflow (snapshot / dry-run / apply / rollback / disable-proxy)
	var runtimeApplyWriter runtimeapplySvc.ConfigWriter
	if processMgr != nil {
		runtimeApplyWriter = processMgr
	}
	runtimeApplySvcInst := runtimeapplySvc.NewService(database, dataDir, configSyncSvc, runtimeApplyWriter, nodeServerRepoForSync, sharedNodeMgr)

	// Initialize services
	authSvc := authService.NewService(userRepo, jwtSecret)
	userSvc := userService.NewService(userRepo)
	nodeSvc := nodeService.NewService(nodeRepo, sharedNodeMgr, subRepo, clientRepo, planTmplRepo)
	svcSvc := svcService.NewService(svcRepo, endpointRepo)
	subSvc := subService.NewService(subRepo, accessLogRepo, rotationRepo, clientRepo, planSvcInst)
	certSvc := certService.NewService(certRepo, dataDir, "", "lets_encrypt")
	// Initialize HTTP-01 challenge provider for ACME certificate issuance
	// This integrates with the Gin router to serve challenges at /.well-known/acme-challenge/
	http01Provider := cert.NewHTTP01Provider()
	certSvc.SetHTTP01Provider(http01Provider)
	// Sync cert content from DB to disk on startup
	certSvc.SyncCertsToDisk(nil)
	binaryResolver := binariesSvc.NewResolver(dataDir)
	systemSettingSvc := systemSvc.NewSettingService(systemSettingRepo, notifTmplRepo)
	geoRuleSvcInst := geoRuleSvc.NewService(ruleSourceRepo, geoRuleRepo, dataDir, systemSettingSvc)

	// Traffic sync service (periodically syncs traffic stats to DB)
	trafficSyncSvc := trafficsyncSvc.NewService(systemSettingSvc, inboundConfigSvc, 30*time.Second)
	geoRuleScheduler := geoRuleSvc.NewScheduler(ruleSourceRepo, geoRuleSvcInst)
	warpSvcInst := warpSvc.NewService(warpRepo)
	rateLimitConfigRepo := repository.NewRateLimitConfigRepository(database)
	rateLimitSvcInst := rateLimitSvc.NewService(rateLimitConfigRepo)
	dnsChannelSvc := certService.NewDNSChannelService(dnsChannelRepo)
	acmeEmailSvc := certService.NewACMEEmailService(acmeAccountRepo)
	notifier := systemSvc.NewNotifier(systemSettingSvc)
	backupSvcInst := backupService.NewService(backupRepo, dataDir, database)
	// Start the backup scheduler so scheduled backups and the retention policy
	// actually run (it was previously defined but never started).
	backupScheduler := backupService.NewBackupScheduler(backupSvcInst)
	backupScheduler.Start()
	systemSvc.RegisterNotifier(notifier, repository.NewNotificationTemplateRepository(database))
	audit.Register(database)
	proxySiteRepo := repository.NewProxySiteRepository(database)
	caddySiteRepo := repository.NewCaddySiteRepository(database)
	nginxSiteRepo := repository.NewNginxSiteRepository(database)
	proxyMgr = proxySvc.NewManager(dataDir, proxySiteRepo, systemSettingRepo)
	proxyMgr.SetCrashLogRepo(repository.NewCrashLogRepository(database))
	caddyMgr := proxySvc.NewCaddyManager(dataDir, caddySiteRepo, systemSettingRepo)
	nginxMgr := proxySvc.NewNginxManager(dataDir, nginxSiteRepo, systemSettingRepo)

	// Set server_id = 1 (local node) for per-node site filtering.
	// All proxy/caddy/nginx site queries filter by server_id to ensure
	// data isolation between node servers.
	proxyMgr.SetServerID(1)
	caddyMgr.SetServerID(1)
	nginxMgr.SetServerID(1)

	// Log management
	logPolicyRepo := repository.NewLogRotationPolicyRepository(database)
	logJobRepo := repository.NewLogRotationJobRepository(database)
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
	logManageSvc := logManageSvc.NewService(logPolicyRepo, logJobRepo, systemSettingRepo, caddySiteRepo, dataDir, proxyBrokerAdapter)

	// Wire log mode provider so process.Manager knows the log output mode for each core
	processMgr.SetLogModeProvider(logManageSvc)

	// Wire core log broker provider so logmanage can read core process stdout
	logManageSvc.SetCoreLogBrokerProvider(&coreLogBrokerAdapter{processMgr: processMgr})

	// Wire client log packaging dependencies (for subscription user log access)
	clientLogPkgRepo := repository.NewClientLogPackageRepository(database)
	if sharedNodeMgr != nil {
		logManageSvc.InitClientLogDeps(clientLogPkgRepo, clientRepo, planSvcInst, inboundConfigRepo, sharedNodeMgr)
	} else {
		logManageSvc.InitClientLogDeps(clientLogPkgRepo, clientRepo, planSvcInst, inboundConfigRepo, nil)
	}

	// Wire log mode provider to configsync so generated configs include correct log section
	configSyncSvc.SetLogModeProvider(logManageSvc)

	// Wire core config section getter so logmanage can derive output mode from
	// the actual xray/singbox log section (syncs with protocol center changes)
	logManageSvc.SetCoreConfigSectionGetter(coreConfigSvc)

	// Wire config generators so auto-start uses the new separated managers
	proxyMgr.SetCaddyConfigGen(caddyMgr.GenerateConfig)
	proxyMgr.SetNginxConfigGen(nginxMgr.GenerateConfig)

	// Seed default geo rules for local node (server_id=1)
	geoRuleSvcInst.SeedDefaults(nil, 1)

	// Auto-download missing binaries and rules on startup (async, non-blocking).
	// After a backup restore (database-only backup), binaries and rules are
	// absent from disk. This ensures they are re-downloaded automatically so
	// the system can resume normal operation without manual intervention.
	go func() {
		ctx := context.Background()

		// --- 1. Auto-download missing core binaries ---
		// Only download cores that have auto_start=true in inbound_services.
		// Cores that are configured but not enabled don't need their binary on disk.
		startupInstaller := binariesSvc.NewInstaller(dataDir, binaryResolver, systemSettingSvc)
		if processMgr != nil {
			startupInstaller.SetProcessController(processMgr)
		}

		var neededCores []string
		database.Table("inbound_services").
			Joins("JOIN process_guardians ON process_guardians.service_id = inbound_services.id").
			Where("process_guardians.auto_start = ? AND process_guardians.manually_stopped = ?", true, false).
			Distinct("inbound_services.core_type").
			Pluck("inbound_services.core_type", &neededCores)

		installed := binaryResolver.ListInstalled()
		installedMap := make(map[string]bool)
		for _, b := range installed {
			if b.Installed {
				installedMap[b.CoreType] = true
			}
		}

		for _, ct := range neededCores {
			if installedMap[ct] {
				continue
			}
			log.Printf("[startup] auto-downloading missing binary: %s", ct)
			info, err := startupInstaller.InstallForPlatform(ctx, ct, "", "", "", runtime.GOOS, runtime.GOARCH)
			if err != nil {
				log.Printf("[startup] failed to download %s: %v", ct, err)
			} else {
				log.Printf("[startup] successfully downloaded %s (version: %s)", ct, info.Version)
			}
		}

		// --- 2. Auto-download missing proxy binaries (Caddy/Nginx) ---
		// If a proxy has auto_start enabled but its binary is not on disk
		// (e.g. after a database-only restore), download it automatically.
		for _, pt := range []proxySvc.ProxyType{proxySvc.ProxyCaddy, proxySvc.ProxyNginx} {
			info := proxyMgr.DetectProxy(pt)
			if info.Installed {
				continue
			}
			autoStart, _, _ := proxyMgr.GetProxyGuardianSettings(ctx, pt)
			if !autoStart {
				continue
			}
			log.Printf("[startup] auto-downloading missing proxy binary: %s", pt)
			if err := proxyMgr.InstallProxy(ctx, pt); err != nil {
				log.Printf("[startup] failed to download %s: %v", pt, err)
			} else {
				log.Printf("[startup] successfully downloaded %s", pt)
			}
		}

		// --- 3. Auto-download missing geo rule files ---
		// Check if any rule files referenced in the DB are missing from disk.
		// If so, trigger an async sync to download them.
		rulesDir := filepath.Join(dataDir, "rules")
		ruleFiles, err := geoRuleRepo.ListByServer(ctx, 1)
		if err == nil {
			needSync := false
			for _, f := range ruleFiles {
				if _, err := os.Stat(filepath.Join(rulesDir, f.FileName)); err != nil {
					needSync = true
					break
				}
			}
			if needSync {
				log.Println("[startup] downloading missing geo rule files")
				geoRuleSvcInst.StartSyncAllByServer(1)
			}
		}
	}()

	// Subscription format service
	subfmtSvcInst := subfmtSvc.NewService(subfmtRepo, subfmtRuleRepo)
	subfmtSvcInst.SeedDefaults(context.Background())

	// Subscription endpoint service
	subendSvcInst := subendSvc.NewService(subendRepo, caddySiteRepo, nginxSiteRepo, func(ctx context.Context, proxyType string) bool {
		info := proxyMgr.DetectProxy(proxySvc.ProxyType(proxyType))
		return info.Status == "running"
	}, "8080", func(proxyType string) {
		if proxyType == "nginx" {
			proxyMgr.ReloadProxy(context.Background(), proxySvc.ProxyNginx)
		} else {
			proxyMgr.ReloadProxy(context.Background(), proxySvc.ProxyCaddy)
		}
	})

	// Client service
	inboundMgr := clientSvc.NewInboundManager(inboundConfigRepo, apiPort, jwtSecret)
	clientSvcInst := clientSvc.NewService(clientRepo, planSvcInst, subRepo, inboundConfigRepo, inboundMgr)

	// Inject client service into plan service for account synchronization
	// when new nodes are added to a plan's node group.
	planSvcInst.SetAccountSyncer(clientSvcInst)
	// Inject subscription repo so Renew/Upgrade can update the subscriptions table.
	planSvcInst.SetSubscriptionRepo(subRepo)
	// Inject user syncer so Renew/Upgrade push updated client config to all nodes.
	if sharedUserSyncer != nil {
		planSvcInst.SetUserSyncer(sharedUserSyncer)
	}
	// Inject setting store so plan service can read currency.default from DB.
	planSvcInst.SetSettingStore(systemSettingRepo)

	// Mihomo service
	mihomoConfigRepo := repository.NewMihomoConfigRepository(database)
	mihomoGeneralRepo := repository.NewMihomoGeneralRepository(database)
	mihomoDNSRepo := repository.NewMihomoDNSRepository(database)
	mihomoHostRepo := repository.NewMihomoHostRepository(database)
	mihomoTunnelRepo := repository.NewMihomoTunnelRepository(database)
	mihomoSnifferRepo := repository.NewMihomoSnifferRepository(database)
	mihomoExtCtrlRepo := repository.NewMihomoExternalControllerRepository(database)
	mihomoTLSRepo := repository.NewMihomoTLSRepository(database)
	mihomoTunRepo := repository.NewMihomoTunRepository(database)
	mihomoProxyGroupRepo := repository.NewMihomoProxyGroupRepository(database)
	mihomoRuleProviderRepo := repository.NewMihomoRuleProviderRepository(database)
	mihomoRuleRepo := repository.NewMihomoRuleRepository(database)
	mihomoOutboundRepo := repository.NewMihomoSpecialOutboundRepository(database)
	mihomoSvcInst := mihomoSvc.NewService(
		mihomoConfigRepo, mihomoGeneralRepo, mihomoDNSRepo, mihomoHostRepo,
		mihomoTunnelRepo, mihomoSnifferRepo, mihomoExtCtrlRepo, mihomoTLSRepo,
		mihomoTunRepo, mihomoProxyGroupRepo, mihomoRuleProviderRepo, mihomoRuleRepo, mihomoOutboundRepo,
	)

	// Client auth service
	clientauthSvcInst := clientauthSvc.NewService(clientRepo, planTmplRepo, subRepo, systemSettingSvc, jwtSecret)

	// Start rule sync scheduler
	geoRuleScheduler.Start()

	// Initialize handlers
	authH := authHandler.NewHandler(authSvc)
	userH := usersHandler.NewHandler(userSvc)
	nodeH := nodesHandler.NewHandler(nodeSvc)
	coreConfigH := coreconfigHandler.NewHandler(coreConfigSvc)
	svcH := servicesHandler.NewHandler(svcSvc)
	subH := subsHandler.NewHandler(subSvc)
	subfmtH := subfmtHandler.NewHandler(subfmtSvcInst)
	subendH := subendHandler.NewHandler(subendSvcInst)
	cfIPEntryRepo := repository.NewCFIPEntryRepository(database)
	extSrcRepo := repository.NewExternalNodeSourceRepository(database)

	planH := planHandler.NewHandler(planSvcInst)

	publicSubH := subsHandler.NewPublicHandler(subRepo, inboundConfigRepo, shareDomainRepo, cfIPEntryRepo, clientRepo, planSvcInst, extNodeRepo, mihomoConfigRepo, mihomoRuleRepo, systemSettingRepo, accessLogRepo)
	publicSubH.SetRateLimitPusher(rateLimitSvcInst, nodeServerRepoForSync)
	if tracker := initNetworkTrafficAccounting(subRepo, clientRepo, inboundConfigRepo); tracker != nil {
		publicSubH.SetNetworkTracker(tracker)
	}
	clientH := clientHandler.NewHandler(clientSvcInst)
	clientauthH := clientauthHandler.NewHandler(clientauthSvcInst)
	clientauthH.SetRateLimitPusher(rateLimitSvcInst, nodeServerRepoForSync, clientRepo, planTmplRepo)
	commerceSvcInst := clientauthSvc.NewCommerceService(
		database, planTmplRepo,
		repository.NewPlanVariantRepository(database),
		repository.NewPlanOrderRepository(database),
		repository.NewPromotionRepository(database),
		clientRepo, subRepo,
		repository.NewChargeSiteRepository(database),
		repository.NewChargeOrderRepository(database),
		repository.NewChargeCardRepository(database),
		repository.NewClientBalanceRepository(database),
		repository.NewBalanceTransactionRepository(database),
		systemSettingSvc, jwtSecret,
	)
	commerceH := clientauthHandler.NewCommerceHandler(commerceSvcInst)

	// Start callback retry scheduler (runs every 1 minute)
	commerceSvcInst.StartRetryScheduler(1 * time.Minute)

	// v3.2: Start processing recovery scheduler (runs every 1 minute)
	// Recovers stuck processing orders, cleans up nonces (24h) and expired secrets (7d)
	commerceSvcInst.StartProcessingRecovery(1 * time.Minute)

	// Start expired order cleanup scheduler (runs every 5 minutes)
	commerceSvcInst.StartCleanupScheduler(5 * time.Minute)

	// P1-P3 extended services
	referralSvcInst := clientauthSvc.NewReferralService(
		database, repository.NewReferralCodeRepository(database),
		repository.NewReferralRecordRepository(database),
		repository.NewReferralWithdrawalRepository(database),
		repository.NewClientBalanceRepository(database),
		repository.NewBalanceTransactionRepository(database),
		clientRepo, systemSettingSvc,
	)
	ticketSvcInst := clientauthSvc.NewTicketService(
		repository.NewTicketRepository(database),
		repository.NewTicketMessageRepository(database),
		clientRepo,
	)
	giftSvcInst := clientauthSvc.NewGiftService(
		repository.NewClientGiftRepository(database),
		clientRepo, subRepo, database,
	)
	articleSvcInst := clientauthSvc.NewArticleService(
		repository.NewArticleRepository(database),
	)
	sharedAcctSvcInst := clientauthSvc.NewSharedAccountService(
		repository.NewSharedAccountRepository(database),
		repository.NewSharedAccountRepairLogRepository(database),
		clientRepo,
	)
	ipWhitelistSvcInst := clientauthSvc.NewIPWhitelistService(
		repository.NewIPWhitelistEntryRepository(database),
		clientRepo,
	)
	selfUnbanSvcInst := clientauthSvc.NewSelfUnbanService(
		repository.NewClientSelfUnbanLogRepository(database),
		clientRepo, systemSettingSvc,
	)
	subResetSvcInst := clientauthSvc.NewSubscriptionResetService(subRepo, clientRepo)
	subEndpointRepo := repository.NewSubscriptionEndpointRepository(database)
	clientExtH := clientextHandler.NewHandler(
		referralSvcInst, ticketSvcInst, giftSvcInst, articleSvcInst,
		sharedAcctSvcInst, ipWhitelistSvcInst, selfUnbanSvcInst, subResetSvcInst,
		clientRepo, repository.NewClientOAuthBindingRepository(database),
		subRepo, subEndpointRepo, publicSubH, database,
	)

	// Set balance repo on clientauth service for /me extension
	clientauthSvcInst.SetBalanceRepo(repository.NewClientBalanceRepository(database))

	// P1: 注入 ReferralService — 注册时自动绑定邀请关系 + 套餐激活时触发返利
	clientauthSvcInst.SetReferralService(referralSvcInst)
	commerceSvcInst.SetReferralService(referralSvcInst)

	// 注入 UserSyncer — 退款过期后从节点移除用户凭证
	if sharedUserSyncer != nil {
		commerceSvcInst.SetUserSyncer(sharedUserSyncer)
	}

	// 注入 AccountSyncer — 套餐激活后复用 plan.Service.SyncAccountsForTemplate
	// 为客户端在所有入站创建账号（生成 UUID/Password → 写 config_json → xray gRPC 热添加）
	commerceSvcInst.SetAccountSyncer(planSvcInst)
	sharedAccountSyncer = planSvcInst // 供 NewPortalEngine 使用

	mihomoH := mihomoHandler.NewHandler(mihomoSvcInst)
	certH := certsHandler.NewHandler(certSvc)
	// Reuse processMgr if injected from main.go (shared with LocalAgent),
	// otherwise create a new one.
	if processMgr == nil {
		processMgr = process.NewManager(dataDir, svcRepo, guardianRepo, crashLogRepo, binaryResolver)
	}
	coreConfigH.SetProcessManager(processMgr)

	// Set config generator: reads from core_configs and writes merged config file
	// Uses the shared PrepareFullConfig function to ensure all nodes (local + remote)
	// get identical config generation logic: cleanEmpty, strip _disabled, filter
	// disabled routing rules, API inbound/routing, inbounds/outbounds merge.
	processMgr.SetConfigGenerator(func(ctx context.Context, coreType string, configPath string) error {
		fullConfig, err := coreConfigSvc.GetFull(ctx, 1, coreType)
		if err != nil {
			return err
		}

		data, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
			Ctx:              ctx,
			ServerID:         1,
			CoreType:         coreType,
			FullConfig:       fullConfig,
			InboundProvider:  inboundConfigSvc,
			OutboundProvider: outboundConfigSvc,
		})
		if err != nil {
			return err
		}

		return os.WriteFile(configPath, data, 0640)
	})
	processH := servicesHandler.NewProcessHandler(processMgr, svcRepo, crashLogRepo, dataDir)
	metricsH := metricsHandler.NewHandler(database)
	securityH := securityHandler.NewHandler(database)
	backupsH := backupsHandler.NewHandler(database, dataDir, backupSvcInst)
	systemH := systemHandler.NewHandler(database, systemSettingSvc, dataDir, version)
	proxyH := systemHandler.NewProxyHandler(proxyMgr, caddyMgr, nginxMgr)
	caddyH := systemHandler.NewCaddyHandler(caddyMgr, proxyMgr)
	nginxH := systemHandler.NewNginxHandler(nginxMgr, proxyMgr)
	geoRuleH := geoRuleHandler.NewHandler(geoRuleSvcInst, geoRuleScheduler)
	if sharedNodeMgr != nil {
		geoRuleSvcInst.SetNodeOperator(sharedNodeMgr)
		// Inject RemoteSyncer so configsync can push configs to remote Agents.
		configSyncSvc.SetRemoteSyncer(sharedNodeMgr)
		// Wire rate limit service so PushIPAssociation can reach remote Agents.
		rateLimitSvcInst.SetNodeOperator(sharedNodeMgr)
	}
	warpH := warpHandler.NewHandler(warpSvcInst)
	rateLimitH := rateLimitHandler.NewHandler(rateLimitSvcInst)
	certChannelH := certsHandler.NewChannelHandler(dnsChannelSvc)
	certEmailH := certsHandler.NewEmailHandler(acmeEmailSvc)
	logManageH := logManageHandler.NewHandler(logManageSvc)
	// Wire node log manager for centralized remote node log management
	if sharedNodeMgr != nil {
		logManageHandler.SetNodeLogManager(sharedNodeMgr)
	}

	// Wire proxy log mode applier for remote nodes.
	// When log mode changes on a remote node, the proxy config (Caddyfile/nginx.conf)
	// must be regenerated with new log directives, synced to the agent, and the
	// proxy reloaded/restarted accordingly.
	if sharedNodeMgr != nil {
		logManageHandler.SetProxyLogModeApplier(func(ctx context.Context, serverID int64, proxyType string, newMode string) {
			var config string
			var err error
			switch proxyType {
			case "caddy":
				config, err = caddyMgr.GenerateConfigForServer(ctx, serverID)
			case "nginx":
				config, err = nginxMgr.GenerateConfigForServer(ctx, serverID)
			default:
				return
			}
			if err != nil {
				log.Printf("router: generate %s config for server %d after log mode switch: %v", proxyType, serverID, err)
				return
			}

			// Sync the regenerated config to the remote agent
			if err := sharedNodeMgr.SyncProxyConfig(ctx, serverID, proxyType, []byte(config)); err != nil {
				log.Printf("router: sync %s config to server %d after log mode switch: %v", proxyType, serverID, err)
				return
			}

			// Reload for file/off mode (config change only), restart for stdout mode
			// (need to re-attach stdout pipe)
			action := "reload"
			if newMode == "stdout" {
				action = "restart"
			}
			if err := sharedNodeMgr.ControlProxy(ctx, serverID, proxyType, action); err != nil {
				log.Printf("router: %s %s on server %d after log mode switch: %v", action, proxyType, serverID, err)
			} else {
				log.Printf("router: %s %s on server %d after log mode switch to %s", action, proxyType, serverID, newMode)
			}
		})
	}

	// Client log handler (for subscription user log packaging)
	clientLogH := clientauthHandler.NewLogHandler(logManageSvc)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ACME HTTP-01 challenge endpoint
	// This serves challenge responses at /.well-known/acme-challenge/<token>
	// Compatible with acme.sh and all ACME clients
	r.GET("/.well-known/acme-challenge/:token", http01Provider.Handler())

	// Serve embedded frontend static files
	adminFSBytes := embed.AdminFSBytes()

	// Normalize web path
	if webPath == "" {
		webPath = "/"
	}
	webPath = strings.TrimRight(webPath, "/")
	if webPath == "/" {
		webPath = ""
	}

	// SPA fallback + static asset serving
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API routes always accessible
		if strings.HasPrefix(path, "/api/") {
			c.JSON(404, gin.H{"code": "NOT_FOUND", "message": "endpoint not found"})
			return
		}

		// Check for static assets (both /assets/ and {webPath}/assets/)
		assetPath := ""
		if strings.HasPrefix(path, "/assets/") {
			assetPath = path
		} else if webPath != "" && strings.HasPrefix(path, webPath+"/assets/") {
			assetPath = strings.TrimPrefix(path, webPath)
		} else if strings.HasPrefix(path, "/p/") && !strings.HasSuffix(path, "/p/") {
			// Serve portal static files (e.g. qrcode.min.js) from embed
			filePath := "dist/portal" + strings.TrimPrefix(path, "/p")
			data, err := adminFSBytes.ReadFile(filePath)
			if err != nil {
				c.Status(404)
				return
			}
			ext := strings.ToLower(filepath.Ext(filePath))
			ct := "application/octet-stream"
			switch ext {
			case ".js":
				ct = "application/javascript; charset=utf-8"
			case ".css":
				ct = "text/css; charset=utf-8"
			}
			c.Data(200, ct, data)
			return
		}

		if assetPath != "" {
			filePath := "dist" + assetPath
			data, err := adminFSBytes.ReadFile(filePath)
			if err != nil {
				c.Status(404)
				return
			}
			ext := strings.ToLower(filepath.Ext(assetPath))
			var ct string
			switch ext {
			case ".js", ".mjs":
				ct = "application/javascript; charset=utf-8"
			case ".css":
				ct = "text/css; charset=utf-8"
			case ".json":
				ct = "application/json; charset=utf-8"
			case ".svg":
				ct = "image/svg+xml"
			case ".html", ".htm":
				ct = "text/html; charset=utf-8"
			case ".woff", ".woff2":
				ct = "font/" + ext[1:]
			case ".ttf":
				ct = "font/ttf"
			case ".png":
				ct = "image/png"
			case ".jpg", ".jpeg":
				ct = "image/jpeg"
			case ".gif":
				ct = "image/gif"
			case ".ico":
				ct = "image/x-icon"
			default:
				ct = mime.TypeByExtension(ext)
				if ct == "" {
					ct = http.DetectContentType(data)
				}
			}
			c.Data(200, ct, data)
			return
		}

		// If web path is configured
		if webPath != "" {
			// Only serve requests under web path
			if strings.HasPrefix(path, webPath) {
				data, err := adminFSBytes.ReadFile("dist/index.html")
				if err != nil {
					c.Status(500)
					return
				}
				// Inject base path and asset paths when using web path
				html := string(data)
				if webPath != "" {
					html = strings.ReplaceAll(html, `src="/assets/`, `src="`+webPath+"/assets/")
					html = strings.ReplaceAll(html, `href="/assets/`, `href="`+webPath+"/assets/")
					// Inject base path for Vue Router
					inject := fmt.Sprintf(`<script>window.__CF_BASE_PATH__="%s/";</script>`, webPath)
					html = strings.Replace(html, "<head>", "<head>"+inject, 1)
				}
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				c.Data(200, "text/html; charset=utf-8", []byte(html))
				return
			}

			// Everything else (including /) -> 404
			c.Status(404)
			return
		}

		// No web path: only serve index.html for known SPA routes
		if isSPARoute(path) {
			data, err := adminFSBytes.ReadFile("dist/index.html")
			if err != nil {
				c.Status(500)
				return
			}
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Data(200, "text/html; charset=utf-8", data)
			return
		}
		c.Status(404)
	})

	// Portal template service — shared across protected routes, /p page, and portal engine
	portalTmplSvc := portalSvc.NewService(repository.NewPortalTemplateRepository(database))
	portalTmplSvc.WithSettingStore(systemSettingSvc)
	portalTmplSvc.SeedBuiltInTemplate(context.Background())

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Auth (public): only credential exchange belongs here. Anything that
		// resolves the caller's identity must sit behind AuthRequired, or the
		// user_id read from the context is silently 0.
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/login", middleware.LoginRateLimit(), authH.Login)
			authGroup.POST("/refresh", authH.Refresh)
			authGroup.POST("/logout", authH.Logout)
		}

		// Auth (authenticated): operates on the calling user.
		authSelfGroup := v1.Group("/auth")
		authSelfGroup.Use(middleware.AuthRequired())
		{
			authSelfGroup.GET("/me", authH.Me)
			authSelfGroup.POST("/change-password", authH.ChangePassword)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// Users: administrator account management is super_admin only,
			// otherwise any authenticated principal could mint itself a
			// super_admin account. Listing stays at admin level.
			usersGroup := protected.Group("/users")
			{
				usersGroup.GET("", middleware.RequireRole(domainuser.RoleAdmin), userH.List)
				usersGroup.POST("", middleware.RequireRole(domainuser.RoleSuperAdmin), userH.Create)
				usersGroup.PUT("/:id", middleware.RequireRole(domainuser.RoleSuperAdmin), userH.Update)
				usersGroup.DELETE("/:id", middleware.RequireRole(domainuser.RoleSuperAdmin), userH.Delete)
			}

			// Nodes
			nodesGroup := protected.Group("/nodes")
			{
				nodesGroup.GET("", nodeH.List)
				nodesGroup.POST("", nodeH.Create)
				nodesGroup.GET("/:id", nodeH.GetByID)
				nodesGroup.PUT("/:id", nodeH.Update)
				nodesGroup.DELETE("/:id", nodeH.Delete)
				nodesGroup.GET("/:id/online-users", nodeH.GetOnlineUsers)
			}

			// Inbound Configs (moved under /node-servers/:id/)
			inboundCfgH := inboundconfigHandler.NewHandler(inboundConfigSvc)

			// Outbound Configs (moved under /node-servers/:id/)
			outboundCfgH := outboundconfigHandler.NewHandler(outboundConfigSvc)

			// Inject config sync hooks so inbound/outbound changes trigger
			// full config sync to remote nodes.
			// Inbound changes also trigger mirror sync (mirrors replicate inbounds
			// and caddy/nginx proxy configs); outbound changes do NOT trigger
			// mirror sync.
			inboundSyncHook := &configSyncHookAdapter{syncSvc: configSyncSvc, triggerMirrorSync: true}
			outboundSyncHook := &configSyncHookAdapter{syncSvc: configSyncSvc, triggerMirrorSync: false}
			inboundCfgH.SetSyncHook(inboundSyncHook)
			outboundCfgH.SetSyncHook(outboundSyncHook)

			// NOTE: SyncData and traffic reset hooks are injected after
			// nodeServerSvcInst is created (see below).

			// Share Domain Configs
			shareDomainSvc := sharedomainconfigSvc.NewService(shareDomainRepo)
			shareDomainH := sharedomainconfigHandler.NewHandler(shareDomainSvc)
			shareDomainGroup := protected.Group("/share-domains")
			{
				shareDomainGroup.GET("", shareDomainH.List)
				shareDomainGroup.POST("", shareDomainH.Create)
				shareDomainGroup.PUT("/:id", shareDomainH.Update)
				shareDomainGroup.DELETE("/:id", shareDomainH.Delete)
				shareDomainGroup.DELETE("/all", shareDomainH.ClearAll)
				shareDomainGroup.POST("/seed", shareDomainH.SeedDefaults)
			}

			// CF IP Sources & Entries
			cfipSourceRepo := repository.NewCFIPSourceRepository(database)
			cfipEntryRepo := repository.NewCFIPEntryRepository(database)
			cfipSvcInst := cfipSvc.NewService(cfipSourceRepo, cfipEntryRepo)
			cfipSvcInst.StartScheduler(context.Background())
			cfipH := cfipHandler.NewHandler(cfipSvcInst)
			cfipGroup := protected.Group("/cfip")
			{
				cfipGroup.GET("/sources", cfipH.ListSources)
				cfipGroup.POST("/sources", cfipH.CreateSource)
				cfipGroup.PUT("/sources/:id", cfipH.UpdateSource)
				cfipGroup.DELETE("/sources/:id", cfipH.DeleteSource)
				cfipGroup.POST("/sources/:id/fetch", cfipH.FetchSource)
				cfipGroup.POST("/fetch-all", cfipH.FetchAll)
				cfipGroup.GET("/entries", cfipH.ListEntries)
				cfipGroup.POST("/entries", cfipH.AddEntry)
				cfipGroup.DELETE("/entries/:id", cfipH.DeleteEntry)
				cfipGroup.DELETE("/entries", cfipH.ClearEntries)
				cfipGroup.POST("/seed", cfipH.SeedDefaults)
			}

			// Services (moved under /node-servers/:id/services)
			// Geo Rules (moved under /node-servers/:id/geo-rules)

			// Subscriptions
			subsGroup := protected.Group("/subscriptions")
			{
				subsGroup.GET("", subH.List)
				subsGroup.POST("", subH.Create)
				subsGroup.GET("/:id", subH.GetByID)
				subsGroup.PUT("/:id", subH.Update)
				subsGroup.DELETE("/:id", subH.Delete)
				subsGroup.POST("/:id/rotate", subH.Rotate)
				subsGroup.POST("/:id/revoke", subH.Revoke)
				subsGroup.POST("/:id/restore", subH.Restore)
				subsGroup.POST("/:id/enable", subH.Enable)
				subsGroup.POST("/:id/reset-usage", subH.ResetUsage)
				subsGroup.GET("/:id/access-logs", subH.ListAccessLogs)
				subsGroup.GET("/:id/rotations", subH.ListRotations)
			}

			// Subscription Formats
			subfmtGroup := protected.Group("/subscription-formats")
			{
				subfmtGroup.GET("", subfmtH.List)
				subfmtGroup.POST("", subfmtH.Create)
				subfmtGroup.GET("/:id", subfmtH.GetByID)
				subfmtGroup.PUT("/:id", subfmtH.Update)
				subfmtGroup.DELETE("/:id", subfmtH.Delete)
				subfmtGroup.GET("/:id/rules", subfmtH.ListRules)
				subfmtGroup.POST("/:id/rules", subfmtH.CreateRule)
				subfmtGroup.PUT("/:id/rules/:rid", subfmtH.UpdateRule)
				subfmtGroup.DELETE("/:id/rules/:rid", subfmtH.DeleteRule)
				subfmtGroup.POST("/:id/rules/sort", subfmtH.SortRules)
			}

			// Subscription Endpoints
			subendGroup := protected.Group("/subscription-endpoints")
			{
				subendGroup.GET("", subendH.List)
				subendGroup.POST("", subendH.Create)
				subendGroup.GET("/:id", subendH.GetByID)
				subendGroup.PUT("/:id", subendH.Update)
				subendGroup.DELETE("/:id", subendH.Delete)
			}

			// Clients
			clientsGroup := protected.Group("/clients")
			{
				clientsGroup.GET("", clientH.ListClients)
				clientsGroup.POST("", clientH.CreateClient)
				clientsGroup.GET("/:id", clientH.GetClient)
				clientsGroup.PUT("/:id", clientH.UpdateClient)
				clientsGroup.DELETE("/:id", clientH.DeleteClient)
				clientsGroup.POST("/:id/suspend", clientH.SuspendClient)
				clientsGroup.POST("/:id/restore", clientH.RestoreClient)
				clientsGroup.POST("/:id/reset-usage", clientH.ResetUsage)
				clientsGroup.GET("/:id/orders", planH.ListClientOrders)
				clientsGroup.POST("/:id/renew", planH.Renew)
				clientsGroup.POST("/:id/upgrade", planH.Upgrade)
				clientsGroup.POST("/:id/downgrade", planH.Downgrade)
			}

			// Plan Templates (套餐模板)
			planTmplGroup := protected.Group("/plan-templates")
			{
				planTmplGroup.GET("", planH.ListTemplates)
				planTmplGroup.POST("", planH.CreateTemplate)
				planTmplGroup.GET("/:id", planH.GetTemplate)
				planTmplGroup.PUT("/:id", planH.UpdateTemplate)
				planTmplGroup.DELETE("/:id", planH.DeleteTemplate)
				planTmplGroup.PUT("/:id/node-groups", planH.SetTemplateNodeGroups)
				planTmplGroup.GET("/:id/accessible-nodes", planH.GetAccessibleNodes)
				planTmplGroup.GET("/:id/billing-node-options", planH.GetBillingNodeOptions)
				planTmplGroup.GET("/:id/variants", planH.ListVariants)
				planTmplGroup.POST("/:id/variants", planH.CreateVariant)
				// Account synchronization: manually trigger account creation for existing clients
				planTmplGroup.POST("/sync-all-accounts", planH.SyncAllAccounts)
				planTmplGroup.POST("/:id/sync-accounts", planH.SyncAccountsForTemplate)
			}

			// Plan Variants (套餐规格)
			planVariantGroup := protected.Group("/plan-variants")
			{
				planVariantGroup.PUT("/:id", planH.UpdateVariant)
				planVariantGroup.DELETE("/:id", planH.DeleteVariant)
			}

			// Traffic Packs (独立流量叠加包，不绑定模板，适用全部套餐)
			trafficPackGroup := protected.Group("/traffic-packs")
			{
				trafficPackGroup.GET("", planH.ListTrafficPacks)
				trafficPackGroup.POST("", planH.CreateTrafficPack)
				trafficPackGroup.PUT("/:id", planH.UpdateTrafficPack)
				trafficPackGroup.DELETE("/:id", planH.DeleteTrafficPack)
			}

			// Charge Sites (充值站点管理)
			chargeAdminH := chargeHandler.NewHandler(
				repository.NewChargeSiteRepository(database),
				repository.NewChargeOrderRepository(database),
				repository.NewChargeCardRepository(database),
				repository.NewClientBalanceRepository(database),
				repository.NewBalanceTransactionRepository(database),
				clientRepo,
				userRepo, // v3.3: 用于查询代理员信息
				database,
			)
			// 注入 PlanRefunder — 退款时回退套餐时间 + 移除节点账号
			chargeAdminH.SetPlanRefunder(commerceSvcInst)
			// Charge sites — admin only (includes rotate-secret / clear-prev-secret
			// which expose/rotate signing secrets).
			chargeSiteGroup := protected.Group("/charge-sites").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				chargeSiteGroup.GET("", chargeAdminH.ListSites)
				chargeSiteGroup.GET("/:id", chargeAdminH.GetSite)
				chargeSiteGroup.POST("", chargeAdminH.CreateSite)
				chargeSiteGroup.PUT("/:id", chargeAdminH.UpdateSite)
				chargeSiteGroup.DELETE("/:id", chargeAdminH.DeleteSite)
				chargeSiteGroup.POST("/:id/generate-verify-file", chargeAdminH.GenerateVerifyFile)
				chargeSiteGroup.POST("/:id/generate-verify", chargeAdminH.GenerateVerifyFile) // 文档路径别名
				chargeSiteGroup.POST("/:id/verify-domain-http", chargeAdminH.VerifyDomainHTTP)
				chargeSiteGroup.POST("/:id/verify-domain", chargeAdminH.VerifyDomainHTTP) // 文档路径别名
				chargeSiteGroup.POST("/:id/verify-domain-dns", chargeAdminH.VerifyDomainDNS)
				chargeSiteGroup.POST("/:id/verify-dns", chargeAdminH.VerifyDomainDNS) // 文档路径别名
				chargeSiteGroup.GET("/:id/integration-doc", chargeAdminH.GetSiteIntegrationDoc)
				chargeSiteGroup.POST("/:id/rotate-secret", chargeAdminH.RotateSecret)
				chargeSiteGroup.POST("/:id/clear-prev-secret", chargeAdminH.ClearPrevSecret)
			}
			// Charge orders (incl. refund) — admin only (money-affecting).
			chargeOrderGroup := protected.Group("/charge-orders").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				chargeOrderGroup.GET("", chargeAdminH.ListOrders)
				chargeOrderGroup.GET("/:id", chargeAdminH.GetOrder)
				chargeOrderGroup.POST("/:id/refund", chargeAdminH.RefundOrder)
			}
			// Charge cards — admin only (includes CSV export of card data).
			chargeCardGroup := protected.Group("/charge-cards").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				chargeCardGroup.GET("", chargeAdminH.ListCards)
				chargeCardGroup.POST("", chargeAdminH.CreateCard)
				chargeCardGroup.POST("/batch", chargeAdminH.BatchCreateCards)
				chargeCardGroup.GET("/export-csv", chargeAdminH.ExportCardsCSV) // v3.3: CSV 导出（放在 /:id 之前避免路由冲突）
				chargeCardGroup.GET("/:id/usages", chargeAdminH.GetCardUsages)  // v3.3: 使用记录
				chargeCardGroup.DELETE("/:id", chargeAdminH.DeleteCard)
			}
			clientBalanceGroup := protected.Group("/client-balances")
			{
				clientBalanceGroup.GET("", chargeAdminH.ListBalances)
				clientBalanceGroup.GET("/transactions", chargeAdminH.ListBalanceTransactionsAdmin)
			}

			// Callback events (v3.1: merged logs into events) & retry management
			callbackEventGroup := protected.Group("/charge-callback-events")
			{
				callbackEventGroup.GET("", chargeAdminH.ListCallbackEvents)
			}
			callbackRetryGroup := protected.Group("/charge-callback-retries")
			{
				callbackRetryGroup.GET("", chargeAdminH.ListCallbackRetries)
				callbackRetryGroup.POST("/:id/retry", chargeAdminH.RetryCallback)
			}

			// P1-P3: Admin routes for extended modules
			// Referral withdrawals (approve/reject payouts) — admin only.
			referralWithdrawalGroup := protected.Group("/referral-withdrawals").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				referralWithdrawalGroup.GET("", clientExtH.AdminListWithdrawals)
				referralWithdrawalGroup.POST("/:id/approve", clientExtH.AdminApproveWithdrawal)
				referralWithdrawalGroup.POST("/:id/reject", clientExtH.AdminRejectWithdrawal)
			}
			referralAdminGroup := protected.Group("/referral")
			{
				referralAdminGroup.GET("/stats", clientExtH.AdminReferralStats)
				referralAdminGroup.GET("/records", clientExtH.AdminListReferralRecords)
				referralAdminGroup.GET("/config", clientExtH.AdminGetReferralConfig)
				referralAdminGroup.PUT("/config", clientExtH.AdminSetReferralConfig)
			}
			ipWhitelistAdminGroup := protected.Group("/ip-whitelists")
			{
				ipWhitelistAdminGroup.GET("", clientExtH.AdminListAllIPWhitelists)
			}
			oauthBindingAdminGroup := protected.Group("/client-oauth-bindings")
			{
				oauthBindingAdminGroup.GET("", clientExtH.AdminListClientOAuthBindings)
			}
			ticketAdminGroup := protected.Group("/tickets")
			{
				ticketAdminGroup.GET("", clientExtH.AdminListTickets)
				ticketAdminGroup.GET("/:id", clientExtH.AdminGetTicket)
				ticketAdminGroup.POST("/:id/reply", clientExtH.AdminReplyTicket)
				ticketAdminGroup.POST("/:id/close", clientExtH.AdminCloseTicket)
				ticketAdminGroup.POST("/:id/assign", clientExtH.AdminAssignTicket)
			}
			giftAdminGroup := protected.Group("/client-gifts")
			{
				giftAdminGroup.GET("", clientExtH.AdminListGifts)
				giftAdminGroup.POST("", clientExtH.AdminCreateGift)
				giftAdminGroup.POST("/batch", clientExtH.AdminCreateGiftBatch)
				giftAdminGroup.GET("/search-clients", clientExtH.AdminSearchClients)
			}
			articleAdminGroup := protected.Group("/articles")
			{
				articleAdminGroup.GET("", clientExtH.AdminListArticles)
				articleAdminGroup.GET("/:id", clientExtH.AdminGetArticle)
				articleAdminGroup.POST("", clientExtH.AdminCreateArticle)
				articleAdminGroup.PUT("/:id", clientExtH.AdminUpdateArticle)
				articleAdminGroup.DELETE("/:id", clientExtH.AdminDeleteArticle)
			}
			sharedAccountAdminGroup := protected.Group("/shared-accounts")
			{
				sharedAccountAdminGroup.GET("", clientExtH.AdminListSharedAccounts)
				sharedAccountAdminGroup.GET("/:id", clientExtH.AdminGetSharedAccount)
				sharedAccountAdminGroup.POST("", clientExtH.AdminCreateSharedAccount)
				sharedAccountAdminGroup.PUT("/:id", clientExtH.AdminUpdateSharedAccount)
				sharedAccountAdminGroup.DELETE("/:id", clientExtH.AdminDeleteSharedAccount)
				sharedAccountAdminGroup.GET("/:id/repair-logs", clientExtH.AdminRepairLogs)
			}

			// Inbounds with server name (for member management UI)
			protected.GET("/inbounds-with-server", planH.ListAllInboundsWithServer)

			// Node Groups (节点分组)
			nodeGroupRoute := protected.Group("/node-groups")
			{
				nodeGroupRoute.GET("", planH.ListNodeGroups)
				nodeGroupRoute.POST("", planH.CreateNodeGroup)
				nodeGroupRoute.PUT("/:id", planH.UpdateNodeGroup)
				nodeGroupRoute.DELETE("/:id", planH.DeleteNodeGroup)
				nodeGroupRoute.GET("/:id/members", planH.ListGroupMembers)
				nodeGroupRoute.POST("/:id/members", planH.AddGroupMember)
				nodeGroupRoute.DELETE("/:id/members/:mid", planH.RemoveGroupMember)
			}

			// Promotions (促销优惠)
			promoGroup := protected.Group("/promotions")
			{
				promoGroup.GET("", planH.ListPromotions)
				promoGroup.POST("", planH.CreatePromotion)
				promoGroup.PUT("/:id", planH.UpdatePromotion)
				promoGroup.DELETE("/:id", planH.DeletePromotion)
				promoGroup.POST("/validate", planH.ValidatePromotion)
			}

			// Mihomo Config
			mihomoGroup := protected.Group("/mihomo")
			{
				mihomoGroup.GET("/configs", mihomoH.List)
				mihomoGroup.POST("/configs", mihomoH.Create)
				mihomoGroup.GET("/configs/:id", mihomoH.GetByID)
				mihomoGroup.PUT("/configs/:id", mihomoH.Update)
				mihomoGroup.DELETE("/configs/:id", mihomoH.Delete)
				mihomoGroup.GET("/configs/:id/proxy-groups", mihomoH.ListProxyGroups)
				mihomoGroup.POST("/configs/:id/proxy-groups", mihomoH.CreateProxyGroup)
				mihomoGroup.DELETE("/proxy-groups/:id", mihomoH.DeleteProxyGroup)
				mihomoGroup.GET("/configs/:id/rules", mihomoH.ListRules)
				mihomoGroup.POST("/configs/:id/rules", mihomoH.CreateRule)
				mihomoGroup.DELETE("/rules/:id", mihomoH.DeleteRule)
				mihomoGroup.GET("/configs/:id/rule-providers", mihomoH.ListRuleProviders)
				mihomoGroup.POST("/configs/:id/rule-providers", mihomoH.CreateRuleProvider)
				mihomoGroup.DELETE("/rule-providers/:id", mihomoH.DeleteRuleProvider)
				mihomoGroup.GET("/configs/:id/yaml", mihomoH.GenerateYAML)
			}

			// External Nodes
			extNodeH := extNodeHandler.NewExternalNodeHandler(extSrcRepo, extNodeRepo)
			extNodeH.RegisterRoutes(protected)

			// Node Servers (multi-region architecture)
			nodeServerRepo := repository.NewNodeServerRepository(database)
			revocationRepo := repository.NewCertRevocationRepository(database)
			bootstrapTokenMgr := agent.NewBootstrapTokenManager(nodeServerRepo, caService)
			nodeServerSvcInst := nodeServerSvc.NewService(nodeServerRepo, bootstrapTokenMgr, caService)

			// Wire up CRL and CertDistributor if CA is available
			if caService != nil {
				certRepoForNodeServer := repository.NewCertificateRepository(database)
				// Use the shared NodeServerManager from main.go (same instance that runs
				// the gRPC server and receives Agent connections). This ensures that
				// HTTP API status queries and commands use the same connection map.
				nodeMgrForCRL := sharedNodeMgr
				crlInst := agent.NewCertRevocationList(revocationRepo, nodeServerRepo, nodeMgrForCRL, caService)
				certDistribInst := agent.NewCertDistributor(certRepoForNodeServer, nodeServerRepo, nodeMgrForCRL)
				nodeServerSvcInst.WithCRL(crlInst).WithCertDistributor(certDistribInst)

				// Wire cert update callback: when a certificate is updated/renewed
				// in the certificate center, automatically distribute to all nodes.
				certH.OnCertUpdated(func(ctx context.Context, certID int64) {
					if err := nodeServerSvcInst.DistributeCertificateToAll(ctx, certID); err != nil {
						log.Printf("[router] auto-distribute cert %d after update: %v", certID, err)
					} else {
						log.Printf("[router] auto-distributed cert %d to all associated nodes", certID)
					}
				})

				// Inject LocalAgent — use the shared instance from main.go if available.
				// This ensures both the gRPC server and HTTP API use the same LocalAgent
				// with all dependencies (ProxyManager, BinaryInstaller) properly injected,
				// instead of creating a second incomplete instance.
				binaryInstaller := binariesSvc.NewInstaller(dataDir, binaryResolver, systemSettingSvc)
				binaryInstaller.SetProcessController(processMgr) // Enable stop/start during binary updates
				var localAgent *agent.LocalAgent
				if sharedLocalAgent != nil {
					localAgent = sharedLocalAgent
				} else {
					// Fallback: create a new LocalAgent (backward compatible)
					localAgent = agent.NewLocalAgent(dataDir, processMgr, svcRepo)
					localAgent = localAgent.WithProxyManager(agent.NewProxyManagerAdapter(proxyMgr))
					localAgent = localAgent.WithBinaryInstaller(agent.NewBinaryInstallerAdapter(binaryInstaller))
				}
				nodeMgrForCRL.SetLocalAgent(localAgent)
				// Also update the shared NodeServerManager's LocalAgent so the gRPC
				// heartbeat handler (used by remote Agents) has the complete instance
				// with ProxyManager and BinaryInstaller, not the partial one from main.go.
				if sharedNodeMgr != nil {
					sharedNodeMgr.SetLocalAgent(localAgent)
				}
				nodeMgrForCRL.SetSettingStore(systemSettingSvc)

				// Set binary lister for node server service (used for ListReleases API)
				// Wrap binaries.Installer to match nodeserver.BinaryLister interface
				nodeServerSvcInst.SetBinaryLister(&binaryListerAdapter{installer: binaryInstaller})

				// Phase 3: Node Types & Chain Proxy wiring
				inboundRepoForNodeServer := repository.NewInboundConfigRepository(database)
				caddySiteRepoForMirror := repository.NewCaddySiteRepository(database)
				nginxSiteRepoForMirror := repository.NewNginxSiteRepository(database)
				nodeMgrForPhase3 := nodeMgrForCRL // reuse the same manager

				failoverMgr := agent.NewFailoverManager(nodeServerRepo, nodeMgrForPhase3)
			mirrorSyncMgr := agent.NewMirrorSyncManager(nodeServerRepo, inboundRepoForNodeServer, nodeMgrForPhase3).
				SetCaddySiteRepository(caddySiteRepoForMirror).
				SetNginxSiteRepository(nginxSiteRepoForMirror).
				SetSettingRepository(systemSettingRepo)
				chainGen := agent.NewChainConfigGenerator(nodeServerRepo)
				chainHealthRegistry := agent.NewChainHealthMonitorRegistry(nodeMgrForPhase3, nodeServerRepo)
				billingFilter := agent.NewBillingFilter(nodeServerRepo)

				nodeServerSvcInst.
					WithInboundRepo(inboundRepoForNodeServer).
					WithNodeManager(nodeMgrForPhase3).
					WithFailoverManager(failoverMgr).
					WithMirrorSync(mirrorSyncMgr).
					WithChainGenerator(chainGen).
					WithChainHealthRegistry(chainHealthRegistry).
					WithBillingFilter(billingFilter)

				// Phase 5: Dashboard & Enhancements wiring
				multiHealthChecker := healthcheck.NewMultiServerHealthChecker(nodeMgrForPhase3, nodeServerRepo)
				geoIPResolver := subRegion.NewSimpleGeoIPResolver()
				regionSelector := subRegion.NewRegionSelector(geoIPResolver, nodeServerRepo)

				nodeServerSvcInst.
					WithHealthChecker(multiHealthChecker).
					WithRegionSelector(regionSelector)

			// Phase 6: Advanced Features wiring
			metricsCollector := agent.NewMetricsCollector(database, nodeServerRepo, nodeMgrForPhase3)
			// 本地节点（面板自身）主机指标由 StatsService 直接采集，不走独立 agent 的 gRPC 接口
			statsSvc := systemSvc.NewStatsService(version)
			metricsCollector.SetLocalStatsProvider(statsSvc.CollectServerMetric)

			// 注入 xray 核心用户流量采集（本地 + 远程），用于实时速率图表
			// 本地：直连 xray gRPC Stats API（reset_=false，累计值）
			// 远程：通过 coredata.Provider → Agent RPC 查询
			// 仅统计 user>>> 条目，自动排除 inbound>>>api>>> 等 API 流量
			localXrayClient := coredata.NewXrayClient(coredata.ResolveStatsAddr(context.Background(), coredata.StatsAddrDeps{
				SettingGetter: func(ctx context.Context, key string) (string, error) {
					return systemSettingSvc.GetByKey(ctx, key)
				},
				APIListenGetter: func(ctx context.Context) (string, error) {
					full, err := coreConfigSvc.GetFull(ctx, coredata.LocalServerID, "xray")
					if err != nil {
						return "", err
					}
					apiRaw, ok := full["api"]
					if !ok {
						return "", fmt.Errorf("no api section")
					}
					apiMap, ok := apiRaw.(map[string]interface{})
					if !ok {
						return "", fmt.Errorf("api not map")
					}
					listen, ok := apiMap["listen"].(string)
					if !ok || listen == "" {
						return "", fmt.Errorf("no listen")
					}
					return listen, nil
				},
			}))
			metricsCollector.SetLocalTrafficProvider(func() (int64, int64) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				resp, err := localXrayClient.RawStats(ctx, false)
				if err != nil {
					return 0, 0
				}
				sr, err := coredata.ParseStatsResponse(resp)
				if err != nil {
					return 0, 0
				}
				var up, down int64
				for _, u := range sr.Users {
					up += u.Uplink
					down += u.Downlink
				}
				return up, down
			})
			metricsCollector.SetRemoteTrafficProvider(func(ctx context.Context, serverID int64) (int64, int64) {
				// 通过 coredata.Provider 查询远程节点 xray 用户流量
				p := coredata.NewProvider(serverID, coredata.Options{
					IsLocal: nodeMgrForPhase3.IsLocalServer,
					Remote:  nodeServerSvcInst,
				})
				users, err := p.GetUserTraffic(ctx, serverID)
				if err != nil {
					return 0, 0
				}
				var up, down int64
				for _, u := range users {
					up += u.Uplink
					down += u.Downlink
				}
				return up, down
			})

			go metricsCollector.RunLoop(context.Background())

			// Periodic speed test: run once shortly after startup, then on an
			// interval driven by SPEEDTEST_INTERVAL_MIN (default 5 minutes).
			speedTestMgr := agent.NewSpeedTestManager(nodeServerRepo, nodeMgrForPhase3)
			if iv := os.Getenv("SPEEDTEST_INTERVAL_MIN"); iv != "" {
				if mins, err := strconv.Atoi(iv); err == nil && mins > 0 {
					speedTestMgr = speedTestMgr.WithInterval(time.Duration(mins) * time.Minute)
				}
			}
			go speedTestMgr.RunLoop(context.Background())
				loadBalancer := agent.NewLoadBalancer(nodeServerRepo, nodeMgrForPhase3).WithSpeedTest(speedTestMgr)
				agentUpdater := agent.NewAgentUpdater(nodeServerRepo, nodeMgrForPhase3).WithDataDir(dataDir)

				nodeMgrForPhase3.SetMetricsCollector(metricsCollector)
				nodeMgrForPhase3.SetAgentUpdater(agentUpdater)
				nodeMgrForPhase3.SetRateLimitSyncer(rateLimitSvcInst)

				// Wire DataSyncBuilder: reads panel DB tables → builds SyncDataRequest → sends to Agent
				// Also generates final runtime config via PrepareFullConfig so Agent
				// doesn't need to re-implement the config preparation pipeline.
				dataSyncProvider := configsyncSvc.NewDataSyncBuilder(
					configsyncSvc.NewRepositoryDataProvider(
						coreConfigRepo,
						inboundConfigRepo,
						outboundConfigRepo,
						warpRepo,
						shareDomainRepo,
					).WithCertificateProvider(certRepoForNodeServer, nodeServerRepo).WithProxySiteProvider(proxySiteRepo, caddySiteRepo, nginxSiteRepo).WithSystemSettingProvider(systemSettingRepo),
				).WithConfigProviders(coreConfigSvc, inboundConfigSvc, outboundConfigSvc).WithLogModeProvider(logManageSvc)
				nodeMgrForPhase3.SetDataSyncProvider(dataSyncProvider)
				nodeMgrForPhase3.SetProxyCertCollector(dataSyncProvider)

				// Set proxy settings provider for per-node nginx/caddy settings sync
				nodeMgrForPhase3.SetProxySettingsProvider(&proxySettingsProvider{
					settingStore: systemSettingRepo,
				})

				nodeServerSvcInst.
					WithMetricsCollector(metricsCollector).
					WithSpeedTestManager(speedTestMgr).
					WithLoadBalancer(loadBalancer).
					WithAgentUpdater(agentUpdater)

				// Create and wire the traffic quota service for monthly traffic monitoring
				trafficQuotaSvc := nodeServerSvc.NewNodeTrafficQuotaService(
					database,
					nodeServerRepo,
					nodeMgrForPhase3,
				)
			nodeServerSvcInst.WithTrafficQuotaService(trafficQuotaSvc)
			nodeMgrForPhase3.SetTrafficQuotaProcessor(trafficQuotaSvc)

			// Wire node traffic quota service into client auth for GET /client-auth/traffic-history
			clientauthSvcInst.SetTrafficQuotaSvc(trafficQuotaSvc)

			// Start the traffic quota checker in the background
			go trafficQuotaSvc.Start(context.Background())

			// Wire trafficsync to also accumulate node-level monthly traffic
			trafficSyncSvc.SetNodeAccumulator(trafficQuotaSvc)

			// Start traffic event consumer for remote node traffic (Agent → Panel via heartbeat stream)
			// This consumes TrafficEvent from the NodeServerManager's onTraffic channel
			// and persists remote node traffic deltas to the database.
			if sharedNodeMgr != nil {
				trafficEventConsumer := agent.NewTrafficEventConsumer(
					sharedNodeMgr, nil, inboundConfigSvc,
				).WithServerRepo(nodeServerRepo).WithNodeAccumulator(trafficQuotaSvc)
				go trafficEventConsumer.Start(context.Background())
			}

				// Supplementary repos wiring
				auditLogRepo := repository.NewNodeAuditLogRepository(database)
				tagRepo := repository.NewServerTagRepository(database)
				distQueueRepo := repository.NewCertDistributionQueueRepository(database)
				crlRepoForSvc := revocationRepo         // reuse the already-created revocation repo
				certRepoForSvc := certRepoForNodeServer // reuse the already-created cert repo

				nodeServerSvcInst.
					WithAuditLogRepo(auditLogRepo).
					WithTagRepo(tagRepo).
					WithDistQueueRepo(distQueueRepo).
					WithCertRepo(certRepoForSvc).
					WithCRLRepo(crlRepoForSvc).
					WithDB(database)
			}

			nodeServerH := nodeServerHandler.NewHandler(nodeServerSvcInst)
			nodeServerH.SetCoreConfigService(coreConfigSvc)
			nodeServerH.SetInboundConfigService(inboundConfigSvc)
			nodeServerH.SetOutboundConfigService(outboundConfigSvc)
			nodeServerH.SetProxyHandler(proxyH)
			nodeServerH.SetGeoRuleService(geoRuleSvcInst)

			// Wire up NodeOperator so proxy/caddy/nginx handlers can route
			// operations (start/stop/restart/reload, config sync, status)
			// to remote nodes via gRPC.
			proxyH.SetNodeOperator(nodeServerSvcInst)
			caddyH.SetNodeOperator(nodeServerSvcInst)
			nginxH.SetNodeOperator(nodeServerSvcInst)
			processH.SetNodeOperator(nodeServerSvcInst)

			// Inject SyncData hook so traffic reset also pushes inbound_configs
			// table to remote Agent's local SQLite DB.
			syncDataHook := &syncDataHookAdapter{nodeServerSvc: nodeServerSvcInst}
			inboundCfgH.SetSyncDataHook(syncDataHook)

			// Wire nodeServerSvc into the configSyncHookAdapter so that
			// inbound config changes also trigger mirror node sync.
			inboundSyncHook.nodeServerSvc = nodeServerSvcInst
			outboundSyncHook.nodeServerSvc = nodeServerSvcInst

			// Inject traffic reset hook so subscription/client reset propagates
			// to inbound configs and remote Agent nodes.
			trafficHook := &trafficResetHookAdapter{inboundSvc: inboundConfigSvc, syncSvc: configSyncSvc, nodeServerSvc: nodeServerSvcInst}
			subSvc.SetTrafficResetHook(trafficHook)
			clientSvcInst.SetTrafficResetHook(trafficHook)

			// Inject the user syncer so the quota/expiry enforcer can block users at
			// the proxy cores, and start the periodic enforcement scan.
			if sharedUserSyncer != nil {
				subSvc.SetUserSyncer(sharedUserSyncer)
				subSvc.StartEnforcer(context.Background(), 5*time.Minute)
			}
			// Node servers: administrative surface (delete, certificate
			// distribution, core control, data sync). Gated at admin; the few
			// most destructive routes are additionally restricted to super_admin.
			nodeServersGroup := protected.Group("/node-servers")
			nodeServersGroup.Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				nodeServersGroup.GET("", nodeServerH.List)
				nodeServersGroup.POST("", nodeServerH.Create)
				nodeServersGroup.GET("/ca-cert", nodeServerH.GetCACert)
				nodeServersGroup.GET("/ca-status", nodeServerH.GetCAStatus)
				nodeServersGroup.POST("/distribute-cert", nodeServerH.DistributeCertificateToAll)
				nodeServersGroup.POST("/migrate-cert", nodeServerH.MigrateCertificate)
				nodeServersGroup.GET("/:id", nodeServerH.GetByID)
				nodeServersGroup.PUT("/:id", nodeServerH.Update)
				nodeServersGroup.DELETE("/:id", nodeServerH.Delete)
				nodeServersGroup.POST("/:id/bootstrap-token", nodeServerH.GenerateBootstrapToken)
				nodeServersGroup.POST("/:id/revoke-cert", nodeServerH.RevokeAgentCertificate)
				nodeServersGroup.POST("/:id/distribute-cert", nodeServerH.DistributeCertificate)
				// Phase 3: Node Types & Chain Proxy
				nodeServersGroup.GET("/:id/failover-status", nodeServerH.GetFailoverStatus)
				nodeServersGroup.GET("/:id/recover-mode", nodeServerH.GetServerRecoverMode)
				nodeServersGroup.PUT("/:id/recover-mode", nodeServerH.SetServerRecoverMode)
				nodeServersGroup.POST("/failover-check", nodeServerH.TriggerFailoverCheck)
				nodeServersGroup.POST("/:id/sync-mirror", nodeServerH.SyncMirror)
				nodeServersGroup.GET("/:id/mirror-status", nodeServerH.GetMirrorStatus)
				nodeServersGroup.GET("/:id/chain-config", nodeServerH.GenerateChainConfig)
				nodeServersGroup.GET("/:id/chain-exit-config", nodeServerH.GenerateChainExitConfig)
				nodeServersGroup.GET("/:id/chain-health", nodeServerH.GetChainHealthStatus)
				nodeServersGroup.GET("/billable", nodeServerH.ListBillableServers)
				nodeServersGroup.POST("/validate-chain-config", nodeServerH.ValidateChainConfig)
				// Phase 4: User Sync & Traffic Aggregation
				nodeServersGroup.POST("/users/sync", nodeServerH.SyncUser)
				nodeServersGroup.DELETE("/users/:client_id", nodeServerH.SyncUserRemoval)
				nodeServersGroup.GET("/users/retry-queue", nodeServerH.GetRetryQueueStatus)
				nodeServersGroup.POST("/users/retry-queue/process", nodeServerH.ProcessRetryQueue)
				nodeServersGroup.GET("/traffic/stats", nodeServerH.GetTrafficStats)
				// Phase 5: Dashboard & Enhancements
				nodeServersGroup.GET("/dashboard", nodeServerH.GetDashboardStatus)
				nodeServersGroup.POST("/recommend-region", nodeServerH.RecommendRegion)
				nodeServersGroup.GET("/:id/install-script", nodeServerH.GetAgentInstallScript)

				// Phase 6: Advanced Features
				nodeServersGroup.GET("/:id/metrics-history", nodeServerH.GetMetricsHistory)
				nodeServersGroup.GET("/metrics/aggregated", nodeServerH.GetAggregatedMetrics)
				nodeServersGroup.POST("/:id/speed-test", nodeServerH.TestServerSpeed)
				nodeServersGroup.POST("/speed-test-all", nodeServerH.TestAllServersSpeed)
				nodeServersGroup.GET("/speed-test-results", nodeServerH.GetSpeedTestResults)
				nodeServersGroup.POST("/select-nodes", nodeServerH.SelectNodesForSubscription)
				nodeServersGroup.POST("/:id/update-agent", nodeServerH.UpdateAgent)
				nodeServersGroup.POST("/update-all-agents", nodeServerH.UpdateAllAgents)
				nodeServersGroup.GET("/pending-agent-updates", nodeServerH.GetPendingAgentUpdates)

				// Supplementary: Server Management
				nodeServersGroup.GET("/:id/status", nodeServerH.GetServerStatus)
				nodeServersGroup.GET("/:id/online-users", nodeH.GetOnlineUsers)
				nodeServersGroup.GET("/online-summary", nodeH.GetOnlineSummary)
				nodeServersGroup.GET("/online-users", nodeH.GetAllOnlineUsers)
				// Stop/control the core process — most destructive, super_admin only.
				nodeServersGroup.Group("").Use(middleware.RequireRole(domainuser.RoleSuperAdmin)).POST("/:id/control", nodeServerH.ControlCore)
				nodeServersGroup.POST("/:id/sync", nodeServerH.SyncServerConfig)
				nodeServersGroup.GET("/:id/bootstrap-cmd", nodeServerH.GetBootstrapCommand)
				nodeServersGroup.PUT("/:id/node-type", nodeServerH.UpdateNodeType)
				nodeServersGroup.POST("/:id/failover", nodeServerH.TriggerServerFailover)
				nodeServersGroup.GET("/:id/certificates", nodeServerH.ListServerCertificates)
				nodeServersGroup.POST("/:id/certificates/:cid", nodeServerH.AssociateCertificate)
				nodeServersGroup.DELETE("/:id/certificates/:cid", nodeServerH.DissociateCertificate)

				// Rate Limit Config
				nodeServersGroup.GET("/:id/rate-limit", rateLimitH.GetConfig)
				nodeServersGroup.PUT("/:id/rate-limit", rateLimitH.UpdateConfig)
				nodeServersGroup.GET("/:id/rate-limit/status", rateLimitH.GetStatus)

				// Supplementary: Agent Degradation & Reconnect
				nodeServersGroup.GET("/:id/degradation-status", nodeServerH.GetDegradationStatus)
				nodeServersGroup.POST("/:id/force-reconnect", nodeServerH.ForceReconnect)

				// Binary & Proxy Installation (local + remote via Agent)
				nodeServersGroup.POST("/:id/binaries/install", nodeServerH.InstallBinary)
				nodeServersGroup.GET("/:id/binaries", nodeServerH.ListBinaries)
				nodeServersGroup.GET("/:id/binaries/releases", nodeServerH.ListReleases)
				nodeServersGroup.GET("/:id/proxy-status", nodeServerH.GetRemoteProxyStatus)

				// Binary Commands (key generation, interface listing)
				nodeServersGroup.POST("/:id/binaries/x25519-key", nodeServerH.GenerateX25519Key)
				nodeServersGroup.POST("/:id/binaries/mldsa65-key", nodeServerH.GenerateMldsa65Key)
				nodeServersGroup.POST("/:id/binaries/wg-key", nodeServerH.GenerateWgKey)
				nodeServersGroup.POST("/:id/binaries/ech-generate", nodeServerH.GenerateEchKeys)
				nodeServersGroup.POST("/:id/binaries/vlessenc", nodeServerH.GenerateVlessEnc)
				nodeServersGroup.GET("/:id/binaries/interfaces", nodeServerH.ListInterfaces)

				// Supplementary: Node Traffic Quota
				nodeServersGroup.GET("/:id/traffic-usage", nodeServerH.GetTrafficUsage)
				nodeServersGroup.GET("/:id/traffic-history", nodeServerH.GetTrafficHistory)
				nodeServersGroup.POST("/:id/traffic-resume", nodeServerH.ResumeTrafficSuspended)

				// Supplementary: Node Tags
				nodeServersGroup.GET("/:id/tags", nodeServerH.ListServerTags)
				nodeServersGroup.POST("/:id/tags", nodeServerH.AddServerTag)
				nodeServersGroup.DELETE("/:id/tags/:tagKey", nodeServerH.RemoveServerTag)

				// Node-server-scoped Protocol Config Management
				// These routes allow managing protocol config per node server.
				// GET operations read from panel DB; Save operations sync to target node.
				nodeServersGroup.GET("/:id/protocols", nodeServerH.ListProtocols)
				nodeServersGroup.GET("/:id/protocols/:coreType", nodeServerH.GetProtocol)
				nodeServersGroup.PUT("/:id/protocols/:coreType", nodeServerH.SaveProtocol)
				nodeServersGroup.GET("/:id/protocols/:coreType/section/:section", nodeServerH.GetProtocolSection)
				nodeServersGroup.PUT("/:id/protocols/:coreType/section/:section", nodeServerH.SaveProtocolSection)
				nodeServersGroup.POST("/:id/protocols/:coreType/validate", nodeServerH.ValidateProtocol)
				nodeServersGroup.GET("/:id/protocols/:coreType/config-file", nodeServerH.GetConfigFile)

				// Node-server-scoped Proxy Management
				// These routes allow managing proxy (Caddy/Nginx) per node server.
				nodeServersGroup.GET("/:id/proxy/status", nodeServerH.GetProxyStatus)
				nodeServersGroup.POST("/:id/proxy/control", nodeServerH.ControlProxy)
				nodeServersGroup.POST("/:id/proxy/sync", nodeServerH.SyncProxyConfig)

				// Node-server-scoped Proxy Site Management (Caddy/Nginx)
				// Proxy sites, SNI, L4, log settings are all per-node now.
				nodeServersGroup.GET("/:id/proxy/sites", proxyH.ListSites)
				nodeServersGroup.POST("/:id/proxy/sites", proxyH.CreateSite)
				nodeServersGroup.PUT("/:id/proxy/sites/:siteId", proxyH.UpdateSite)
				nodeServersGroup.DELETE("/:id/proxy/sites/:siteId", proxyH.DeleteSite)
				nodeServersGroup.POST("/:id/proxy/generate-config", proxyH.GenerateConfig)
				nodeServersGroup.GET("/:id/proxy/config", proxyH.GetConfig)
				nodeServersGroup.PUT("/:id/proxy/config", proxyH.UpdateConfig)
				nodeServersGroup.GET("/:id/proxy/sni-settings", proxyH.GetSNISettings)
				nodeServersGroup.POST("/:id/proxy/sni-settings", proxyH.SaveSNISettings)
				nodeServersGroup.POST("/:id/proxy/batch-update-ports", proxyH.BatchUpdatePorts)
				nodeServersGroup.DELETE("/:id/proxy/sni-rules", proxyH.DeleteAllSNI)
				nodeServersGroup.POST("/:id/proxy/sni-rules/toggle", proxyH.ToggleSNI)
				nodeServersGroup.GET("/:id/proxy/l4-settings", proxyH.GetL4Settings)
				nodeServersGroup.POST("/:id/proxy/l4-settings", proxyH.SaveL4Settings)
				nodeServersGroup.DELETE("/:id/proxy/l4-rules", proxyH.DeleteAllL4)
				nodeServersGroup.POST("/:id/proxy/guardian", proxyH.UpdateGuardian)
				nodeServersGroup.GET("/:id/proxy/log-settings", proxyH.GetLogSettings)
				nodeServersGroup.POST("/:id/proxy/log-settings", proxyH.SaveLogSettings)
				nodeServersGroup.GET("/:id/proxy/nginx-settings", proxyH.GetNginxSettings)
				nodeServersGroup.POST("/:id/proxy/nginx-settings", proxyH.SaveNginxSettings)
				nodeServersGroup.POST("/:id/proxy/install", nodeServerH.InstallProxy)
				nodeServersGroup.GET("/:id/proxy/releases", proxyH.ListReleases)
				nodeServersGroup.POST("/:id/proxy/download", proxyH.DownloadVersion)
				nodeServersGroup.POST("/:id/proxy/start", proxyH.Start)
				nodeServersGroup.POST("/:id/proxy/stop", proxyH.Stop)
				nodeServersGroup.POST("/:id/proxy/restart", proxyH.Restart)
				nodeServersGroup.POST("/:id/proxy/reload", proxyH.Reload)

				// Caddy site management (per node)
				nodeServersGroup.GET("/:id/caddy/sites", caddyH.ListSites)
				nodeServersGroup.POST("/:id/caddy/sites", caddyH.CreateSite)
				nodeServersGroup.PUT("/:id/caddy/sites/:siteId", caddyH.UpdateSite)
				nodeServersGroup.DELETE("/:id/caddy/sites/:siteId", caddyH.DeleteSite)
				nodeServersGroup.POST("/:id/caddy/generate-config", caddyH.GenerateConfig)
				nodeServersGroup.POST("/:id/caddy/save-and-apply", caddyH.SaveAndApply)
				nodeServersGroup.GET("/:id/caddy/sni-settings", caddyH.GetSNISettings)
				nodeServersGroup.POST("/:id/caddy/sni-settings", caddyH.SaveSNISettings)
				nodeServersGroup.POST("/:id/caddy/sni-rules/toggle", caddyH.ToggleSNI)
				nodeServersGroup.DELETE("/:id/caddy/sni-rules", caddyH.DeleteAllSNI)
				nodeServersGroup.POST("/:id/caddy/batch-update-ports", caddyH.BatchUpdatePorts)
				nodeServersGroup.POST("/:id/caddy/batch-update-log-enabled", caddyH.BatchUpdateLogEnabled)
				nodeServersGroup.DELETE("/:id/caddy/l4-rules", caddyH.DeleteAllL4)

				// Nginx site management (per node)
				nodeServersGroup.GET("/:id/nginx/sites", nginxH.ListSites)
				nodeServersGroup.POST("/:id/nginx/sites", nginxH.CreateSite)
				nodeServersGroup.PUT("/:id/nginx/sites/:siteId", nginxH.UpdateSite)
				nodeServersGroup.DELETE("/:id/nginx/sites/:siteId", nginxH.DeleteSite)
				nodeServersGroup.POST("/:id/nginx/generate-config", nginxH.GenerateConfig)
				nodeServersGroup.POST("/:id/nginx/save-and-apply", nginxH.SaveAndApply)
				nodeServersGroup.GET("/:id/nginx/settings", nginxH.GetSettings)
				nodeServersGroup.POST("/:id/nginx/settings", nginxH.SaveSettings)
				nodeServersGroup.POST("/:id/nginx/sni-rules/toggle", nginxH.ToggleSNI)
				nodeServersGroup.DELETE("/:id/nginx/sni-rules", nginxH.DeleteAllSNI)
				nodeServersGroup.POST("/:id/nginx/batch-update-ports", nginxH.BatchUpdatePorts)
				nodeServersGroup.DELETE("/:id/nginx/l4-rules", nginxH.DeleteAllL4)

				// Nginx WAF Management
				nodeServersGroup.GET("/:id/nginx/waf/config", nginxH.GetWAFConfig)
				nodeServersGroup.PUT("/:id/nginx/waf/config", nginxH.SaveWAFConfig)
				nodeServersGroup.GET("/:id/nginx/waf/rules", nginxH.ListWAFRules)
				nodeServersGroup.GET("/:id/nginx/waf/rules/:name", nginxH.GetWAFRule)
				nodeServersGroup.PUT("/:id/nginx/waf/rules/:name", nginxH.SaveWAFRule)
				nodeServersGroup.GET("/:id/nginx/waf/domain-config", nginxH.GetWAFDomainConfig)
				nodeServersGroup.PUT("/:id/nginx/waf/domain-config", nginxH.SaveWAFDomainConfig)
				nodeServersGroup.GET("/:id/nginx/waf/domain-rules", nginxH.ListWAFDomainDirs)
				nodeServersGroup.GET("/:id/nginx/waf/domain-rules/:domain/:rule", nginxH.GetWAFDomainRule)
				nodeServersGroup.PUT("/:id/nginx/waf/domain-rules/:domain/:rule", nginxH.SaveWAFDomainRule)
				nodeServersGroup.POST("/:id/nginx/waf/toggle-domain", nginxH.ToggleWAFDomain)
				nodeServersGroup.GET("/:id/nginx/waf/domain-status", nginxH.CheckWAFDomain)

				// Inbound Configs (per node)
				nodeServersGroup.GET("/:id/inbound-configs", inboundCfgH.List)
				nodeServersGroup.POST("/:id/inbound-configs", inboundCfgH.Create)
				nodeServersGroup.GET("/:id/inbound-configs/:cfgId", inboundCfgH.Get)
				nodeServersGroup.PUT("/:id/inbound-configs/:cfgId", inboundCfgH.Update)
				nodeServersGroup.DELETE("/:id/inbound-configs/:cfgId", inboundCfgH.Delete)
				nodeServersGroup.POST("/:id/inbound-configs/:cfgId/users/:email/pause", inboundCfgH.PauseUser)
				nodeServersGroup.POST("/:id/inbound-configs/:cfgId/users/:email/resume", inboundCfgH.ResumeUser)
				nodeServersGroup.POST("/:id/inbound-configs/reset-all-traffic", inboundCfgH.ResetAllTraffic)

				// Outbound Configs (per node)
				nodeServersGroup.GET("/:id/outbound-configs", outboundCfgH.List)
				nodeServersGroup.POST("/:id/outbound-configs", outboundCfgH.Create)
				nodeServersGroup.GET("/:id/outbound-configs/:cfgId", outboundCfgH.Get)
				nodeServersGroup.PUT("/:id/outbound-configs/:cfgId", outboundCfgH.Update)
				nodeServersGroup.DELETE("/:id/outbound-configs/:cfgId", outboundCfgH.Delete)
				nodeServersGroup.PUT("/:id/outbound-configs/reorder", outboundCfgH.Reorder)

				// Services (per node) — service management with process control
				nodeServersGroup.GET("/:id/services", svcH.List)
				nodeServersGroup.POST("/:id/services", svcH.Create)
				nodeServersGroup.GET("/:id/services/:serviceId", svcH.GetByID)
				nodeServersGroup.PUT("/:id/services/:serviceId", svcH.Update)
				nodeServersGroup.DELETE("/:id/services/:serviceId", svcH.Delete)
				nodeServersGroup.GET("/:id/services/:serviceId/endpoints", svcH.ListEndpoints)
				nodeServersGroup.POST("/:id/services/:serviceId/endpoints", svcH.CreateEndpoint)
				nodeServersGroup.PUT("/:id/services/:serviceId/endpoints/:endpointId", svcH.UpdateEndpoint)
				nodeServersGroup.DELETE("/:id/services/:serviceId/endpoints/:endpointId", svcH.DeleteEndpoint)

				// Process management (per node service)
				nodeServersGroup.POST("/:id/services/:serviceId/start", processH.Start)
				nodeServersGroup.POST("/:id/services/:serviceId/stop", processH.Stop)
				nodeServersGroup.POST("/:id/services/:serviceId/restart", processH.Restart)
				nodeServersGroup.GET("/:id/services/:serviceId/process-status", processH.GetProcessStatus)
				nodeServersGroup.PUT("/:id/services/:serviceId/guardian", processH.UpdateGuardian)
				nodeServersGroup.GET("/:id/services/:serviceId/crash-logs", processH.GetCrashLogs)
				nodeServersGroup.GET("/:id/services/:serviceId/crash-logs/:logId", processH.GetCrashLog)
				nodeServersGroup.DELETE("/:id/services/:serviceId/crash-logs", processH.ClearCrashLogs)
				nodeServersGroup.GET("/:id/services/process-status", processH.ListAllProcessStatuses)

				// Geo Rules (per node) — rule source and file management
				nodeServersGroup.GET("/:id/geo-rules/github-assets", geoRuleH.FetchGitHubAssets)
				nodeServersGroup.GET("/:id/geo-rules/sources", geoRuleH.ListSources)
				nodeServersGroup.POST("/:id/geo-rules/sources", geoRuleH.CreateSource)
				nodeServersGroup.PUT("/:id/geo-rules/sources/:sourceId", geoRuleH.UpdateSource)
				nodeServersGroup.DELETE("/:id/geo-rules/sources/:sourceId", geoRuleH.DeleteSource)
				nodeServersGroup.GET("/:id/geo-rules/sources/:sourceId/assets", geoRuleH.FetchReleaseAssets)
				nodeServersGroup.POST("/:id/geo-rules/sources/:sourceId/select", geoRuleH.SelectFiles)
				nodeServersGroup.POST("/:id/geo-rules/sources/:sourceId/sync", geoRuleH.SyncSource)
				nodeServersGroup.PUT("/:id/geo-rules/sources/:sourceId/schedule", geoRuleH.ToggleSchedule)
				nodeServersGroup.POST("/:id/geo-rules/sync-all", geoRuleH.SyncAll)
				nodeServersGroup.GET("/:id/geo-rules/sync-status/:key", geoRuleH.SyncStatus)
				nodeServersGroup.GET("/:id/geo-rules/files", geoRuleH.ListFiles)
				nodeServersGroup.POST("/:id/geo-rules/files/:fileId/download", geoRuleH.Download)
				nodeServersGroup.DELETE("/:id/geo-rules/files/:fileId", geoRuleH.DeleteFile)
				nodeServersGroup.PUT("/:id/geo-rules/files/:fileId/auto-sync", geoRuleH.SetAutoSync)
				nodeServersGroup.POST("/:id/geo-rules/seed", geoRuleH.SeedDefaults)
				nodeServersGroup.GET("/:id/geo-rules/status", geoRuleH.Status)

				// WARP (per node) — each node has its own independent WARP configs
				nodeServersGroup.GET("/:id/warp", warpH.List)
				nodeServersGroup.GET("/:id/warp/status", warpH.Status)
				nodeServersGroup.POST("/:id/warp/register", warpH.Register)
				nodeServersGroup.POST("/:id/warp/generate-config", warpH.GenerateConfig)
				nodeServersGroup.GET("/:id/warp/config", warpH.GetConfig)
				nodeServersGroup.DELETE("/:id/warp/:warpId", warpH.Delete)

				// Share Domain Configs (per node — direct domains only)
				nodeServersGroup.GET("/:id/share-domains", shareDomainH.ListByServer)
				nodeServersGroup.POST("/:id/share-domains", shareDomainH.Create)
				nodeServersGroup.PUT("/:id/share-domains/:domainId", shareDomainH.Update)
				nodeServersGroup.DELETE("/:id/share-domains/:domainId", shareDomainH.Delete)
				nodeServersGroup.DELETE("/:id/share-domains/all", shareDomainH.ClearAll)
			}

			// CA Center (separate group, reuses nodeServerH) — admin only.
			caCenterGroup := protected.Group("/ca").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				caCenterGroup.GET("/status", nodeServerH.GetCAStatus)
				caCenterGroup.GET("/agent-certs", nodeServerH.ListAgentCerts)
				caCenterGroup.POST("/agent-certs/:server_id/renew", nodeServerH.RenewAgentCertificate)
				caCenterGroup.GET("/revocations", nodeServerH.ListRevocations)
				caCenterGroup.GET("/revocations/export", nodeServerH.ExportCRL)
				caCenterGroup.GET("/distribution-queue", nodeServerH.GetDistributionQueue)
				caCenterGroup.POST("/distribution-queue/:id/retry", nodeServerH.RetryDistribution)
				caCenterGroup.DELETE("/distribution-queue/:id", nodeServerH.DeleteDistributionQueueItem)
			}

			// Region Overview (separate group, reuses nodeServerH)
			regionsOverviewGroup := protected.Group("/regions")
			{
				regionsOverviewGroup.GET("/overview", nodeServerH.GetRegionsOverview)
				regionsOverviewGroup.GET("/:region/servers", nodeServerH.ListServersByRegion)
			}

			// Node Audit Logs (separate group, reuses nodeServerH) — admin only.
			nodeAuditGroup := protected.Group("/node-audit-logs").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				nodeAuditGroup.GET("", nodeServerH.ListNodeAuditLogs)
				nodeAuditGroup.GET("/export", nodeServerH.ExportNodeAuditLogs)
			}

			// Certificates — admin only (issue/deploy/delete are high-risk).
			certsGroup := protected.Group("/certificates").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				certsGroup.GET("", certH.List)
				certsGroup.POST("", certH.Create)
				certsGroup.POST("/issue", certH.Issue)
				certsGroup.GET("/detect-ip", certH.DetectIP)
				certsGroup.GET("/check-port", certH.CheckPort)
				certsGroup.POST("/upload", certH.Upload)
				certsGroup.GET("/:id", certH.GetByID)
				certsGroup.PUT("/:id", certH.Update)
				certsGroup.DELETE("/:id", certH.Delete)
			certsGroup.POST("/:id/renew", certH.Renew)
		}

			// DNS Channels — admin only.
			certChannelsGroup := protected.Group("/cert-channels").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				certChannelsGroup.GET("", certChannelH.List)
				certChannelsGroup.POST("", certChannelH.Create)
				certChannelsGroup.GET("/providers", certChannelH.Providers)
				certChannelsGroup.PUT("/:id", certChannelH.Update)
				certChannelsGroup.DELETE("/:id", certChannelH.Delete)
			}

			// ACME Accounts — admin only.
			acmeAccountsGroup := protected.Group("/acme-accounts").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				acmeAccountsGroup.GET("", certEmailH.List)
				acmeAccountsGroup.POST("", certEmailH.Create)
				acmeAccountsGroup.GET("/default", certEmailH.GetDefault)
				acmeAccountsGroup.PUT("/:id", certEmailH.Update)
				acmeAccountsGroup.DELETE("/:id", certEmailH.Delete)
			}

			// Xray Stats & Control — moved under /node-servers/:id/
			xrayStatsH := xrayStatsHandler.NewHandler(systemSettingSvc)
			xrayStatsH.SetCoreConfigService(coreConfigSvc)
			xrayStatsH.SetNodeOperator(nodeServerSvcInst)
			xrayControlH := xrayControlHandler.NewHandler(systemSettingSvc)
			xrayControlH.SetCoreConfigService(coreConfigSvc)
			xrayControlH.SetNodeOperator(nodeServerSvcInst)

			nodeServersGroup.GET("/:id/xray/stats", xrayStatsH.GetStats)
			nodeServersGroup.GET("/:id/xray/stats/users", xrayStatsH.GetUserStats)
			nodeServersGroup.POST("/:id/xray/stats/reset/:email", xrayStatsH.ResetUserStats)

			// Xray Control (dynamic inbound/outbound/routing)
			nodeServersGroup.POST("/:id/xray/control/inbound", xrayControlH.AddInbound)
			nodeServersGroup.DELETE("/:id/xray/control/inbound/:tag", xrayControlH.RemoveInbound)
			nodeServersGroup.POST("/:id/xray/control/outbound", xrayControlH.AddOutbound)
			nodeServersGroup.DELETE("/:id/xray/control/outbound/:tag", xrayControlH.RemoveOutbound)
			nodeServersGroup.POST("/:id/xray/control/inbound/:tag/user", xrayControlH.AddUser)
			nodeServersGroup.DELETE("/:id/xray/control/inbound/:tag/user/:email", xrayControlH.RemoveUser)
			nodeServersGroup.POST("/:id/xray/control/firewall/apply", xrayControlH.ApplyFirewallRules)
			nodeServersGroup.POST("/:id/xray/control/firewall/remove", xrayControlH.RemoveFirewallRules)
			nodeServersGroup.POST("/:id/xray/control/routing", xrayControlH.ApplyRouting)
			nodeServersGroup.POST("/:id/xray/control/routing/rule", xrayControlH.AddRoutingRule)
			nodeServersGroup.DELETE("/:id/xray/control/routing/rule/:tag", xrayControlH.RemoveRoutingRule)
			nodeServersGroup.GET("/:id/xray/control/routing/rules", xrayControlH.ListRoutingRules)
			nodeServersGroup.GET("/:id/xray/control/balancer/:tag", xrayControlH.GetBalancerInfo)
			nodeServersGroup.POST("/:id/xray/control/balancer/:tag", xrayControlH.SetBalancerTarget)
			nodeServersGroup.POST("/:id/xray/control/sync", func(c *gin.Context) {
				if err := configSyncSvc.SyncNow(c.Request.Context()); err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				c.JSON(200, gin.H{"message": "config synced successfully", "last_sync": configSyncSvc.LastSync()})
			})

			// Data sync: push full DB tables to remote Agent (for Agent startup / manual refresh).
			// Most destructive (apply_and_start defaults to true) — super_admin only.
			nodeServersGroup.Group("").Use(middleware.RequireRole(domainuser.RoleSuperAdmin)).POST("/:id/sync-data", func(c *gin.Context) {
				serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
				if err != nil {
					c.JSON(400, gin.H{"error": "invalid server id"})
					return
				}
				var req struct {
					ApplyAndStart bool `json:"apply_and_start"`
				}
				c.ShouldBindJSON(&req)
				// Default: apply_and_start=true
				if !req.ApplyAndStart {
					req.ApplyAndStart = true
				}
				resp, err := nodeServerSvcInst.SyncServerData(c.Request.Context(), serverID, req.ApplyAndStart)
				if err != nil {
					nodeServerSvcInst.CreateAuditLog(c.Request.Context(), serverID, 0, "sync_data", fmt.Sprintf("apply_and_start=%v", req.ApplyAndStart), c.ClientIP(), c.Request.UserAgent(), false, err.Error())
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				nodeServerSvcInst.CreateAuditLog(c.Request.Context(), serverID, 0, "sync_data", fmt.Sprintf("synced data, apply_and_start=%v", req.ApplyAndStart), c.ClientIP(), c.Request.UserAgent(), true, "")
				c.JSON(200, resp)
			})

			// Geo Rules (moved under /node-servers/:id/geo-rules)

			// Metrics — admin only (operational data).
			metricsGroup := protected.Group("/metrics").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				metricsGroup.GET("/overview", metricsH.Overview)
				metricsGroup.GET("/services", metricsH.ServiceMetrics)
				metricsGroup.GET("/nodes", metricsH.NodeMetrics)
				metricsGroup.GET("/certificates", metricsH.CertMetrics)

			}

			// Security — admin only (audit, alerts, channels).
			securityGroup := protected.Group("/security").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				securityGroup.GET("/audit-events", securityH.ListAuditEvents)
				securityGroup.GET("/alerts", securityH.ListAlerts)
				securityGroup.GET("/alert-channels", securityH.ListAlertChannels)
				securityGroup.POST("/alert-channels", securityH.CreateAlertChannel)
				securityGroup.PUT("/alert-channels/:id", securityH.UpdateAlertChannel)
				securityGroup.DELETE("/alert-channels/:id", securityH.DeleteAlertChannel)
				securityGroup.POST("/alerts/:id/resolve", securityH.ResolveAlert)
			}

			// Backups — admin for read/manage; destructive restore/delete/upload
			// requires super_admin.
			backupsGroup := protected.Group("/backups")
			backupsGroup.Use(middleware.RequireRole(domainuser.RoleAdmin))
			superAdminBackups := backupsGroup.Group("").Use(middleware.RequireRole(domainuser.RoleSuperAdmin))
		{
			backupsGroup.GET("/database", backupsH.ListDBBackups)
			backupsGroup.POST("/database", backupsH.CreateDBBackup)
			backupsGroup.POST("/cleanup-jobs", backupsH.CreateCleanupJob)

			// Full backup
				backupsGroup.POST("/create", backupsH.CreateFullBackup)
				backupsGroup.GET("/list", backupsH.ListFullBackups)
				backupsGroup.GET("/:id/download", backupsH.DownloadBackup)

				// Schedules
				backupsGroup.GET("/schedules", backupsH.ListSchedules)
				backupsGroup.POST("/schedules", backupsH.CreateSchedule)
				backupsGroup.PUT("/schedules/:id", backupsH.UpdateSchedule)
				backupsGroup.DELETE("/schedules/:id", backupsH.DeleteSchedule)

				// Storages
backupsGroup.GET("/storages", backupsH.ListStorages)
backupsGroup.POST("/storages", backupsH.CreateStorage)
backupsGroup.PUT("/storages/:id", backupsH.UpdateStorage)
backupsGroup.DELETE("/storages/:id", backupsH.DeleteStorage)
backupsGroup.GET("/storages/:id/files", backupsH.ListRemoteFiles)

			// Destructive (super_admin only)
			superAdminBackups.POST("/database/restore", backupsH.RestoreDBBackup)
			superAdminBackups.POST("/:id/restore", backupsH.RestoreBackup)
			superAdminBackups.DELETE("/:id", backupsH.DeleteBackup)
			superAdminBackups.POST("/upload-restore", backupsH.UploadRestore)
			superAdminBackups.POST("/remote-restore", backupsH.RemoteRestore)
			superAdminBackups.GET("/restore-status", backupsH.GetRestoreStatus)
			superAdminBackups.POST("/:id/upload-ftp", backupsH.UploadToFTP)
			superAdminBackups.POST("/:id/upload-s3", backupsH.UploadToS3)
			superAdminBackups.POST("/:id/upload-webdav", backupsH.UploadToWebDAV)
			}

			// System — admin for config/management; the most destructive
			// operations (system update/rollback/restart/migrate, db migrate,
			// encryption key generation, reality reset) require super_admin.
			systemGroup := protected.Group("/system")
			systemGroup.Use(middleware.RequireRole(domainuser.RoleAdmin))
			superAdminSystem := systemGroup.Group("").Use(middleware.RequireRole(domainuser.RoleSuperAdmin))
			{
				systemGroup.GET("/stats", systemH.GetSystemStats)
				systemGroup.GET("/bootstrap-state", systemH.GetBootstrapState)
				systemGroup.PUT("/bootstrap-finish", systemH.FinishBootstrap)
				systemGroup.GET("/releases", systemH.ListReleases)
				systemGroup.GET("/update-jobs", systemH.ListUpdateJobs)
				systemGroup.GET("/panel-config", systemH.GetPanelConfig)
				systemGroup.PUT("/panel-config", systemH.UpdatePanelConfig)
				systemGroup.GET("/notification-config", systemH.GetNotificationConfig)
				systemGroup.PUT("/notification-config", systemH.UpdateNotificationConfig)
				systemGroup.POST("/test-smtp", systemH.TestSMTP)
				systemGroup.POST("/test-telegram", systemH.TestTelegram)
				systemGroup.POST("/upload-mtls-ca", systemH.UploadMTLSCA)
				systemGroup.GET("/github-config", systemH.GetGitHubConfig)
				systemGroup.PUT("/github-config", systemH.UpdateGitHubConfig)
				systemGroup.GET("/db-config", systemH.GetDBConfig)
				systemGroup.PUT("/db-config", systemH.UpdateDBConfig)
				systemGroup.POST("/db-config/test", systemH.TestDBConnection)
				systemGroup.GET("/subscription-billing-templates", systemH.GetSubscriptionBillingTemplates)
				systemGroup.PUT("/subscription-billing-templates", systemH.UpdateSubscriptionBillingTemplates)
				systemGroup.GET("/notification-templates", systemH.ListTemplates)
				systemGroup.POST("/notification-templates", systemH.CreateTemplate)
				systemGroup.PUT("/notification-templates/:id", systemH.UpdateTemplate)
				systemGroup.DELETE("/notification-templates/:id", systemH.DeleteTemplate)
				systemGroup.GET("/reality-targets", systemH.GetRealityTargets)
				systemGroup.PUT("/reality-targets", systemH.UpdateRealityTargets)
				systemGroup.GET("/config-file/:coreType", systemH.GetConfigFile)
				// Generic settings API (used by self-unban, OAuth config, etc.)
				systemGroup.GET("/settings", systemH.GetSettings)
				systemGroup.PUT("/settings/batch", systemH.BatchUpdateSettings)
				systemGroup.GET("/encryption-key-status", systemH.GetEncryptionKeyStatus)
				systemGroup.GET("/jwt-secret-status", systemH.GetJWTSecretStatus)
	systemGroup.GET("/env-config", systemH.GetEnvConfig)
				systemGroup.PUT("/env-config", systemH.UpdateEnvConfig)

	// Maintenance mode (GET is public via v1 group below; PUT requires admin)
			systemGroup.PUT("/maintenance", systemH.UpdateMaintenance)

			// Public maintenance status (no auth, for sub/portal engines)
			v1.GET("/system/maintenance", systemH.GetMaintenance)

				// Destructive (super_admin only)
				superAdminSystem.POST("/update", systemH.SystemUpdate)
				superAdminSystem.POST("/rollback", systemH.SystemRollback)
				superAdminSystem.POST("/restart", systemH.Restart)
				superAdminSystem.POST("/migrate-data", systemH.MigrateData)
				superAdminSystem.POST("/db-config/migrate", systemH.MigrateDBData)
				superAdminSystem.POST("/reality-targets/reset", systemH.ResetRealityTargets)
				superAdminSystem.POST("/generate-encryption-key", systemH.GenerateEncryptionKey)
				superAdminSystem.POST("/regenerate-jwt-secret", systemH.RegenerateJWTSecret)
			}

			// Proxy/Caddy/Nginx management routes moved under /node-servers/:id/

			// Log management — admin only (writer takeover, stdout capture).
			logManageGroup := protected.Group("/log-manage").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				logManageGroup.GET("/status", logManageH.GetStatus)
				logManageGroup.GET("/policies", logManageH.ListPolicies)
				logManageGroup.POST("/policies", logManageH.CreatePolicy)
				logManageGroup.GET("/policies/:id", logManageH.GetPolicy)
				logManageGroup.PUT("/policies/:id", logManageH.UpdatePolicy)
				logManageGroup.DELETE("/policies/:id", logManageH.DeletePolicy)
				logManageGroup.POST("/policies/:id/rotate", logManageH.Rotate)
				logManageGroup.GET("/policies/:id/jobs", logManageH.ListPolicyJobs)
				logManageGroup.POST("/discover", logManageH.DiscoverLogs)
				logManageGroup.POST("/sync", logManageH.SyncPolicies)
				logManageGroup.GET("/policies/:id/log", logManageH.GetLog)
				logManageGroup.GET("/policies/:id/log-stream", logManageH.StreamLog)
				logManageGroup.GET("/log-output", logManageH.GetLogOutput)
				logManageGroup.PUT("/log-output", logManageH.SaveLogOutput)
				logManageGroup.GET("/core-log-output", logManageH.GetCoreLogOutput)
				logManageGroup.PUT("/core-log-output", logManageH.SaveCoreLogOutput)
				logManageGroup.POST("/log-output/takeover/:type", logManageH.TakeoverStdout)
				logManageGroup.POST("/log-output/release/:type", logManageH.ReleaseStdout)
				logManageGroup.GET("/proxy/:type/stdout-log", logManageH.ProxyStdoutLog)
				logManageGroup.GET("/proxy/:type/stdout-stream", logManageH.ProxyStdoutStream)
				logManageGroup.GET("/core/:type/stdout-log", logManageH.ProxyStdoutLog)
				logManageGroup.GET("/core/:type/stdout-stream", logManageH.ProxyStdoutStream)
				logManageGroup.GET("/writers", logManageH.GetWriterStatus)
				logManageGroup.POST("/writers/:type/start", logManageH.StartWriter)
				logManageGroup.POST("/writers/:type/stop", logManageH.StopWriter)
				logManageGroup.PUT("/writers/:type/config", logManageH.UpdateWriterConfig)
				logManageGroup.GET("/writers/:type/files", logManageH.ListWriterFiles)
				// Node-based centralized log management
				logManageGroup.GET("/nodes/:serverId/logs", logManageH.ListNodeLogs)
				logManageGroup.GET("/nodes/:serverId/logs/:type", logManageH.GetNodeLog)
				logManageGroup.GET("/nodes/:serverId/logs/:type/stream", logManageH.StreamNodeLog)
				logManageGroup.POST("/nodes/:serverId/logs/:type/rotate", logManageH.RotateNodeLog)
			}

			// Agent bootstrap (public — no auth, uses bootstrap token, registered on v1 not protected)
			v1.POST("/agent/bootstrap", func(c *gin.Context) {
				var req struct {
					ServerID       int64  `json:"server_id" binding:"required"`
					BootstrapToken string `json:"bootstrap_token" binding:"required"`
					Hostname       string `json:"hostname"`
					AgentVersion   string `json:"agent_version"`
				}
				if err := c.ShouldBindJSON(&req); err != nil {
					response.BadRequest(c, "INVALID_BODY", err.Error())
					return
				}
				if bootstrapTokenMgr == nil || caService == nil {
					response.InternalError(c, "NOT_INITIALIZED", "CA or bootstrap manager not initialized")
					return
				}
				certResult, err := bootstrapTokenMgr.ValidateBootstrapToken(c.Request.Context(), req.ServerID, req.BootstrapToken)
				if err != nil {
					response.BadRequest(c, "BOOTSTRAP_FAILED", err.Error())
					return
				}
				response.OK(c, gin.H{
					"server_id":       req.ServerID,
					"cert_pem":        string(certResult.CertPEM),
					"key_pem":         string(certResult.KeyPEM),
					"ca_cert_pem":     string(certResult.CACertPEM),
					"cert_expires_at": certResult.ExpiresAt.Unix(),
				})
			})

			// Agent download + install script (public — no auth, bootstrap token verifies identity)
			agentH := agentHandler.NewHandler(dataDir, bootstrapTokenMgr).WithSettingStore(systemSettingRepo)
			v1.GET("/agent/download", agentH.DownloadAgent)
			v1.GET("/agent/install", agentH.GetInstallScript)
			// Agent version & releases (requires auth — admin only)
			protected.Group("").Use(middleware.RequireRole(domainuser.RoleAdmin)).GET("/agent/version", agentH.GetAgentVersion)
			protected.Group("").Use(middleware.RequireRole(domainuser.RoleAdmin)).GET("/agent/releases", agentH.GetAgentReleases)

		// Placeholder routes for remaining modules
		registerPlaceholderRoutes(protected, database, runtimecfgHandler.NewHandler(runtimeApplySvcInst))

			// Portal template management (admin only)
			portalH := portalHandler.NewHandler(portalTmplSvc)
			portalGroup := protected.Group("/portal/templates")
			{
				portalGroup.GET("", portalH.List)
				portalGroup.POST("", portalH.Create)
				portalGroup.GET("/:id", portalH.Get)
				portalGroup.PUT("/:id", portalH.Update)
				portalGroup.DELETE("/:id", portalH.Delete)
				portalGroup.POST("/:id/activate", portalH.Activate)
			}

			// Portal site management (active config / cache) — admin only.
			portalActiveGroup := protected.Group("/portal").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				portalActiveGroup.GET("/active", portalH.GetActive)
				portalActiveGroup.PUT("/active", portalH.UpdateActive)
				portalActiveGroup.POST("/reset", portalH.ResetToDefault)
				// Render cache management
				portalActiveGroup.GET("/cache", portalH.CacheStatus)
				portalActiveGroup.DELETE("/cache", portalH.CacheClear)
				portalActiveGroup.POST("/cache/rebuild", portalH.CacheRebuild)
				portalActiveGroup.PUT("/cache/dev-mode", portalH.CacheDevMode)
			}
		}
	}

	// Public subscription endpoint — guarded by maintenance mode
	r.GET("/sub/:token", middleware.MaintenanceGuard(), publicSubH.ServeSubscription)

	// Portal page — served from active custom template (cached)
	// Note: uses /p to avoid collision with admin SPA route /portal (PortalManagement.vue)
	// Guarded by maintenance mode so portal visitors see the maintenance page.
	r.GET("/p", middleware.MaintenanceGuard(), func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		html := portalTmplSvc.RenderPage(c.Request.Context())
		c.Data(200, "text/html; charset=utf-8", []byte(html))
	})

	// Client auth (public)
	clientAuthGroup := r.Group("/api/v1/client-auth")
	{
		clientAuthGroup.POST("/register", clientauthH.Register)
		clientAuthGroup.POST("/login", clientauthH.Login)
		clientAuthGroup.POST("/refresh", clientauthH.Refresh)

		// OAuth2 login routes (public — no auth required)
		oauthSvcInst := oauthService.NewService(systemSettingSvc, repository.NewClientOAuthBindingRepository(database), clientRepo)
		oauthH := oauthHandler.NewHandler(oauthSvcInst, jwtSecret)
		clientAuthGroup.GET("/oauth/providers", oauthH.ProvidersStatus)
		clientAuthGroup.GET("/oauth/:provider", oauthH.Authorize)
		clientAuthGroup.GET("/oauth/:provider/callback", oauthH.Callback)
	}

	// Public subscription endpoints (for portal) — guarded by maintenance mode
	// Direct DB read returning a plain array so the portal frontend can consume d.data directly (no pagination wrapper).
	subendRepoPortal := repository.NewSubscriptionEndpointRepository(database)
	r.GET("/api/v1/public/subscription-endpoints", middleware.MaintenanceGuard(), func(c *gin.Context) {
		endpoints, err := subendRepoPortal.List(c.Request.Context())
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		response.OK(c, endpoints)
	})

	// Public settings (for portal)
	// v3.4: portalTmplSvc used in public/settings to return latest portal config
	r.GET("/api/v1/public/settings", middleware.MaintenanceGuard(), func(c *gin.Context) {
		registrationEnabled, _ := systemSettingSvc.GetByKey(c.Request.Context(), "panel.registration_enabled")
		chatProvider, _ := systemSettingSvc.GetByKey(c.Request.Context(), "chat.provider")
		chatSiteID, _ := systemSettingSvc.GetByKey(c.Request.Context(), "chat.site_id")
		result := gin.H{
			"registration_enabled": registrationEnabled == "true",
			"chat_provider":        chatProvider,
			"chat_site_id":         chatSiteID,
		}
		// v3.4: 返回最新 portal 配置，前端可用于热更新模块/导航
		if cfg, err := portalTmplSvc.GetActiveConfig(c.Request.Context()); err == nil && cfg != nil {
			result["portal_config"] = cfg
		}
		response.OK(c, result)
	})

	// Public: effective portal runtime config — lean payload served from the
	// in-memory cache in production (fresh DB read only on cache miss).
	r.GET("/api/v1/public/portal-config", middleware.MaintenanceGuard(), func(c *gin.Context) {
		body, err := portalTmplSvc.GetPublicConfigJSON(c.Request.Context())
		if err != nil {
			response.InternalError(c, "PORTAL_CONFIG_FAILED", err.Error())
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	})

	// Public articles (no auth required)
	r.GET("/api/v1/public/articles", middleware.MaintenanceGuard(), clientExtH.PublicListArticles)
	r.GET("/api/v1/public/articles/:slug", middleware.MaintenanceGuard(), clientExtH.PublicGetArticle)

	// Public: list enabled client configs (for portal "more subscriptions" dialog)
	mihomoCfgRepoPublic := repository.NewMihomoConfigRepository(database)
	subFmtRepoPublic := repository.NewSubscriptionFormatRepository(database)
	r.GET("/api/v1/public/client-configs", middleware.MaintenanceGuard(), func(c *gin.Context) {
		planIDStr := c.Query("plan_id")
		var planID int64
		if planIDStr != "" {
			planID, _ = strconv.ParseInt(planIDStr, 10, 64)
		}

		// 1) Fetch enabled MihomoConfigs, filter by plan_id
		configs, err := mihomoCfgRepoPublic.List(c.Request.Context())
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		var cfgItems []gin.H
		for _, cfg := range configs {
			if !cfg.Enabled {
				continue
			}
			// Parse plan_ids from JSON string (manual handling for compatibility)
			var planIDs []int64
			if cfg.PlanIDs != nil && len(cfg.PlanIDs) > 0 {
				planIDs = []int64(cfg.PlanIDs)
			}
			// Filter by plan_id: show if plan_ids is empty (universal) or contains the plan_id
			if len(planIDs) > 0 && !containsInt64(planIDs, planID) {
				continue
			}
			cfgItems = append(cfgItems, gin.H{
				"id":          cfg.ID,
				"name":        cfg.Name,
				"description": cfg.Description,
				"format":      cfg.Format,
				"client_type": cfg.ClientType,
				"platform":    cfg.Platform,
				"sort_order":  cfg.SortOrder,
				"plan_ids":    planIDs,
				"type":        "config",
			})
		}

		// 2) Fetch enabled SubscriptionFormats, filter by plan_id
		formats, err := subFmtRepoPublic.List(c.Request.Context())
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		var fmtItems []gin.H
		for _, fmt := range formats {
			if !fmt.Enabled {
				continue
			}
			// Filter by plan_id: show if plan_ids is empty (universal) or contains the plan_id
			if len(fmt.PlanIDs) > 0 && !containsInt64(fmt.PlanIDs, planID) {
				continue
			}
			fmtItems = append(fmtItems, gin.H{
				"id":          fmt.ID,
				"name":        fmt.Name,
				"description": fmt.Description,
				"format":      fmt.Format,
				"client_type": fmt.ClientType,
				"platform":    "",
				"sort_order":  fmt.SortOrder,
				"plan_ids":    fmt.PlanIDs,
				"type":        "format",
			})
		}

		// Merge and return
		allItems := append(cfgItems, fmtItems...)
		if allItems == nil {
			allItems = []gin.H{}
		}
		response.OK(c, allItems)
	})

	// Charge callback (public, called by charge sites)
	r.POST("/api/v1/charge/callback/:site_id", commerceH.HandleChargeCallback)

	// Client auth (protected) — guarded by maintenance mode
	clientAuthProtected := r.Group("/api/v1/client-auth")
	clientAuthProtected.Use(middleware.MaintenanceGuard())
	clientAuthProtected.Use(clientauthHandler.ClientAuthMiddleware(clientauthSvcInst))
	{
		clientAuthProtected.GET("/me", clientauthH.GetMe)
		clientAuthProtected.POST("/rotate-token", clientauthH.RotateToken)
		clientAuthProtected.PUT("/settings", clientauthH.UpdateSettings)
		clientAuthProtected.POST("/change-password", clientauthH.ChangePassword)
		clientAuthProtected.GET("/traffic-history", clientauthH.GetTrafficHistory)
		// Client log package endpoints (subscription user log access)
		clientAuthProtected.GET("/logs/types", clientLogH.LogPackageHealthCheck)
		clientAuthProtected.POST("/logs/package", clientLogH.CreateLogPackage)
		clientAuthProtected.GET("/logs/packages", clientLogH.ListLogPackages)
		clientAuthProtected.GET("/logs/packages/:id", clientLogH.GetLogPackage)
		clientAuthProtected.GET("/logs/packages/:id/download", clientLogH.DownloadLogPackage)
		clientAuthProtected.DELETE("/logs/packages/:id", clientLogH.DeleteLogPackage)

		// P0-1: Plan display
		clientAuthProtected.GET("/plans", commerceH.ListPlans)
		clientAuthProtected.GET("/plans/:id", commerceH.GetPlan)

		// Traffic packs (流量叠加包)
		clientAuthProtected.GET("/traffic-packs", commerceH.ListTrafficPacks)
		clientAuthProtected.POST("/traffic-packs/buy", commerceH.BuyTrafficPack)

		// P0-2: Order management
		clientAuthProtected.POST("/orders", commerceH.CreateOrder)
		clientAuthProtected.GET("/orders", commerceH.ListOrders)
		clientAuthProtected.GET("/orders/:id", commerceH.GetOrder)

		// P0-3: Charging system
		clientAuthProtected.GET("/charge/sites", commerceH.ListChargeSites)
		clientAuthProtected.POST("/charge/create", commerceH.CreateChargeOrder)
		clientAuthProtected.GET("/charge/status/:trade_no", commerceH.GetChargeStatus)
		clientAuthProtected.POST("/charge/redeem-card", commerceH.RedeemCard)
		clientAuthProtected.GET("/charge/orders", commerceH.ListChargeOrders)
		clientAuthProtected.GET("/charge/orders/:id", commerceH.GetChargeOrderDetail)

		// P0-4: Balance system
		clientAuthProtected.GET("/balance", commerceH.GetBalance)
		clientAuthProtected.GET("/balance/transactions", commerceH.ListBalanceTransactions)

		// P1-1: Node list (client visible)
		clientAuthProtected.GET("/nodes", clientExtH.ListClientNodes)
		clientAuthProtected.GET("/nodes/:id/config", clientExtH.GetNodeConfig)
		clientAuthProtected.GET("/nodes/:id/link", clientExtH.GetNodeLink)
		clientAuthProtected.GET("/nodes/:id/qrcode", clientExtH.GetNodeQRCode)

		// P1-2: Referral system
		clientAuthProtected.GET("/referral/code", clientExtH.GetReferralCode)
		clientAuthProtected.GET("/referral/stats", clientExtH.GetReferralStats)
		clientAuthProtected.GET("/referral/records", clientExtH.ListReferralRecords)
		clientAuthProtected.GET("/referral/withdrawals", clientExtH.ListReferralWithdrawals)
		clientAuthProtected.POST("/referral/withdraw", clientExtH.RequestWithdrawal)

		// P1-3: Ticket system
		clientAuthProtected.POST("/tickets", clientExtH.CreateTicket)
		clientAuthProtected.GET("/tickets", clientExtH.ListTickets)
		clientAuthProtected.GET("/tickets/:id", clientExtH.GetTicket)
		clientAuthProtected.POST("/tickets/:id/reply", clientExtH.ReplyTicket)
		clientAuthProtected.POST("/tickets/:id/close", clientExtH.CloseTicket)

		// P1-4: Invoices (reuse plan orders) — 与文档设计统一为 /invoices
		clientAuthProtected.GET("/invoices", commerceH.ListOrders)
		clientAuthProtected.GET("/bills", commerceH.ListOrders) // 向后兼容别名

		// P2-1: Gift system
		clientAuthProtected.GET("/gifts", clientExtH.ListGifts)
		clientAuthProtected.POST("/gifts/:id/claim", clientExtH.ClaimGift)

		// P2-2: Articles
		clientAuthProtected.GET("/articles", clientExtH.ListArticles)
		clientAuthProtected.GET("/articles/:slug", clientExtH.GetArticle)

		// P2-3: Subscription reset — 与文档设计统一为 /reset-subscription
		clientAuthProtected.POST("/reset-subscription", clientExtH.ResetSubscription)
		clientAuthProtected.POST("/subscription/reset", clientExtH.ResetSubscription) // 向后兼容别名

		// P3-1: Shared accounts
		clientAuthProtected.GET("/shared-accounts", clientExtH.ListSharedAccounts)
		clientAuthProtected.GET("/shared-accounts/:id", clientExtH.GetSharedAccount)
		clientAuthProtected.POST("/shared-accounts/:id/repair", clientExtH.RequestRepair)

		// P3-2: IP whitelist — 与文档设计统一为 /settings/whitelist
		clientAuthProtected.GET("/settings/whitelist", clientExtH.ListIPWhitelist)
		clientAuthProtected.POST("/settings/auto-add-ip", clientExtH.AutoAddIP)
		clientAuthProtected.DELETE("/settings/whitelist/:id", clientExtH.RemoveIPWhitelist)
		clientAuthProtected.GET("/ip-whitelist", clientExtH.ListIPWhitelist)          // 向后兼容别名
		clientAuthProtected.POST("/ip-whitelist/auto-add", clientExtH.AutoAddIP)      // 向后兼容别名
		clientAuthProtected.DELETE("/ip-whitelist/:id", clientExtH.RemoveIPWhitelist) // 向后兼容别名

		// P3-3: Self-unban
		clientAuthProtected.POST("/self-unban", clientExtH.SelfUnban)

		// P1-4b: Invoice detail (reuse order query)
		clientAuthProtected.GET("/invoices/:id", commerceH.GetOrder)

		// P3-17: Time check
		clientAuthProtected.GET("/checktime", clientExtH.CheckTime)

		// P3-19: Profile management
		clientAuthProtected.GET("/profile", clientExtH.GetProfile)
		clientAuthProtected.PUT("/profile", clientExtH.UpdateProfile)
		clientAuthProtected.GET("/connection-info", clientExtH.GetConnectionInfo)
		clientAuthProtected.POST("/profile/bind-oauth", clientExtH.BindOAuth)
		clientAuthProtected.DELETE("/profile/unbind-oauth/:provider", clientExtH.UnbindOAuth)

		// P3-20: Subscription formats
		clientAuthProtected.GET("/subscription-formats", clientExtH.ListSubscriptionFormats)
		clientAuthProtected.GET("/subscription-link", clientExtH.GetSubscriptionLink)
		clientAuthProtected.GET("/subscription-link/backup", clientExtH.GetBackupSubscriptionLink)

		// P3-22: Shared account apply
		clientAuthProtected.POST("/shared-accounts/:id/apply", clientExtH.ApplySharedAccount)
	}

	// Check panel mode — if management_only, skip local core process startup
	panelMode, _ := systemSettingSvc.GetByKey(context.Background(), "panel.mode")
	if panelMode == "management_only" {
		log.Println("[Panel] Running in MANAGEMENT_ONLY mode — local core processes disabled")
	} else {
		// Seed default cores and reconcile processes on startup
		processMgr.SeedCores(nil)
		go func() {
			processMgr.Reconcile(nil)
			// Restore firewall rules for hysteria port hopping after process reconcile.
			// Rules are lost on machine reboot and need to be re-created from DB config.
			time.Sleep(2 * time.Second) // wait for cores to start
			restoreFirewallRules(context.Background(), inboundConfigRepo)
		}()
	}

	// Set callback to restart proxy when log output mode is switched
	// Restart is needed because stdout mode requires re-attaching the process's stdout to LogBroker
	logManageSvc.SetOnModeSwitch(func(proxyType string) {
		bgCtx := context.Background()

		// Read the new mode to decide action
		logCfg, _ := logManageSvc.GetLogOutput(bgCtx)
		newMode := "file"
		if proxyType == "nginx" && logCfg != nil {
			newMode = logCfg.Nginx.Mode
		} else if proxyType == "caddy" && logCfg != nil {
			newMode = logCfg.Caddy.Mode
		}

		var pType proxySvc.ProxyType
		switch proxyType {
		case "nginx":
			pType = proxySvc.ProxyNginx
			nginxMgr.SaveAndApply(bgCtx)
		case "caddy":
			pType = proxySvc.ProxyCaddy
			caddyMgr.SaveAndApply(bgCtx)
		default:
			return
		}

		// Only restart for stdout mode (need to re-attach stdout pipe).
		// For file/off mode, reload is sufficient — the config file already
		// reflects the new log directives.
		if newMode == "stdout" {
			log.Printf("router: restarting %s after log mode switch to stdout", proxyType)
			proxyMgr.RestartProxy(bgCtx, pType)
		} else {
			log.Printf("router: reloading %s after log mode switch to %s", proxyType, newMode)
			proxyMgr.ReloadProxy(bgCtx, pType)
		}
	})

	go func() {
		proxyMgr.AutoStartManaged()
		// Start auto-writers after proxy is started (broker is available)
		logManageSvc.StartAutoWriters()
	}()
	logManageSvc.Start()

	// Start config sync service (periodically syncs DB configs to disk)
	configSyncSvc.Start()

	// Start traffic sync service (periodically syncs local traffic stats to DB)
	trafficSyncSvc.Start()

	// Start log cleanup service (periodically removes old log/event records)
	logCleanupSvc := trafficSvc.NewLogCleanupService(database)
	logCleanupSvc.Start(context.Background())

	return r
}

// registerPlaceholderRoutes registers placeholder routes for modules not yet implemented.
func registerPlaceholderRoutes(rg *gin.RouterGroup, database *gorm.DB, runtimeH *runtimecfgHandler.Handler) {
	// Services (access policies, ip bans)
	services := rg.Group("/services")
	accessH := servicesHandler.NewAccessHandler(database, getAccessService(database))
	{
		services.GET("/:id/access-policies", accessH.ListPolicies)
		services.POST("/:id/access-policies", accessH.CreatePolicy)
		services.PUT("/:id/access-policies/:policyId", accessH.UpdatePolicy)
		services.DELETE("/:id/access-policies/:policyId", accessH.DeletePolicy)
		services.GET("/:id/ip-bans", accessH.ListIPBans)
		services.POST("/:id/ip-bans", accessH.CreateIPBan)
		services.DELETE("/:id/ip-bans/:banId", accessH.DeleteIPBan)
	}

	// Runtime config workflow (snapshot / dry-run / apply / rollback / disable-proxy)
	runtime := rg.Group("/runtime")
	{
		runtime.GET("/overview", runtimeH.Overview)
		runtime.GET("/snapshots", runtimeH.ListSnapshots)
		runtime.GET("/snapshots/:id", runtimeH.GetSnapshot)
		runtime.DELETE("/snapshots/:id", runtimeH.DeleteSnapshot)
		runtime.GET("/apply-jobs", runtimeH.ListJobs)
		runtime.GET("/apply-jobs/:id", runtimeH.GetJob)
		runtime.POST("/snapshots", runtimeH.CreateSnapshot)
		runtime.POST("/dry-run", runtimeH.DryRun)
		runtime.POST("/apply", runtimeH.Apply)
		runtime.POST("/apply-jobs/:id/retry", runtimeH.RetryJob)
		runtime.POST("/apply-jobs/:id/cancel", runtimeH.CancelJob)
		runtime.POST("/rollback", runtimeH.Rollback)
		runtime.POST("/disable-proxy", runtimeH.DisableProxy)
		runtime.GET("/nodes", runtimeH.ListNodes)
		runtime.POST("/verify", runtimeH.Verify)
	}

	// Traffic Quotas
	trafficNodeServerRepo := repository.NewNodeServerRepository(database)
	trafficH := trafficHandler.NewHandler(trafficNodeServerRepo, repository.NewTrafficQuotaRepository(database))
	traffic := rg.Group("/traffic")
	{
		// Read-only monitoring view. Quota enforcement lives in the
		// subscription/plan engine (subscription.EnforceQuotaAndExpiry,
		// NodeTrafficQuotaService); this surface only observes it.
		traffic.GET("/quotas", trafficH.ListQuotas)
		traffic.GET("/quotas/:id/usage", trafficH.GetQuotaUsage)
		traffic.GET("/usage-history", trafficH.GetUsageHistory)
		traffic.GET("/reset-logs", trafficH.ListResetLogs)
	}




}

// placeholder returns a handler that returns 501 Not Implemented.
func placeholder(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(501, gin.H{
			"code":    "NOT_IMPLEMENTED",
			"message": name + " not yet implemented",
		})
	}
}

// isSPARoute checks if a path matches a known SPA route.
// Supports both exact matches and dynamic sub-routes (e.g. /node-servers/1/detail).
func isSPARoute(path string) bool {
	// Known SPA route prefixes that may have dynamic sub-routes
	// e.g. /node-servers/1/detail, /node-servers/1
	spaPrefixes := []string{
		"/node-servers",
	}

	for _, prefix := range spaPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	// Known SPA routes (exact match only)
	knownRoutes := []string{
		"/",
		"/login",
		"/dashboard",
		"/online-users",
		"/nodes",
		"/services",
		"/subscriptions",
		"/subscription-formats",
		"/subscription-endpoints",
		"/plans",
		"/clients",
		"/certificates",
		"/users",
		"/log-manage",
		"/settings",
		"/portal",
		"/charge",
		"/tickets",
		"/client-ext",
		"/runtime",
		"/audit",
		"/traffic-quota",
	}

	for _, route := range knownRoutes {
		if path == route {
			return true
		}
	}

	return false
}

// coreLogBrokerAdapter adapts process.Manager to logmanage.CoreLogBrokerProvider interface.
type coreLogBrokerAdapter struct {
	processMgr *process.Manager
}

func (a *coreLogBrokerAdapter) GetCoreLogBroker(coreType string) logManageSvc.LogBrokerReader {
	if a == nil || a.processMgr == nil {
		return nil
	}
	broker := a.processMgr.GetLogBroker(coreType)
	if broker == nil {
		return nil
	}
	return broker // *process.LogBroker implements LogBrokerReader (Snapshot + Subscribe)
}

// restoreFirewallRules scans all enabled hysteria inbound configs for the local server
// and re-creates nftables/iptables rules for port hopping. Called on panel startup
// after process reconcile to restore rules lost on machine reboot.
func restoreFirewallRules(ctx context.Context, repo *repository.InboundConfigRepository) {
	// Local server ID is 1
	configs, err := repo.List(ctx, 1, "xray")
	if err != nil {
		log.Printf("[Firewall] failed to list inbound configs for rule restore: %v", err)
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

// NewSubscriptionEngine creates a separate Gin engine for subscription-only serving on a dedicated port.
// This allows subscription access via domain:port/Token without the /sub prefix.
func NewSubscriptionEngine(database *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.MaintenanceGuard())

	subRepo := repository.NewSubscriptionRepository(database)
	inboundRepo := repository.NewInboundConfigRepository(database)
	shareDomainRepo := repository.NewShareDomainConfigRepository(database)
	cfIPEntryRepo := repository.NewCFIPEntryRepository(database)
	clientRepo := repository.NewClientRepository(database)
	accessLogRepo := repository.NewAccessLogRepository(database)
	mihomoConfigRepo2 := repository.NewMihomoConfigRepository(database)
	mihomoRuleRepo2 := repository.NewMihomoRuleRepository(database)
	extNodeRepo2 := repository.NewExternalNodeRepository(database)
	systemSettingRepo := repository.NewSystemSettingRepository(database)

	// Plan v2 service for node resolution
	planTmplRepo2 := repository.NewPlanTemplateRepository(database)
	planVariantRepo2 := repository.NewPlanVariantRepository(database)
	nodeGroupRepo2 := repository.NewNodeGroupRepository(database)
	nodeGroupMemberRepo2 := repository.NewNodeGroupMemberRepository(database)
	promoRepo2 := repository.NewPromotionRepository(database)
	planOrderRepo2 := repository.NewPlanOrderRepository(database)
	planSvcInst2 := planSvc.NewService(database, planTmplRepo2, planVariantRepo2, nodeGroupRepo2, nodeGroupMemberRepo2, promoRepo2, planOrderRepo2, clientRepo, inboundRepo, extNodeRepo2)

	publicSubH := subsHandler.NewPublicHandler(subRepo, inboundRepo, shareDomainRepo, cfIPEntryRepo, clientRepo, planSvcInst2, extNodeRepo2, mihomoConfigRepo2, mihomoRuleRepo2, systemSettingRepo, accessLogRepo)

	// Subscription at root path - no /sub prefix needed
	r.GET("/:token", publicSubH.ServeSubscription)

	return r
}

// proxySettingsProvider implements agent.ProxySettingsProvider interface
type proxySettingsProvider struct {
	settingStore *repository.SystemSettingRepository
}

// GetNginxSettingsForServer returns nginx settings for a specific server as JSON string.
func (p *proxySettingsProvider) GetNginxSettingsForServer(ctx context.Context, serverID int64) (string, error) {
	key := fmt.Sprintf("proxy.nginx_settings.%d", serverID)
	v, err := p.settingStore.GetByKey(ctx, key)
	if err != nil || v == "" {
		// Fallback to global settings
		v, err = p.settingStore.GetByKey(ctx, "proxy.nginx_settings")
		if err != nil {
			return "", err
		}
	}
	return v, nil
}

// GetCaddySNISettingsForServer returns caddy SNI settings for a specific server as JSON string.
func (p *proxySettingsProvider) GetCaddySNISettingsForServer(ctx context.Context, serverID int64) (string, error) {
	// Build per-node keys
	enabledKey := fmt.Sprintf("proxy.sni_enabled.%d", serverID)
	listenPortKey := fmt.Sprintf("proxy.sni_listen_port.%d", serverID)
	localPortKey := fmt.Sprintf("proxy.sni_local_port.%d", serverID)
	rulesKey := fmt.Sprintf("proxy.proxy_protocol_rules.%d", serverID)

	// Try per-node keys first, fallback to global
	enabled, _ := p.settingStore.GetByKey(ctx, enabledKey)
	if enabled == "" {
		enabled, _ = p.settingStore.GetByKey(ctx, "proxy.sni_enabled")
	}

	listenPort, _ := p.settingStore.GetByKey(ctx, listenPortKey)
	if listenPort == "" {
		listenPort, _ = p.settingStore.GetByKey(ctx, "proxy.sni_listen_port")
	}
	if listenPort == "" {
		listenPort = "443"
	}

	localPort, _ := p.settingStore.GetByKey(ctx, localPortKey)
	if localPort == "" {
		localPort, _ = p.settingStore.GetByKey(ctx, "proxy.sni_local_port")
	}
	if localPort == "" {
		localPort = "8443"
	}

	rules, _ := p.settingStore.GetByKey(ctx, rulesKey)
	if rules == "" {
		rules, _ = p.settingStore.GetByKey(ctx, "proxy.proxy_protocol_rules")
	}
	if rules == "" {
		rules = `["127.0.0.0/8", "::1/128"]`
	}

	// Build JSON response
	result := fmt.Sprintf(`{"enabled":%s,"listen_port":"%s","local_port":"%s","proxy_protocol_rules":%s}`,
		enabled, listenPort, localPort, rules)
	return result, nil
}

// NewPortalEngine creates a separate Gin engine for portal-only serving on a dedicated port.
// This allows the portal to be accessed at http://domain:port/ (root path, no /p prefix).
// On the main panel the portal is served at /p to avoid collision with the admin SPA route /portal.
func NewPortalEngine(database *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.MaintenanceGuard())

	// Repositories
	systemSettingRepo := repository.NewSystemSettingRepository(database)
	portalTmplRepo := repository.NewPortalTemplateRepository(database)
	portalTmplSvc := portalSvc.NewService(portalTmplRepo)
	portalTmplSvc.SeedBuiltInTemplate(context.Background())

	systemSettingSvc := systemSvc.NewSettingService(systemSettingRepo, nil)
	portalTmplSvc.WithSettingStore(systemSettingSvc)

	// Client auth handler
	clientRepo := repository.NewClientRepository(database)
	planTmplRepo := repository.NewPlanTemplateRepository(database)
	subRepo := repository.NewSubscriptionRepository(database)
	clientauthSvcInst := clientauthSvc.NewService(clientRepo, planTmplRepo, subRepo, systemSettingRepo, os.Getenv("JWT_SECRET"))
	clientauthH := clientauthHandler.NewHandler(clientauthSvcInst)

	// Commerce service (plans, orders, charging, balance)
	commerceSvcInst := clientauthSvc.NewCommerceService(
		database,
		planTmplRepo,
		repository.NewPlanVariantRepository(database),
		repository.NewPlanOrderRepository(database),
		repository.NewPromotionRepository(database),
		clientRepo,
		subRepo,
		repository.NewChargeSiteRepository(database),
		repository.NewChargeOrderRepository(database),
		repository.NewChargeCardRepository(database),
		repository.NewClientBalanceRepository(database),
		repository.NewBalanceTransactionRepository(database),
		systemSettingSvc,
		os.Getenv("JWT_SECRET"),
	)
	commerceH := clientauthHandler.NewCommerceHandler(commerceSvcInst)

	// P1-P3 extended services
	referralSvcInst := clientauthSvc.NewReferralService(
		database, repository.NewReferralCodeRepository(database),
		repository.NewReferralRecordRepository(database),
		repository.NewReferralWithdrawalRepository(database),
		repository.NewClientBalanceRepository(database),
		repository.NewBalanceTransactionRepository(database),
		clientRepo, systemSettingSvc,
	)
	ticketSvcInst := clientauthSvc.NewTicketService(
		repository.NewTicketRepository(database),
		repository.NewTicketMessageRepository(database),
		clientRepo,
	)
	giftSvcInst := clientauthSvc.NewGiftService(
		repository.NewClientGiftRepository(database),
		clientRepo, subRepo, database,
	)
	articleSvcInst := clientauthSvc.NewArticleService(
		repository.NewArticleRepository(database),
	)
	sharedAcctSvcInst := clientauthSvc.NewSharedAccountService(
		repository.NewSharedAccountRepository(database),
		repository.NewSharedAccountRepairLogRepository(database),
		clientRepo,
	)
	ipWhitelistSvcInst := clientauthSvc.NewIPWhitelistService(
		repository.NewIPWhitelistEntryRepository(database),
		clientRepo,
	)
	selfUnbanSvcInst := clientauthSvc.NewSelfUnbanService(
		repository.NewClientSelfUnbanLogRepository(database),
		clientRepo, systemSettingSvc,
	)
	subResetSvcInst := clientauthSvc.NewSubscriptionResetService(subRepo, clientRepo)
	subEndpointRepo2 := repository.NewSubscriptionEndpointRepository(database)

	// Set balance repo on clientauth service for /me extension
	clientauthSvcInst.SetBalanceRepo(repository.NewClientBalanceRepository(database))

	// P1: 注入 ReferralService — 注册时自动绑定邀请关系 + 套餐激活时触发返利
	clientauthSvcInst.SetReferralService(referralSvcInst)

	// Wire node traffic quota service for GET /client-auth/traffic-history
	// (only the DB-backed history query is used; nodeMgr is not required here)
	nodeServerRepoForTH := repository.NewNodeServerRepository(database)
	portalTrafficQuotaSvc := nodeServerSvc.NewNodeTrafficQuotaService(database, nodeServerRepoForTH, nil)
	clientauthSvcInst.SetTrafficQuotaSvc(portalTrafficQuotaSvc)

	commerceSvcInst.SetReferralService(referralSvcInst)

	// 注入 UserSyncer — Portal 引擎退款过期后也需从节点移除用户
	if sharedUserSyncer != nil {
		commerceSvcInst.SetUserSyncer(sharedUserSyncer)
	}

	// 注入 AccountSyncer — Portal 引擎套餐激活后也需创建入站账号
	if sharedAccountSyncer != nil {
		commerceSvcInst.SetAccountSyncer(sharedAccountSyncer)
	}

	// Public maintenance status (no auth, for portal frontend polling)
	r.GET("/api/v1/system/maintenance", func(c *gin.Context) {
		active, message, source := middleware.GetMaintenanceInfo()
		response.OK(c, gin.H{
			"enabled": active,
			"message": message,
			"source":  source,
		})
	})

	// Root path serves the portal page (cached)
	r.GET("/", func(c *gin.Context) {
		html := portalTmplSvc.RenderPage(c.Request.Context())
		c.Data(200, "text/html; charset=utf-8", []byte(html))
	})

	// Static files (qrcode.min.js etc.)
	r.GET("/static/:file", func(c *gin.Context) {
		file := c.Param("file")
		data, err := embed.AdminFSBytes().ReadFile("dist/portal/" + file)
		if err != nil {
			c.Status(404)
			return
		}
		ext := strings.ToLower(filepath.Ext(file))
		ct := "application/octet-stream"
		switch ext {
		case ".js":
			ct = "application/javascript; charset=utf-8"
		case ".css":
			ct = "text/css; charset=utf-8"
		}
		c.Data(200, ct, data)
	})

	// Client auth (public)
	clientAuthGroup := r.Group("/api/v1/client-auth")
	{
		clientAuthGroup.POST("/register", clientauthH.Register)
		clientAuthGroup.POST("/login", clientauthH.Login)
		clientAuthGroup.POST("/refresh", clientauthH.Refresh)
	}

	// Log manage deps (for client log packaging)
	logPolicyRepo := repository.NewLogRotationPolicyRepository(database)
	logJobRepo := repository.NewLogRotationJobRepository(database)
	caddySiteRepo := repository.NewCaddySiteRepository(database)
	logManageSvcInst := logManageSvc.NewService(logPolicyRepo, logJobRepo, systemSettingRepo, caddySiteRepo, "", nil)
	clientLogPkgRepo := repository.NewClientLogPackageRepository(database)
	planSvcInst := planSvc.NewService(
		database, planTmplRepo,
		repository.NewPlanVariantRepository(database),
		repository.NewNodeGroupRepository(database),
		repository.NewNodeGroupMemberRepository(database),
		repository.NewPromotionRepository(database),
		repository.NewPlanOrderRepository(database),
		clientRepo,
		repository.NewInboundConfigRepository(database),
		repository.NewExternalNodeRepository(database),
	)
	inboundConfigRepo := repository.NewInboundConfigRepository(database)
	logManageSvcInst.InitClientLogDeps(clientLogPkgRepo, clientRepo, planSvcInst, inboundConfigRepo, nil)
	clientLogH := clientauthHandler.NewLogHandler(logManageSvcInst)

	portalPublicSubH := subsHandler.NewPublicHandler(
		subRepo, inboundConfigRepo, repository.NewShareDomainConfigRepository(database),
		repository.NewCFIPEntryRepository(database), clientRepo, planSvcInst,
		repository.NewExternalNodeRepository(database),
		repository.NewMihomoConfigRepository(database), repository.NewMihomoRuleRepository(database),
		systemSettingRepo, repository.NewAccessLogRepository(database),
	)
	if tracker := initNetworkTrafficAccounting(subRepo, clientRepo, inboundConfigRepo); tracker != nil {
		portalPublicSubH.SetNetworkTracker(tracker)
	}
	clientExtH := clientextHandler.NewHandler(
		referralSvcInst, ticketSvcInst, giftSvcInst, articleSvcInst,
		sharedAcctSvcInst, ipWhitelistSvcInst, selfUnbanSvcInst, subResetSvcInst,
		clientRepo, repository.NewClientOAuthBindingRepository(database),
		subRepo, subEndpointRepo2, portalPublicSubH, database,
	)

	// Client auth (protected) — guarded by maintenance mode
	clientAuthProtected := r.Group("/api/v1/client-auth")
	clientAuthProtected.Use(middleware.MaintenanceGuard())
	clientAuthProtected.Use(clientauthHandler.ClientAuthMiddleware(clientauthSvcInst))
	{
		clientAuthProtected.GET("/me", clientauthH.GetMe)
		clientAuthProtected.POST("/rotate-token", clientauthH.RotateToken)
		clientAuthProtected.PUT("/settings", clientauthH.UpdateSettings)
		clientAuthProtected.POST("/change-password", clientauthH.ChangePassword)
		clientAuthProtected.GET("/traffic-history", clientauthH.GetTrafficHistory)
		// Client log package endpoints (subscription user log access)
		clientAuthProtected.GET("/logs/types", clientLogH.LogPackageHealthCheck)
		clientAuthProtected.POST("/logs/package", clientLogH.CreateLogPackage)
		clientAuthProtected.GET("/logs/packages", clientLogH.ListLogPackages)
		clientAuthProtected.GET("/logs/packages/:id", clientLogH.GetLogPackage)
		clientAuthProtected.GET("/logs/packages/:id/download", clientLogH.DownloadLogPackage)
		clientAuthProtected.DELETE("/logs/packages/:id", clientLogH.DeleteLogPackage)

		// P0-1: Plan display
		clientAuthProtected.GET("/plans", commerceH.ListPlans)
		clientAuthProtected.GET("/plans/:id", commerceH.GetPlan)

		// Traffic packs (流量叠加包)
		clientAuthProtected.GET("/traffic-packs", commerceH.ListTrafficPacks)
		clientAuthProtected.POST("/traffic-packs/buy", commerceH.BuyTrafficPack)

		// P0-2: Order management
		clientAuthProtected.POST("/orders", commerceH.CreateOrder)
		clientAuthProtected.GET("/orders", commerceH.ListOrders)
		clientAuthProtected.GET("/orders/:id", commerceH.GetOrder)

		// P0-3: Charging system
		clientAuthProtected.GET("/charge/sites", commerceH.ListChargeSites)
		clientAuthProtected.POST("/charge/create", commerceH.CreateChargeOrder)
		clientAuthProtected.GET("/charge/status/:trade_no", commerceH.GetChargeStatus)
		clientAuthProtected.POST("/charge/redeem-card", commerceH.RedeemCard)
		clientAuthProtected.GET("/charge/orders", commerceH.ListChargeOrders)
		clientAuthProtected.GET("/charge/orders/:id", commerceH.GetChargeOrderDetail)

		// P0-4: Balance system
		clientAuthProtected.GET("/balance", commerceH.GetBalance)
		clientAuthProtected.GET("/balance/transactions", commerceH.ListBalanceTransactions)

		// P1-1: Node list (client visible)
		clientAuthProtected.GET("/nodes", clientExtH.ListClientNodes)
		clientAuthProtected.GET("/nodes/:id/config", clientExtH.GetNodeConfig)
		clientAuthProtected.GET("/nodes/:id/link", clientExtH.GetNodeLink)
		clientAuthProtected.GET("/nodes/:id/qrcode", clientExtH.GetNodeQRCode)

		// P1-2: Referral system
		clientAuthProtected.GET("/referral/code", clientExtH.GetReferralCode)
		clientAuthProtected.GET("/referral/stats", clientExtH.GetReferralStats)
		clientAuthProtected.GET("/referral/records", clientExtH.ListReferralRecords)
		clientAuthProtected.GET("/referral/withdrawals", clientExtH.ListReferralWithdrawals)
		clientAuthProtected.POST("/referral/withdraw", clientExtH.RequestWithdrawal)

		// P1-3: Ticket system
		clientAuthProtected.POST("/tickets", clientExtH.CreateTicket)
		clientAuthProtected.GET("/tickets", clientExtH.ListTickets)
		clientAuthProtected.GET("/tickets/:id", clientExtH.GetTicket)
		clientAuthProtected.POST("/tickets/:id/reply", clientExtH.ReplyTicket)
		clientAuthProtected.POST("/tickets/:id/close", clientExtH.CloseTicket)

		// P1-4: Invoices (reuse plan orders) — 与文档设计统一为 /invoices
		clientAuthProtected.GET("/invoices", commerceH.ListOrders)
		clientAuthProtected.GET("/bills", commerceH.ListOrders) // 向后兼容别名

		// P2-1: Gift system
		clientAuthProtected.GET("/gifts", clientExtH.ListGifts)
		clientAuthProtected.POST("/gifts/:id/claim", clientExtH.ClaimGift)

		// P2-2: Articles
		clientAuthProtected.GET("/articles", clientExtH.ListArticles)
		clientAuthProtected.GET("/articles/:slug", clientExtH.GetArticle)

		// P2-3: Subscription reset — 与文档设计统一为 /reset-subscription
		clientAuthProtected.POST("/reset-subscription", clientExtH.ResetSubscription)
		clientAuthProtected.POST("/subscription/reset", clientExtH.ResetSubscription) // 向后兼容别名

		// P3-1: Shared accounts
		clientAuthProtected.GET("/shared-accounts", clientExtH.ListSharedAccounts)
		clientAuthProtected.GET("/shared-accounts/:id", clientExtH.GetSharedAccount)
		clientAuthProtected.POST("/shared-accounts/:id/repair", clientExtH.RequestRepair)

		// P3-2: IP whitelist — 与文档设计统一为 /settings/whitelist
		clientAuthProtected.GET("/settings/whitelist", clientExtH.ListIPWhitelist)
		clientAuthProtected.POST("/settings/auto-add-ip", clientExtH.AutoAddIP)
		clientAuthProtected.DELETE("/settings/whitelist/:id", clientExtH.RemoveIPWhitelist)
		clientAuthProtected.GET("/ip-whitelist", clientExtH.ListIPWhitelist)          // 向后兼容别名
		clientAuthProtected.POST("/ip-whitelist/auto-add", clientExtH.AutoAddIP)      // 向后兼容别名
		clientAuthProtected.DELETE("/ip-whitelist/:id", clientExtH.RemoveIPWhitelist) // 向后兼容别名

		// P3-3: Self-unban
		clientAuthProtected.POST("/self-unban", clientExtH.SelfUnban)

		// P1-4b: Invoice detail (reuse order query)
		clientAuthProtected.GET("/invoices/:id", commerceH.GetOrder)

		// P3-17: Time check
		clientAuthProtected.GET("/checktime", clientExtH.CheckTime)

		// P3-19: Profile management
		clientAuthProtected.GET("/profile", clientExtH.GetProfile)
		clientAuthProtected.PUT("/profile", clientExtH.UpdateProfile)
		clientAuthProtected.GET("/connection-info", clientExtH.GetConnectionInfo)
		clientAuthProtected.POST("/profile/bind-oauth", clientExtH.BindOAuth)
		clientAuthProtected.DELETE("/profile/unbind-oauth/:provider", clientExtH.UnbindOAuth)

		// P3-20: Subscription formats
		clientAuthProtected.GET("/subscription-formats", clientExtH.ListSubscriptionFormats)
		clientAuthProtected.GET("/subscription-link", clientExtH.GetSubscriptionLink)
		clientAuthProtected.GET("/subscription-link/backup", clientExtH.GetBackupSubscriptionLink)

		// P3-22: Shared account apply
		clientAuthProtected.POST("/shared-accounts/:id/apply", clientExtH.ApplySharedAccount)
	}

	return r
}

// containsInt64 checks if a slice contains a specific int64 value.
func containsInt64(slice []int64, val int64) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
