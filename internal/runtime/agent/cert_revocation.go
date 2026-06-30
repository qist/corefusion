package agent

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"sync"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// CertRevocationList manages revoked Agent certificates and provides
// CRL (Certificate Revocation List) functionality for mTLS verification.
type CertRevocationList struct {
	mu             sync.RWMutex
	cachedCRL      *x509.RevocationList
	lastRefresh    time.Time
	revocationRepo *repository.CertRevocationRepository
	serverRepo     *repository.NodeServerRepository
	nodeMgr        *NodeServerManager
	ca             *CertificateAuthority
}

// NewCertRevocationList creates a new CertRevocationList.
func NewCertRevocationList(
	revocationRepo *repository.CertRevocationRepository,
	serverRepo *repository.NodeServerRepository,
	nodeMgr *NodeServerManager,
	ca *CertificateAuthority,
) *CertRevocationList {
	return &CertRevocationList{
		revocationRepo: revocationRepo,
		serverRepo:     serverRepo,
		nodeMgr:        nodeMgr,
		ca:             ca,
	}
}

// RevokeCertificate revokes the current certificate for a server.
// This disconnects the Agent and clears its cert info.
func (crl *CertRevocationList) RevokeCertificate(ctx context.Context, serverID int64, adminID int64, reason string) error {
	server, err := crl.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	if server.AgentCertSerial == "" {
		return fmt.Errorf("server %d has no active certificate", serverID)
	}

	// Record revocation
	revocation := &entity.AgentCertRevocation{
		ServerID:    serverID,
		CertSerial:  server.AgentCertSerial,
		RevokedAt:   time.Now(),
		RevokedBy:   adminID,
		Reason:      reason,
		EffectiveAt: time.Now(),
	}
	if err := crl.revocationRepo.Create(ctx, revocation); err != nil {
		return fmt.Errorf("create revocation: %w", err)
	}

	// Disconnect the Agent
	if crl.nodeMgr != nil {
		crl.nodeMgr.DisconnectServer(ctx, serverID)
	}

	// Clear server cert info
	server.AgentCertSerial = ""
	server.AgentCertExpiresAt = nil
	server.AgentCertLastRenewedAt = nil
	server.AgentStatus = "revoked"
	if err := crl.serverRepo.Update(ctx, server); err != nil {
		log.Printf("[CRL] failed to update server %d status: %v", serverID, err)
	}

	// Refresh CRL
	crl.refreshCRL(ctx)

	log.Printf("[CRL] certificate revoked for server %d (serial=%s, reason=%s)",
		serverID, server.AgentCertSerial, reason)

	return nil
}

// IsRevoked checks if a certificate serial number has been revoked.
// Called during mTLS handshake verification.
func (crl *CertRevocationList) IsRevoked(serial string) bool {
	crl.mu.RLock()
	defer crl.mu.RUnlock()

	if crl.cachedCRL == nil {
		return false
	}

	for _, entry := range crl.cachedCRL.RevokedCertificates {
		if entry.SerialNumber.String() == serial {
			return true
		}
	}
	return false
}

// refreshCRL rebuilds the CRL from database records and signs it with the CA key.
func (crl *CertRevocationList) refreshCRL(ctx context.Context) {
	revocations, err := crl.revocationRepo.ListActive(ctx)
	if err != nil {
		log.Printf("[CRL] failed to list revocations: %v", err)
		return
	}

	template := &x509.RevocationList{
		Number:     big.NewInt(time.Now().Unix()),
		ThisUpdate: time.Now(),
		NextUpdate: time.Now().Add(24 * time.Hour),
	}

	for _, rev := range revocations {
		serial, err := strconv.ParseInt(rev.CertSerial, 10, 64)
		if err != nil {
			// Try parsing as big int
			bigSerial, ok := new(big.Int).SetString(rev.CertSerial, 10)
			if !ok {
				log.Printf("[CRL] skipping invalid serial: %s", rev.CertSerial)
				continue
			}
			template.RevokedCertificates = append(template.RevokedCertificates,
				pkix.RevokedCertificate{
					SerialNumber:   bigSerial,
					RevocationTime: rev.RevokedAt,
				})
			continue
		}
		template.RevokedCertificates = append(template.RevokedCertificates,
			pkix.RevokedCertificate{
				SerialNumber:   big.NewInt(serial),
				RevocationTime: rev.RevokedAt,
			})
	}

	crlDER, err := x509.CreateRevocationList(rand.Reader, template, crl.ca.GetCACert(), crl.ca.GetCAKey())
	if err != nil {
		log.Printf("[CRL] failed to create CRL: %v", err)
		return
	}

	parsedCRL, err := x509.ParseRevocationList(crlDER)
	if err != nil {
		log.Printf("[CRL] failed to parse CRL: %v", err)
		return
	}

	crl.mu.Lock()
	crl.cachedCRL = parsedCRL
	crl.lastRefresh = time.Now()
	crl.mu.Unlock()

	log.Printf("[CRL] refreshed, %d revoked certificate(s)", len(revocations))
}

// RunRefreshLoop periodically refreshes the CRL (every 1 hour).
func (crl *CertRevocationList) RunRefreshLoop(ctx context.Context) {
	// Initial refresh
	crl.refreshCRL(ctx)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[CRL] refresh loop stopped")
			return
		case <-ticker.C:
			crl.refreshCRL(ctx)
		}
	}
}

// VerifyCallback returns a function suitable for use as tls.Config.VerifyPeerCertificate
// to check if the connecting Agent's certificate has been revoked.
func (crl *CertRevocationList) VerifyCallback() func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
			return fmt.Errorf("no verified certificate chain")
		}

		cert := verifiedChains[0][0]

		if crl.IsRevoked(cert.SerialNumber.String()) {
			return fmt.Errorf("certificate %s has been revoked", cert.SerialNumber)
		}

		return nil
	}
}
