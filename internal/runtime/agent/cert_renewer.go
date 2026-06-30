package agent

import (
	"context"
	"log"
	"time"

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

// NewCertRenewer creates a new CertRenewer with the default 24h interval.
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

	// Run once shortly after startup
	time.AfterFunc(1*time.Minute, func() {
		if err := r.checkAndRenew(ctx); err != nil {
			log.Printf("[CertRenewer] initial check error: %v", err)
		}
	})

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
		if server.IsLocal {
			continue // Local server doesn't need Agent certs
		}

		log.Printf("[CertRenewer] server %d (%s) cert expiring at %v, renewing...",
			server.ID, server.Hostname, server.AgentCertExpiresAt)

		// Issue new certificate
		certResult, err := r.ca.IssueAgentCertificate(ctx, server.ID, server.Hostname)
		if err != nil {
			log.Printf("[CertRenewer] failed to renew cert for server %d: %v", server.ID, err)
			continue
		}

		// Push new cert to Agent via gRPC stream
		if err := r.nodeMgr.PushAgentCertificate(ctx, server.ID, certResult); err != nil {
			log.Printf("[CertRenewer] failed to push new cert to server %d: %v (will retry on reconnect)",
				server.ID, err)
			// Non-fatal: Agent will fetch new cert on next reconnect
		}

		// Update database
		if err := r.serverRepo.UpdateAgentCert(ctx, server.ID, certResult.Serial, certResult.ExpiresAt); err != nil {
			log.Printf("[CertRenewer] failed to update cert record for server %d: %v", server.ID, err)
			continue
		}

		log.Printf("[CertRenewer] server %d cert renewed, new expiry %v", server.ID, certResult.ExpiresAt)
	}

	return nil
}
