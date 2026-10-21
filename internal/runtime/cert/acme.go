package cert

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"corefusion/pkg/netutil"

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
	// KeyType / KeyBits 决定**签出来的私钥类型** ✓（ecc / rsa + 位数 ✓）。
	// ⚠ 以前这两个参数**根本没传到这里** ✗，createClient 里写死 EC256 ✗
	// → 用户选 ECC384 / RSA 签出来全是 ECC256 ✗（真机现象 ✓）。
	// 空值或非法值 → 回退 EC256（保持历史默认行为 ✓）。
	KeyType string `json:"key_type,omitempty"`
	KeyBits int    `json:"key_bits,omitempty"`
}

// EABConfig holds External Account Binding credentials.
// Required for CAs that need EAB (ZeroSSL, Google Public CA).
type EABConfig struct {
	KeyID  string `json:"key_id"`  // EAB key ID
	MACKey string `json:"mac_key"` // EAB MAC key (base64url encoded)
}

// 面板侧统一的密钥类型取值（前端也发这两个 ✓）。
const (
	KeyTypeECC = "ecc"
	KeyTypeRSA = "rsa"
)

// NormalizeKeyType 把自由文本归一为 ecc / rsa（空 → ecc = 历史默认 ✓）。
func NormalizeKeyType(keyType string) string {
	switch strings.ToLower(strings.TrimSpace(keyType)) {
	case KeyTypeRSA, "rsa2048", "rsa3072", "rsa4096", "rsa8192":
		return KeyTypeRSA
	default:
		return KeyTypeECC
	}
}

// CertRelPaths 返回某名称在**指定密钥类型**下的相对路径（相对 dataDir ✓）。
//
// 布局约定（必须写死在这里 ✗，否则各处会各写一套 ✓）：
//   - ecc → certs/<name>/cert.pem | key.pem | fullchain.pem   ← **历史布局，保持不变**
//     （已有站点配置/内核配置都引用它 ✓，改了等于把所有引用打断 ✗）
//   - 其他 → certs/<name>/<keytype>/...                        ← RSA 等放子目录
//
// ✗ 为什么必须分开：两种类型的文件名完全相同 ✗ → 同域同时签发 ECC 与 RSA 会
// **互相覆盖** ✗ —— 这正是"同时选 ECC+RSA 只留一份"的磁盘侧原因 ✓。
func CertRelPaths(name, keyType string) (certRel, keyRel, fullchainRel string) {
	dir := name
	if kt := NormalizeKeyType(keyType); kt != KeyTypeECC {
		dir = filepath.Join(dir, kt)
	}
	return filepath.Join("certs", dir, "cert.pem"),
		filepath.Join("certs", dir, "key.pem"),
		filepath.Join("certs", dir, "fullchain.pem")
}

// RSA 密钥位数的上下限 ✓。
//
// ⚠ 上限**按 CA 区分** ✗，不是一个全局常数：
//   - Let's Encrypt（正式/staging）在 finalize 阶段直接拒绝 8192：
//     `urn:ietf:params:acme:error:badCSR :: invalid public key in CSR: key size not supported: 8192`
//     （真机 2026-10-20 踩到 ✓，而且是在 DNS-01 全部通过、白跑几分钟后才失败 ✗）；
//   - **ZeroSSL 支持 8192** ✓ —— 2026-10-20 用真实 DNS-01 在 ZeroSSL 正式环境实测签发成功，
//     签出的证书公钥确认为 8192 位 ✓（所以不能像之前那样全局封杀 ✗）；
//   - Google Public CA 仍按 4096 上限处理 ✓（保守，未实测 8192 ✓）。
const (
	MinRSAKeyBits        = 2048
	MaxLetsEncryptRSAKey = 4096
	MaxZeroSSLRSAKeyBits = 8192
	MaxGoogleRSAKeyBits  = 4096
)

