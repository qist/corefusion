package certificate

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"

	"corefusion/internal/model/entity"
	"corefusion/internal/runtime/cert"
	"corefusion/internal/store/repository"
)

// Service handles certificate business logic.
type Service struct {
	repo           *repository.CertificateRepository
	dataDir        string
	acmeEmail      string
	caProvider     cert.CAProvider
	http01Provider *cert.HTTP01Provider // Custom HTTP-01 provider for Gin integration
	// dnsResolver 让服务**自己**能把"库里记的 dns_channel_id"解析成凭据 ✓。
	//
	// ⚠ 以前凭据的获取完全推给调用方 ✗（签发接口自己解析 ✓、续期接口却什么都不传 ✗）→
	// 前端点"续期"（不带任何参数 ✓）时，创建时选的 DNS 通道**凭空消失** ✗。
	// 续期的口径是"创建时选什么就用什么" ✓ → 服务必须能自己找到它 ✓（可选注入 ✓）。
	dnsResolver DNSChannelResolver

	// opMu / opsInFlight 让"同一张证书的签发/续期"串行化 ✓（见 beginCertOp ✓）。
	opMu        sync.Mutex
	opsInFlight map[string]bool
}

// ErrCertOpInProgress 表示"同一张证书已经有一次签发/续期在跑" ✓（调用方映射成 409 ✓）。
var ErrCertOpInProgress = errors.New("该证书正在签发/续期中，请等它跑完（上一次操作仍在后台进行）")

// beginCertOp 给一次签发/续期"上锁" ✓，返回释放函数 ✓；已有同名操作在跑时返回 ok=false ✓。
//
// Why: 签发/续期被**故意脱离了请求上下文**（见 detachedCertOpContext ✓）→
// 客户端断开后任务仍在跑 ✓（这正是要的 ✓），于是"用户看界面没反应又点一次" ✗ /
// "自动续期与手动续期撞上" ✗ 都会变成**同一张证书两个续期并发** ✗ →
// 两边各自写证书文件 + 各自 Update 同一行 ✗ → 落库的 cert_content 与磁盘/最终证书可能对不上 ✗，
// 还白白消耗 CA 配额 ✗。
//
// 口径 ✓：把第二次**直接拒掉** ✓（不做排队 ✓）—— 续期是幂等动作 ✓，
// "正在跑，稍后看结果"比"排队再签一张"更好解释 ✓，也不会把 CA 配额翻倍 ✗。
func (s *Service) beginCertOp(key string) (func(), bool) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.opsInFlight == nil {
		s.opsInFlight = make(map[string]bool, 4)
	}
	if s.opsInFlight[key] {
		return nil, false
	}
	s.opsInFlight[key] = true
	return func() {
		s.opMu.Lock()
		delete(s.opsInFlight, key)
		s.opMu.Unlock()
	}, true
}

// detachedCertOpContext 把请求上下文换成"**保留值、去掉取消**"的上下文 ✓（自带超时兜底 ✓）。
//
// Why: dns-01 要等 DNS 传播 ✓（真机实测一次 60s 上下 ✓，慢的时候到 2 分钟 ✓），
// 前端/反代常常在这之前就断开 ✗ → `c.Request.Context()` 被取消 ✗ →
// **证书其实已经签出来了** ✓（日志里已有 `Server responded with a certificate` ✓、磁盘也有产物 ✓），
// 唯独紧接着的**落库**那一步被取消打断 ✗ → 接口 500、库里**没有这张证书** ✗ ——
// 真机 2026-09-23 就踩到过（遗留 `data/certs/*.e2e923.example.com/` 这种"有文件、库里没有"的无主目录 ✓）。
//
// 口径 ✓：**一旦开始签发/续期，就保证把它跑完并落库** ✓ —— 客户端断不断开都一样 ✓
// （DB 是唯一权威 ✓；磁盘产物绝不能变成"没人认领"的孤儿 ✗）。
// 不变的东西 ✗：调用方仍然拿不到结果 ✓（断开的连接不会复活 ✓），只是**结果一定进库** ✓。
func detachedCertOpContext(ctx context.Context) (context.Context, context.CancelFunc) {
	// 15 分钟只是"别无限跑"的兜底 ✓：dns-01 + CA 侧队列的极端情况也够用 ✓。
	return context.WithTimeout(context.WithoutCancel(ctx), 15*time.Minute)
}

// DNSChannelResolver 把"面板里的 DNS 通道 ID"解析成 dns-01 凭据 ✓。
//
// 放在这里（而不是直接依赖 `*DNSChannelService` ✓）有两个原因 ✓：
// 一是服务层只需要这一个能力 ✓；二是便于测试注入假实现 ✓（不碰数据库 ✓）。
type DNSChannelResolver interface {
	ResolveDNSChannel(ctx context.Context, id int64) (*cert.DNSProviderConfig, error)
}

// NewService creates a new certificate service.
func NewService(repo *repository.CertificateRepository, dataDir, acmeEmail string, caProvider cert.CAProvider) *Service {
	return &Service{
		repo:       repo,
		dataDir:    dataDir,
		acmeEmail:  acmeEmail,
		caProvider: caProvider,
	}
}

// SetDNSChannelResolver 注入 DNS 通道解析器 ✓（续期按库里的 dns_channel_id 还原凭据 ✓，可选 ✓）。
func (s *Service) SetDNSChannelResolver(r DNSChannelResolver) { s.dnsResolver = r }

// SetHTTP01Provider sets the custom HTTP-01 provider for ACME challenges.
// This should be called before issuing certificates to enable Gin router integration.
func (s *Service) SetHTTP01Provider(provider *cert.HTTP01Provider) {
	s.http01Provider = provider
}

// GetHTTP01Provider returns the HTTP-01 provider, creating one if needed.
func (s *Service) GetHTTP01Provider() *cert.HTTP01Provider {
	if s.http01Provider == nil {
		s.http01Provider = cert.NewHTTP01Provider()
	}
	return s.http01Provider
}

// SetDefaultACMEEmail 设置面板本机的默认 ACME 账号邮箱 ✓ ——
// ⚠ 不设的话注册时"没有联系邮箱" ✗：Let's Encrypt 能忍 ✓，**Google 直接拒** ✗
// （`Accounts must have at least one contact.` ✓，真机踩到 ✓）。续期路径也吃这一份 ✓。
func (s *Service) SetDefaultACMEEmail(email string) {
	if strings.TrimSpace(email) != "" {
		s.acmeEmail = strings.TrimSpace(email)
	}
}

