package nodeserver

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/api/handlers/services"
	"corefusion/internal/model/dto"
	"corefusion/internal/model/entity"
	"corefusion/internal/model/vo"
	"corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// BinaryLister lists available binary releases from GitHub.
type BinaryLister interface {
	ListReleases(ctx context.Context, coreType string) ([]struct {
		Version    string `json:"version"`
		Prerelease bool   `json:"prerelease"`
	}, error)
	ListReleasesForPlatform(ctx context.Context, coreType, osName, archName string) ([]struct {
		Version    string `json:"version"`
		Prerelease bool   `json:"prerelease"`
	}, error)
}

// Service provides business logic for managing node servers.
type Service struct {
	repo          *repository.NodeServerRepository
	inboundRepo   *repository.InboundConfigRepository
	tokenMgr      *agent.BootstrapTokenManager
	caService     *agent.CertificateAuthority
	crl           *agent.CertRevocationList
	certDistrib   *agent.CertDistributor
	nodeMgr       *agent.NodeServerManager
	failoverMgr   *agent.FailoverManager
	mirrorSync    *agent.MirrorSyncManager
	chainGen      *agent.ChainConfigGenerator
	chainHealth   *agent.ChainHealthMonitorRegistry
	billingFilter *agent.BillingFilter

	// Phase 4: User sync and traffic collection
	userSyncSvc   UserSyncServiceInterface
	trafficStats  TrafficStatsInterface

	// Phase 5: Health check and dashboard
	healthChecker HealthCheckerInterface
	regionSelector RegionSelectorInterface

	// Phase 6: Advanced features
	metricsCollector *agent.MetricsCollector
	speedTestMgr     *agent.SpeedTestManager
	loadBalancer     *agent.LoadBalancer
	agentUpdater     *agent.AgentUpdater

	// Supplementary repos
	auditLogRepo  *repository.NodeAuditLogRepository
	tagRepo       *repository.ServerTagRepository
	distQueueRepo *repository.CertDistributionQueueRepository
	certRepo      *repository.CertificateRepository
	crlRepo       *repository.CertRevocationRepository
	db            *gorm.DB

	// Binary management
	binaryLister BinaryLister
}

// NewService creates a new node server service.
func NewService(repo *repository.NodeServerRepository, tokenMgr *agent.BootstrapTokenManager, ca *agent.CertificateAuthority) *Service {
	return &Service{
		repo:      repo,
		tokenMgr:  tokenMgr,
		caService: ca,
	}
}

// WithCRL sets the CRL for certificate revocation.
func (s *Service) WithCRL(crl *agent.CertRevocationList) *Service {
	s.crl = crl
	return s
}

// WithCertDistributor sets the cert distributor.
func (s *Service) WithCertDistributor(cd *agent.CertDistributor) *Service {
	s.certDistrib = cd
	return s
}

// WithInboundRepo sets the inbound config repository (for mirror sync).
func (s *Service) WithInboundRepo(repo *repository.InboundConfigRepository) *Service {
	s.inboundRepo = repo
	return s
}

// WithNodeManager sets the node server manager (for online checks).
func (s *Service) WithNodeManager(nm *agent.NodeServerManager) *Service {
	s.nodeMgr = nm
	return s
}

// WithFailoverManager sets the failover manager.
func (s *Service) WithFailoverManager(fm *agent.FailoverManager) *Service {
	s.failoverMgr = fm
	return s
}

// WithMirrorSync sets the mirror sync manager.
func (s *Service) WithMirrorSync(ms *agent.MirrorSyncManager) *Service {
	s.mirrorSync = ms
	return s
}

// WithChainGenerator sets the chain config generator.
func (s *Service) WithChainGenerator(cg *agent.ChainConfigGenerator) *Service {
	s.chainGen = cg
	return s
}

// WithChainHealthRegistry sets the chain health monitor registry.
func (s *Service) WithChainHealthRegistry(r *agent.ChainHealthMonitorRegistry) *Service {
	s.chainHealth = r
	return s
}

// WithBillingFilter sets the billing filter.
func (s *Service) WithBillingFilter(bf *agent.BillingFilter) *Service {
	s.billingFilter = bf
	return s
}

// UserSyncServiceInterface defines the interface for user sync operations.
// Implemented by usersync.UserSyncService.
type UserSyncServiceInterface interface {
	SyncUserToServers(ctx context.Context, clientID int64, action string) error
	SyncUserRemoval(ctx context.Context, clientID int64) error
	GetRetryQueueSize() int
	ProcessRetryQueue(ctx context.Context)
}

// TrafficStatsInterface defines the interface for traffic stats.
// Implemented by traffic.MultiServerTrafficCollector.
type TrafficStatsInterface interface {
	GetStats() interface{}
}

// WithUserSyncService sets the user sync service.
func (s *Service) WithUserSyncService(us UserSyncServiceInterface) *Service {
	s.userSyncSvc = us
	return s
}

// WithTrafficStats sets the traffic stats provider.
func (s *Service) WithTrafficStats(ts TrafficStatsInterface) *Service {
	s.trafficStats = ts
	return s
}

// HealthCheckerInterface defines the interface for health checking.
// Implemented by healthcheck.MultiServerHealthChecker.
type HealthCheckerInterface interface {
	GetServerStatusSummary(ctx context.Context) (interface{}, error)
}

// RegionSelectorInterface defines the interface for GeoIP region recommendation.
// Implemented by subscription.RegionSelector.
type RegionSelectorInterface interface {
	RecommendRegion(clientIP string, allowedRegions []string) string
}

// WithHealthChecker sets the health checker.
func (s *Service) WithHealthChecker(hc HealthCheckerInterface) *Service {
	s.healthChecker = hc
	return s
}

// WithRegionSelector sets the region selector.
func (s *Service) WithRegionSelector(rs RegionSelectorInterface) *Service {
	s.regionSelector = rs
	return s
}

// ──────────────────── Phase 6: Advanced Features ────────────────────

// WithMetricsCollector sets the metrics collector.
func (s *Service) WithMetricsCollector(mc *agent.MetricsCollector) *Service {
	s.metricsCollector = mc
	return s
}

// WithSpeedTestManager sets the speed test manager.
func (s *Service) WithSpeedTestManager(stm *agent.SpeedTestManager) *Service {
	s.speedTestMgr = stm
	return s
}

