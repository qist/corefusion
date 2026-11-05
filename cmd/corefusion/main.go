package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"corefusion/internal/api/handlers/system"
	"corefusion/internal/api/middleware"
	"corefusion/internal/api/router"
	"corefusion/internal/auth/jwt"
	"corefusion/internal/constants"
	"corefusion/internal/migration"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/runtime/agent/ratelimit"
	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/runtime/process"
	secretcrypto "corefusion/internal/security/crypto"
	authService "corefusion/internal/service/auth"
	binariesSvc "corefusion/internal/service/binaries"
	"corefusion/internal/service/bootstrap"
	certService "corefusion/internal/service/certificate"
	planSvc "corefusion/internal/service/plan"
	ratelimitsvc "corefusion/internal/service/ratelimit"
	sysService "corefusion/internal/service/system"
	usersync "corefusion/internal/service/usersync"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

func main() {
	// 无参数或第一个参数不是已知命令时，默认启动 server
	if len(os.Args) < 2 {
		runServer([]string{})
		return
	}

	switch os.Args[1] {
	case "server":
		runServer(os.Args[2:])
	case "migrate":
		runMigrate(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "reset-init":
		runResetInit(os.Args[2:])
	case "admin":
		runAdmin(os.Args[2:])
	case "version", "--version", "-v", "-V":
		fmt.Printf("corefusion %s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		// 未知命令也默认启动 server（保留参数传递）
		runServer(os.Args[1:])
	}
}

func printUsage() {
	fmt.Print(`CoreFusion - Multi-protocol proxy management panel

Usage:
  corefusion                    Start the server management panel
  corefusion <command> [options]

Commands:
  admin     Offline admin commands (reset-password / list) — use when you cannot sign in
  init      Non-interactive first-boot initialization (CLI)
  reset-init  Reset first-boot initialization state (destructive, keeps data)
  migrate   Run database migrations
  version   Print version
  help      Show this help

`)
}

// loadEnv reads the .env file and returns a map of key-value pairs.
// It also populates os.Environ so that code using os.Getenv (e.g.
// CF_SECRET_ENCRYPTION_KEY, JWT_SECRET, CF_TRUSTED_PROXIES) can read values
// from .env. Existing system environment variables are NOT overwritten.
// hardenDotEnvPermissions 确保 `.env` 与 `.env.secret` 只有属主可读
// （安全审计 v5 P-4：它们含 JWT_SECRET / 加密密钥）。
//
// 放在 loadEnv 里是有意的：进程有多个启动入口（server / agent 等）都会先 loadEnv ✓，
// 挂在读取处能保证**每条启动路径**都收紧一次，不依赖谁记得调用 ✗。只在权限**过宽**时才 chmod ✗，
// 并用 sync.Once 避免重复系统调用。
func hardenDotEnvPermissions() {
	for _, path := range []string{".env", secretcrypto.SecretEnvFile} {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Mode().Perm()&0077 == 0 {
			continue // 已只有属主可访问
		}
		if err := os.Chmod(path, 0600); err != nil {
			log.Printf("[startup] warning: chmod %s failed: %v", path, err)
			continue
		}
		log.Printf("[startup] hardened %s permissions to 0600 (file contains secrets)", path)
	}
}

var (
	dotEnvHardened  sync.Once
	secretRelocated sync.Once
)

func loadEnv() map[string]string {
	dotEnvHardened.Do(hardenDotEnvPermissions)
	env := make(map[string]string)

	if f, err := os.Open(".env"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// System environment variables take precedence over .env file
				// values. If a key is already set in the system environment
				// (e.g. via `export CF_DB_DSN=mysql://...`), use that value
				// instead of the .env file value.
				if sysVal := os.Getenv(key); sysVal != "" {
					val = sysVal
				}
				env[key] = val
				// Populate os.Environ for keys not yet set.
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
	}

	// 独立密钥文件：.env 里没有该键时再从 .env.secret 读（.env 优先，保证存量行为不变）。
	for key := range secretcrypto.SecretEnvKeys {
		if env[key] != "" {
			continue
		}
		if v := secretcrypto.ReadSecretEnvValue(key); v != "" {
			env[key] = v
			if os.Getenv(key) == "" {
				os.Setenv(key, v)
			}
		}
	}

	// 存量部署：把 .env 里的密钥搬到 .env.secret（同值，幂等）。
	secretRelocated.Do(func() { secretcrypto.RelocateKeyToSecretFile("CF_SECRET_ENCRYPTION_KEY") })

	return env
}

// ensureEncryptionKey 确保进程有可用的加密主密钥（幂等）：
//  1. 系统环境变量 / .env / .env.secret 已有 → 直接复用；
//  2. 都没有 → 生成 32 字节随机密钥写入 .env.secret（失败回退 .env）。
//
// ⚠ 必须在 db.Init **之前**调用：RunDataMigrations 里有一趟"历史主机名派生
// 密钥加密的密文 → 用当前主密钥重写"。密钥没就绪时 getKey() 会退回 hostname
// 派生，迁移会拿错误的主密钥去判断（并把值重写成同样解不开的形式）。
func ensureEncryptionKey() {
	if os.Getenv("CF_SECRET_ENCRYPTION_KEY") == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("failed to generate secret encryption key: %v", err)
		}
		key := hex.EncodeToString(b)
		if err := secretcrypto.SaveSecretEnvValue("CF_SECRET_ENCRYPTION_KEY", key); err != nil {
			// 写独立文件失败时退回 .env，绝不因为落盘失败而让密钥只存在内存里
			// （重启后密钥丢失 = 库里所有密文报废）。
			log.Printf("[startup] warning: write %s failed (%v), falling back to .env", secretcrypto.SecretEnvFile, err)
			saveEnvValue("CF_SECRET_ENCRYPTION_KEY", key)
			log.Println("[startup] generated random CF_SECRET_ENCRYPTION_KEY and saved to .env")
		} else {
			log.Printf("[startup] generated random CF_SECRET_ENCRYPTION_KEY and saved to %s", secretcrypto.SecretEnvFile)
		}
		os.Setenv("CF_SECRET_ENCRYPTION_KEY", key)
	}

	// 密钥就绪后，把 env 文件里历史主机名密钥加密的值（典型：CF_DB_DSN）
	// 用当前主密钥重写。必须在 db.Init 之前做 —— DSN 要在打开连接前解开。
	for _, path := range []string{".env", secretcrypto.SecretEnvFile} {
		n, err := secretcrypto.ReencryptEnvFile(path)
		if err != nil {
			log.Printf("[startup] warning: re-encrypt %s failed: %v", path, err)
			continue
		}
		if n > 0 {
			log.Printf("[startup] re-encrypted %d legacy value(s) in %s with the current key", n, path)
		}
	}
}

