package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	certpkg "corefusion/internal/runtime/cert"
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
	nodeCerts  *repository.NodeCertificateRepository
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

// WithNodeCertificates 注入"节点 ↔ 证书"多对多仓库 ✓：
// 一个节点可以绑定多张证书，一张证书也可以被多个节点使用，
// 只按 node_servers.certificate_id 单值分发会漏掉其余绑定（多证书场景）。
func (d *CertDistributor) WithNodeCertificates(repo *repository.NodeCertificateRepository) *CertDistributor {
	d.nodeCerts = repo
	return d
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

	// ⚠ 面板不持有 PEM 的证书**不能分发** ✗（节点上报的证书按当前口径面板只留元数据 ✓）。
	// 少了这道守卫会把**空内容**推给节点 ✓ → 节点上正在用的证书文件被覆盖成空 ✗ → TLS 直接断 ✗。
	if strings.TrimSpace(cert.CertContent) == "" || strings.TrimSpace(cert.KeyContent) == "" {
		return fmt.Errorf("证书 %d 的面板副本没有 PEM（按当前口径由节点持有 ✓），无法分发：请在该节点上取件，或改用面板签发/上传的证书", certID)
	}

	// Determine save path on Agent side — use the same relative path structure
	// as the certificates table so xray config references resolve correctly.
	// The Agent's dataDir is /opt/corefusion-agent/data, and xray configs use
	// relative paths like "../../../certs/gg/cert.pem" (relative to data/runtime/).
	//
	// ⚠ 必须写**全部路径别名** ✗：规范目录按域名（certs/<domain>）✓，
	// 但历史证书的站点配置引用的是名称目录（name=gg / domain=example.com →
	// 配置写 certs/gg/cert.pem ✓）。只写域名目录会表现为
	// "签发/分发都成功，站点却一直用旧证书" ✗（真机 2026-10-20 踩到 ✓）。
	variants := certpkg.CertPathVariants(cert.Name, cert.Domain, cert.KeyType, cert.CertPath)
	if len(variants) == 0 {
		return fmt.Errorf("certificate %d has no valid target directory", certID)
	}

	// Fullchain content: use cert content as fallback (fullchain is usually
	// the same as cert for most setups; if separate fullchain content exists
	// in DB, it would be stored in a fullchain_content field).
	fullchainContent := cert.CertContent

	var errs []error
	for _, p := range variants {
		cmd := &agentpb.DistributeCertCommand{
			CertName:     cert.Domain,
			CertPem:      []byte(cert.CertContent),
			KeyPem:       []byte(cert.KeyContent),
			FullchainPem: []byte(fullchainContent),
			TargetPath:   p.Dir,
			ExpiresAt:    cert.ExpiresAt.Unix(),
			ReloadCore:   true, // Notify core to reload after cert replacement
		}
		if err := d.nodeMgr.DistributeCertificate(ctx, serverID, cmd); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Dir, err))
			continue
		}
		log.Printf("[CertDistributor] cert %s (id=%d) distributed to server %d → %s",
			cert.Domain, certID, serverID, p.Dir)
	}
	if len(errs) > 0 {
		return fmt.Errorf("distribute to server %d: %v", serverID, errs)
	}

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

	// 目标节点 = 多对多表里的绑定 ∪ 历史单值字段（node_servers.certificate_id）。
	// 只看单值字段会漏掉"一个节点绑定多张证书"里除主证书外的那些。
	targetSet := make(map[int64]bool)
	if d.nodeCerts != nil {
		if ids, err := d.nodeCerts.ListNodeIDsByCertificate(ctx, certID); err == nil {
			for _, id := range ids {
				targetSet[id] = true
			}
		} else {
			log.Printf("[CertDistributor] list nodes for cert %d: %v", certID, err)
		}
	}
	var targets []int64
	for _, s := range servers {
		if s.CertificateID == certID {
			targetSet[s.ID] = true
		}
		if targetSet[s.ID] && !s.IsLocal {
			targets = append(targets, s.ID)
			targetSet[s.ID] = false // 去重：多对多与单值字段可能同时命中
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

	// Check if Agent cert also needs renewal (expired while offline)
	if server.AgentCertExpiresAt != nil && time.Now().After(*server.AgentCertExpiresAt) {
		log.Printf("[CertDistributor] server %d agent cert expired while offline, needs re-bootstrap", serverID)
		return
	}

	// 重连补发要覆盖**全部绑定证书**（多对多 ✓），不只是历史单值字段那张。
	certIDs := make([]int64, 0, 2)
	seen := map[int64]bool{}
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			certIDs = append(certIDs, id)
		}
	}
	if d.nodeCerts != nil {
		if ids, err := d.nodeCerts.ListCertificateIDsByNode(ctx, serverID); err == nil {
			for _, id := range ids {
				add(id)
			}
		} else {
			log.Printf("[CertDistributor] reconnect: list certs for server %d: %v", serverID, err)
		}
	}
	add(server.CertificateID)
	if len(certIDs) == 0 {
		return // No certificate associated
	}

	log.Printf("[CertDistributor] server %d reconnected, distributing cert(s) %v", serverID, certIDs)
	for _, certID := range certIDs {
		if err := d.DistributeToServer(ctx, serverID, certID); err != nil {
			log.Printf("[CertDistributor] reconnect distribution failed for server %d cert %d: %v", serverID, certID, err)
		}
	}
}