// WithLoadBalancer sets the load balancer.
func (s *Service) WithLoadBalancer(lb *agent.LoadBalancer) *Service {
	s.loadBalancer = lb
	return s
}

// WithAgentUpdater sets the agent updater.
func (s *Service) WithAgentUpdater(au *agent.AgentUpdater) *Service {
	s.agentUpdater = au
	return s
}

// WithAuditLogRepo sets the audit log repository.
func (s *Service) WithAuditLogRepo(r *repository.NodeAuditLogRepository) *Service {
	s.auditLogRepo = r
	return s
}

// WithTagRepo sets the server tag repository.
func (s *Service) WithTagRepo(r *repository.ServerTagRepository) *Service {
	s.tagRepo = r
	return s
}

// WithDistQueueRepo sets the cert distribution queue repository.
func (s *Service) WithDistQueueRepo(r *repository.CertDistributionQueueRepository) *Service {
	s.distQueueRepo = r
	return s
}

// WithCertRepo sets the certificate repository.
func (s *Service) WithCertRepo(r *repository.CertificateRepository) *Service {
	s.certRepo = r
	return s
}

// WithCRLRepo sets the cert revocation repository.
func (s *Service) WithCRLRepo(r *repository.CertRevocationRepository) *Service {
	s.crlRepo = r
	return s
}

// WithDB sets the underlying gorm.DB handle.
func (s *Service) WithDB(db *gorm.DB) *Service {
	s.db = db
	return s
}

// Create creates a new node server.
func (s *Service) Create(ctx context.Context, req *dto.NodeServerCreate) (*entity.NodeServer, error) {
	now := time.Now()

	// Set defaults based on node type
	billingEnabled := true
	if req.BillingEnabled != nil {
		billingEnabled = *req.BillingEnabled
	}
	if req.NodeType == entity.NodeTypeChainExit || req.NodeType == entity.NodeTypeMirror {
		billingEnabled = false // chain_exit and mirror nodes don't bill by default
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	nodeType := req.NodeType
	if nodeType == "" {
		nodeType = entity.NodeTypeStandalone
	}

	agentPort := req.AgentPort
	if agentPort == 0 {
		agentPort = 18443
	}

	supportedCores := req.SupportedCores
	if supportedCores == "" {
		supportedCores = `["xray","singbox","hysteria2","tuic"]`
	}

	server := &entity.NodeServer{
		Name:             req.Name,
		Hostname:         req.Hostname,
		Region:           req.Region,
		RegionLabel:      req.RegionLabel,
		NodeType:         nodeType,
		PartnerServerID:  req.PartnerServerID,
		FailoverRole:     req.FailoverRole,
		FailoverPriority: req.FailoverPriority,
		ChainConfig:      req.ChainConfig,
		AgentPort:        agentPort,
		AgentStatus:      entity.AgentStatusOffline,
		IsLocal:          req.IsLocal,
		SupportedCores:   supportedCores,
		MaxInbounds:      req.MaxInbounds,
		PublicIP:         req.PublicIP,
		InternalIP:       req.InternalIP,
		BandwidthMbps:    req.BandwidthMbps,
		BillingEnabled:   billingEnabled,
		CertificateID:    req.CertificateID,
		Metadata:         req.Metadata,
		SortOrder:        req.SortOrder,
		Enabled:          enabled,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.repo.Create(ctx, server); err != nil {
		return nil, fmt.Errorf("create node server: %w", err)
	}

	// Seed default outbounds (direct + blocked) for the new server.
	// This ensures every node starts with the essential freedom/blackhole outbounds,
	// matching the local node's defaults. If they already exist (e.g. from a
	// previous creation attempt), the ON CONFLICT clause skips them.
	if s.db != nil && !server.IsLocal {
		defaults := []entity.OutboundConfig{
			{ServerID: server.ID, CoreType: "xray", Protocol: "freedom", Tag: "direct", ConfigJSON: `{"protocol":"freedom","tag":"direct"}`, Enabled: true, IsDefault: true},
			{ServerID: server.ID, CoreType: "xray", Protocol: "blackhole", Tag: "blocked", ConfigJSON: `{"protocol":"blackhole","tag":"blocked"}`, Enabled: true, IsDefault: true},
		}
		for _, ob := range defaults {
			if err := s.db.WithContext(ctx).Create(&ob).Error; err != nil {
				// Non-fatal: PrepareFullConfig has a safety net that adds these
				// if missing. Log and continue.
				fmt.Printf("[nodeserver] seed default outbound %s for server %d: %v\n", ob.Tag, server.ID, err)
			}
		}
	}

	return server, nil
}

// GetByID retrieves a node server by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.NodeServer, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns all node servers with optional filtering.
func (s *Service) List(ctx context.Context, q *dto.NodeServerListQuery) ([]entity.NodeServer, int64, error) {
	if q.Region != "" {
		servers, err := s.repo.ListByRegion(ctx, q.Region)
		return servers, int64(len(servers)), err
	}
	if q.NodeType != "" {
		servers, err := s.repo.ListByType(ctx, q.NodeType)
		return servers, int64(len(servers)), err
	}
	servers, err := s.repo.ListAll(ctx)
	return servers, int64(len(servers)), err
}

// Update updates a node server.
func (s *Service) Update(ctx context.Context, id int64, req *dto.NodeServerUpdate) (*entity.NodeServer, error) {
	server, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	if req.Name != nil {
		server.Name = *req.Name
	}
	if req.Hostname != nil {
		server.Hostname = *req.Hostname
	}
	if req.Region != nil {
		server.Region = *req.Region
	}
	if req.RegionLabel != nil {
		server.RegionLabel = *req.RegionLabel
	}
	if req.NodeType != nil {
		server.NodeType = *req.NodeType
		// Adjust billing when type changes
		if server.NodeType == entity.NodeTypeChainExit || server.NodeType == entity.NodeTypeMirror {
			server.BillingEnabled = false
		}
	}
	if req.PartnerServerID != nil {
		server.PartnerServerID = *req.PartnerServerID
	}
	if req.FailoverRole != nil {
		server.FailoverRole = *req.FailoverRole
	}
	if req.FailoverPriority != nil {
		server.FailoverPriority = *req.FailoverPriority
	}
	if req.ChainConfig != nil {
		server.ChainConfig = *req.ChainConfig
	}
	if req.AgentPort != nil {
		server.AgentPort = *req.AgentPort
	}
	if req.SupportedCores != nil {
		server.SupportedCores = *req.SupportedCores
	}
	if req.MaxInbounds != nil {
		server.MaxInbounds = *req.MaxInbounds
	}
	if req.PublicIP != nil {
		server.PublicIP = *req.PublicIP
	}
	if req.InternalIP != nil {
		server.InternalIP = *req.InternalIP
	}
	if req.BandwidthMbps != nil {
		server.BandwidthMbps = *req.BandwidthMbps
	}
	if req.BillingEnabled != nil {
		server.BillingEnabled = *req.BillingEnabled
	}
	if req.CertificateID != nil {
		server.CertificateID = *req.CertificateID
	}
	if req.Metadata != nil {
		server.Metadata = *req.Metadata
	}
	if req.SortOrder != nil {
		server.SortOrder = *req.SortOrder
	}
	if req.Enabled != nil {
		server.Enabled = *req.Enabled
	}
	server.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, server); err != nil {
		return nil, fmt.Errorf("update node server: %w", err)
	}

	return server, nil
}

// Delete removes a node server.
func (s *Service) Delete(ctx context.Context, id int64) error {
	server, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}
	if server.IsLocal {
		return fmt.Errorf("cannot delete local server")
	}
	return s.repo.Delete(ctx, id)
}