// getACMEClient returns an ACME client for the given CA provider.
//
// email 非空时优先用它 ✓（面板的 ACME 账号配置 ✓）；为空则退回服务级默认 ✓。
func (s *Service) getACMEClient(caProvider cert.CAProvider, email ...string) *cert.ACMEClient {
	if caProvider == "" {
		caProvider = s.caProvider
	}
	useEmail := s.acmeEmail
	if len(email) > 0 && strings.TrimSpace(email[0]) != "" {
		useEmail = strings.TrimSpace(email[0])
	}
	// Use custom HTTP-01 provider if available for Gin router integration
	if s.http01Provider != nil {
		return cert.NewACMEClientWithHTTP01(s.dataDir, useEmail, caProvider, s.http01Provider)
	}
	return cert.NewACMEClient(s.dataDir, useEmail, caProvider)
}

// CreateCertificateInput is the input for creating a certificate.
type CreateCertificateInput struct {
	Domain        string    `json:"domain" binding:"required"`
	Issuer        string    `json:"issuer" binding:"required"`
	ProviderType  string    `json:"provider_type" binding:"required"`
	CertPath      string    `json:"cert_path" binding:"required"`
	KeyPath       string    `json:"key_path" binding:"required"`
	FullchainPath string    `json:"fullchain_path"`
	AutoRenew     bool      `json:"auto_renew"`
	ExpiresAt     time.Time `json:"expires_at" binding:"required"`
}

// Create creates a new certificate record.
func (s *Service) Create(ctx context.Context, input CreateCertificateInput) (*entity.Certificate, error) {
	// Check if domain already exists
	existing, _ := s.repo.GetByDomain(ctx, input.Domain)
	if existing != nil {
		return nil, errors.New("certificate already exists for this domain")
	}

	autoRenew := 0
	if input.AutoRenew {
		autoRenew = 1
	}

	cert := &entity.Certificate{
		Domain:        input.Domain,
		Issuer:        input.Issuer,
		ProviderType:  input.ProviderType,
		CertPath:      input.CertPath,
		KeyPath:       input.KeyPath,
		FullchainPath: input.FullchainPath,
		Status:        "active",
		AutoRenew:     autoRenew,
		ExpiresAt:     input.ExpiresAt,
	}

	if err := s.repo.Create(ctx, cert); err != nil {
		return nil, err
	}

	return cert, nil
}

// UploadCertificateInput is the input for uploading a certificate.
type UploadCertificateInput struct {
	Name        string `json:"name"`   // User-friendly name
	Domain      string `json:"domain"` // Primary domain (auto-detected from cert if empty)
	CertContent string `json:"cert_content"`
	KeyContent  string `json:"key_content"`
}

// Upload creates a certificate from pasted content or file paths.
func (s *Service) Upload(ctx context.Context, input UploadCertificateInput) (*entity.Certificate, error) {
	if input.CertContent == "" || input.KeyContent == "" {
		return nil, errors.New("cert and key content are required")
	}

	// Save cert content to temp file first to parse it
	// S-7 fix: use random temp file names to prevent concurrent uploads from overwriting each other
	tmpDir := filepath.Join(s.dataDir, "certs_tmp")
	os.MkdirAll(tmpDir, 0750)
	randSuffix := fmt.Sprintf("%x", time.Now().UnixNano())
	tmpCert := filepath.Join(tmpDir, "tmp_cert_"+randSuffix+".pem")
	tmpKey := filepath.Join(tmpDir, "tmp_key_"+randSuffix+".pem")
	os.WriteFile(tmpCert, []byte(input.CertContent), 0600)
	os.WriteFile(tmpKey, []byte(input.KeyContent), 0600)

	// Parse certificate to get CN, SANs, issuer, expiry
	expiresAt, err := parseCertExpiry(tmpCert)
	if err != nil {
		os.Remove(tmpCert)
		os.Remove(tmpKey)
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	issuer := parseCertIssuer(tmpCert)
	sans := parseCertDomains(tmpCert)
	keyType, keyBits, _ := detectKeyType(tmpKey)

	// Determine primary domain: user input > first SAN > CN
	domain := input.Domain
	if domain == "" {
		// Parse first SAN from JSON array
		parsed := parseCertDomainList(tmpCert)
		if len(parsed) > 0 {
			domain = parsed[0]
		}
	}
	if domain == "" {
		domain = "unknown"
	}

	// Generate name: user input > domain
	name := input.Name
	if name == "" {
		name = domain
	}

	// Save to final directory using name as key (unique per cert)
	// ✅ 按**密钥类型**分目录：否则同名上传 RSA + ECC 会互相覆盖 ✗
	certPath, keyPath, _ := cert.CertRelPaths(sanitizeDomain(name), keyType)
	os.MkdirAll(filepath.Join(s.dataDir, filepath.Dir(certPath)), 0750)
	os.WriteFile(filepath.Join(s.dataDir, certPath), []byte(input.CertContent), 0600)
	os.WriteFile(filepath.Join(s.dataDir, keyPath), []byte(input.KeyContent), 0600)
	os.Remove(tmpCert)
	os.Remove(tmpKey)

	cert := &entity.Certificate{
		Name:         name,
		Domain:       domain,
		Domains:      sans,
		Issuer:       issuer,
		ProviderType: "upload",
		KeyType:      keyType,
		KeyBits:      keyBits,
		CertPath:     certPath,
		KeyPath:      keyPath,
		CertContent:  input.CertContent,
		KeyContent:   input.KeyContent,
		Status:       "active",
		AutoRenew:    0,
		ExpiresAt:    expiresAt,
	}

	// If a certificate with the same domain already exists, update it in place
	// so that existing config references pick up the new content automatically.
	if existing, err := s.repo.GetByDomainAndKeyType(ctx, domain, keyType); err == nil {
		existing.Name = name
		existing.Domains = sans
		existing.Issuer = issuer
		existing.ProviderType = "upload"
		existing.KeyType = keyType
		existing.KeyBits = keyBits
		existing.CertPath = certPath
		existing.KeyPath = keyPath
		existing.CertContent = input.CertContent
		existing.KeyContent = input.KeyContent
		existing.Status = "active"
		existing.ExpiresAt = expiresAt
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	if err := s.repo.Create(ctx, cert); err != nil {
		return nil, err
	}

	return cert, nil
}

// parseCertDomainList returns the DNS SANs as a string slice.
func parseCertDomainList(certPath string) []string {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil
	}
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	// DNS SAN + **IP SAN**：IP 证书（RFC 8738）没有 DNSNames，身份全在 IPAddresses 里 ✗
	// —— 只读 DNSNames 会让 IP 证书的 SAN 列表变成 [""] ✗
	domains := certSANIdentifiers(x509Cert)
	if len(domains) > 0 {
		return domains
	}
	if cn := strings.TrimSpace(x509Cert.Subject.CommonName); cn != "" {
		return []string{cn}
	}
	return nil
}

// certSANIdentifiers 返回证书的**全部**标识符：DNS SAN 在前、IP SAN 在后 ✓。
func certSANIdentifiers(c *x509.Certificate) []string {
	out := make([]string, 0, len(c.DNSNames)+len(c.IPAddresses))
	out = append(out, c.DNSNames...)
	for _, ip := range c.IPAddresses {
		out = append(out, ip.String())
	}
	return out
}

// sanitizeDomain converts a domain to a safe directory name.
// S-8 fix: strip path separators and traversal sequences to prevent directory traversal.
func sanitizeDomain(domain string) string {
	// Remove any path components — only keep the base name
	domain = filepath.Base(domain)
	// Replace wildcard and dots, and remove any remaining path separators
	domain = strings.ReplaceAll(strings.ReplaceAll(domain, "*", "_wildcard"), ".", "_")
	domain = strings.ReplaceAll(domain, "/", "_")
	domain = strings.ReplaceAll(domain, "\\", "_")
	domain = strings.ReplaceAll(domain, "..", "_")
	return domain
}

// parseCertExpiry parses a PEM certificate file and returns the expiry time.
func parseCertExpiry(certPath string) (time.Time, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("read cert file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return time.Time{}, errors.New("no PEM block found in certificate")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse x509: %w", err)
	}

	return x509Cert.NotAfter, nil
}

