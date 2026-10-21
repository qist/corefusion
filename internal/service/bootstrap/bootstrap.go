package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	seccrypto "corefusion/internal/security/crypto"
	"corefusion/internal/store/db"
)

// 首次启动初始化。
const (
	SessionFileName       = ".corefusion_init"
	BootstrapCompletedKey = "bootstrap.completed"
	// PendingApplyKey 记录"重启后需要落地的安装任务"（核心/代理/站点），
	// 由正式启动流程读取并异步执行，执行后清除。
	PendingApplyKey = "bootstrap.pending_apply"

	InitPortRangeStart = 18000
	InitPortRangeEnd   = 18999
	InitTokenLength    = 32
	// InitSessionTTL 初始化会话有效期（设计 3.2.3「默认 2 小时过期」）。
	InitSessionTTL = 2 * time.Hour

	// BootWebPath 是初始化向导在前端的路由前缀。
	BootWebPath = "/init"

	// LoopbackAddr 是「未配置 TLS 时面板只监听本机」的回环地址（设计 §1）。
	LoopbackAddr = "127.0.0.1"
)

// SupportedCores 是向导「核心安装」步骤可选的核心类型。
// Hysteria2/TUIC 等 QUIC 协议由 sing-box 承载，独立 hysteria2 核心已下线，不在选项内。
var SupportedCores = []string{"xray", "singbox"}

// 代理类型。
const (
	ProxyNone  = "none"
	ProxyNginx = "nginx"
	ProxyCaddy = "caddy"
)

// webPathPattern 校验管理面板访问路径：以 "/" 开头，仅字母/数字/连字符/下划线。
var webPathPattern = regexp.MustCompile(`^/[A-Za-z0-9_-]*$`)

// hostPattern 校验单个域名 / IP。
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// Session 是初始化会话（内存 + data/.corefusion_init 临时文件，加密存储）。
type Session struct {
	Port      int       `json:"port"`
	Token     string    `json:"token"`
	StartTime time.Time `json:"start_time"`
}

// Expired 返回会话是否已超过 InitSessionTTL。
func (s *Session) Expired() bool { return time.Since(s.StartTime) > InitSessionTTL }

// Remaining 返回会话剩余有效期。
func (s *Session) Remaining() time.Duration {
	r := InitSessionTTL - time.Since(s.StartTime)
	if r < 0 {
		return 0
	}
	return r
}

// NewSession 随机分配端口与令牌，生成一个新的初始化会话。
func NewSession() (*Session, error) {
	port, err := allocatePort()
	if err != nil {
		return nil, err
	}
	return &Session{Port: port, Token: randomString(InitTokenLength), StartTime: time.Now()}, nil
}

// SessionPath 返回初始化会话文件路径。
func SessionPath(dataDir string) string { return filepath.Join(dataDir, SessionFileName) }

// Save 加密写入会话文件（0600）。
func (s *Session) Save(dataDir string) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	enc, err := seccrypto.EncryptSecret(string(b))
	if err != nil {
		return err
	}
	path := SessionPath(dataDir)
	if err := os.WriteFile(path, []byte(enc), 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

// LoadSession 读取会话文件；解密失败或内容非法时返回 ok=false。
func LoadSession(dataDir string) (*Session, bool) {
	raw, err := os.ReadFile(SessionPath(dataDir))
	if err != nil {
		return nil, false
	}
	txt := strings.TrimSpace(string(raw))
	if seccrypto.IsEncrypted(txt) {
		dec, derr := seccrypto.DecryptSecret(txt)
		if derr != nil {
			return nil, false
		}
		txt = strings.TrimSpace(dec)
	}
	var s Session
	if err := json.Unmarshal([]byte(txt), &s); err != nil {
		return nil, false
	}
	if s.Port == 0 || s.Token == "" {
		return nil, false
	}
	return &s, true
}

// RemoveSession 删除初始化会话文件（初始化完成 / reset-init 时调用）。
func RemoveSession(dataDir string) { _ = os.Remove(SessionPath(dataDir)) }

// IsFirstBoot 判断是否应进入首次启动初始化模式（设计 3.1.2）。
func IsFirstBoot(env map[string]string, dataDir string) bool {
	if len(env) == 0 {
		if _, err := os.Stat(dataDir); err == nil {
			return false
		}
		return true
	}
	if strings.TrimSpace(env["SAFE_PATHS"]) == "" {
		return true
	}
	if strings.TrimSpace(env["CF_DB_DSN"]) == "" {
		return true
	}
	return false
}

// allocatePort 在 InitPortRangeStart..InitPortRangeEnd 内随机找一个可用端口。
func allocatePort() (int, error) {
	for i := 0; i < 200; i++ {
		p, err := randInt(InitPortRangeStart, InitPortRangeEnd+1)
		if err != nil {
			return 0, err
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p))
		if err == nil {
			_ = ln.Close()
			return p, nil
		}
	}
	return 0, errors.New("no free port in 18000-18999")
}