// NormalizeCAProvider 归一 CA 写法 ✓ —— 库里的 `issuer` 既可能是 provider id（`zerossl`），
// 也可能是展示名（`Let's Encrypt` / `Google Trust Services`）✓，
// 按 CA 判定密钥规格时必须先归到同一套枚举 ✗，否则会误判 ✓。
func NormalizeCAProvider(p string) CAProvider {
	s := strings.ToLower(strings.TrimSpace(p))
	switch {
	case s == "", s == "le", s == "letsencrypt", s == "let's encrypt", s == "lets_encrypt":
		return CAProviderLetsEncrypt
	case strings.Contains(s, "staging"):
		return CAProviderLetsEncryptStaging
	// 从证书 issuer 的 O/CN 解析出来的值可能是 "ZeroSSL" / "ZeroSSL GmbH" 之类 ✓
	case strings.HasPrefix(s, "zerossl"), s == "zero ssl":
		return CAProviderZeroSSL
	// 同理 "Google Trust Services" / "Google Trust Services LLC" 都要归到 google ✓
	case strings.HasPrefix(s, "google"):
		return CAProviderGoogle
	default:
		return CAProvider(s)
	}
}

// ValidateKeySpec 校验**客户端（lego）能生成**的密钥规格 ✓ —— 与具体 CA 无关。
//
// 支持：ecc 256/384（lego 只有 EC256/EC384 ✓，没有 EC521 ✗）；
//
//	rsa 2048/3072/4096/8192 ✓（8192 能不能签出取决于 CA，见 ValidateKeySpecForCA ✓）。
//
// 空类型 / 位数 0 = 用类型默认 ✓（保持历史行为）。
func ValidateKeySpec(keyType string, keyBits int) error {
	if NormalizeKeyType(keyType) == KeyTypeRSA {
		switch keyBits {
		case 0, 2048, 3072, 4096, 8192:
			return nil
		default:
			return fmt.Errorf("不支持的 RSA 位数 %d（可选 2048/3072/4096/8192）", keyBits)
		}
	}
	switch keyBits {
	case 0, 256, 384:
		return nil
	case 521:
		return fmt.Errorf("ECC 521 不受支持（lego 只提供 EC256/EC384）；请改用 ECC 256 或 384")
	default:
		return fmt.Errorf("不支持的 ECC 位数 %d（可选 256/384）", keyBits)
	}
}

// ValidateKeySpecForCA 在 ValidateKeySpec 之上叠加**该 CA 的**密钥规格限制 ✓。
//
// 为什么必须按 CA 判定 ✗：RSA 8192 不是"公信 CA 一律不支持"——
// Let's Encrypt 明确拒绝 ✗，但 ZeroSSL 支持 ✓（2026-10-20 真实 DNS-01 实测签出 8192 位证书 ✓）。
// 统一封杀会把 ZeroSSL 本来能用的组合挡在门外 ✗。
func ValidateKeySpecForCA(caProvider CAProvider, keyType string, keyBits int) error {
	if err := ValidateKeySpec(keyType, keyBits); err != nil {
		return err
	}
	if NormalizeKeyType(keyType) != KeyTypeRSA || keyBits != 8192 {
		return nil
	}
	switch NormalizeCAProvider(string(caProvider)) {
	case CAProviderZeroSSL:
		return nil // 实测支持 ✓
	case CAProviderLetsEncrypt, CAProviderLetsEncryptStaging:
		return fmt.Errorf("Let's Encrypt 不支持 RSA 8192（会返回 key size not supported: 8192）；" +
			"请改用 RSA 4096 或更小，或把 CA 切换到 ZeroSSL（ZeroSSL 支持 8192）")
	case CAProviderGoogle:
		return fmt.Errorf("Google Public CA 不支持 RSA 8192（上限 RSA 4096）；请改用 RSA 4096 或切换到 ZeroSSL")
	default:
		return fmt.Errorf("CA %q 不支持 RSA 8192（目前仅 ZeroSSL 支持）；请改用 RSA 4096 或更小", string(caProvider))
	}
}