// parseCertDomains parses a PEM certificate file and returns all DNS SANs.
func parseCertDomains(certPath string) string {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return ""
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return ""
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return ""
	}

	// 同 parseCertDomainList：必须包含 IP SAN ✓，否则 IP 证书会存成 [""] ✗
	domains := certSANIdentifiers(x509Cert)
	if len(domains) == 0 {
		if cn := strings.TrimSpace(x509Cert.Subject.CommonName); cn != "" {
			domains = []string{cn}
		}
	}
	if len(domains) == 0 {
		return "[]"
	}

	// 用标准 JSON 编码（域名/文本里出现引号等字符也不会破坏结构 ✓）
	buf, err := json.Marshal(domains)
	if err != nil {
		return "[]"
	}
	return string(buf)
}

// parseCertIssuer parses a PEM certificate file and returns the issuer organization or CN.
func parseCertIssuer(certPath string) string {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "unknown"
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return "unknown"
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "unknown"
	}

	// Try Organization first, then CommonName
	if len(x509Cert.Issuer.Organization) > 0 {
		return x509Cert.Issuer.Organization[0]
	}
	if x509Cert.Issuer.CommonName != "" {
		return x509Cert.Issuer.CommonName
	}
	return "unknown"
}

// detectKeyType detects the key type from a PEM private key file.
func detectKeyType(keyPath string) (string, int, error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", 0, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return "", 0, errors.New("no PEM block found in key")
	}

	// Try PKCS#8 (RSA or EC)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return "rsa", k.N.BitLen(), nil
		case *ecdsa.PrivateKey:
			return "ecc", k.Curve.Params().BitSize, nil
		}
	}

	// Try PKCS#1 RSA
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return "rsa", rsaKey.N.BitLen(), nil
	}

	// Try EC
	ecKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err == nil {
		return "ecc", ecKey.Curve.Params().BitSize, nil
	}

	return "unknown", 0, nil
}

// GetByID retrieves a certificate by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.Certificate, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all certificates with pagination.
func (s *Service) List(ctx context.Context, limit, offset int) ([]entity.Certificate, int64, error) {
	return s.repo.List(ctx, limit, offset)
}

// LandCertificateFiles 把库里那份证书**落地**到面板本机磁盘 ✓
// （`cert.pem` / `key.pem` / `fullchain.pem` ✓）。
//
// 用户口径 ✓：**"面板本身也可以理解成一个节点"** ✓ ——
// **"面板关联哪个证书，就落地哪个证书"** ✓（跟节点一样 ✓）。
// 所以只有被**关联到本机节点（Local (Panel)）**的证书才写文件 ✓；
// 只被远端节点使用的证书**不落地** ✗ —— 库里那份完整副本就够了 ✓，
// 磁盘副本只会多一处需要同步的状态 ✗（真机上出现过"文件在、库为空"的不一致 ✗）。
//
// 内容一律取自**库里的权威副本** ✓（"有人把证书文件删了，库里还完整有一份" ✓
// → 于是任何时候都能按需**重新落地** ✓）。
func (s *Service) LandCertificateFiles(ctx context.Context, certID int64) (*entity.Certificate, error) {
	c, err := s.repo.GetByID(ctx, certID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("certificate %d not found", certID)
	}
	if strings.TrimSpace(c.CertContent) == "" || strings.TrimSpace(c.KeyContent) == "" {
		return nil, fmt.Errorf("证书 %d 在库里没有内容，无法落地（先让它签发或上报一次即可 ✓）", certID)
	}
	name := strings.TrimSpace(c.Name)
	if name == "" {
		name = c.Domain
	}
	// ⚠ 目录名必须与 **`cert_distributor` 的分发路径一致** ✗ —— 它写的是 `certs/<Domain>/` ✓
	// （真机核对过：节点上是 `certs/node-owner.example.com/` ✓）。这里原来用"消毒后的 name" ✗
	// → 面板落地到 `certs/acme-node-owner_example_com/` ✗，而配置里引用的是 `certs/<域名>/` ✗
	// → **文件在、路径不对 → 内核拿不到证书** ✗（真机搭建测试时踩到 ✓）。
	relDir := certDirRel(c) // ✅ 按密钥类型分目录 ✓
	certRel := filepath.Join(relDir, "cert.pem")
	keyRel := filepath.Join(relDir, "key.pem")
	chainRel := filepath.Join(relDir, "fullchain.pem")
	if merr := os.MkdirAll(filepath.Join(s.dataDir, relDir), 0750); merr != nil {
		return nil, merr
	}
	// 权限与签发路径保持一致 ✓：证书 0640（可能被非 root 的服务读 ✓）、私钥 0600 ✓
	if werr := os.WriteFile(filepath.Join(s.dataDir, certRel), []byte(withTrailingNewline(c.CertContent)), 0640); werr != nil {
		return nil, werr
	}
	if werr := os.WriteFile(filepath.Join(s.dataDir, keyRel), []byte(withTrailingNewline(c.KeyContent)), 0600); werr != nil {
		return nil, werr
	}
	// fullchain：库里存的本来就是"叶 + 中间链" ✓（见 runtime/cert/pemutil.go 的说明 ✓）
	if werr := os.WriteFile(filepath.Join(s.dataDir, chainRel), []byte(withTrailingNewline(c.CertContent)), 0640); werr != nil {
		return nil, werr
	}
	c.CertPath = certRel
	c.KeyPath = keyRel
	c.FullchainPath = chainRel
	if uerr := s.repo.Update(ctx, c); uerr != nil {
		return nil, uerr
	}
	log.Printf("[cert] 证书 %d（%s）已落地到面板本机 %s ✓（面板自身也是一个节点 ✓，只有关联到它的证书才落地 ✓）",
		c.ID, name, relDir)
	return c, nil
}