func randInt(min, max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		return 0, err
	}
	return min + int(n.Int64()), nil
}

// randomString 生成 n 位小写字母+数字的随机串（crypto/rand）。
func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic("crypto/rand unavailable: " + err.Error())
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b)
}

// RandomHex 生成 n 字节的随机十六进制串（用于 JWT/加密密钥，n=32 → 64 字符）。
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// RandomUsername 生成随机管理员用户名（8 位）。
func RandomUsername() string { return "cf" + randomString(6) }

// RandomPassword 生成满足强度要求（大小写+数字+特殊字符）的随机口令。
func RandomPassword(n int) string {
	if n < 8 {
		n = 16
	}
	const (
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		lower   = "abcdefghijkmnpqrstuvwxyz"
		digit   = "23456789"
		special = "!@#%^*-_=+"
	)
	all := upper + lower + digit + special
	pick := func(set string) byte {
		i, _ := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		return set[i.Int64()]
	}
	out := make([]byte, 0, n)
	out = append(out, pick(upper), pick(lower), pick(digit), pick(special))
	for len(out) < n {
		out = append(out, pick(all))
	}
	for i := len(out) - 1; i > 0; i-- {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		j := int(n.Int64())
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

// PasswordStrength 返回口令强度分（0-4）与不满足项（设计 8.2）。
func PasswordStrength(pw string) (int, []string) {
	var errs []string
	if len(pw) < 8 {
		errs = append(errs, "password must be at least 8 characters")
	}
	score := 0
	if regexp.MustCompile(`[A-Z]`).MatchString(pw) {
		score++
	} else {
		errs = append(errs, "password must contain an uppercase letter")
	}
	if regexp.MustCompile(`[a-z]`).MatchString(pw) {
		score++
	} else {
		errs = append(errs, "password must contain a lowercase letter")
	}
	if regexp.MustCompile(`[0-9]`).MatchString(pw) {
		score++
	} else {
		errs = append(errs, "password must contain a digit")
	}
	if regexp.MustCompile(`[^A-Za-z0-9]`).MatchString(pw) {
		score++
	} else {
		errs = append(errs, "password must contain a special character")
	}
	return score, errs
}

// Draft 是初始化向导采集的配置（分步保存，最终由 Complete 落库）。
type Draft struct {
	// --- 步骤 1：数据库 ---
	DBType       string `json:"db_type"` // sqlite | mysql | postgres
	DBHost       string `json:"db_host"`
	DBPort       string `json:"db_port"`
	DBUser       string `json:"db_user"`
	DBPassword   string `json:"db_password"`
	DBName       string `json:"db_name"`
	DBSSLMode    string `json:"db_sslmode"`
	DBSQLitePath string `json:"db_sqlite_path"`

	// --- 步骤 2：数据目录 ---
	DataDir string `json:"data_dir"`

	// --- 步骤 3：env 加密密钥 ---
	JWTSecret           string `json:"jwt_secret"`
	SecretEncryptionKey string `json:"secret_encryption_key"`
	AutoGenerateSecrets bool   `json:"auto_generate_secrets"`

	// --- 步骤 4：核心安装 ---
	Cores []string `json:"cores"`

	// --- 步骤 5：代理选择 + 默认货币 / 时区 ---
	ProxyType string `json:"proxy_type"`
	// DefaultCurrency 写入 currency.default（充值卡/余额等默认货币）。
	DefaultCurrency string `json:"default_currency"`
	// Timezone 写入 panel.timezone（面板与代理进程的 TZ）。
	Timezone string `json:"timezone"`

	// --- 步骤 6：面板 / 订阅 / 门户 ---
	ListenAddr       string `json:"listen_addr"`
	ListenPort       string `json:"listen_port"`
	SubscriptionPort string `json:"subscription_port"`
	PortalPort       string `json:"portal_port"`
	WebPath          string `json:"web_path"`

	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`

	PanelDomain        string `json:"panel_domain"`
	SubscriptionDomain string `json:"subscription_domain"`
	PortalDomain       string `json:"portal_domain"`

	ProxyAddPanel        bool `json:"proxy_add_panel"`
	ProxyAddSubscription bool `json:"proxy_add_subscription"`
	ProxyAddPortal       bool `json:"proxy_add_portal"`

	// --- 步骤 7：HTTPS ---
	TLSMode      string   `json:"tls_mode"`
	TLSDomains   []string `json:"tls_domains"`
	TLSEmail     string   `json:"tls_email"`
	ACMEProvider string   `json:"acme_provider"`

	// ACME 签发细节（完成初始化时写进目标数据库，见 complete.go persistACMEConfig；
	// 自动续期按“创建时的选择”走，所以这里必须全量记录）。
	// 挑战方式/密钥类型见下方 TLSChallenge / TLSKeyType / TLSKeyBits。
	ACMEDNSProvider     string            `json:"acme_dns_provider"`
	ACMEDNSEnv          map[string]string `json:"acme_dns_env"` // dns-01 凭据（会话文件整体加密）
	ACMEEABKeyID        string            `json:"acme_eab_key_id"`
	ACMEEABMACKey       string            `json:"acme_eab_mac_key"`
	ACMEGoogleProjectID string            `json:"acme_google_project_id"`
	ACMEGoogleSAJSON    string            `json:"acme_google_sa_json"`

	TLSChallenge string `json:"tls_challenge"` // http-01 / dns-01（空= http-01）
	TLSKeyType   string `json:"tls_key_type"`  // ecc | rsa（空= ecc）
	TLSKeyBits   int    `json:"tls_key_bits"`

	TLSCertPath string `json:"tls_cert_path"`
	TLSKeyPath  string `json:"tls_key_path"`
}

// NewDraft 返回带默认值的草稿。
func NewDraft(dataDir string) *Draft {
	dd := strings.TrimSpace(dataDir)
	if dd == "" {
		dd = "data"
	}
	d := &Draft{
		DBType:               db.DBTypeSQLite,
		DataDir:              dd,
		AutoGenerateSecrets:  true,
		JWTSecret:            RandomHex(32),
		SecretEncryptionKey:  RandomHex(32),
		ListenAddr:           LoopbackAddr,
		ListenPort:           "8080",
		SubscriptionPort:     "0",
		PortalPort:           "0",
		WebPath:              "/",
		AdminUsername:        RandomUsername(),
		AdminPassword:        RandomPassword(16),
		ProxyType:            ProxyNone,
		DefaultCurrency:      "CNY",
		Timezone:             "Asia/Shanghai",
		ProxyAddPanel:        true,
		ProxyAddSubscription: true,
		ProxyAddPortal:       true,
		DBSSLMode:            "disable",
		DBName:               "corefusion",
	}
	if ports, err := RandomFreePorts(3); err == nil && len(ports) == 3 {
		d.ListenPort = strconv.Itoa(ports[0])
		d.SubscriptionPort = strconv.Itoa(ports[1])
		d.PortalPort = strconv.Itoa(ports[2])
	}
	return d
}

// RandomFreePorts 分配 n 个互不相同的空闲 TCP 端口。
func RandomFreePorts(n int) ([]int, error) {
	if n <= 0 {
		return nil, nil
	}
	lns := make([]net.Listener, 0, n)
	ports := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, l := range lns {
				_ = l.Close()
			}
			return nil, err
		}
		lns = append(lns, ln)
		ports = append(ports, ln.Addr().(*net.TCPAddr).Port)
	}
	for _, l := range lns {
		_ = l.Close()
	}
	return ports, nil
}

// SQLitePath 返回 SQLite 数据库文件路径（默认 <dataDir>/sqlite/app.db）。
func (d *Draft) SQLitePath(dataDir string) string {
	if p := strings.TrimSpace(d.DBSQLitePath); p != "" {
		return p
	}
	dd := strings.TrimSpace(d.DataDir)
	if dd == "" {
		dd = dataDir
	}
	return filepath.Join(dd, "sqlite", "app.db")
}

// DSN 返回写入 .env 的 CF_DB_DSN。
func (d *Draft) DSN(dataDir string) string {
	switch d.DBType {
	case db.DBTypeMySQL:
		return "mysql://" + d.nativeMySQLDSN()
	case db.DBTypePostgres:
		return d.postgresDSN()
	default:
		return d.SQLitePath(dataDir)
	}
}

// postgresDSN 用标准 URI 构造 PG DSN：口令走 percent-encoding。
//
// ⚠ 不转义会踩坑 ✗（真机 2026-10-12 验证）：口令含 '@' 时 `postgres://u:Cf@2026test@host/db`
// 会被 pgx 解析成 host=`2026test@host` → `no such host` ✗。用 url.UserPassword 转义后为
// `postgres://u:Cf%402026test@host/db` ✓。
func (d *Draft) postgresDSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.DBUser, d.DBPassword),
		Host:     d.DBHost + ":" + d.DBPort,
		Path:     "/" + d.DBName,
		RawQuery: "sslmode=" + d.sslMode(),
	}
	return u.String()
}