// resolveKeyType 把面板的 (key_type, key_bits) 映射成 lego 的密钥类型 ✓。
// 位数为 0 或非法 → 用该类型的默认（ecc→256、rsa→2048 ✓）。
func resolveKeyType(keyType string, keyBits int) certcrypto.KeyType {
	if NormalizeKeyType(keyType) == KeyTypeRSA {
		switch keyBits {
		case 3072:
			return certcrypto.RSA3072
		case 4096:
			return certcrypto.RSA4096
		case 8192:
			return certcrypto.RSA8192
		default:
			return certcrypto.RSA2048
		}
	}
	switch keyBits {
	case 384:
		return certcrypto.EC384
	case 0, 256:
		return certcrypto.EC256
	default:
		// ⚠ lego 只支持 EC256 / EC384 ✗（**没有 EC521** ✓）→ 不支持的位数退回 256，
		// 并且**明确告警** ✗（UI 里也不再提供 521 ✓，避免"选了不生效" ✓）。
		log.Printf("[ACME] WARNING: unsupported ecc key bits %d (lego supports 256/384); falling back to EC256", keyBits)
		return certcrypto.EC256
	}
}

// keyTypeOfKey 从已解析的私钥反推密钥类型（续期时用 ✓ —— 续期复用原私钥 ✓，
// 但日志/参数要如实反映它 ✓）。
func keyTypeOfKey(key crypto.PrivateKey) certcrypto.KeyType {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		switch k.N.BitLen() {
		case 3072:
			return certcrypto.RSA3072
		case 4096:
			return certcrypto.RSA4096
		case 8192:
			return certcrypto.RSA8192
		default:
			return certcrypto.RSA2048
		}
	case *ecdsa.PrivateKey:
		switch k.Curve.Params().BitSize {
		case 384:
			return certcrypto.EC384
		default:
			return certcrypto.EC256
		}
	}
	return certcrypto.EC256
}

// parsePrivateKey 解析 PEM 里的私钥：EC / PKCS#8 / PKCS#1 都认 ✓。
// ⚠ 以前只调用 x509.ParseECPrivateKey ✗ → **RSA 证书续期必然失败** ✗。
func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return k, nil
	}
	return x509.ParsePKCS1PrivateKey(der)
}