// SyncLocalCopyIfLanded 在证书内容变化后（**续期/重报** ✓）刷新**面板本机**那份副本 ✓。
//
// 为什么必须有它 ✗（真机验证"续期后内核是否用上新证书"时抓到的缺口 ✓）：
// 远端节点在续期后会收到 `auto-SyncData`（里面带证书内容 ✓）→ 文件被刷新 ✓；
// 而**面板本机**没有 sync ✗ → `data/certs/<域名>/` 那份**停在旧证书** ✗
// → 本机内核（caddy / nginx / xray）继续吐**旧证书** ✗ —— 从外面看就是"续期了但没生效" ✗。
//
// 设计上刻意保守 ✓：**只有本机已经落地过**（说明本机节点确实在用这张 ✓）才刷新 ✓，
// 从不主动为本机新建副本 ✗（否则等于把所有证书又都落一份到面板 ✗，违背既定口径 ✓）。
func (s *Service) SyncLocalCopyIfLanded(ctx context.Context, certID int64) error {
	c, err := s.repo.GetByID(ctx, certID)
	if err != nil || c == nil {
		return err
	}
	if strings.TrimSpace(string(c.Domain)) == "" {
		return nil
	}
	local := filepath.Join(s.dataDir, certDirRel(c), "cert.pem")
	if _, serr := os.Stat(local); serr != nil {
		return nil // 本机没这份 → 不动 ✓
	}
	if _, lerr := s.LandCertificateFiles(ctx, certID); lerr != nil {
		return lerr
	}
	log.Printf("[cert] 已把证书 %d 的本机副本刷新为最新（本机节点在用这张证书 ✓）", certID)
	return nil
}

// certDirRel 返回某证书在磁盘上的**相对目录**（按密钥类型 ✓）。
//
// ✗ 为什么需要它：这里以前按域名重算路径 ✗ —— 同域同时签 ECC/RSA 时，
// 这个"落地/刷新本机副本"的动作会把 RSA 记录**又写回 ECC 的旧目录** ✗，
// 等于"分别存储"被抹平 ✗（真机 E2E 抓到的 ✓）。统一走 cert.CertRelPaths ✓。
func certDirRel(c *entity.Certificate) string {
	certRel, _, _ := cert.CertRelPaths(string(c.Domain), c.KeyType)
	return filepath.Dir(certRel)
}

// withTrailingNewline 保证文件以换行结尾 ✓（PEM 常规 ✓，也避免有些解析器抱怨 ✓）。
func withTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

// UpdateCertificateInput is the input for updating a certificate.
type UpdateCertificateInput struct {
	Domain        *string    `json:"domain"`
	Issuer        *string    `json:"issuer"`
	ProviderType  *string    `json:"provider_type"`
	CertPath      *string    `json:"cert_path"`
	KeyPath       *string    `json:"key_path"`
	FullchainPath *string    `json:"fullchain_path"`
	AutoRenew     *bool      `json:"auto_renew"`
	ExpiresAt     *time.Time `json:"expires_at"`
	Status        *string    `json:"status"`
}

// Update updates a certificate.
func (s *Service) Update(ctx context.Context, id int64, input UpdateCertificateInput) (*entity.Certificate, error) {
	cert, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Domain != nil {
		cert.Domain = *input.Domain
	}
	if input.Issuer != nil {
		cert.Issuer = *input.Issuer
	}
	if input.ProviderType != nil {
		cert.ProviderType = *input.ProviderType
	}
	if input.CertPath != nil {
		cert.CertPath = *input.CertPath
	}
	if input.KeyPath != nil {
		cert.KeyPath = *input.KeyPath
	}
	if input.FullchainPath != nil {
		cert.FullchainPath = *input.FullchainPath
	}
	if input.AutoRenew != nil {
		if *input.AutoRenew {
			cert.AutoRenew = 1
		} else {
			cert.AutoRenew = 0
		}
	}
	if input.ExpiresAt != nil {
		cert.ExpiresAt = *input.ExpiresAt
	}
	if input.Status != nil {
		cert.Status = *input.Status
	}

	if err := s.repo.Update(ctx, cert); err != nil {
		return nil, err
	}

	return cert, nil
}

// Delete deletes a certificate by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	// Get cert info first to delete files
	cert, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete certificate files from disk
	if cert.CertPath != "" {
		os.Remove(cert.CertPath)
	}
	if cert.KeyPath != "" {
		os.Remove(cert.KeyPath)
	}
	if cert.FullchainPath != "" {
		os.Remove(cert.FullchainPath)
	}

	// Try to remove the certificate directory if empty
	if cert.CertPath != "" {
		dir := filepath.Dir(cert.CertPath)
		os.Remove(dir) // Only removes if empty
	}

	return s.repo.Delete(ctx, id)
}

// Renew updates the certificate after a successful renewal.
func (s *Service) Renew(ctx context.Context, id int64, newCertPath, newKeyPath, newFullchainPath string, newExpiresAt time.Time) (*entity.Certificate, error) {
	cert, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cert.CertPath = newCertPath
	cert.KeyPath = newKeyPath
	cert.FullchainPath = newFullchainPath
	cert.ExpiresAt = newExpiresAt
	cert.LastRenewedAt = &now

	if err := s.repo.Update(ctx, cert); err != nil {
		return nil, err
	}

	return cert, nil
}

// FindExpiringSoon retrieves certificates expiring within the given duration.
func (s *Service) FindExpiringSoon(ctx context.Context, within time.Duration) ([]entity.Certificate, error) {
	return s.repo.FindExpiringSoon(ctx, within)
}

