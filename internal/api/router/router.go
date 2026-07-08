package router

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	extNodeHandler "corefusion/internal/api/handlers"
	agentHandler "corefusion/internal/api/handlers/agent"
	authHandler "corefusion/internal/api/handlers/auth"
	backupsHandler "corefusion/internal/api/handlers/backups"

	certsHandler "corefusion/internal/api/handlers/certificates"
	cfipHandler "corefusion/internal/api/handlers/cfip"
	clientHandler "corefusion/internal/api/handlers/client"
	clientauthHandler "corefusion/internal/api/handlers/clientauth"
	coreconfigHandler "corefusion/internal/api/handlers/coreconfig"
	geoRuleHandler "corefusion/internal/api/handlers/georule"
	inboundconfigHandler "corefusion/internal/api/handlers/inboundconfig"
	logManageHandler "corefusion/internal/api/handlers/logmanage"
	metricsHandler "corefusion/internal/api/handlers/metrics"
	mihomoHandler "corefusion/internal/api/handlers/mihomo"
	nodesHandler "corefusion/internal/api/handlers/nodes"
	nodeServerHandler "corefusion/internal/api/handlers/nodeserver"
	outboundconfigHandler "corefusion/internal/api/handlers/outboundconfig"
	planHandler "corefusion/internal/api/handlers/plan"
	securityHandler "corefusion/internal/api/handlers/security"
	servicesHandler "corefusion/internal/api/handlers/services"
	sharedomainconfigHandler "corefusion/internal/api/handlers/sharedomainconfig"
	subendHandler "corefusion/internal/api/handlers/subendpoint"
	subfmtHandler "corefusion/internal/api/handlers/subformat"
	subsHandler "corefusion/internal/api/handlers/subscriptions"
	systemHandler "corefusion/internal/api/handlers/system"
	usersHandler "corefusion/internal/api/handlers/users"
	warpHandler "corefusion/internal/api/handlers/warp"
	xrayControlHandler "corefusion/internal/api/handlers/xraycontrol"
	xrayStatsHandler "corefusion/internal/api/handlers/xraystats"
	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/app/embed"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/runtime/firewall"
	binariesSvc "corefusion/internal/service/binaries"
	nodeServerSvc "corefusion/internal/service/nodeserver"
	"corefusion/internal/runtime/cert"
	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/runtime/process"
	authService "corefusion/internal/service/auth"
	backupService "corefusion/internal/service/backup"
	certService "corefusion/internal/service/certificate"
	cfipSvc "corefusion/internal/service/cfip"
	clientSvc "corefusion/internal/service/client"
	clientauthSvc "corefusion/internal/service/clientauth"
	configsyncSvc "corefusion/internal/service/configsync"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	geoRuleSvc "corefusion/internal/service/georule"
	inboundconfigSvc "corefusion/internal/service/inboundconfig"
	logManageSvc "corefusion/internal/service/logmanage"
	mihomoSvc "corefusion/internal/service/mihomo"
	nodeService "corefusion/internal/service/node"
	outboundconfigSvc "corefusion/internal/service/outboundconfig"
	planSvc "corefusion/internal/service/plan"
	proxySvc "corefusion/internal/service/proxy"
	svcService "corefusion/internal/service/service"
	sharedomainconfigSvc "corefusion/internal/service/sharedomainconfig"
	subendSvc "corefusion/internal/service/subendpoint"
	subfmtSvc "corefusion/internal/service/subformat"
	subService "corefusion/internal/service/subscription"
	subRegion "corefusion/internal/service/subscription"
	systemSvc "corefusion/internal/service/system"
	trafficsyncSvc "corefusion/internal/service/trafficsync"
	userService "corefusion/internal/service/user"
	warpSvc "corefusion/internal/service/warp"
	"corefusion/internal/store/repository"
)

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

// configSyncHookAdapter adapts configsync.Service to the ConfigSyncHook interface
// used by inboundconfig and outboundconfig handlers.
type configSyncHookAdapter struct {
	syncSvc *configsyncSvc.Service
}

