package cert

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	// DNS providers - only import what we need
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/dnspod"
	"github.com/go-acme/lego/v4/providers/dns/gcloud"
	"github.com/go-acme/lego/v4/providers/dns/godaddy"
	"github.com/go-acme/lego/v4/providers/dns/namecheap"
	"github.com/go-acme/lego/v4/providers/dns/route53"
)

// CA provider URLs
const (
	CALetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
	CALetsEncrypt        = "https://acme-v02.api.letsencrypt.org/directory"
	CAZeroSSL            = "https://acme.zerossl.com/v2/DV90"
	CAGooglePublicCA     = "https://dv.acme-v02.api.pki.goog/directory"
)

// CAProvider represents an ACME CA provider.
type CAProvider string

const (
	CAProviderLetsEncrypt CAProvider = "lets_encrypt"
	// CAProviderLetsEncryptStaging 走 LE 的 staging 目录：测试签发时用它，
	// 避免消耗正式环境的速率/失败配额（证书不被浏览器信任，仅用于验证流程 ✓）。
	CAProviderLetsEncryptStaging CAProvider = "lets_encrypt_staging"
	CAProviderZeroSSL            CAProvider = "zerossl"
	CAProviderGoogle             CAProvider = "google"
)

// ChallengeType represents the ACME challenge type.
type ChallengeType string

const (
	ChallengeHTTP01 ChallengeType = "http-01"
	ChallengeDNS01  ChallengeType = "dns-01"
)

// DNSProvider represents a DNS provider for DNS-01 challenge.
type DNSProvider string

const (
	DNSProviderCloudflare DNSProvider = "cloudflare"
	DNSProviderAliDNS     DNSProvider = "alidns"
	DNSProviderDNSPod     DNSProvider = "dnspod"
	DNSProviderRoute53    DNSProvider = "route53"
	DNSProviderGoogle     DNSProvider = "google"
	DNSProviderGoDaddy    DNSProvider = "godaddy"
	DNSProviderNamecheap  DNSProvider = "namecheap"
)

// ACMEClient handles ACME certificate operations using go-acme/lego.
type ACMEClient struct {
	// challengeProvider 是自定义 HTTP-01 挑战提供者（webroot 等）；为空则退回内置 provider ✓
	challengeProvider challenge.Provider
	dataDir           string
	email             string
	caProvider        CAProvider
	caURL             string
	accountDir        string
	http01Provider    *HTTP01Provider // Custom HTTP-01 provider for Gin integration
}

// NewACMEClient creates a new ACME client.
func NewACMEClient(dataDir, email string, caProvider CAProvider) *ACMEClient {
	caURL := CALetsEncrypt
	switch caProvider {
	case CAProviderZeroSSL:
		caURL = CAZeroSSL
	case CAProviderGoogle:
		caURL = CAGooglePublicCA
	case CAProviderLetsEncryptStaging:
		caURL = CALetsEncryptStaging
	case CAProviderLetsEncrypt, "":
		caURL = CALetsEncrypt
	}

	return &ACMEClient{
		dataDir:    dataDir,
		email:      email,
		caProvider: caProvider,
		caURL:      caURL,
		accountDir: filepath.Join(dataDir, "acme", "accounts"),
	}
}

// NewACMEClientWithHTTP01 creates a new ACME client with a custom HTTP-01 provider.
// The http01Provider integrates with the existing Gin router to serve ACME challenges.
func NewACMEClientWithHTTP01(dataDir, email string, caProvider CAProvider, http01Provider *HTTP01Provider) *ACMEClient {
	client := NewACMEClient(dataDir, email, caProvider)
	client.http01Provider = http01Provider
	return client
}

// NewACMEClientWithChallenge 用**任意** challenge.Provider 提供 HTTP-01 挑战。
//
// 用途（节点侧签发）：节点 80 端口常已被 nginx/caddy 占用 ✗（例如跑着 Harbor），
// 此时既不能停服务 ✗ 也不该独占 80 ✗ → 用 WebrootProvider 把挑战文件写进现有站点根目录 ✓，
// 由现有 web 服务器直接提供 ✓（无需重启 ✓）。

