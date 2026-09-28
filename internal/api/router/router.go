package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	chainlinkHandler "corefusion/internal/api/handlers/chainlink"

	certsHandler "corefusion/internal/api/handlers/certificates"
	cfipHandler "corefusion/internal/api/handlers/cfip"
	chargeHandler "corefusion/internal/api/handlers/charge"
	clientHandler "corefusion/internal/api/handlers/client"
	clientauthHandler "corefusion/internal/api/handlers/clientauth"
	clientextHandler "corefusion/internal/api/handlers/clientext"
	coreconfigHandler "corefusion/internal/api/handlers/coreconfig"
	endpointconfigHandler "corefusion/internal/api/handlers/endpointconfig"
	geoRuleHandler "corefusion/internal/api/handlers/georule"
	inboundconfigHandler "corefusion/internal/api/handlers/inboundconfig"
	logManageHandler "corefusion/internal/api/handlers/logmanage"
	metricsHandler "corefusion/internal/api/handlers/metrics"
	mihomoHandler "corefusion/internal/api/handlers/mihomo"
	nodesHandler "corefusion/internal/api/handlers/nodes"
	nodeServerHandler "corefusion/internal/api/handlers/nodeserver"
	oauthHandler "corefusion/internal/api/handlers/oauth"
	outboundconfigHandler "corefusion/internal/api/handlers/outboundconfig"
	passkeyHandler "corefusion/internal/api/handlers/passkey"
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
	twofaHandler "corefusion/internal/api/handlers/twofa"
	usersHandler "corefusion/internal/api/handlers/users"
	wafHandler "corefusion/internal/api/handlers/waf"
	warpHandler "corefusion/internal/api/handlers/warp"
	xrayControlHandler "corefusion/internal/api/handlers/xraycontrol"
	xrayStatsHandler "corefusion/internal/api/handlers/xraystats"
	"corefusion/internal/api/middleware"
	"corefusion/internal/api/response"
	"corefusion/internal/app/embed"
	"corefusion/internal/coredata"
	domainuser "corefusion/internal/domain/user"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/runtime/cert"
	"corefusion/internal/runtime/firewall"
	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/runtime/process"
	accessPkg "corefusion/internal/service/access"
	"corefusion/internal/service/audit"
	authService "corefusion/internal/service/auth"
	backupService "corefusion/internal/service/backup"
	binariesSvc "corefusion/internal/service/binaries"
	certService "corefusion/internal/service/certificate"
	cfipSvc "corefusion/internal/service/cfip"
	clientSvc "corefusion/internal/service/client"
	clientauthSvc "corefusion/internal/service/clientauth"
	configsyncSvc "corefusion/internal/service/configsync"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	endpointconfigSvc "corefusion/internal/service/endpointconfig"
	geoRuleSvc "corefusion/internal/service/georule"
	inboundconfigSvc "corefusion/internal/service/inboundconfig"
	logManageSvc "corefusion/internal/service/logmanage"
	mihomoSvc "corefusion/internal/service/mihomo"
	nodeService "corefusion/internal/service/node"
	nodeServerSvc "corefusion/internal/service/nodeserver"
	oauthService "corefusion/internal/service/oauth"
	outboundconfigSvc "corefusion/internal/service/outboundconfig"
	passkeySvc "corefusion/internal/service/passkey"
	planSvc "corefusion/internal/service/plan"
	portalSvc "corefusion/internal/service/portal"
	proxySvc "corefusion/internal/service/proxy"
	rateLimitSvc "corefusion/internal/service/ratelimit"
	runtimeapplySvc "corefusion/internal/service/runtimeapply"
	svcService "corefusion/internal/service/service"
	sharedomainconfigSvc "corefusion/internal/service/sharedomainconfig"
	subendSvc "corefusion/internal/service/subendpoint"
	subfmtSvc "corefusion/internal/service/subformat"
	subRegion "corefusion/internal/service/subscription"
	subService "corefusion/internal/service/subscription"
	systemSvc "corefusion/internal/service/system"
	trafficSvc "corefusion/internal/service/traffic"
	trafficsyncSvc "corefusion/internal/service/trafficsync"
	twofaSvc "corefusion/internal/service/twofa"
	userService "corefusion/internal/service/user"
	wafSvc "corefusion/internal/service/waf"
	warpSvc "corefusion/internal/service/warp"
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
//
// ⚠ 这里以前硬编码 `"corefusion-dev-secret"` ✗（安全审计 v5 P-3）：知道这个字符串的人就能
// 伪造出能通过鉴权的管理端 token（JWT 用的是对称密钥 ✗）。生产路径（`cmd/corefusion`）会显式
// 传入 `JWT_SECRET`，但**库代码不该带一个全世界都知道的默认密钥** ✗ → 改成每次随机生成：
// 进程重启即失效（正好适合测试/临时场景），且**随机源失败就 panic**（失败关闭，不退回可猜值 ✗）。
func New(database *gorm.DB) *gin.Engine {
	return NewWithSecret(database, RandomEphemeralSecret())
}

// RandomEphemeralSecret 生成一次性随机 JWT 密钥（32 字节 → 64 hex）。
func RandomEphemeralSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("corefusion: cannot generate JWT secret: " + err.Error())
	}
	return hex.EncodeToString(buf)
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

// reloadLocalProxiesAfterCertRefresh 让**面板本机**的代理重新加载证书 ✓。
//
// ⚠ 为什么必须**重启**而不是 reload ✗（真机实测 ✓）：caddy 会把证书**缓存在内存**里 ✓，
// 即使配置文件 reload 了、磁盘上的证书文件已经是新的 ✗，握手吐出来的**还是旧证书** ✗
// （实测：文件序列号已是 066C6B84… ✓ 而 caddy 仍吐 0596B774… ✗）。重启进程才会重新读文件 ✓。
//
// 代价 ✓：本机 caddy/nginx 会**短暂中断**（1~2 秒 ✓）—— 只在"本机在用的证书被续期"时发生 ✓
// （每张证书 ~60 天一次 ✓），可以接受 ✓。
func reloadLocalProxiesAfterCertRefresh(certID int64) {
	if proxyMgr == nil {
		return
	}
	ctx := context.Background()
	for _, pt := range []proxySvc.ProxyType{proxySvc.ProxyCaddy, proxySvc.ProxyNginx} {
		// 「该不该重启」统一由 RestartProxy 自己判断（未安装 / 未在运行 / 用户手动停掉 → no-op），
		// 这里不再复述同一判断，避免同一口径散落两处后各自漂移。
		if rerr := proxyMgr.RestartProxy(ctx, pt); rerr != nil {
			log.Printf("[cert] 证书 %d 刷新后重启本机代理 %s 失败：%v", certID, pt, rerr)
		} else {
			// 注意：跳过（no-op）也是 nil，所以这行只说"请求已处理"，
			// 实际重启与否、以及跳过原因由 RestartProxy 自己记日志，别在这里把跳过说成重启。
			log.Printf("[cert] 证书 %d 刷新：本机代理 %s 重启请求已处理（实际动作见 proxy 日志）", certID, pt)
		}
	}
}

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
	a.syncServerConfig(serverID, coreType, false)
}

// SyncServerConfigHotApplied 与 SyncServerConfig 相同，但声明「该核心的改动已经通过核心
// API 热下发到运行中的进程」——xray 的入站/出站/路由都是这样（面板改完会立刻调
// xray/control/inbound|outbound|routing）。此时本地节点只写盘、不重启核心，避免把
// 该核心的全部在线连接踢掉；配置写盘保证下次重启后仍然一致。
//
// 远端节点无需特殊处理：SyncData 默认 reload_strategy=NONE（已运行时只写文件不重启）。
// 目前只有 xray 具备热下发能力：sing-box 的 v2ray API 仅注册 StatsService，没有
// AlterInbound，入站/用户变更必须重启核心才能生效。
func (a *configSyncHookAdapter) SyncServerConfigHotApplied(_ context.Context, serverID int64, coreType string) {
	a.syncServerConfig(serverID, coreType, true)
}

