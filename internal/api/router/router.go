package router

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	authHandler "corefusion/internal/api/handlers/auth"
	backupsHandler "corefusion/internal/api/handlers/backups"
	binariesHandler "corefusion/internal/api/handlers/binaries"
	certsHandler "corefusion/internal/api/handlers/certificates"
	geoRuleHandler "corefusion/internal/api/handlers/georule"
	logManageHandler "corefusion/internal/api/handlers/logmanage"
	metricsHandler "corefusion/internal/api/handlers/metrics"
	nodesHandler "corefusion/internal/api/handlers/nodes"
	securityHandler "corefusion/internal/api/handlers/security"
	servicesHandler "corefusion/internal/api/handlers/services"
	subsHandler "corefusion/internal/api/handlers/subscriptions"
	systemHandler "corefusion/internal/api/handlers/system"
	usersHandler "corefusion/internal/api/handlers/users"
	warpHandler "corefusion/internal/api/handlers/warp"
	proxySvc "corefusion/internal/service/proxy"
	"corefusion/internal/api/middleware"
	"corefusion/internal/app/embed"
	"corefusion/internal/runtime/process"
	authService "corefusion/internal/service/auth"
	binariesSvc "corefusion/internal/service/binaries"
	certService "corefusion/internal/service/certificate"
	geoRuleSvc "corefusion/internal/service/georule"
	logManageSvc "corefusion/internal/service/logmanage"
	nodeService "corefusion/internal/service/node"
	svcService "corefusion/internal/service/service"
	subService "corefusion/internal/service/subscription"
	systemSvc "corefusion/internal/service/system"
	userService "corefusion/internal/service/user"
	backupService "corefusion/internal/service/backup"
	warpSvc "corefusion/internal/service/warp"
	"corefusion/internal/store/repository"
)

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