func NewACMEClientWithChallenge(dataDir, email string, caProvider CAProvider, provider challenge.Provider) *ACMEClient {
	client := NewACMEClient(dataDir, email, caProvider)
	client.challengeProvider = provider
	return client
}

// IssueResult contains the result of a certificate issuance.
type IssueResult struct {
	CertPath       string
	KeyPath        string
	FullchainPath  string
	CertContent    string
	KeyContent     string
	ExpiresAt      time.Time
	AccountKeyPath string
}

// CertificateInfo 是证书自检用的摘要信息。
type CertificateInfo struct {
	Subject     string
	Issuer      string
	DNSNames    []string
	IPAddresses []string
	NotBefore   time.Time
	NotAfter    time.Time
}

// ParseCertificateInfo 解析 PEM 证书的 subject/issuer/SAN/有效期（Agent 侧自检用 ✓）。
// 为什么要自检："签出来了" 不等于 "签对了" ✗ —— 必须确认 SAN 里真的有申请的那个 IP/域名 ✓。
func ParseCertificateInfo(pemBytes []byte) (*CertificateInfo, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	ips := make([]string, 0, len(c.IPAddresses))
	for _, ip := range c.IPAddresses {
		ips = append(ips, ip.String())
	}
	return &CertificateInfo{
		Subject:     c.Subject.String(),
		Issuer:      c.Issuer.String(),
		DNSNames:    c.DNSNames,
		IPAddresses: ips,
		NotBefore:   c.NotBefore,
		NotAfter:    c.NotAfter,
	}, nil
}

// DNSProviderConfig holds DNS provider credentials.
type DNSProviderConfig struct {
	Provider DNSProvider       `json:"provider"`
	EnvVars  map[string]string `json:"env_vars"`
}

// IssueOptions contains options for certificate issuance.
type IssueOptions struct {
	Domains       []string           `json:"domains"`
	ChallengeType ChallengeType      `json:"challenge_type"`
	DNSProvider   *DNSProviderConfig `json:"dns_provider,omitempty"`
	EAB           *EABConfig         `json:"eab,omitempty"` // Required for ZeroSSL and Google CA
	// Profile 是 ACME 的证书 profile（RFC 8555 §7.1.4 扩展）。
	// **IP 证书必须用 `shortlived`** ✗：Let's Encrypt 从 2026-01-15 起 GA 支持 IP 证书，
	// 但策略上 IP 证书必须是短时证书（约 6 天）→ 不带该 profile 会被 CA 拒绝。
	Profile string `json:"profile,omitempty"`
}

// EABConfig holds External Account Binding credentials.
// Required for CAs that need EAB (ZeroSSL, Google Public CA).
type EABConfig struct {
	KeyID  string `json:"key_id"`  // EAB key ID
	MACKey string `json:"mac_key"` // EAB MAC key (base64url encoded)
}