// Issue issues a new certificate.
func (c *ACMEClient) Issue(ctx context.Context, opts IssueOptions) (*IssueResult, error) {
	if len(opts.Domains) == 0 {
		return nil, fmt.Errorf("at least one domain required")
	}
	// ⚠ 密钥规格必须**在打到 CA 之前**校验 ✗：不支持的组合（如 LE 的 RSA 8192 ✗）
	// 会在 DNS-01 全部通过后的 finalize 阶段才被 CA 拒绝 ✗ —— 白等几分钟、白消耗订单 ✗。
	// 上限**按 CA** 区分 ✓（ZeroSSL 支持 8192 ✓，LE/Google 只到 4096 ✓）。
	if err := ValidateKeySpecForCA(c.caProvider, opts.KeyType, opts.KeyBits); err != nil {
		return nil, err
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

	// Create lego client —— ✅ 这里以前写死 EC256 ✗，用户的 key_type/key_bits 完全没生效 ✗
	keyType := resolveKeyType(opts.KeyType, opts.KeyBits)
	log.Printf("[ACME] issuing %v: key_type=%s bits=%d → lego %s",
		opts.Domains, NormalizeKeyType(opts.KeyType), opts.KeyBits, keyType)
	client, err := c.createClient(user, hasIPIdentifier(opts.Domains), keyType)
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
	result, err := c.saveCertificate(opts.Domains[0], opts.KeyType, certResource)
	if err != nil {
		return nil, err
	}

	log.Printf("[ACME] certificate issued for %v via %s, expires at %s",
		opts.Domains, c.caProvider, result.ExpiresAt.Format(time.RFC3339))

	return result, nil
}

// RenewOptions 是"续期"的参数 ✓ —— 与 IssueOptions 对称 ✓。
//
// ⚠ 为什么必须**显式**给 ChallengeType ✗：以前续期是**靠猜**的 ✗ ——
// `dnsProvider == nil` 就当作 http-01 ✓。于是"创建时选的 dns-01"连同
// "创建时选的那条 DNS 通道凭据"在续期时被静默丢掉 ✗（通配符直接签不出来 ✗，
// 普通域名则被悄悄换了验证方式 ✗）。
// 现在由调用方**如实传入** ✓；传不出来就明确报错 ✓，绝不降级 ✗。
//
// ⚠ 为什么不给 KeyType ✗：续期**复用现有私钥** ✓，落盘目录必须与私钥的真实类型一致 ✗
// —— 库里的 key_type 一旦脏（例如实际是 RSA 却写着 ecc ✓），按库值走会写到另一个目录 ✗，
// 把同域的那份证书覆盖掉 ✗（双证书场景 ✓）。所以这里**按私钥反推** ✓。
type RenewOptions struct {
	Domains       []string
	CertPath      string
	KeyPath       string
	ChallengeType ChallengeType
	// Profile 必须与**创建时**一致 ✓（IP 证书 = shortlived ✓）✗：
	// 不带 profile 去续 IP 证书会被 Let's Encrypt 拒 ✓。
	Profile     string
	EAB         *EABConfig
	DNSProvider *DNSProviderConfig
}

// Renew renews an existing certificate.
//
// 域名不是"调用方随便给的" ✗：lego 会从**现有证书的 SAN** 里取 ✓
// （所以 SAN 不会在续期时被丢掉 ✓）；opts.Domains 用来判断"是否含 IP"（CSR 规则不同 ✓，
// 并决定 DisableCommonName ✓）与决定落盘目录 ✓，
// 因此 **opts.Domains[0] 必须是主域名** ✗（与签发时保持一致 ✓）。
func (c *ACMEClient) Renew(ctx context.Context, opts RenewOptions) (*IssueResult, error) {
	if len(opts.Domains) == 0 {
		return nil, fmt.Errorf("at least one domain required")
	}

	certPEM, err := os.ReadFile(opts.CertPath)
	if err != nil {
		return nil, fmt.Errorf("read cert: %w", err)
	}

	keyPEM, err := os.ReadFile(opts.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}

	// Parse private key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("invalid key PEM")
	}

	// ⚠ 以前只认 EC ✗（x509.ParseECPrivateKey）→ RSA 证书续期必失败 ✗（真机踩点 ✓）
	key, err := parsePrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}

	// 挑战方式：**如实使用**调用方给的值 ✓（空 → http-01 ✓，与历史默认一致 ✓）
	challengeType := opts.ChallengeType
	if challengeType == "" {
		challengeType = ChallengeHTTP01
	}
	if challengeType == ChallengeDNS01 && opts.DNSProvider == nil {
		// ✗ 绝不静默降级成 http-01：那等于把"创建时选的 dns-01"偷偷换掉 ✓
		return nil, fmt.Errorf("dns-01 challenge requires a DNS provider config (DNS 通道凭据缺失，无法续期)")
	}

	// ⚠ 账号密钥必须是**账号自己的**（从账号库读 ✓），绝不能用证书私钥顶替 ✗ ——
	// 真机实测（2026-09-24，证书 id=20 续期 ✓）：
	//   拿证书私钥当账号密钥 → 用它注册出一个**新账号** ✓ → 而 CSR 又用**同一把**私钥 ✓
	//   → LE 在 finalize 直接拒 ✓：`malformed :: certificate public key must be different than account key` ✗；
	//   更糟的是注册成功后的 `saveAccount` 会把 `data/acme/accounts/<ca>.json` **覆盖成该证书的私钥** ✗
	//   （账号身份被换掉 ✗，且每次续期都会重新注册一个账号 ✗ —— 真机已复现并留证 ✓）。
	// 证书私钥只应该出现在 CSR 里 ✓（见下面 `certificate.Resource{PrivateKey: keyPEM}` ✓）。
	user, err := c.loadOrCreateAccount()
	if err != nil {
		return nil, fmt.Errorf("load account: %w", err)
	}

	client, err := c.createClient(user, hasIPIdentifier(opts.Domains), keyTypeOfKey(key))
	if err != nil {
		return nil, err
	}

	// Setup challenge
	if err := c.setupChallenge(client, challengeType, opts.DNSProvider); err != nil {
		return nil, err
	}

	// Register if needed（与 Issue 同口径 ✓）：
	// 账号已存在（有 registration URI ✓）→ **不重复注册** ✓（真机噪声少、也不会误耗 EAB ✓）；
	// 需要首次注册时**必须带上 EAB** ✗ —— Google / ZeroSSL 首次注册强制要求 ✗。
	if user.Registration == nil {
		var reg *registration.Resource
		var regErr error
		if opts.EAB != nil && opts.EAB.KeyID != "" {
			reg, regErr = client.Registration.RegisterWithExternalAccountBinding(
				registration.RegisterEABOptions{
					TermsOfServiceAgreed: true,
					Kid:                  opts.EAB.KeyID,
					HmacEncoded:          opts.EAB.MACKey,
				})
		} else {
			reg, regErr = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		}
		if regErr != nil {
			return nil, fmt.Errorf("register: %w", regErr)
		}
		user.Registration = reg
		c.saveAccount(user)
	}

	// Renew
	// ⚠ 必须用 RenewWithOptions ✗：`Renew(...)` **传不了 Profile** ✓，
	// 而 IP 证书必须 `shortlived` ✓（不带 profile 续 IP 证书会被 LE 拒 ✓）。
	certResource, err := client.Certificate.RenewWithOptions(certificate.Resource{
		Domain:      opts.Domains[0],
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}, &certificate.RenewOptions{
		Bundle:  true,
		Profile: opts.Profile,
	})
	if err != nil {
		return nil, fmt.Errorf("renew: %w", err)
	}

	result, err := c.saveCertificate(opts.Domains[0], panelKeyTypeOfKey(key), certResource)
	if err != nil {
		return nil, err
	}

	log.Printf("[ACME] certificate renewed for %v via %s (challenge=%s), expires at %s",
		opts.Domains, c.caProvider, challengeType, result.ExpiresAt.Format(time.RFC3339))

	return result, nil
}