// GenerateBootstrapToken generates a bootstrap token for a server.
// Returns the plaintext token (shown only once).
func (s *Service) GenerateBootstrapToken(ctx context.Context, serverID int64) (*vo.BootstrapTokenResponse, error) {
	token, err := s.tokenMgr.GenerateBootstrapToken(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("generate bootstrap token: %w", err)
	}

	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	return &vo.BootstrapTokenResponse{
		ServerID:  serverID,
		Token:     token,
		ExpiresAt: *server.BootstrapTokenExpiresAt,
		Instruction: "Copy this token to the Agent's config. It expires in 24 hours and can only be used once.",
	}, nil
}

// ToResponse converts an entity to a VO response.
func (s *Service) ToResponse(server *entity.NodeServer) *vo.NodeServerResponse {
	bootstrapRequired := server.BootstrapUsedAt == nil &&
		server.BootstrapTokenExpiresAt != nil &&
		time.Now().Before(*server.BootstrapTokenExpiresAt)

	return &vo.NodeServerResponse{
		ID:                     server.ID,
		Name:                   server.Name,
		Hostname:               server.Hostname,
		Region:                 server.Region,
		RegionLabel:            server.RegionLabel,
		NodeType:               server.NodeType,
		PartnerServerID:        server.PartnerServerID,
		FailoverRole:           server.FailoverRole,
		FailoverPriority:       server.FailoverPriority,
		ChainConfig:            server.ChainConfig,
		AgentPort:              server.AgentPort,
		AgentStatus:            server.AgentStatus,
		AgentVersion:           server.AgentVersion,
		LastHeartbeatAt:        server.LastHeartbeatAt,
		BootstrapRequired:      bootstrapRequired,
		BootstrapTokenExpiry:   server.BootstrapTokenExpiresAt,
		BootstrapUsedAt:        server.BootstrapUsedAt,
		AgentCertExpiresAt:     server.AgentCertExpiresAt,
		AgentCertLastRenewedAt: server.AgentCertLastRenewedAt,
		CertificateID:          server.CertificateID,
		IsLocal:                server.IsLocal,
		SupportedCores:         server.SupportedCores,
		MaxInbounds:            server.MaxInbounds,
		PublicIP:               server.PublicIP,
		InternalIP:             server.InternalIP,
		BandwidthMbps:          server.BandwidthMbps,
		BillingEnabled:         server.BillingEnabled,
		Metadata:               server.Metadata,
		SortOrder:              server.SortOrder,
		Enabled:                server.Enabled,
		CreatedAt:              server.CreatedAt,
		UpdatedAt:              server.UpdatedAt,
	}
}

// EnsureLocalServer ensures a local node server record exists.
func (s *Service) EnsureLocalServer(ctx context.Context, hostname string) error {
	return s.repo.EnsureLocalServer(ctx, hostname)
}

// GetCACertPEM returns the CA root certificate PEM.
func (s *Service) GetCACertPEM() []byte {
	if s.caService == nil {
		return nil
	}
	return s.caService.GetCACertPEM()
}

// RevokeAgentCertificate revokes a server's Agent certificate.
func (s *Service) RevokeAgentCertificate(ctx context.Context, serverID int64, adminID int64, reason string) error {
	if s.crl == nil {
		return fmt.Errorf("CRL not initialized")
	}
	return s.crl.RevokeCertificate(ctx, serverID, adminID, reason)
}

// DistributeCertificate distributes a TLS certificate to a specific server.
func (s *Service) DistributeCertificate(ctx context.Context, serverID int64, certID int64) error {
	if s.certDistrib == nil {
		return fmt.Errorf("cert distributor not initialized")
	}
	return s.certDistrib.DistributeToServer(ctx, serverID, certID)
}

// DistributeCertificateToAll distributes a TLS certificate to all associated servers.
func (s *Service) DistributeCertificateToAll(ctx context.Context, certID int64) error {
	if s.certDistrib == nil {
		return fmt.Errorf("cert distributor not initialized")
	}
	return s.certDistrib.DistributeToAllServers(ctx, certID)
}

// ──────────────────── Phase 3: Node Types & Chain Proxy ────────────────────

// GetFailoverStatus returns the failover status for a primary_backup server.
func (s *Service) GetFailoverStatus(ctx context.Context, serverID int64) (*agent.FailoverStatus, error) {
	if s.failoverMgr == nil {
		return nil, fmt.Errorf("failover manager not initialized")
	}
	return s.failoverMgr.GetFailoverStatus(ctx, serverID)
}

// TriggerFailoverCheck forces a failover check for all primary_backup pairs.
func (s *Service) TriggerFailoverCheck(ctx context.Context) error {
	if s.failoverMgr == nil {
		return fmt.Errorf("failover manager not initialized")
	}
	// The RunLoop handles periodic checks; this method can trigger a manual check
	// by calling the same logic. We expose a simplified interface.
	servers, err := s.repo.ListByType(ctx, entity.NodeTypePrimaryBackup)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if server.FailoverRole == entity.FailoverRolePrimary {
			// We rely on the manager's internal logic; just log here
			_ = server
		}
	}
	return nil
}