func (a *configSyncHookAdapter) SyncServerConfig(_ context.Context, serverID int64, coreType string) {
	if a.syncSvc != nil {
		if err := a.syncSvc.SyncNow(context.Background()); err != nil {
			log.Printf("[router] configSyncHook: SyncNow failed for server %d: %v", serverID, err)
		}
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
	planRepo := repository.NewPlanRepository(database)
	planNodeRepo := repository.NewPlanNodeRepository(database)
	clientRepo := repository.NewClientRepository(database)

	coreConfigRepo := repository.NewCoreConfigRepository(database)
	coreConfigSvc := coreconfigSvc.NewService(coreConfigRepo)
	if err := coreConfigSvc.SeedDefaults(context.Background(), 1); err != nil {
		log.Printf("WARNING: seed core configs failed: %v", err)
	}

	// Inbound config service (separate table for inbound configurations)
	inboundConfigRepo := repository.NewInboundConfigRepository(database)
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

	// Initialize services
	authSvc := authService.NewService(userRepo, jwtSecret)
	userSvc := userService.NewService(userRepo)
	nodeSvc := nodeService.NewService(nodeRepo)
	svcSvc := svcService.NewService(svcRepo, endpointRepo)
	subSvc := subService.NewService(subRepo, accessLogRepo, rotationRepo, clientRepo, planNodeRepo)
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
	dnsChannelSvc := certService.NewDNSChannelService(dnsChannelRepo)
	acmeEmailSvc := certService.NewACMEEmailService(acmeAccountRepo)
	notifier := systemSvc.NewNotifier(systemSettingSvc)
	backupSvcInst := backupService.NewService(backupRepo, dataDir)
	_ = notifier // TODO: integrate with scheduler for template notifications
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

	// Wire config generators so auto-start uses the new separated managers
	proxyMgr.SetCaddyConfigGen(caddyMgr.GenerateConfig)
	proxyMgr.SetNginxConfigGen(nginxMgr.GenerateConfig)

	// Seed default geo rules for local node (server_id=1)
	geoRuleSvcInst.SeedDefaults(nil, 1)

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
	clientSvcInst := clientSvc.NewService(clientRepo, planRepo, planNodeRepo, subRepo, inboundConfigRepo, inboundMgr)

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
	clientauthSvcInst := clientauthSvc.NewService(clientRepo, planRepo, subRepo, systemSettingSvc, jwtSecret)

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
	extNodeRepo := repository.NewExternalNodeRepository(database)

	// Plan v2: templates, variants, node groups, promotions, orders
	planTmplRepo := repository.NewPlanTemplateRepository(database)
	planVariantRepo := repository.NewPlanVariantRepository(database)
	nodeGroupRepo := repository.NewNodeGroupRepository(database)
	nodeGroupMemberRepo := repository.NewNodeGroupMemberRepository(database)
	promoRepo := repository.NewPromotionRepository(database)
	planOrderRepo := repository.NewPlanOrderRepository(database)
	planSvcInst := planSvc.NewService(database, planTmplRepo, planVariantRepo, nodeGroupRepo, nodeGroupMemberRepo, promoRepo, planOrderRepo, clientRepo, inboundConfigRepo, extNodeRepo)
	planH := planHandler.NewHandler(planSvcInst)

	publicSubH := subsHandler.NewPublicHandler(subRepo, inboundConfigRepo, shareDomainRepo, cfIPEntryRepo, clientRepo, planRepo, planNodeRepo, extNodeRepo, mihomoConfigRepo, mihomoRuleRepo, systemSettingRepo, accessLogRepo)
	clientH := clientHandler.NewHandler(clientSvcInst)
	clientauthH := clientauthHandler.NewHandler(clientauthSvcInst)
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
	}
	warpH := warpHandler.NewHandler(warpSvcInst)
	certChannelH := certsHandler.NewChannelHandler(dnsChannelSvc)
	certEmailH := certsHandler.NewEmailHandler(acmeEmailSvc)
	logManageH := logManageHandler.NewHandler(logManageSvc)

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
		} else if strings.HasPrefix(path, "/portal/") && !strings.HasSuffix(path, "/portal/") {
			// Serve portal static files (e.g. qrcode.min.js) from embed
			filePath := "dist/portal" + strings.TrimPrefix(path, "/portal")
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
			c.Data(200, "text/html; charset=utf-8", data)
			return
		}
		c.Status(404)
	})

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Auth (public)
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/login", authH.Login)
			authGroup.POST("/refresh", authH.Refresh)
			authGroup.POST("/logout", authH.Logout)
			authGroup.GET("/me", authH.Me)
			authGroup.POST("/change-password", authH.ChangePassword)
		}

		// Protected routes
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// Users
			usersGroup := protected.Group("/users")
			{
				usersGroup.GET("", userH.List)
				usersGroup.POST("", userH.Create)
				usersGroup.PUT("/:id", userH.Update)
				usersGroup.DELETE("/:id", userH.Delete)
			}

			// Nodes
			nodesGroup := protected.Group("/nodes")
			{
				nodesGroup.GET("", nodeH.List)
				nodesGroup.POST("", nodeH.Create)
				nodesGroup.GET("/:id", nodeH.GetByID)
				nodesGroup.PUT("/:id", nodeH.Update)
				nodesGroup.DELETE("/:id", nodeH.Delete)
			}

			// Inbound Configs (moved under /node-servers/:id/)
			inboundCfgH := inboundconfigHandler.NewHandler(inboundConfigSvc)

			// Outbound Configs (moved under /node-servers/:id/)
			outboundCfgH := outboundconfigHandler.NewHandler(outboundConfigSvc)

			// Inject config sync hook so inbound/outbound changes trigger
			// full config sync to remote nodes.
			syncHook := &configSyncHookAdapter{syncSvc: configSyncSvc}
			inboundCfgH.SetSyncHook(syncHook)
			outboundCfgH.SetSyncHook(syncHook)

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

			// Plans
			plansGroup := protected.Group("/plans")
			{
				plansGroup.GET("", clientH.ListPlans)
				plansGroup.POST("", clientH.CreatePlan)
				plansGroup.GET("/:id", clientH.GetPlan)
				plansGroup.PUT("/:id", clientH.UpdatePlan)
				plansGroup.DELETE("/:id", clientH.DeletePlan)
				plansGroup.POST("/:id/nodes", clientH.AddPlanNode)
				plansGroup.DELETE("/:id/nodes/:nid", clientH.DeletePlanNode)
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
				planTmplGroup.GET("/:id/variants", planH.ListVariants)
				planTmplGroup.POST("/:id/variants", planH.CreateVariant)
			}

			// Plan Variants (套餐规格)
			planVariantGroup := protected.Group("/plan-variants")
			{
				planVariantGroup.PUT("/:id", planH.UpdateVariant)
				planVariantGroup.DELETE("/:id", planH.DeleteVariant)
			}

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
			nodeMgrForPhase3 := nodeMgrForCRL // reuse the same manager

			failoverMgr := agent.NewFailoverManager(nodeServerRepo, nodeMgrForPhase3)
			mirrorSyncMgr := agent.NewMirrorSyncManager(nodeServerRepo, inboundRepoForNodeServer, nodeMgrForPhase3)
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
			speedTestMgr := agent.NewSpeedTestManager(nodeServerRepo, nodeMgrForPhase3)
			loadBalancer := agent.NewLoadBalancer(nodeServerRepo, nodeMgrForPhase3).WithSpeedTest(speedTestMgr)
			agentUpdater := agent.NewAgentUpdater(nodeServerRepo, nodeMgrForPhase3).WithDataDir(dataDir)

			nodeMgrForPhase3.SetMetricsCollector(metricsCollector)

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
				).WithCertificateProvider(certRepoForNodeServer, nodeServerRepo).WithProxySiteProvider(proxySiteRepo, caddySiteRepo, nginxSiteRepo),
			).WithConfigProviders(coreConfigSvc, inboundConfigSvc, outboundConfigSvc)
			nodeMgrForPhase3.SetDataSyncProvider(dataSyncProvider)
