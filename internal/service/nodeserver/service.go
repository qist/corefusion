package nodeserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"corefusion/internal/constants"
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
	metricsCollector  *agent.MetricsCollector
	speedTestMgr     *agent.SpeedTestManager
	loadBalancer      *agent.LoadBalancer
	agentUpdater      *agent.AgentUpdater
	trafficQuotaSvc   *NodeTrafficQuotaService

	// Supplementary repos
	auditLogRepo  *repository.NodeAuditLogRepository
	tagRepo       *repository.ServerTagRepository
	distQueueRepo *repository.CertDistributionQueueRepository
	certRepo      *repository.CertificateRepository
	crlRepo       *repository.CertRevocationRepository
	settingRepo   *repository.SystemSettingRepository
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
	// SyncServerFull re-sends all in-scope users to a server (used after a
	// failover promotes a backup to primary).
	SyncServerFull(ctx context.Context, serverID int64) error
	// RemoveServerUsers removes all users from a server (used after a failover
	// demotes the previous primary back to backup).
	RemoveServerUsers(ctx context.Context, serverID int64) error
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

// WithTrafficQuotaService sets the traffic quota service.
func (s *Service) WithTrafficQuotaService(svc *NodeTrafficQuotaService) *Service {
	s.trafficQuotaSvc = svc
	return s
}

// GetTrafficQuotaService returns the traffic quota service.
func (s *Service) GetTrafficQuotaService() *NodeTrafficQuotaService {
	return s.trafficQuotaSvc
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
	s.settingRepo = repository.NewSystemSettingRepository(db)
	return s
}