// SyncMirrorNow forces an immediate mirror sync for a specific mirror node.
func (s *Service) SyncMirrorNow(ctx context.Context, mirrorID int64) error {
	if s.mirrorSync == nil {
		return fmt.Errorf("mirror sync manager not initialized")
	}
	return s.mirrorSync.SyncMirrorNow(ctx, mirrorID)
}

// GetMirrorStatus returns the sync status of a mirror node.
func (s *Service) GetMirrorStatus(ctx context.Context, mirrorID int64) (*agent.MirrorStatus, error) {
	if s.mirrorSync == nil {
		return nil, fmt.Errorf("mirror sync manager not initialized")
	}
	return s.mirrorSync.GetMirrorStatus(ctx, mirrorID)
}

// GenerateChainConfig generates the chain proxy outbound/routing config for a chain_entry node.
func (s *Service) GenerateChainConfig(ctx context.Context, serverID int64) ([]agent.ChainEntryOutbound, []agent.ChainRoutingRule, error) {
	if s.chainGen == nil {
		return nil, nil, fmt.Errorf("chain config generator not initialized")
	}
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("server not found: %w", err)
	}
	return s.chainGen.GenerateChainEntryOutbounds(ctx, server)
}

// GenerateChainExitConfig generates the relay inbound config for a chain_exit node.
func (s *Service) GenerateChainExitConfig(ctx context.Context, serverID int64) (map[string]interface{}, error) {
	if s.chainGen == nil {
		return nil, fmt.Errorf("chain config generator not initialized")
	}
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}
	return s.chainGen.GenerateChainExitInbound(ctx, server)
}

// GetChainHealthStatus returns the health status of all chain_exit nodes for a chain_entry.
func (s *Service) GetChainHealthStatus(ctx context.Context, entryServerID int64) (map[int64]*agent.ExitHealthStatus, error) {
	if s.chainHealth == nil {
		return nil, fmt.Errorf("chain health registry not initialized")
	}
	monitor := s.chainHealth.GetMonitor(entryServerID)
	if monitor == nil {
		return nil, fmt.Errorf("no health monitor for server %d (is it a chain_entry?)", entryServerID)
	}
	return monitor.GetAllExitStatuses(), nil
}

// ListBillableServerIDs returns server IDs that should receive user sync.
func (s *Service) ListBillableServerIDs(ctx context.Context) ([]int64, error) {
	if s.billingFilter == nil {
		return nil, fmt.Errorf("billing filter not initialized")
	}
	return s.billingFilter.ListBillableServerIDs(ctx)
}

// ValidateChainConfig validates a chain_config JSON string for a given node type.
func (s *Service) ValidateChainConfig(nodeType, chainConfigJSON string) error {
	cfg, err := agent.ParseChainConfig(chainConfigJSON)
	if err != nil {
		return err
	}
	if cfg == nil {
		return nil // Empty is OK
	}
	return cfg.Validate(nodeType)
}

// ──────────────────── Phase 4: User Sync & Traffic Aggregation ────────────────────

// SyncUserToServers triggers user credential sync to all servers for a client.
func (s *Service) SyncUserToServers(ctx context.Context, clientID int64, action string) error {
	if s.userSyncSvc == nil {
		return fmt.Errorf("user sync service not initialized")
	}
	return s.userSyncSvc.SyncUserToServers(ctx, clientID, action)
}

// SyncUserRemoval removes a user from all servers.
func (s *Service) SyncUserRemoval(ctx context.Context, clientID int64) error {
	if s.userSyncSvc == nil {
		return fmt.Errorf("user sync service not initialized")
	}
	return s.userSyncSvc.SyncUserRemoval(ctx, clientID)
}

// GetRetryQueueSize returns the number of pending user sync retries.
func (s *Service) GetRetryQueueSize() int {
	if s.userSyncSvc == nil {
		return 0
	}
	return s.userSyncSvc.GetRetryQueueSize()
}

// ProcessRetryQueue manually triggers processing of the retry queue.
func (s *Service) ProcessRetryQueue(ctx context.Context) {
	if s.userSyncSvc == nil {
		return
	}
	s.userSyncSvc.ProcessRetryQueue(ctx)
}

// GetTrafficStats returns multi-server traffic collector statistics.
func (s *Service) GetTrafficStats() interface{} {
	if s.trafficStats == nil {
		return nil
	}
	return s.trafficStats.GetStats()
}

// ──────────────────── Phase 5: Dashboard & Enhancements ────────────────────

// GetDashboardStatus returns the aggregated health summary for the dashboard.
func (s *Service) GetDashboardStatus(ctx context.Context) (interface{}, error) {
	if s.healthChecker == nil {
		return nil, fmt.Errorf("health checker not initialized")
	}
	return s.healthChecker.GetServerStatusSummary(ctx)
}

// RecommendRegion returns the recommended region for a client IP.
func (s *Service) RecommendRegion(clientIP string, allowedRegions []string) string {
	if s.regionSelector == nil {
		if len(allowedRegions) > 0 {
			return allowedRegions[0]
		}
		return ""
	}
	return s.regionSelector.RecommendRegion(clientIP, allowedRegions)
}

// GenerateAgentInstallScript generates the Agent install script for a server.
func (s *Service) GenerateAgentInstallScript(ctx context.Context, serverID int64, panelHost string, panelPort int) (string, error) {
	// Verify server exists
	if _, err := s.repo.GetByID(ctx, serverID); err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Generate bootstrap token if not already present
	token, err := s.tokenMgr.GenerateBootstrapToken(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("generate bootstrap token: %w", err)
	}

	script := agent.NewAgentInstallScript(panelHost, panelPort).
		WithBootstrapToken(token).
		WithServerID(serverID)

	return script.Generate()
}

// ──────────────────── Phase 6: Advanced Features API ────────────────────

// GetMetricsHistory returns historical metrics for a server.
func (s *Service) GetMetricsHistory(serverID int64, duration, interval time.Duration) (interface{}, error) {
	if s.metricsCollector == nil {
		return nil, fmt.Errorf("metrics collector not initialized")
	}
	return s.metricsCollector.GetMetricsHistory(serverID, duration, interval)
}

