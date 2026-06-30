package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	// AgentCertValidity defines the validity period for Agent mTLS certificates.
	AgentCertValidity = 10 * 24 * time.Hour // 10 天有效期

	// AgentCertRenewBefore defines how long before expiry to start renewal.
	AgentCertRenewBefore = 3 * 24 * time.Hour // 到期前 3 天开始续签

	// AgentCertRenewInterval defines the renewal check frequency.
	AgentCertRenewInterval = 24 * time.Hour // 每天检查一次

	// CAValidity defines the CA root certificate validity period.
	CAValidity = 10 * 365 * 24 * time.Hour // 10 年
)

// AgentCertResult holds the result of issuing an Agent mTLS certificate.
type AgentCertResult struct {
	Serial    string    `json:"serial"`
	CertPEM   []byte    `json:"cert_pem"`
	KeyPEM    []byte    `json:"key_pem"`
	CACertPEM []byte    `json:"ca_cert_pem"` // CA 根证书（Agent 需要用于验证面板）
	ExpiresAt time.Time `json:"expires_at"`
}

// CertificateAuthority is the panel's internal CA that signs Agent mTLS certificates.
type CertificateAuthority struct {
	caCert    *x509.Certificate
	caKey     *ecdsa.PrivateKey
	caCertPEM []byte
	caKeyPEM  []byte
	dataDir   string
}

// InitCA initializes or loads the CA root certificate.
// On first run it self-signs a CA root certificate (valid for 10 years).
func InitCA(dataDir string) (*CertificateAuthority, error) {
	caCertPath := filepath.Join(dataDir, "certs", "mtls", "ca-cert.pem")
	caKeyPath := filepath.Join(dataDir, "certs", "mtls", "ca-key.pem")

	// Try loading existing CA
	if certPEM, err := os.ReadFile(caCertPath); err == nil {
		if keyPEM, err := os.ReadFile(caKeyPath); err == nil {
			ca, err := loadCA(certPEM, keyPEM, dataDir)
			if err != nil {
				return nil, fmt.Errorf("load existing CA: %w", err)
			}
			log.Printf("[CA] loaded existing CA root certificate (expires %v)", ca.caCert.NotAfter)
			return ca, nil
		}
	}

	// First time: self-sign a CA root certificate
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"CoreFusion"},
			CommonName:   "CoreFusion Agent CA",
		},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().Add(CAValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})

	keyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return nil, fmt.Errorf("marshal CA key: %w", err)
	}
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	// Save to disk
	if err := os.MkdirAll(filepath.Dir(caCertPath), 0750); err != nil {
		return nil, fmt.Errorf("create CA dir: %w", err)
	}
	if err := os.WriteFile(caCertPath, caCertPEM, 0640); err != nil {
		return nil, fmt.Errorf("write CA cert: %w", err)
	}
	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		return nil, fmt.Errorf("write CA key: %w", err)
	}

	log.Printf("[CA] self-signed CA root certificate created (expires %v)", template.NotAfter)

	return &CertificateAuthority{
		caCert:    caCert,
		caKey:     caKey,
		caCertPEM: caCertPEM,
		caKeyPEM:  caKeyPEM,
		dataDir:   dataDir,
	}, nil
}

// loadCA parses existing PEM-encoded CA certificate and key.
func loadCA(certPEM, keyPEM []byte, dataDir string) (*CertificateAuthority, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode CA key PEM")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA key: %w", err)
	}

	return &CertificateAuthority{
		caCert:    caCert,
		caKey:     caKey,
		caCertPEM: certPEM,
		caKeyPEM:  keyPEM,
		dataDir:   dataDir,
	}, nil
}

// IssueAgentCertificate signs an mTLS client certificate for an Agent.
func (ca *CertificateAuthority) IssueAgentCertificate(ctx context.Context, serverID int64, hostname string) (*AgentCertResult, error) {
	// Generate Agent-specific key pair
	agentKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate agent key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}
	expiresAt := time.Now().Add(AgentCertValidity)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{"CoreFusion"},
			OrganizationalUnit: []string{fmt.Sprintf("server-%d", serverID)},
			CommonName:         hostname,
		},
		NotBefore:   time.Now().Add(-5 * time.Minute), // allow 5 min clock skew
		NotAfter:    expiresAt,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{hostname},
		IPAddresses: []net.IP{},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, &agentKey.PublicKey, ca.caKey)
	if err != nil {
		return nil, fmt.Errorf("create agent cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(agentKey)
	if err != nil {
		return nil, fmt.Errorf("marshal agent key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	log.Printf("[CA] agent certificate issued for server-%d (%s), serial=%s, expires=%v",
		serverID, hostname, serialNumber.String(), expiresAt)

	return &AgentCertResult{
		Serial:    serialNumber.String(),
		CertPEM:   certPEM,
		KeyPEM:    keyPEM,
		CACertPEM: ca.caCertPEM,
		ExpiresAt: expiresAt,
	}, nil
}

// GetCACertPEM returns the CA root certificate PEM (Agents need this to verify the panel).
func (ca *CertificateAuthority) GetCACertPEM() []byte {
	return ca.caCertPEM
}

// GetCACert returns the parsed CA certificate.
func (ca *CertificateAuthority) GetCACert() *x509.Certificate {
	return ca.caCert
}

// GetCAKey returns the CA private key (used for CRL signing).
func (ca *CertificateAuthority) GetCAKey() *ecdsa.PrivateKey {
	return ca.caKey
}
