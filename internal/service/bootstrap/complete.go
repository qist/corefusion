package bootstrap

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/auth/password"
	"corefusion/internal/migration"
	"corefusion/internal/model/entity"
	seccrypto "corefusion/internal/security/crypto"
	certsvc "corefusion/internal/service/certificate"
	systemsvc "corefusion/internal/service/system"
	wafsvc "corefusion/internal/service/waf"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

// PendingApply 描述"重启后要落地的安装任务"：代理类型、要安装的核心、
// 以及要添加到代理的面板/订阅/门户域名。由正式启动流程异步执行。
type PendingApply struct {
	ProxyType          string   `json:"proxy_type"`
	Cores              []string `json:"cores"`
	PanelDomain        string   `json:"panel_domain"`
	SubscriptionDomain string   `json:"subscription_domain"`
	PortalDomain       string   `json:"portal_domain"`
}

// EnvKV 返回初始化完成后应写入 .env 的键值对（顺序稳定，便于展示与下载）。
func EnvKV(dataDir string, d *Draft) [][2]string {
	if strings.TrimSpace(d.DataDir) == "" {
		d.DataDir = dataDir
	}
	kv := [][2]string{
		{"SAFE_PATHS", d.DataDir},
		// mysql/postgres 的 DSN 含口令 → 落 .env 时加密（与其它敏感配置同一套 AES-256-GCM）。
		{"CF_DB_DSN", db.EncodeDSNForEnv(d.DSN(dataDir))},
	}
	if d.DBType == db.DBTypeMySQL || d.DBType == db.DBTypePostgres {
		kv = append(kv, [2]string{"CF_DB_MAX_OPEN_CONNS", "100"}, [2]string{"CF_DB_MAX_IDLE_CONNS", "10"})
	}
	kv = append(kv,
		[2]string{"JWT_SECRET", d.JWTSecret},
		[2]string{"CF_SECRET_ENCRYPTION_KEY", d.SecretEncryptionKey},
	)
	return kv
}

// EnvContent 生成 .env 文本（供展示与下载，不写盘）。
func EnvContent(dataDir string, d *Draft) string {
	var b strings.Builder
	nl := string(rune(10))
	b.WriteString("# CoreFusion 首次启动初始化生成（权限 0600）" + nl)
	b.WriteString("# 生成后仍可在「系统设置 → 环境配置」中修改" + nl + nl)
	b.WriteString("# === 数据目录 ===" + nl)
	for _, kv := range EnvKV(dataDir, d) {
		b.WriteString(kv[0] + "=" + kv[1] + nl)
		if kv[0] == "CF_DB_DSN" {
			b.WriteString(nl + "# === 加密密钥 ===" + nl)
		}
	}
	return b.String()
}

// SaveEnv 把初始化配置合并写入当前目录的 .env（0600）。
func SaveEnv(dataDir string, d *Draft) error {
	content := ""
	if b, err := os.ReadFile(".env"); err == nil {
		content = string(b)
	}
	for _, kv := range EnvKV(dataDir, d) {
		content = upsertEnvLine(content, kv[0], kv[1])
	}
	if err := os.WriteFile(".env", []byte(content), 0600); err != nil {
		return fmt.Errorf("write .env: %w", err)
	}
	_ = os.Chmod(".env", 0600)
	return nil
}

// upsertEnvLine 在 .env 内容里新增/更新一个键（保持其它行原样）。
func upsertEnvLine(content, key, value string) string {
	nl := string(rune(10))
	content = strings.ReplaceAll(content, string(rune(13))+nl, nl)
	lines := []string{}
	found := false
	for _, line := range strings.Split(content, nl) {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
				lines = append(lines, key+"="+value)
				found = true
				continue
			}
		}
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	return strings.TrimRight(strings.Join(lines, nl), nl) + nl
}