func (a *configSyncHookAdapter) syncServerConfig(serverID int64, coreType string, hotApplied bool) {
	if a.syncSvc != nil {
		var err error
		if hotApplied {
			err = a.syncSvc.SyncNowOpts(context.Background(), configsyncSvc.NoRestart(coreType))
		} else {
			err = a.syncSvc.SyncNow(context.Background())
		}
		if err != nil {
			log.Printf("[router] configSyncHook: SyncNow failed for server %d: %v", serverID, err)
		}
	}
	// For remote nodes (serverID > 1), also trigger SyncData to push
	// inbound_configs to the Agent's local SQLite and—crucially—land
	// TLS certificate PEM files on disk. SyncNow only pushes the xray
	// config file, but certificates referenced in the config are only
	// sent via SyncData's collectCertificates step.
	if a.nodeServerSvc != nil && serverID > 1 {
		go func() {
			if _, err := a.nodeServerSvc.SyncServerData(context.Background(), serverID, true); err != nil {
				log.Printf("[router] configSyncHook: SyncData failed for server %d: %v", serverID, err)
			} else {
				log.Printf("[router] configSyncHook: SyncData success for server %d", serverID)
			}
		}()
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

// mirrorCheckerAdapter adapts nodeserver.Service to the MirrorChecker interface.
// It checks if a server is a mirror node to enforce read-only constraints.
type mirrorCheckerAdapter struct {
	nodeServerSvc *nodeServerSvc.Service
}

func (a *mirrorCheckerAdapter) IsMirrorNode(ctx context.Context, serverID int64) bool {
	if a.nodeServerSvc == nil {
		return false
	}
	srv, err := a.nodeServerSvc.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		return false
	}
	return srv.NodeType == "mirror"
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
	// 访问日志：gin 默认会把完整 query 写进日志，而管理端鉴权支持 `?token=`（前端下载/日志 SSE 流在用）
	// → 会把手里的 JWT 明文落到日志文件里。改用带脱敏的 formatter（不改变鉴权行为）。
	r.Use(gin.LoggerWithFormatter(middleware.AccessLogFormatter))
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	// 请求体上限：多数 handler 直接 io.ReadAll，原先没有任何上限（multipart 上传/还原走流式，内部跳过）。
	r.Use(middleware.BodyLimit(middleware.DefaultMaxBodyBytes))
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
	// EAB 独立配置（Google / ZeroSSL 必填 ✓）：和 DNS 通道一样单独管理 ✓
	eabConfigRepo := repository.NewEABConfigRepository(database)
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
	// Scheduler start is deferred until after systemSettingSvc is created (below)
	// to allow reading configurable intervals from DB.

	inboundConfigSvc := inboundconfigSvc.NewService(inboundConfigRepo, database)
	outboundConfigRepo := repository.NewOutboundConfigRepository(database)
	outboundConfigSvc := outboundconfigSvc.NewService(outboundConfigRepo)
	endpointConfigRepo := repository.NewEndpointConfigRepository(database)
	endpointConfigSvc := endpointconfigSvc.NewService(endpointConfigRepo)

	// Config sync service (periodically syncs DB configs to disk)
	configSyncSvc := configsyncSvc.NewService(coreConfigSvc, inboundConfigSvc, outboundConfigSvc, dataDir, 5*time.Minute)

	// Inject process.Manager as the config writer so configsync uses the same
	// mutex-protected write path as LocalAgent.SyncConfig, avoiding race conditions.
	if processMgr != nil {
		configSyncSvc.SetConfigWriter(processMgr)
		// 本地节点写盘后重启核心：xray/sing-box 均不支持配置热加载（远端由 Agent 重启）
		configSyncSvc.SetLocalCoreRestarter(processMgr)
	}

	// Inject NodeServerRepository as the server lister so configsync can
	// discover all registered servers (local + remote) for multi-node sync.
	nodeServerRepoForSync := repository.NewNodeServerRepository(database)
	configSyncSvc.SetServerLister(nodeServerRepoForSync)

	// Inject serverRepo into inboundConfigSvc so that List queries for
	// mirror nodes resolve to the primary's inbound configs.
	inboundConfigSvc.SetServerRepo(nodeServerRepoForSync)

	// Runtime config workflow (snapshot / dry-run / apply / rollback / disable-proxy)
	var runtimeApplyWriter runtimeapplySvc.ConfigWriter
	if processMgr != nil {
		runtimeApplyWriter = processMgr
	}
	runtimeApplySvcInst := runtimeapplySvc.NewService(database, dataDir, configSyncSvc, runtimeApplyWriter, nodeServerRepoForSync, sharedNodeMgr)

	// Initialize services
	authSvc := authService.NewService(userRepo, jwtSecret)
	// 让鉴权中间件能校验令牌版本（登出即吊销旧 token，M8）
	middleware.SetTokenVersionProvider(userRepo)
	// 一次性短时票据（M10）：SSE/下载这类无法自定义 Header 的场景用它替代 URL 里的长期 token
	middleware.InitTicketStore(60 * time.Second)
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
	// 让节点回传的公网证书（Agent 本机 ACME 签的 ✓）落到本服务的落盘/落库逻辑里 ✓
	if sharedNodeMgr != nil {
		sharedNodeMgr.SetPublicCertSink(publicCertSinkAdapter{svc: certSvc})
	}
	binaryResolver := binariesSvc.NewResolver(dataDir)
	// 让配置校验跑真实核心二进制（sing-box check / xray -test）：
	// 面板拼装配置时写入未知字段（如 dns.servers[].address、route 规则的 strategy）
	// 在 sing-box 上会直接 FATAL，纯 JSON 语法检查发现不了。
	coreConfigSvc.SetBinaryValidator(binariesSvc.NewValidator(binaryResolver))
	systemSettingSvc := systemSvc.NewSettingService(systemSettingRepo, notifTmplRepo)
	// 存量明文敏感配置（SMTP 密码 / Telegram token / OAuth secret）统一 AES-256-GCM 加密落库，幂等
	if serr := systemSettingSvc.EncryptExistingSecrets(context.Background()); serr != nil {
		log.Printf("[router] encrypt existing secrets skipped: %v", serr)
	}

	// ── Start background schedulers with configurable intervals ──
	// All intervals are read from system_settings (scheduler.* prefix).
	// Defaults are used if settings are missing or invalid.
	schedCfg, _ := systemSettingSvc.GetSchedulerConfig(context.Background())
	// Trial plan auto-convert/expiry scheduler
	planSvcInst.StartTrialScheduler(context.Background(), time.Duration(schedCfg.TrialScanInterval)*time.Minute)
	// Pending downgrade auto-execution scheduler
	planSvcInst.StartDowngradeScheduler(context.Background(), time.Duration(schedCfg.DowngradeScanInterval)*time.Minute)

	geoRuleSvcInst := geoRuleSvc.NewService(ruleSourceRepo, geoRuleRepo, dataDir, systemSettingSvc)

	// WAF Rule Store (unified nginx/caddy rules in database)
	wafRuleRepo := repository.NewWafRuleRepository(database)
	wafCfgRepo := repository.NewWafConfigRepository(database)
	wafDomainCfgRepo := repository.NewWafDomainConfigRepository(database)
	wafSvcInst := wafSvc.NewService(wafRuleRepo, wafCfgRepo, wafDomainCfgRepo, dataDir, systemSettingSvc)
	// Initialize from GitHub if table is empty (async, non-blocking)
	go func() {
		if err := wafSvcInst.EnsureInitialized(context.Background()); err != nil {
			log.Printf("WARNING: waf rule initialization failed: %v", err)
		} else {
			// Land rules to local disk
			if err := wafSvcInst.LandAllRules(context.Background()); err != nil {
				log.Printf("WARNING: waf rule land to disk failed: %v", err)
			}
		}
	}()

	// Traffic sync service (periodically syncs traffic stats to DB)
	trafficSyncSvc := trafficsyncSvc.NewService(systemSettingSvc, inboundConfigSvc, 30*time.Second)
	// 核心未运行/未安装时不去探测其 stats API（避免每 30s 无效连接与日志刷屏）
	if processMgr != nil {
		trafficSyncSvc.SetCoreProbeGate(processMgr.IsCoreRunning)
	}
	geoRuleScheduler := geoRuleSvc.NewScheduler(ruleSourceRepo, geoRuleSvcInst)
	warpSvcInst := warpSvc.NewService(warpRepo)
	rateLimitConfigRepo := repository.NewRateLimitConfigRepository(database)
	rateLimitSvcInst := rateLimitSvc.NewService(rateLimitConfigRepo)
	dnsChannelSvc := certService.NewDNSChannelService(dnsChannelRepo)
	eabConfigSvc := certService.NewEABConfigService(eabConfigRepo)
	acmeEmailSvc := certService.NewACMEEmailService(acmeAccountRepo)
	notifier := systemSvc.NewNotifier(systemSettingSvc)
	backupSvcInst := backupService.NewService(backupRepo, dataDir, database)
	// Start the backup scheduler so scheduled backups and the retention policy
	// actually run (it was previously defined but never started).
	backupScheduler := backupService.NewBackupScheduler(backupSvcInst, time.Duration(schedCfg.BackupCheckInterval)*time.Minute)
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
	// 双证书 ✓：生成器要按域名收集同域名的多张证书（ECC + RSA ✓）
	caddyMgr.SetCertificateRepo(certRepo)
	nginxMgr.SetCertificateRepo(certRepo)

	// Set server_id = 1 (local node) for per-node site filtering.
	// All proxy/caddy/nginx site queries filter by server_id to ensure
	// data isolation between node servers.
	proxyMgr.SetServerID(1)
	caddyMgr.SetServerID(1)
	nginxMgr.SetServerID(1)

	// After nginx (re)install, tar.gz overwrites config.lua with defaults.
	// This callback re-syncs WAF config from DB so user customizations survive.
	proxyMgr.SetOnProxyInstalled(func(ctx context.Context, proxyType string) {
		if proxyType == "nginx" {
			if err := wafSvcInst.LandRulesForServer(ctx, 1); err != nil {
				log.Printf("[WAF] post-install re-sync failed: %v", err)
			}
		}
	})

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
	// Wire server repo so configsync can detect mirror nodes and use primary's data
	configSyncSvc.SetServerRepo(nodeServerRepoForSync)

	// 注入链式代理生成器到 configsync（设计文档 §6.3 / §13.2）：入口选出口生成的
	// chain-* outbounds / 路由规则 / balancers / observatory 通过 PrepareFullConfig
	// 合并进每个节点（本地 + 远程）的最终配置。
	chainLinkGen := agent.NewChainLinkGenerator(
		repository.NewChainLinkRepository(database),
		nodeServerRepoForSync,
		extNodeRepo,
	)
	// 链式出口"使用出口节点上已有的入站"这条路径需要读入站配置（协议/端口/传输/凭据）。
	chainLinkGen.SetInboundRepo(repository.NewInboundConfigRepository(database))
	// 外部出口引用的是 external_node_servers.id（注册成虚拟节点的外部节点）→
	// 生成器需要由它跳到 external_nodes 才能拿到协议/地址/凭据（见 buildExternalOutbound）。
	chainLinkGen.SetExtServerRepo(repository.NewExternalNodeServerRepository(database))
	configSyncSvc.SetChainLinkProvider(chainLinkGen)

	// 注入 endpoint provider：让 sing-box 的 WireGuard/Tailscale/OpenConnect/OpenVPN
	// 端点（endpoint_configs 表）真正进入运行时配置的顶层 endpoints[]。
	configSyncSvc.SetEndpointProvider(endpointConfigSvc)

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
	inboundMgr.SetServerRepo(nodeServerRepoForSync)
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
	// 2FA（TOTP）服务：登录两步验证 + 绑定/关闭/恢复码管理。
	twoFASvc := twofaSvc.NewService(userRepo, repository.NewRecoveryCodeRepository(database), "CoreFusion Panel")
	authH := authHandler.NewHandler(authSvc, twoFASvc)
	twofaH := twofaHandler.NewHandler(twoFASvc, authSvc)
	// Passkey（WebAuthn 通行密钥）：注册/登录/白名单管理。
	// RP 显示名用面板品牌名，浏览器授权弹窗展示给用户。
	passkeyH := passkeyHandler.NewHandler(
		passkeySvc.NewService(userRepo, repository.NewWebAuthnCredentialRepository(database), systemSettingRepo, "CoreFusion Panel"),
		authSvc,
	)
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
	// 删除客户时**级联清理余额/流水** ✓（不清理会变孤儿 ✗ → 新号撞同 ID 继承旧余额 ✗）
	clientSvcInst.SetBalanceCleaner(struct {
		*repository.ClientBalanceRepository
		*repository.BalanceTransactionRepository
	}{
		ClientBalanceRepository:      repository.NewClientBalanceRepository(database),
		BalanceTransactionRepository: repository.NewBalanceTransactionRepository(database),
	})
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

	// Start callback retry scheduler
	commerceSvcInst.StartRetryScheduler(time.Duration(schedCfg.CallbackRetryInterval) * time.Minute)

	// v3.2: Start processing recovery scheduler
	// Recovers stuck processing orders, cleans up nonces (24h) and expired secrets (7d)
	commerceSvcInst.StartProcessingRecovery(time.Duration(schedCfg.ProcessingRecoveryInterval) * time.Minute)

	// Start expired order cleanup scheduler
	commerceSvcInst.StartCleanupScheduler(time.Duration(schedCfg.OrderCleanupInterval) * time.Minute)

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
	// 证书列表展示"签发节点" ✓：把 issuer_node_id 翻译成节点名 ✓（光秃秃的数字看不出是谁签的 ✗）
	certH.SetNodeNameResolver(repository.NewNodeServerRepository(database))
	// 证书页"选节点签发"：面板在 SNAT 后面时，公网证书只能由节点在本机签 ✓
	if sharedNodeMgr != nil {
		certH.SetNodeIssuer(sharedNodeMgr)
	}
	// 邮箱/CA 归**面板** ✓（`acme_account_configs` 里的 ACME 账号 ✓）→ 必须接线 ✗，
	// 否则节点侧签发会因"没有邮箱"而失败 ✓（真机踩到过：面板日志明确写了
	// `panel did not supply an ACME account email` ✓）。
	if dnsChannelSvc != nil {
		// DNS 通道 → dns-01 提供者配置 ✓（凭据在通道里 ✓，按 ID 解析 ✓）
		certH.SetDNSChannelResolver(dnsChannelSvc)
		// ⚠ 同一个解析器也必须给**证书服务** ✗：续期时服务要按库里记的 `dns_channel_id`
		// **自己**还原凭据 ✓（前端续期不带任何参数 ✓）。
		// 只接给 handler 的话，续期就还是拿不到 DNS 凭据 → 又退化成 http-01 ✗（真机 bug ✓）。
		certSvc.SetDNSChannelResolver(dnsChannelSvc)
	}
	if eabConfigSvc != nil {
		// EAB 独立配置 → 签发时按 eab_config_id 解析出 CA + EAB ✓（不用手填 ✗）
		certH.SetEABConfigResolver(eabConfigAdapter{svc: eabConfigSvc})
	}
	// 面板本机的 ACME 默认邮箱 ✓ —— 否则注册时"没有联系邮箱" ✗（Google 直接拒 ✓）
	if acmeEmailSvc != nil {
		if def, derr := acmeEmailSvc.GetDefault(context.Background()); derr == nil && def != nil {
			certSvc.SetDefaultACMEEmail(def.Email)
			log.Printf("[router] panel ACME default email = %s", def.Email)
		}
	}
	// 节点续期也要能"首次自动注册" ✓ → 把 CA → (邮箱, EAB) 的能力注入给节点管理器 ✓
	if sharedNodeMgr != nil {
		sharedNodeMgr.SetPublicCertIssuanceEnv(publicCertEnvAdapter{acmeSvc: acmeEmailSvc, eabSvc: eabConfigSvc})
	}
	if acmeEmailSvc != nil {
		certH.SetACMEEmailResolver(acmeEmailAdapter{svc: acmeEmailSvc})
		if sharedNodeMgr != nil {
			// 自动续期走的是 manager 的 IssuePublicCert ✓，同样要能解析默认账号邮箱 ✓
			sharedNodeMgr.SetACMEAccountResolver(acmeEmailAdapter{svc: acmeEmailSvc})
		}
	}
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

		// 注入链式代理生成器（§5/§6）：使入口选出口生成的 outbounds/rules/balancers
		// 在 PrepareFullConfig 时合并进最终 xray 配置（复用上面为 configsync 构建的实例）。
		data, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
			Ctx:               ctx,
			ServerID:          1,
			CoreType:          coreType,
			FullConfig:        fullConfig,
			InboundProvider:   inboundConfigSvc,
			OutboundProvider:  outboundConfigSvc,
			EndpointProvider:  endpointConfigSvc,
			ChainLinkProvider: chainLinkGen,
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
	wafH := wafHandler.NewHandler(wafSvcInst)
	if sharedNodeMgr != nil {
		geoRuleSvcInst.SetNodeOperator(sharedNodeMgr)
		geoRuleSvcInst.SetNodeLister(nodeServerRepoForSync)
		wafSvcInst.SetSyncer(&wafNodeSyncerAdapter{mgr: sharedNodeMgr})
		// Inject RemoteSyncer so configsync can push configs to remote Agents.
		configSyncSvc.SetRemoteSyncer(sharedNodeMgr)
		// Wire rate limit service so PushIPAssociation can reach remote Agents.
		rateLimitSvcInst.SetNodeOperator(sharedNodeMgr)
	}
	warpH := warpHandler.NewHandler(warpSvcInst)
	rateLimitH := rateLimitHandler.NewHandler(rateLimitSvcInst)
	certChannelH := certsHandler.NewChannelHandler(dnsChannelSvc)
	certEABH := certsHandler.NewEABHandler(eabConfigSvc)
	certEmailH := certsHandler.NewEmailHandler(acmeEmailSvc)
	logManageH := logManageHandler.NewHandler(logManageSvc)
	// Create log cleanup service for DB table cleanup rules
	logCleanupSvc := trafficSvc.NewLogCleanupService(database, time.Duration(schedCfg.LogCleanupInterval)*time.Minute)
	cleanupH := logManageHandler.NewCleanupHandler(logCleanupSvc).WithSettingStore(systemSettingSvc)
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
	// 给库里已有的门户模板打 M7 安全补丁（用户内容不再执行 HTML）——全自动、幂等
	if perr := portalTmplSvc.EnsureSecurityPatches(context.Background()); perr != nil {
		log.Printf("[router] portal security patch skipped: %v", perr)
	}

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
			// 2FA 第二步：密码通过后凭一次性登录票据验证 TOTP/恢复码换取 token。
			// ⚠ 与密码登录同享登录限流：动态码也是可爆破对象。
			authGroup.POST("/login/2fa", middleware.LoginRateLimit(), twofaH.Login2FA)
			// 登录页查询指定用户名是否已启用 2FA（公开）——据此把输入框切换为
			// "密码"或"动态码/恢复码"。只返回 enabled 布尔，不泄露账号存在性。
			authGroup.GET("/login/2fa-status", authH.Login2FAStatus)
			// Passkey（无密码登录）：
			//   - status：前端据此显示/隐藏登录按钮（未认证阶段只认白名单已有条目）；
			//   - login/begin + login/finish：discoverable 登录仪式。
			// ⚠ 这些接口**不**放在 AuthRequired 后面 —— passkey 本身就是登录入口。
			authGroup.GET("/passkey/status", passkeyH.Status)
			authGroup.POST("/passkey/login/begin", passkeyH.LoginBegin)
			authGroup.POST("/passkey/login/finish", passkeyH.LoginFinish)
		}

		// Auth (authenticated): operates on the calling user.
		authSelfGroup := v1.Group("/auth")
		authSelfGroup.Use(middleware.AuthRequired())
		{
			authSelfGroup.GET("/me", authH.Me)
			authSelfGroup.POST("/change-password", authH.ChangePassword)
			// 登出必须**已认证** ✗：它要按"当前用户"吊销令牌（bump token_version）。
			// 原先注册在公开的 authGroup 里 → handler 拿不到 user_id → 吊销被静默跳过
			// （实测：登出后旧 token 仍然 200，且库里版本没变）。
			authSelfGroup.POST("/logout", authH.Logout)
			// 签发一次性短时票据（M10）：日志 SSE 流与备份下载用它，避免把长期 token 放进 URL
			authSelfGroup.POST("/ticket", authH.IssueTicket)
			// 2FA 管理：绑定/启用/关闭/恢复码/状态（已登录用户管理自己）
			authSelfGroup.GET("/2fa/status", twofaH.Status)
			authSelfGroup.POST("/2fa/setup", twofaH.Setup)
			authSelfGroup.POST("/2fa/enable", twofaH.Enable)
			authSelfGroup.POST("/2fa/disable", twofaH.Disable)
			authSelfGroup.POST("/2fa/recovery-codes", twofaH.RegenerateRecovery)
			// Passkey 管理（已登录用户管理自己的凭证）
			authSelfGroup.POST("/passkey/register/begin", passkeyH.RegisterBegin)
			authSelfGroup.POST("/passkey/register/finish", passkeyH.RegisterFinish)
			authSelfGroup.GET("/passkey/credentials", passkeyH.ListCredentials)
			authSelfGroup.DELETE("/passkey/credentials/:id", passkeyH.DeleteCredential)
			// RP ID 白名单管理（管理员）：设置页"把当前访问域名加入白名单"按钮
			passkeyAdmin := middleware.RequireRole(domainuser.RoleAdmin)
			authSelfGroup.GET("/passkey/rpids", passkeyAdmin, passkeyH.ListRPIDs)
			authSelfGroup.POST("/passkey/rpids", passkeyAdmin, passkeyH.UpdateRPIDs)
		}

		// Protected routes
		//
		// ⚠ `Use()` 必须在**任何路由注册之前**调用 —— Gin 在注册时就把当时的 handler 链快照进去了
		// （`combineHandlers(group.Handlers, handlers)`），之后再 `Use` 对已注册路由**无效** ✗。
		// 这里原本把 `Use(AuthRequired())` 写在 9 条 WAF 路由**之后**，导致它们**完全没有鉴权**：
		// 实测不带任何凭据 `GET /api/v1/waf/rules` 返回 200 与规则内容 ✗（安全审计 v5 C1）。
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired())
		{
			// WAF Rule Library（全局配置，会下发到所有节点）
			//   读：任意已登录用户；写 / 同步 / 下发：管理员（与 charge-sites 等全局配置一致）
			wafWrite := middleware.RequireRole(domainuser.RoleAdmin)
			protected.GET("/waf/rules", wafH.ListRules)
			protected.GET("/waf/rules/:name", wafH.GetRule)
			protected.PUT("/waf/rules/:name", wafWrite, wafH.SaveRule)
			protected.DELETE("/waf/rules/:name", wafWrite, wafH.DeleteRule)
			protected.POST("/waf/sync-github", wafWrite, wafH.SyncFromGitHub)
			protected.POST("/waf/sync-all", wafWrite, wafH.SyncAll)
			protected.GET("/waf/github-source", wafH.GetGitHubSource)
			protected.PUT("/waf/github-source", wafWrite, wafH.SaveGitHubSource)
			protected.POST("/waf/push-all", wafWrite, wafH.PushToAllNodes)

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

			// Endpoint Configs (WireGuard/Tailscale/OpenConnect/OpenVPN endpoints)
			endpointCfgH := endpointconfigHandler.NewHandler(endpointConfigSvc)

			// Inject config sync hooks so inbound/outbound/endpoint changes trigger
			// full config sync to remote nodes.
			// Inbound changes also trigger mirror sync (mirrors replicate inbounds
			// and caddy/nginx proxy configs); outbound/endpoint changes do NOT.
			inboundSyncHook := &configSyncHookAdapter{syncSvc: configSyncSvc, triggerMirrorSync: true}
			outboundSyncHook := &configSyncHookAdapter{syncSvc: configSyncSvc, triggerMirrorSync: false}
			endpointSyncHook := &configSyncHookAdapter{syncSvc: configSyncSvc, triggerMirrorSync: false}
			inboundCfgH.SetSyncHook(inboundSyncHook)
			outboundCfgH.SetSyncHook(outboundSyncHook)
			endpointCfgH.SetSyncHook(endpointSyncHook)

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
			cfipSvcInst.StartScheduler(context.Background(), time.Duration(schedCfg.CFIPCheckInterval)*time.Minute)
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
				// 导出充值订单 CSV（原来只有列表 ✗）；与 /:id 同级 static+param 共存（gin 支持 ✓）
				chargeOrderGroup.GET("/export-csv", chargeAdminH.ExportOrdersCSV)
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
				// 导出余额流水 CSV（原来只能在弹窗里逐个客户端看 ✗）
				clientBalanceGroup.GET("/transactions/export-csv", chargeAdminH.ExportBalanceTransactionsCSV)
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
			// 节点↔证书 多对多 ✓（与 main.go 同一个 DB ✓，多实例无妨 ✓）
			nodeServerSvcInst := nodeServerSvc.NewService(nodeServerRepo, bootstrapTokenMgr, caService).
				WithNodeCertificates(repository.NewNodeCertificateRepository(database))

			// Chain Proxy (chain links, external node servers, topology)
			chainLinkRepo := repository.NewChainLinkRepository(database)
			extNodeServerRepo := repository.NewExternalNodeServerRepository(database)
			inboundConfigRepo := repository.NewInboundConfigRepository(database)
			chainLinkH := chainlinkHandler.NewHandler(chainLinkRepo, extNodeServerRepo, nodeServerRepo, extNodeRepo, inboundConfigRepo)
			chainLinkH.RegisterRoutes(protected)

			// Inject chain proxy repo into public subscription handler so that
			// external node servers are rendered as virtual entries (§8.2).
			publicSubH.SetChainProxyRepos(extNodeServerRepo)

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
				// 把"节点回传落库"这条路也接上分发 ✓（原来只有面板侧更新才会分发 ✗）
				autoDistributeCertFn = func(ctx context.Context, certID int64) {
					if derr := nodeServerSvcInst.DistributeCertificateToAll(ctx, certID); derr != nil {
						log.Printf("[router] auto-distribute cert %d after node report: %v", certID, derr)
					} else {
						log.Printf("[router] auto-distributed cert %d (node-reported) to using nodes", certID)
					}
				}

				certH.OnCertUpdated(func(ctx context.Context, certID int64) {
					if err := nodeServerSvcInst.DistributeCertificateToAll(ctx, certID); err != nil {
						log.Printf("[router] auto-distribute cert %d after update: %v", certID, err)
					} else {
						log.Printf("[router] auto-distributed cert %d to all associated nodes", certID)
						// 面板本机那份副本同样要刷新 + 重启本机代理 ✓（否则本机内核一直吐旧证书 ✗）
						if certSvc != nil {
							go func(id int64) {
								if lerr := certSvc.SyncLocalCopyIfLanded(context.Background(), id); lerr != nil {
									log.Printf("[cert] 刷新本机证书副本失败（cert %d）：%v", id, lerr)
									return
								}
								reloadLocalProxiesAfterCertRefresh(id)
							}(certID)
						}
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
				// Inject sing-box V2Ray API address so the Agent's XrayAPIProxy
				// can serve stats requests for sing-box nodes too.
				sbAddr := coredata.ResolveStatsAddr(context.Background(), coredata.StatsAddrDeps{
					SettingGetter: func(ctx context.Context, key string) (string, error) {
						return systemSettingSvc.GetByKey(ctx, key)
					},
					V2RayAPIListenGetter: func(ctx context.Context) (string, error) {
						full, err := coreConfigSvc.GetFull(ctx, coredata.LocalServerID, "singbox")
						if err != nil {
							return "", err
						}
						expRaw, ok := full["experimental"]
						if !ok {
							return "", fmt.Errorf("no experimental section")
						}
						expMap, ok := expRaw.(map[string]interface{})
						if !ok {
							return "", fmt.Errorf("experimental not map")
						}
						v2apiRaw, ok := expMap["v2ray_api"]
						if !ok {
							return "", fmt.Errorf("no v2ray_api section")
						}
						v2apiMap, ok := v2apiRaw.(map[string]interface{})
						if !ok {
							return "", fmt.Errorf("v2ray_api not map")
						}
						listen, ok := v2apiMap["listen"].(string)
						if !ok || listen == "" {
							return "", fmt.Errorf("no listen")
						}
						return listen, nil
					},
				})
				localAgent.WithSingboxStatsAddr(sbAddr)
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
				chainHealthRegistry := agent.NewChainHealthMonitorRegistry(nodeMgrForPhase3, nodeServerRepo).WithChainRepos(chainLinkRepo)
				go chainHealthRegistry.StartAll(context.Background())

				// WAF 镜像周期性对账 ✓（兜底：事件驱动 fanout 在"镜像恰好离线"时会漏掉 ✗，
				// 镜像恢复在线后由这里自动补上 ✓；内容没变不会白重启代理 ✓）。
				go wafH.RunMirrorReconcileLoop(context.Background(), 10*time.Minute,
					func(id int64) bool { return nodeMgrForPhase3.IsOnline(id) })
				billingFilter := agent.NewBillingFilter(nodeServerRepo)

				// Inject NodeServerManager as HealthChecker for chain link health API
				chainLinkH.SetHealthChecker(nodeMgrForPhase3)

				nodeServerSvcInst.
					WithInboundRepo(inboundRepoForNodeServer).
					WithNodeManager(nodeMgrForPhase3).
					WithFailoverManager(failoverMgr).
					WithMirrorSync(mirrorSyncMgr).
					WithChainGenerator(chainGen).
					WithChainHealthRegistry(chainHealthRegistry).
					WithBillingFilter(billingFilter)

				// Inject nodeServerRepo into CaddyManager and NginxManager so they
				// can detect mirror nodes and generate config from the primary's data.
				caddyMgr.SetNodeServerRepo(nodeServerRepo)
				nginxMgr.SetNodeServerRepo(nodeServerRepo)
				// Inject nodeServerRepo into WAF Service so it can detect mirror
				// nodes and collect WAF rules from the primary's server_id.
				wafSvcInst.SetNodeServerRepo(nodeServerRepo)

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

				// 注入核心用户流量采集（本地 + 远程），用于实时速率图表
				// 支持 xray 和 sing-box 两种核心类型：
				//   xray:    直连 xray gRPC Stats API (api.listen, 默认 127.0.0.1:10085)
				//   singbox: 直连 sing-box V2Ray API (experimental.v2ray_api.listen, 默认 127.0.0.1:8080)
				// 远程：通过 coredata.Provider → Agent RPC 查询（协议兼容，统一走 XrayAPIOnServer）
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
				// sing-box V2Ray API client。注意：sing-box 与 xray 的 stats protobuf
				// 字段一致，但 gRPC 服务名不同（sing-box 用
				// v2ray.core.app.stats.command.StatsService），必须用 NewV2RayClient，
				// 否则查询一直 Unimplemented，实时速率里 sing-box 部分恒为 0。
				localSingboxClient := coredata.NewV2RayClient(coredata.ResolveStatsAddr(context.Background(), coredata.StatsAddrDeps{
					SettingGetter: func(ctx context.Context, key string) (string, error) {
						return systemSettingSvc.GetByKey(ctx, key)
					},
					V2RayAPIListenGetter: func(ctx context.Context) (string, error) {
						full, err := coreConfigSvc.GetFull(ctx, coredata.LocalServerID, "singbox")
						if err != nil {
							return "", err
						}
						expRaw, ok := full["experimental"]
						if !ok {
							return "", fmt.Errorf("no experimental section")
						}
						expMap, ok := expRaw.(map[string]interface{})
						if !ok {
							return "", fmt.Errorf("experimental not map")
						}
						v2apiRaw, ok := expMap["v2ray_api"]
						if !ok {
							return "", fmt.Errorf("no v2ray_api section")
						}
						v2apiMap, ok := v2apiRaw.(map[string]interface{})
						if !ok {
							return "", fmt.Errorf("v2ray_api not map")
						}
						listen, ok := v2apiMap["listen"].(string)
						if !ok || listen == "" {
							return "", fmt.Errorf("no listen")
						}
						return listen, nil
					},
				}))
				// collectLocalTraffic tries xray first, then sing-box, summing
				// user traffic from whichever core is running. This handles
				// nodes that run xray only, sing-box only, or both.
				collectLocalTraffic := func() (int64, int64) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					var up, down int64
					// 核心未运行时跳过该核心的探测（本地节点常常只跑其中一个核心）
					xrayRunning := processMgr == nil || processMgr.IsCoreRunning("xray")
					sboxRunning := processMgr == nil || processMgr.IsCoreRunning("singbox")
					// Try xray
					if resp, err := localXrayClient.RawStats(ctx, false); xrayRunning && err == nil {
						if sr, err := coredata.ParseStatsResponse(resp); err == nil {
							for _, u := range sr.Users {
								up += u.Uplink
								down += u.Downlink
							}
						}
					}
					// Try sing-box (V2Ray API is protocol-compatible with xray StatsService)
					if resp, err := localSingboxClient.RawStats(ctx, false); sboxRunning && err == nil {
						if sr, err := coredata.ParseStatsResponse(resp); err == nil {
							for _, u := range sr.Users {
								up += u.Uplink
								down += u.Downlink
							}
						}
					}
					return up, down
				}
				metricsCollector.SetLocalTrafficProvider(collectLocalTraffic)
				metricsCollector.SetRemoteTrafficProvider(func(ctx context.Context, serverID int64) (int64, int64) {
					// 通过 coredata.Provider 查询远程节点用户流量
					// 协议兼容：sing-box V2Ray API 使用与 xray 相同的 StatsService protobuf
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
				// Wire WAF syncer so remote nodes get WAF rules on reconnect
				nodeMgrForPhase3.SetWAFSyncer(wafSvcInst)
				// Wire geo rule syncer so remote nodes get geoip.dat/geosite.dat on reconnect
				nodeMgrForPhase3.SetGeoRuleSyncer(geoRuleSvcInst)
				// Wire node lister so WAF can push to all nodes
				wafSvcInst.SetNodeLister(nodeServerRepo)
				// NOTE: The automatic migration that flipped node rule merge_mode from
				// "replace" to "append" has been removed. It was overriding the user's
				// explicit merge_mode choice on every restart. The merge_mode is now
				// fully user-controlled and persisted correctly via Upsert.

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
					).WithCertificateProvider(certRepoForNodeServer, nodeServerRepo).WithProxySiteProvider(proxySiteRepo, caddySiteRepo, nginxSiteRepo).WithSystemSettingProvider(systemSettingRepo).WithChainRepos(
						repository.NewChainLinkRepository(database),
						repository.NewExternalNodeRepository(database),
					),
				).WithConfigProviders(coreConfigSvc, inboundConfigSvc, outboundConfigSvc).WithLogModeProvider(logManageSvc).WithServerRepo(nodeServerRepo).WithChainLinkProvider(
					agent.NewChainLinkGenerator(
						repository.NewChainLinkRepository(database),
						nodeServerRepo,
						repository.NewExternalNodeRepository(database),
					),
				).WithEndpointProvider(endpointConfigSvc)
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

				// Inject sing-box V2Ray API address into trafficsync so it can
				// also collect traffic from sing-box inbounds (protocol-compatible
				// with xray's StatsService gRPC).
				sbStatsAddr := coredata.ResolveStatsAddr(context.Background(), coredata.StatsAddrDeps{
					SettingGetter: func(ctx context.Context, key string) (string, error) {
						return systemSettingSvc.GetByKey(ctx, key)
					},
					V2RayAPIListenGetter: func(ctx context.Context) (string, error) {
						full, err := coreConfigSvc.GetFull(ctx, coredata.LocalServerID, "singbox")
						if err != nil {
							return "", err
						}
						expRaw, ok := full["experimental"]
						if !ok {
							return "", fmt.Errorf("no experimental section")
						}
						expMap, ok := expRaw.(map[string]interface{})
						if !ok {
							return "", fmt.Errorf("experimental not map")
						}
						v2apiRaw, ok := expMap["v2ray_api"]
						if !ok {
							return "", fmt.Errorf("no v2ray_api section")
						}
						v2apiMap, ok := v2apiRaw.(map[string]interface{})
						if !ok {
							return "", fmt.Errorf("v2ray_api not map")
						}
						listen, ok := v2apiMap["listen"].(string)
						if !ok || listen == "" {
							return "", fmt.Errorf("no listen")
						}
						return listen, nil
					},
				})
				trafficSyncSvc.SetSingboxStatsAddr(sbStatsAddr)

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
			// 面板本机 = 一个节点 ✓：**关联到它的证书**要落地成文件 ✓（跟 Agent 一样 ✓），
			// 其它证书只留库里那一份 ✓（用户口径：存数据库就是了 ✓）。
			nodeServerHandler.LandLocalCertFilesFn = func(ctx context.Context, certID int64) error {
				_, lerr := certSvc.LandCertificateFiles(ctx, certID)
				return lerr
			}
			nodeServerH.SetCoreConfigService(coreConfigSvc)
			nodeServerH.SetInboundConfigService(inboundConfigSvc)
			nodeServerH.SetOutboundConfigService(outboundConfigSvc)
			nodeServerH.SetEndpointConfigService(endpointConfigSvc)
			nodeServerH.SetProxyHandler(proxyH)
			nodeServerH.SetGeoRuleService(geoRuleSvcInst)

			// Wire up NodeOperator so proxy/caddy/nginx handlers can route
			// operations (start/stop/restart/reload, config sync, status)
			// to remote nodes via gRPC.
			proxyH.SetNodeOperator(nodeServerSvcInst)
			caddyH.SetNodeOperator(nodeServerSvcInst)
			nginxH.SetNodeOperator(nodeServerSvcInst)
			processH.SetNodeOperator(nodeServerSvcInst)
			wafH.SetNodeOperator(&wafNodeOperatorAdapter{svc: nodeServerSvcInst, mgr: sharedNodeMgr})
			wafH.SetProxyManager(&wafProxyReloaderAdapter{mgr: proxyMgr})

			// Inject SyncData hook so traffic reset also pushes inbound_configs
			// table to remote Agent's local SQLite DB.
			syncDataHook := &syncDataHookAdapter{nodeServerSvc: nodeServerSvcInst}
			inboundCfgH.SetSyncDataHook(syncDataHook)

			// Inject mirror checker so inbound write operations on mirror
			// nodes are rejected (mirror configs are read-only).
			inboundCfgH.SetMirrorChecker(&mirrorCheckerAdapter{nodeServerSvc: nodeServerSvcInst})

			// Wire nodeServerSvc into the configSyncHookAdapter so that
			// inbound config changes also trigger mirror node sync.
			inboundSyncHook.nodeServerSvc = nodeServerSvcInst
			outboundSyncHook.nodeServerSvc = nodeServerSvcInst

			// 注入「核心重载」能力：sing-box 的 v2ray API 只有 StatsService，
			// 不支持 xray 那样的 AlterInbound 热加用户，用户变更只能
			// 「重新生成并下发配置 + 重启核心」才能生效。
			if inboundMgr != nil {
				inboundMgr.SetCoreReloader(func(ctx context.Context, serverID int64, coreType string) error {
					if coreType == "" {
						coreType = "singbox"
					}
					// 1) 按核心重新生成并下发配置。
					//    注意：SyncServerData(applyAndStart=true) 内部只生成 xray
					//    配置（coreType 硬编码为 xray），sing-box 的配置不会更新，
					//    因此这里必须走 configsync 的按核心同步入口。
					if configSyncSvc == nil {
						return fmt.Errorf("config sync service not available")
					}
					if err := configSyncSvc.SyncServerCore(ctx, serverID, coreType); err != nil {
						return fmt.Errorf("sync %s config for server %d: %w", coreType, serverID, err)
					}
					// 2) 远端节点：SyncConfig 只负责写文件，需要显式重启核心才会
					//    加载新用户（sing-box 的 v2ray API 没有热加接口）。
					//    本地节点在 SyncServerCore 内已 RestartIfRunning，不重复重启。
					if serverID != 1 && nodeServerSvcInst != nil {
						if err := nodeServerSvcInst.ControlCore(ctx, serverID, coreType, "restart"); err != nil {
							return fmt.Errorf("restart core: %w", err)
						}
					}
					return nil
				})
			}

			// Inject config sync hook into chainlink handler so that
			// chain_links mutations automatically
			// trigger config regeneration and push to affected nodes
			// (design doc §13.2).
			chainSyncHook := &configSyncHookAdapter{syncSvc: configSyncSvc, nodeServerSvc: nodeServerSvcInst}
			chainLinkH.SetConfigSyncHook(chainSyncHook)

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
				nodeServersGroup.POST("/:id/binaries/reality-key", nodeServerH.GenerateRealityKey)
				// 由已有私钥推导公钥（sing-box 入站只存私钥，编辑器/分享链接需要 pbk）
				nodeServersGroup.POST("/:id/binaries/reality-public-key", nodeServerH.DeriveRealityPublicKey)
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
				// 创建入站/端点时取一个该节点未占用的端口（避免同端口 TCP/UDP 重复）
				nodeServersGroup.GET("/:id/free-port", nodeServerH.GetFreePort)
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

				// Unified WAF Management (database-backed, shared nginx/caddy)
				// Per-node WAF config
				nodeServersGroup.GET("/:id/waf/config", wafH.GetConfig)
				nodeServersGroup.PUT("/:id/waf/config", wafH.SaveConfig)
				nodeServersGroup.GET("/:id/waf/rules", wafH.ListNodeRules)
				nodeServersGroup.GET("/:id/waf/rules/:name", wafH.GetNodeRule)
				nodeServersGroup.PUT("/:id/waf/rules/:name", wafH.SaveNodeRule)
				nodeServersGroup.DELETE("/:id/waf/rules/:name", wafH.DeleteNodeRule)
				nodeServersGroup.GET("/:id/waf/domain-config", wafH.ListDomainConfigs)
				nodeServersGroup.PUT("/:id/waf/domain-config", wafH.SaveDomainConfigsBulk)
				nodeServersGroup.GET("/:id/waf/domain-config/:domain", wafH.GetDomainConfig)
				nodeServersGroup.PUT("/:id/waf/domain-config/:domain", wafH.SaveDomainConfig)
				nodeServersGroup.DELETE("/:id/waf/domain-config/:domain", wafH.DeleteDomainConfig)
				nodeServersGroup.GET("/:id/waf/domain-rules", wafH.ListDomainRuleDirs)
				nodeServersGroup.GET("/:id/waf/domain-rules/:domain/:name", wafH.GetDomainRuleFile)
				nodeServersGroup.PUT("/:id/waf/domain-rules/:domain/:name", wafH.SaveDomainRuleFile)
				nodeServersGroup.GET("/:id/waf/domain-status", wafH.GetDomainStatus)
				nodeServersGroup.POST("/:id/waf/enable-domain", wafH.EnableDomain)
				nodeServersGroup.POST("/:id/waf/disable-domain", wafH.DisableDomain)
				nodeServersGroup.POST("/:id/waf/sync", wafH.SyncToNode)
				nodeServersGroup.POST("/:id/waf/push", wafH.PushToNode)
				nodeServersGroup.GET("/:id/waf/status", wafH.GetWAFStatus)
				nodeServersGroup.PUT("/:id/waf/status", wafH.SetWAFStatus)

				// Inbound Configs (per node)
				nodeServersGroup.GET("/:id/inbound-configs", inboundCfgH.List)
				// 该节点上（xray + sing-box）已占用的 Server Names，供编辑器标记「已使用」
				nodeServersGroup.GET("/:id/used-server-names", inboundCfgH.UsedServerNames)
				// 该节点已启用的端口跳跃规则 + 空闲段建议，供编辑器提前标出冲突段
				nodeServersGroup.GET("/:id/port-hop/rules", inboundCfgH.PortHopRules)
				nodeServersGroup.POST("/:id/inbound-configs", inboundCfgH.Create)
				nodeServersGroup.GET("/:id/inbound-configs/:cfgId", inboundCfgH.Get)
				// 该入站逐用户的分享链接（与订阅同源：subscriptionView + resolveHostPort +
				// finalMaskWithPortHop + sharelink.Generate），供编辑器二维码/复制使用
				nodeServersGroup.GET("/:id/inbound-configs/:cfgId/share-links", publicSubH.InboundShareLinks)
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

				// Endpoint Configs (per node — WireGuard/Tailscale/OpenConnect/OpenVPN)
				nodeServersGroup.GET("/:id/endpoint-configs", endpointCfgH.List)
				nodeServersGroup.POST("/:id/endpoint-configs", endpointCfgH.Create)
				nodeServersGroup.GET("/:id/endpoint-configs/:cfgId", endpointCfgH.Get)
				nodeServersGroup.PUT("/:id/endpoint-configs/:cfgId", endpointCfgH.Update)
				nodeServersGroup.DELETE("/:id/endpoint-configs/:cfgId", endpointCfgH.Delete)

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

				// Geo Rules — global rule source and file management (migrated from per-node to global)
				// These are now under /api/v1/geo-rules/* (no node-servers/:id prefix)
				// Per-node push endpoint stays under node-servers for backwards compat
				nodeServersGroup.POST("/:id/geo-rules/push", geoRuleH.PushToNode)

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
				// 选节点签发：面板在 SNAT 后面时，公网证书只能由节点在本机签 ✓
				certsGroup.POST("/issue-on-node", certH.IssueOnNode)
				certsGroup.GET("/detect-ip", certH.DetectIP)
				certsGroup.GET("/check-port", certH.CheckPort)
				certsGroup.POST("/upload", certH.Upload)
				// 证书下载 / 打包下载 ✓ —— 浏览器无法自定义请求头 ✗，走一次性票据
				// （scope=cert_download ✓，见 middleware 的 ticketRouteScopes ✓）
				certsGroup.GET("/export", certH.Export)
				certsGroup.GET("/:id/download", certH.Download)
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

			// EAB 配置 — admin only.
			// EAB（外部账号绑定）独立管理 ✓：Google Public CA / ZeroSSL 必填 ✓，
			// 签发时只选关联 ✓（不用每次手填 EAB-KID / EAB-HMAC-Key ✗）。
			eabConfigsGroup := protected.Group("/eab-configs").Use(middleware.RequireRole(domainuser.RoleAdmin))
			{
				eabConfigsGroup.GET("", certEABH.List)
				eabConfigsGroup.POST("", certEABH.Create)
				// ZeroSSL 用**邮箱自动领取 EAB** ✓（同 acme.sh 的机制 ✓）→ 之后自动关联 ✓
				eabConfigsGroup.POST("/fetch-zerossl", certEABH.FetchZeroSSL)
				// Google：用 GCP 授权（服务账号 JSON / 临时 token）自动创建 EAB ✓
				eabConfigsGroup.POST("/fetch-google", certEABH.FetchGoogle)
				eabConfigsGroup.PUT("/:id", certEABH.Update)
				eabConfigsGroup.DELETE("/:id", certEABH.Delete)
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
			// sing-box 的 hysteria2/hysteria 端口跳跃（端口段 → 入站端口转发）。
			// 防火墙规则本身与核心无关（nft/iptables DNAT），因此复用同一套实现，
			// 只是给 sing-box 编辑器一个同名的调用入口。
			nodeServersGroup.POST("/:id/singbox/control/firewall/apply", xrayControlH.ApplyFirewallRules)
			nodeServersGroup.POST("/:id/singbox/control/firewall/remove", xrayControlH.RemoveFirewallRules)
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
				// 导出 CSV：前端 Security.vue "导出"按钮调的就是这个（原先后端没有 → 404）✓
				securityGroup.GET("/audit-events/export", securityH.ExportAuditEvents)
				securityGroup.GET("/alerts", securityH.ListAlerts)
				securityGroup.GET("/alert-channels", securityH.ListAlertChannels)
				securityGroup.POST("/alert-channels", securityH.CreateAlertChannel)
				securityGroup.PUT("/alert-channels/:id", securityH.UpdateAlertChannel)
				securityGroup.DELETE("/alert-channels/:id", securityH.DeleteAlertChannel)
				securityGroup.POST("/alerts/:id/resolve", securityH.ResolveAlert)
				// "全部标记已解决"：前端调 /alerts/resolve（原先后端没有 → 404）✓
				// 与上面的 /alerts/:id/resolve 同级 static + param 共存（gin v1.12 支持，已实测 ✓）
				securityGroup.POST("/alerts/resolve", securityH.ResolveAllAlerts)
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
				systemGroup.POST("/telegram-chat-id", systemH.GetTelegramChatID)
				systemGroup.POST("/test-smtp", systemH.TestSMTP)
				systemGroup.POST("/test-telegram", systemH.TestTelegram)
				systemGroup.POST("/upload-mtls-ca", systemH.UploadMTLSCA)
				systemGroup.GET("/github-config", systemH.GetGitHubConfig)
				systemGroup.PUT("/github-config", systemH.UpdateGitHubConfig)
				systemGroup.GET("/db-config", systemH.GetDBConfig)
				systemGroup.PUT("/db-config", systemH.UpdateDBConfig)
				systemGroup.POST("/db-config/test", systemH.TestDBConnection)
				systemGroup.GET("/db-stats", systemH.GetDBStats)
				systemGroup.POST("/db-stats/reset", systemH.ResetDBStats)
				systemGroup.GET("/db-stats/ping", systemH.PingDB)
				systemGroup.GET("/db-pool-stats", systemH.GetDBPoolStats)
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
				// 一步切换数据库（迁移 + 写 .env + 校验；失败则 .env 不变）
				superAdminSystem.POST("/db-config/switch", systemH.SwitchDatabase)
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

				// Database table cleanup rules
				logManageGroup.GET("/cleanup-rules", cleanupH.ListCleanupRules)
				logManageGroup.PUT("/cleanup-rules/:id", cleanupH.UpdateCleanupRule)
				logManageGroup.POST("/cleanup-all", cleanupH.CleanupAll)
				logManageGroup.POST("/cleanup-table/:table", cleanupH.CleanupTable)
				// Scheduler interval configuration
				logManageGroup.GET("/scheduler-config", cleanupH.GetSchedulerConfig)
				logManageGroup.PUT("/scheduler-config", cleanupH.UpdateSchedulerConfig)
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
			// Geo rule file download — public (no auth), used by remote Agents
			v1.GET("/geo-rules/files/:fileId/download-file", geoRuleH.DownloadFile)
			v1.GET("/agent/install", agentH.GetInstallScript)
			// Agent version & releases (requires auth — admin only)
			protected.Group("").Use(middleware.RequireRole(domainuser.RoleAdmin)).GET("/agent/version", agentH.GetAgentVersion)
			protected.Group("").Use(middleware.RequireRole(domainuser.RoleAdmin)).GET("/agent/releases", agentH.GetAgentReleases)

			// Placeholder routes for remaining modules
			registerPlaceholderRoutes(protected, database, runtimecfgHandler.NewHandler(runtimeApplySvcInst))

			// Geo Rules — global rule source and file management (no node-servers prefix)
			// These routes are public-except-auth (any authenticated user can read, admin can write).
			geoRuleGlobalGroup := protected.Group("/geo-rules")
			{
				geoRuleGlobalGroup.GET("/github-assets", geoRuleH.FetchGitHubAssets)
				geoRuleGlobalGroup.GET("/sources", geoRuleH.ListSources)
				geoRuleGlobalGroup.POST("/sources", geoRuleH.CreateSource)
				geoRuleGlobalGroup.PUT("/sources/:sourceId", geoRuleH.UpdateSource)
				geoRuleGlobalGroup.DELETE("/sources/:sourceId", geoRuleH.DeleteSource)
				geoRuleGlobalGroup.GET("/sources/:sourceId/assets", geoRuleH.FetchReleaseAssets)
				geoRuleGlobalGroup.POST("/sources/:sourceId/select", geoRuleH.SelectFiles)
				geoRuleGlobalGroup.POST("/sources/:sourceId/sync", geoRuleH.SyncSource)
				geoRuleGlobalGroup.PUT("/sources/:sourceId/schedule", geoRuleH.ToggleSchedule)
				geoRuleGlobalGroup.POST("/sync-all", geoRuleH.SyncAll)
				geoRuleGlobalGroup.GET("/sync-status/:key", geoRuleH.SyncStatus)
				geoRuleGlobalGroup.GET("/files", geoRuleH.ListFiles)
				geoRuleGlobalGroup.POST("/files/:fileId/download", geoRuleH.Download)
				// download-file is public (no auth) — Agent downloads geo rule files
				// from the panel using a relative URL, same as /agent/download.
				// Registered in the public v1 group below.
				geoRuleGlobalGroup.DELETE("/files/:fileId", geoRuleH.DeleteFile)
				geoRuleGlobalGroup.PUT("/files/:fileId/auto-sync", geoRuleH.SetAutoSync)
				geoRuleGlobalGroup.POST("/seed", geoRuleH.SeedDefaults)
				geoRuleGlobalGroup.GET("/status", geoRuleH.Status)
				geoRuleGlobalGroup.POST("/push-all", geoRuleH.PushToAllNodes)
			}

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
	r.GET("/sub/:token", middleware.MaintenanceGuard(), middleware.SubscribeRateLimit(), publicSubH.ServeSubscription)

	// Portal page — served from active custom template (cached)
	// Note: uses /p to avoid collision with admin SPA route /portal (PortalManagement.vue)
	// Guarded by maintenance mode so portal visitors see the maintenance page.
	r.GET("/p", middleware.MaintenanceGuard(), func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		html := portalTmplSvc.RenderPage(c.Request.Context())
		// 门户页面的安全响应头（自动；只设不影响自定义 HTML/JS/CDN 的那几条）
		for k, v := range portalSvc.SecurityHeaders() {
			c.Header(k, v)
		}
		c.Data(200, "text/html; charset=utf-8", []byte(html))
	})

	// Client auth (public)
	clientAuthGroup := r.Group("/api/v1/client-auth")
	{
		clientAuthGroup.POST("/register", middleware.RegisterRateLimit(), clientauthH.Register)
		clientAuthGroup.POST("/login", middleware.ClientLoginRateLimit(), clientauthH.Login)
		clientAuthGroup.POST("/refresh", middleware.RegisterRateLimit(), clientauthH.Refresh)

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
	r.GET("/api/v1/public/subscription-endpoints", middleware.MaintenanceGuard(), middleware.PublicAPIRateLimit(), func(c *gin.Context) {
		endpoints, err := subendRepoPortal.List(c.Request.Context())
		if err != nil {
			response.InternalError(c, "LIST_FAILED", err.Error())
			return
		}
		response.OK(c, endpoints)
	})

	// Public settings (for portal)
	// v3.4: portalTmplSvc used in public/settings to return latest portal config
	r.GET("/api/v1/public/settings", middleware.MaintenanceGuard(), middleware.PublicAPIRateLimit(), func(c *gin.Context) {
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
	r.GET("/api/v1/public/portal-config", middleware.MaintenanceGuard(), middleware.PublicAPIRateLimit(), func(c *gin.Context) {
		body, err := portalTmplSvc.GetPublicConfigJSON(c.Request.Context())
		if err != nil {
			response.InternalError(c, "PORTAL_CONFIG_FAILED", err.Error())
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
	})

	// Public articles (no auth required)
	r.GET("/api/v1/public/articles", middleware.MaintenanceGuard(), middleware.PublicAPIRateLimit(), clientExtH.PublicListArticles)
	r.GET("/api/v1/public/articles/:slug", middleware.MaintenanceGuard(), middleware.PublicAPIRateLimit(), clientExtH.PublicGetArticle)

	// Public: list enabled client configs (for portal "more subscriptions" dialog)
	mihomoCfgRepoPublic := repository.NewMihomoConfigRepository(database)
	subFmtRepoPublic := repository.NewSubscriptionFormatRepository(database)
	r.GET("/api/v1/public/client-configs", middleware.MaintenanceGuard(), middleware.PublicAPIRateLimit(), func(c *gin.Context) {
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
		// 客户端一次性短时票据（M10）：门户"下载日志包"用（window.open 无法带 Header）
		clientAuthProtected.POST("/ticket", clientauthH.IssueTicket)
		// 客户端登出：令牌版本 +1 → 此前签发的所有 token（含 refresh）立即失效
		clientAuthProtected.POST("/logout", clientauthH.Logout)
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
	// 进程控制 API 需要实时感知面板模式（保存设置后无需重启即可拦截本地核心 start/restart）
	processH.SetPanelModeFunc(func() string {
		m, _ := systemSettingSvc.GetByKey(context.Background(), "panel.mode")
		return m
	})

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
		// 纯管理模式：面板只做管理分发，不自动启动本机代理进程（设计 §4.4）
		if panelMode == "management_only" {
			log.Println("[Panel] MANAGEMENT_ONLY mode — local proxy (caddy/nginx) auto-start skipped")
		} else {
			proxyMgr.AutoStartManaged()
		}
		// Start auto-writers after proxy is started (broker is available)
		logManageSvc.StartAutoWriters()
	}()
	logManageSvc.Start(time.Duration(schedCfg.ClientLogCleanupInterval)*time.Minute, time.Duration(schedCfg.WAFLogCleanupInterval)*time.Minute)

	// Start config sync service (periodically syncs DB configs to disk)
	configSyncSvc.Start()

	// Start traffic sync service (periodically syncs local traffic stats to DB)
	trafficSyncSvc.Start()

	// Start log cleanup service (periodically removes old log/event records)
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
		"/chain-proxy",
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
		"/db-stats",
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
	var infos []firewall.InboundConfigInfo
	// 端口跳跃规则与核心无关（nft DNAT 端口段 → 入站端口），因此 xray 的 hysteria
	// 与 sing-box 的 hysteria2/hysteria 都要恢复；否则机器重启后 sing-box 的
	// 端口跳跃会静默失效（配置里 _portHop 还在，但规则没了）。
	targets := []struct {
		coreType  string
		protocols map[string]bool
	}{
		{"xray", map[string]bool{"hysteria": true}},
		{"singbox", map[string]bool{"hysteria2": true, "hysteria": true}},
	}
	for _, t := range targets {
		configs, err := repo.List(ctx, 1, t.coreType)
		if err != nil {
			log.Printf("[Firewall] failed to list %s inbound configs for rule restore: %v", t.coreType, err)
			continue
		}
		for _, c := range configs {
			if !c.Enabled || !t.protocols[c.Protocol] {
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
	r.GET("/:token", middleware.SubscribeRateLimit(), publicSubH.ServeSubscription)

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
	// 给库里已有的门户模板打 M7 安全补丁（用户内容不再执行 HTML）——全自动、幂等
	if perr := portalTmplSvc.EnsureSecurityPatches(context.Background()); perr != nil {
		log.Printf("[router] portal security patch skipped: %v", perr)
	}

	systemSettingSvc := systemSvc.NewSettingService(systemSettingRepo, nil)
	portalTmplSvc.WithSettingStore(systemSettingSvc)
	// 存量明文敏感配置统一加密落库（幂等，主面板引擎已执行过则这里直接跳过）
	if serr := systemSettingSvc.EncryptExistingSecrets(context.Background()); serr != nil {
		log.Printf("[router] portal: encrypt existing secrets skipped: %v", serr)
	}

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
		// 门户页面的安全响应头（自动；只设不影响自定义 HTML/JS/CDN 的那几条）
		for k, v := range portalSvc.SecurityHeaders() {
			c.Header(k, v)
		}
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
		clientAuthGroup.POST("/register", middleware.RegisterRateLimit(), clientauthH.Register)
		clientAuthGroup.POST("/login", middleware.ClientLoginRateLimit(), clientauthH.Login)
		clientAuthGroup.POST("/refresh", middleware.RegisterRateLimit(), clientauthH.Refresh)

		// OAuth2 登录路由（独立门户端口引擎也要有 ✓）：
		// 独立域名访问门户时 redirect_uri 按请求 Host 动态拼 ✓ → 在 GitHub/Google 后台
		// 注册"该域名"的回调即可 ✓（不必是面板域名 ✓）。
		// ⚠ 登录成功跳回的是**根路径 /** ✓（本引擎门户不挂在 /p ✓）→ SetPortalBasePath("/") ✓。
		oauthSvcPortal := oauthService.NewService(systemSettingSvc, repository.NewClientOAuthBindingRepository(database), clientRepo)
		oauthHPortal := oauthHandler.NewHandler(oauthSvcPortal, os.Getenv("JWT_SECRET"))
		oauthHPortal.SetPortalBasePath("/")
		clientAuthGroup.GET("/oauth/providers", oauthHPortal.ProvidersStatus)
		clientAuthGroup.GET("/oauth/:provider", oauthHPortal.Authorize)
		clientAuthGroup.GET("/oauth/:provider/callback", oauthHPortal.Callback)
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
		// 客户端一次性短时票据（M10）：门户"下载日志包"用（window.open 无法带 Header）
		clientAuthProtected.POST("/ticket", clientauthH.IssueTicket)
		// 客户端登出：令牌版本 +1 → 此前签发的所有 token（含 refresh）立即失效
		clientAuthProtected.POST("/logout", clientauthH.Logout)
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

// wafNodeSyncerAdapter adapts *agent.NodeServerManager to waf.NodeSyncer.
// It converts waf.WAFFile (service layer) to agentpb.WAFFile (proto layer)
// before forwarding to NodeServerManager.SyncWAFFiles.
type wafNodeSyncerAdapter struct {
	mgr *agent.NodeServerManager
}

func (a *wafNodeSyncerAdapter) SyncWAFFiles(ctx context.Context, serverID int64, files []*wafSvc.WAFFile) error {
	pbFiles := make([]*agentpb.WAFFile, 0, len(files))
	for _, f := range files {
		pbFiles = append(pbFiles, &agentpb.WAFFile{
			Path:    f.Path,
			Content: f.Content,
			Hash:    f.Hash,
		})
	}
	return a.mgr.SyncWAFFiles(ctx, serverID, pbFiles)
}

// wafNodeOperatorAdapter adapts nodeserver.Service + agent.NodeServerManager
// to wafHandler.NodeOperator. It delegates IsLocalServer to nodeserver.Service
// and converts waf.WAFFile to agentpb.WAFFile for SyncWAFFiles.
type wafNodeOperatorAdapter struct {
	svc *nodeServerSvc.Service
	mgr *agent.NodeServerManager
}

func (a *wafNodeOperatorAdapter) IsLocalServer(ctx context.Context, serverID int64) bool {
	if a.svc != nil {
		return a.svc.IsLocalServer(ctx, serverID)
	}
	if a.mgr != nil {
		return a.mgr.IsLocalServer(serverID)
	}
	return true
}

func (a *wafNodeOperatorAdapter) ReadConfigFileOnServer(ctx context.Context, serverID int64, coreType string) ([]byte, string, error) {
	if a.svc != nil {
		return a.svc.ReadConfigFileOnServer(ctx, serverID, coreType)
	}
	return nil, "", fmt.Errorf("node operator not configured")
}

func (a *wafNodeOperatorAdapter) SyncWAFFiles(ctx context.Context, serverID int64, files []*wafSvc.WAFFile) error {
	pbFiles := make([]*agentpb.WAFFile, 0, len(files))
	for _, f := range files {
		pbFiles = append(pbFiles, &agentpb.WAFFile{
			Path:    f.Path,
			Content: f.Content,
			Hash:    f.Hash,
		})
	}
	return a.mgr.SyncWAFFiles(ctx, serverID, pbFiles)
}

func (a *wafNodeOperatorAdapter) ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error {
	if a.svc != nil {
		return a.svc.ControlProxy(ctx, serverID, proxyType, action)
	}
	if a.mgr != nil {
		return a.mgr.ControlProxy(ctx, serverID, proxyType, action)
	}
	return fmt.Errorf("node operator not configured for ControlProxy")
}

func (a *wafNodeOperatorAdapter) GetInstalledProxyTypes(ctx context.Context, serverID int64) ([]string, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("node service not configured")
	}
	result, err := a.svc.GetProxyStatusRemote(ctx, serverID)
	if err != nil {
		return nil, err
	}
	resp, ok := result.(*agentpb.GetProxyStatusResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected proxy status response type")
	}
	var types []string
	for _, p := range resp.Proxies {
		if p.Installed {
			types = append(types, p.Type)
		}
	}
	return types, nil
}

// wafProxyReloaderAdapter adapts *proxy.Manager to wafHandler.ProxyReloader.
// proxy.Manager.ReloadProxy takes proxy.ProxyType (a string typedef),
// so we convert the string argument.
type wafProxyReloaderAdapter struct {
	mgr *proxySvc.Manager
}

func (a *wafProxyReloaderAdapter) DetectProxy(proxyType string) wafHandler.ProxyInfo {
	info := a.mgr.DetectProxy(proxySvc.ProxyType(proxyType))
	return wafHandler.ProxyInfo{
		Installed: info.Installed,
		Status:    info.Status,
	}
}

func (a *wafProxyReloaderAdapter) ReloadProxy(ctx context.Context, proxyType string) error {
	return a.mgr.ReloadProxy(ctx, proxySvc.ProxyType(proxyType))
}

func (a *wafProxyReloaderAdapter) SaveAndApply(ctx context.Context, proxyType string) error {
	return a.mgr.SaveAndApplyProxy(ctx, proxySvc.ProxyType(proxyType))
}

// autoDistributeCertFn 在"节点签好的证书落库"之后，把最新证书**同步给使用它的节点** ✓。
//
// ⚠ 用包级函数变量而不是结构体字段 ✗：适配器是以**值**传给 manager 的 ✓，
// 之后再改副本无效 ✗；函数变量在调用时读取 ✓，因此可以后置注入 ✓。
var autoDistributeCertFn func(ctx context.Context, certID int64)

// publicCertSinkAdapter 把 Agent 回传的公网证书接到证书服务的落盘落库逻辑上 ✓。
// 放在 router 包是为了避免 runtime/agent ↔ service/certificate 之间的循环依赖 ✗。
type publicCertSinkAdapter struct {
	svc *certService.Service
}

func (a publicCertSinkAdapter) StoreAgentIssuedCert(ctx context.Context, res *agent.PublicCertResult) (int64, error) {
	if a.svc == nil || res == nil {
		return 0, nil
	}
	c, err := a.svc.StoreAgentIssuedCert(ctx, certService.AgentIssuedCertInput{
		CertID:       res.CertID,
		Identifiers:  res.Identifiers,
		CertPEM:      res.CertPEM,
		KeyPEM:       res.KeyPEM,
		FullchainPEM: res.FullchainPEM,
		ExpiresAt:    res.ExpiresAt,
		CA:           res.CA,
		ServerID:     res.ServerID,
		// 挑战方式 + 面板 DNS 通道 id：节点回传的实际值 ✓ → 落库 = "创建时的选择" ✓
		//（续期时原样复用 ✓，见 certificates.challenge_type/dns_channel_id 的注释 ✓）
		ChallengeType: res.Challenge,
		DNSChannelID:  res.DNSChannelID,
	})
	if err != nil || c == nil {
		return 0, err
	}
	// 落库成功后**自动分发**给使用该证书的其它节点 ✓（异步 ✓，不拖慢节点回传 ✓）
	// 只有**面板持有 PEM** 的证书才自动分发 ✓ —— 节点上报的证书按新口径只留元数据 ✗，
	// 分发守卫会拒绝 ✓，没必要白跑一趟 ✓（也就不会在日志里刷无意义的失败 ✗）。
	if autoDistributeCertFn != nil && strings.TrimSpace(c.CertContent) != "" {
		go autoDistributeCertFn(context.Background(), c.ID)
	}
	// 续期/重报后还要刷新**面板本机**的副本 ✓（本机没有 sync ✗，不刷新的话本机内核会一直吐旧证书 ✗），
	// 并在刷新成功后**重启本机代理** ✓（caddy 缓存证书 ✗，只 reload 不生效 ✓）。
	if a.svc != nil {
		go func(id int64) {
			if lerr := a.svc.SyncLocalCopyIfLanded(context.Background(), id); lerr != nil {
				log.Printf("[cert] 刷新本机证书副本失败（cert %d）：%v", id, lerr)
				return
			}
			reloadLocalProxiesAfterCertRefresh(id)
		}(c.ID)
	}
	return c.ID, nil
}

// acmeEmailAdapter 把面板的 ACME 邮箱服务接成"解析邮箱"能力 ✓。
// 放在 router 包以避免 runtime/agent、api/handlers 与 service 之间的循环依赖 ✗。
type acmeEmailAdapter struct {
	svc *certService.ACMEEmailService
}

// ResolveACMEAccount 返回账号的邮箱 + CA + EAB ✓（签发时自动带上 ✓，不用手填 ✗）。
func (a acmeEmailAdapter) ResolveACMEAccount(ctx context.Context, id int64) (certsHandler.ACMEAccountInfo, error) {
	if a.svc == nil {
		return certsHandler.ACMEAccountInfo{}, fmt.Errorf("acme account service not available")
	}
	var (
		cfg *entity.ACMEAccountConfig
		err error
	)
	if id > 0 {
		cfg, err = a.svc.GetByID(ctx, id)
	} else {
		cfg, err = a.svc.GetDefault(ctx)
	}
	if err != nil || cfg == nil {
		return certsHandler.ACMEAccountInfo{}, err
	}
	return certsHandler.ACMEAccountInfo{
		Email:      cfg.Email,
		CAProvider: cfg.CAProvider,
		EABKeyID:   cfg.EABKeyID,
		EABMACKey:  cfg.EABMACKey,
	}, nil
}

func (a acmeEmailAdapter) ResolveACMEEmail(ctx context.Context, id int64) (string, error) {
	if a.svc == nil {
		return "", fmt.Errorf("acme email service not available")
	}
	if id > 0 {
		cfg, err := a.svc.GetByID(ctx, id)
		if err != nil {
			return "", err
		}
		return cfg.Email, nil
	}
	cfg, err := a.svc.GetDefault(ctx)
	if err != nil {
		return "", err
	}
	return cfg.Email, nil
}

// publicCertEnvAdapter 给"节点续期"准备注册上下文 ✓：
// 邮箱取面板的默认 ACME 账号 ✓，EAB 按 CA 自动关联 ✓（与签发路径同一套规则 ✓）。
type publicCertEnvAdapter struct {
	acmeSvc *certService.ACMEEmailService
	eabSvc  *certService.EABConfigService
}

func (a publicCertEnvAdapter) IssuanceContextFor(ctx context.Context, caProvider string) (string, string, string) {
	email := ""
	if a.acmeSvc != nil {
		if def, err := a.acmeSvc.GetDefault(ctx); err == nil && def != nil {
			email = strings.TrimSpace(def.Email)
		}
	}
	ca := strings.ToLower(strings.TrimSpace(caProvider))
	if a.eabSvc == nil || ca == "" {
		return email, "", ""
	}

	// ⚠ 节点侧必须带"**没用过的**"EAB ✗ ——
	// 目标节点可能是**从没注册过账号**的机器（例如 1.8 ✗），它第一次签发要真的去注册 ✓，
	// 而库里那把很可能**已经用来注册过别的账号、按 Google 的规则已作废** ✗。
	// 所以这里按 CA **现场申请一把新的** ✓（Google 用已存服务账号 ✓、ZeroSSL 用邮箱换 ✓）；
	// 拿不到新的（没授权/没邮箱）再退回库里那把 ✓（对已有账号的节点，反正也不会用 EAB ✓）。
	switch ca {
	case "google":
		if cfg, err := a.eabSvc.MintGoogleEAB(ctx); err == nil && cfg != nil && cfg.KeyID != "" {
			log.Printf("[router] node issuance: minted fresh Google EAB kid=%s", cfg.KeyID)
			return email, cfg.KeyID, cfg.MACKey
		} else if err != nil {
			log.Printf("[router] node issuance: mint Google EAB failed (%v), falling back to stored one", err)
		}
	case "zerossl":
		if strings.Contains(email, "@") {
			if cfg, err := a.eabSvc.FetchZeroSSLEAB(ctx, email); err == nil && cfg != nil && cfg.KeyID != "" {
				log.Printf("[router] node issuance: fetched fresh ZeroSSL EAB kid=%s", cfg.KeyID)
				return email, cfg.KeyID, cfg.MACKey
			} else if err != nil {
				log.Printf("[router] node issuance: fetch ZeroSSL EAB failed (%v), falling back to stored one", err)
			}
		}
	}

	cfg, err := a.eabSvc.GetByCA(ctx, ca)
	if err != nil || cfg == nil {
		return email, "", ""
	}
	return email, strings.TrimSpace(cfg.KeyID), strings.TrimSpace(cfg.MACKey)
}

// eabConfigAdapter 把面板的 EAB 独立配置服务接成"按 ID 解析 CA + EAB"能力 ✓。
type eabConfigAdapter struct {
	svc *certService.EABConfigService
}

// ResolveEABConfigForCA 按 CA 自动关联一份 EAB ✓（找不到返回 nil,nil ✓）。
func (a eabConfigAdapter) ResolveEABConfigForCA(ctx context.Context, caProvider string) (*certsHandler.EABConfigInfo, error) {
	if a.svc == nil {
		return nil, nil
	}
	cfg, err := a.svc.GetByCA(ctx, caProvider)
	if err != nil || cfg == nil {
		return nil, err
	}
	return &certsHandler.EABConfigInfo{
		Name:       cfg.Name,
		CAProvider: cfg.CAProvider,
		KeyID:      cfg.KeyID,
		MACKey:     cfg.MACKey,
	}, nil
}

// FetchGoogleEAB 用 GCP 授权创建一对 Google EAB ✓（必须授权 ✓，邮箱换不到 ✗）。
func (a eabConfigAdapter) FetchGoogleEAB(ctx context.Context, projectID, serviceAccountJSON, accessToken string) (*certsHandler.EABConfigInfo, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("eab config service not available")
	}
	cfg, err := a.svc.FetchGoogleEAB(ctx, projectID, serviceAccountJSON, accessToken)
	if err != nil || cfg == nil {
		return nil, err
	}
	return &certsHandler.EABConfigInfo{
		Name: cfg.Name, CAProvider: cfg.CAProvider, KeyID: cfg.KeyID, MACKey: cfg.MACKey,
	}, nil
}

// MintGoogleEAB 强制用已存服务账号申请一把全新的 Google EAB ✓（给节点首次注册用 ✓）。
func (a eabConfigAdapter) MintGoogleEAB(ctx context.Context) (*certsHandler.EABConfigInfo, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("eab config service not available")
	}
	cfg, err := a.svc.MintGoogleEAB(ctx)
	if err != nil || cfg == nil {
		return nil, err
	}
	return &certsHandler.EABConfigInfo{
		Name: cfg.Name, CAProvider: cfg.CAProvider, KeyID: cfg.KeyID, MACKey: cfg.MACKey,
	}, nil
}

// EnsureGoogleEAB 保证 Google 有一份可用 EAB ✓（缺了就用存过的服务账号现场再申请 ✓）。
func (a eabConfigAdapter) EnsureGoogleEAB(ctx context.Context) (*certsHandler.EABConfigInfo, bool, error) {
	if a.svc == nil {
		return nil, false, nil
	}
	cfg, fresh, err := a.svc.EnsureGoogleEAB(ctx)
	if err != nil || cfg == nil {
		return nil, false, err
	}
	return &certsHandler.EABConfigInfo{
		Name: cfg.Name, CAProvider: cfg.CAProvider, KeyID: cfg.KeyID, MACKey: cfg.MACKey,
	}, fresh, nil
}

// FetchZeroSSLEAB 用邮箱向 ZeroSSL 自动领 EAB ✓（首次申请即自动注册 ✓）。
func (a eabConfigAdapter) FetchZeroSSLEAB(ctx context.Context, email string) (*certsHandler.EABConfigInfo, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("eab config service not available")
	}
	cfg, err := a.svc.FetchZeroSSLEAB(ctx, email)
	if err != nil || cfg == nil {
		return nil, err
	}
	return &certsHandler.EABConfigInfo{
		Name:       cfg.Name,
		CAProvider: cfg.CAProvider,
		KeyID:      cfg.KeyID,
		MACKey:     cfg.MACKey,
	}, nil
}

func (a eabConfigAdapter) ResolveEABConfig(ctx context.Context, id int64) (*certsHandler.EABConfigInfo, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("eab config service not available")
	}
	cfg, err := a.svc.GetByID(ctx, id)
	if err != nil || cfg == nil {
		return nil, err
	}
	return &certsHandler.EABConfigInfo{
		Name:       cfg.Name,
		CAProvider: cfg.CAProvider,
		KeyID:      cfg.KeyID,
		MACKey:     cfg.MACKey,
	}, nil
}
