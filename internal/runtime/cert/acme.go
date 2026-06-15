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
	"os"
	"path/filepath"
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
	CAProviderZeroSSL     CAProvider = "zerossl"
	CAProviderGoogle      CAProvider = "google"
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
	dataDir       string
	email         string
	caProvider    CAProvider
	caURL         string
	accountDir    string
	http01Provider *HTTP01Provider // Custom HTTP-01 provider for Gin integration
}

// NewACMEClient creates a new ACME client.
func NewACMEClient(dataDir, email string, caProvider CAProvider) *ACMEClient {
	caURL := CALetsEncrypt
	switch caProvider {
	case CAProviderZeroSSL:
		caURL = CAZeroSSL
	case CAProviderGoogle:
		caURL = CAGooglePublicCA
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

// IssueResult contains the result of a certificate issuance.
type IssueResult struct {
	CertPath      string
	KeyPath       string
	FullchainPath string
	CertContent   string
	KeyContent    string
	ExpiresAt     time.Time
	AccountKeyPath string
}

// DNSProviderConfig holds DNS provider credentials.
type DNSProviderConfig struct {
	Provider DNSProvider       `json:"provider"`
	EnvVars  map[string]string `json:"env_vars"`
}

// IssueOptions contains options for certificate issuance.
type IssueOptions struct {
	Domains       []string          `json:"domains"`
	ChallengeType ChallengeType     `json:"challenge_type"`
	DNSProvider   *DNSProviderConfig `json:"dns_provider,omitempty"`
	EAB           *EABConfig        `json:"eab,omitempty"` // Required for ZeroSSL and Google CA
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
	client, err := c.createClient(user)
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

	client, err := c.createClient(user)
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

// createClient creates a lego client.
func (c *ACMEClient) createClient(user *ACMEUser) (*lego.Client, error) {
	config := lego.NewConfig(user)
	config.CADirURL = c.caURL
	config.Certificate.KeyType = certcrypto.EC256

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
		// Use custom HTTP-01 provider that integrates with Gin router if available
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
		return client.Challenge.SetDNS01Provider(provider,
			dns01.CondOption(true, dns01.CondOption(true, dns01.CondOption(true, nil))))

	default:
		return fmt.Errorf("unsupported challenge type: %s", challengeType)
	}
}

// createDNSProvider creates a DNS provider based on config.
func (c *ACMEClient) createDNSProvider(config *DNSProviderConfig) (challenge.Provider, error) {
	// Set environment variables for the provider
	for k, v := range config.EnvVars {
		os.Setenv(k, v)
	}

	switch config.Provider {
	case DNSProviderCloudflare:
		cfg := cloudflare.NewDefaultConfig()
		return cloudflare.NewDNSProviderConfig(cfg)
	case DNSProviderAliDNS:
		cfg := alidns.NewDefaultConfig()
		return alidns.NewDNSProviderConfig(cfg)
	case DNSProviderDNSPod:
		cfg := dnspod.NewDefaultConfig()
		return dnspod.NewDNSProviderConfig(cfg)
	case DNSProviderRoute53:
		cfg := route53.NewDefaultConfig()
		return route53.NewDNSProviderConfig(cfg)
	case DNSProviderGoogle:
		cfg := gcloud.NewDefaultConfig()
		return gcloud.NewDNSProviderConfig(cfg)
	case DNSProviderGoDaddy:
		cfg := godaddy.NewDefaultConfig()
		return godaddy.NewDNSProviderConfig(cfg)
	case DNSProviderNamecheap:
		cfg := namecheap.NewDefaultConfig()
		return namecheap.NewDNSProviderConfig(cfg)
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
		Email:   user.Email,
		CAURL:   c.caURL,
		KeyPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyPEM})),
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

	if err := os.WriteFile(certPath, certResource.Certificate, 0640); err != nil {
		return nil, fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, certResource.PrivateKey, 0600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}
	if err := os.WriteFile(fullchainPath, certResource.Certificate, 0640); err != nil {
		return nil, fmt.Errorf("write fullchain: %w", err)
	}

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
	Email            string `json:"email"`
	CAURL            string `json:"ca_url"`
	KeyPEM           string `json:"key_pem"`
	RegistrationURI  string `json:"registration_uri"`
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
func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey         { return u.key }

// GetSupportedProviders returns a list of supported DNS providers with their required env vars.
func GetSupportedProviders() map[DNSProvider][]string {
	return map[DNSProvider][]string{
		DNSProviderCloudflare: {"CLOUDFLARE_EMAIL", "CLOUDFLARE_API_KEY"},
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