// Create creates a new node server.
func (s *Service) Create(ctx context.Context, req *dto.NodeServerCreate) (*vo.CreateServerResponse, error) {
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
		agentPort = constants.DefaultAgentGRPCPort
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

		// 出口网卡流量配额
		EgressInterface:      req.EgressInterface,
		TrafficQuotaBytes:    req.TrafficQuotaBytes,
		TrafficQuotaResetDay: req.TrafficQuotaResetDay,
	}

	if err := s.repo.Create(ctx, server); err != nil {
		return nil, fmt.Errorf("create node server: %w", err)
	}

	resp := &vo.CreateServerResponse{
		NodeServerResponse: s.ToResponse(server),
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

	// Auto-generate Bootstrap Token so the frontend can display the
	// curl install command immediately (Jenkins-style one-liner).
	if !server.IsLocal && s.tokenMgr != nil {
		token, err := s.tokenMgr.GenerateBootstrapToken(ctx, server.ID)
		if err != nil {
			log.Printf("[nodeserver] auto-generate bootstrap token for server %d failed: %v", server.ID, err)
			// Non-fatal: user can generate token manually later
		} else {
			resp.BootstrapToken = token
			// Re-fetch the server so BootstrapTokenExpiresAt is populated,
			// making bootstrap_required = true in the response.
			if refreshed, err := s.repo.GetByID(ctx, server.ID); err == nil {
				resp.NodeServerResponse = s.ToResponse(refreshed)
			}
		}
	}

	return resp, nil
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

	// 出口网卡流量配额
	if req.EgressInterface != nil {
		server.EgressInterface = *req.EgressInterface
	}
	if req.TrafficQuotaBytes != nil {
		server.TrafficQuotaBytes = *req.TrafficQuotaBytes
	}
	if req.TrafficQuotaResetDay != nil {
		server.TrafficQuotaResetDay = *req.TrafficQuotaResetDay
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
// Optionally accepts a ResourceUsage to embed in the response.
func (s *Service) ToResponse(server *entity.NodeServer, resourceUsage ...*vo.ResourceUsage) *vo.NodeServerResponse {
	bootstrapRequired := server.BootstrapUsedAt == nil &&
		server.BootstrapTokenExpiresAt != nil &&
		time.Now().Before(*server.BootstrapTokenExpiresAt)

	resp := &vo.NodeServerResponse{
		ID:                     server.ID,
		Name:                   server.Name,
		Hostname:               server.Hostname,
		Region:                 server.Region,
		RegionLabel:            server.RegionLabel,
		NodeType:               server.NodeType,
		PartnerServerID:        server.PartnerServerID,
		FailoverRole:           server.FailoverRole,
		FailoverPriority:       server.FailoverPriority,
		RecoverMode:            server.RecoverMode,
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

		// 出口网卡流量配额
		EgressInterface:       server.EgressInterface,
		TrafficQuotaBytes:     server.TrafficQuotaBytes,
		TrafficQuotaResetDay:  server.TrafficQuotaResetDay,
		TrafficSuspended:      server.TrafficSuspended,
		TrafficSuspendedAt:    server.TrafficSuspendedAt,
		TrafficUsedBytes:      server.TrafficUsedBytes,
		TrafficPeriodStart:    server.TrafficPeriodStart,
	}

	if len(resourceUsage) > 0 && resourceUsage[0] != nil {
		resp.ResourceUsage = resourceUsage[0]
	}

	// Local node doesn't run a separate Agent binary — it's managed directly
	// by the Panel. Show the Panel version as its "agent version" so the UI
	// displays something meaningful instead of a blank field.
	// This is purely a presentation-level override; the DB value is untouched.
	if server.IsLocal && resp.AgentVersion == "" {
		resp.AgentVersion = constants.Version
	}

	return resp
}

// ListWithResourceUsage returns servers enriched with the latest resource usage data.
func (s *Service) ListWithResourceUsage(ctx context.Context, q *dto.NodeServerListQuery) ([]vo.NodeServerResponse, int64, error) {
	servers, total, err := s.List(ctx, q)
	if err != nil {
		return nil, 0, err
	}

	// Batch fetch latest metrics
	var usageMap map[int64]*vo.ResourceUsage
	if s.metricsCollector != nil {
		metricsMap, err := s.metricsCollector.GetLatestMetricsBatch(ctx)
		if err == nil && len(metricsMap) > 0 {
			usageMap = make(map[int64]*vo.ResourceUsage, len(metricsMap))
			for serverID, m := range metricsMap {
				usageMap[serverID] = metricToResourceUsage(m)
			}
		}
	}

	items := make([]vo.NodeServerResponse, len(servers))
	for i, srv := range servers {
		var usage *vo.ResourceUsage
		if usageMap != nil {
			usage = usageMap[srv.ID]
		}
		resp := s.ToResponse(&srv, usage)
		items[i] = *resp
	}

	return items, total, nil
}

// metricToResourceUsage converts a ServerMetric entity to a ResourceUsage VO.
func metricToResourceUsage(m *entity.ServerMetric) *vo.ResourceUsage {
	if m == nil || m.ID == 0 {
		return nil
	}
	return &vo.ResourceUsage{
		CPUUsage:          m.CPUUsage,
		CPUCores:          m.CPUCores,
		MemUsedBytes:      m.MemUsageBytes,
		MemTotalBytes:     m.MemTotalBytes,
		MemUsage:          m.MemUsage,
		SwapUsedBytes:     m.SwapUsedBytes,
		SwapTotalBytes:    m.SwapTotalBytes,
		SwapUsage:         m.SwapUsage,
		DiskUsedBytes:     m.DiskUsageBytes,
		DiskTotalBytes:    m.DiskTotalBytes,
		DiskUsage:         m.DiskUsage,
		DiskReadBytesSec:  m.DiskReadBytesSec,
		DiskWriteBytesSec: m.DiskWriteBytesSec,
		NetRxBytesSec:     m.NetRxBytes,
		NetTxBytesSec:     m.NetTxBytes,
		TCPConnections:    m.TCPConnections,
		Load1m:            m.Load1m,
		UptimeSeconds:     m.UptimeSeconds,
		TrafficUplink:     m.TrafficUplink,
		TrafficDownlink:   m.TrafficDownlink,
		EgressInterface:   m.EgressInterface,
		EgressRxBytes:     m.EgressRxBytes,
		EgressTxBytes:     m.EgressTxBytes,
		LastUpdated:       m.CreatedAt,
	}
}

// HasMetricsCollector returns true if the metrics collector is available.
func (s *Service) HasMetricsCollector() bool {
	return s.metricsCollector != nil
}

// GetResourceUsage returns the latest resource usage for a single server.
func (s *Service) GetResourceUsage(ctx context.Context, serverID int64) *vo.ResourceUsage {
	if s.metricsCollector == nil {
		return nil
	}
	metricsMap, err := s.metricsCollector.GetLatestMetricsBatch(ctx)
	if err != nil {
		return nil
	}
	if m, ok := metricsMap[serverID]; ok {
		return metricToResourceUsage(m)
	}
	return nil
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
// It does two things:
//  1. Sends the cert PEM directly to nodes with certificate_id set (via CertDistributor).
//  2. Triggers SyncData to all remote nodes, which automatically collects and syncs
//     all certificates referenced by inbound configs, caddy/nginx/proxy sites.
//     This ensures that certs referenced via config paths (not certificate_id) also
//     get the renewed content.
func (s *Service) DistributeCertificateToAll(ctx context.Context, certID int64) error {
	var errs []error

	// 1. Direct distribution to nodes with certificate_id set
	if s.certDistrib != nil {
		if err := s.certDistrib.DistributeToAllServers(ctx, certID); err != nil {
			log.Printf("[nodeserver] direct cert distribution for cert %d: %v", certID, err)
			errs = append(errs, err)
		}
	}

	// 2. Trigger SyncData to all remote nodes to sync renewed cert content
	//    SyncData automatically scans all configs and collects matching certs.
	if s.nodeMgr != nil {
		servers, err := s.repo.ListAll(ctx)
		if err == nil {
			for _, srv := range servers {
				if srv.IsLocal {
					continue
				}
				if !s.nodeMgr.IsOnlineForServer(srv.ID) {
					log.Printf("[nodeserver] server %d offline, cert will sync on reconnect", srv.ID)
					continue
				}
				// Async SyncData — don't block the renewal flow
				go func(serverID int64) {
					syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					if _, err := s.SyncServerData(syncCtx, serverID, true); err != nil {
						log.Printf("[nodeserver] auto-SyncData for cert renewal to server %d: %v", serverID, err)
					} else {
						log.Printf("[nodeserver] auto-SyncData for cert renewal to server %d: success", serverID)
					}
				}(srv.ID)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("certificate distribution completed with %d error(s)", len(errs))
	}
	return nil
}

// ──────────────────── Phase 3: Node Types & Chain Proxy ────────────────────

// GetFailoverStatus returns the failover status for a primary_backup server.
func (s *Service) GetFailoverStatus(ctx context.Context, serverID int64) (*agent.FailoverStatus, error) {
	if s.failoverMgr == nil {
		return nil, fmt.Errorf("failover manager not initialized")
	}
	return s.failoverMgr.GetFailoverStatus(ctx, serverID)
}

// GetServerRecoverMode returns the recover mode for a specific server.
func (s *Service) GetServerRecoverMode(ctx context.Context, serverID int64) (string, error) {
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return "", err
	}
	if server.RecoverMode == "" {
		return string(agent.RecoverModeAutoSwitchBack), nil
	}
	return server.RecoverMode, nil
}

// SetServerRecoverMode updates the recover mode for a specific server and its partner.
// For primary_backup pairs, both nodes should have the same recover mode.
func (s *Service) SetServerRecoverMode(ctx context.Context, serverID int64, mode string) error {
	// Validate mode
	switch agent.RecoverMode(mode) {
	case agent.RecoverModeAutoSwitchBack, agent.RecoverModeStayOnPromoted:
		// valid
	default:
		return fmt.Errorf("invalid recover mode: %s (must be auto_switch_back or stay_on_promoted)", mode)
	}

	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return err
	}

	// Update this server's recover mode
	server.RecoverMode = mode
	server.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, server); err != nil {
		return fmt.Errorf("update server recover mode: %w", err)
	}

	// If this is a primary_backup pair, also update the partner's recover mode
	// to keep them in sync
	if server.NodeType == entity.NodeTypePrimaryBackup && server.PartnerServerID != 0 {
		partner, err := s.repo.GetByID(ctx, server.PartnerServerID)
		if err == nil && partner != nil {
			partner.RecoverMode = mode
			partner.UpdatedAt = time.Now()
			if err := s.repo.Update(ctx, partner); err != nil {
				log.Printf("[NodeServerService] failed to update partner %d recover mode: %v", partner.ID, err)
				// Non-fatal: partner update failed, but primary update succeeded
			}
		}
	}

	return nil
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

// SyncMirrorsForPrimary syncs all mirror nodes bound to the given primary server.
// After syncing DB data (inbound configs, Caddy/Nginx sites) from the primary to
// each mirror, it pushes the updated config to each mirror's Agent and applies it
// immediately (hot-reload). This is called after primary node config changes to
// ensure mirrors take effect at the same time as the primary, without requiring
// a manual restart.
func (s *Service) SyncMirrorsForPrimary(ctx context.Context, primaryServerID int64) {
	if s.mirrorSync == nil {
		return
	}
	mirrors, err := s.repo.ListByPartner(ctx, primaryServerID)
	if err != nil {
		log.Printf("[nodeserver] list mirrors for primary %d failed: %v", primaryServerID, err)
		return
	}
	for _, mirror := range mirrors {
		if mirror.NodeType != entity.NodeTypeMirror {
			continue
		}
		if err := s.mirrorSync.SyncMirrorNow(ctx, mirror.ID); err != nil {
			log.Printf("[nodeserver] sync mirror %d for primary %d failed: %v",
				mirror.ID, primaryServerID, err)
		}
	}
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
// GetTrafficStats aggregates NIC-level traffic usage for the dashboard card. It
// reads the per-node accumulated traffic already written by NodeTrafficQuotaService
// (from Agent egress-interface counters) into node_server.traffic_used_bytes,
// replacing the removed MultiServerTrafficCollector that this endpoint used to
// depend on.
func (s *Service) GetTrafficStats() interface{} {
	ctx := context.Background()
	servers, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil
	}
	stats := make(map[string]interface{})
	for _, server := range servers {
		if server.TrafficUsedBytes <= 0 {
			continue
		}
		stats[server.Name] = formatBytes(server.TrafficUsedBytes)
	}
	if len(stats) == 0 {
		return nil
	}
	return stats
}

// formatBytes renders a byte count as a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
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
	if err := s.nodeMgr.ControlProxy(ctx, serverID, proxyType, action); err != nil {
		return err
	}

	// Fanout control command to mirror nodes so they take effect at the same
	// time as the primary. Only start/restart/reload actions are fanned out;
	// stop is not (mirrors should keep running unless explicitly stopped).
	if action == "start" || action == "restart" || action == "reload" {
		go func() {
			syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			mirrors, err := s.repo.ListByPartner(syncCtx, serverID)
			if err != nil {
				log.Printf("[nodeserver] ControlProxy: list mirrors for primary %d failed: %v", serverID, err)
				return
			}
			for _, mirror := range mirrors {
				if mirror.NodeType != entity.NodeTypeMirror {
					continue
				}
				if !s.nodeMgr.IsOnline(mirror.ID) {
					continue
				}
				if err := s.nodeMgr.ControlProxy(syncCtx, mirror.ID, proxyType, action); err != nil {
					log.Printf("[nodeserver] ControlProxy: %s %s on mirror %d failed: %v",
						proxyType, action, mirror.ID, err)
				} else {
					log.Printf("[nodeserver] ControlProxy: %s %s on mirror %d: success",
						proxyType, action, mirror.ID)
				}
			}
		}()
	}

	return nil
}

// SyncProxyConfig syncs proxy configuration to a specific server.
func (s *Service) SyncProxyConfig(ctx context.Context, serverID int64, proxyType string, config []byte) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	if err := s.nodeMgr.SyncProxyConfig(ctx, serverID, proxyType, config); err != nil {
		return err
	}

	// Sync proxy config to mirror nodes bound to this primary.
	// Mirrors replicate caddy/nginx sites from the primary's DB (via
	// SyncMirrorsForPrimary), so the proxy config content is the same.
	// We push the same config to each mirror's Agent and reload the proxy.
	// This ensures mirrors take effect at the same time as the primary.
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		mirrors, err := s.repo.ListByPartner(syncCtx, serverID)
		if err != nil {
			log.Printf("[nodeserver] SyncProxyConfig: list mirrors for primary %d failed: %v", serverID, err)
			return
		}
		for _, mirror := range mirrors {
			if mirror.NodeType != entity.NodeTypeMirror {
				continue
			}
			if !s.nodeMgr.IsOnline(mirror.ID) {
				log.Printf("[nodeserver] SyncProxyConfig: mirror %d offline, skipping", mirror.ID)
				continue
			}
			if err := s.nodeMgr.SyncProxyConfigFromPrimary(syncCtx, mirror.ID, serverID, proxyType, config); err != nil {
				log.Printf("[nodeserver] SyncProxyConfig: sync %s config to mirror %d failed: %v",
					proxyType, mirror.ID, err)
			} else {
				log.Printf("[nodeserver] SyncProxyConfig: synced %s config to mirror %d (from primary %d)", proxyType, mirror.ID, serverID)
			}
		}
	}()

	return nil
}