// saveEnvValue updates or adds a key-value pair in the .env file.
func saveEnvValue(key, value string) {
	lines := []string{}
	found := false

	if data, err := os.ReadFile(".env"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				parts := strings.SplitN(trimmed, "=", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
					lines = append(lines, key+"="+value)
					found = true
					continue
				}
			}
			lines = append(lines, line)
		}
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	// .env 里可能含 JWT_SECRET / CF_SECRET_ENCRYPTION_KEY 等密钥 → 权限收到 0600 ✓
	// （安全审计 v5 P-4）。注意 WriteFile 对**已存在**的文件不会改权限 ✗ → 显式 Chmod 一次。
	envPath := ".env"
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		log.Printf("[startup] warning: write %s failed: %v", envPath, err)
	}
	if err := os.Chmod(envPath, 0600); err != nil {
		log.Printf("[startup] warning: chmod %s failed: %v", envPath, err)
	}
}

// resolveConfigDir 决定 .env / .env.secret 的基准目录：
//   - 当前目录已有 .env → 当前目录（systemd WorkingDirectory 的正常情况）；
//   - 否则可执行文件目录有 .env / .env.secret → 部署目录；
//   - 都没有 → 当前目录（首次部署，就在 cwd 生成）。
//
// 为什么必须锚定：.env / .env.secret 都是相对路径。手工在 /root 下跑
// `corefusion migrate` 时，cwd 没有 .env，旧逻辑会**静默生成一个不同的**密钥文件
// （/root/.env.secret），之后库里已有的 enc: 密文就再也解不开了。
func resolveConfigDir(cwd, exeDir string) string {
	if cwd == "" {
		cwd = "."
	}
	if _, err := os.Stat(filepath.Join(cwd, ".env")); err == nil {
		return cwd
	}
	if exeDir != "" {
		if absExe, err := filepath.Abs(exeDir); err == nil {
			exeDir = absExe
		}
		if absCwd, err := filepath.Abs(cwd); err == nil && absCwd == exeDir {
			return cwd
		}
		for _, name := range []string{".env", secretcrypto.SecretEnvFile} {
			if _, err := os.Stat(filepath.Join(exeDir, name)); err == nil {
				return exeDir
			}
		}
	}
	return cwd
}

// enterConfigDir 在读取 .env 之前把工作目录切到部署目录（见 resolveConfigDir）。
func enterConfigDir() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := resolveConfigDir(cwd, filepath.Dir(exe))
	if dir == cwd {
		return
	}
	if err := os.Chdir(dir); err != nil {
		log.Printf("[startup] warning: chdir %s failed: %v (relative .env/.env.secret stay in %s)", dir, err, cwd)
		return
	}
	log.Printf("[startup] no .env in %s; switched to deployment dir %s", cwd, dir)
}