// Summary 返回配置清单（供向导"配置清单"步骤展示与下载）。
func Summary(dataDir string, d *Draft) map[string]any {
	listenAddr := d.ResolvedListenAddr()
	kv := EnvKV(dataDir, d)
	env := make([][2]string, 0, len(kv))
	for _, e := range kv {
		// 敏感项脱敏：密钥、以及 CF_DB_DSN（.env 里存的是 enc: 密文）。
		if e[0] == "JWT_SECRET" || e[0] == "CF_SECRET_ENCRYPTION_KEY" || e[0] == "CF_DB_DSN" {
			env = append(env, [2]string{e[0], maskSecret(e[1])})
			continue
		}
		env = append(env, e)
	}
	panelVia := d.viaProxy(d.ProxyAddPanel)
	subVia := d.viaProxy(d.ProxyAddSubscription)
	portalVia := d.viaProxy(d.ProxyAddPortal)
	return map[string]any{
		"db_type":                d.DBType,
		"db_dsn":                 maskDSN(d.DSN(dataDir)),
		"data_dir":               d.DataDir,
		"cores":                  d.Cores,
		"proxy_type":             d.ProxyType,
		"default_currency":       d.DefaultCurrency,
		"timezone":               d.Timezone,
		"proxy_add_panel":        d.ProxyAddPanel,
		"proxy_add_subscription": d.ProxyAddSubscription,
		"proxy_add_portal":       d.ProxyAddPortal,
		"listen_addr":            listenAddr,
		"listen_port":            d.ListenPort,
		"subscription_port":      d.SubscriptionPort,
		"portal_port":            d.PortalPort,
		"web_path":               d.WebPath,
		"panel_domain":           d.PanelDomain,
		"subscription_domain":    d.SubscriptionDomain,
		"portal_domain":          d.PortalDomain,
		"tls_mode":               d.TLSMode,
		"admin_username":         d.AdminUsername,
		// 管理员口令只在配置清单里显示这一次（用户要求可见，便于保存后登录）。
		"admin_password": d.AdminPassword,
		// .env 展示用键值对（已脱敏）；前端「配置清单」的 .env 表格读取此字段。
		"env": env,
		"access": map[string]string{
			"panel":        accessURL(d.PanelDomain, listenAddr, d.ListenPort, d.servesTLS(panelVia), panelVia),
			"subscription": accessURL(d.SubscriptionDomain, listenAddr, firstNonEmpty(d.SubscriptionPort, d.ListenPort), d.servesTLS(subVia), subVia),
			"portal":       accessURL(d.PortalDomain, listenAddr, firstNonEmpty(d.PortalPort, d.ListenPort), d.servesTLS(portalVia), portalVia),
		},
	}
}

// tlsIssued 是否已签发 / 配置了证书（证书与私钥路径齐全）。
func (d *Draft) tlsIssued() bool {
	return strings.TrimSpace(d.TLSCertPath) != "" && strings.TrimSpace(d.TLSKeyPath) != ""
}

// hasDomainCert 已配置的证书是否涵盖至少一个域名（DNS SAN）。
// 只有涵盖域名的证书才能安全挂到「域名」反代站点上（仅 IP 的证书挂上去会导致 TLS 报错）。
func (d *Draft) hasDomainCert() bool {
	if !d.tlsIssued() {
		return false
	}
	return certCoversDomain(d.TLSCertPath)
}

// servesTLS 判断某入口对外是否走 HTTPS：
//   - 走反向代理（viaProxy）→ 取决于代理是否拿到涵盖域名的证书；
//   - 面板/订阅/门户直连 → 只要面板加载了证书即为 HTTPS（含 IP 证书）。
func (d *Draft) servesTLS(viaProxy bool) bool {
	if viaProxy {
		return d.hasDomainCert()
	}
	return d.tlsIssued()
}

// viaProxy 判断该项是否"经由反向代理对外服务"：
// 必须是选了 nginx/caddy 且勾了对应的一键加入（面板自身只监听 127.0.0.1）。
func (d *Draft) viaProxy(add bool) bool {
	return (d.ProxyType == ProxyNginx || d.ProxyType == ProxyCaddy) && add
}

