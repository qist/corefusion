package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

const (
	// AgentCertValidity defines the validity period for Agent mTLS certificates.
	AgentCertValidity = 10 * 24 * time.Hour // 10 天有效期

	// AgentCertRenewBefore defines how long before expiry to start renewal.
	// 3 天前开始续签，确保证书在过期前有充足时间完成推送。
	AgentCertRenewBefore = 3 * 24 * time.Hour // 到期前 3 天开始续签

	// AgentCertRenewInterval defines the renewal check frequency.
	// 每 1 小时检查一次，确保及时续签并重试推送失败的证书。
	AgentCertRenewInterval = 1 * time.Hour // 每 1 小时检查一次

	// CAValidity defines the CA root certificate validity period.
	CAValidity = 10 * 365 * 24 * time.Hour // 10 年

	// CAOverlapWindow defines how long before the current CA expires a new CA
	// root is generated. During this window both the old (outgoing) and new
	// (incoming) CA roots are valid and trusted, so Agents keep working while
	// their short-lived (AgentCertValidity) certificates naturally rotate onto
	// the new CA. The old CA is dropped from the trust pool only once it expires.
	CAOverlapWindow = 90 * 24 * time.Hour // 提前 90 天开始生成新 CA
)

// AgentCertResult holds the result of issuing an Agent mTLS certificate.
type AgentCertResult struct {
	Serial    string    `json:"serial"`
	CertPEM   []byte    `json:"cert_pem"`
	KeyPEM    []byte    `json:"key_pem"`
	CACertPEM []byte    `json:"ca_cert_pem"` // CA 根证书 bundle（Agent 需要用于验证面板，含所有有效 CA）
	ExpiresAt time.Time `json:"expires_at"`
}

// caEntry is a single loaded CA root (cert + decrypted key) held in memory.
type caEntry struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
	keyPEM  []byte
}

// CertificateAuthority manages the panel's internal CA roots used to sign
// Agent mTLS certificates. It may hold multiple concurrent CA roots during a
// rotation: the outgoing (old) CA and the incoming (new) CA are both trusted so
// existing Agents keep working while they re-enroll onto the new CA.
//
// The CA private keys are stored encrypted in the database (ca_roots table,
// via crypto.EncryptSecret); the plaintext PEM files under dataDir/certs/mtls/
// are only materialized for Agent bootstrap downloads and are never the source
// of truth.
type CertificateAuthority struct {
	caDir string
	// entries holds all currently valid CA roots, ordered by NotAfter ascending.
	entries []caEntry
}

// EnsureCA loads all valid CA roots from the database, generating and persisting
// a new CA root when none exist, or when the current CA is within CAOverlapWindow
// of expiry and no incoming CA already covers that window.
func EnsureCA(dataDir string, caRepo *repository.CARootRepository) (*CertificateAuthority, error) {
	ctx := context.Background()
	ca := &CertificateAuthority{caDir: filepath.Join(dataDir, "certs", "mtls")}

	rows, err := caRepo.ListValid(ctx)
	if err != nil {
		return nil, fmt.Errorf("list CA roots: %w", err)
	}

	for i := range rows {
		entry, err := loadEntryFromDB(&rows[i])
		if err != nil {
			return nil, fmt.Errorf("load CA root %d: %w", rows[i].ID, err)
		}
		ca.entries = append(ca.entries, *entry)
	}
	ca.sortEntries()

	// First run: no CA at all -> generate the initial root.
	if len(ca.entries) == 0 {
		if _, err := ca.generateAndPersist(ctx, caRepo); err != nil {
			return nil, fmt.Errorf("create initial CA: %w", err)
		}
	} else if ca.needsNewCA() {
		// A new CA is needed only if no existing CA already extends past the
		// overlap window of the soonest-expiring CA.
		threshold := time.Now().Add(CAOverlapWindow)
		has, err := caRepo.HasValidWithin(ctx, threshold)
		if err != nil {
			return nil, fmt.Errorf("check incoming CA: %w", err)
		}
		if !has {
			if _, err := ca.generateAndPersist(ctx, caRepo); err != nil {
				return nil, fmt.Errorf("rotate CA: %w", err)
			}
		}
	}

	// Re-materialize the plaintext CA bundle for Agent bootstrap downloads.
	if err := ca.writePEMToDisk(); err != nil {
		return nil, fmt.Errorf("materialize CA PEM: %w", err)
	}

	if len(ca.entries) == 1 {
		log.Printf("[CA] loaded CA root certificate (expires %v)", ca.entries[0].cert.NotAfter)
	} else {
		exps := make([]string, 0, len(ca.entries))
		for _, e := range ca.entries {
			exps = append(exps, e.cert.NotAfter.Format("2006-01-02"))
		}
		log.Printf("[CA] loaded %d CA roots (expiry: %v) — rotation in progress", len(ca.entries), exps)
	}

	return ca, nil
}