func runServer(args []string) {
	// Record panel start time
	sysService.InitStartTime()

	// .env / .env.secret 都是相对路径：先确保在部署目录（systemd 下是 no-op）。
	enterConfigDir()

	// Phase 1: Read .env to get data directory
	env := loadEnv()
	dataDir := env["SAFE_PATHS"]
	if dataDir == "" {
		dataDir = "data"
	}

	// Convert to absolute path (relative to cwd, same as proxy)
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			dataDir = abs
		}
	}

	// 首次启动检测（设计 3.1）：未配置 SAFE_PATHS / CF_DB_DSN 且无既有部署 → 进入初始化向导，
	// 不启动正式面板。初始化完成后重跑本进程即走正常启动路径。
	if bootstrap.IsFirstBoot(env, dataDir) {
		log.Println("[startup] first boot detected; starting initialization wizard")
		runInitMode(env, dataDir)
		return
	}

	// Ensure data directory structure
	initDataDir(dataDir)

	// 加密密钥必须在**任何 DB 初始化/数据迁移之前**就绪：
	// 存量迁移里有一趟「历史主机名密钥加密的密文 → 用当前主密钥重写」，
	// 如果这时 env 里还没有密钥，getKey() 会用 hostname 派生去判断，
	// 迁移会误判（甚至把值写成 hostname 密钥后才发现刚生成了新密钥）。
	ensureEncryptionKey()

	// Phase 2: Initialize database (from .env config, default: SQLite)
	dbPath := filepath.Join(dataDir, "sqlite", "app.db")
	dbCfg := db.ConfigFromEnv(env, dbPath)
	if err := db.Init(dbCfg); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer db.Close()

	// Record startup DB config for change detection (used by /system/db-config API)
	db.RecordStartup(dbCfg)

	// Run migrations
	if err := migration.AutoMigrate(db.DB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Phase 2.5: Initialize internal CA (for Agent mTLS)
	caRepo := repository.NewCARootRepository(db.DB)
	caService, err := agent.EnsureCA(dataDir, caRepo)
	if err != nil {
		log.Fatalf("failed to initialize CA: %v", err)
	}

	// Initialize NodeServer repository
	nodeServerRepo := repository.NewNodeServerRepository(db.DB)
	// 节点 ↔ 证书 **多对多** ✓（一个节点可绑多张：IP 证书 + 域名证书 ✓）
	nodeCertRepo := repository.NewNodeCertificateRepository(db.DB)

	// Ensure local server record exists
	if err := nodeServerRepo.EnsureLocalServer(context.Background(), "local-panel"); err != nil {
		log.Printf("WARNING: ensure local server: %v", err)
	}

	// Create process.Manager — shared between main.go (LocalAgent) and router (HTTP API).
	// This is the single source of truth for core process lifecycle management.
	// The healthcheck.Guardian inside process.Manager provides auto-restart on crash.
	svcRepo := repository.NewServiceRepository(db.DB)
	guardianRepo := repository.NewProcessGuardianRepository(db.DB)
	crashLogRepo := repository.NewCrashLogRepository(db.DB)
	binaryResolver := binariesSvc.NewResolver(dataDir)
	processMgr := process.NewManager(dataDir, svcRepo, guardianRepo, crashLogRepo, binaryResolver)

	// Initialize NodeServerManager (gRPC server for Agent connections)
	nodeMgr := agent.NewNodeServerManager(nodeServerRepo, caService)
	nodeMgr.SetNodeCertificates(nodeCertRepo)

	// Cache the local server ID to avoid DB queries on every NodeOperator method call.
	nodeMgr.InitLocalServerID(context.Background())

	// Inject LocalAgent — wraps the existing process.Manager for local node operations.
	// Local server (is_local=1) operations go through process.Manager directly,
	// not through gRPC. The Agent's role is to GUARD (守护) core processes,
	// and process.Manager + Guardian already implements this completely.
	localAgent := agent.NewLocalAgent(dataDir, processMgr, svcRepo)

	// 本地节点限速（in-process Agent）：挂载限速管理器，与远程节点同一套
	// 探测/启用/IP 关联路径（此前限速只覆盖远程节点 ✗）。
	// 初始化（iface 探测/恢复/清理循环）在 ratelimitsvc.InitLocalBackend；
	// ⚠ router 的 fallback LocalAgent 构造路径也必须挂载（见 router.go）。
	localAgent = localAgent.WithRateLimitManager(ratelimit.NewManager(""))
	localAgent = localAgent.WithRateLimitInitFunc(func(ctx context.Context, maxDown, maxUp int32) error {
		return ratelimitsvc.InitLocalBackend(ctx, db.DB, localAgent, maxDown, maxUp)
	})
	// 面板启动时若本地限速已启用 → 自动初始化
	go func() {
		if err := ratelimitsvc.InitLocalBackend(context.Background(), db.DB, localAgent, 0, 0); err != nil {
			log.Printf("[RateLimitService] local rate limit not initialized at startup: %v", err)
		}
	}()
	nodeMgr.SetLocalAgent(localAgent)
	// 同步给 router 包：确保 HTTP API / 订阅推送路径与 gRPC 侧使用**同一个** LocalAgent。
	// 否则 router 的 fallback 会创建第二个实例（旧版本无限速管理器）并通过
	// SetLocalAgent 覆盖 nodeMgr 的实例 → 本地 IP 关联报 "rate limit manager not configured"。
	router.SetLocalAgent(localAgent)

	// Initialize Cert Revocation List (CRL)
	revocationRepo := repository.NewCertRevocationRepository(db.DB)
	crl := agent.NewCertRevocationList(revocationRepo, nodeServerRepo, nodeMgr, caService)
	nodeMgr.WithCRL(crl)

	// Start gRPC server for Agents (mTLS)
	// Default port 18443; can be changed via system settings (panel.agent_grpc_port).
	// After changing, restart the panel for it to take effect.
	agentGRPCPort := constants.DefaultAgentGRPCPort
	// Try read from system_settings (DB already initialized above)
	grpcSettingRepo := repository.NewSystemSettingRepository(db.DB)
	if v, err := grpcSettingRepo.GetByKey(context.Background(), "panel.agent_grpc_port"); err == nil && v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			agentGRPCPort = p
		}
	}
	agentGRPCAddr := fmt.Sprintf(":%d", agentGRPCPort)
	log.Printf("[Panel] Agent gRPC server starting on %s", agentGRPCAddr)
	if err := nodeMgr.StartGRPCServer(context.Background(), agentGRPCAddr); err != nil {
		log.Printf("WARNING: failed to start Agent gRPC server: %v", err)
	} else {
		defer nodeMgr.StopGRPCServer()
	}

	// Start CRL refresh loop (background)
	go crl.RunRefreshLoop(context.Background())

	// Initialize CertDistributor (external TLS cert distribution to Agents)
	certRepo := repository.NewCertificateRepository(db.DB)
	_ = agent.NewCertDistributor(certRepo, nodeServerRepo, nodeMgr) // wired in router.go per-request

	// Start Agent certificate auto-renewer (background goroutine)
	// Checks every 6 hours, renews certs 3 days before expiry.
	// Also wired into NodeServerManager so certs are renewed on Agent reconnect.
	certRenewer := agent.NewCertRenewer(caService, nodeServerRepo).WithPusher(nodeMgr)
	nodeMgr.WithCertRenewer(certRenewer)
	go certRenewer.RunLoop(context.Background())
	log.Println("Agent certificate auto-renewer started (interval=6h, renew-before=3d)")

	// 公网证书（ACME 签的域名/IP 证书）自动续期 ✓ —— 与上面那条区分开 ✗：
	// 上面那个续的是 **Agent 的 mTLS 证书**（面板自签 CA ✓），这条续的是**对外的公网证书** ✓。
	// 由绑定节点的 Agent 在**节点本机**签发（面板在 SNAT 后面时 CA 回连不到面板 ✗）。
	// 把历史上的单值 `node_servers.certificate_id` 搬进多对多表 ✓（幂等 ✓，每次启动都可跑 ✓）
	if n, merr := nodeCertRepo.MigrateLegacyCertificateID(context.Background()); merr != nil {
		log.Printf("[Panel] migrate legacy node certificates failed: %v", merr)
	} else if n > 0 {
		log.Printf("[Panel] migrated %d legacy node↔certificate bindings into node_certificates", n)
	}
	publicCertRenewer := agent.NewPublicCertRenewer(certRepo, nodeServerRepo, nodeMgr).
		WithNodeCertificates(nodeCertRepo).
		// ⚠ 必须把 DNS 通道解析器交给**自动续期** ✗：不开这个口子，
		// 创建时选的 dns-01 在自动续期里会丢掉 ✗（节点侧默认 http-01 ✓ → 通配符证书续不出来 ✓）。
		// 用同一个通道服务（无状态 ✓，只包一层 repo ✓），保证与签发走**同一条解析路径** ✓。
		WithDNSChannelResolver(certService.NewDNSChannelService(repository.NewDNSChannelRepository(db.DB)))
	nodeMgr.WithPublicCertRenewer(publicCertRenewer)
	go publicCertRenewer.RunLoop(context.Background())
	log.Println("Public certificate auto-renewer started (interval=6h, renew-before=5d, node-side ACME)")

	// Phase 3: Start FailoverManager, MirrorSync, and ChainHealth monitors
	inboundRepoForPhase3 := repository.NewInboundConfigRepository(db.DB)
	failoverMgr := agent.NewFailoverManager(nodeServerRepo, nodeMgr)
	go failoverMgr.RunLoop(context.Background())
	log.Println("Failover manager started (primary_backup auto-switch, per-pair recover mode)")

	mirrorSyncMgr := agent.NewMirrorSyncManager(nodeServerRepo, inboundRepoForPhase3, nodeMgr).
		SetSettingRepository(grpcSettingRepo)
	go mirrorSyncMgr.RunLoop(context.Background())
	log.Println("Mirror sync manager started (config replication)")

	chainHealthRegistry := agent.NewChainHealthMonitorRegistry(nodeMgr, nodeServerRepo)
	go func() {
		if err := chainHealthRegistry.StartAll(context.Background()); err != nil {
			log.Printf("chain health registry: %v", err)
		}
	}()
	log.Println("Chain health monitor registry started (multi-exit failover)")

	// Phase 4: Start UserSyncService and MultiServerTrafficCollector
	billingFilter := agent.NewBillingFilter(nodeServerRepo)
	clientRepo := repository.NewClientRepository(db.DB)
	extNodeRepoForSync := repository.NewExternalNodeRepository(db.DB)

	// Plan v2 service for node resolution (used by usersync)
	planTmplRepoForSync := repository.NewPlanTemplateRepository(db.DB)
	planVariantRepoForSync := repository.NewPlanVariantRepository(db.DB)
	nodeGroupRepoForSync := repository.NewNodeGroupRepository(db.DB)
	nodeGroupMemberRepoForSync := repository.NewNodeGroupMemberRepository(db.DB)
	promoRepoForSync := repository.NewPromotionRepository(db.DB)
	planOrderRepoForSync := repository.NewPlanOrderRepository(db.DB)
	planSvcInst := planSvc.NewService(db.DB, planTmplRepoForSync, planVariantRepoForSync, nodeGroupRepoForSync, nodeGroupMemberRepoForSync, promoRepoForSync, planOrderRepoForSync, clientRepo, inboundRepoForPhase3, extNodeRepoForSync)

	userSyncSvc := usersync.NewUserSyncService(
		clientRepo, planSvcInst, inboundRepoForPhase3,
		nodeServerRepo, nodeMgr, billingFilter,
	)
	go userSyncSvc.RunRetryLoop(context.Background())
	log.Println("User sync service started (multi-server fan-out + offline retry)")

	// 注入 UserSyncService 到 router（供 CommerceService 使用）
	// 套餐激活后自动同步用户凭证到所有节点 + 退款时移除用户
	router.SetUserSyncer(userSyncSvc)

	// Wire UserSyncService into the FailoverManager so that failover/recovery
	// replays user credentials onto the newly active primary (Phase 4).
	failoverMgr.SetUserSync(userSyncSvc)

	// NOTE: MultiServerTrafficCollector was removed because it competed with
	// TrafficEventConsumer (started in router.go) for the same onTraffic channel.
	// Having two consumers on the same channel caused every other traffic event
	// to be consumed by MultiServerTrafficCollector (which submits to Aggregator)
	// instead of TrafficEventConsumer (which updates inbound_configs._usedUp/_usedDown).
	// This resulted in ~50% of remote node traffic being lost from the DB.
	// TrafficEventConsumer in router.go now handles all remote traffic events.
	// trafficAggregator := traffic.NewAggregator(db.DB)
	// trafficAggregator.Start(context.Background())
	// multiCollector := traffic.NewMultiServerTrafficCollector(nodeMgr, nodeServerRepo, trafficAggregator)
	// multiCollector.Start(context.Background())
	// log.Println("Multi-server traffic collector started (remote + mirror merge)")

	// Phase 5: Start MultiServerHealthChecker
	multiHealthChecker := healthcheck.NewMultiServerHealthChecker(nodeMgr, nodeServerRepo)
	go multiHealthChecker.RunLoop(context.Background())
	log.Println("Multi-server health checker started (30s interval + alerts)")

	// Phase 3: Read panel settings from database
	settingRepo := repository.NewSystemSettingRepository(db.DB)
	settings := readSettings(settingRepo)

	// Sync data_dir in DB with .env (env is source of truth)
	if dbDataDir := settings["data_dir"]; dbDataDir != dataDir {
		settingRepo.Set(nil, "panel.data_dir", dataDir)
		settings["data_dir"] = dataDir
	}

	log.Printf("Data directory: %s", dataDir)

	// Initialize trusted proxies for real IP extraction
	initTrustedProxies(settings)

	// Phase 4: Setup JWT
	// JWT_SECRET: auto-generate a random secret on first run if not set.
	// This avoids the insecure default "corefusion-dev-secret".
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("failed to generate JWT secret: %v", err)
		}
		jwtSecret = hex.EncodeToString(b)
		saveEnvValue("JWT_SECRET", jwtSecret)
		os.Setenv("JWT_SECRET", jwtSecret)
		log.Println("[startup] generated random JWT_SECRET and saved to .env")
	}
	jwtHandler := jwt.New(jwtSecret, 0, 0)
	middleware.SetJWT(jwtHandler)

	// CF_SECRET_ENCRYPTION_KEY 已在 db.Init 之前由 ensureEncryptionKey() 就绪
	// （存量密文重加密迁移依赖它）。这里不再重复生成。

	// Create initial admin
	//
	// 安全审计 v5 S4：这里原本写死 "admin"/"admin123" ✗ —— 新部署即存在可预测口令。
	// 现在：① CF_INITIAL_ADMIN_PASSWORD 可显式指定；② 否则**随机生成并只打印一次**。
	// ⚠ 已有用户时 CreateInitialAdmin 直接返回"已存在"，**不会**改动现有口令 ✓
	//   （静默改口令会把现有部署挡在门外）；已有部署里若仍是弱默认口令，仅打印告警（见下）。
	userRepo := repository.NewUserRepository(db.DB)
	authSvc := authService.NewService(userRepo, jwtSecret)
	initialPw, generated := resolveInitialAdminPassword(os.Getenv)
	if err := authSvc.CreateInitialAdmin(context.Background(), "admin", initialPw); err != nil {
		log.Printf("initial admin: %v", err)
	} else if generated {
		log.Printf("==================================================================")
		log.Printf("[startup] 已创建初始管理员 admin，随机口令（**仅显示这一次**，请立即登录并修改）：")
		log.Printf("[startup]     %s", initialPw)
		log.Printf("[startup] 如需自定义，可用环境变量 %s 指定。", initialAdminPasswordEnv)
		log.Printf("==================================================================")
	} else {
		log.Printf("[startup] 已创建初始管理员 admin（口令来自 %s）", initialAdminPasswordEnv)
	}
	warnIfWeakAdminPassword(context.Background(), userRepo, "admin")

	// Phase 5: Build listen address (from database settings only; PORT env removed)
	listenAddr := ":8080"
	if addr := settings["listen_addr"]; addr != "" {
		listenAddr = addr + ":"
	}
	if port := settings["listen_port"]; port != "" {
		if listenAddr[len(listenAddr)-1] == ':' {
			listenAddr = listenAddr + port
		}
	}

	// Extract port for internal API calls
	apiPort := "8080"
	if idx := strings.LastIndex(listenAddr, ":"); idx >= 0 {
		if p := listenAddr[idx+1:]; p != "" {
			apiPort = p
		}
	}

	// Phase 6: Create router
	router.SetCAService(caService)
	router.SetProcessManager(processMgr) // share processMgr with router (HTTP API handlers)
	router.SetNodeServerManager(nodeMgr) // share nodeMgr with router (same gRPC server instance)
	r := router.NewWithFullConfig(db.DB, jwtSecret, settings["web_path"], dataDir, Version, apiPort)

	// ⚠ Gin 默认 `trustedProxies = 0.0.0.0/0`（信任所有代理）→ `c.ClientIP()` 直接采信
	// `X-Forwarded-For`，可被任意伪造（安全审计 v5 S3）。这里用与 netutil.GetRealIP **同一份**
	// 配置（panel.trusted_proxies）设置，保证两处口径一致；未配置时传 nil = 只信任直连地址。
	if proxies := netutil.TrustedProxyStrings(); len(proxies) > 0 {
		if err := r.SetTrustedProxies(proxies); err != nil {
			log.Printf("[panel] SetTrustedProxies(%v) failed: %v", proxies, err)
		}
	} else if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("[panel] SetTrustedProxies(nil) failed: %v", err)
	}

	// Load maintenance mode from database (survives restarts)
	system.LoadMaintenanceFromDB(context.Background(), grpcSettingRepo)

	// Phase 7: TLS configuration
	tlsEnabled := settings["tls_enabled"] == "true"
	// 证书/密钥路径可能以 data 目录为基准存的是相对路径（证书库 cert.CertPath、
	// 初始化落库都是这种形态）；直接按进程 cwd 打开会找不到 ——
	// 生产踩过：cwd=/opt/corefusion 却去开 certs/<ip>/cert.pem。
	tlsCertPath := resolveDataPath(dataDir, settings["tls_cert_path"])
	tlsKeyPath := resolveDataPath(dataDir, settings["tls_key_path"])
	mtlsEnabled := settings["mtls_enabled"] == "true"
	mtlsCAPath := resolveDataPath(dataDir, settings["mtls_ca_path"])

	if tlsEnabled && (tlsCertPath == "" || tlsKeyPath == "") {
		log.Println("WARNING: TLS enabled but cert/key paths not set, falling back to HTTP")
		tlsEnabled = false
	}

	// 面板 / 订阅 / 门户**共用同一份 TLS 材料**：证书配在面板上时，独立端口的订阅站
	// 与门户站也加载同一张证书（复用面板证书，含 IP 证书）。
	//
	// ⚠ 证书与 mTLS 客户端 CA 必须**热加载** ✗：证书中心续期/重签后文件已换新，
	// 但旧实现只在启动时 LoadX509KeyPair 一次 ✗ → 不重启面板就一直用旧证书 ✗。
	var panelTLSSvc *panelTLS
	if tlsEnabled {
		var tlsErr error
		// mtlsCAPath 为空 = 没配 CA → 按原行为跳过 mTLS（不阻断启动）
		panelTLSSvc, tlsErr = newPanelTLS(tlsCertPath, tlsKeyPath, mtlsCAPath, mtlsEnabled && mtlsCAPath != "")
		if tlsErr != nil {
			log.Fatalf("failed to load TLS certificate: %v", tlsErr)
		}
		// 换证书路径 / 开关 mTLS 也要免重启生效（证书内容变化本身由文件时间戳即时感知）
		go panelTLSSvc.watchSettings(settingRepo, dataDir)
	}

	// Phase 8: Start server with graceful shutdown
	// WriteTimeout is set to 5 minutes to accommodate long-running operations
	// such as binary downloads (~20s), process stop/start, and certificate
	// generation. ReadTimeout stays at 30s since clients should send requests
	// quickly. IdleTimeout is 120s for keep-alive connections.
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       120 * time.Second,
	}

	// 订阅 / 门户独立端口与面板**监听同一地址**：面板 127.0.0.1（走反向代理）时它们
	// 也只在回环，由代理对外；面板 0.0.0.0 / :: 时对外直连。
	bindHost := strings.TrimSpace(settings["listen_addr"])
	bindAddr := func(port string) string {
		if bindHost == "" {
			return ":" + port
		}
		if strings.Contains(bindHost, ":") {
			return "[" + bindHost + "]:" + port
		}
		return bindHost + ":" + port
	}

	// Start subscription server on dedicated port if configured
	subPort := settings["subscription_port"]
	var subSrv *http.Server
	if subPort != "" && subPort != "0" && subPort != apiPort {
		subEngine := router.NewSubscriptionEngine(db.DB)
		subSrv = &http.Server{
			Addr:              bindAddr(subPort),
			Handler:           subEngine,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       120 * time.Second,
		}
		if panelTLSSvc != nil {
			subSrv.TLSConfig = panelTLSSvc.Config()
		}
		go func() {
			log.Printf("Subscription server starting on :%s (tls=%v, reuses panel cert)", subPort, tlsEnabled)
			subLn, lnErr := netutil.ListenTCP(subSrv.Addr)
			if lnErr != nil {
				log.Printf("subscription server listen error: %v", lnErr)
				return
			}
			var err error
			if tlsEnabled {
				err = subSrv.ServeTLS(subLn, "", "")
			} else {
				err = subSrv.Serve(subLn)
			}
			if err != nil && err != http.ErrServerClosed {
				log.Printf("subscription server error: %v", err)
			}
		}()
	}

	// Start portal server on dedicated port if configured (跳过前提：门户总开关未关闭)
	portalPort := settings["portal_port"]
	portalSwitchedOff := settings["portal_enabled"] == "false"
	var portalSrv *http.Server
	if !portalSwitchedOff && portalPort != "" && portalPort != "0" && portalPort != apiPort {
		portalEngine := router.NewPortalEngine(db.DB)
		portalSrv = &http.Server{
			Addr:              bindAddr(portalPort),
			Handler:           portalEngine,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       120 * time.Second,
		}
		if panelTLSSvc != nil {
			portalSrv.TLSConfig = panelTLSSvc.Config()
		}
		go func() {
			log.Printf("Portal server starting on :%s (tls=%v, reuses panel cert)", portalPort, tlsEnabled)
			portalLn, lnErr := netutil.ListenTCP(portalSrv.Addr)
			if lnErr != nil {
				log.Printf("portal server listen error: %v", lnErr)
				return
			}
			var err error
			if tlsEnabled {
				err = portalSrv.ServeTLS(portalLn, "", "")
			} else {
				err = portalSrv.Serve(portalLn)
			}
			if err != nil && err != http.ErrServerClosed {
				log.Printf("portal server error: %v", err)
			}
		}()
	}

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("shutting down...")
		router.Shutdown()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		if subSrv != nil {
			subSrv.Shutdown(ctx)
		}
		if portalSrv != nil {
			portalSrv.Shutdown(ctx)
		}
	}()

	// 面板主端口：显式禁用 MPTCP（见 netutil.ListenTCP，CVE-2026-98432 加固）
	mainLn, listenErr := netutil.ListenTCP(listenAddr)
	if listenErr != nil {
		log.Fatalf("failed to listen on %s: %v", listenAddr, listenErr)
	}
	if tlsEnabled {
		srv.TLSConfig = panelTLSSvc.Config()
		log.Printf("CoreFusion %s starting on %s (HTTPS)", Version, listenAddr)
		if err := srv.ServeTLS(mainLn, "", ""); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	} else {
		log.Printf("CoreFusion %s starting on %s (HTTP)", Version, listenAddr)
		if err := srv.Serve(mainLn); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	}
}