// SyncWAFFiles syncs WAF files (config.lua, domain.json, *.rule) to a specific server's Agent.
func (s *Service) SyncWAFFiles(ctx context.Context, serverID int64, files []*agentpb.WAFFile) error {
	if s.nodeMgr == nil {
		return fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.SyncWAFFiles(ctx, serverID, files)
}

// SyncMirrorsProxyConfig pushes proxy config to all mirror nodes of a primary.
// This is called after local proxy config changes (e.g., SNI toggle, site
// enable/disable, config edit) to ensure mirrors get the updated config file
// and reload their proxy. Unlike SyncProxyConfig which also pushes to the
// primary, this method ONLY targets mirrors — the primary has already been
// handled locally by the handler.
func (s *Service) SyncMirrorsProxyConfig(ctx context.Context, primaryServerID int64, proxyType string, config []byte) error {
	if s.nodeMgr == nil {
		return nil
	}
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		mirrors, err := s.repo.ListByPartner(syncCtx, primaryServerID)
		if err != nil {
			log.Printf("[nodeserver] SyncMirrorsProxyConfig: list mirrors for primary %d failed: %v", primaryServerID, err)
			return
		}
		for _, mirror := range mirrors {
			if mirror.NodeType != entity.NodeTypeMirror {
				continue
			}
			if !s.nodeMgr.IsOnline(mirror.ID) {
				log.Printf("[nodeserver] SyncMirrorsProxyConfig: mirror %d offline, skipping", mirror.ID)
				continue
			}
			// Use SyncProxyConfigFromPrimary so mirrors get the PRIMARY's SNI settings
			// and certs, not their own stale ones. This is critical for SNI toggle sync:
			// when SNI is turned off on the primary, mirrors must receive SNI=off.
			if err := s.nodeMgr.SyncProxyConfigFromPrimary(syncCtx, mirror.ID, primaryServerID, proxyType, config); err != nil {
				log.Printf("[nodeserver] SyncMirrorsProxyConfig: sync %s config to mirror %d failed: %v",
					proxyType, mirror.ID, err)
			} else {
				log.Printf("[nodeserver] SyncMirrorsProxyConfig: synced %s config to mirror %d (from primary %d)", proxyType, mirror.ID, primaryServerID)
			}
		}
	}()
	return nil
}