nodeMgrForPhase3.SetProxyCertCollector(dataSyncProvider)

			nodeServerSvcInst.
				WithMetricsCollector(metricsCollector).
				WithSpeedTestManager(speedTestMgr).
				WithLoadBalancer(loadBalancer).
				WithAgentUpdater(agentUpdater)

			// Supplementary repos wiring
			auditLogRepo := repository.NewNodeAuditLogRepository(database)
			tagRepo := repository.NewServerTagRepository(database)
			distQueueRepo := repository.NewCertDistributionQueueRepository(database)
			crlRepoForSvc := revocationRepo // reuse the already-created revocation repo
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

		// Wire up NodeOperator so proxy/caddy/nginx handlers can route
		// operations (start/stop/restart/reload, config sync, status)
		// to remote nodes via gRPC.
		proxyH.SetNodeOperator(nodeServerSvcInst)
		caddyH.SetNodeOperator(nodeServerSvcInst)
		nginxH.SetNodeOperator(nodeServerSvcInst)
		processH.SetNodeOperator(nodeServerSvcInst)
			nodeServersGroup := protected.Group("/node-servers")
			{
				nodeServersGroup.GET("", nodeServerH.List)
				nodeServersGroup.POST("", nodeServerH.Create)
				nodeServersGroup.GET("/ca-cert", nodeServerH.GetCACert)
				nodeServersGroup.POST("/distribute-cert", nodeServerH.DistributeCertificateToAll)
				nodeServersGroup.GET("/:id", nodeServerH.GetByID)
				nodeServersGroup.PUT("/:id", nodeServerH.Update)
				nodeServersGroup.DELETE("/:id", nodeServerH.Delete)
				nodeServersGroup.POST("/:id/bootstrap-token", nodeServerH.GenerateBootstrapToken)
				nodeServersGroup.POST("/:id/revoke-cert", nodeServerH.RevokeAgentCertificate)
				nodeServersGroup.POST("/:id/distribute-cert", nodeServerH.DistributeCertificate)
				// Phase 3: Node Types & Chain Proxy
				nodeServersGroup.GET("/:id/failover-status", nodeServerH.GetFailoverStatus)
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
				nodeServersGroup.POST("/:id/control", nodeServerH.ControlCore)
				nodeServersGroup.POST("/:id/sync", nodeServerH.SyncServerConfig)
				nodeServersGroup.GET("/:id/bootstrap-cmd", nodeServerH.GetBootstrapCommand)
				nodeServersGroup.PUT("/:id/node-type", nodeServerH.UpdateNodeType)
				nodeServersGroup.POST("/:id/failover", nodeServerH.TriggerServerFailover)
				nodeServersGroup.GET("/:id/certificates", nodeServerH.ListServerCertificates)
				nodeServersGroup.POST("/:id/certificates/:cid", nodeServerH.AssociateCertificate)
				nodeServersGroup.DELETE("/:id/certificates/:cid", nodeServerH.DissociateCertificate)

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

			// Inbound Configs (per node)
			nodeServersGroup.GET("/:id/inbound-configs", inboundCfgH.List)
			nodeServersGroup.POST("/:id/inbound-configs", inboundCfgH.Create)
			nodeServersGroup.GET("/:id/inbound-configs/:cfgId", inboundCfgH.Get)
			nodeServersGroup.PUT("/:id/inbound-configs/:cfgId", inboundCfgH.Update)
			nodeServersGroup.DELETE("/:id/inbound-configs/:cfgId", inboundCfgH.Delete)
			nodeServersGroup.POST("/:id/inbound-configs/:cfgId/users/:email/pause", inboundCfgH.PauseUser)
			nodeServersGroup.POST("/:id/inbound-configs/:cfgId/users/:email/resume", inboundCfgH.ResumeUser)

			// Outbound Configs (per node)
			nodeServersGroup.GET("/:id/outbound-configs", outboundCfgH.List)
			nodeServersGroup.POST("/:id/outbound-configs", outboundCfgH.Create)
			nodeServersGroup.GET("/:id/outbound-configs/:cfgId", outboundCfgH.Get)
			nodeServersGroup.PUT("/:id/outbound-configs/:cfgId", outboundCfgH.Update)
			nodeServersGroup.DELETE("/:id/outbound-configs/:cfgId", outboundCfgH.Delete)

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

			// CA Center (separate group, reuses nodeServerH)
			caCenterGroup := protected.Group("/ca")
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

			// Node Audit Logs (separate group, reuses nodeServerH)
			nodeAuditGroup := protected.Group("/node-audit-logs")
			{
				nodeAuditGroup.GET("", nodeServerH.ListNodeAuditLogs)
				nodeAuditGroup.GET("/export", nodeServerH.ExportNodeAuditLogs)
			}

			// Certificates
			certsGroup := protected.Group("/certificates")
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
				certsGroup.POST("/:id/deploy", certH.Deploy)
			}

			// DNS Channels
			certChannelsGroup := protected.Group("/cert-channels")
			{
				certChannelsGroup.GET("", certChannelH.List)
				certChannelsGroup.POST("", certChannelH.Create)
				certChannelsGroup.GET("/providers", certChannelH.Providers)
				certChannelsGroup.PUT("/:id", certChannelH.Update)
				certChannelsGroup.DELETE("/:id", certChannelH.Delete)
			}

			// ACME Accounts
			acmeAccountsGroup := protected.Group("/acme-accounts")
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

			// Data sync: push full DB tables to remote Agent (for Agent startup / manual refresh)
			nodeServersGroup.POST("/:id/sync-data", func(c *gin.Context) {
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
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				c.JSON(200, resp)
			})

		// Geo Rules (moved under /node-servers/:id/geo-rules)

			// Metrics
			metricsGroup := protected.Group("/metrics")
			{
				metricsGroup.GET("/overview", metricsH.Overview)
				metricsGroup.GET("/services", metricsH.ServiceMetrics)
				metricsGroup.GET("/nodes", metricsH.NodeMetrics)
				metricsGroup.GET("/certificates", metricsH.CertMetrics)
				metricsGroup.GET("/connection-failures", metricsH.ConnectionFailures)
				metricsGroup.GET("/udp-quic", metricsH.UDPQUICMetrics)
			}

			// Security
			securityGroup := protected.Group("/security")
			{
				securityGroup.GET("/audit-events", securityH.ListAuditEvents)
				securityGroup.GET("/alerts", securityH.ListAlerts)
				securityGroup.GET("/alert-channels", securityH.ListAlertChannels)
				securityGroup.POST("/alert-channels", securityH.CreateAlertChannel)
				securityGroup.PUT("/alert-channels/:id", securityH.UpdateAlertChannel)
				securityGroup.POST("/alerts/:id/resolve", securityH.ResolveAlert)
			}

			// Backups
			backupsGroup := protected.Group("/backups")
			{
				backupsGroup.GET("/configs", backupsH.ListConfigBackups)
				backupsGroup.POST("/configs", backupsH.CreateConfigBackup)
				backupsGroup.POST("/configs/restore", backupsH.RestoreConfigBackup)
				backupsGroup.GET("/database", backupsH.ListDBBackups)
				backupsGroup.POST("/database", backupsH.CreateDBBackup)
				backupsGroup.POST("/database/restore", backupsH.RestoreDBBackup)
				backupsGroup.GET("/retention-policies", backupsH.ListRetentionPolicies)
				backupsGroup.POST("/retention-policies", backupsH.CreateRetentionPolicy)
				backupsGroup.PUT("/retention-policies/:id", backupsH.UpdateRetentionPolicy)
				backupsGroup.DELETE("/retention-policies/:id", backupsH.DeleteRetentionPolicy)
				backupsGroup.GET("/cleanup-jobs", backupsH.ListCleanupJobs)
				backupsGroup.POST("/cleanup-jobs", backupsH.CreateCleanupJob)
				backupsGroup.GET("/snapshots/:id/validation", backupsH.GetSnapshotValidation)
				backupsGroup.POST("/snapshots/:id/validate", backupsH.ValidateSnapshot)

				// Full backup
				backupsGroup.POST("/create", backupsH.CreateFullBackup)
				backupsGroup.GET("/list", backupsH.ListFullBackups)
				backupsGroup.GET("/:id/download", backupsH.DownloadBackup)
				backupsGroup.POST("/:id/restore", backupsH.RestoreBackup)
				backupsGroup.DELETE("/:id", backupsH.DeleteBackup)
				backupsGroup.POST("/upload-restore", backupsH.UploadRestore)
				backupsGroup.POST("/:id/upload-ftp", backupsH.UploadToFTP)
				backupsGroup.POST("/:id/upload-s3", backupsH.UploadToS3)
				backupsGroup.POST("/:id/upload-webdav", backupsH.UploadToWebDAV)

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
			}

			// System
			systemGroup := protected.Group("/system")
			{
				systemGroup.GET("/stats", systemH.GetSystemStats)
				systemGroup.GET("/bootstrap-state", systemH.GetBootstrapState)
				systemGroup.PUT("/bootstrap-finish", systemH.FinishBootstrap)
				systemGroup.GET("/releases", systemH.ListReleases)
				systemGroup.POST("/update", systemH.SystemUpdate)
				systemGroup.POST("/rollback", systemH.SystemRollback)
				systemGroup.GET("/update-jobs", systemH.ListUpdateJobs)
				systemGroup.GET("/panel-config", systemH.GetPanelConfig)
				systemGroup.PUT("/panel-config", systemH.UpdatePanelConfig)
				systemGroup.GET("/notification-config", systemH.GetNotificationConfig)
				systemGroup.PUT("/notification-config", systemH.UpdateNotificationConfig)
				systemGroup.POST("/test-smtp", systemH.TestSMTP)
				systemGroup.POST("/test-telegram", systemH.TestTelegram)
				systemGroup.POST("/restart", systemH.Restart)
				systemGroup.POST("/migrate-data", systemH.MigrateData)
				systemGroup.POST("/upload-mtls-ca", systemH.UploadMTLSCA)
				systemGroup.GET("/github-config", systemH.GetGitHubConfig)
				systemGroup.PUT("/github-config", systemH.UpdateGitHubConfig)
				systemGroup.GET("/subscription-billing-templates", systemH.GetSubscriptionBillingTemplates)
				systemGroup.PUT("/subscription-billing-templates", systemH.UpdateSubscriptionBillingTemplates)
				systemGroup.GET("/notification-templates", systemH.ListTemplates)
				systemGroup.POST("/notification-templates", systemH.CreateTemplate)
				systemGroup.PUT("/notification-templates/:id", systemH.UpdateTemplate)
				systemGroup.DELETE("/notification-templates/:id", systemH.DeleteTemplate)
				systemGroup.GET("/reality-targets", systemH.GetRealityTargets)
				systemGroup.PUT("/reality-targets", systemH.UpdateRealityTargets)
				systemGroup.POST("/reality-targets/reset", systemH.ResetRealityTargets)
				systemGroup.GET("/config-file/:coreType", systemH.GetConfigFile)
			}

			// Proxy/Caddy/Nginx management routes moved under /node-servers/:id/

			// Log management
			logManageGroup := protected.Group("/log-manage")
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
				logManageGroup.POST("/log-output/takeover/:type", logManageH.TakeoverStdout)
				logManageGroup.POST("/log-output/release/:type", logManageH.ReleaseStdout)
				logManageGroup.GET("/proxy/:type/stdout-log", logManageH.ProxyStdoutLog)
				logManageGroup.GET("/proxy/:type/stdout-stream", logManageH.ProxyStdoutStream)
				logManageGroup.GET("/writers", logManageH.GetWriterStatus)
				logManageGroup.POST("/writers/:type/start", logManageH.StartWriter)
				logManageGroup.POST("/writers/:type/stop", logManageH.StopWriter)
				logManageGroup.PUT("/writers/:type/config", logManageH.UpdateWriterConfig)
				logManageGroup.GET("/writers/:type/files", logManageH.ListWriterFiles)
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
		protected.GET("/agent/version", agentH.GetAgentVersion)
		protected.GET("/agent/releases", agentH.GetAgentReleases)

		// Placeholder routes for remaining modules
		registerPlaceholderRoutes(protected)
		}
	}

	// Public subscription endpoint
	r.GET("/sub/:token", publicSubH.ServeSubscription)

	// Portal page
	r.GET("/portal", func(c *gin.Context) {
		data, err := embed.AdminFSBytes().ReadFile("dist/portal/index.html")
		if err != nil {
			c.Status(404)
			return
		}
		c.Data(200, "text/html; charset=utf-8", data)
	})

	// Client auth (public)
	clientAuthGroup := r.Group("/api/v1/client-auth")
	{
		clientAuthGroup.POST("/register", clientauthH.Register)
		clientAuthGroup.POST("/login", clientauthH.Login)
		clientAuthGroup.POST("/refresh", clientauthH.Refresh)
	}

	// Public subscription endpoints (for portal)
	r.GET("/api/v1/public/subscription-endpoints", subendH.List)

	// Public settings (for portal)
	r.GET("/api/v1/public/settings", func(c *gin.Context) {
		registrationEnabled, _ := systemSettingSvc.GetByKey(c.Request.Context(), "panel.registration_enabled")
		response.OK(c, gin.H{
			"registration_enabled": registrationEnabled == "true",
		})
	})

	// Client auth (protected)
	clientAuthProtected := r.Group("/api/v1/client-auth")
	clientAuthProtected.Use(clientauthHandler.ClientAuthMiddleware(clientauthSvcInst))
	{
		clientAuthProtected.GET("/me", clientauthH.GetMe)
		clientAuthProtected.POST("/rotate-token", clientauthH.RotateToken)
		clientAuthProtected.PUT("/settings", clientauthH.UpdateSettings)
		clientAuthProtected.POST("/change-password", clientauthH.ChangePassword)
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

	// Set callback to reload proxy config when log output mode is switched
	logManageSvc.SetOnModeSwitch(func(proxyType string) {
		log.Printf("router: reloading %s config after log mode switch", proxyType)
		bgCtx := context.Background()
		switch proxyType {
		case "nginx":
			nginxMgr.SaveAndApply(bgCtx)
			proxyMgr.ReloadProxy(bgCtx, proxySvc.ProxyNginx)
		case "caddy":
			caddyMgr.SaveAndApply(bgCtx)
			proxyMgr.ReloadProxy(bgCtx, proxySvc.ProxyCaddy)
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

	// Start traffic sync service (periodically syncs traffic stats to DB)
	trafficSyncSvc.Start()

	return r
}

// registerPlaceholderRoutes registers placeholder routes for modules not yet implemented.
func registerPlaceholderRoutes(rg *gin.RouterGroup) {
	// Services (access policies, ip bans)
	services := rg.Group("/services")
	{
		services.GET("/:id/logs", placeholder("get service logs"))
		services.GET("/:id/access-policies", placeholder("list access policies"))
		services.POST("/:id/access-policies", placeholder("create access policy"))
		services.PUT("/:id/access-policies/:policyId", placeholder("update access policy"))
		services.DELETE("/:id/access-policies/:policyId", placeholder("delete access policy"))
		services.GET("/:id/ip-bans", placeholder("list ip bans"))
		services.POST("/:id/ip-bans", placeholder("create ip ban"))
		services.DELETE("/:id/ip-bans/:banId", placeholder("delete ip ban"))
	}

	// Runtime
	runtime := rg.Group("/runtime")
	{
		runtime.GET("/overview", placeholder("runtime overview"))
		runtime.GET("/snapshots", placeholder("list snapshots"))
		runtime.GET("/apply-jobs", placeholder("list apply jobs"))
		runtime.GET("/apply-jobs/:id", placeholder("get apply job"))
		runtime.POST("/snapshots", placeholder("create snapshot"))
		runtime.POST("/dry-run", placeholder("dry run"))
		runtime.POST("/apply", placeholder("apply"))
		runtime.POST("/apply-jobs/:id/retry", placeholder("retry job"))
		runtime.POST("/apply-jobs/:id/cancel", placeholder("cancel job"))
		runtime.POST("/rollback", placeholder("rollback"))
		runtime.POST("/disable-proxy", placeholder("disable proxy"))
	}

	// Server Share
	share := rg.Group("/server-share")
	{
		share.GET("/profiles", placeholder("list share profiles"))
		share.POST("/profiles", placeholder("create share profile"))
		share.GET("/profiles/:id", placeholder("get share profile"))
		share.PUT("/profiles/:id", placeholder("update share profile"))
		share.DELETE("/profiles/:id", placeholder("delete share profile"))
		share.GET("/profiles/:id/revisions", placeholder("list revisions"))
		share.GET("/profiles/:id/events", placeholder("list events"))
		share.POST("/profiles/:id/preview", placeholder("preview"))
		share.POST("/profiles/:id/publish", placeholder("publish"))
		share.POST("/profiles/:id/revoke", placeholder("revoke"))
		share.GET("/profiles/:id/qrcode", placeholder("get qrcode"))
		share.GET("/profiles/:id/export", placeholder("export"))
	}

	// Traffic Quotas
	traffic := rg.Group("/traffic")
	{
		traffic.GET("/quotas", placeholder("list quotas"))
		traffic.POST("/quotas", placeholder("create quota"))
		traffic.PUT("/quotas/:id", placeholder("update quota"))
		traffic.GET("/quotas/:id/usage", placeholder("get quota usage"))
		traffic.GET("/usage-summary", placeholder("get usage summary"))
		traffic.GET("/sessions", placeholder("list sessions"))
		traffic.GET("/reset-logs", placeholder("list reset logs"))
		traffic.POST("/quotas/:id/reset", placeholder("reset quota"))
	}

	// Client Config
	client := rg.Group("/client")
	{
		client.GET("/merge-profiles", placeholder("list merge profiles"))
		client.POST("/merge-profiles", placeholder("create merge profile"))
		client.GET("/merge-profiles/:id", placeholder("get merge profile"))
		client.PUT("/merge-profiles/:id", placeholder("update merge profile"))
		client.POST("/merge-profiles/:id/sync", placeholder("sync merge profile"))
		client.POST("/merge-profiles/:id/render", placeholder("render merge profile"))
		client.GET("/merge-profiles/:id/generated-profiles", placeholder("list generated profiles"))
		client.POST("/merge-profiles/:id/sources", placeholder("add source"))
		client.PUT("/merge-profiles/:id/sources/:sourceId", placeholder("update source"))
		client.POST("/merge-profiles/:id/sources/:sourceId/sync", placeholder("sync source"))
		client.POST("/merge-profiles/:id/apply", placeholder("apply profile"))

		client.GET("/route-profiles", placeholder("list route profiles"))
		client.POST("/route-profiles", placeholder("create route profile"))
		client.GET("/route-profiles/:id", placeholder("get route profile"))
		client.PUT("/route-profiles/:id", placeholder("update route profile"))
		client.GET("/route-profiles/:id/rule-sources", placeholder("list rule sources"))
		client.POST("/route-profiles/:id/rule-sources", placeholder("create rule source"))
		client.POST("/route-profiles/:id/rule-sources/:sourceId/sync", placeholder("sync rule source"))
		client.POST("/route-profiles/:id/rules/preview", placeholder("preview rules"))
		client.POST("/route-profiles/:id/rules/apply", placeholder("apply rules"))
		client.GET("/route-profiles/:id/rule-sync-jobs", placeholder("list rule sync jobs"))
		client.GET("/route-profiles/:id/route-apply-jobs", placeholder("list route apply jobs"))
	}

	// Routing Regions
	regions := rg.Group("/routing/regions")
	{
		regions.GET("", placeholder("list regions"))
		regions.POST("", placeholder("create region"))
		regions.PUT("/:id", placeholder("update region"))
		regions.DELETE("/:id", placeholder("delete region"))
	}

	// Reverse Proxies
	proxies := rg.Group("/reverse-proxies")
	{
		proxies.GET("", placeholder("list reverse proxies"))
		proxies.POST("", placeholder("create reverse proxy"))
		proxies.PUT("/:id", placeholder("update reverse proxy"))
		proxies.POST("/:id/reload", placeholder("reload reverse proxy"))
		proxies.POST("/:id/restart", placeholder("restart reverse proxy"))
		proxies.GET("/:id/config", placeholder("get reverse proxy config"))
	}

	// Route Runtime (OpenWrt)
	routeRuntime := rg.Group("/route-runtime")
	{
		routeRuntime.GET("/openwrt/integration-status/:routeProfileID", placeholder("get integration status"))
		routeRuntime.POST("/openwrt/initialize-includes", placeholder("initialize includes"))
		routeRuntime.POST("/openwrt/repair-includes", placeholder("repair includes"))
		routeRuntime.POST("/openwrt/dry-run", placeholder("openwrt dry run"))
		routeRuntime.POST("/openwrt/validate", placeholder("openwrt validate"))
		routeRuntime.POST("/openwrt/apply", placeholder("openwrt apply"))
		routeRuntime.POST("/openwrt/rollback", placeholder("openwrt rollback"))
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
		"/nodes",
		"/services",
		"/subscriptions",
		"/subscription-formats",
		"/subscription-endpoints",
		"/clients",
		"/certificates",
		"/users",
		"/log-manage",
		"/settings",
		"/portal",
		"/plans",
	}

	for _, route := range knownRoutes {
		if path == route {
			return true
		}
	}

	return false
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

	subRepo := repository.NewSubscriptionRepository(database)
	inboundRepo := repository.NewInboundConfigRepository(database)
	shareDomainRepo := repository.NewShareDomainConfigRepository(database)
	cfIPEntryRepo := repository.NewCFIPEntryRepository(database)
	clientRepo := repository.NewClientRepository(database)
	planRepo := repository.NewPlanRepository(database)
	planNodeRepo := repository.NewPlanNodeRepository(database)
	accessLogRepo := repository.NewAccessLogRepository(database)
	mihomoConfigRepo2 := repository.NewMihomoConfigRepository(database)
	mihomoRuleRepo2 := repository.NewMihomoRuleRepository(database)
	extNodeRepo2 := repository.NewExternalNodeRepository(database)
	systemSettingRepo := repository.NewSystemSettingRepository(database)

	publicSubH := subsHandler.NewPublicHandler(subRepo, inboundRepo, shareDomainRepo, cfIPEntryRepo, clientRepo, planRepo, planNodeRepo, extNodeRepo2, mihomoConfigRepo2, mihomoRuleRepo2, systemSettingRepo, accessLogRepo)

	// Subscription at root path - no /sub prefix needed
	r.GET("/:token", publicSubH.ServeSubscription)

	return r
}