// needsNewCA reports whether the soonest-expiring CA is within CAOverlapWindow
// of expiry (i.e. it is time to start rotating).
func (ca *CertificateAuthority) needsNewCA() bool {
	if len(ca.entries) == 0 {
		return true
	}
	soonest := ca.entries[0].cert.NotAfter
	return time.Until(soonest) <= CAOverlapWindow
}

// generateAndPersist self-signs a new CA root, encrypts its key, persists it to
// the database, and loads it into memory.
func (ca *CertificateAuthority) generateAndPersist(ctx context.Context, caRepo *repository.CARootRepository) (*caEntry, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
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

	encKey, err := crypto.EncryptSecret(string(caKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("encrypt CA key: %w", err)
	}

	if err := caRepo.Create(ctx, &entity.CARoot{
		CommonName: template.Subject.CommonName,
		CertPEM:    string(caCertPEM),
		KeyPEMEnc:  encKey,
		ExpiresAt:  template.NotAfter,
	}); err != nil {
		return nil, fmt.Errorf("persist CA to DB: %w", err)
	}

	entry := &caEntry{cert: caCert, key: caKey, certPEM: caCertPEM, keyPEM: caKeyPEM}
	ca.entries = append(ca.entries, *entry)
	ca.sortEntries()

	if len(ca.entries) == 1 {
		log.Printf("[CA] self-signed initial CA root (expires %v)", template.NotAfter)
	} else {
		log.Printf("[CA] rotated: new CA root created and stored encrypted (expires %v)", template.NotAfter)
	}
	return entry, nil
}

// sortEntries orders CA roots by NotAfter ascending so entries[0] is the
// soonest-expiring (outgoing) CA and the last entry is the newest (incoming) CA.
func (ca *CertificateAuthority) sortEntries() {
	sort.Slice(ca.entries, func(i, j int) bool {
		return ca.entries[i].cert.NotAfter.Before(ca.entries[j].cert.NotAfter)
	})
}

// latest returns the newest CA root (used to sign new certificates).
func (ca *CertificateAuthority) latest() *caEntry {
	if len(ca.entries) == 0 {
		return nil
	}
	return &ca.entries[len(ca.entries)-1]
}

// loadEntryFromDB decrypts a single CA root row and parses it.
func loadEntryFromDB(row *entity.CARoot) (*caEntry, error) {
	certPEM := []byte(row.CertPEM)

	keyPEM, err := crypto.DecryptSecret(row.KeyPEMEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt CA key: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode CA key PEM")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA key: %w", err)
	}

	return &caEntry{cert: caCert, key: caKey, certPEM: certPEM, keyPEM: []byte(keyPEM)}, nil
}

// writePEMToDisk materializes the CA certificate bundle (all valid CA certs
// concatenated) and the latest CA private key as plaintext PEM files under
// dataDir/certs/mtls/. Agents download the bundle during bootstrap to verify the
// panel; the private key file is never the source of truth (the encrypted DB is)
// and is written with restrictive permissions.
func (ca *CertificateAuthority) writePEMToDisk() error {
	if err := os.MkdirAll(ca.caDir, 0750); err != nil {
		return fmt.Errorf("create CA dir: %w", err)
	}

	// ca-cert.pem holds the full CA bundle (all valid roots) for Agent bootstrap.
	bundle := ca.GetCABundlePEM()
	if err := os.WriteFile(filepath.Join(ca.caDir, "ca-cert.pem"), bundle, 0640); err != nil {
		return fmt.Errorf("write CA bundle: %w", err)
	}

	// ca-key.pem holds the latest CA private key (used by the panel process only;
	// on rotation the newest key is the one that signs new certificates).
	latest := ca.latest()
	if latest != nil {
		if err := os.WriteFile(filepath.Join(ca.caDir, "ca-key.pem"), latest.keyPEM, 0600); err != nil {
			return fmt.Errorf("write CA key: %w", err)
		}
	}
	return nil
}

// GetCABundlePEM returns all valid CA root certificates concatenated as PEM.
// Agents use this bundle as their RootCAs pool so they trust both the outgoing
// and incoming CA during a rotation.
func (ca *CertificateAuthority) GetCABundlePEM() []byte {
	var bundle []byte
	for _, e := range ca.entries {
		bundle = append(bundle, e.certPEM...)
	}
	return bundle
}

