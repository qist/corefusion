package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"corefusion/internal/migration"
	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/agent"
	bootstrap "corefusion/internal/service/bootstrap"
	"corefusion/internal/service/certificate"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
	"corefusion/pkg/netutil"
)

// runInitMode 以“初始化向导服务”方式启动：独立随机端口 + init_token（设计 3.2 / 3.5）。
// 不启动正式面板；初始化完成后由 onComplete 重启进程，届时 .env 已配置完整 → 正常启动。
func runInitMode(env map[string]string, dataDir string) {
	for _, d := range []string{
		filepath.Join(dataDir, "sqlite"),
		filepath.Join(dataDir, "certs"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "runtime"),
	} {
		_ = os.MkdirAll(d, 0750)
	}

	dbCfg := db.ConfigFromEnv(env, filepath.Join(dataDir, "sqlite", "app.db"))
	if err := db.Init(dbCfg); err != nil {
		log.Fatalf("init mode: failed to init database: %v", err)
	}
	defer db.Close()
	db.RecordStartup(dbCfg)
	if err := migration.AutoMigrate(db.DB); err != nil {
		log.Fatalf("init mode: failed to run migrations: %v", err)
	}

	sess, ok := bootstrap.LoadSession(dataDir)
	if !ok || sess.Expired() {
		s, err := bootstrap.NewSession()
		if err != nil {
			log.Fatalf("init mode: %v", err)
		}
		sess = s
		if err := sess.Save(dataDir); err != nil {
			log.Fatalf("init mode: save session: %v", err)
		}
	}

	// HTTPS 证书复用面板既有证书库（internal/service/certificate），不另建目录。
	// dnsSvc：dns-01 接入通道；eabSvc：ZeroSSL/Google EAB 自动申请。
	certSvc := certificate.NewService(repository.NewCertificateRepository(db.DB), dataDir, "", "")
	dnsSvc := certificate.NewDNSChannelService(repository.NewDNSChannelRepository(db.DB))
	eabSvc := certificate.NewEABConfigService(repository.NewEABConfigRepository(db.DB))

	srv := bootstrap.NewServer(dataDir, sess, certSvc, dnsSvc, eabSvc, restartSelf)
	addr := fmt.Sprintf(":%d", sess.Port)
	ln, err := netutil.ListenTCP(addr)
	if err != nil {
		log.Fatalf("init mode: listen on %s failed: %v", addr, err)
	}
	printInitBanner(sess)
	log.Printf("CoreFusion %s init wizard listening on %s (HTTP)", Version, addr)
	if err := http.Serve(ln, srv.Engine()); err != nil {
		log.Fatalf("init server error: %v", err)
	}
}

// printInitBanner 输出初始化向导访问地址与令牌（中国控制台横幅，设计 3.2.3）。
func printInitBanner(sess *bootstrap.Session) {
	host := "127.0.0.1"
	for _, ip := range localIPv4s() {
		host = ip
		break
	}
	url := fmt.Sprintf("http://%s:%d%s?init_token=%s", host, sess.Port, bootstrap.BootWebPath, sess.Token)
	log.Printf("==================================================================")
	log.Printf("CoreFusion 首次启动 - 初始化向导")
	log.Printf("  访问地址: %s", url)
	log.Printf("  初始化令牌: %s", sess.Token)
	log.Printf("  有效期: %s", sess.Remaining().Round(time.Second))
	log.Printf("  提示: 请勿把初始化令牌分享给他人；完成后本端口关闭。")
	log.Printf("==================================================================")
}

// localIPv4s 返回本机非回环 IPv4 地址。
func localIPv4s() []string {
	var out []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.IsLoopback() {
			continue
		}
		if ip4 := ipn.IP.To4(); ip4 != nil {
			out = append(out, ip4.String())
		}
	}
	return out
}

