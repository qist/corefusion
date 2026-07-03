package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// CertDistributor manages the distribution of external service TLS certificates
// (from the certificates table) to remote Agent nodes.
type CertDistributor struct {
	certRepo   *repository.CertificateRepository
	serverRepo *repository.NodeServerRepository
	nodeMgr    *NodeServerManager
}

// NewCertDistributor creates a new CertDistributor.
func NewCertDistributor(
	certRepo *repository.CertificateRepository,
	serverRepo *repository.NodeServerRepository,
	nodeMgr *NodeServerManager,
) *CertDistributor {
	return &CertDistributor{
		certRepo:   certRepo,
		serverRepo: serverRepo,
		nodeMgr:    nodeMgr,
	}
}

// DistributeToServer distributes a certificate to a specific server.
func (d *CertDistributor) DistributeToServer(ctx context.Context, serverID int64, certID int64) error {
	server, err := d.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Local server uses files directly, no distribution needed
	if server.IsLocal {
		log.Printf("[CertDistributor] server %d is local, skipping distribution", serverID)
		return nil
	}

	// Check if Agent is online
	if !d.nodeMgr.IsOnline(serverID) {
		log.Printf("[CertDistributor] server %d is offline, cert will be distributed on reconnect", serverID)
		return fmt.Errorf("server %d offline, will retry on reconnect", serverID)
	}

	cert, err := d.certRepo.GetByID(ctx, certID)
	if err != nil {
		return fmt.Errorf("certificate not found: %w", err)
	}

	// Determine save path on Agent side — use the same relative path structure
	// as the certificates table so xray config references resolve correctly.
	// The Agent's dataDir is /opt/corefusion-agent/data, and xray configs use
	// relative paths like "../../../certs/gg/cert.pem" (relative to data/runtime/).
	// So we write to dataDir/certs/<domain>/cert.pem etc.
	certDir := fmt.Sprintf("certs/%s", cert.Domain)

	// Fullchain content: use cert content as fallback (fullchain is usually
	// the same as cert for most setups; if separate fullchain content exists
	// in DB, it would be stored in a fullchain_content field).
	fullchainContent := cert.CertContent

	cmd := &agentpb.DistributeCertCommand{
		CertName:     cert.Domain,
		CertPem:      []byte(cert.CertContent),
		KeyPem:       []byte(cert.KeyContent),
		FullchainPem: []byte(fullchainContent),
		TargetPath:   certDir,
		ExpiresAt:    cert.ExpiresAt.Unix(),
		ReloadCore:   true, // Notify core to reload after cert replacement
	}

	if err := d.nodeMgr.DistributeCertificate(ctx, serverID, cmd); err != nil {
		return fmt.Errorf("distribute to server %d: %w", serverID, err)
	}

	log.Printf("[CertDistributor] cert %s (id=%d) distributed to server %d",
		cert.Domain, certID, serverID)

	return nil
}

// DistributeToAllServers distributes a certificate to all servers that have
// the given certificate_id associated.
func (d *CertDistributor) DistributeToAllServers(ctx context.Context, certID int64) error {
	// Find all servers associated with this certificate
	servers, err := d.serverRepo.ListAll(ctx)
	if err != nil {
		return fmt.Errorf("list servers: %w", err)
	}

	var targets []int64
	for _, s := range servers {
		if s.CertificateID == certID && !s.IsLocal {
			targets = append(targets, s.ID)
		}
	}

	if len(targets) == 0 {
		log.Printf("[CertDistributor] no remote servers associated with cert %d", certID)
		return nil
	}

	log.Printf("[CertDistributor] distributing cert %d to %d server(s)", certID, len(targets))

	var wg sync.WaitGroup
	errCh := make(chan error, len(targets))

	for _, sid := range targets {
		wg.Add(1)
		go func(serverID int64) {
			defer wg.Done()
			if err := d.DistributeToServer(ctx, serverID, certID); err != nil {
				errCh <- fmt.Errorf("server %d: %w", serverID, err)
			}
		}(sid)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		log.Printf("[CertDistributor] distribution completed with %d error(s) out of %d server(s)",
			len(errs), len(targets))
		for _, e := range errs {
			log.Printf("[CertDistributor] error: %v", e)
		}
		return fmt.Errorf("distribution failed on %d/%d servers", len(errs), len(targets))
	}

	log.Printf("[CertDistributor] cert %d distributed to all %d server(s) successfully", certID, len(targets))
	return nil
}

// OnCertificateRenewed is called when an ACME certificate is renewed.
// It re-distributes the renewed certificate to all associated servers.
// This should be hooked into the existing certificate renewal flow.
func (d *CertDistributor) OnCertificateRenewed(ctx context.Context, certID int64) {
	log.Printf("[CertDistributor] certificate %d renewed, re-distributing to servers...", certID)

	// Small delay to ensure DB is updated with new cert content
	time.AfterFunc(2*time.Second, func() {
		bgCtx := context.Background()
		if err := d.DistributeToAllServers(bgCtx, certID); err != nil {
			log.Printf("[CertDistributor] re-distribution failed: %v", err)
		}
	})
}

// DistributeOnReconnect distributes the associated certificate when an Agent reconnects.
// Called by the NodeServerManager when a server comes back online.
func (d *CertDistributor) DistributeOnReconnect(ctx context.Context, serverID int64) {
	server, err := d.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		log.Printf("[CertDistributor] reconnect distribution: server %d not found: %v", serverID, err)
		return
	}

	if server.CertificateID == 0 {
		return // No certificate associated
	}

	// Check if Agent cert also needs renewal (expired while offline)
	if server.AgentCertExpiresAt != nil && time.Now().After(*server.AgentCertExpiresAt) {
		log.Printf("[CertDistributor] server %d agent cert expired while offline, needs re-bootstrap", serverID)
		return
	}

	log.Printf("[CertDistributor] server %d reconnected, distributing cert %d", serverID, server.CertificateID)
	if err := d.DistributeToServer(ctx, serverID, server.CertificateID); err != nil {
		log.Printf("[CertDistributor] reconnect distribution failed for server %d: %v", serverID, err)
	}
}
