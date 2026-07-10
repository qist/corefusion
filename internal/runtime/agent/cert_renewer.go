package agent

import (
	"context"
	"log"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// CertRenewer periodically checks and auto-renews Agent certificates nearing expiry.
type CertRenewer struct {
	ca         *CertificateAuthority
	serverRepo *repository.NodeServerRepository
	nodeMgr    NodeCertPusher // interface for pushing certs to agents via gRPC
	interval   time.Duration
}

// NodeCertPusher defines the interface for pushing new certificates to Agents.
// Implemented by NodeServerManager (to be added in a later phase).
type NodeCertPusher interface {
	// PushAgentCertificate pushes a renewed certificate to the Agent on the given server.
	// If the Agent is offline, the error is non-fatal — the Agent will fetch the new cert on reconnect.
	PushAgentCertificate(ctx context.Context, serverID int64, cert *AgentCertResult) error
}

// noopCertPusher is a no-op implementation used before NodeServerManager is built.
type noopCertPusher struct{}

func (noopCertPusher) PushAgentCertificate(ctx context.Context, serverID int64, cert *AgentCertResult) error {
	log.Printf("[CertRenewer] (noop) cert pushed for server %d would happen here", serverID)
	return nil
}

// NewCertRenewer creates a new CertRenewer with the default interval.
func NewCertRenewer(ca *CertificateAuthority, serverRepo *repository.NodeServerRepository) *CertRenewer {
	return &CertRenewer{
		ca:         ca,
		serverRepo: serverRepo,
		nodeMgr:    noopCertPusher{},
		interval:   AgentCertRenewInterval,
	}
}

// WithPusher sets the certificate pusher (NodeServerManager).
func (r *CertRenewer) WithPusher(pusher NodeCertPusher) *CertRenewer {
	r.nodeMgr = pusher
	return r
}

// WithInterval sets the renewal check interval.
func (r *CertRenewer) WithInterval(d time.Duration) *CertRenewer {
	r.interval = d
	return r
}

// RunLoop starts the renewal loop. Blocks until ctx is cancelled.
func (r *CertRenewer) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on startup (no delay) so certs that expired while the
	// panel was down are renewed right away.
	if err := r.checkAndRenew(ctx); err != nil {
		log.Printf("[CertRenewer] initial check error: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("[CertRenewer] stopped")
			return
		case <-ticker.C:
			if err := r.checkAndRenew(ctx); err != nil {
				log.Printf("[CertRenewer] check error: %v", err)
			}
		}
	}
}

// checkAndRenew finds all servers with expiring certs and renews them.
func (r *CertRenewer) checkAndRenew(ctx context.Context) error {
	// Find servers whose certs expire within AgentCertRenewBefore (3 days)
	threshold := time.Now().Add(AgentCertRenewBefore)
	servers, err := r.serverRepo.ListCertExpiringSoon(ctx, threshold)
	if err != nil {
		return err
	}

	if len(servers) == 0 {
		return nil
	}

	log.Printf("[CertRenewer] found %d server(s) with certs expiring before %v", len(servers), threshold)

	for _, server := range servers {
		r.renewServerCert(ctx, &server)
	}

	return nil
}

// RenewServerCertIfNeeded checks a single server's cert and renews it if it's
// expiring soon. This is called when an Agent reconnects to ensure it always
// has a fresh certificate, even if it was offline during the periodic check.
func (r *CertRenewer) RenewServerCertIfNeeded(ctx context.Context, serverID int64) {
	server, err := r.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		log.Printf("[CertRenewer] failed to get server %d for on-reconnect renewal: %v", serverID, err)
		return
	}

	if server.IsLocal {
		return // Local server doesn't need Agent certs
	}

	if server.AgentCertExpiresAt == nil {
		return // No cert to renew
	}

	// Check if the cert is within the renewal window
	threshold := time.Now().Add(AgentCertRenewBefore)
	if server.AgentCertExpiresAt.After(threshold) {
		return // Cert is still fresh, no need to renew
	}

	log.Printf("[CertRenewer] server %d (%s) cert expiring at %v, renewing on reconnect...",
		server.ID, server.Hostname, server.AgentCertExpiresAt)

	r.renewServerCert(ctx, server)
}

// renewServerCert issues a new cert for the given server, pushes it to the
// Agent, and updates the database ONLY if push succeeded.
//
// If the Agent is offline or push fails, the DB is NOT updated — the cert
// will be re-issued and re-pushed on the next check cycle or on Agent reconnect.
// This prevents the scenario where the DB shows a renewed cert but the Agent
// is still using the old (possibly expired) cert.
func (r *CertRenewer) renewServerCert(ctx context.Context, server *entity.NodeServer) {
	if server.IsLocal {
		return
	}

	log.Printf("[CertRenewer] server %d (%s) cert expiring at %v, renewing...",
		server.ID, server.Hostname, server.AgentCertExpiresAt)

	// Issue new certificate
	certResult, err := r.ca.IssueAgentCertificate(ctx, server.ID, server.Hostname)
	if err != nil {
		log.Printf("[CertRenewer] failed to renew cert for server %d: %v", server.ID, err)
		return
	}

	// Push new cert to Agent via gRPC stream.
	// Only update DB if push succeeds — otherwise the Agent still has the old
	// cert and we need to retry on the next cycle.
	if err := r.nodeMgr.PushAgentCertificate(ctx, server.ID, certResult); err != nil {
		log.Printf("[CertRenewer] failed to push new cert to server %d: %v (will retry on next check)",
			server.ID, err)
		return
	}

	// Push succeeded — now safe to update DB
	if err := r.serverRepo.UpdateAgentCert(ctx, server.ID, certResult.Serial, certResult.ExpiresAt); err != nil {
		log.Printf("[CertRenewer] failed to update cert record for server %d: %v", server.ID, err)
		return
	}

	log.Printf("[CertRenewer] server %d cert renewed and pushed, new expiry %v", server.ID, certResult.ExpiresAt)
}