// ClientCAPool returns an x509.CertPool containing all valid CA roots, used by
// the panel gRPC server to verify incoming Agent client certificates. Both the
// outgoing and incoming CA are trusted during a rotation.
func (ca *CertificateAuthority) ClientCAPool() *x509.CertPool {
	pool := x509.NewCertPool()
	for _, e := range ca.entries {
		pool.AddCert(e.cert)
	}
	return pool
}

// GetCACertPEM returns the CA root certificate bundle PEM (Agents need this to
// verify the panel).
func (ca *CertificateAuthority) GetCACertPEM() []byte {
	return ca.GetCABundlePEM()
}

// CAStatus describes a single CA root for observability/ops.
type CAStatus struct {
	Subject     string    `json:"subject"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	ExpiresIn   string    `json:"expires_in"`   // human-readable remaining validity
	IsOutgoing  bool      `json:"is_outgoing"`  // soonest-expiring (being replaced) during rotation
	IsIncoming  bool      `json:"is_incoming"`  // newest CA (signs new certificates)
	InOverlap   bool      `json:"in_overlap"`   // within CAOverlapWindow of expiry
	SerialNumber string   `json:"serial_number"`
}

// ListCAStatus returns the status of all loaded (valid) CA roots, soonest-
// expiring first. Useful for ops to observe rotation state.
func (ca *CertificateAuthority) ListCAStatus() []CAStatus {
	out := make([]CAStatus, 0, len(ca.entries))
	for i, e := range ca.entries {
		remaining := time.Until(e.cert.NotAfter)
		st := CAStatus{
			Subject:     e.cert.Subject.CommonName,
			NotBefore:   e.cert.NotBefore,
			NotAfter:    e.cert.NotAfter,
			ExpiresIn:   remaining.Round(time.Hour).String(),
			IsOutgoing:  i == 0 && len(ca.entries) > 1,
			IsIncoming:  i == len(ca.entries)-1,
			InOverlap:   remaining <= CAOverlapWindow,
			SerialNumber: e.cert.SerialNumber.String(),
		}
		out = append(out, st)
	}
	return out
}

// GetCACert returns the newest CA certificate.
func (ca *CertificateAuthority) GetCACert() *x509.Certificate {
	latest := ca.latest()
	if latest == nil {
		return nil
	}
	return latest.cert
}

// GetCAKey returns the newest CA private key (used for CRL signing and issuing
// new certificates).
func (ca *CertificateAuthority) GetCAKey() *ecdsa.PrivateKey {
	latest := ca.latest()
	if latest == nil {
		return nil
	}
	return latest.key
}

// GetCAKeyForCA returns the private key matching a specific CA certificate
// (selected by SKI/AKI) so the correct key signs a CRL issued by that CA.
func (ca *CertificateAuthority) GetCAKeyForCA(caCert *x509.Certificate) *ecdsa.PrivateKey {
	for _, e := range ca.entries {
		if e.cert.Equal(caCert) {
			return e.key
		}
	}
	return ca.latest().key
}

// IssueAgentCertificate signs an mTLS client certificate for an Agent using the
// newest (incoming) CA root. The returned bundle contains all valid CA certs.
func (ca *CertificateAuthority) IssueAgentCertificate(ctx context.Context, serverID int64, hostname string) (*AgentCertResult, error) {
	latest := ca.latest()
	if latest == nil {
		return nil, fmt.Errorf("no CA root available")
	}

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

	certDER, err := x509.CreateCertificate(rand.Reader, template, latest.cert, &agentKey.PublicKey, latest.key)
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
		CACertPEM: ca.GetCABundlePEM(),
		ExpiresAt: expiresAt,
	}, nil
}

// IssueServerCertificate signs a TLS server certificate for the Panel's gRPC
// server using the newest CA root. This certificate is presented to Agents
// during the mTLS handshake; Agents trust it because their CA bundle includes
// the newest CA.
func (ca *CertificateAuthority) IssueServerCertificate() (tls.Certificate, error) {
	latest := ca.latest()
	if latest == nil {
		return tls.Certificate{}, fmt.Errorf("no CA root available")
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate server key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial number: %w", err)
	}

	expiresAt := time.Now().Add(CAValidity / 2) // 5 years

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"CoreFusion"},
			CommonName:   "corefusion-panel",
		},
		NotBefore:   time.Now().Add(-5 * time.Minute),
		NotAfter:    expiresAt,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{"corefusion-panel", "localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, latest.cert, &serverKey.PublicKey, latest.key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create server cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal server key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	serverCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load server key pair: %w", err)
	}

	log.Printf("[CA] panel gRPC server certificate issued (serial=%s, expires=%v)", serialNumber.String(), expiresAt)
	return serverCert, nil
}
