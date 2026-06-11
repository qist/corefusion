package certificate

import (
	"context"
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
	repo       *repository.CertificateRepository
	dataDir    string
	acmeEmail  string
	caProvider cert.CAProvider
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

// getACMEClient returns an ACME client for the given CA provider.
func (s *Service) getACMEClient(caProvider cert.CAProvider) *cert.ACMEClient {
	if caProvider == "" {
		caProvider = s.caProvider
	}
	return cert.NewACMEClient(s.dataDir, s.acmeEmail, caProvider)
}

// CreateCertificateInput is the input for creating a certificate.
type CreateCertificateInput struct {
	Domain       string `json:"domain" binding:"required"`
	Issuer       string `json:"issuer" binding:"required"`
	ProviderType string `json:"provider_type" binding:"required"`
	CertPath     string `json:"cert_path" binding:"required"`
	KeyPath      string `json:"key_path" binding:"required"`
	FullchainPath string `json:"fullchain_path"`
	AutoRenew    bool   `json:"auto_renew"`
	ExpiresAt    time.Time `json:"expires_at" binding:"required"`
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
	Name      string `json:"name"`           // User-friendly name
	Domain    string `json:"domain"`         // Primary domain (auto-detected from cert if empty)
	CertContent  string `json:"cert_content"`
	KeyContent   string `json:"key_content"`
}

// Upload creates a certificate from pasted content or file paths.
func (s *Service) Upload(ctx context.Context, input UploadCertificateInput) (*entity.Certificate, error) {
	if input.CertContent == "" || input.KeyContent == "" {
		return nil, errors.New("cert and key content are required")
	}

	// Save cert content to temp file first to parse it
	tmpDir := filepath.Join(s.dataDir, "certs_tmp")
	os.MkdirAll(tmpDir, 0750)
	tmpCert := filepath.Join(tmpDir, "tmp_cert.pem")
	tmpKey := filepath.Join(tmpDir, "tmp_key.pem")
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
		Name:          name,
		Domain:        domain,
		Domains:       sans,
		Issuer:        issuer,
		ProviderType:  "upload",
		KeyType:       keyType,
		KeyBits:       keyBits,
		CertPath:      certPath,
		KeyPath:       keyPath,
		CertContent:   input.CertContent,
		KeyContent:    input.KeyContent,
		Status:        "active",
		AutoRenew:     0,
		ExpiresAt:     expiresAt,
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
	if len(x509Cert.DNSNames) > 0 {
		return x509Cert.DNSNames
	}
	return []string{x509Cert.Subject.CommonName}
}

// sanitizeDomain converts a domain to a safe directory name.
func sanitizeDomain(domain string) string {
	return strings.ReplaceAll(strings.ReplaceAll(domain, "*", "_wildcard"), ".", "_")
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

	domains := x509Cert.DNSNames
	if len(domains) == 0 {
		// Fall back to CN
		domains = []string{x509Cert.Subject.CommonName}
	}

	// Return as JSON array
	parts := make([]string, len(domains))
	for i, d := range domains {
		parts[i] = `"` + d + `"`
	}
	return "[" + strings.Join(parts, ",") + "]"
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

// IssueACMEInput is the input for ACME certificate issuance.
type IssueACMEInput struct {
	Domains       []string                  `json:"domains" binding:"required"`
	ChallengeType string                    `json:"challenge_type"` // "http-01" or "dns-01"
	CAProvider    cert.CAProvider            `json:"ca_provider"`    // "lets_encrypt", "zerossl", "google"
	DNSProvider   *cert.DNSProviderConfig    `json:"dns_provider,omitempty"`
	EAB           *cert.EABConfig           `json:"eab,omitempty"` // Required for ZeroSSL and Google CA
}

// DNSProviderConfig is a type alias for the cert package type.
type DNSProviderConfig = cert.DNSProviderConfig

// DNSProvider is a type alias for the cert package type.
type DNSProvider = cert.DNSProvider

// IssueACME issues a new certificate via ACME.
func (s *Service) IssueACME(ctx context.Context, input IssueACMEInput) (*entity.Certificate, error) {
	if len(input.Domains) == 0 {
		return nil, errors.New("at least one domain required")
	}

	caProvider := input.CAProvider
	if caProvider == "" {
		caProvider = s.caProvider
	}

	acme := s.getACMEClient(caProvider)

	result, err := acme.Issue(ctx, cert.IssueOptions{
		Domains:       input.Domains,
		ChallengeType: cert.ChallengeType(input.ChallengeType),
		DNSProvider:   input.DNSProvider,
		EAB:           input.EAB,
	})
	if err != nil {
		return nil, err
	}

	issuerName := string(caProvider)
	if issuerName == "" {
		issuerName = "lets_encrypt"
	}

	certRecord := &entity.Certificate{
		Domain:       input.Domains[0],
		Issuer:       issuerName,
		ProviderType: "acme",
		CertPath:     result.CertPath,
		KeyPath:      result.KeyPath,
		FullchainPath: result.FullchainPath,
		CertContent:  result.CertContent,
		KeyContent:   result.KeyContent,
		Status:       "active",
		AutoRenew:    1,
		ExpiresAt:    result.ExpiresAt,
	}

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