// initDataDir creates the data directory structure.
func initDataDir(dataDir string) {
	dirs := []string{
		filepath.Join(dataDir, "sqlite"),
		filepath.Join(dataDir, "certs"),
		filepath.Join(dataDir, "certs", "mtls"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "backup"),
		filepath.Join(dataDir, "runtime"),
		filepath.Join(dataDir, "rules"),
		filepath.Join(dataDir, "rules_tmp"),
		filepath.Join(dataDir, "binaries", "active"),
		filepath.Join(dataDir, "binaries", "backup"),
		filepath.Join(dataDir, "cache"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0750)
	}
}

// initTrustedProxies configures the trusted proxy list for netutil.GetRealIP.
// Reads from database setting panel.trusted_proxies only (CF_TRUSTED_PROXIES env removed).
// Format: comma-separated IPs/CIDRs, or "cloudflare" for all Cloudflare ranges.
// Loopback (127.0.0.1, ::1) is ALWAYS trusted: the panel manages a same-host
// Caddy/Nginx reverse proxy by design, and without it audit logs would record
// 127.0.0.1 for every request. User-configured entries are appended.
func initTrustedProxies(settings map[string]string) {
	// 回环始终可信：同机反代（Caddy/Nginx）是面板标准部署形态。
	proxies := []string{"127.0.0.1", "::1"}

	raw := settings["trusted_proxies"]
	if raw == "" {
		log.Println("[netutil] no extra trusted proxies configured — loopback (127.0.0.1, ::1) trusted for same-host reverse proxy")
	} else {
		entries := strings.Split(raw, ",")
		for _, e := range entries {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			if strings.EqualFold(e, "cloudflare") {
				proxies = append(proxies, netutil.CloudflareProxyRanges()...)
				continue
			}
			proxies = append(proxies, e)
		}
	}
	netutil.SetTrustedProxies(proxies)
}