// Issue issues a new certificate.
func (c *ACMEClient) Issue(ctx context.Context, opts IssueOptions) (*IssueResult, error) {
	if len(opts.Domains) == 0 {
		return nil, fmt.Errorf("at least one domain required")
	}

	challengeType := opts.ChallengeType
	if challengeType == "" {
		challengeType = ChallengeHTTP01
	}

	// Load or create account
	user, err := c.loadOrCreateAccount()
	if err != nil {
		return nil, fmt.Errorf("load account: %w", err)
	}

	// Create lego client
	client, err := c.createClient(user, hasIPIdentifier(opts.Domains))
	if err != nil {
		return nil, err
	}

	// Setup challenge
	if err := c.setupChallenge(client, challengeType, opts.DNSProvider); err != nil {
		return nil, fmt.Errorf("setup challenge: %w", err)
	}

	// Register if needed
	if user.Registration == nil {
		var reg *registration.Resource
		var regErr error

		if opts.EAB != nil && opts.EAB.KeyID != "" {
			// EAB registration (ZeroSSL, Google CA)
			reg, regErr = client.Registration.RegisterWithExternalAccountBinding(
				registration.RegisterEABOptions{
					TermsOfServiceAgreed: true,
					Kid:                  opts.EAB.KeyID,
					HmacEncoded:          opts.EAB.MACKey,
				})
		} else {
			// Standard registration (Let's Encrypt)
			reg, regErr = client.Registration.Register(registration.RegisterOptions{
				TermsOfServiceAgreed: true,
			})
		}

		if regErr != nil {
			return nil, fmt.Errorf("register: %w", regErr)
		}
		user.Registration = reg
		c.saveAccount(user)
	}

	// Request certificate
	certResource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: opts.Domains,
		Bundle:  true,
		Profile: opts.Profile,
	})
	if err != nil {
		return nil, fmt.Errorf("obtain certificate: %w", err)
	}

	// Save certificate
	result, err := c.saveCertificate(opts.Domains[0], certResource)
	if err != nil {
		return nil, err
	}

	log.Printf("[ACME] certificate issued for %v via %s, expires at %s",
		opts.Domains, c.caProvider, result.ExpiresAt.Format(time.RFC3339))

	return result, nil
}

// Renew renews an existing certificate.
func (c *ACMEClient) Renew(ctx context.Context, domains []string, certPath, keyPath string, dnsProvider *DNSProviderConfig) (*IssueResult, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read cert: %w", err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}

	// Parse private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("invalid key PEM")
	}

	key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}

	// Create user with existing key
	user := &ACMEUser{
		Email: c.email,
		key:   key,
	}

	client, err := c.createClient(user, hasIPIdentifier(domains))
	if err != nil {
		return nil, err
	}

	// Setup challenge
	challengeType := ChallengeHTTP01
	if dnsProvider != nil {
		challengeType = ChallengeDNS01
	}
	if err := c.setupChallenge(client, challengeType, dnsProvider); err != nil {
		return nil, err
	}

	// Register
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	user.Registration = reg

	// Renew
	certResource, err := client.Certificate.Renew(certificate.Resource{
		Domain:      domains[0],
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}, true, false, "")
	if err != nil {
		return nil, fmt.Errorf("renew: %w", err)
	}

	result, err := c.saveCertificate(domains[0], certResource)
	if err != nil {
		return nil, err
	}

	log.Printf("[ACME] certificate renewed for %v, expires at %s",
		domains, result.ExpiresAt.Format(time.RFC3339))

	return result, nil
}

// hasIPIdentifier 判断申请里是否含 IP 标识符（IP 证书与域名证书的 CSR 规则不同 ✓）。
func hasIPIdentifier(domains []string) bool {
	for _, d := range domains {
		if net.ParseIP(strings.TrimSpace(d)) != nil {
			return true
		}
	}
	return false
}

// createClient creates a lego client.
//
// disableCommonName=true 时 CSR 里**不写 CommonName** ✓ —— 这是 IP 证书的硬要求：
// RFC 8738 规定 IP 证书只能靠 **IP SAN** 表达身份；LE 对"CN 是 IP"的 CSR 直接报
// `badCSR :: CSR contains IP address in Common Name` ✗（真机实测踩到 ✓）。
func (c *ACMEClient) createClient(user *ACMEUser, disableCommonName bool) (*lego.Client, error) {
	config := lego.NewConfig(user)
	config.CADirURL = c.caURL
	config.Certificate.KeyType = certcrypto.EC256
	config.Certificate.DisableCommonName = disableCommonName

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return client, nil
}