// restartSelf 重新执行当前二进制（初始化完成后切换到正式服务）。
//
// 用 syscall.Exec 原地替换进程镜像（PID 不变）：面板多由 systemd（Type=simple）
// 托管，spawn 子进程再 exit(0) 会被 systemd 判定为服务正常退出，整个 cgroup
// （含刚起的子进程）被回收，表现为"初始化完成后面板起不来"。与自更新/重启接口
// 的 restart_unix.go 保持同一做法。
func restartSelf() {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(0)
	}
	argv := append([]string{exe}, os.Args[1:]...)
	if err := syscall.Exec(exe, argv, os.Environ()); err != nil {
		// exec 失败的兜底：退回 spawn，至少尝试拉起新进程。
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Start(); err != nil {
			log.Printf("restart failed: %v", err)
		}
	}
	os.Exit(0)
}

// runInit 是 CLI 非交互初始化（“cli 方式去初始化”）：直接落库配置并标记完成，
// 无需启动 Web 向导。TLS 支持 none / self_signed / acme。
func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	listenAddr := fs.String("listen-addr", "127.0.0.1", "panel listen address (when TLS is not configured, only 127.0.0.1 is used)")
	listenPort := fs.String("listen-port", "", "panel listen port (default: random free port)")
	subPort := fs.String("subscription-port", "", "subscription dedicated port (default: random; 0 = same as panel)")
	portalPort := fs.String("portal-port", "", "portal dedicated port (default: random; 0 = same as panel)")
	webPath := fs.String("web-path", "/", "panel web path")
	dataDir := fs.String("data-dir", "", "data directory (default: SAFE_PATHS or ./data)")
	adminUser := fs.String("admin-username", "", "admin username (default: random)")
	adminPass := fs.String("admin-password", "", "admin password (default: random, printed once)")
	tlsMode := fs.String("tls-mode", "none", "tls mode: none | self_signed | acme")
	tlsDomain := fs.String("tls-domain", "", "TLS domain (comma separated), required for self_signed/acme")
	tlsEmail := fs.String("tls-email", "", "ACME account email (required for zerossl/google)")
	tlsChallenge := fs.String("tls-challenge", "http-01", "ACME challenge type: http-01 | dns-01")
	tlsKeyType := fs.String("tls-key-type", "ecc", "certificate key type: ecc | rsa")
	tlsKeyBits := fs.Int64("tls-key-bits", 0, "key bits (ecc: 256/384; rsa: 2048/3072/4096; 0 = provider default)")
	tlsDNSChannel := fs.Int64("tls-dns-channel", 0, "existing DNS channel id for dns-01")
	tlsDNSProvider := fs.String("tls-dns-provider", "", "dns-01 provider name (cloudflare/alidns/dnspod/...), creates a DNS channel")
	tlsIPCert := fs.Bool("tls-ip-cert", false, "issue IP certificate from local public IPv4/IPv6 (Let's Encrypt only)")
	acmeProvider := fs.String("acme-provider", "lets_encrypt", "ACME CA: lets_encrypt | lets_encrypt_staging | zerossl | google")
	dbType := fs.String("db-type", "sqlite", "database type: sqlite | mysql | postgres")
	dbHost := fs.String("db-host", "", "database host (mysql/postgres)")
	dbPort := fs.String("db-port", "", "database port (mysql/postgres)")
	dbUser := fs.String("db-user", "", "database user (mysql/postgres)")
	dbPass := fs.String("db-password", "", "database password (mysql/postgres)")
	dbName := fs.String("db-name", "corefusion", "database name (mysql/postgres; created automatically if missing)")
	dbSSLMode := fs.String("db-sslmode", "disable", "postgres sslmode")
	proxyType := fs.String("proxy-type", "none", "proxy: none | nginx | caddy")
	coresFlag := fs.String("cores", "", "cores to install (comma separated): xray,singbox (Hysteria2/TUIC are carried by sing-box)")
	panelDomain := fs.String("panel-domain", "", "panel domain (added to proxy)")
	subDomain := fs.String("subscription-domain", "", "subscription domain (endpoint + share domain + proxy)")
	portalDomain := fs.String("portal-domain", "", "portal callback domain (oauth.callback_domain + proxy)")
	defaultCurrency := fs.String("currency", "CNY", "default currency (currency.default)")
	timezone := fs.String("timezone", "Asia/Shanghai", "panel timezone (panel.timezone), e.g. Asia/Shanghai")
	if err := fs.Parse(args); err != nil {
		log.Fatalf("parse flags: %v", err)
	}

	env := loadEnv()
	dd := strings.TrimSpace(*dataDir)
	if dd == "" {
		dd = env["SAFE_PATHS"]
	}
	if dd == "" {
		dd = "data"
	}
	if abs, err := filepath.Abs(dd); err == nil {
		dd = abs
	}
	for _, d := range []string{filepath.Join(dd, "sqlite"), filepath.Join(dd, "certs"), filepath.Join(dd, "logs")} {
		_ = os.MkdirAll(d, 0750)
	}

	// 端口默认随机（NewDraft 已分配空闲端口）；仅当命令行显式给出时覆盖。
	draft := bootstrap.NewDraft(dd)
	// 监听地址由"是否配置证书"决定（见 Complete），命令行 --listen-addr 仅作记录。
	_ = *listenAddr
	if *listenPort != "" {
		draft.ListenPort = *listenPort
	}
	if *subPort != "" {
		draft.SubscriptionPort = *subPort
	}
	if *portalPort != "" {
		draft.PortalPort = *portalPort
	}
	draft.WebPath = *webPath
	if *adminUser != "" {
		draft.AdminUsername = strings.TrimSpace(*adminUser)
	}
	if *adminPass != "" {
		draft.AdminPassword = *adminPass
	}
	// 数据库 / 代理 / 核心 / 域名。
	draft.DBType = strings.ToLower(strings.TrimSpace(*dbType))
	draft.DBHost, draft.DBPort = *dbHost, *dbPort
	draft.DBUser, draft.DBPassword, draft.DBName = *dbUser, *dbPass, *dbName
	draft.DBSSLMode = *dbSSLMode
	draft.ProxyType = strings.ToLower(strings.TrimSpace(*proxyType))
	draft.Cores = splitCSV(*coresFlag)
	draft.PanelDomain, draft.SubscriptionDomain, draft.PortalDomain = *panelDomain, *subDomain, *portalDomain
	draft.DefaultCurrency = strings.ToUpper(strings.TrimSpace(*defaultCurrency))
	if tz := strings.TrimSpace(*timezone); tz != "" {
		draft.Timezone = tz
	}

	dbCfg := draft.DBConfig(dd)
	if err := bootstrap.EnsureDatabase(dbCfg, draft.DBName); err != nil {
		log.Fatalf("init: database: %v", err)
	}
	if err := db.Init(dbCfg); err != nil {
		log.Fatalf("init: failed to init database: %v", err)
	}
	defer db.Close()
	if err := migration.AutoMigrate(db.DB); err != nil {
		log.Fatalf("init: failed to run migrations: %v", err)
	}
	// HTTPS 证书复用面板既有证书库（internal/service/certificate），不另建目录。
	certSvc := certificate.NewService(repository.NewCertificateRepository(db.DB), dd, "", "")
	switch strings.ToLower(strings.TrimSpace(*tlsMode)) {
	case "", "none":
	case "self_signed":
		cp, kp, err := bootstrap.SelfSigned(context.Background(), dd, certSvc, splitCSV(*tlsDomain))
		if err != nil {
			log.Fatalf("init: self-signed certificate failed: %v", err)
		}
		draft.TLSMode, draft.TLSCertPath, draft.TLSKeyPath = "self_signed", cp, kp
	case "acme":
		domains := splitCSV(*tlsDomain)
		if *tlsIPCert {
			// IP 证书：自动取本机公网 IPv4/IPv6，--tls-domain 可再手动追加。
			domains = append(bootstrap.LocalPublicIPs(), domains...)
		}
		cp, kp, err := bootstrap.IssueACME(context.Background(), dd, certSvc, bootstrap.ACMEDeps{
			EAB: certificate.NewEABConfigService(repository.NewEABConfigRepository(db.DB)),
			DNS: certificate.NewDNSChannelService(repository.NewDNSChannelRepository(db.DB)),
		}, bootstrap.IssueACMEOptions{
			Domains:      domains,
			Email:        *tlsEmail,
			Provider:     *acmeProvider,
			Challenge:    *tlsChallenge,
			KeyType:      *tlsKeyType,
			KeyBits:      int(*tlsKeyBits),
			DNSChannelID: int64(*tlsDNSChannel),
			DNSProvider:  *tlsDNSProvider,
			IPCert:       *tlsIPCert,
		})
		if err != nil {
			log.Fatalf("init: ACME issuance failed: %v", err)
		}
		draft.TLSMode, draft.TLSCertPath, draft.TLSKeyPath = "acme", cp, kp
	default:
		log.Fatalf("init: unsupported tls-mode %q", *tlsMode)
	}
	if err := draft.Validate(); err != nil {
		log.Fatalf("init: invalid config: %v", err)
	}

	if err := bootstrap.Complete(context.Background(), dd, draft); err != nil {
		log.Fatalf("init: %v", err)
	}
	log.Printf("初始化完成：管理员 %s，端口 %s:%s，访问路径 %s", draft.AdminUsername, draft.ResolvedListenAddr(), draft.ListenPort, draft.WebPath)
	if *adminPass == "" {
		log.Printf("[startup] 管理员随机口令（仅显示这一次）：%s", draft.AdminPassword)
	}
	log.Printf("请运行 ./corefusion server 启动面板。")

	// 顺带确保 CA / 本地节点记录存在（与正式启动一致），失败不阻塞。
	if caRepo := repository.NewCARootRepository(db.DB); true {
		if _, err := agent.EnsureCA(dd, caRepo); err != nil {
			log.Printf("init: warning: ensure CA: %v", err)
		}
	}
}