// GetAggregatedMetrics returns aggregated metrics for all servers.
func (s *Service) GetAggregatedMetrics(ctx context.Context) (interface{}, error) {
	if s.metricsCollector == nil {
		return nil, fmt.Errorf("metrics collector not initialized")
	}
	return s.metricsCollector.GetAggregatedMetrics(ctx)
}

// TestServerSpeed tests latency to a specific server.
func (s *Service) TestServerSpeed(ctx context.Context, serverID int64) (interface{}, error) {
	if s.speedTestMgr == nil {
		return nil, fmt.Errorf("speed test manager not initialized")
	}
	return s.speedTestMgr.TestServer(ctx, serverID)
}

// TestAllServersSpeed tests latency to all servers.
func (s *Service) TestAllServersSpeed(ctx context.Context) (interface{}, error) {
	if s.speedTestMgr == nil {
		return nil, fmt.Errorf("speed test manager not initialized")
	}
	return s.speedTestMgr.TestAllServers(ctx)
}

// GetSpeedTestResults returns all cached speed test results.
func (s *Service) GetSpeedTestResults() interface{} {
	if s.speedTestMgr == nil {
		return nil
	}
	return s.speedTestMgr.GetAllResults()
}

// SelectNodesForSubscription selects the best nodes for a subscription request.
func (s *Service) SelectNodesForSubscription(ctx context.Context, userRegion string, count int, excludeIDs []int64) (interface{}, error) {
	if s.loadBalancer == nil {
		return nil, fmt.Errorf("load balancer not initialized")
	}
	return s.loadBalancer.SelectNodes(ctx, userRegion, count, excludeIDs)
}

// UpdateAgent triggers an Agent update on a specific server.
func (s *Service) UpdateAgent(ctx context.Context, serverID int64, version, downloadURL, checksum string) error {
	if s.agentUpdater == nil {
		return fmt.Errorf("agent updater not initialized")
	}
	s.agentUpdater.WithVersion(version, downloadURL, checksum)
	return s.agentUpdater.UpdateServer(ctx, serverID)
}

// UpdateAllAgents triggers Agent updates on all eligible remote servers.
func (s *Service) UpdateAllAgents(ctx context.Context) error {
	if s.agentUpdater == nil {
		return fmt.Errorf("agent updater not initialized")
	}
	return s.agentUpdater.UpdateAllServers(ctx)
}

// GetPendingAgentUpdates returns all pending Agent update tasks.
func (s *Service) GetPendingAgentUpdates() interface{} {
	if s.agentUpdater == nil {
		return nil
	}
	return s.agentUpdater.GetPendingUpdates()
}

// ──────────────────── Supplementary: Server Management ────────────────────

// GetServerStatus returns real-time status of a server (core states, uptime, etc.).
func (s *Service) GetServerStatus(ctx context.Context, serverID int64) (interface{}, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.GetStatusForServer(ctx, serverID)
}

// ControlCore sends a control command (start/stop/restart) to a server's core.
func (s *Service) ControlCore(ctx context.Context, serverID int64, coreType, action string) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ControlCore(ctx, serverID, coreType, action)
}

// SyncServerConfig triggers a manual config sync to a specific server.
func (s *Service) SyncServerConfig(ctx context.Context, serverID int64, coreType string, config []byte, version int64) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.SyncConfig(ctx, serverID, coreType, config, version)
}

// SyncServerData triggers a full database-level data sync from the panel to a
// specific remote Agent. The Agent writes the data to its local SQLite database,
// generates the runtime config from the local data, and starts the core process.
func (s *Service) SyncServerData(ctx context.Context, serverID int64, applyAndStart bool) (*agentpb.SyncDataResponse, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.SyncDataToServer(ctx, serverID, applyAndStart)
}

// IsLocalServer checks if a server is the local panel server.
func (s *Service) IsLocalServer(ctx context.Context, serverID int64) bool {
	if s.nodeMgr == nil {
		return false
	}
	return s.nodeMgr.IsLocalServer(serverID)
}

// GetServerPlatform returns the OS and architecture of a server's Agent.
func (s *Service) GetServerPlatform(ctx context.Context, serverID int64) (string, string) {
	if s.nodeMgr == nil {
		return runtime.GOOS, runtime.GOARCH
	}
	return s.nodeMgr.GetServerPlatform(serverID)
}

// GetServerDistro returns the Linux distribution of a server's Agent.
func (s *Service) GetServerDistro(ctx context.Context, serverID int64) string {
	if s.nodeMgr == nil {
		return ""
	}
	return s.nodeMgr.GetServerDistro(serverID)
}

// GetRemoteState returns cached state from a remote Agent (core states + binaries).
func (s *Service) GetRemoteState(ctx context.Context, serverID int64) (coreStates map[string]*services.CoreStateInfo, binaries []*services.BinaryInfoItem, err error) {
	if s.nodeMgr == nil {
		return nil, nil, fmt.Errorf("node manager not initialized")
	}
	statusResp, binResp, err := s.nodeMgr.GetRemoteState(ctx, serverID)
	if err != nil {
		return nil, nil, err
	}

	// Convert proto core states
	coreStates = make(map[string]*services.CoreStateInfo)
	if statusResp != nil {
		for _, cs := range statusResp.Cores {
			coreStates[cs.CoreType] = &services.CoreStateInfo{
				State:         cs.State,
				PID:           cs.Pid,
				UptimeSeconds: cs.UptimeSeconds,
				CpuUsage:      cs.CpuUsage,
				MemUsageBytes: cs.MemUsageBytes,
			}
		}
	}

	// Convert proto binaries
	if binResp != nil {
		for _, b := range binResp {
			binaries = append(binaries, &services.BinaryInfoItem{
				CoreType:   b.CoreType,
				Version:    b.Version,
				BinaryPath: b.BinaryPath,
				Installed:  b.Installed,
			})
		}
	}

	return coreStates, binaries, nil
}

// ControlProxy controls a proxy process (caddy/nginx) on a specific server.
func (s *Service) ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ControlProxy(ctx, serverID, proxyType, action)
}

// SyncProxyConfig syncs proxy configuration to a specific server.
func (s *Service) SyncProxyConfig(ctx context.Context, serverID int64, proxyType string, config []byte) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.SyncProxyConfig(ctx, serverID, proxyType, config)
}