// setupChallenge sets up the challenge solver.
func (c *ACMEClient) setupChallenge(client *lego.Client, challengeType ChallengeType, dnsConfig *DNSProviderConfig) error {
	switch challengeType {
	case ChallengeHTTP01, "":
		// 自定义 provider 优先：webroot（把挑战写进现有站点根目录）或 Gin 集成 provider
		if c.challengeProvider != nil {
			log.Printf("[ACME] Using custom HTTP-01 provider (webroot)")
			return client.Challenge.SetHTTP01Provider(c.challengeProvider)
		}
		if c.http01Provider != nil {
			log.Printf("[ACME] Using integrated HTTP-01 provider (serves via existing web server)")
			return client.Challenge.SetHTTP01Provider(c.http01Provider)
		}
		// Fallback to lego's built-in provider server on port 80
		log.Printf("[ACME] Using built-in HTTP-01 provider on port 80")
		return client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80"))

	case ChallengeDNS01:
		if dnsConfig == nil {
			return fmt.Errorf("dns provider config required for dns-01 challenge")
		}
		provider, err := c.createDNSProvider(dnsConfig)
		if err != nil {
			return err
		}
		// ⚠ 这里原本传的是 `dns01.CondOption(true, dns01.CondOption(true, dns01.CondOption(true, nil)))` ✗：
		// 最里层就是 **nil** ✗ → lego 在 `dns01.NewChallenge` 里执行 `opt(chlg)` 时**空指针 panic** ✗
		// → **dns-01 从来就没能工作过** ✗（真机踩到 ✓：请求 500、日志停在 dns_challenge.go:66 ✓）。
		opts := []dns01.ChallengeOption{
			// Cloudflare 写入 TXT 后，权威 NS 常常还没刷新 ✓ → 不强制"权威 NS 一致" ✗，
			// 但仍按下面的超时等递归解析器看到记录 ✓（否则很容易卡死在传播检查里 ✗）。
			dns01.DisableAuthoritativeNssPropagationRequirement(),
			// ⚠ 默认会用**本机/内网 DNS**（真机看到 192.168.2.84:53 ✗）→ Cloudflare 明明已写入 ✓，
			// 内网解析器却要 2 分钟才刷新 ✗ → 每个域名都白等 2 分钟 ✗。
			// 直接查公共解析器 ✓ → 等待从"分钟级"降到"秒级" ✓。
			dns01.AddRecursiveNameservers([]string{"1.1.1.1:53", "8.8.8.8:53"}),
			// 明确要求"**递归解析器**能看到记录"✓ —— 否则下面那条 DisableAuthoritative… 一开，
			// 两道检查就都不做了 ✗ → lego 会**立刻**说"就绪" ✗ → CA 去查时记录还没发布 ✗
			// （真机踩到：`No TXT record found at _acme-challenge.…` 403 ✗）。
			dns01.RecursiveNSsPropagationRequirement(),
			// ⚠ PropagationWait 的语义是"**先无条件睡 wait**，再检查"✗，不是"最多等 wait" ✗
			// （真机实测：写 2m 就每个域名都慢 2 分钟 ✗，不是"最多等 2 分钟" ✗）。
			//
			// ⚠⚠ 第二个参数必须是 **true（睡够就直接放行、不做本地校验）** ✗：
			//   目标机器（节点/内网）**常常连不到公共解析器** ✗ —— 1.8 实测卡在
			//   `Waiting for DNS record propagation` 100+ 秒 ✗，而记录其实早就发布了 ✓;
			//   本地校验不过 ≠ 记录没发布 ✗，只会白白拖死签发 ✗。
			//   Cloudflare 是**秒级**发布 ✓ → 睡 45 秒足够 ✓，之后交给 **CA 自己的解析器**验证 ✓
			//   （它一定能看到 ✓）→ 面板本机与各节点都不再依赖本机 DNS ✓。
			dns01.PropagationWait(45*time.Second, true),
		}
		return client.Challenge.SetDNS01Provider(provider, opts...)

	default:
		return fmt.Errorf("unsupported challenge type: %s", challengeType)
	}
}