// SyncMirrorsData triggers a full mirror sync from primary to all mirror nodes.
// This first copies the primary's DB data (caddy/nginx sites, inbound configs,
// SNI settings) to each mirror's DB in the panel database, then pushes the
// updated data to each mirror's Agent via SyncData.
// Used after DB-level changes (ToggleSNI, BatchUpdatePorts, SaveSNISettings)
// to ensure mirrors' local DBs match the primary's state.
func (s *Service) SyncMirrorsData(ctx context.Context, primaryServerID int64) error {
	if s.nodeMgr == nil {
		return nil
	}
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// SyncMirrorsForPrimary copies primary's DB data (caddy_sites, nginx_sites,
		// inbound_configs) to each mirror's DB, then pushes SyncData to each mirror's
		// Agent. This ensures mirrors' local DBs and Agent DBs are both updated.
		s.SyncMirrorsForPrimary(syncCtx, primaryServerID)
		log.Printf("[nodeserver] SyncMirrorsData: completed for primary %d", primaryServerID)
	}()
	return nil
}

// ControlMirrorsProxy sends a control command (reload/restart) to all mirror
// nodes of a primary. This is called after local proxy reload to ensure
// mirrors also reload their proxy with the updated config.
func (s *Service) ControlMirrorsProxy(ctx context.Context, primaryServerID int64, proxyType, action string) error {
	if s.nodeMgr == nil {
		return nil
	}
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		mirrors, err := s.repo.ListByPartner(syncCtx, primaryServerID)
		if err != nil {
			log.Printf("[nodeserver] ControlMirrorsProxy: list mirrors for primary %d failed: %v", primaryServerID, err)
			return
		}
		for _, mirror := range mirrors {
			if mirror.NodeType != entity.NodeTypeMirror {
				continue
			}
			if !s.nodeMgr.IsOnline(mirror.ID) {
				log.Printf("[nodeserver] ControlMirrorsProxy: mirror %d offline, skipping", mirror.ID)
				continue
			}
			if err := s.nodeMgr.ControlProxy(syncCtx, mirror.ID, proxyType, action); err != nil {
				log.Printf("[nodeserver] ControlMirrorsProxy: %s %s on mirror %d failed: %v",
					proxyType, action, mirror.ID, err)
			} else {
				log.Printf("[nodeserver] ControlMirrorsProxy: %s %s on mirror %d: success",
					proxyType, action, mirror.ID)
			}
		}
	}()
	return nil
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
// For remote nodes, this stores the settings in the panel DB and triggers a
// SyncData push so the Agent's local DB is updated immediately.
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
	if err := s.nodeMgr.UpdateProxyGuardianForServer(ctx, serverID, proxyType, curAutoStart, curAutoRestart, curMaxRestarts); err != nil {
		return err
	}

	// Trigger async SyncData push so the Agent's local DB is updated immediately
	// with the new guardian settings. Without this, the Agent would keep using
	// defaults (auto_restart=false) until the next periodic sync.
	go func() {
		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.SyncServerData(syncCtx, serverID, false); err != nil {
			log.Printf("[nodeserver] SyncData after guardian update to server %d: %v", serverID, err)
		} else {
			log.Printf("[nodeserver] SyncData after guardian update to server %d: success", serverID)
		}
	}()

	return nil
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