// checkPort80ForHTTP01 checks port 80 status for HTTP-01 challenges.
// Returns error if port 80 is occupied by an external service.
func (s *Service) checkPort80ForHTTP01() error {
	if s.http01Provider == nil {
		return nil
	}

	status, err := s.http01Provider.CheckPort80Status()
	if err != nil {
		return fmt.Errorf("check port 80: %w", err)
	}

	switch status.Strategy {
	case cert.Port80StrategyDirect:
		// Port 80 is free, will start temporary server
		s.http01Provider.SetStrategy(cert.Port80StrategyDirect)
		log.Printf("[ACME] Port 80 is free, will start temporary HTTP server for ACME challenge")
		return nil

	case cert.Port80StrategyProxy:
		// Panel's proxy is on port 80, use Gin router
		s.http01Provider.SetStrategy(cert.Port80StrategyProxy)
		log.Printf("[ACME] Panel proxy manages port 80, using integrated HTTP-01 provider")
		return nil

	case cert.Port80StrategyConflict:
		// Port 80 is occupied by external service
		return fmt.Errorf("port 80 is occupied by %s, please stop it before issuing HTTP-01 certificate, or use DNS-01 challenge instead", status.Process)

	default:
		return fmt.Errorf("unknown port 80 strategy")
	}
}

// IssueACMEInput is the input for ACME certificate issuance.
type IssueACMEInput struct {
	Domains []string `json:"domains" binding:"required"`
	// KeyType / KeyBits：用户选的密钥类型 ✓（ecc/rsa + 位数 ✓）。
	// ⚠ 以前**入参里根本没有** ✗ → 前端传了也被丢掉 ✗ → 签出来永远是 ECC256 ✗。
	KeyType       string `json:"key_type"`
	KeyBits       int    `json:"key_bits"`
	ChallengeType string `json:"challenge_type"` // "http-01" or "dns-01"
	// DNSChannelID 是本次签发选用的**面板 DNS 通道** ID ✓（凭据仍在 handler 里解析成 DNSProvider ✓）。
	//
	// ⚠ 它的**唯一作用是把选择记下来** ✗：续期要按"创建时选的那条通道"走 ✓，
	// 而服务层拿到的是解析后的凭据（DNSProvider ✓）、**认不出是哪条通道** ✗ →
	// 必须由调用方把 ID 一起带进来 ✓，随证书落库 ✓（真机 bug：续期时选择丢失 ✓）。
	DNSChannelID int64                   `json:"dns_channel_id"`
	CAProvider   cert.CAProvider         `json:"ca_provider"`       // "lets_encrypt" / "lets_encrypt_staging" / "zerossl" / "google"
	Profile      string                  `json:"profile,omitempty"` // IP 证书需 "shortlived"（会自动补）
	DNSProvider  *cert.DNSProviderConfig `json:"dns_provider,omitempty"`
	EAB          *cert.EABConfig         `json:"eab,omitempty"` // Required for ZeroSSL and Google CA
	// Email 是本次签发使用的 ACME 账号邮箱 ✓ ——
	// ⚠ Google Public CA **必须有联系邮箱** ✗（真机报 `Accounts must have at least one contact.` ✓），
	// 而面板构造服务时邮箱是空的 ✗ → 必须由调用方（面板账号配置）传进来 ✓。
	Email string `json:"email,omitempty"`
}

// DNSProviderConfig is a type alias for the cert package type.
type DNSProviderConfig = cert.DNSProviderConfig

// DNSProvider is a type alias for the cert package type.
type DNSProvider = cert.DNSProvider

// IsPubliclyReachableIP 判断该 IP 是否可能被公信 CA 验证（导出给 Agent 侧签发复用 ✓）。
func IsPubliclyReachableIP(ip net.IP) bool { return ipUsableForPublicCA(ip) }

// DefaultProfileFor 返回该组 SAN 需要的 ACME profile：
// 只要含 **IP**（公网）就必须 `shortlived` ✓ —— LE 策略要求 IP 证书是短时证书（约 6 天），
// 不带该 profile 会被 CA 拒绝 ✗。域名返回空串（按 CA 默认 profile）。
func DefaultProfileFor(domains []string) string {
	for _, d := range domains {
		if net.ParseIP(strings.TrimSpace(d)) != nil {
			return "shortlived"
		}
	}
	return ""
}

// ipUsableForPublicCA 判断一个 IP 是否可能被公信 CA 验证。
//
// ⚠ **公信 CA 可以给公网 IP 签证书** ✓ —— Let's Encrypt 2025-07 签出首张 IP 证书、
// 2026-01-15 **IP 证书 GA** ✓（策略上必须是短时证书、约 6 天 → 见 Profile=shortlived）。
// 但**私网/保留地址永远不行** ✗：CA 必须能从公网连到这个 IP 做验证（HTTP-01/TLS-ALPN-01），
// 10.x / 172.16-31.x / 192.168.x / 127.x / 169.254.x / 100.64/10（CGNAT）/
// 192.0.2、198.51.100、203.0.113（文档保留）等都到不了 → 提前拒绝并说明原因，
// 免得一路走到 CA 那里才失败（还消耗速率配额 ✗）。
func ipUsableForPublicCA(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return false
	}
	// CGNAT 100.64.0.0/10
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
		// 文档保留段（RFC 5737）——测试时会用到，但它们不可能被 CA 验证
		if (ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 2) ||
			(ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100) ||
			(ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113) {
			return false
		}
	}
	// 文档保留段（IPv6 2001:db8::/32）
	if len(ip) == net.IPv6len && ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8 {
		return false
	}
	return true
}

// ValidateACMEDomains 校验 ACME 申请里的名字，并**提前**拒绝那些注定失败的输入。
//
// 结论（用户指正 + 核实）：
//   - **公网 IP 可以签** ✓（LE IP 证书 2026-01-15 GA，必须短时/`shortlived`）；
//   - **私网 IP 不行** ✗（没有任何公信 CA 能验证私网地址）→ 那种场景请用面板内置 CA 或改用域名；
//   - 单标签名字（`localhost`、`node1`）公信 CA 不签 ✗；
//   - 通配符必须走 DNS-01 ✗（HTTP-01 不做通配符校验）。
//
// 以前这里只检查"有没有域名" ✗ → IP 会一路走到 CA 才报一个难懂的错误（还消耗失败配额）。
func ValidateACMEDomains(domains []string, challengeType string) error {
	if len(domains) == 0 {
		return errors.New("at least one domain required")
	}
	for _, d := range domains {
		name := strings.TrimSpace(d)
		if name == "" {
			return errors.New("empty domain in request")
		}
		if ip := net.ParseIP(name); ip != nil {
			if !ipUsableForPublicCA(ip) {
				return fmt.Errorf("IP %s 无法由公信 CA 签发：它是私网/保留地址，CA 不可能从公网连到它做验证 ✗。"+
					"可行方案：① 用**面板内置 CA** 签（适合内部互信）；② 改用**公网 IP**（LE 自 2026-01-15 起支持 IP 证书，"+
					"但必须是短时证书）；③ 改用域名 + DNS-01", name)
			}
			continue // 公网 IP → 放行 ✓
		}
		host := strings.TrimPrefix(name, "*.")
		if !strings.Contains(host, ".") {
			return fmt.Errorf("ACME 不能为单标签名字（%s）签发证书：公信 CA 要求可验证的公开域名；"+
				"内部互信请改用**面板内置 CA**", name)
		}
		if strings.HasPrefix(name, "*.") && challengeType != "" && challengeType != "dns-01" {
			return fmt.Errorf("通配符证书（%s）必须使用 dns-01 挑战（HTTP-01 不支持通配符）", name)
		}
	}
	return nil
}