// accessURL 生成访问地址。勾选了「一键添加到反向代理」且配置了域名时，
// 流量走代理的 80/443（上游面板监听 127.0.0.1）→ 不带端口。
func accessURL(domain, addr, port string, tls, viaProxy bool) string {
	scheme := "http"
	if tls {
		scheme = "https"
	}
	host := normalizeHost(domain)
	if host == "" {
		host = addr
	}
	// 用域名 + 勾了反代 → 由代理对外服务（80/443），不加端口；
	// 否则面板本机直连才带端口。
	if (viaProxy && normalizeHost(domain) != "") || strings.TrimSpace(port) == "" || port == "0" {
		return scheme + "://" + host
	}
	return scheme + "://" + host + ":" + port
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" && a != "0" {
		return a
	}
	return b
}

// independentPort 返回"真正独立"的端口：非空、非 0，且与面板监听端口不同。
// 与 cmd/corefusion 启动逻辑一致（subPort/portalPort != "" && != "0" && != apiPort）——
// 只有此条件成立时才会另起独立 HTTP 服务，该服务在**根路径**对外（订阅 /:token、门户 /），
// 所以反代必须整域名直通，不能再加 /sub、/p 前缀（否则 404）。
func (d *Draft) independentPort(port string) string {
	p := strings.TrimSpace(port)
	if p == "" || p == "0" || p == strings.TrimSpace(d.ListenPort) {
		return ""
	}
	return p
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "********"
	}
	return s[:4] + "********" + s[len(s)-4:]
}

// maskDSN 隐藏 DSN 中的口令。
func maskDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	proto := strings.Index(dsn, "://")
	if at < 0 || proto < 0 || at < proto {
		return dsn
	}
	head := dsn[:proto+3]
	user := dsn[proto+3 : at]
	if i := strings.Index(user, ":"); i >= 0 {
		user = user[:i] + ":********"
	}
	return head + user + dsn[at:]
}

// EnsureDatabase 确保目标数据库可用：连不上时尝试自动建库（mysql/pg）。
//
// 口径：库不存在 → 用**同一账号**连"维护库"（mysql 不带库名 / PG 连 postgres 库）
// 执行 CREATE DATABASE；账号没有 CREATE 权限时返回明确提示（让用户先手动建库）。
func EnsureDatabase(target db.Config, dbname string) error {
	err := db.TestConnection(target)
	if err == nil {
		return nil
	}
	if target.Type != db.DBTypeMySQL && target.Type != db.DBTypePostgres {
		return fmt.Errorf("数据库不可用: %w", err)
	}
	if cerr := db.CreateDatabase(target, dbname); cerr != nil {
		return fmt.Errorf("数据库 %q 连不上且自动创建失败：%v；原始连接错误：%v", dbname, cerr, err)
	}
	if rerr := db.TestConnection(target); rerr != nil {
		return fmt.Errorf("数据库 %q 已创建但仍连不上: %w", dbname, rerr)
	}
	log.Printf("[bootstrap] auto-created database %s on %s", dbname, target.Type)
	return nil
}