// panelKeyTypeOfKey 从**已解析的私钥**反推面板口径的密钥类型 ✓（ecc / rsa ✓）——
// 续期落盘目录以它为准 ✓（理由见 RenewOptions 的注释 ✓）。
func panelKeyTypeOfKey(key crypto.PrivateKey) string {
	if _, ok := key.(*rsa.PrivateKey); ok {
		return KeyTypeRSA
	}
	return KeyTypeECC
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
func (c *ACMEClient) createClient(user *ACMEUser, disableCommonName bool, keyType certcrypto.KeyType) (*lego.Client, error) {
	config := lego.NewConfig(user)
	config.CADirURL = c.caURL
	config.Certificate.KeyType = keyType
	config.Certificate.DisableCommonName = disableCommonName

	// lego 默认的 HTTPClient 用**系统解析器**：systemd-resolved 机器上
	// /etc/resolv.conf 往往只有 127.0.0.53 stub → stub 不在时连 ACME 目录都打不开 ✗。
	// 只替换 DialContext（优先读 resolved 上游 nameserver），proxy/TLS/超时保持 lego 原样 ✓。
	applyResolvedDNS(config.HTTPClient)

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return client, nil
}

// applyResolvedDNS patches an http.Client (in place) so DNS is resolved via the
// local upstream nameservers instead of the system resolver. The transport is
// cloned first, so a client shared with the caller is never mutated. Other
// transport settings — notably Proxy — are preserved.
func applyResolvedDNS(client *http.Client) {
	if client == nil {
		return
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport == nil {
		return
	}
	cloned := transport.Clone()
	cloned.DialContext = netutil.NewResolvedDialContext(30 * time.Second)
	client.Transport = cloned
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
func (c *ACMEClient) saveCertificate(domain, keyType string, certResource *certificate.Resource) (*IssueResult, error) {
	// ✅ 路径按密钥类型分开：ecc 用历史布局 ✓，rsa 落到 certs/<domain>/rsa/ ✓
	// （以前两种类型同名同目录 → 互相覆盖 ✗）
	certPath, keyPath, fullchainPath := CertRelPaths(domain, keyType)
	certDir := filepath.Join(c.dataDir, filepath.Dir(certPath))
	if err := os.MkdirAll(certDir, 0750); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

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
