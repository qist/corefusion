package bootstrap

import (
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	"corefusion/internal/app/embed"
	corecert "corefusion/internal/runtime/cert"
	certsvc "corefusion/internal/service/certificate"
	"corefusion/internal/store/db"
)

// Server 是首次启动初始化向导服务（独立端口 + init_token，设计 3.2/3.5）。
type Server struct {
	dataDir string
	session *Session
	// certSvc 是面板既有的证书服务：向导的 HTTPS 配置复用它来落盘/注册证书。
	certSvc *certsvc.Service
	// dnsSvc / eabSvc：dns-01 接入与 EAB 自动申请（可为 nil）。
	dnsSvc *certsvc.DNSChannelService
	eabSvc *certsvc.EABConfigService
	// onComplete 在初始化完成后被调用（通常触发进程重启）。
	onComplete func()

	mu        sync.Mutex
	draft     *Draft
	completed bool
}

// NewServer 创建初始化向导服务。
func NewServer(dataDir string, sess *Session, certSvc *certsvc.Service, dnsSvc *certsvc.DNSChannelService, eabSvc *certsvc.EABConfigService, onComplete func()) *Server {
	return &Server{
		dataDir:    dataDir,
		session:    sess,
		certSvc:    certSvc,
		dnsSvc:     dnsSvc,
		eabSvc:     eabSvc,
		onComplete: onComplete,
		draft:      NewDraft(dataDir),
	}
}

// Session 返回当前初始化会话（供启动横幅打印访问地址/令牌）。
func (s *Server) Session() *Session { return s.session }

// Engine 构建初始化向导的 HTTP 引擎。
func (s *Server) Engine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "initialized": false, "init_mode": true})
	})

	grp := r.Group("/api/v1/init")
	grp.Use(s.authMiddleware())
	{
		grp.GET("/status", s.handleStatus)
		grp.GET("/random-ports", s.handleRandomPorts)
		grp.POST("/config/db", s.handleDB)
		grp.POST("/config/db/test", s.handleDBTest)
		grp.POST("/config/datadir", s.handleDataDir)
		grp.POST("/config/env", s.handleEnv)
		grp.POST("/config/core", s.handleCore)
		grp.POST("/config/proxy", s.handleProxy)
		grp.POST("/config/sites", s.handleSites)
		grp.POST("/config/admin", s.handleAdmin)
		grp.POST("/config/https", s.handleHTTPS)
		grp.GET("/dns-providers", s.handleDNSProviders)
		grp.GET("/public-ip", s.handlePublicIP)
		grp.GET("/summary", s.handleSummary)
		grp.GET("/env", s.handleEnvFile)
		grp.POST("/complete", s.handleComplete)
	}

	r.NoRoute(s.serveSPA)
	return r
}

// authMiddleware 校验初始化令牌与会话有效期（设计 3.5.1）。
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.session.Expired() {
			response.Error(c, http.StatusUnauthorized, "INIT_SESSION_EXPIRED", "init session expired; restart the panel to begin again")
			c.Abort()
			return
		}
		tok := c.Query("init_token")
		if tok == "" {
			tok = c.GetHeader("X-Init-Token")
		}
		if tok != s.session.Token {
			response.Error(c, http.StatusUnauthorized, "INIT_TOKEN_INVALID", "invalid init token")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) snapshot() Draft {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.draft
}

func (s *Server) update(fn func(d *Draft)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.draft)
}

// handleStatus 返回初始化状态与当前草稿（前端据此恢复进度）。
func (s *Server) handleStatus(c *gin.Context) {
	d := s.snapshot()
	response.OK(c, gin.H{
		"init_mode":          true,
		"completed":          s.completed,
		"listen_addr":        d.ResolvedListenAddr(),
		"session_expires_in": int(s.session.Remaining().Seconds()),
		"supported_cores":    SupportedCores,
		"draft":              d,
	})
}