// Complete 执行初始化完成流程：写 .env → 初始化所选数据库 → 迁移 →
// 创建管理员 → 落库面板配置/域名/代理站点 → 记录待安装任务 → 标记完成。
// 供向导服务与 CLI init 命令共用。
func Complete(ctx context.Context, dataDir string, d *Draft) error {
	if strings.TrimSpace(d.DataDir) == "" {
		d.DataDir = dataDir
	}
	if d.AutoGenerateSecrets || strings.TrimSpace(d.JWTSecret) == "" {
		d.JWTSecret = RandomHex(32)
	}
	if d.AutoGenerateSecrets || strings.TrimSpace(d.SecretEncryptionKey) == "" {
		d.SecretEncryptionKey = RandomHex(32)
	}
	if strings.TrimSpace(d.DBType) == "" {
		d.DBType = db.DBTypeSQLite
	}

	// 让后续 EncryptSecret 使用最终密钥（crypto 会缓存"首次使用"的密钥，
	// 向导此前可能已用回环兜底密钥加密过会话文件）。
	_ = seccrypto.SetKey(d.SecretEncryptionKey)

	// 1. 先确认目标数据库连得上（连不上就不写 .env、不动现有数据）。
	target := d.DBConfig(dataDir)
	if err := EnsureDatabase(target, d.DBName); err != nil {
		return err
	}

	// 2. 写 .env（SAFE_PATHS / CF_DB_DSN / 密钥 / 连接池）。
	if err := SaveEnv(dataDir, d); err != nil {
		return err
	}

	// 3. 切到目标数据库并迁移（首次初始化 = 新库，直接建表 + 种子数据）。
	_ = db.Close()
	if err := db.Init(target); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	db.RecordStartup(target)
	if err := migration.AutoMigrate(db.DB); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	settingSvc := systemsvc.NewSettingService(
		repository.NewSystemSettingRepository(db.DB),
		repository.NewNotificationTemplateRepository(db.DB),
	)
	userRepo := repository.NewUserRepository(db.DB)

	// 4. 创建管理员。
	if u, _ := userRepo.GetByUsername(ctx, d.AdminUsername); u == nil {
		hash, herr := password.Hash(d.AdminPassword)
		if herr != nil {
			return fmt.Errorf("hash password: %w", herr)
		}
		if cerr := userRepo.Create(ctx, &entity.User{Username: d.AdminUsername, PasswordHash: hash, Role: "super_admin", Status: 1}); cerr != nil {
			return fmt.Errorf("create admin: %w", cerr)
		}
	}

	// 5. 面板配置（监听地址由"是否配置证书"决定，不取用户输入）。
	listenAddr := d.ResolvedListenAddr()
	kv := map[string]string{
		"panel.listen_addr":       listenAddr,
		"panel.listen_port":       d.ListenPort,
		"panel.subscription_port": d.SubscriptionPort,
		"panel.portal_port":       d.PortalPort,
		"panel.web_path":          d.WebPath,
		"panel.data_dir":          d.DataDir,
	}
	for k, v := range kv {
		if err := settingSvc.SetByKey(ctx, k, v); err != nil {
			return fmt.Errorf("save %s: %w", k, err)
		}
	}
	// 面板是否加载证书：
	//   - 面板被「一键添加到反向代理」(add_panel) → 代理终止 TLS，面板不加载证书（上游走 HTTP）；
	//     否则把（尤其仅 IP 的）证书写进面板，会让代理的 HTTP 上游打到 HTTPS 面板 → 转发打不开。
	//   - 否则只要签发了证书（域名或 IP）→ 面板自身加载证书对外提供 HTTPS。
	if !d.viaProxy(d.ProxyAddPanel) && d.tlsIssued() {
		_ = settingSvc.SetByKey(ctx, "panel.tls_enabled", "true")
		_ = settingSvc.SetByKey(ctx, "panel.tls_cert_path", d.TLSCertPath)
		_ = settingSvc.SetByKey(ctx, "panel.tls_key_path", d.TLSKeyPath)
	} else {
		_ = settingSvc.SetByKey(ctx, "panel.tls_enabled", "false")
	}

	// 默认货币（充值卡/余额等默认货币）。
	if c := strings.ToUpper(strings.TrimSpace(d.DefaultCurrency)); c != "" {
		_ = settingSvc.SetByKey(ctx, "currency.default", c)
	}
	// 时区（面板设置 → 注入代理进程 TZ；留空保持默认 Asia/Shanghai）。
	if tz := strings.TrimSpace(d.Timezone); tz != "" {
		_ = settingSvc.SetByKey(ctx, "panel.timezone", tz)
	}

	// 6. 域名：面板对外地址 + 门户回调域名 + 订阅端点/分享域名。
	if h := normalizeHost(d.PanelDomain); h != "" {
		scheme := "http"
		if d.TLSCertPath != "" {
			scheme = "https"
		}
		_ = settingSvc.SetByKey(ctx, "panel.external_url", scheme+"://"+h)
	}
	if h := normalizeHost(d.PortalDomain); h != "" {
		_ = settingSvc.SetByKey(ctx, "oauth.callback_domain", h)
	}
	if h := normalizeHost(d.SubscriptionDomain); h != "" {
		epRepo := repository.NewSubscriptionEndpointRepository(db.DB)
		// 订阅端点路径必须与实际服务路由一致：
		//   - 配置了独立订阅端口 → 订阅引擎在根路径提供 /:token，路径写 "/"；
		//   - 否则由面板在主端口用 /sub/<token> 提供，路径写 "/sub"。
		// 不一致会生成 404 的订阅 URL（预览/客户端拉取空白）。
		subPath := "/sub"
		if d.independentPort(d.SubscriptionPort) != "" {
			subPath = "/"
		}
		if err := epRepo.Create(ctx, &entity.SubscriptionEndpoint{Domain: h, Path: subPath, Enabled: true, Description: "首次启动初始化创建"}); err != nil {
			log.Printf("[bootstrap] warning: create subscription endpoint failed: %v", err)
		}
		sdRepo := repository.NewShareDomainConfigRepository(db.DB)
		if err := sdRepo.Create(ctx, &entity.ShareDomainConfig{Type: "direct", Value: h, Port: 443, Ports: "[443]", Enabled: true}); err != nil {
			log.Printf("[bootstrap] warning: create share domain failed: %v", err)
		}
	}

	// 7. 代理站点（面板 / 订阅 / 门户）。
	if d.ProxyType == ProxyNginx || d.ProxyType == ProxyCaddy {
		createProxySites(ctx, dataDir, d)
	}

	// 7.5 ACME 自动申请配置 + 已签发证书登记进目标库（第 7 步在初始化用的库里签发，
	// 切库后目标库必须有记录，自动续期才有依据）。
	persistACMEConfig(ctx, dataDir, d)

	// 8. 记录待安装任务（核心 / 代理二进制），由重启后的正式启动流程异步落地。
	pending := PendingApply{
		ProxyType:          d.ProxyType,
		Cores:              d.Cores,
		PanelDomain:        normalizeHost(d.PanelDomain),
		SubscriptionDomain: normalizeHost(d.SubscriptionDomain),
		PortalDomain:       normalizeHost(d.PortalDomain),
	}
	if (d.ProxyType != ProxyNone && d.ProxyType != "") || len(d.Cores) > 0 {
		if b, err := json.Marshal(pending); err == nil {
			_ = settingSvc.SetByKey(ctx, PendingApplyKey, string(b))
		}
	}

	// 9. 标记完成 + 删除初始化会话文件。
	if err := settingSvc.SetByKey(ctx, BootstrapCompletedKey, "true"); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	RemoveSession(dataDir)
	log.Printf("[bootstrap] initialization completed: admin=%s db=%s listen=%s:%s web_path=%s tls=%v proxy=%s cores=%v",
		d.AdminUsername, d.DBType, listenAddr, d.ListenPort, d.WebPath, d.TLSCertPath != "", d.ProxyType, d.Cores)
	return nil
}