func (d *Draft) nativeMySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.DBUser, d.DBPassword, d.DBHost, d.DBPort, d.DBName)
}

func (d *Draft) sslMode() string {
	if strings.TrimSpace(d.DBSSLMode) == "" {
		return "disable"
	}
	return strings.TrimSpace(d.DBSSLMode)
}

// DBConfig 返回用于连接/初始化的 db.Config（DSN 已归一化）。
func (d *Draft) DBConfig(dataDir string) db.Config {
	cfg := db.DefaultConfig()
	switch d.DBType {
	case db.DBTypeMySQL:
		cfg.Type = db.DBTypeMySQL
		cfg.DSN = d.nativeMySQLDSN()
		cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime = 100, 10, time.Hour
	case db.DBTypePostgres:
		cfg.Type = db.DBTypePostgres
		cfg.DSN = d.postgresDSN()
		cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime = 100, 10, time.Hour
	default:
		cfg.Type = db.DBTypeSQLite
		cfg.DSN = d.SQLitePath(dataDir)
	}
	return cfg
}

// ResolvedListenAddr 按"面板是否走反向代理 / 是否配置了证书"决定监听地址：
//   - 面板被加入反向代理（add_panel）→ 由代理对外服务，面板只监听回环；
//   - 否则配置了证书（域名或 IP）→ 0.0.0.0（面板自身提供 HTTPS）；
//   - 其余 → 仅回环。
func (d *Draft) ResolvedListenAddr() string {
	if d.viaProxy(d.ProxyAddPanel) {
		return LoopbackAddr
	}
	if d.tlsIssued() {
		return "0.0.0.0"
	}
	return LoopbackAddr
}