// handleRandomPorts 返回 n 个空闲端口，供向导的「随机端口」按钮使用。
func (s *Server) handleRandomPorts(c *gin.Context) {
	n := 1
	if v, err := strconv.Atoi(c.DefaultQuery("n", "1")); err == nil {
		n = v
	}
	if n < 1 {
		n = 1
	}
	if n > 16 {
		n = 16
	}
	ports, err := RandomFreePorts(n)
	if err != nil {
		response.InternalError(c, "PORT_ALLOC_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"ports": ports})
}

// handleDB 保存数据库配置（步骤 1）。
func (s *Server) handleDB(c *gin.Context) {
	var req struct {
		DBType       *string `json:"db_type"`
		DBHost       *string `json:"db_host"`
		DBPort       *string `json:"db_port"`
		DBUser       *string `json:"db_user"`
		DBPassword   *string `json:"db_password"`
		DBName       *string `json:"db_name"`
		DBSSLMode    *string `json:"db_sslmode"`
		DBSQLitePath *string `json:"db_sqlite_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	s.update(func(d *Draft) {
		if req.DBType != nil {
			d.DBType = strings.ToLower(strings.TrimSpace(*req.DBType))
		}
		if req.DBHost != nil {
			d.DBHost = strings.TrimSpace(*req.DBHost)
		}
		if req.DBPort != nil {
			d.DBPort = strings.TrimSpace(*req.DBPort)
		}
		if req.DBUser != nil {
			d.DBUser = strings.TrimSpace(*req.DBUser)
		}
		if req.DBPassword != nil {
			d.DBPassword = *req.DBPassword
		}
		if req.DBName != nil {
			d.DBName = strings.TrimSpace(*req.DBName)
		}
		if req.DBSSLMode != nil {
			d.DBSSLMode = strings.TrimSpace(*req.DBSSLMode)
		}
		if req.DBSQLitePath != nil {
			d.DBSQLitePath = strings.TrimSpace(*req.DBSQLitePath)
		}
	})
	d := s.snapshot()
	// 校验数据库段落（不校验整个草稿，允许后续步骤尚未填写）。
	if err := validateDB(d); err != nil {
		response.BadRequest(c, "INVALID_DB", err.Error())
		return
	}
	// mysql/pg：保存时即测试连接；库不存在则用同一账号自动创建（有 CREATE 权限时）。
	if d.DBType == db.DBTypeMySQL || d.DBType == db.DBTypePostgres {
		cfg := d.DBConfig(s.dataDir)
		if err := db.TestConnection(cfg); err != nil {
			if cerr := db.CreateDatabase(cfg, d.DBName); cerr != nil {
				response.BadRequest(c, "DB_UNREACHABLE",
					fmt.Sprintf("连接失败：%v；自动创建数据库失败：%v（可手动建库后重试）", err, cerr))
				return
			}
			if rerr := db.TestConnection(cfg); rerr != nil {
				response.BadRequest(c, "DB_UNREACHABLE",
					fmt.Sprintf("数据库 %q 已自动创建但仍连不上: %v", d.DBName, rerr))
				return
			}
			response.OK(c, gin.H{"message": fmt.Sprintf("database saved; database %q did not exist and was created automatically", d.DBName)})
			return
		}
	}
	response.OK(c, gin.H{"message": "database saved"})
}

// handleDBTest 测试数据库连接（向导"测试连接"按钮）：用表单当前值直接测试，不保存草稿。
// mysql/pg 库不存在时尝试自动创建（成功会在结果里标注 db_created）。
func (s *Server) handleDBTest(c *gin.Context) {
	var req struct {
		DBType       *string `json:"db_type"`
		DBHost       *string `json:"db_host"`
		DBPort       *string `json:"db_port"`
		DBUser       *string `json:"db_user"`
		DBPassword   *string `json:"db_password"`
		DBName       *string `json:"db_name"`
		DBSSLMode    *string `json:"db_sslmode"`
		DBSQLitePath *string `json:"db_sqlite_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	// 以当前草稿为底、表单值覆盖（未提交的字段沿用草稿），不落草稿。
	d := s.snapshot()
	if req.DBType != nil {
		d.DBType = strings.ToLower(strings.TrimSpace(*req.DBType))
	}
	if req.DBHost != nil {
		d.DBHost = strings.TrimSpace(*req.DBHost)
	}
	if req.DBPort != nil {
		d.DBPort = strings.TrimSpace(*req.DBPort)
	}
	if req.DBUser != nil {
		d.DBUser = strings.TrimSpace(*req.DBUser)
	}
	if req.DBPassword != nil {
		d.DBPassword = *req.DBPassword
	}
	if req.DBName != nil {
		d.DBName = strings.TrimSpace(*req.DBName)
	}
	if req.DBSSLMode != nil {
		d.DBSSLMode = strings.TrimSpace(*req.DBSSLMode)
	}
	if req.DBSQLitePath != nil {
		d.DBSQLitePath = strings.TrimSpace(*req.DBSQLitePath)
	}
	if err := validateDB(d); err != nil {
		response.BadRequest(c, "INVALID_DB", err.Error())
		return
	}
	result := gin.H{"connected": true, "db_created": false}
	if d.DBType == db.DBTypeMySQL || d.DBType == db.DBTypePostgres {
		cfg := d.DBConfig(s.dataDir)
		if err := db.TestConnection(cfg); err != nil {
			if cerr := db.CreateDatabase(cfg, d.DBName); cerr != nil {
				response.BadRequest(c, "DB_UNREACHABLE",
					fmt.Sprintf("连接失败：%v；自动创建数据库失败：%v（可手动建库后重试）", err, cerr))
				return
			}
			if rerr := db.TestConnection(cfg); rerr != nil {
				response.BadRequest(c, "DB_UNREACHABLE",
					fmt.Sprintf("数据库 %q 已自动创建但仍连不上: %v", d.DBName, rerr))
				return
			}
			result["db_created"] = true
		}
	}
	result["message"] = "connection ok"
	response.OK(c, result)
}

// validateDB 只校验数据库相关字段。
func validateDB(d Draft) error {
	switch d.DBType {
	case "sqlite", "":
		return nil
	case "mysql", "postgres":
		if err := validateHost("db_host", d.DBHost); err != nil {
			return err
		}
		if err := ValidatePort("db_port", d.DBPort, true); err != nil {
			return err
		}
		if strings.TrimSpace(d.DBUser) == "" {
			return fmt.Errorf("db_user is required")
		}
		if strings.TrimSpace(d.DBName) == "" {
			return fmt.Errorf("db_name is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported db_type: %s", d.DBType)
	}
}

// handleDataDir 保存数据目录（步骤 2）。
func (s *Server) handleDataDir(c *gin.Context) {
	var req struct {
		DataDir *string `json:"data_dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	if req.DataDir == nil || strings.TrimSpace(*req.DataDir) == "" {
		response.BadRequest(c, "INVALID_REQUEST", "data_dir is required")
		return
	}
	s.update(func(d *Draft) { d.DataDir = strings.TrimSpace(*req.DataDir) })
	response.OK(c, gin.H{"data_dir": strings.TrimSpace(*req.DataDir)})
}

// handleEnv 生成/保存 env 加密密钥（步骤 3）。
func (s *Server) handleEnv(c *gin.Context) {
	var req struct {
		JWTSecret           *string `json:"jwt_secret"`
		SecretEncryptionKey *string `json:"secret_encryption_key"`
		AutoGenerate        *bool   `json:"auto_generate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	s.update(func(d *Draft) {
		if req.AutoGenerate != nil {
			d.AutoGenerateSecrets = *req.AutoGenerate
		}
		if req.JWTSecret != nil && strings.TrimSpace(*req.JWTSecret) != "" {
			d.JWTSecret = strings.TrimSpace(*req.JWTSecret)
			d.AutoGenerateSecrets = false
		}
		if req.SecretEncryptionKey != nil && strings.TrimSpace(*req.SecretEncryptionKey) != "" {
			d.SecretEncryptionKey = strings.TrimSpace(*req.SecretEncryptionKey)
			d.AutoGenerateSecrets = false
		}
		if d.AutoGenerateSecrets || d.JWTSecret == "" {
			d.JWTSecret = RandomHex(32)
		}
		if d.AutoGenerateSecrets || d.SecretEncryptionKey == "" {
			d.SecretEncryptionKey = RandomHex(32)
		}
	})
	d := s.snapshot()
	response.OK(c, gin.H{"jwt_secret": d.JWTSecret, "secret_encryption_key": d.SecretEncryptionKey})
}

// handleCore 保存核心安装选择（步骤 4）。
func (s *Server) handleCore(c *gin.Context) {
	var req struct {
		Cores []string `json:"cores"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	cores := make([]string, 0, len(req.Cores))
	for _, c0 := range req.Cores {
		if !IsSupportedCore(c0) {
			response.BadRequest(c, "INVALID_CORE", "unsupported core: "+c0)
			return
		}
		cores = append(cores, strings.ToLower(strings.TrimSpace(c0)))
	}
	s.update(func(d *Draft) { d.Cores = cores })
	response.OK(c, gin.H{"cores": cores})
}

// handleProxy 保存代理选择（步骤 5）：nginx / caddy 二选一（或 none）。
func (s *Server) handleProxy(c *gin.Context) {
	var req struct {
		ProxyType string `json:"proxy_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	p := strings.ToLower(strings.TrimSpace(req.ProxyType))
	switch p {
	case "", ProxyNone, ProxyNginx, ProxyCaddy:
	default:
		response.BadRequest(c, "INVALID_PROXY", "proxy_type must be none, nginx or caddy")
		return
	}
	s.update(func(d *Draft) { d.ProxyType = p })
	response.OK(c, gin.H{"proxy_type": p})
}

// handleSites 保存面板/订阅/门户端口与访问路径、域名、以及"添加到代理"开关（步骤 6）。
func (s *Server) handleSites(c *gin.Context) {
	var req struct {
		ListenPort         *string `json:"listen_port"`
		SubscriptionPort   *string `json:"subscription_port"`
		PortalPort         *string `json:"portal_port"`
		WebPath            *string `json:"web_path"`
		PanelDomain        *string `json:"panel_domain"`
		SubscriptionDomain *string `json:"subscription_domain"`
		PortalDomain       *string `json:"portal_domain"`
		AddPanel           *bool   `json:"add_panel"`
		AddSubscription    *bool   `json:"add_subscription"`
		AddPortal          *bool   `json:"add_portal"`
		DefaultCurrency    *string `json:"default_currency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	s.update(func(d *Draft) {
		if req.ListenPort != nil {
			d.ListenPort = strings.TrimSpace(*req.ListenPort)
		}
		if req.SubscriptionPort != nil {
			d.SubscriptionPort = strings.TrimSpace(*req.SubscriptionPort)
		}
		if req.PortalPort != nil {
			d.PortalPort = strings.TrimSpace(*req.PortalPort)
		}
		if req.WebPath != nil {
			d.WebPath = strings.TrimSpace(*req.WebPath)
		}
		if req.PanelDomain != nil {
			d.PanelDomain = strings.TrimSpace(*req.PanelDomain)
		}
		if req.SubscriptionDomain != nil {
			d.SubscriptionDomain = strings.TrimSpace(*req.SubscriptionDomain)
		}
		if req.PortalDomain != nil {
			d.PortalDomain = strings.TrimSpace(*req.PortalDomain)
		}
		if req.AddPanel != nil {
			d.ProxyAddPanel = *req.AddPanel
		}
		if req.AddSubscription != nil {
			d.ProxyAddSubscription = *req.AddSubscription
		}
		if req.AddPortal != nil {
			d.ProxyAddPortal = *req.AddPortal
		}
		if req.DefaultCurrency != nil {
			d.DefaultCurrency = strings.ToUpper(strings.TrimSpace(*req.DefaultCurrency))
		}
	})
	d := s.snapshot()
	if err := ValidatePort("listen_port", d.ListenPort, true); err != nil {
		response.BadRequest(c, "INVALID_PORT", err.Error())
		return
	}
	if err := ValidatePort("subscription_port", d.SubscriptionPort, false); err != nil {
		response.BadRequest(c, "INVALID_PORT", err.Error())
		return
	}
	if err := ValidatePort("portal_port", d.PortalPort, false); err != nil {
		response.BadRequest(c, "INVALID_PORT", err.Error())
		return
	}
	if !webPathPattern.MatchString(d.WebPath) {
		response.BadRequest(c, "INVALID_PATH", "web_path must start with '/' and contain only letters, digits, '-' or '_'")
		return
	}
	for _, v := range []struct{ n, v string }{
		{"panel_domain", d.PanelDomain},
		{"subscription_domain", d.SubscriptionDomain},
		{"portal_domain", d.PortalDomain},
	} {
		if strings.TrimSpace(v.v) != "" {
			if err := validateHost(v.n, normalizeHost(v.v)); err != nil {
				response.BadRequest(c, "INVALID_DOMAIN", err.Error())
				return
			}
		}
	}
	response.OK(c, gin.H{"message": "sites saved"})
}

// handleAdmin 保存管理员账号（步骤 6）。
func (s *Server) handleAdmin(c *gin.Context) {
	var req struct {
		Username *string `json:"username"`
		Password *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	d := s.snapshot()
	username := d.AdminUsername
	pw := d.AdminPassword
	if req.Username != nil {
		username = strings.TrimSpace(*req.Username)
	}
	if req.Password != nil {
		pw = *req.Password
	}
	if username == "" {
		response.BadRequest(c, "INVALID_REQUEST", "username is required")
		return
	}
	if _, errs := PasswordStrength(pw); len(errs) > 0 {
		response.BadRequest(c, "WEAK_PASSWORD", strings.Join(errs, "; "))
		return
	}
	s.update(func(dt *Draft) {
		dt.AdminUsername = username
		dt.AdminPassword = pw
	})
	response.OK(c, gin.H{"message": "admin saved"})
}

// ACME 在这一步就**直接把证书申请好**（域名/IP），挑战方式、密钥级别、DNS 接入、
// EAB 一并采集并随签发落库（以后自动续期按原选择走）。
func (s *Server) handleHTTPS(c *gin.Context) {
	var req struct {
		Mode     string   `json:"mode"`
		Domains  []string `json:"domains"`
		Email    string   `json:"email"`
		Provider string   `json:"provider"`
		CertPEM  string   `json:"cert_pem"`
		KeyPEM   string   `json:"key_pem"`
		// ACME 扩展参数。
		Challenge      string            `json:"challenge"`
		KeyType        string            `json:"key_type"`
		KeyBits        int               `json:"key_bits"`
		DNSProvider    string            `json:"dns_provider"`
		DNSEnv         map[string]string `json:"dns_env"`
		DNSChannelID   int64             `json:"dns_channel_id"`
		IPCert         bool              `json:"ip_cert"`
		EABKeyID       string            `json:"eab_key_id"`
		EABMACKey      string            `json:"eab_mac_key"`
		ProjectID      string            `json:"project_id"`
		ServiceAccount string            `json:"service_account"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_REQUEST", err.Error())
		return
	}
	switch req.Mode {
	case "", "none":
		s.update(func(d *Draft) {
			d.TLSMode = ""
			d.TLSDomains = nil
			d.TLSCertPath = ""
			d.TLSKeyPath = ""
		})
	case "self_signed":
		cp, kp, err := SelfSigned(c.Request.Context(), s.dataDir, s.certSvc, req.Domains)
		if err != nil {
			response.InternalError(c, "CERT_SETUP_FAILED", err.Error())
			return
		}
		s.update(func(d *Draft) {
			d.TLSMode = "self_signed"
			d.TLSDomains = req.Domains
			d.TLSCertPath = cp
			d.TLSKeyPath = kp
		})
	case "upload":
		cp, kp, err := SaveUploaded(c.Request.Context(), s.dataDir, s.certSvc, []byte(req.CertPEM), []byte(req.KeyPEM))
		if err != nil {
			response.BadRequest(c, "CERT_SETUP_FAILED", err.Error())
			return
		}
		s.update(func(d *Draft) {
			d.TLSMode = "upload"
			d.TLSCertPath = cp
			d.TLSKeyPath = kp
		})
	case "acme":
		domains, err := s.acmeDomains(req)
		if err != nil {
			response.BadRequest(c, "INVALID_REQUEST", err.Error())
			return
		}
		cp, kp, ierr := IssueACME(c.Request.Context(), s.dataDir, s.certSvc, ACMEDeps{EAB: s.eabSvc, DNS: s.dnsSvc}, IssueACMEOptions{
			Domains:        domains,
			Email:          req.Email,
			Provider:       req.Provider,
			Challenge:      req.Challenge,
			KeyType:        req.KeyType,
			KeyBits:        req.KeyBits,
			DNSChannelID:   req.DNSChannelID,
			DNSProvider:    req.DNSProvider,
			DNSEnv:         req.DNSEnv,
			IPCert:         req.IPCert,
			EABKeyID:       req.EABKeyID,
			EABMACKey:      req.EABMACKey,
			ProjectID:      req.ProjectID,
			ServiceAccount: req.ServiceAccount,
		})
		if ierr != nil {
			response.InternalError(c, "CERT_SETUP_FAILED", ierr.Error())
			return
		}
		// 记录本次实际用到的 EAB（ZeroSSL 由邮箱换取 / Google 自动申请或手填）与 DNS 凭据，
		// 完成后由 persistACMEConfig 写进目标数据库 → 以后自动申请/续期可用。
		eabKID, eabMAC := strings.TrimSpace(req.EABKeyID), strings.TrimSpace(req.EABMACKey)
		gProject, gSA := "", ""
		if s.eabSvc != nil {
			if cfg, cerr := s.eabSvc.GetByCA(c.Request.Context(), req.Provider); cerr == nil && cfg != nil {
				eabKID, eabMAC = cfg.KeyID, cfg.MACKey
				gProject, gSA = cfg.GoogleProjectID, cfg.GoogleSAJSON
			}
		}
		s.update(func(d *Draft) {
			d.TLSMode = "acme"
			d.TLSDomains = domains
			d.TLSEmail = req.Email
			d.ACMEProvider = req.Provider
			d.TLSChallenge = certsvc.NormalizeChallengeType(req.Challenge)
			d.TLSKeyType = corecert.NormalizeKeyType(req.KeyType)
			d.TLSKeyBits = req.KeyBits
			d.ACMEDNSProvider = strings.ToLower(strings.TrimSpace(req.DNSProvider))
			d.ACMEDNSEnv = req.DNSEnv
			d.ACMEEABKeyID = eabKID
			d.ACMEEABMACKey = eabMAC
			d.ACMEGoogleProjectID = gProject
			d.ACMEGoogleSAJSON = gSA
			d.TLSCertPath = cp
			d.TLSKeyPath = kp
		})
	default:
		response.BadRequest(c, "INVALID_REQUEST", "unsupported tls mode: "+req.Mode)
		return
	}
	d := s.snapshot()
	response.OK(c, gin.H{"mode": d.TLSMode, "tls_cert_path": d.TLSCertPath, "listen_addr": d.ResolvedListenAddr()})
}

// acmeDomains 组装 ACME 申请的 SAN 列表：
//   - IP 证书 → 本机公网 IPv4/IPv6 自动发现（有 v6 一并取；只有 v6 就取 v6），可再手动追加；
//   - 域名证书 → 默认自动带上面板/订阅/门户域名（去重），用户手动添加的域名可叠加。
func (s *Server) acmeDomains(req struct {
	Mode           string            `json:"mode"`
	Domains        []string          `json:"domains"`
	Email          string            `json:"email"`
	Provider       string            `json:"provider"`
	CertPEM        string            `json:"cert_pem"`
	KeyPEM         string            `json:"key_pem"`
	Challenge      string            `json:"challenge"`
	KeyType        string            `json:"key_type"`
	KeyBits        int               `json:"key_bits"`
	DNSProvider    string            `json:"dns_provider"`
	DNSEnv         map[string]string `json:"dns_env"`
	DNSChannelID   int64             `json:"dns_channel_id"`
	IPCert         bool              `json:"ip_cert"`
	EABKeyID       string            `json:"eab_key_id"`
	EABMACKey      string            `json:"eab_mac_key"`
	ProjectID      string            `json:"project_id"`
	ServiceAccount string            `json:"service_account"`
}) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(list []string) {
		for _, l := range list {
			l = strings.ToLower(strings.TrimSpace(l))
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			out = append(out, l)
		}
	}
	if req.IPCert {
		// IP 证书只有 Let's Encrypt（含 staging）支持。
		if p := normalizeCAProvider(req.Provider); p != corecert.CAProviderLetsEncrypt && p != corecert.CAProviderLetsEncryptStaging {
			return nil, fmt.Errorf("IP 证书目前仅 Let's Encrypt 支持，请选择 Let's Encrypt")
		}
		// 自动发现本机公网地址（与证书页「自动检测」同一套：有 IPv4 取 IPv4；
		// 有 IPv6 一并取；只有 IPv6 就取 IPv6）。
		v4, v6, derr := s.certSvc.DetectPublicIP()
		if derr != nil {
			return nil, fmt.Errorf("自动获取本机公网 IP 失败：%w", derr)
		}
		add([]string{v4, v6})
	} else {
		// 域名证书：默认自动带上面板/订阅/门户域名。
		add(s.siteDomains())
	}
	add(req.Domains) // 手动添加的域名总是允许
	if len(out) == 0 {
		return nil, fmt.Errorf("未选择域名：请填写面板/订阅/门户域名或手动追加域名，或把「证书目标」改为「使用 IP」签发 IP 证书")
	}
	return out, nil
}

// siteDomains 返回草稿里的面板/订阅/门户域名（去空白、去 scheme）。
func (s *Server) siteDomains() []string {
	d := s.snapshot()
	return []string{normalizeHost(d.PanelDomain), normalizeHost(d.SubscriptionDomain), normalizeHost(d.PortalDomain)}
}

// handleDNSProviders 返回支持的 DNS 接入服务商及所需凭据字段（向导 dns-01 表单用）。
func (s *Server) handleDNSProviders(c *gin.Context) {
	response.OK(c, gin.H{"providers": certsvc.SupportedDNSProviders()})
}

// handlePublicIP 返回本机公网 IPv4/IPv6（向导「IP 证书」的自动获取，与证书页同一套口径）：
// 有 IPv4 取 IPv4，有 IPv6 一并取；只有 IPv6 就取 IPv6。
func (s *Server) handlePublicIP(c *gin.Context) {
	ipv4, ipv6 := "", ""
	if s.certSvc != nil {
		if v4, v6, err := s.certSvc.DetectPublicIP(); err == nil {
			ipv4, ipv6 = v4, v6
		}
	}
	// 兜底：网卡上的公网地址（外网探测失败时仍能展示）。
	for _, ip := range LocalPublicIPs() {
		if strings.Contains(ip, ":") {
			if ipv6 == "" {
				ipv6 = ip
			}
		} else if ipv4 == "" {
			ipv4 = ip
		}
	}
	ips := make([]string, 0, 2)
	if ipv4 != "" {
		ips = append(ips, ipv4)
	}
	if ipv6 != "" {
		ips = append(ips, ipv6)
	}
	response.OK(c, gin.H{"ipv4": ipv4, "ipv6": ipv6, "ips": ips})
}

// handleSummary 返回配置清单（步骤 7）。
func (s *Server) handleSummary(c *gin.Context) {
	d := s.snapshot()
	response.OK(c, Summary(s.dataDir, &d))
}

// handleEnvFile 下载 .env 文件（配置清单"下载 env"）。
func (s *Server) handleEnvFile(c *gin.Context) {
	d := s.snapshot()
	content := EnvContent(s.dataDir, &d)
	c.Header("Content-Disposition", "attachment; filename=corefusion.env")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}

// handleComplete 完成初始化（校验 → 落库 → 写 .env → 标记 → 重启）。
func (s *Server) handleComplete(c *gin.Context) {
	s.mu.Lock()
	if s.completed {
		s.mu.Unlock()
		response.Conflict(c, "INIT_ALREADY_COMPLETED", "first-boot initialization has already been completed")
		return
	}
	d := *s.draft
	s.mu.Unlock()

	if err := d.Validate(); err != nil {
		response.BadRequest(c, "INVALID_CONFIG", err.Error())
		return
	}
	if err := Complete(c.Request.Context(), s.dataDir, &d); err != nil {
		response.InternalError(c, "COMPLETE_FAILED", err.Error())
		return
	}
	s.mu.Lock()
	s.draft = &d
	s.completed = true
	s.mu.Unlock()

	response.OK(c, gin.H{
		"message":          "first-boot initialization completed",
		"restart_required": true,
		"web_path":         d.WebPath,
		"listen_addr":      d.ResolvedListenAddr(),
		"listen_port":      d.ListenPort,
		"access":           Summary(s.dataDir, &d)["access"],
	})
	if s.onComplete != nil {
		go func() {
			time.Sleep(700 * time.Millisecond)
			s.onComplete()
		}()
	}
}

// serveSPA 提供嵌入的前端产物（与正式面板共用），SPA 路由包含 /init。
func (s *Server) serveSPA(c *gin.Context) {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "endpoint not found"})
		return
	}
	fsys := embed.AdminFSBytes()
	if strings.HasPrefix(path, "/assets/") || (len(path) > 1 && strings.Contains(path[1:], ".") && !strings.Contains(path, "..")) {
		data, err := fsys.ReadFile("dist" + path)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
		if ct == "" {
			ct = http.DetectContentType(data)
		}
		c.Data(http.StatusOK, ct, data)
		return
	}
	data, err := fsys.ReadFile("dist/index.html")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}