// persistACMEConfig 把向导采集的 ACME 自动申请配置写进目标数据库：
//   - ACME 账号邮箱（默认账号，续期/再签都用它注册）；
//   - EAB（ZeroSSL 由邮箱换取 / Google 自动申请或手填）；
//   - dns-01 的 DNS 接入通道（凭据加密入库）；
//   - 已签发证书的记录（provider_type=acme、auto_renew=1，含挑战方式与 DNS 通道）。
//
// 为什么必须做：向导第 7 步是在"初始化用的库"里签发的，Complete 随后切到目标库，
// 不搬过来的话目标库里"没有这张证书"，自动续期无从谈起。
func persistACMEConfig(ctx context.Context, dataDir string, d *Draft) {
	if d.TLSMode != "acme" {
		return
	}
	caProvider := strings.TrimSpace(d.ACMEProvider)

	// 1. dns-01 通道（向导直填凭据时）→ 建一条加密通道，拿到 ID 供证书记录引用。
	var dnsChannelID int64
	if certsvc.NormalizeChallengeType(d.TLSChallenge) == "dns-01" && strings.TrimSpace(d.ACMEDNSProvider) != "" {
		dnsSvc := certsvc.NewDNSChannelService(repository.NewDNSChannelRepository(db.DB))
		if ch, err := dnsSvc.Create(ctx, "初始化 - "+d.ACMEDNSProvider, strings.ToLower(strings.TrimSpace(d.ACMEDNSProvider)), d.ACMEDNSEnv); err != nil {
			log.Printf("[bootstrap] warning: create dns channel failed: %v", err)
		} else {
			dnsChannelID = ch.ID
		}
	}

	// 2. ACME 账号邮箱（默认账号）。
	if email := strings.TrimSpace(d.TLSEmail); email != "" {
		acmeSvc := certsvc.NewACMEEmailService(repository.NewACMEAccountConfigRepository(db.DB))
		if _, err := acmeSvc.Create(ctx, certsvc.ACMEAccountInput{Email: email, IsDefault: true, CAProvider: caProvider}); err != nil {
			log.Printf("[bootstrap] warning: save acme email failed: %v", err)
		}
	}

	// 3. EAB（Google / ZeroSSL）。
	if strings.TrimSpace(d.ACMEEABKeyID) != "" || strings.TrimSpace(d.ACMEGoogleSAJSON) != "" {
		eabSvc := certsvc.NewEABConfigService(repository.NewEABConfigRepository(db.DB))
		name := "初始化 - " + caProvider
		if strings.EqualFold(caProvider, "google") && strings.TrimSpace(d.ACMEGoogleProjectID) != "" {
			name = "Google Public CA - " + strings.TrimSpace(d.ACMEGoogleProjectID)
		}
		if _, err := eabSvc.Create(ctx, certsvc.EABConfigInput{
			Name: name, CAProvider: caProvider,
			KeyID: d.ACMEEABKeyID, MACKey: d.ACMEEABMACKey,
			GoogleProjectID: d.ACMEGoogleProjectID, GoogleSAJSON: d.ACMEGoogleSAJSON,
		}); err != nil {
			log.Printf("[bootstrap] warning: save EAB config failed: %v", err)
		}
	}

	// 4. 证书记录：把已签好的证书（内容在磁盘上）登记进证书库 → 可自动续期。
	registerIssuedCert(ctx, dataDir, d, dnsChannelID)
}