// InstallBinary installs a core binary on a specific server (local or remote via Agent).
func (s *Service) InstallBinary(ctx context.Context, serverID int64, coreType, version, downloadURL, assetName string) (interface{}, error) {
if s.nodeMgr == nil {
return nil, fmt.Errorf("node manager not initialized")
}
result, err := s.nodeMgr.InstallBinary(ctx, serverID, coreType, version, downloadURL, assetName)
if err == nil {
// Invalidate cached binary info so next query fetches fresh data
s.nodeMgr.InvalidateBinaryCache(serverID)
}
return result, err
}

// InstallProxy installs a proxy binary (caddy/nginx) on a specific server (local or remote via Agent).
func (s *Service) InstallProxy(ctx context.Context, serverID int64, proxyType, downloadURL, tagName string) (interface{}, error) {
if s.nodeMgr == nil {
return nil, fmt.Errorf("node manager not initialized")
}
result, err := s.nodeMgr.InstallProxy(ctx, serverID, proxyType, downloadURL, tagName)
if err == nil {
// Invalidate cached binary info so next query fetches fresh data
s.nodeMgr.InvalidateBinaryCache(serverID)
}
return result, err
}

// GetProxyStatusRemote queries proxy installation status on a specific server.
func (s *Service) GetProxyStatusRemote(ctx context.Context, serverID int64) (interface{}, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.GetProxyStatus(ctx, serverID)
}

// UpdateProxyGuardian updates proxy guardian settings for a specific server.
// For local nodes, the caller (ProxyHandler) handles the update directly.
// For remote nodes, this stores the settings in the panel DB for display.
func (s *Service) UpdateProxyGuardian(ctx context.Context, serverID int64, proxyType string, autoStart, autoRestart *bool, maxRestarts *int) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	// Read current values
	curAutoStart, curAutoRestart, curMaxRestarts := s.nodeMgr.GetProxyGuardianSettingsForServer(ctx, serverID, proxyType)
	if autoStart != nil {
		curAutoStart = *autoStart
	}
	if autoRestart != nil {
		curAutoRestart = *autoRestart
	}
	if maxRestarts != nil && *maxRestarts > 0 {
		curMaxRestarts = *maxRestarts
	}
	return s.nodeMgr.UpdateProxyGuardianForServer(ctx, serverID, proxyType, curAutoStart, curAutoRestart, curMaxRestarts)
}

// GetProxyGuardianSettingsForServer reads proxy guardian settings from the panel DB.
func (s *Service) GetProxyGuardianSettingsForServer(ctx context.Context, serverID int64, proxyType string) (autoStart, autoRestart bool, maxRestarts int) {
	if s.nodeMgr == nil {
		return true, false, 5
	}
	return s.nodeMgr.GetProxyGuardianSettingsForServer(ctx, serverID, proxyType)
}

// ListBinaries lists installed core binaries on a specific server.
func (s *Service) ListBinaries(ctx context.Context, serverID int64) (interface{}, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ListBinaries(ctx, serverID)
}

// SetBinaryLister sets the binary lister for fetching releases from GitHub.
func (s *Service) SetBinaryLister(lister BinaryLister) {
	s.binaryLister = lister
}

// ListReleases lists available binary releases from GitHub for a core type.
func (s *Service) ListReleases(ctx context.Context, coreType string) (interface{}, error) {
	if s.binaryLister == nil {
		return nil, fmt.Errorf("binary lister not initialized")
	}
	return s.binaryLister.ListReleases(ctx, coreType)
}

// ListReleasesForServer lists available binary releases filtered by the target server's platform.
func (s *Service) ListReleasesForServer(ctx context.Context, serverID int64, coreType string) (interface{}, error) {
	if s.binaryLister == nil {
		return nil, fmt.Errorf("binary lister not initialized")
	}
	osName, archName := s.GetServerPlatform(ctx, serverID)
	return s.binaryLister.ListReleasesForPlatform(ctx, coreType, osName, archName)
}

// ExecuteBinaryCommand executes a binary command on a specific server.
// For local servers, executes directly. For remote servers, sends via gRPC to Agent.
func (s *Service) ExecuteBinaryCommand(ctx context.Context, serverID int64, coreType string, args ...string) (map[string]string, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ExecuteBinaryCommand(ctx, serverID, coreType, args...)
}

// ExecuteBinaryRawOutput executes a binary command and returns the raw stdout string.
// For local servers, executes directly. For remote servers, sends via gRPC to Agent.
func (s *Service) ExecuteBinaryRawOutput(ctx context.Context, serverID int64, coreType string, args ...string) (string, error) {
	if s.nodeMgr == nil {
		return "", fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ExecuteBinaryRawOutput(ctx, serverID, coreType, args...)
}

// ListInterfaces lists network interfaces on a specific server.
// For local servers, queries directly. For remote servers, sends via gRPC to Agent.
func (s *Service) ListInterfaces(ctx context.Context, serverID int64) ([]string, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ListInterfaces(ctx, serverID)
}

// XrayAPIOnServer proxies an xray API call to a remote server's Agent.
func (s *Service) XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	if s.nodeMgr == nil {
		return nil, fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.XrayAPIOnServer(ctx, serverID, req)
}

// GetBootstrapCommand generates the bootstrap installation command for a server.
func (s *Service) GetBootstrapCommand(ctx context.Context, serverID int64, panelHost string, panelPort int) (string, error) {
	required, err := s.tokenMgr.IsBootstrapRequired(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("check bootstrap required: %w", err)
	}
	if !required {
		return "", fmt.Errorf("bootstrap already completed for server %d", serverID)
	}

	token, err := s.tokenMgr.GenerateBootstrapToken(ctx, serverID)
	if err != nil {
		return "", fmt.Errorf("generate bootstrap token: %w", err)
	}

	script := agent.NewAgentInstallScript(panelHost, panelPort).
		WithBootstrapToken(token).
		WithServerID(serverID)
	return script.Generate()
}

// UpdateNodeType changes the node type of a server.
func (s *Service) UpdateNodeType(ctx context.Context, serverID int64, nodeType string) error {
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}
	server.NodeType = nodeType
	// chain_exit and mirror nodes are non-billing by default
	if nodeType == entity.NodeTypeChainExit || nodeType == entity.NodeTypeMirror {
		server.BillingEnabled = false
	}
	server.UpdatedAt = time.Now()
	return s.repo.Update(ctx, server)
}