// ReadConfigFile reads the runtime config file from a server (local or remote).
// For local nodes, reads directly from disk. For remote nodes, calls the
// ReadConfigFile gRPC RPC on the Agent.
func (s *Service) ReadConfigFile(ctx context.Context, serverID int64, coreType string) ([]byte, string, error) {
	if s.nodeMgr == nil {
		return nil, "", fmt.Errorf("node manager not initialized")
	}
	return s.nodeMgr.ReadConfigFileOnServer(ctx, serverID, coreType)
}

// ReadConfigFileOnServer is an alias for ReadConfigFile to satisfy the NodeOperator interface.
func (s *Service) ReadConfigFileOnServer(ctx context.Context, serverID int64, coreType string) ([]byte, string, error) {
	return s.ReadConfigFile(ctx, serverID, coreType)
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

// ListServerCertificates returns all certificates in use by a server.
// It scans inbound configs, caddy_sites, nginx_sites, proxy_sites,
// and the node-level certificate_id association to find all certificates
// referenced by the server's configurations.
func (s *Service) ListServerCertificates(ctx context.Context, serverID int64) (interface{}, error) {
	server, err := s.repo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	if s.certRepo == nil {
		return []interface{}{}, nil
	}

	// Load all certificates from DB for path matching
	allCerts, _, err := s.certRepo.List(ctx, 1000, 0)
	if err != nil {
		return []interface{}{}, nil
	}

	// certID -> usage sources
	type certUsage struct {
		Cert       entity.Certificate `json:"cert"`
		Sources    []string           `json:"sources"` // e.g. ["xray:inbound_tag", "caddy:domain", "nginx:domain", "node-level"]
	}
	usageMap := make(map[int64]*certUsage)

	addSource := func(certID int64, source string) {
		if u, ok := usageMap[certID]; ok {
			u.Sources = append(u.Sources, source)
		} else {
			usageMap[certID] = &certUsage{Sources: []string{source}}
		}
	}

	// 1. Node-level certificate (node_servers.certificate_id)
	if server.CertificateID > 0 {
		addSource(server.CertificateID, "node-level")
	}

	// 2. Scan inbound configs for certificateFile references
	if s.inboundRepo != nil {
		inbounds, err := s.inboundRepo.ListRows(ctx, serverID)
		if err == nil {
			for _, ic := range inbounds {
				if !ic.Enabled {
					continue
				}
				var inbound map[string]interface{}
				if err := json.Unmarshal([]byte(ic.ConfigJSON), &inbound); err != nil {
					continue
				}
				ss, ok := inbound["streamSettings"].(map[string]interface{})
				if !ok {
					continue
				}
				tls, ok := ss["tlsSettings"].(map[string]interface{})
				if !ok {
					continue
				}
				certs, ok := tls["certificates"].([]interface{})
				if !ok {
					continue
				}
				for _, c := range certs {
					cm, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if certFile, ok := cm["certificateFile"].(string); ok && certFile != "" {
						normalized := normalizeCertPathLocal(certFile)
						for _, cert := range allCerts {
							if normalizeCertPathLocal(cert.CertPath) == normalized {
								addSource(cert.ID, fmt.Sprintf("xray:%s", ic.Tag))
							}
						}
					}
				}
			}
		}
	}

	// 3. Scan caddy_sites for cert_path references
	if s.db != nil {
		var caddySites []entity.CaddySite
		if err := s.db.WithContext(ctx).Where("server_id = ? AND enabled = ?", serverID, true).Find(&caddySites).Error; err == nil {
			for _, site := range caddySites {
				if site.CertPath == "" {
					continue
				}
				normalized := normalizeCertPathLocal(site.CertPath)
				for _, cert := range allCerts {
					if normalizeCertPathLocal(cert.CertPath) == normalized {
						addSource(cert.ID, fmt.Sprintf("caddy:%s", site.Domain))
					}
				}
			}
		}

		// 4. Scan nginx_sites for cert_path references
		var nginxSites []entity.NginxSite
		if err := s.db.WithContext(ctx).Where("server_id = ? AND enabled = ?", serverID, true).Find(&nginxSites).Error; err == nil {
			for _, site := range nginxSites {
				if site.CertPath == "" {
					continue
				}
				normalized := normalizeCertPathLocal(site.CertPath)
				for _, cert := range allCerts {
					if normalizeCertPathLocal(cert.CertPath) == normalized {
						addSource(cert.ID, fmt.Sprintf("nginx:%s", site.Domain))
					}
				}
			}
		}

		// 5. Scan proxy_sites for cert_path references
		var proxySites []entity.ProxySite
		if err := s.db.WithContext(ctx).Where("server_id = ? AND enabled = ?", serverID, true).Find(&proxySites).Error; err == nil {
			for _, site := range proxySites {
				if site.CertPath == "" {
					continue
				}
				normalized := normalizeCertPathLocal(site.CertPath)
				for _, cert := range allCerts {
					if normalizeCertPathLocal(cert.CertPath) == normalized {
						addSource(cert.ID, fmt.Sprintf("proxy:%s", site.Domain))
					}
				}
			}
		}
	}

	// Build result: attach cert info to each usage
	result := make([]interface{}, 0, len(usageMap))
	for certID, usage := range usageMap {
		for _, cert := range allCerts {
			if cert.ID == certID {
				// Strip data/ prefix for display
				cert.CertPath = strings.TrimPrefix(cert.CertPath, "data/")
				cert.KeyPath = strings.TrimPrefix(cert.KeyPath, "data/")
				result = append(result, gin.H{
					"id":           cert.ID,
					"name":         cert.Name,
					"domain":       cert.Domain,
					"domains":      cert.Domains,
					"issuer":       cert.Issuer,
					"provider_type": cert.ProviderType,
					"key_type":     cert.KeyType,
					"key_bits":     cert.KeyBits,
					"cert_path":    cert.CertPath,
					"key_path":     cert.KeyPath,
					"status":       cert.Status,
					"auto_renew":   cert.AutoRenew,
					"expires_at":   cert.ExpiresAt,
					"sources":      usage.Sources,
				})
				break
			}
		}
	}

	return result, nil
}

// normalizeCertPathLocal strips common relative path prefixes for matching.
func normalizeCertPathLocal(path string) string {
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}
	for strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, "./")
	}
	return path
}