// registerIssuedCert 读回已签发的证书文件并登记进证书库
// （与节点签发同一套解析：解析 SAN/密钥类型/签发者，provider_type=acme、auto_renew=1）。
func registerIssuedCert(ctx context.Context, dataDir string, d *Draft, dnsChannelID int64) {
	certPEM, err := os.ReadFile(d.TLSCertPath)
	if err != nil {
		log.Printf("[bootstrap] warning: read issued cert %s: %v", d.TLSCertPath, err)
		return
	}
	keyPEM, err := os.ReadFile(d.TLSKeyPath)
	if err != nil {
		log.Printf("[bootstrap] warning: read issued key %s: %v", d.TLSKeyPath, err)
		return
	}
	svc := certsvc.NewService(repository.NewCertificateRepository(db.DB), dataDir, d.TLSEmail, normalizeCAProvider(d.ACMEProvider))
	if _, err := svc.StoreAgentIssuedCert(ctx, certsvc.AgentIssuedCertInput{
		Identifiers:   d.TLSDomains,
		CertPEM:       string(certPEM),
		KeyPEM:        string(keyPEM),
		CA:            d.ACMEProvider,
		ChallengeType: d.TLSChallenge,
		DNSChannelID:  dnsChannelID,
	}); err != nil {
		log.Printf("[bootstrap] warning: register issued cert failed: %v", err)
	}
}