// resolveDataPath 把"相对 data 目录"的路径还原成可直接打开的路径：
// 绝对路径与空串原样返回；相对路径优先拼到 dataDir 下，拼出来的不存在时
// 再按进程 cwd 试一次（兼容历史把路径写成相对 cwd 的部署）。
func resolveDataPath(dataDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	dd := strings.TrimSpace(dataDir)
	if dd != "" {
		joined := filepath.Join(dd, p)
		if fileExists(joined) {
			return joined
		}
	}
	if fileExists(p) {
		return p
	}
	if dd != "" {
		return filepath.Join(dd, p)
	}
	return p
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// readSettings reads all panel settings from database.
func readSettings(repo *repository.SystemSettingRepository) map[string]string {
	keys := []string{
		"panel.listen_addr", "panel.listen_port", "panel.subscription_port", "panel.portal_port", "panel.portal_enabled",
		"panel.tls_enabled", "panel.tls_cert_path", "panel.tls_key_path",
		"panel.mtls_enabled", "panel.mtls_ca_path",
		"panel.web_path", "panel.timezone", "panel.data_dir",
		"panel.trusted_proxies",
	}
	settings := make(map[string]string)
	for _, key := range keys {
		if v, err := repo.GetByKey(nil, key); err == nil && v != "" {
			settings[key[len("panel."):]] = v
		}
	}
	return settings
}

func runMigrate(args []string) {
	enterConfigDir()
	env := loadEnv()
	dataDir := env["SAFE_PATHS"]
	if dataDir == "" {
		dataDir = "data"
	}
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			dataDir = abs
		}
	}

	dbCfg := db.ConfigFromEnv(env, filepath.Join(dataDir, "sqlite", "app.db"))
	if err := db.Init(dbCfg); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer db.Close()

	if err := migration.AutoMigrate(db.DB); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations completed successfully")
}
