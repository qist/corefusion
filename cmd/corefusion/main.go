package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"corefusion/internal/api/handlers/system"
	"corefusion/internal/api/middleware"
	"corefusion/internal/api/router"
	"corefusion/internal/auth/jwt"
	"corefusion/internal/constants"
	"corefusion/internal/migration"
	"corefusion/internal/runtime/agent"
	"corefusion/internal/runtime/healthcheck"
	"corefusion/internal/runtime/process"
	authService "corefusion/internal/service/auth"
	binariesSvc "corefusion/internal/service/binaries"
	planSvc "corefusion/internal/service/plan"
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
	case "version":
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
  migrate   Run database migrations
  version   Print version
  help      Show this help

`)
}

// loadEnv reads the .env file and returns a map of key-value pairs.
// It also populates os.Environ so that code using os.Getenv (e.g.
// CF_SECRET_ENCRYPTION_KEY, JWT_SECRET, CF_TRUSTED_PROXIES) can read values
// from .env. Existing system environment variables are NOT overwritten.
func loadEnv() map[string]string {
	env := make(map[string]string)
	f, err := os.Open(".env")
	if err != nil {
		return env
	}
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
	return env
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

func runServer(args []string) {
	// Record panel start time
	sysService.InitStartTime()

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

	// Ensure data directory structure
	initDataDir(dataDir)

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

	// Cache the local server ID to avoid DB queries on every NodeOperator method call.
	nodeMgr.InitLocalServerID(context.Background())

	// Inject LocalAgent — wraps the existing process.Manager for local node operations.
	// Local server (is_local=1) operations go through process.Manager directly,
	// not through gRPC. The Agent's role is to GUARD (守护) core processes,
	// and process.Manager + Guardian already implements this completely.
	localAgent := agent.NewLocalAgent(dataDir, processMgr, svcRepo)
	nodeMgr.SetLocalAgent(localAgent)

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
	tlsCertPath := settings["tls_cert_path"]
	tlsKeyPath := settings["tls_key_path"]
	mtlsEnabled := settings["mtls_enabled"] == "true"
	mtlsCAPath := settings["mtls_ca_path"]

	if tlsEnabled && (tlsCertPath == "" || tlsKeyPath == "") {
		log.Println("WARNING: TLS enabled but cert/key paths not set, falling back to HTTP")
		tlsEnabled = false
	}

	// Phase 8: Start server with graceful shutdown
	// WriteTimeout is set to 5 minutes to accommodate long-running operations
	// such as binary downloads (~20s), process stop/start, and certificate
	// generation. ReadTimeout stays at 30s since clients should send requests
	// quickly. IdleTimeout is 120s for keep-alive connections.
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	// Start subscription server on dedicated port if configured
	subPort := settings["subscription_port"]
	var subSrv *http.Server
	if subPort != "" && subPort != "0" && subPort != apiPort {
		subEngine := router.NewSubscriptionEngine(db.DB)
		subSrv = &http.Server{
			Addr:         ":" + subPort,
			Handler:      subEngine,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 5 * time.Minute,
			IdleTimeout:  120 * time.Second,
		}
		go func() {
			log.Printf("Subscription server starting on :%s", subPort)
			if err := subSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("subscription server error: %v", err)
			}
		}()
	}

	// Start portal server on dedicated port if configured
	portalPort := settings["portal_port"]
	var portalSrv *http.Server
	if portalPort != "" && portalPort != "0" && portalPort != apiPort {
		portalEngine := router.NewPortalEngine(db.DB)
		portalSrv = &http.Server{
			Addr:         ":" + portalPort,
			Handler:      portalEngine,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 5 * time.Minute,
			IdleTimeout:  120 * time.Second,
		}
		go func() {
			log.Printf("Portal server starting on :%s", portalPort)
			if err := portalSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	if tlsEnabled {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}

		if mtlsEnabled && mtlsCAPath != "" {
			caCert, err := os.ReadFile(mtlsCAPath)
			if err != nil {
				log.Printf("WARNING: failed to read mTLS CA: %v, skipping mTLS", err)
			} else {
				pool := x509.NewCertPool()
				if pool.AppendCertsFromPEM(caCert) {
					tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
					tlsCfg.ClientCAs = pool
					log.Printf("mTLS enabled, CA: %s", mtlsCAPath)
				} else {
					log.Println("WARNING: failed to parse mTLS CA cert")
				}
			}
		}

		srv.TLSConfig = tlsCfg
		log.Printf("CoreFusion %s starting on %s (HTTPS)", Version, listenAddr)
		if err := srv.ListenAndServeTLS(tlsCertPath, tlsKeyPath); err != nil {
			log.Fatalf("failed to start server: %v", err)
		}
	} else {
		log.Printf("CoreFusion %s starting on %s (HTTP)", Version, listenAddr)
		if err := srv.ListenAndServe(); err != nil {
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
// If not set, no headers are trusted (only RemoteAddr is used — safest default).
func initTrustedProxies(settings map[string]string) {
	raw := settings["trusted_proxies"]
	if raw == "" {
		log.Println("[netutil] no trusted proxies configured — only RemoteAddr will be used for IP extraction")
		return
	}

	entries := strings.Split(raw, ",")
	var proxies []string
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
	if len(proxies) > 0 {
		netutil.SetTrustedProxies(proxies)
	} else {
		log.Println("[netutil] trusted_proxies set but no valid entries found")
	}
}

// readSettings reads all panel settings from database.
func readSettings(repo *repository.SystemSettingRepository) map[string]string {
	keys := []string{
		"panel.listen_addr", "panel.listen_port", "panel.subscription_port", "panel.portal_port",
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

func runClient(args []string) {
	fmt.Println("client mode not yet implemented")
}

func runMigrate(args []string) {
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