// createProxySites 为面板/订阅/门户创建反代站点（仅落库；代理配置在重启后生效）。
//
// ⚠ 写入的是**真实生成器读取的表**（nginx_sites / caddy_sites，按所选代理类型），
// 不是通用 proxy_sites —— 配置由 NginxManager / CaddyManager 从这两张表生成，
// 只写 proxy_sites 不会落到 nginx/caddy 配置里（这正是此前"模板不完整"的原因）。
//
// 订阅/门户使用内置 forward 模板（subscription_forward / portal_forward）：upstream 与
// path 随系统设置派生 ——
//   - 配置了**独立端口**：独立的订阅/门户引擎在**根路径**对外（订阅 /:token、门户 /），
//     反代整域名直通（path 空），不能再加 /sub、/p，否则 404；
//   - 未配置独立端口：回退**面板端口**，订阅走 /sub、门户走 /p。
//
// 只有**涵盖域名的证书**才绑定到这些站点（cert_path/key_path）——反代站点是域名站点，
// 挂仅 IP 的证书会让 TLS 校验失败；未签发域名证书时不挂，代理走 HTTP。
func createProxySites(ctx context.Context, dataDir string, d *Draft) {
	certPath, keyPath := "", ""
	if d.hasDomainCert() {
		certPath = relToDataDir(dataDir, d.TLSCertPath)
		keyPath = relToDataDir(dataDir, d.TLSKeyPath)
	}
	panelUpstream := "127.0.0.1:" + strings.TrimSpace(d.ListenPort)

	// 订阅：独立端口 → 独立订阅引擎在根路径服务（/:token）→ 整域名直通（path 空）；
	// 否则回退面板端口、由面板的 /sub 提供 → path "/sub"。
	subUpstream := panelUpstream
	subPath := "/sub"
	if p := d.independentPort(d.SubscriptionPort); p != "" {
		subUpstream = "127.0.0.1:" + p
		subPath = ""
	}

	// 门户：独立端口 → 独立门户引擎在根路径服务（/）→ 整域名直通（path 空）；
	// 否则与面板共用端口 → 走 /p。
	portalUpstream := panelUpstream
	portalPath := "/p"
	if p := d.independentPort(d.PortalPort); p != "" {
		portalUpstream = "127.0.0.1:" + p
		portalPath = ""
	}

	type siteSpec struct {
		domain   string
		upstream string
		template string
		path     string
		isPanel  bool
	}
	var specs []siteSpec
	if d.ProxyAddPanel {
		if h := normalizeHost(d.PanelDomain); h != "" {
			// 面板转发使用专用模板 panel_forward（upstream/path 由面板主端口派生；
			// 生成器为其附加 SSE/流式反代参数，见 nginx_config.go / caddy_manager.go）。
			specs = append(specs, siteSpec{h, panelUpstream, "panel_forward", "", true})
		}
	}
	if d.ProxyAddSubscription {
		if h := normalizeHost(d.SubscriptionDomain); h != "" {
			specs = append(specs, siteSpec{h, subUpstream, "subscription_forward", subPath, false})
		}
	}
	if d.ProxyAddPortal {
		if h := normalizeHost(d.PortalDomain); h != "" {
			specs = append(specs, siteSpec{h, portalUpstream, "portal_forward", portalPath, false})
		}
	}
	if len(specs) == 0 {
		return
	}

	switch d.ProxyType {
	case ProxyNginx:
		repo := repository.NewNginxSiteRepository(db.DB)
		for _, s := range specs {
			site := &entity.NginxSite{
				ServerID: 1, Domain: s.domain, Upstream: s.upstream, Template: s.template,
				Path: s.path, CertPath: certPath, KeyPath: keyPath,
				Enabled: true, IsPanel: s.isPanel, Priority: 100,
			}
			if err := repo.Create(ctx, site); err != nil {
				log.Printf("[bootstrap] warning: create nginx site %s%s failed: %v", s.domain, s.path, err)
			}
			provisionForwardWAF(ctx, dataDir, s.template, s.domain)
		}
	case ProxyCaddy:
		repo := repository.NewCaddySiteRepository(db.DB)
		for _, s := range specs {
			site := &entity.CaddySite{
				ServerID: 1, Domain: s.domain, Upstream: s.upstream, Template: s.template,
				Path: s.path, CertPath: certPath, KeyPath: keyPath,
				Enabled: true, IsPanel: s.isPanel, Priority: 100,
			}
			if err := repo.Create(ctx, site); err != nil {
				log.Printf("[bootstrap] warning: create caddy site %s%s failed: %v", s.domain, s.path, err)
			}
			provisionForwardWAF(ctx, dataDir, s.template, s.domain)
		}
	}
}