// runResetInit 强制重新初始化（设计 3.6.2）：删除 .env 与初始化标记，保留数据目录。
func runResetInit(args []string) {
	env := loadEnv()
	dd := env["SAFE_PATHS"]
	if dd == "" {
		dd = "data"
	}
	if abs, err := filepath.Abs(dd); err == nil {
		dd = abs
	}

	fmt.Println("警告: 此操作将删除初始化配置（.env）与初始化标记，并重置管理员/访问路径。")
	fmt.Println("数据目录将保留。")
	fmt.Print("请输入 \"RESET\" 确认: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(line) != "RESET" {
		fmt.Println("已取消。")
		return
	}

	dbCfg := db.ConfigFromEnv(env, filepath.Join(dd, "sqlite", "app.db"))
	if err := db.Init(dbCfg); err == nil {
		if err := db.DB.Where("role IN ?", []string{"super_admin", "admin"}).Delete(&entity.User{}).Error; err != nil {
			log.Printf("reset: delete admins: %v", err)
		}
		// key 是 MySQL 保留字：MySQL 用反引号，PostgreSQL/SQLite 用双引号
		// （写死反引号会让 PostgreSQL 上的 reset-init 清不掉初始化标记）。
		keyCol := `"key"`
		if db.DB.Dialector.Name() == "mysql" {
			keyCol = "`key`"
		}
		if err := db.DB.Exec("DELETE FROM system_settings WHERE "+keyCol+" IN (?, ?)", "bootstrap.completed", "panel.web_path").Error; err != nil {
			log.Printf("reset: clear init marks: %v", err)
		}
		db.Close()
	} else {
		log.Printf("reset: database not reachable, marks left as-is: %v", err)
	}
	bootstrap.RemoveSession(dd)
	if err := os.Remove(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("reset: remove .env: %v", err)
	}
	fmt.Println("重置完成，请重新运行 ./corefusion 进行初始化。")
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
