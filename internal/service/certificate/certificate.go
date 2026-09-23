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
	certDir := filepath.Join(s.dataDir, "certs", sanitizeDomain(name))
	os.MkdirAll(certDir, 0750)
	// Store relative paths for portability
	certPath := filepath.Join("certs", sanitizeDomain(name), "cert.pem")
	keyPath := filepath.Join("certs", sanitizeDomain(name), "key.pem")
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
	if existing, err := s.repo.GetByDomain(ctx, domain); err == nil {
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

// panelCertListCap 是"面板列表一次最多捞多少行" ✓ —— 证书表量级很小 ✓（几十~几百 ✓），
// 为的是能**在内存里过滤掉 IP 证书后仍然给出准确的分页与总数** ✓。
const panelCertListCap = 2000

// ListForPanel 是**面板证书列表接口**专用的列表 ✓：默认**不展示 IP 证书** ✗。
//
// 为什么要这样 ✗（用户口径 ✓）："IP 证书不需要同步到面板 [的证书清单]" ✓ ——
// IP 证书是节点自己的事 ✓，摆在清单里会让人误以为"面板在管它" ✗。
// ⚠ 但**数据库里照样留着** ✓：入库是刻意的 ✓（有人把证书文件删了，库里还完整有一份 ✓），
// 续期器也仍然能按它去触发节点续期 ✓。所以这里是**视图层过滤** ✗，不是数据层删除 ✓。
//
// 为什么在内存里过滤 ✗：SQLite 里没法"判断某个域名是不是 IP" ✗；证书表很小 ✓，
// 过滤后**重算 total** ✓ → 分页是准的 ✓（不会出现"总数 16、翻页却少两行"✗）。
// includeIP=true 时返回全部 ✓（保留给排查/运维用 ✓）。
func (s *Service) ListForPanel(ctx context.Context, limit, offset int, includeIP bool) ([]entity.Certificate, int64, error) {
	if includeIP {
		return s.repo.List(ctx, limit, offset)
	}
	all, _, err := s.repo.List(ctx, panelCertListCap, 0)
	if err != nil {
		return nil, 0, err
	}
	visible := make([]entity.Certificate, 0, len(all))
	hidden := 0
	for i := range all {
		if isIPOnlyCertificate(all[i].Domain, all[i].Domains, nil) {
			hidden++
			continue
		}
		visible = append(visible, all[i])
	}
	if hidden > 0 {
		// 只打一次聚合日志 ✓（别每条证书刷一行 ✗）
		log.Printf("[cert] 面板列表隐藏了 %d 张 IP 证书（节点本地证书，库里仍保留 ✓）", hidden)
	}
	total := int64(len(visible))
	if offset < 0 {
		offset = 0
	}
	if offset >= len(visible) {
		return []entity.Certificate{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(visible) {
		end = len(visible)
	}
	return visible[offset:end], total, nil
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
	Domains       []string                `json:"domains" binding:"required"`
	ChallengeType string                  `json:"challenge_type"`    // "http-01" or "dns-01"
	CAProvider    cert.CAProvider         `json:"ca_provider"`       // "lets_encrypt" / "lets_encrypt_staging" / "zerossl" / "google"
	Profile       string                  `json:"profile,omitempty"` // IP 证书需 "shortlived"（会自动补）
	DNSProvider   *cert.DNSProviderConfig `json:"dns_provider,omitempty"`
	EAB           *cert.EABConfig         `json:"eab,omitempty"` // Required for ZeroSSL and Google CA
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
	// 提前拦掉注定失败的输入（IP / 单标签名 / 通配符+HTTP-01），并给出可行路线
	if err := ValidateACMEDomains(input.Domains, input.ChallengeType); err != nil {
		return nil, err
	}

	// For HTTP-01 challenges, check port 80 status
	if input.ChallengeType == "" || input.ChallengeType == string(cert.ChallengeHTTP01) {
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
		ChallengeType: cert.ChallengeType(input.ChallengeType),
		DNSProvider:   input.DNSProvider,
		EAB:           input.EAB,
		Profile:       profile,
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
		Name:          input.Domains[0],
		Domain:        input.Domains[0],
		Domains:       sansJSON,
		Issuer:        issuerName,
		ProviderType:  "acme",
		KeyType:       keyType,
		KeyBits:       keyBits,
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
	if existing, err := s.repo.GetByDomain(ctx, input.Domains[0]); err == nil {
		// 续期/重签也把元数据补齐 ✓（名字为空时补上 ✓，已有的名字不覆盖 ✓）
		if strings.TrimSpace(existing.Name) == "" {
			existing.Name = input.Domains[0]
		}
		existing.Domains = sansJSON
		existing.KeyType = keyType
		existing.KeyBits = keyBits
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

// RenewACME renews a certificate via ACME.
func (s *Service) RenewACME(ctx context.Context, id int64, dnsProvider *cert.DNSProviderConfig) (*entity.Certificate, error) {
	certRecord, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use the same CA provider as the original issuance
	caProvider := cert.CAProvider(certRecord.Issuer)
	acme := s.getACMEClient(caProvider)

	domains := []string{certRecord.Domain}
	result, err := acme.Renew(ctx, domains, certRecord.CertPath, certRecord.KeyPath, dnsProvider)
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

	if err := s.repo.Update(ctx, certRecord); err != nil {
		return nil, err
	}

	return certRecord, nil
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