// IssueACME issues a new certificate via ACME.
func (s *Service) IssueACME(ctx context.Context, input IssueACMEInput) (*entity.Certificate, error) {
	// ⚠ 顺序很重要 ✓：**先**把上下文换成"不受断开影响"的 ✓（见 detachedCertOpContext ✓）——
	// 证书一旦签出来就必须落库 ✓，否则又是一份"有文件、库里没有"的孤儿 ✗。
	ctx, cancel := detachedCertOpContext(ctx)
	defer cancel()

	// 同一张证书（域名 + 密钥类型 ✓，与库里的身份口径一致 ✓）不允许两个签发并发 ✓
	//（并发签发会各自写同一份文件 + 各自落库 ✗）。
	firstDomain := ""
	if len(input.Domains) > 0 {
		firstDomain = strings.ToLower(strings.TrimSpace(input.Domains[0]))
	}
	if done, ok := s.beginCertOp("issue:" + firstDomain + ":" + cert.NormalizeKeyType(input.KeyType)); ok {
		defer done()
	} else {
		return nil, ErrCertOpInProgress
	}

	// 挑战方式**先归一化** ✓：空 = http-01（历史默认 ✓）→ 校验/下发/落库都用同一个值 ✓，
	// 免得出现"库里记空、实际走 http-01"这种对不上的状态 ✓（续期要靠它还原 ✓）。
	challengeType := NormalizeChallengeType(input.ChallengeType)

	// 提前拦掉注定失败的输入（IP / 单标签名 / 通配符+HTTP-01），并给出可行路线
	if err := ValidateACMEDomains(input.Domains, challengeType); err != nil {
		return nil, err
	}

	// For HTTP-01 challenges, check port 80 status
	if challengeType == string(cert.ChallengeHTTP01) {
		if err := s.checkPort80ForHTTP01(); err != nil {
			return nil, err
		}
	}

	caProvider := input.CAProvider
	if caProvider == "" {
		caProvider = s.caProvider
	}

	acme := s.getACMEClient(caProvider, input.Email)

	// **IP 证书必须用 shortlived profile** ✓（LE 策略：IP 证书只能是短时证书、约 6 天）。
	// 用户没显式传就自动补上；同时提示有效期很短、必须能自动续 ✓。
	profile := strings.TrimSpace(input.Profile)
	if profile == "" {
		profile = DefaultProfileFor(input.Domains)
	}
	if profile != "" {
		log.Printf("[cert] IP 证书申请 %s → 使用 profile=shortlived（LE 的 IP 证书有效期约 6 天，务必自动续期）",
			input.Domains[0])
	}

	result, err := acme.Issue(ctx, cert.IssueOptions{
		Domains:       input.Domains,
		ChallengeType: cert.ChallengeType(challengeType),
		DNSProvider:   input.DNSProvider,
		EAB:           input.EAB,
		Profile:       profile,
		// ✅ 根因修复 ✓：用户选的密钥类型（ecc/rsa + 位数）真正传下去 ✗→✓
		KeyType: cert.NormalizeKeyType(input.KeyType),
		KeyBits: input.KeyBits,
	})
	if err != nil {
		return nil, err
	}

	issuerName := string(caProvider)
	if issuerName == "" {
		issuerName = "lets_encrypt"
	}

	// ⚠ 这里原本只填了 Domain/Issuer/路径 ✗ → Name/Domains/KeyType/KeyBits 全是空 ✗
	// → 证书列表里"名字空白、域名空白、密钥类型空白" ✗（真机签发后一眼就能看到 ✗）。
	// 这些都能从**刚签出来的证书本身**解析出来 ✓（和上传路径同一套解析 ✓）。
	certRefPath := filepath.Join(s.dataDir, result.CertPath)
	keyRefPath := filepath.Join(s.dataDir, result.KeyPath)
	sansJSON := parseCertDomains(certRefPath)
	keyType, keyBits, _ := detectKeyType(keyRefPath)

	certRecord := &entity.Certificate{
		Name:         input.Domains[0],
		Domain:       input.Domains[0],
		Domains:      sansJSON,
		Issuer:       issuerName,
		ProviderType: "acme",
		KeyType:      keyType,
		KeyBits:      keyBits,
		// ✅ 把"创建时的选择"记下来 ✓：续期就按它走 ✓（前端续期不带任何参数 ✓，
		// 全靠这两个字段还原 ✓，真机 bug 的根因 ✓）。
		ChallengeType: challengeType,
		DNSChannelID:  input.DNSChannelID,
		CertPath:      result.CertPath,
		KeyPath:       result.KeyPath,
		FullchainPath: result.FullchainPath,
		CertContent:   result.CertContent,
		KeyContent:    result.KeyContent,
		Status:        "active",
		AutoRenew:     1,
		ExpiresAt:     result.ExpiresAt,
	}

	// If a certificate for the same domain already exists, update it in place
	// rather than creating a duplicate record. The file paths are the same
	// (certs/<domain>/cert.pem), so all existing config references remain valid.
	now := time.Now()
	// ✅ 身份是 (域名, 密钥类型) ✓：同域再签 RSA 时**新建**一条 ✓ 而不是改写 ECC 那条 ✗
	if existing, err := s.repo.GetByDomainAndKeyType(ctx, input.Domains[0], keyType); err == nil {
		// 续期/重签也把元数据补齐 ✓（名字为空时补上 ✓，已有的名字不覆盖 ✓）
		if strings.TrimSpace(existing.Name) == "" {
			existing.Name = input.Domains[0]
		}
		existing.Domains = sansJSON
		existing.KeyType = keyType
		existing.KeyBits = keyBits
		// 重签/补签也要把"本次的选择"更新掉 ✓（与库里的旧值不一致时以本次为准 ✓）
		existing.ChallengeType = challengeType
		existing.DNSChannelID = input.DNSChannelID
		existing.Issuer = issuerName
		existing.ProviderType = "acme"
		existing.CertPath = result.CertPath
		existing.KeyPath = result.KeyPath
		existing.FullchainPath = result.FullchainPath
		existing.CertContent = result.CertContent
		existing.KeyContent = result.KeyContent
		existing.Status = "active"
		existing.AutoRenew = 1
		existing.ExpiresAt = result.ExpiresAt
		existing.LastRenewedAt = &now
		if err := s.repo.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	certRecord.LastRenewedAt = &now
	if err := s.repo.Create(ctx, certRecord); err != nil {
		return nil, err
	}

	return certRecord, nil
}

// RenewACMEInput 描述一次续期请求 ✓。
//
// 决议口径（用户口径 ✓）：**创建时选了什么，续期就用什么** ✓。
// 所以这里每个字段都**可以留空** ✓ —— 留空 = "沿用签发时落库的选择" ✓，
// 只有显式传值才算覆盖 ✓（前端续期按钮不带任何参数 ✓ → 完全按库里的选择续 ✓）。
type RenewACMEInput struct {
	ChallengeType string
	DNSChannelID  int64
	DNSProvider   *cert.DNSProviderConfig
	// Email 是本次续期使用的 ACME 账号邮箱 ✓（留空 = 服务级默认 ✓）；
	// 走到"需要首次注册"时它是必需的 ✗（Google 会拒无联系邮箱的账号 ✓）。
	Email string
	// EAB 只在**首次注册**时用得上 ✓（Google / ZeroSSL 注册强制要求 ✓）。
	EAB *cert.EABConfig
}

// RenewACME renews a certificate via ACME.
//
// 关键口径（本次修复 ✓）：**创建时选什么，续期就用什么** ✓
//   - 挑战方式：请求显式给 ✓ > 签发时落库的 challenge_type ✓ > 兜底（含通配符 → dns-01 ✓，其余 → http-01 ✓）；
//   - DNS 凭据：请求显式给 ✓ > 落库的 dns_channel_id（**服务自己现场解析** ✓）；
//   - CA / profile / 密钥类型：按库里记录与现有私钥还原 ✓；
//   - ✗ **绝不静默降级**：dns-01 拿不到凭据就明确报错 ✓，不再偷偷换成 http-01 ✗。
func (s *Service) RenewACME(ctx context.Context, id int64, in RenewACMEInput) (*entity.Certificate, error) {
	// ⚠ 与 IssueACME 同一口径 ✓：**开跑就保证跑完并落库** ✓（客户端断开不再打断落库 ✗→✓），
	// 同时同一张证书不允许两个续期并发 ✓（否则两边都写文件、都 Update 同一行 ✗）。
	ctx, cancel := detachedCertOpContext(ctx)
	defer cancel()
	if done, ok := s.beginCertOp(fmt.Sprintf("renew:%d", id)); ok {
		defer done()
	} else {
		return nil, ErrCertOpInProgress
	}

	certRecord, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 标识符（含 SAN ✓）：必须按**同一组名字**续 ✓，主域名排第一位 ✓
	//（落盘目录以 domains[0] 为准 ✓，顺序变了就会写到别的目录去 ✗）。
	identifiers := primaryFirst(certRecord.Domain, certIdentifiersFromJSON(certRecord.Domains))

	// ① 挑战方式：显式 > 落库 > 兜底
	challengeType := ""
	switch {
	case strings.TrimSpace(in.ChallengeType) != "":
		challengeType = NormalizeChallengeType(in.ChallengeType)
	case strings.TrimSpace(certRecord.ChallengeType) != "":
		challengeType = NormalizeChallengeType(certRecord.ChallengeType)
	default:
		// 历史数据（这两个字段上线前签的证书 ✓）没有记录 ✗ → 只能按证书内容兜底 ✓：
		// 含通配符 → 只能是 dns-01 ✓；其余按历史行为 http-01 ✓。
		challengeType = string(cert.ChallengeHTTP01)
		if hasWildcardIdentifier(identifiers) {
			challengeType = string(cert.ChallengeDNS01)
		}
		log.Printf("[cert] renew %d: 库里没有挑战方式记录（历史数据）→ 按证书内容兜底为 %s", id, challengeType)
	}

	// ② DNS 凭据：显式 > 落库的通道 ID（现场解析 ✓，全程不打印凭据 ✓）
	dnsProvider := in.DNSProvider
	channelID := in.DNSChannelID
	if channelID <= 0 {
		channelID = certRecord.DNSChannelID
	}
	if dnsProvider == nil && channelID > 0 {
		if s.dnsResolver == nil {
			return nil, fmt.Errorf("DNS 通道解析器未注入，无法还原 dns-01 凭据（dns_channel_id=%d）", channelID)
		}
		cfg, derr := s.dnsResolver.ResolveDNSChannel(ctx, channelID)
		if derr != nil {
			return nil, fmt.Errorf("解析 DNS 通道 %d 失败: %w", channelID, derr)
		}
		dnsProvider = cfg
	}

	// ③ ✗ 绝不静默降级：创建时选了 dns-01，续期就必须走 dns-01 ✓
	if challengeType == string(cert.ChallengeDNS01) && dnsProvider == nil {
		return nil, fmt.Errorf("这张证书是按 dns-01 签发/记录的，但拿不到 DNS 通道凭据（dns_channel_id=%d）—— "+
			"请在续期时指定 dns_channel_id，或先补上对应 DNS 通道；为避免偷偷换成 http-01，本次续期已终止", channelID)
	}

	// CA 与创建时一致 ✓（issuer 里存的就是当时的 CA ✓）
	caProvider := cert.CAProvider(certRecord.Issuer)
	acme := s.getACMEClient(caProvider, in.Email)

	result, err := acme.Renew(ctx, cert.RenewOptions{
		Domains: identifiers,
		// ⚠ 库里存的是**相对路径** ✗ → 必须拼上 dataDir ✓ ——
		// 否则能不能读到文件取决于进程 CWD ✓（面板恰好总在 /opt/corefusion 跑 ✓，属于侥幸 ✓）。
		CertPath:      filepath.Join(s.dataDir, certRecord.CertPath),
		KeyPath:       filepath.Join(s.dataDir, certRecord.KeyPath),
		ChallengeType: cert.ChallengeType(challengeType),
		// profile 按标识符重算 ✓（IP 证书 = shortlived ✓，与创建时同一套规则 ✓）
		Profile:     DefaultProfileFor(identifiers),
		EAB:         in.EAB,
		DNSProvider: dnsProvider,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	certRecord.CertPath = result.CertPath
	certRecord.KeyPath = result.KeyPath
	certRecord.FullchainPath = result.FullchainPath
	certRecord.CertContent = result.CertContent
	certRecord.KeyContent = result.KeyContent
	certRecord.ExpiresAt = result.ExpiresAt
	certRecord.LastRenewedAt = &now
	// 把**本次实际使用的选择**写回去 ✓：下次续期继续沿用 ✓，顺带把历史数据补全 ✓。
	certRecord.ChallengeType = challengeType
	if channelID > 0 {
		certRecord.DNSChannelID = channelID
	}

	if err := s.repo.Update(ctx, certRecord); err != nil {
		return nil, err
	}

	log.Printf("[cert] renewed cert %d (%s): ca=%s challenge=%s dns_channel=%d identifiers=%v",
		certRecord.ID, certRecord.Domain, caProvider, challengeType, certRecord.DNSChannelID, identifiers)

	return certRecord, nil
}

// NormalizeChallengeType 归一化挑战方式 ✓：`dns-01` 认几种写法 ✓，
// 其余（含空 ✓）一律按 `http-01` ✓ —— 与"挑战方式为空时默认 http-01"的历史行为一致 ✓。
func NormalizeChallengeType(ch string) string {
	switch strings.ToLower(strings.TrimSpace(ch)) {
	case "dns-01", "dns01", "dns":
		return string(cert.ChallengeDNS01)
	default:
		return string(cert.ChallengeHTTP01)
	}
}

// certIdentifiersFromJSON 把库里 `certificates.domains`（标准 JSON 数组 ✓）解析成标识符列表 ✓。
// 解析不出来（空串 / `[]` / 脏数据 ✓）→ 返回空列表 ✓，由调用方退回主域名 ✓。
// ⚠ 与 `certSANIdentifiers(cert *x509.Certificate)`（从证书里解析 ✓）是两回事，别混用 ✗。
func certIdentifiersFromJSON(domains string) []string {
	if strings.TrimSpace(domains) == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(domains), &arr); err != nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, d := range arr {
		if d = strings.TrimSpace(d); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// primaryFirst 把主域名放到第一位 ✓（去重、忽略空值 ✓）。
//
// ⚠ 顺序为什么重要 ✗：续期落盘路径以 `domains[0]` 为准 ✓（`certs/<domains[0]>/…` ✓），
// 顺序变了就会写到另一个目录 ✗ → 原路径上的证书不会被更新 ✗（等于"续了但没生效" ✓）。
func primaryFirst(primary string, identifiers []string) []string {
	primary = strings.TrimSpace(primary)
	out := make([]string, 0, len(identifiers)+1)
	seen := make(map[string]bool, len(identifiers)+1)
	if primary != "" {
		out = append(out, primary)
		seen[primary] = true
	}
	for _, id := range identifiers {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		out = append(out, id)
		seen[id] = true
	}
	return out
}

// hasWildcardIdentifier 判断标识符里是否含通配符 ✓（含 → 只可能走 dns-01 ✓）。
func hasWildcardIdentifier(identifiers []string) bool {
	for _, id := range identifiers {
		if strings.HasPrefix(strings.TrimSpace(id), "*.") {
			return true
		}
	}
	return false
}

// DetectPublicIP detects the public IPv4 and IPv6 addresses of the server.
func (s *Service) DetectPublicIP() (ipv4, ipv6 string, err error) {
	client := &http.Client{Timeout: 8 * time.Second}

	// Detect IPv4
	req4, _ := http.NewRequest("GET", "http://ip.sb", nil)
	req4.Header.Set("User-Agent", "curl/7.0")
	resp4, err := client.Do(req4)
	if err == nil {
		defer resp4.Body.Close()
		body, _ := io.ReadAll(resp4.Body)
		ipv4 = strings.TrimSpace(string(body))
	}

	// Detect IPv6
	client6 := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	req6, _ := http.NewRequest("GET", "http://ip.sb", nil)
	req6.Header.Set("User-Agent", "curl/7.0")
	resp6, err := client6.Do(req6)
	if err == nil {
		defer resp6.Body.Close()
		body, _ := io.ReadAll(resp6.Body)
		ipv6 = strings.TrimSpace(string(body))
	}

	if ipv4 == "" && ipv6 == "" {
		return "", "", fmt.Errorf("failed to detect public IP")
	}

	return ipv4, ipv6, nil
}

// CheckPortInUse checks if a port is in use and returns the process info.
func (s *Service) CheckPortInUse(port string) (bool, string) {
	// Try to listen on the port
	addr := ":" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Port is in use, try to find the process
		return true, findProcessByPort(port)
	}
	ln.Close()
	return false, ""
}

// findProcessByPort finds the process using a specific port.
func findProcessByPort(port string) string {
	// Use lsof to find the process
	cmd := exec.Command("lsof", "-i", ":"+port, "-t")
	out, err := cmd.Output()
	if err != nil {
		return "unknown process"
	}
	pid := strings.TrimSpace(string(out))
	if pid == "" {
		return "unknown process"
	}
	// Get process name
	cmd2 := exec.Command("ps", "-p", pid, "-o", "comm=")
	out2, err := cmd2.Output()
	if err != nil {
		return "PID " + pid
	}
	return strings.TrimSpace(string(out2)) + " (PID " + pid + ")"
}

// SyncCertsToDisk writes all cert content from DB to disk files.
// Called on startup to ensure disk files match DB state.
func (s *Service) SyncCertsToDisk(ctx context.Context) error {
	certs, _, err := s.repo.List(ctx, 1000, 0)
	if err != nil {
		return err
	}
	count := 0
	for _, c := range certs {
		if c.CertContent == "" || c.KeyContent == "" {
			continue
		}
		certPath := c.CertPath
		keyPath := c.KeyPath
		// Resolve relative paths
		if !filepath.IsAbs(certPath) {
			certPath = filepath.Join(s.dataDir, certPath)
		}
		if !filepath.IsAbs(keyPath) {
			keyPath = filepath.Join(s.dataDir, keyPath)
		}
		// Create directory
		os.MkdirAll(filepath.Dir(certPath), 0750)
		// Write files
		if err := os.WriteFile(certPath, []byte(c.CertContent), 0600); err != nil {
			log.Printf("Sync cert %s: %v", c.Domain, err)
			continue
		}
		if err := os.WriteFile(keyPath, []byte(c.KeyContent), 0600); err != nil {
			log.Printf("Sync key %s: %v", c.Domain, err)
			continue
		}
		count++
	}
	if count > 0 {
		log.Printf("Synced %d certificates from database to disk", count)
	}
	return nil
}