// ValidatePort 校验端口字符串；required=false 时允许空值 / "0"。
func ValidatePort(name, value string, required bool) error {
	v := strings.TrimSpace(value)
	if v == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	port, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%s must be an integer", name)
	}
	low := 1
	if !required {
		low = 0
	}
	if port < low || port > 65535 {
		return fmt.Errorf("%s must be between %d and 65535", name, low)
	}
	return nil
}

func validateHost(name, value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return fmt.Errorf("%s is required", name)
	}
	// IP 字面量（内网 IP、127.0.0.1、::1、IPv6 等）允许直接使用。
	if net.ParseIP(v) != nil {
		return nil
	}
	if !hostPattern.MatchString(v) {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

// Validate 校验草稿是否可用于完成初始化。
func (d *Draft) Validate() error {
	switch d.DBType {
	case db.DBTypeSQLite, "":
	case db.DBTypeMySQL, db.DBTypePostgres:
		if err := validateHost("db_host", d.DBHost); err != nil {
			return err
		}
		if err := ValidatePort("db_port", d.DBPort, true); err != nil {
			return err
		}
		if strings.TrimSpace(d.DBUser) == "" {
			return errors.New("db_user is required")
		}
		if strings.TrimSpace(d.DBName) == "" {
			return errors.New("db_name is required")
		}
	default:
		return fmt.Errorf("unsupported db_type: %s", d.DBType)
	}
	if strings.TrimSpace(d.DataDir) == "" {
		return errors.New("data_dir is required")
	}
	if err := ValidatePort("listen_port", d.ListenPort, true); err != nil {
		return err
	}
	if err := ValidatePort("subscription_port", d.SubscriptionPort, false); err != nil {
		return err
	}
	if err := ValidatePort("portal_port", d.PortalPort, false); err != nil {
		return err
	}
	if !webPathPattern.MatchString(d.WebPath) {
		return errors.New("web_path must start with '/' and contain only letters, digits, '-' or '_'")
	}
	if strings.TrimSpace(d.AdminUsername) == "" {
		return errors.New("admin_username is required")
	}
	if _, errs := PasswordStrength(d.AdminPassword); len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	for _, c := range d.Cores {
		if !IsSupportedCore(c) {
			return fmt.Errorf("unsupported core: %s", c)
		}
	}
	switch d.ProxyType {
	case "", ProxyNone, ProxyNginx, ProxyCaddy:
	default:
		return fmt.Errorf("unsupported proxy_type: %s", d.ProxyType)
	}
	for _, v := range []struct{ n, v string }{
		{"panel_domain", d.PanelDomain},
		{"subscription_domain", d.SubscriptionDomain},
		{"portal_domain", d.PortalDomain},
	} {
		if strings.TrimSpace(v.v) != "" {
			host := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v.v), "https://"), "http://")
			if err := validateHost(v.n, host); err != nil {
				return err
			}
		}
	}
	if d.TLSMode != "" && d.TLSCertPath == "" {
		return errors.New("tls certificate is not configured")
	}
	// 时区必须是 tzdata 能解析的 IANA 名称（写进 panel.timezone 后会注入代理进程的 TZ）。
	if tz := strings.TrimSpace(d.Timezone); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return fmt.Errorf("unsupported timezone: %s", tz)
		}
	}
	return nil
}

// IsSupportedCore 判断是否支持某核心类型。
func IsSupportedCore(c string) bool {
	for _, s := range SupportedCores {
		if strings.EqualFold(strings.TrimSpace(c), s) {
			return true
		}
	}
	return false
}

// PathFor returns d.WebPath normalized for the panel ("" means root).
func (d *Draft) PathFor() string {
	p := strings.TrimRight(strings.TrimSpace(d.WebPath), "/")
	if p == "/" {
		p = ""
	}
	return p
}