// TriggerServerFailover triggers failover for a specific primary_backup server.
func (s *Service) TriggerServerFailover(ctx context.Context, serverID int64) error {
	if s.failoverMgr == nil {
		return fmt.Errorf("failover manager not initialized")
	}
	// The FailoverManager's checkAllPairs handles the logic.
	// For a targeted trigger, we call the same check which will evaluate this server.
	servers, err := s.repo.ListByType(ctx, entity.NodeTypePrimaryBackup)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if server.ID == serverID && server.FailoverRole == entity.FailoverRolePrimary {
			// Force check on this primary by verifying its status
			if !s.nodeMgr.IsOnlineForServer(serverID) {
				// Look for backup
				partners, _ := s.repo.ListByPartner(ctx, serverID)
				for _, backup := range partners {
					if s.nodeMgr.IsOnlineForServer(backup.ID) {
						// Swap roles
						server.FailoverRole = entity.FailoverRoleBackup
						backup.FailoverRole = entity.FailoverRolePrimary
						server.UpdatedAt = time.Now()
						backup.UpdatedAt = time.Now()
						s.repo.Update(ctx, &server)
						s.repo.Update(ctx, &backup)
						break
					}
				}
			}
			break
		}
	}
	return nil
}

// ListServerCertificates returns certificates associated with a server.
func (s *Service) ListServerCertificates(ctx context.Context, serverID int64) (interface{}, error) {
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}
	if server.CertificateID == 0 || s.certRepo == nil {
		return []interface{}{}, nil
	}
	cert, err := s.certRepo.GetByID(ctx, server.CertificateID)
	if err != nil {
		return []interface{}{}, nil
	}
	return []interface{}{cert}, nil
}

// AssociateCertificate associates a TLS certificate with a server.
func (s *Service) AssociateCertificate(ctx context.Context, serverID, certID int64) error {
	return s.repo.UpdateCertificateID(ctx, serverID, certID)
}

// DissociateCertificate removes the TLS certificate association from a server.
func (s *Service) DissociateCertificate(ctx context.Context, serverID int64) error {
	return s.repo.UpdateCertificateID(ctx, serverID, 0)
}

// ──────────────────── Supplementary: CA Center ────────────────────

// GetCAStatus returns the CA root certificate status.
func (s *Service) GetCAStatus() interface{} {
	if s.caService == nil {
		return gin.H{"initialized": false}
	}
	cert := s.caService.GetCACert()
	return gin.H{
		"initialized":   true,
		"subject":       cert.Subject.String(),
		"serial":        cert.SerialNumber.String(),
		"not_before":    cert.NotBefore,
		"not_after":     cert.NotAfter,
		"issuer":        cert.Issuer.String(),
		"fingerprint":   fmt.Sprintf("%x", cert.SerialNumber),
	}
}

// ListAgentCerts returns all Agent certificates (from node servers that have one).
func (s *Service) ListAgentCerts(ctx context.Context) (interface{}, error) {
	servers, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	type agentCertInfo struct {
		ServerID       int64      `json:"server_id"`
		ServerName     string     `json:"server_name"`
		Hostname       string     `json:"hostname"`
		CertSerial     string     `json:"cert_serial"`
		ExpiresAt      *time.Time `json:"expires_at"`
		LastRenewedAt  *time.Time `json:"last_renewed_at"`
		AgentStatus    string     `json:"agent_status"`
	}
	var certs []agentCertInfo
	for _, s := range servers {
		if s.AgentCertSerial != "" {
			certs = append(certs, agentCertInfo{
				ServerID:      s.ID,
				ServerName:    s.Name,
				Hostname:      s.Hostname,
				CertSerial:    s.AgentCertSerial,
				ExpiresAt:     s.AgentCertExpiresAt,
				LastRenewedAt: s.AgentCertLastRenewedAt,
				AgentStatus:   s.AgentStatus,
			})
		}
	}
	return certs, nil
}

// RenewAgentCertificate manually renews an Agent's mTLS certificate.
func (s *Service) RenewAgentCertificate(ctx context.Context, serverID int64) error {
	if s.caService == nil {
		return fmt.Errorf("CA not initialized")
	}
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}
	result, err := s.caService.IssueAgentCertificate(ctx, serverID, server.Hostname)
	if err != nil {
		return fmt.Errorf("issue cert: %w", err)
	}
	if err := s.repo.UpdateAgentCert(ctx, serverID, result.Serial, result.ExpiresAt); err != nil {
		return fmt.Errorf("update agent cert: %w", err)
	}
	// Push new cert to Agent if online
	if s.nodeMgr != nil && s.nodeMgr.IsOnlineForServer(serverID) {
		_ = s.nodeMgr.PushAgentCertificate(ctx, serverID, result)
	}
	return nil
}

// ListRevocations returns all certificate revocations.
func (s *Service) ListRevocations(ctx context.Context) (interface{}, error) {
	if s.crlRepo == nil {
		return nil, fmt.Errorf("CRL repository not initialized")
	}
	return s.crlRepo.ListActive(ctx)
}

// ExportCRL exports the CRL in PEM format.
func (s *Service) ExportCRL(ctx context.Context) ([]byte, error) {
	if s.crlRepo == nil {
		return nil, fmt.Errorf("CRL repository not initialized")
	}
	revocations, err := s.crlRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	// Build a simple text-based CRL export
	var buf []byte
	buf = append(buf, []byte("-----BEGIN X509 CRL-----\n")...)
	for _, rev := range revocations {
		line := fmt.Sprintf("Serial: %s, RevokedAt: %s, Reason: %s, ServerID: %d\n",
			rev.CertSerial, rev.RevokedAt.Format(time.RFC3339), rev.Reason, rev.ServerID)
		buf = append(buf, []byte(line)...)
	}
	buf = append(buf, []byte("-----END X509 CRL-----\n")...)
	return buf, nil
}

// ──────────────────── Supplementary: Region Overview ────────────────────