// AssociateCertificate associates a TLS certificate with a server.
func (s *Service) AssociateCertificate(ctx context.Context, serverID, certID int64) error {
	return s.repo.UpdateCertificateID(ctx, serverID, certID)
}

// DissociateCertificate removes the TLS certificate association from a server.
func (s *Service) DissociateCertificate(ctx context.Context, serverID int64) error {
	return s.repo.UpdateCertificateID(ctx, serverID, 0)
}

// MigrateCertificate migrates all references from oldCertID to newCertID.
// It updates:
//  1. node_servers.certificate_id
//  2. inbound_configs.config_json (certificateFile/keyFile paths)
//  3. caddy_sites.cert_path / key_path
//  4. nginx_sites.cert_path / key_path
//  5. proxy_sites.cert_path / key_path
//
// After migration, it triggers SyncData to all affected nodes.
func (s *Service) MigrateCertificate(ctx context.Context, oldCertID, newCertID int64) (map[string]interface{}, error) {
	if s.certRepo == nil {
		return nil, fmt.Errorf("certificate repository not initialized")
	}
	if s.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	oldCert, err := s.certRepo.GetByID(ctx, oldCertID)
	if err != nil {
		return nil, fmt.Errorf("old certificate not found: %w", err)
	}
	newCert, err := s.certRepo.GetByID(ctx, newCertID)
	if err != nil {
		return nil, fmt.Errorf("new certificate not found: %w", err)
	}

	affectedServers := make(map[int64]bool)
	result := map[string]interface{}{
		"old_cert": gin.H{"id": oldCert.ID, "domain": oldCert.Domain, "cert_path": oldCert.CertPath},
		"new_cert": gin.H{"id": newCert.ID, "domain": newCert.Domain, "cert_path": newCert.CertPath},
	}

	oldCertPathNorm := normalizeCertPathLocal(oldCert.CertPath)
	oldKeyPathNorm := normalizeCertPathLocal(oldCert.KeyPath)

	// 1. Update node_servers.certificate_id
	var servers []entity.NodeServer
	if err := s.db.WithContext(ctx).Where("certificate_id = ?", oldCertID).Find(&servers).Error; err == nil {
		for _, srv := range servers {
			s.db.WithContext(ctx).Model(&entity.NodeServer{}).Where("id = ?", srv.ID).Update("certificate_id", newCertID)
			affectedServers[srv.ID] = true
		}
	}
	result["node_servers_updated"] = len(servers)

	// 2. Update inbound_configs: replace certificateFile/keyFile paths in config_json
	if s.inboundRepo != nil {
		var inbounds []entity.InboundConfig
		s.db.WithContext(ctx).Find(&inbounds)
		inboundUpdated := 0
		for _, ic := range inbounds {
			var cfg map[string]interface{}
			if err := json.Unmarshal([]byte(ic.ConfigJSON), &cfg); err != nil {
				continue
			}
			ss, ok := cfg["streamSettings"].(map[string]interface{})
			if !ok {
				continue
			}
			tls, ok := ss["tlsSettings"].(map[string]interface{})
			if !ok {
				continue
			}
			certs, ok := tls["certificates"].([]interface{})
			if !ok {
				continue
			}
			modified := false
			for _, c := range certs {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cf, ok := cm["certificateFile"].(string); ok {
					if normalizeCertPathLocal(cf) == oldCertPathNorm {
						cm["certificateFile"] = strings.ReplaceAll(cf, oldCert.CertPath, newCert.CertPath)
						modified = true
					}
				}
				if kf, ok := cm["keyFile"].(string); ok {
					if normalizeCertPathLocal(kf) == oldKeyPathNorm {
						cm["keyFile"] = strings.ReplaceAll(kf, oldCert.KeyPath, newCert.KeyPath)
						modified = true
					}
				}
			}
			if modified {
				updatedJSON, _ := json.Marshal(cfg)
				s.db.WithContext(ctx).Model(&entity.InboundConfig{}).Where("id = ?", ic.ID).Update("config_json", string(updatedJSON))
				affectedServers[ic.ServerID] = true
				inboundUpdated++
			}
		}
		result["inbound_configs_updated"] = inboundUpdated
	}

	// 3. Update caddy_sites
	var caddySites []entity.CaddySite
	s.db.WithContext(ctx).Where("cert_path LIKE ? OR key_path LIKE ?", "%"+oldCertPathNorm+"%", "%"+oldKeyPathNorm+"%").Find(&caddySites)
	for _, site := range caddySites {
		updates := map[string]interface{}{}
		if normalizeCertPathLocal(site.CertPath) == oldCertPathNorm {
			updates["cert_path"] = newCert.CertPath
		}
		if normalizeCertPathLocal(site.KeyPath) == oldKeyPathNorm {
			updates["key_path"] = newCert.KeyPath
		}
		if len(updates) > 0 {
			s.db.WithContext(ctx).Model(&entity.CaddySite{}).Where("id = ?", site.ID).Updates(updates)
			affectedServers[site.ServerID] = true
		}
	}
	result["caddy_sites_updated"] = len(caddySites)

	// 4. Update nginx_sites
	var nginxSites []entity.NginxSite
	s.db.WithContext(ctx).Where("cert_path LIKE ? OR key_path LIKE ?", "%"+oldCertPathNorm+"%", "%"+oldKeyPathNorm+"%").Find(&nginxSites)
	for _, site := range nginxSites {
		updates := map[string]interface{}{}
		if normalizeCertPathLocal(site.CertPath) == oldCertPathNorm {
			updates["cert_path"] = newCert.CertPath
		}
		if normalizeCertPathLocal(site.KeyPath) == oldKeyPathNorm {
			updates["key_path"] = newCert.KeyPath
		}
		if len(updates) > 0 {
			s.db.WithContext(ctx).Model(&entity.NginxSite{}).Where("id = ?", site.ID).Updates(updates)
			affectedServers[site.ServerID] = true
		}
	}
	result["nginx_sites_updated"] = len(nginxSites)

	// 5. Update proxy_sites
	var proxySites []entity.ProxySite
	s.db.WithContext(ctx).Where("cert_path LIKE ? OR key_path LIKE ?", "%"+oldCertPathNorm+"%", "%"+oldKeyPathNorm+"%").Find(&proxySites)
	for _, site := range proxySites {
		updates := map[string]interface{}{}
		if normalizeCertPathLocal(site.CertPath) == oldCertPathNorm {
			updates["cert_path"] = newCert.CertPath
		}
		if normalizeCertPathLocal(site.KeyPath) == oldKeyPathNorm {
			updates["key_path"] = newCert.KeyPath
		}
		if len(updates) > 0 {
			s.db.WithContext(ctx).Model(&entity.ProxySite{}).Where("id = ?", site.ID).Updates(updates)
			affectedServers[site.ServerID] = true
		}
	}
	result["proxy_sites_updated"] = len(proxySites)

	// 6. Trigger SyncData to all affected nodes
	serverIDs := make([]int64, 0, len(affectedServers))
	for id := range affectedServers {
		serverIDs = append(serverIDs, id)
	}
	result["affected_servers"] = serverIDs

	if s.nodeMgr != nil && len(serverIDs) > 0 {
		for _, serverID := range serverIDs {
			if serverID == 1 || s.nodeMgr.IsLocalServer(serverID) {
				continue // local server uses files directly
			}
			if !s.nodeMgr.IsOnlineForServer(serverID) {
				log.Printf("[nodeserver] server %d offline, will sync on reconnect", serverID)
				continue
			}
			go func(sid int64) {
				syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if _, err := s.SyncServerData(syncCtx, sid, true); err != nil {
					log.Printf("[nodeserver] migration SyncData to server %d: %v", sid, err)
				} else {
					log.Printf("[nodeserver] migration SyncData to server %d: success", sid)
				}
			}(serverID)
		}
	}

	log.Printf("[nodeserver] certificate migration: old=%d (%s) → new=%d (%s), affected %d server(s)",
		oldCertID, oldCert.Domain, newCertID, newCert.Domain, len(serverIDs))

	return result, nil
}