// provisionForwardWAF 为「转发」类站点（面板/门户/订阅）生成站点级 WAF 配置：
//   - 面板 panel_forward：域名级 URL 白名单（保持 WAF 开启，仅放行面板自身管理 API
//     对应的检查），其余规则回落到全局规则（见 waf.Service.EnsurePanelForwardWhitelist）。
//   - 门户/订阅 portal_forward/subscription_forward：独立的域名规则目录（继承全局规则，
//     可站点级覆盖），不追加白名单（见 waf.Service.EnsureForwardDomainConfig）。
//
// 不再用「整体关闭该站点 WAF」的方式，避免面板/门户/订阅域名失去防护。
func provisionForwardWAF(ctx context.Context, dataDir, template, domain string) {
	switch template {
	case "panel_forward", "portal_forward", "subscription_forward":
	default:
		return
	}
	if strings.TrimSpace(domain) == "" {
		return
	}
	svc := wafsvc.NewService(
		repository.NewWafRuleRepository(db.DB),
		repository.NewWafConfigRepository(db.DB),
		repository.NewWafDomainConfigRepository(db.DB),
		dataDir, nil,
	)
	var err error
	if template == "panel_forward" {
		err = svc.EnsurePanelForwardWhitelist(ctx, 1, domain)
	} else {
		err = svc.EnsureForwardDomainConfig(ctx, 1, domain)
	}
	if err != nil {
		log.Printf("[bootstrap] warning: provision %s WAF config for %s failed: %v", template, domain, err)
	}
}

// relToDataDir 把 dataDir 下的绝对路径转成相对路径（与证书库 cert_path 同一格式，
// 便于代理配置与证书替换逻辑匹配）；不在 dataDir 下则原样返回。
func relToDataDir(dataDir, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) {
		return path
	}
	if dd := strings.TrimSpace(dataDir); dd != "" {
		if rel, err := filepath.Rel(dd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}

// certCoversDomain 读回已配置的证书，判断其是否包含至少一个域名（DNS SAN）。
// 仅 IP 的证书不能挂到「域名」反代站点（TLS 校验会失败）。
func certCoversDomain(certPath string) bool {
	b, err := os.ReadFile(strings.TrimSpace(certPath))
	if err != nil {
		return false
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	return len(cert.DNSNames) > 0
}

// normalizeHost 去掉 scheme 与多余空白。
func normalizeHost(h string) string {
	h = strings.TrimSpace(h)
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	h = strings.TrimSuffix(h, "/")
	return strings.TrimSpace(h)
}

// LoadPendingApply 读取待安装任务（不存在返回 nil）。
func LoadPendingApply(raw string) *PendingApply {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var p PendingApply
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil
	}
	return &p
}