// createDNSProvider creates a DNS provider based on config.
func (c *ACMEClient) createDNSProvider(config *DNSProviderConfig) (challenge.Provider, error) {
	// 先把"用户/脚本里常见的写法"归一到 lego 认的标准名 ✓
	// （例如 Cloudflare 的 `CF_Token` → `CF_DNS_API_TOKEN` ✓，见 dns_env.go ✓）
	for k, v := range normalizeDNSEnvVars(config.Provider, config.EnvVars) {
		if err := os.Setenv(k, v); err != nil {
			return nil, fmt.Errorf("set dns env %s: %w", k, err)
		}
	}

	// ⚠ 关键 ✗：必须用各提供者的 **`NewDNSProvider()`** —— 只有它才会**从环境变量读凭据** ✓；
	// 而 `NewDefaultConfig()` 只是**构造一个默认结构体**（云flare 等实现里根本不读环境变量 ✗）→
	// 用它再交给 `NewDNSProviderConfig(cfg)` 会得到"凭据为空"的报错 ✗
	// （真机现象：填了 `CF_Token` 也报 `invalid credentials: ... must be set` ✗，
	//   看起来像没填，其实是工厂拿错了构造函数 ✗）。
	switch config.Provider {
	case DNSProviderCloudflare:
		return cloudflare.NewDNSProvider()
	case DNSProviderAliDNS:
		return alidns.NewDNSProvider()
	case DNSProviderDNSPod:
		return dnspod.NewDNSProvider()
	case DNSProviderRoute53:
		return route53.NewDNSProvider()
	case DNSProviderGoogle:
		return gcloud.NewDNSProvider()
	case DNSProviderGoDaddy:
		return godaddy.NewDNSProvider()
	case DNSProviderNamecheap:
		return namecheap.NewDNSProvider()
	default:
		return nil, fmt.Errorf("unsupported dns provider: %s", config.Provider)
	}
}

// loadOrCreateAccount loads an existing ACME account or creates a new one.
func (c *ACMEClient) loadOrCreateAccount() (*ACMEUser, error) {
	accountPath := c.accountPath()

	// Try to load existing account
	if data, err := os.ReadFile(accountPath); err == nil {
		var accountData AccountData
		if err := json.Unmarshal(data, &accountData); err == nil {
			key, err := loadPrivateKey(accountData.KeyPEM)
			if err == nil {
				user := &ACMEUser{
					Email: accountData.Email,
					key:   key,
				}
				if accountData.RegistrationURI != "" {
					user.Registration = &registration.Resource{
						URI: accountData.RegistrationURI,
					}
				}
				return user, nil
			}
		}
	}

	// Create new account
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	user := &ACMEUser{
		Email: c.email,
		key:   key,
	}

	return user, nil
}

// saveAccount saves the ACME account to disk.
func (c *ACMEClient) saveAccount(user *ACMEUser) error {
	if err := os.MkdirAll(c.accountDir, 0750); err != nil {
		return err
	}

	keyPEM, err := x509.MarshalECPrivateKey(user.key.(*ecdsa.PrivateKey))
	if err != nil {
		return err
	}

	accountData := AccountData{
		Email:  user.Email,
		CAURL:  c.caURL,
		KeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyPEM})),
	}

	if user.Registration != nil {
		accountData.RegistrationURI = user.Registration.URI
	}

	data, err := json.MarshalIndent(accountData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.accountPath(), data, 0600)
}

// accountPath returns the path to the account file.
func (c *ACMEClient) accountPath() string {
	// Use CA provider name as filename
	caName := string(c.caProvider)
	if caName == "" {
		caName = "lets_encrypt"
	}
	return filepath.Join(c.accountDir, caName+".json")
}