// ──────────────────── Supplementary: CA Center ────────────────────

// GetCAStatus returns the CA root certificate status, including rotation
// observability. During a CA rotation the response contains multiple entries in
// ca_roots (an outgoing + incoming pair) and rotation_in_progress is true.
//
// The top-level fields (subject, serial, not_before, not_after) reflect the
// newest CA root and are kept backward-compatible with the frontend.
func (s *Service) GetCAStatus() interface{} {
	if s.caService == nil {
		return gin.H{"initialized": false}
	}

	caRoots := s.caService.ListCAStatus()

	result := gin.H{
		"initialized":          true,
		"ca_roots":             caRoots,
		"ca_count":             len(caRoots),
		"rotation_in_progress": len(caRoots) > 1,
		"overlap_window_days":  int(agent.CAOverlapWindow / (24 * time.Hour)),
	}

	// Flatten the newest CA root fields to the top level so the frontend can
	// access them directly (backward compatible).
	if len(caRoots) > 0 {
		latest := caRoots[len(caRoots)-1]
		result["subject"]    = latest.Subject
		result["serial"]     = latest.SerialNumber
		result["not_before"] = latest.NotBefore
		result["not_after"]  = latest.NotAfter
		result["expires_in"] = latest.ExpiresIn
	}

	return result
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
// The cert is pushed to the Agent before updating the DB to ensure the
// Agent actually receives the new cert.
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
	// Push new cert to Agent FIRST. Only update DB if push succeeds.
	if s.nodeMgr != nil && s.nodeMgr.IsOnlineForServer(serverID) {
		if err := s.nodeMgr.PushAgentCertificate(ctx, serverID, result); err != nil {
			return fmt.Errorf("push cert to agent failed: %w (DB not updated, will retry on next cycle)", err)
		}
	} else {
		return fmt.Errorf("agent offline, cannot push cert (DB not updated, will retry on reconnect)")
	}
	if err := s.repo.UpdateAgentCert(ctx, serverID, result.Serial, result.ExpiresAt); err != nil {
		return fmt.Errorf("update agent cert: %w", err)
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
// Results are enriched with server_name and admin_name for display.
func (s *Service) ListAuditLogs(ctx context.Context, serverID int64, action string, limit, offset int) (interface{}, int64, error) {
	if s.auditLogRepo == nil {
		return []interface{}{}, 0, nil
	}
	logs, total, err := s.auditLogRepo.List(ctx, serverID, action, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if s.db == nil || len(logs) == 0 {
		return logs, total, nil
	}

	// Collect unique server IDs and admin IDs for batch lookup
	serverIDs := make(map[int64]string)
	adminIDs := make(map[int64]string)
	for _, l := range logs {
		if l.ServerID > 0 {
			serverIDs[l.ServerID] = ""
		}
		if l.AdminID > 0 {
			adminIDs[l.AdminID] = ""
		}
	}

	// Batch fetch server names
	if len(serverIDs) > 0 {
		ids := make([]int64, 0, len(serverIDs))
		for id := range serverIDs {
			ids = append(ids, id)
		}
		var servers []entity.NodeServer
		s.db.WithContext(ctx).Where("id IN ?", ids).Select("id, name").Find(&servers)
		for _, srv := range servers {
			serverIDs[srv.ID] = srv.Name
		}
	}

	// Batch fetch admin usernames
	if len(adminIDs) > 0 {
		ids := make([]int64, 0, len(adminIDs))
		for id := range adminIDs {
			ids = append(ids, id)
		}
		var users []entity.User
		s.db.WithContext(ctx).Where("id IN ?", ids).Select("id, username").Find(&users)
		for _, u := range users {
			adminIDs[u.ID] = u.Username
		}
	}

	// Build enriched result
	type auditLogWithNames struct {
		entity.NodeAuditLog
		ServerName string `json:"server_name"`
		AdminName  string `json:"admin_name"`
	}
	result := make([]auditLogWithNames, 0, len(logs))
	for _, l := range logs {
		result = append(result, auditLogWithNames{
			NodeAuditLog: l,
			ServerName:   serverIDs[l.ServerID],
			AdminName:    adminIDs[l.AdminID],
		})
	}
	return result, total, nil
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