// NewWithFullConfig creates a new Gin engine with full config including data directory and version.
func NewWithFullConfig(database *gorm.DB, jwtSecret string, webPath string, dataDir string, version string) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())

	if dataDir == "" {
		dataDir = "data"
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

	// Initialize services
	authSvc := authService.NewService(userRepo, jwtSecret)
	userSvc := userService.NewService(userRepo)
	nodeSvc := nodeService.NewService(nodeRepo)
	svcSvc := svcService.NewService(svcRepo, endpointRepo)
	subSvc := subService.NewService(subRepo, accessLogRepo, rotationRepo)
	certSvc := certService.NewService(certRepo, dataDir, "", "lets_encrypt")
	// Sync cert content from DB to disk on startup
	certSvc.SyncCertsToDisk(nil)
	binaryResolver := binariesSvc.NewResolver(dataDir)
	systemSettingSvc := systemSvc.NewSettingService(systemSettingRepo, notifTmplRepo)
	geoRuleSvcInst := geoRuleSvc.NewService(ruleSourceRepo, geoRuleRepo, dataDir, systemSettingSvc)
	geoRuleScheduler := geoRuleSvc.NewScheduler(ruleSourceRepo, geoRuleSvcInst)
	warpSvcInst := warpSvc.NewService(warpRepo, dataDir)
	dnsChannelSvc := certService.NewDNSChannelService(dnsChannelRepo)
	acmeEmailSvc := certService.NewACMEEmailService(acmeAccountRepo)
	notifier := systemSvc.NewNotifier(systemSettingSvc)
	backupSvcInst := backupService.NewService(backupRepo, dataDir)
	_ = notifier // TODO: integrate with scheduler for template notifications
	proxySiteRepo := repository.NewProxySiteRepository(database)
	caddySiteRepo := repository.NewCaddySiteRepository(database)
	nginxSiteRepo := repository.NewNginxSiteRepository(database)
	proxyMgr := proxySvc.NewManager(dataDir, proxySiteRepo, systemSettingRepo)
	caddyMgr := proxySvc.NewCaddyManager(dataDir, caddySiteRepo, systemSettingRepo)
	nginxMgr := proxySvc.NewNginxManager(dataDir, nginxSiteRepo, systemSettingRepo)

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

	// Seed default geo rules
	geoRuleSvcInst.SeedDefaults(nil)

	// Start rule sync scheduler
	geoRuleScheduler.Start()

	// Initialize handlers
	authH := authHandler.NewHandler(authSvc)
	userH := usersHandler.NewHandler(userSvc)
	nodeH := nodesHandler.NewHandler(nodeSvc)
	svcH := servicesHandler.NewHandler(svcSvc)
	subH := subsHandler.NewHandler(subSvc)
	certH := certsHandler.NewHandler(certSvc)
	processMgr := process.NewManager(dataDir, svcRepo, guardianRepo, crashLogRepo, binaryResolver)
	processH := servicesHandler.NewProcessHandler(processMgr, svcRepo, crashLogRepo, dataDir)
	metricsH := metricsHandler.NewHandler(database)
	securityH := securityHandler.NewHandler(database)
	backupsH := backupsHandler.NewHandler(database, dataDir, backupSvcInst)
	systemH := systemHandler.NewHandler(database, systemSettingSvc, dataDir, version)
	proxyH := systemHandler.NewProxyHandler(proxyMgr, caddyMgr, nginxMgr)
	caddyH := systemHandler.NewCaddyHandler(caddyMgr, proxyMgr)
	nginxH := systemHandler.NewNginxHandler(nginxMgr, proxyMgr)
	binariesH := binariesHandler.NewHandler(binaryResolver, dataDir, systemSettingSvc)
	geoRuleH := geoRuleHandler.NewHandler(geoRuleSvcInst, geoRuleScheduler)
	warpH := warpHandler.NewHandler(warpSvcInst)
	certChannelH := certsHandler.NewChannelHandler(dnsChannelSvc)
	certEmailH := certsHandler.NewEmailHandler(acmeEmailSvc)
	logManageH := logManageHandler.NewHandler(logManageSvc)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

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

			// Services
			servicesGroup := protected.Group("/services")
			{
				servicesGroup.GET("", svcH.List)
				servicesGroup.POST("", svcH.Create)
				servicesGroup.GET("/:id", svcH.GetByID)
				servicesGroup.PUT("/:id", svcH.Update)
				servicesGroup.DELETE("/:id", svcH.Delete)
				servicesGroup.GET("/:id/endpoints", svcH.ListEndpoints)
				servicesGroup.POST("/:id/endpoints", svcH.CreateEndpoint)
				servicesGroup.PUT("/:id/endpoints/:endpointId", svcH.UpdateEndpoint)
				servicesGroup.DELETE("/:id/endpoints/:endpointId", svcH.DeleteEndpoint)

				// Process management
				servicesGroup.POST("/:id/start", processH.Start)
				servicesGroup.POST("/:id/stop", processH.Stop)
				servicesGroup.POST("/:id/restart", processH.Restart)
				servicesGroup.GET("/:id/process-status", processH.GetProcessStatus)
				servicesGroup.PUT("/:id/guardian", processH.UpdateGuardian)
				servicesGroup.GET("/:id/crash-logs", processH.GetCrashLogs)
				servicesGroup.GET("/:id/crash-logs/:logId", processH.GetCrashLog)
				servicesGroup.DELETE("/:id/crash-logs", processH.ClearCrashLogs)

				// Process status for all services
				servicesGroup.GET("/process-status", processH.ListAllProcessStatuses)
			}

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

			// Binaries
			binariesGroup := protected.Group("/binaries")
			{
				binariesGroup.GET("", binariesH.List)
				binariesGroup.GET("/releases", binariesH.ListReleases)
				binariesGroup.POST("/download", binariesH.Download)
				binariesGroup.POST("/install", binariesH.Install)
				binariesGroup.POST("/update", binariesH.Update)
				binariesGroup.POST("/rollback", binariesH.Rollback)
			}

			// Geo Rules
			geoRulesGroup := protected.Group("/geo-rules")
			{
				geoRulesGroup.GET("/github-assets", geoRuleH.FetchGitHubAssets)
				geoRulesGroup.GET("/sources", geoRuleH.ListSources)
				geoRulesGroup.POST("/sources", geoRuleH.CreateSource)
				geoRulesGroup.PUT("/sources/:id", geoRuleH.UpdateSource)
				geoRulesGroup.DELETE("/sources/:id", geoRuleH.DeleteSource)
				geoRulesGroup.GET("/sources/:id/assets", geoRuleH.FetchReleaseAssets)
				geoRulesGroup.POST("/sources/:id/select", geoRuleH.SelectFiles)
				geoRulesGroup.POST("/sources/:id/sync", geoRuleH.SyncSource)
				geoRulesGroup.PUT("/sources/:id/schedule", geoRuleH.ToggleSchedule)
				geoRulesGroup.POST("/sync-all", geoRuleH.SyncAll)
				geoRulesGroup.GET("/sync-status/:key", geoRuleH.SyncStatus)
				geoRulesGroup.GET("/files", geoRuleH.ListFiles)
				geoRulesGroup.POST("/files/:id/download", geoRuleH.Download)
				geoRulesGroup.DELETE("/files/:id", geoRuleH.DeleteFile)
				geoRulesGroup.PUT("/files/:id/auto-sync", geoRuleH.SetAutoSync)
			}

			// WARP
			warpGroup := protected.Group("/warp")
			{
				warpGroup.POST("/register", warpH.Register)
				warpGroup.GET("/status", warpH.Status)
				warpGroup.POST("/generate-config", warpH.GenerateConfig)
				warpGroup.GET("/config", warpH.GetConfig)
				warpGroup.DELETE("", warpH.Reset)
			}

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
				systemGroup.GET("/notification-templates", systemH.ListTemplates)
				systemGroup.POST("/notification-templates", systemH.CreateTemplate)
				systemGroup.PUT("/notification-templates/:id", systemH.UpdateTemplate)
				systemGroup.DELETE("/notification-templates/:id", systemH.DeleteTemplate)
			}

			// Proxy management
			proxyGroup := protected.Group("/proxy")
			{
				proxyGroup.GET("/status", proxyH.GetStatus)
				proxyGroup.POST("/install", proxyH.Install)
				proxyGroup.GET("/releases", proxyH.ListReleases)
				proxyGroup.POST("/download", proxyH.DownloadVersion)
				proxyGroup.POST("/start", proxyH.Start)
				proxyGroup.POST("/stop", proxyH.Stop)
				proxyGroup.POST("/restart", proxyH.Restart)
				proxyGroup.POST("/reload", proxyH.Reload)
				proxyGroup.GET("/sites", proxyH.ListSites)
				proxyGroup.POST("/sites", proxyH.CreateSite)
				proxyGroup.PUT("/sites/:id", proxyH.UpdateSite)
				proxyGroup.DELETE("/sites/:id", proxyH.DeleteSite)
				proxyGroup.POST("/generate-config", proxyH.GenerateConfig)
				proxyGroup.GET("/config", proxyH.GetConfig)
				proxyGroup.PUT("/config", proxyH.UpdateConfig)
				proxyGroup.GET("/sni-settings", proxyH.GetSNISettings)
				proxyGroup.POST("/sni-settings", proxyH.SaveSNISettings)
				proxyGroup.POST("/batch-update-ports", proxyH.BatchUpdatePorts)
				proxyGroup.DELETE("/sni-rules", proxyH.DeleteAllSNI)
				proxyGroup.POST("/sni-rules/toggle", proxyH.ToggleSNI)
				proxyGroup.GET("/l4-settings", proxyH.GetL4Settings)
				proxyGroup.POST("/l4-settings", proxyH.SaveL4Settings)
				proxyGroup.DELETE("/l4-rules", proxyH.DeleteAllL4)
				proxyGroup.POST("/guardian", proxyH.UpdateGuardian)
				proxyGroup.GET("/log-settings", proxyH.GetLogSettings)
				proxyGroup.POST("/log-settings", proxyH.SaveLogSettings)
				proxyGroup.GET("/nginx-settings", proxyH.GetNginxSettings)
				proxyGroup.POST("/nginx-settings", proxyH.SaveNginxSettings)
			}

			// Caddy management (separated)
			caddyGroup := protected.Group("/caddy")
			{
				caddyGroup.GET("/sites", caddyH.ListSites)
				caddyGroup.POST("/sites", caddyH.CreateSite)
				caddyGroup.PUT("/sites/:id", caddyH.UpdateSite)
				caddyGroup.DELETE("/sites/:id", caddyH.DeleteSite)
				caddyGroup.POST("/generate-config", caddyH.GenerateConfig)
				caddyGroup.POST("/save-and-apply", caddyH.SaveAndApply)
				caddyGroup.GET("/sni-settings", caddyH.GetSNISettings)
				caddyGroup.POST("/sni-settings", caddyH.SaveSNISettings)
				caddyGroup.POST("/sni-rules/toggle", caddyH.ToggleSNI)
				caddyGroup.DELETE("/sni-rules", caddyH.DeleteAllSNI)
				caddyGroup.POST("/batch-update-ports", caddyH.BatchUpdatePorts)
				caddyGroup.DELETE("/l4-rules", caddyH.DeleteAllL4)
			}

			// Nginx management (separated)
			nginxGroup := protected.Group("/nginx")
			{
				nginxGroup.GET("/sites", nginxH.ListSites)
				nginxGroup.POST("/sites", nginxH.CreateSite)
				nginxGroup.PUT("/sites/:id", nginxH.UpdateSite)
				nginxGroup.DELETE("/sites/:id", nginxH.DeleteSite)
				nginxGroup.POST("/generate-config", nginxH.GenerateConfig)
				nginxGroup.POST("/save-and-apply", nginxH.SaveAndApply)
				nginxGroup.GET("/settings", nginxH.GetSettings)
				nginxGroup.POST("/settings", nginxH.SaveSettings)
				nginxGroup.POST("/sni-rules/toggle", nginxH.ToggleSNI)
				nginxGroup.DELETE("/sni-rules", nginxH.DeleteAllSNI)
				nginxGroup.POST("/batch-update-ports", nginxH.BatchUpdatePorts)
				nginxGroup.DELETE("/l4-rules", nginxH.DeleteAllL4)
			}

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

			// Placeholder routes for remaining modules
			registerPlaceholderRoutes(protected)
		}
	}

	// Public subscription endpoint
	r.GET("/sub/:token", placeholder("get subscription"))

	// Seed default cores and reconcile processes on startup
	processMgr.SeedCores(nil)
	go processMgr.Reconcile(nil)

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
func isSPARoute(path string) bool {
	// Known SPA routes
	knownRoutes := []string{
		"/",
		"/login",
		"/dashboard",
		"/nodes",
		"/services",
		"/subscriptions",
		"/certificates",
		"/users",
		"/proxy",
		"/log-manage",
		"/settings",
	}

	for _, route := range knownRoutes {
		if path == route {
			return true
		}
	}

	return false
}