// saveCertificate saves a certificate resource to disk.
// Paths are stored relative to dataDir for portability.
func (c *ACMEClient) saveCertificate(domain string, certResource *certificate.Resource) (*IssueResult, error) {
	certDir := filepath.Join(c.dataDir, "certs", domain)
	if err := os.MkdirAll(certDir, 0750); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	// Store relative paths for portability
	certPath := filepath.Join("certs", domain, "cert.pem")
	keyPath := filepath.Join("certs", domain, "key.pem")
	fullchainPath := filepath.Join("certs", domain, "fullchain.pem")

	// Write files using full paths
	if err := os.WriteFile(filepath.Join(c.dataDir, certPath), certResource.Certificate, 0640); err != nil {
		return nil, fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(filepath.Join(c.dataDir, keyPath), certResource.PrivateKey, 0600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	if err := os.WriteFile(filepath.Join(c.dataDir, fullchainPath), certResource.Certificate, 0640); err != nil {
		return nil, fmt.Errorf("write fullchain: %w", err)
	}

	// ⚠ 这里原本还有一段**用相对路径**的重复写入（os.WriteFile(certPath, ...) 没拼 dataDir ✗）：
	// 在面板里因为进程 CWD 恰好是数据目录而侥幸没事，但只要 CWD 不同（例如 Agent 侧 `cert issue`）
	// 就会在最后一步报 `write cert: open certs/<domain>/cert.pem: no such file or directory` ✗，
	// 让**明明已经签发成功的证书**被当成失败（真机在 1.8 上踩到 ✓）。上面的绝对路径写入才是唯一的。

	expiresAt, _ := parseCertExpiry(certResource.Certificate)
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(90 * 24 * time.Hour)
	}

	return &IssueResult{
		CertPath:      certPath,
		KeyPath:       keyPath,
		FullchainPath: fullchainPath,
		CertContent:   string(certResource.Certificate),
		KeyContent:    string(certResource.PrivateKey),
		ExpiresAt:     expiresAt,
	}, nil
}

// AccountData is the persisted ACME account.
type AccountData struct {
	Email           string `json:"email"`
	CAURL           string `json:"ca_url"`
	KeyPEM          string `json:"key_pem"`
	RegistrationURI string `json:"registration_uri"`
}

// loadPrivateKey loads an EC private key from PEM.
func loadPrivateKey(pemStr string) (crypto.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// parseCertExpiry parses the expiry date from a PEM-encoded certificate.
func parseCertExpiry(certPEM []byte) (time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("invalid PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}

	return cert.NotAfter, nil
}

// ACMEUser implements the lego.User interface.
type ACMEUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *ACMEUser) GetEmail() string                        { return u.Email }
func (u *ACMEUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// GetSupportedProviders returns a list of supported DNS providers with their required env vars.
func GetSupportedProviders() map[DNSProvider][]string {
	return map[DNSProvider][]string{
		// 直接按 acme.sh / Cloudflare 官方脚本的写法给字段 ✓（`CF_Token` + `CF_Account_ID` ✓）。
		// 服务端会把别名归一成 lego 认的 `CF_DNS_API_TOKEN` ✓（见 dns_env.go ✓）；
		// 标准名与老式全局 Key（CLOUDFLARE_API_KEY + CLOUDFLARE_EMAIL）也都兼容 ✓。
		DNSProviderCloudflare: {"CF_Token", "CF_Account_ID"},
		DNSProviderAliDNS:     {"ALICLOUD_ACCESS_KEY", "ALICLOUD_SECRET_KEY"},
		DNSProviderDNSPod:     {"DNSPOD_API_KEY"},
		DNSProviderRoute53:    {"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
		DNSProviderGoogle:     {"GCE_PROJECT", "GCE_SERVICE_ACCOUNT_FILE"},
		DNSProviderGoDaddy:    {"GODADDY_API_KEY", "GODADDY_API_SECRET"},
		DNSProviderNamecheap:  {"NAMECHEAP_API_USER", "NAMECHEAP_API_KEY"},
	}
}

// GetCAProviders returns supported CA providers.
func GetCAProviders() map[CAProvider]string {
	return map[CAProvider]string{
		CAProviderLetsEncrypt: CALetsEncrypt,
		CAProviderZeroSSL:     CAZeroSSL,
		CAProviderGoogle:      CAGooglePublicCA,
	}
}