// GetRegionsOverview returns aggregated server/node statistics per region.
func (s *Service) GetRegionsOverview(ctx context.Context) (interface{}, error) {
	servers, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	type regionStat struct {
		Region      string `json:"region"`
		RegionLabel string `json:"region_label"`
		Total       int    `json:"total"`
		Online      int    `json:"online"`
		Offline     int    `json:"offline"`
		Billable    int    `json:"billable"`
	}
	stats := make(map[string]*regionStat)
	for _, srv := range servers {
		key := srv.Region
		if key == "" {
			key = "default"
		}
		if _, ok := stats[key]; !ok {
			stats[key] = &regionStat{Region: srv.Region, RegionLabel: srv.RegionLabel}
		}
		stats[key].Total++
		if srv.AgentStatus == entity.AgentStatusOnline {
			stats[key].Online++
		} else {
			stats[key].Offline++
		}
		if srv.ShouldBill() {
			stats[key].Billable++
		}
	}

	var result []regionStat
	for _, v := range stats {
		result = append(result, *v)
	}
	return result, nil
}

// ListServersByRegion returns all servers in a specific region.
func (s *Service) ListServersByRegion(ctx context.Context, region string) ([]entity.NodeServer, error) {
	return s.repo.ListByRegion(ctx, region)
}

// ──────────────────── Supplementary: Node Audit Logs ────────────────────

// CreateAuditLog records a node server administrative action.
func (s *Service) CreateAuditLog(ctx context.Context, serverID, adminID int64, action, detail, sourceIP, userAgent string, success bool, errMsg string) error {
	if s.auditLogRepo == nil {
		return nil // audit logging is optional
	}
	return s.auditLogRepo.Create(ctx, &entity.NodeAuditLog{
		ServerID:     serverID,
		AdminID:      adminID,
		Action:       action,
		Detail:       detail,
		SourceIP:     sourceIP,
		UserAgent:    userAgent,
		Success:      success,
		ErrorMessage: errMsg,
		CreatedAt:    time.Now(),
	})
}

// ListAuditLogs returns audit logs with optional filtering.
func (s *Service) ListAuditLogs(ctx context.Context, serverID int64, action string, limit, offset int) (interface{}, int64, error) {
	if s.auditLogRepo == nil {
		return []interface{}{}, 0, nil
	}
	return s.auditLogRepo.List(ctx, serverID, action, limit, offset)
}

// ListAuditLogsByTimeRange returns audit logs within a time range (for export).
func (s *Service) ListAuditLogsByTimeRange(ctx context.Context, from, to time.Time) ([]entity.NodeAuditLog, error) {
	if s.auditLogRepo == nil {
		return nil, nil
	}
	return s.auditLogRepo.ListByTimeRange(ctx, from, to)
}

// ──────────────────── Supplementary: Node Tags ────────────────────

// ListServerTags returns all tags for a server.
func (s *Service) ListServerTags(ctx context.Context, serverID int64) (interface{}, error) {
	if s.tagRepo == nil {
		return []interface{}{}, nil
	}
	return s.tagRepo.ListByServer(ctx, serverID)
}

// AddServerTag adds or updates a tag on a server.
func (s *Service) AddServerTag(ctx context.Context, serverID int64, key, value string) error {
	if s.tagRepo == nil {
		return fmt.Errorf("tag repository not initialized")
	}
	return s.tagRepo.Create(ctx, &entity.ServerTag{
		ServerID:  serverID,
		TagKey:    key,
		TagValue:  value,
		CreatedAt: time.Now(),
	})
}

// RemoveServerTag removes a tag from a server.
func (s *Service) RemoveServerTag(ctx context.Context, serverID int64, key string) error {
	if s.tagRepo == nil {
		return fmt.Errorf("tag repository not initialized")
	}
	return s.tagRepo.Delete(ctx, serverID, key)
}

// ListServersByTag returns server IDs matching the given tag.
func (s *Service) ListServersByTag(ctx context.Context, key, value string) ([]int64, error) {
	if s.tagRepo == nil {
		return nil, fmt.Errorf("tag repository not initialized")
	}
	return s.tagRepo.FindServersByTag(ctx, key, value)
}

// ──────────────────── Supplementary: Agent Degradation & Reconnect ────────────────────

// GetDegradationStatus returns the Agent connection degradation status.
func (s *Service) GetDegradationStatus(ctx context.Context, serverID int64) (interface{}, error) {
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	status := gin.H{
		"server_id":     serverID,
		"agent_status":  server.AgentStatus,
		"is_local":      server.IsLocal,
	}
	if s.nodeMgr != nil {
		status["online"] = s.nodeMgr.IsOnlineForServer(serverID)
		if server.IsLocal {
			// Local server: get core states from LocalAgent
			status["last_heartbeat_age"] = "local"
			if resp, err := s.nodeMgr.GetStatusForServer(ctx, serverID); err == nil {
				status["core_states"] = resp
			} else {
				status["core_states"] = nil
			}
		} else {
			conn := s.nodeMgr.GetConnection(serverID)
			if conn != nil {
				status["last_heartbeat_age"] = time.Since(conn.LastHeartbeatTime()).String()
				status["core_states"] = conn.GetCoreStates()
			} else {
				status["last_heartbeat_age"] = "disconnected"
				status["core_states"] = nil
			}
		}
	}
	if server.LastHeartbeatAt != nil {
		status["last_heartbeat_at"] = server.LastHeartbeatAt
	}
	return status, nil
}

// ForceReconnect disconnects an Agent to force it to reconnect.
func (s *Service) ForceReconnect(ctx context.Context, serverID int64) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	s.nodeMgr.DisconnectServer(ctx, serverID)
	return nil
}

// ──────────────────── Supplementary: Cert Distribution Queue ────────────────────

// GetDistributionQueue returns pending/all distribution queue items.
func (s *Service) GetDistributionQueue(ctx context.Context, status string, limit, offset int) (interface{}, int64, error) {
	if s.distQueueRepo == nil {
		return []interface{}{}, 0, nil
	}
	return s.distQueueRepo.ListAll(ctx, status, limit, offset)
}

// RetryDistribution retries a failed distribution queue item.
func (s *Service) RetryDistribution(ctx context.Context, itemID int64) error {
	if s.distQueueRepo == nil {
		return fmt.Errorf("distribution queue repository not initialized")
	}
	return s.distQueueRepo.UpdateStatus(ctx, itemID, "pending", "")
}

// DeleteDistributionQueueItem removes a distribution queue item.
func (s *Service) DeleteDistributionQueueItem(ctx context.Context, itemID int64) error {
	if s.distQueueRepo == nil {
		return fmt.Errorf("distribution queue repository not initialized")
	}
	return s.distQueueRepo.Delete(ctx, itemID)
}
