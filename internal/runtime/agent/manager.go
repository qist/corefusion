package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/store/repository"
)

// NodeServerManager manages all node server Agent connections.
// It acts as the gRPC server for Agent→Panel communication (HeartbeatService,
// RegistrationService) and as the gRPC client for Panel→Agent commands (AgentService).
//
// NodeServerManager implements NodeOperator interface:
//   - For local servers (is_local=1): delegates to LocalAgent, which directly
//     reuses the existing process.Manager + healthcheck.Guardian.
//   - For remote servers: sends gRPC commands to the remote Agent binary.
type NodeServerManager struct {
	mu          sync.RWMutex
	connections map[int64]*AgentConnection // server_id → connection
	serverRepo  *repository.NodeServerRepository
	ca          *CertificateAuthority
	crl         *CertRevocationList // optional CRL for cert revocation checks
	localAgent  *LocalAgent          // local node adapter (wraps process.Manager)
	settingStore NodeSettingStore    // optional, for GitHub accel/proxy settings

	// Cached local server ID — loaded once at startup to avoid DB queries
	// on every NodeOperator method call. 0 means not yet initialized.
	localServerID int64

	// gRPC server (listens for Agent connections)
	grpcServer *grpc.Server
	grpcAddr   string

	// Event channels for consumers (traffic, status, etc.)
	onTraffic   chan *TrafficEvent
	onStatusChg chan *StatusEvent

	// Phase 6: Metrics collector (optional, injected by router)
	metricsCollector *MetricsCollector

	// Phase 6b: Traffic quota service (optional, injected by router)
	trafficQuotaSvc TrafficQuotaProcessor

	// Pending task results — taskID → result channel
	pendingTasks   map[string]chan *agentpb.TaskResult
	pendingTasksMu sync.RWMutex

	// 实时在线订阅人快照（由 Agent ONLINE_USERS 上报刷新，带 TTL 过期）
	onlineStore *NodeOnlineStore

	// Data sync provider: reads panel DB tables to build SyncDataRequest
	dataSyncProvider DataSyncProvider

	// Proxy cert collector: collects TLS certs for proxy sites (Caddy/Nginx)
	proxyCertCollector ProxyCertCollector

	// Proxy settings provider: fetches per-node nginx/caddy settings
	proxySettingsProvider ProxySettingsProvider

	// Cert renewer: checks and renews Agent certs on reconnect
	certRenewer *CertRenewer

	// Agent updater: tracks pending Agent binary updates and verifies
	// version on reconnect. Set via SetAgentUpdater.
	agentUpdater *AgentUpdater

	// Rate limit syncer: syncs rate limit config to Agent on reconnect
	rateLimitSyncer RateLimitSyncer

	// WAF syncer: syncs WAF rules/configs to Agent on reconnect
	wafSyncer WAFSyncer

	// GeoRule syncer: pushes geo rule files (geoip.dat, geosite.dat) to Agent on reconnect
	geoRuleSyncer GeoRuleSyncer
}

// WAFSyncer syncs WAF files (rules, configs, domain.json) to a remote Agent.
// Implemented by the WAF service via an adapter in router.go.
// On Agent reconnect, this ensures the remote node has the latest waf-rules/ directory.
type WAFSyncer interface {
	SyncToNode(ctx context.Context, serverID int64) error
}

// GeoRuleSyncer pushes geo rule files to a remote Agent on reconnect.
// Implemented by the georule service.
type GeoRuleSyncer interface {
	SyncToNode(ctx context.Context, serverID int64) error
}

// NodeSettingStore provides access to system settings for GitHub acceleration
// and proxy guardian settings.
// Implemented by systemSettingService.
type NodeSettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// SetSettingStore sets the setting store for GitHub acceleration config.
func (m *NodeServerManager) SetSettingStore(s NodeSettingStore) {
	m.settingStore = s
}

// githubAccelConfig reads the GitHub acceleration domain and proxy URL from settings.
func (m *NodeServerManager) githubAccelConfig() (accelDomain, proxyURL string) {
	if m.settingStore == nil {
		return "", ""
	}
	val, err := m.settingStore.GetByKey(context.Background(), "github.acceleration")
	if err != nil || val == "" {
		return "", ""
	}
	var cfg struct {
		Enabled bool   `json:"enabled"`
		ProxyURL string `json:"proxy_url"`
		Domains []struct {
			URL     string `json:"url"`
			Enabled bool   `json:"enabled"`
		} `json:"domains"`
	}
	if json.Unmarshal([]byte(val), &cfg) != nil {
		return "", ""
	}
	if cfg.Enabled {
		for _, d := range cfg.Domains {
			if d.Enabled && d.URL != "" {
				accelDomain = d.URL
				break
			}
		}
	}
	proxyURL = cfg.ProxyURL
	return
}

// SetLocalAgent sets the local agent adapter.
// The LocalAgent wraps the existing process.Manager and provides local-only
// implementations of NodeOperator methods without going through gRPC.
func (m *NodeServerManager) SetLocalAgent(la *LocalAgent) {
	m.localAgent = la
}

// InitLocalServerID loads the local server ID from DB and caches it.
// Call this once at startup to avoid repeated DB queries in isLocalServer().
func (m *NodeServerManager) InitLocalServerID(ctx context.Context) {
	if m.serverRepo == nil {
		return
	}
	servers, err := m.serverRepo.ListLocal(ctx)
	if err == nil && len(servers) > 0 {
		m.localServerID = servers[0].ID
		log.Printf("[NodeServerManager] cached local server ID: %d", m.localServerID)
	}
}

// DataSyncProvider reads panel database tables and builds SyncDataRequest
// for pushing configuration data to remote Agents.
type DataSyncProvider interface {
	BuildSyncDataRequest(ctx context.Context, serverID int64) (*agentpb.SyncDataRequest, error)
}

// ProxyCertCollector collects TLS certificates used by proxy sites (Caddy/Nginx)
// for a specific server. This is used when syncing proxy configs to remote agents
// so that certificate files are landed on the agent's disk alongside the config.
type ProxyCertCollector interface {
	CollectProxyCertificates(ctx context.Context, serverID int64) []*agentpb.CertificateEntry
}

// ProxySettingsProvider provides nginx/caddy settings for remote nodes.
// Implemented by handlers to fetch per-node settings from database.
type ProxySettingsProvider interface {
	GetNginxSettingsForServer(ctx context.Context, serverID int64) (string, error)
	GetCaddySNISettingsForServer(ctx context.Context, serverID int64) (string, error)
}

// SetDataSyncProvider sets the data sync provider for SyncData operations.
func (m *NodeServerManager) SetDataSyncProvider(p DataSyncProvider) {
	m.dataSyncProvider = p
}

// SetProxyCertCollector sets the proxy certificate collector for SyncProxyConfig operations.
func (m *NodeServerManager) SetProxyCertCollector(c ProxyCertCollector) {
	m.proxyCertCollector = c
}

// SetProxySettingsProvider sets the proxy settings provider for SyncProxyConfig operations.
func (m *NodeServerManager) SetProxySettingsProvider(p ProxySettingsProvider) {
	m.proxySettingsProvider = p
}

// SetMetricsCollector sets the metrics collector for heartbeat-based metric collection.
func (m *NodeServerManager) SetMetricsCollector(mc *MetricsCollector) {
	m.metricsCollector = mc
}

// SetTrafficQuotaProcessor sets the traffic quota processor for egress interface monitoring.
func (m *NodeServerManager) SetTrafficQuotaProcessor(p TrafficQuotaProcessor) {
	m.trafficQuotaSvc = p
}

// SetAgentUpdater sets the Agent updater for tracking pending Agent binary updates.
// When an Agent reconnects with a new version, the updater verifies and marks the
// update task as done (or failed on version mismatch).
func (m *NodeServerManager) SetAgentUpdater(u *AgentUpdater) {
	m.agentUpdater = u
}

// RateLimitSyncer syncs rate limit config to Agent on reconnect.
type RateLimitSyncer interface {
	SyncConfigToAgent(ctx context.Context, serverID int64) error
}

// SetRateLimitSyncer sets the rate limit syncer for pushing rate limit config on Agent reconnect.
func (m *NodeServerManager) SetRateLimitSyncer(s RateLimitSyncer) {
	m.rateLimitSyncer = s
}

// SetWAFSyncer sets the WAF syncer for pushing WAF rules/configs on Agent reconnect.
// On reconnect, the remote node gets all WAF files (rules, config.lua, config.json, domain.json)
// pushed to its proxy/waf-rules/ directory.
func (m *NodeServerManager) SetWAFSyncer(s WAFSyncer) {
	m.wafSyncer = s
}

// SetGeoRuleSyncer sets the geo rule syncer for pushing geo rule files on Agent reconnect.
// On reconnect, the remote node gets all geo rule files (geoip.dat, geosite.dat, etc.)
// pushed to its data/rules/ directory via the panel's HTTP download endpoint.
func (m *NodeServerManager) SetGeoRuleSyncer(s GeoRuleSyncer) {
	m.geoRuleSyncer = s
}

// isLocalServer checks if a server is the local panel server.
// Uses the cached localServerID when available (set by InitLocalServerID),
// avoiding a DB query on every NodeOperator method call.
func (m *NodeServerManager) isLocalServer(serverID int64) bool {
	if serverID == 0 {
		return true
	}
	if m.localServerID > 0 {
		return serverID == m.localServerID
	}
	// Fallback: query DB once (shouldn't happen after InitLocalServerID is called)
	server, err := m.serverRepo.GetByID(context.Background(), serverID)
	if err != nil {
		return false
	}
	return server.IsLocal
}

// IsLocalServer is the exported version of isLocalServer.
func (m *NodeServerManager) IsLocalServer(serverID int64) bool {
	return m.isLocalServer(serverID)
}

// AgentConnection represents an active connection to a remote Agent.
type AgentConnection struct {
	mu          sync.Mutex
	ServerID    int64
	Hostname    string

	// agentVersion is the version reported by the Agent in its last heartbeat.
	// Tracked to detect version changes (e.g. after a self-update).
	agentVersion string

	// Bidirectional stream (Agent→Panel heartbeat)
	stream agentpb.HeartbeatService_StreamServer

	// State
	lastHeartbeat time.Time
	online        bool

	// Cached core states from last status report
	coreStates     map[string]*agentpb.CoreStatus
	statusReported bool // true after first STATUS report received

	// Cached binary info from last ListBinaries query
	cachedBinaries     []*agentpb.BinaryInfo
	cachedBinariesAt   time.Time

	// Cached OS/arch/distro from heartbeat (for asset filtering)
	cachedOS     string
	cachedArch   string
	cachedDistro string
}

// LastHeartbeatTime returns the last heartbeat timestamp.
func (c *AgentConnection) LastHeartbeatTime() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastHeartbeat
}

// GetCoreStates returns a copy of the cached core states.
func (c *AgentConnection) GetCoreStates() map[string]*agentpb.CoreStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]*agentpb.CoreStatus, len(c.coreStates))
	for k, v := range c.coreStates {
		result[k] = v
	}
	return result
}

// IsOnline returns whether the connection is currently online.
func (c *AgentConnection) IsOnline() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.online
}

// TrafficEvent represents a traffic report from an Agent.
type TrafficEvent struct {
	ServerID  int64
	Email     string
	Uplink    int64
	Downlink  int64
	CoreType  string
	Timestamp time.Time
}

// StatusEvent represents a status change event from an Agent.
type StatusEvent struct {
	ServerID int64
	Online   bool
	Cores    []*agentpb.CoreStatus
}

// NewNodeServerManager creates a new NodeServerManager.
func NewNodeServerManager(serverRepo *repository.NodeServerRepository, ca *CertificateAuthority) *NodeServerManager {
	return &NodeServerManager{
		connections: make(map[int64]*AgentConnection),
		serverRepo:  serverRepo,
		ca:          ca,
		onTraffic:    make(chan *TrafficEvent, 1024),
		onStatusChg:  make(chan *StatusEvent, 256),
		pendingTasks: make(map[string]chan *agentpb.TaskResult),
		onlineStore: NewNodeOnlineStore(60),
	}
}

// WithCRL sets the CRL for certificate revocation checking.
func (m *NodeServerManager) WithCRL(crl *CertRevocationList) *NodeServerManager {
	m.crl = crl
	return m
}

// WithCertRenewer sets the cert renewer for on-reconnect certificate renewal.
func (m *NodeServerManager) WithCertRenewer(r *CertRenewer) *NodeServerManager {
	m.certRenewer = r
	return m
}

// StartGRPCServer starts the gRPC server that Agents connect to.
// The server uses mTLS with the internal CA.
func (m *NodeServerManager) StartGRPCServer(ctx context.Context, addr string) error {
	if m.ca == nil {
		return fmt.Errorf("CA not initialized")
	}

	// Issue a server certificate for the Panel's gRPC server (presented to Agents during TLS handshake)
	serverCert, err := m.ca.IssueServerCertificate()
	if err != nil {
		return fmt.Errorf("issue server certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		// Trust all valid CA roots (both the outgoing and incoming CA during a
		// rotation) so Agents keep working while their short-lived certificates
		// re-enroll onto the new CA.
		ClientCAs: m.ca.ClientCAPool(),
	}

	// Add CRL verification if available
	if m.crl != nil {
		tlsConfig.VerifyPeerCertificate = m.crl.VerifyCallback()
	}

	creds := credentials.NewTLS(tlsConfig)

	// 启用 keepalive enforcement：允许 Agent 客户端每 5 秒发送一次 PING，
	// 配合 Agent 侧的 keepalive 参数，使静默断开的 TCP 连接能在 ~15 秒内被检测到。
	m.grpcServer = grpc.NewServer(
		grpc.Creds(creds),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	// Register HeartbeatService (Agent→Panel bidirectional stream)
	agentpb.RegisterHeartbeatServiceServer(m.grpcServer, &heartbeatServiceImpl{mgr: m})

	// Register RegistrationService (Agent Bootstrap, runs without mTLS on separate port or same port)
	// Note: RegistrationService needs to be accessible without mTLS for first-time agents.
	// In production, this would run on a separate listener. For now, we register it here
	// and agents with bootstrap tokens use a separate insecure connection.
	agentpb.RegisterRegistrationServiceServer(m.grpcServer, &registrationServiceImpl{mgr: m})

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	m.grpcAddr = addr

	go func() {
		log.Printf("[NodeServerManager] gRPC server listening on %s (mTLS)", addr)
		if err := m.grpcServer.Serve(lis); err != nil {
			log.Printf("[NodeServerManager] gRPC server error: %v", err)
		}
	}()

	return nil
}

// ============================================================
// NodeOperator interface implementation
// ============================================================

// SyncConfig syncs core configuration to a server.
// For local servers, delegates to LocalAgent (writes file directly).
// For remote servers, sends config via gRPC to the Agent.
func (m *NodeServerManager) SyncConfig(ctx context.Context, serverID int64, coreType string, config []byte, version int64) error {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return fmt.Errorf("local agent not configured")
		}
		return m.localAgent.SyncConfig(ctx, coreType, config, version)
	}
	return m.SyncConfigToServer(ctx, serverID, coreType, config, version)
}

// ControlCore controls a core process on a server.
// For local servers, delegates to LocalAgent → process.Manager.Start/Stop/Restart.
// For remote servers, sends control command via gRPC to the Agent.
func (m *NodeServerManager) ControlCore(ctx context.Context, serverID int64, coreType, action string) error {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ControlCore(ctx, coreType, action)
	}
	return m.ControlCoreOnServer(ctx, serverID, coreType, action)
}

// AlterUsers alters users on a server (hot reload, no core restart).
// For local servers, delegates to LocalAgent → xray gRPC AlterInbound.
// For remote servers, sends AlterUsers via gRPC to the Agent.
func (m *NodeServerManager) AlterUsers(ctx context.Context, serverID int64, req *agentpb.AlterUsersRequest) error {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return fmt.Errorf("local agent not configured")
		}
	return m.localAgent.AlterUsers(ctx, req)
	}
	return m.AlterUsersOnServer(ctx, serverID, req)
}

// ApplyLimits applies rate limiting rules on a server.
// For local servers, delegates to LocalAgent → firewall/nftables.
// For remote servers, sends ApplyLimits via gRPC to the Agent.
func (m *NodeServerManager) ApplyLimits(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return fmt.Errorf("local agent not configured")
		}
	return m.localAgent.ApplyLimits(ctx, req)
	}
	return m.ApplyLimitsOnServer(ctx, serverID, req)
}

// GetStatusForServer queries the status of a server's cores.
// For local servers, delegates to LocalAgent → process.Manager.ListAll.
// For remote servers, queries via gRPC to the Agent.
func (m *NodeServerManager) GetStatusForServer(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.GetStatus(ctx)
	}
	return m.GetServerStatus(ctx, serverID)
}

// IsOnlineForServer checks if a server is online.
// Local servers are always online.
func (m *NodeServerManager) IsOnlineForServer(serverID int64) bool {
	if m.isLocalServer(serverID) {
		return true
	}
	return m.IsOnline(serverID)
}

// AlterUsersOnServer sends an AlterUsers command to a remote server's Agent via stream.
func (m *NodeServerManager) AlterUsersOnServer(ctx context.Context, serverID int64, req *agentpb.AlterUsersRequest) error {
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_AlterUsers{
			AlterUsers: req,
		},
	}
	return m.SendCommand(ctx, serverID, cmd)
}

// ApplyLimitsOnServer sends an ApplyLimits command to a remote server's Agent via stream.
func (m *NodeServerManager) ApplyLimitsOnServer(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error {
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_ApplyLimits{
			ApplyLimits: req,
		},
	}
	return m.SendCommand(ctx, serverID, cmd)
}

// AssociateIP pushes IP ownership information to a server's Agent.
// For local servers, delegates to LocalAgent → ratelimit.Manager.
// For remote servers, sends AssociateIP command via heartbeat stream.
func (m *NodeServerManager) AssociateIP(ctx context.Context, serverID int64, req *agentpb.AssociateIPRequest) error {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return fmt.Errorf("local agent not configured")
		}
		return m.localAgent.AssociateIP(ctx, req)
	}
	return m.AssociateIPOnServer(ctx, serverID, req)
}

// AssociateIPOnServer sends an AssociateIP command to a remote server's Agent via stream.
func (m *NodeServerManager) AssociateIPOnServer(ctx context.Context, serverID int64, req *agentpb.AssociateIPRequest) error {
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_AssociateIp{
			AssociateIp: req,
		},
	}
	return m.SendCommand(ctx, serverID, cmd)
}

// ============================================================
// 代理进程管理（Caddy/Nginx）— NodeOperator 接口实现
// ============================================================

// SyncProxyConfig syncs proxy configuration (Caddy/Nginx) to a server.
// For local servers, delegates to LocalAgent → proxy.Manager.WriteConfigFile.
// For remote servers, sends config via the heartbeat stream.
func (m *NodeServerManager) SyncProxyConfig(ctx context.Context, serverID int64, proxyType string, config []byte) error {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return fmt.Errorf("local agent not configured")
		}
		return m.localAgent.SyncProxyConfig(ctx, proxyType, config)
	}
	// Remote: send via heartbeat stream
	// Collect TLS certificates referenced by proxy sites for this server
	// so the Agent can land them on disk before writing the config file.
	var certs []*agentpb.CertificateEntry
	if m.proxyCertCollector != nil {
		certs = m.proxyCertCollector.CollectProxyCertificates(ctx, serverID)
	}

	// Collect per-node settings for nginx/caddy
	var nginxSettings, caddySNISettings string
	if m.proxySettingsProvider != nil {
		if proxyType == "nginx" {
			nginxSettings, _ = m.proxySettingsProvider.GetNginxSettingsForServer(ctx, serverID)
		} else if proxyType == "caddy" {
			caddySNISettings, _ = m.proxySettingsProvider.GetCaddySNISettingsForServer(ctx, serverID)
		}
	}

	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_SyncProxyConfig{
			SyncProxyConfig: &agentpb.SyncProxyConfigRequest{
				ProxyType:        proxyType,
				ConfigContent:    config,
				Certificates:     certs,
				NginxSettings:    nginxSettings,
				CaddySniSettings: caddySNISettings,
			},
		},
	}
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return fmt.Errorf("send SyncProxyConfig command to server %d: %w", serverID, err)
	}
	return nil
}

// SyncWAFFiles syncs WAF files (config.lua, domain.json, *.rule) to a remote Agent.
// For local servers, files are already on disk (the API handler wrote them directly).
// For remote servers, files are sent via the heartbeat stream.
func (m *NodeServerManager) SyncWAFFiles(ctx context.Context, serverID int64, files []*agentpb.WAFFile) error {
	if m.isLocalServer(serverID) {
		return nil // local files already written by API handler
	}
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_SyncWafFiles{
			SyncWafFiles: &agentpb.SyncWAFFilesRequest{
				Files: files,
			},
		},
	}
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return fmt.Errorf("send SyncWAFFiles to server %d: %w", serverID, err)
	}
	return nil
}

// SyncProxyConfigFromPrimary syncs proxy config to a mirror node, but fetches
// TLS certificates and per-node settings (SNI/nginx) from the PRIMARY's server ID
// instead of the mirror's own. This ensures mirrors receive the same settings as
// the primary (e.g., when SNI is toggled off on the primary, mirrors get SNI=off
// instead of their own stale SNI=on).
func (m *NodeServerManager) SyncProxyConfigFromPrimary(ctx context.Context, mirrorServerID, primaryServerID int64, proxyType string, config []byte) error {
	if m.isLocalServer(mirrorServerID) {
		// Local mirror — shouldn't happen, but handle gracefully
		return m.SyncProxyConfig(ctx, mirrorServerID, proxyType, config)
	}
	// Collect certs from the PRIMARY's server ID (mirrors replicate primary's sites)
	var certs []*agentpb.CertificateEntry
	if m.proxyCertCollector != nil {
		certs = m.proxyCertCollector.CollectProxyCertificates(ctx, primaryServerID)
	}

	// Collect settings from the PRIMARY's server ID
	var nginxSettings, caddySNISettings string
	if m.proxySettingsProvider != nil {
		if proxyType == "nginx" {
			nginxSettings, _ = m.proxySettingsProvider.GetNginxSettingsForServer(ctx, primaryServerID)
		} else if proxyType == "caddy" {
			caddySNISettings, _ = m.proxySettingsProvider.GetCaddySNISettingsForServer(ctx, primaryServerID)
		}
	}

	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_SyncProxyConfig{
			SyncProxyConfig: &agentpb.SyncProxyConfigRequest{
				ProxyType:        proxyType,
				ConfigContent:    config,
				Certificates:     certs,
				NginxSettings:    nginxSettings,
				CaddySniSettings: caddySNISettings,
			},
		},
	}
	if err := m.SendCommand(ctx, mirrorServerID, cmd); err != nil {
		return fmt.Errorf("send SyncProxyConfig (from primary %d) to mirror %d: %w", primaryServerID, mirrorServerID, err)
	}
	return nil
}

// ControlProxy controls a proxy process (Caddy/Nginx) on a server.
// For local servers, delegates to LocalAgent → proxy.Manager.StartProxy/StopProxy/etc.
// For remote servers, sends control command via the heartbeat stream.
func (m *NodeServerManager) ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error {
if m.isLocalServer(serverID) {
if m.localAgent == nil {
return fmt.Errorf("local agent not configured")
}
return m.localAgent.ControlProxy(ctx, proxyType, action)
}
// Remote: send via heartbeat stream
cmd := &agentpb.PanelCommand{
Command: &agentpb.PanelCommand_ControlProxy{
ControlProxy: &agentpb.ControlProxyRequest{
ProxyType: proxyType,
Action:    action,
},
},
}
if err := m.SendCommand(ctx, serverID, cmd); err != nil {
return fmt.Errorf("send ControlProxy command to server %d: %w", serverID, err)
}
return nil
}

// GetProxyGuardianSettingsForServer reads persisted proxy guardian settings from
// the panel's system_settings DB for a specific server. Used for remote nodes
// where the settings are stored in the panel DB (not on the agent).
// Settings keys: proxy.server_{serverID}.{type}.auto_start etc.
func (m *NodeServerManager) GetProxyGuardianSettingsForServer(ctx context.Context, serverID int64, proxyType string) (autoStart, autoRestart bool, maxRestarts int) {
	autoStart = true
	autoRestart = false
	maxRestarts = 5
	if m.settingStore == nil {
		return
	}
	prefix := fmt.Sprintf("proxy.server_%d.%s.", serverID, proxyType)
	if v, err := m.settingStore.GetByKey(ctx, prefix+"auto_start"); err == nil {
		autoStart = v != "false"
	}
	if v, err := m.settingStore.GetByKey(ctx, prefix+"auto_restart"); err == nil {
		autoRestart = v == "true"
	}
	if v, err := m.settingStore.GetByKey(ctx, prefix+"max_restarts"); err == nil {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRestarts = n
		}
	}
	return
}

// UpdateProxyGuardianForServer persists proxy guardian settings to the panel's
// system_settings DB for a specific server. For remote nodes, this is display-only
// (the agent has its own settings). For local nodes, the caller should also update
// the in-memory proxy.Manager.
func (m *NodeServerManager) UpdateProxyGuardianForServer(ctx context.Context, serverID int64, proxyType string, autoStart, autoRestart bool, maxRestarts int) error {
	if m.settingStore == nil {
		return nil
	}
	prefix := fmt.Sprintf("proxy.server_%d.%s.", serverID, proxyType)
	if err := m.settingStore.Set(ctx, prefix+"auto_start", fmt.Sprintf("%v", autoStart)); err != nil {
		return err
	}
	if err := m.settingStore.Set(ctx, prefix+"auto_restart", fmt.Sprintf("%v", autoRestart)); err != nil {
		return err
	}
	return m.settingStore.Set(ctx, prefix+"max_restarts", strconv.Itoa(maxRestarts))
}

// ============================================================
// Phase 6: 新增 RPC 方法（Agent 自动更新、测速、指标查询）
// ============================================================

// InstallBinary installs a core binary on a server.
// For local servers, delegates to LocalAgent → binaries.Installer.
// For remote servers, sends InstallBinary via heartbeat stream.
func (m *NodeServerManager) InstallBinary(ctx context.Context, serverID int64, coreType, version, downloadURL, assetName string) (*agentpb.TaskAccept, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		err := m.localAgent.InstallBinary(ctx, coreType, version, downloadURL, assetName)
		if err != nil {
			return nil, err
		}
		return &agentpb.TaskAccept{Accepted: true, TaskId: "local"}, nil
	}
	// Remote: send via heartbeat stream
	accelDomain, proxyURL := m.githubAccelConfig()
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_InstallBinary{
			InstallBinary: &agentpb.InstallBinaryRequest{
				CoreType:    coreType,
				Version:     version,
				DownloadUrl: downloadURL,
				AssetName:   assetName,
				AccelDomain: accelDomain,
				ProxyUrl:    proxyURL,
			},
		},
	}
	// Wait for TASK_QUEUED confirmation
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("send install_binary command: %w", err)
	}
	accepted := result.Status == agentpb.TaskStatus_TASK_QUEUED || result.Status == agentpb.TaskStatus_TASK_SUCCESS
	return &agentpb.TaskAccept{Accepted: accepted, TaskId: result.TaskId}, nil
}

// InstallProxy installs a proxy binary (caddy/nginx) on a server.
// For local servers, delegates to LocalAgent → proxy.Manager.
// For remote servers, sends InstallProxy via heartbeat stream.
func (m *NodeServerManager) InstallProxy(ctx context.Context, serverID int64, proxyType, downloadURL, tagName string) (*agentpb.TaskAccept, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		var err error
		if downloadURL != "" {
			err = m.localAgent.InstallProxyVersion(ctx, proxyType, downloadURL, tagName)
		} else {
			err = m.localAgent.InstallProxy(ctx, proxyType)
		}
		if err != nil {
			return nil, err
		}
		return &agentpb.TaskAccept{Accepted: true, TaskId: "local"}, nil
	}
	// Remote: send via heartbeat stream
	accelDomain, proxyURL := m.githubAccelConfig()
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_InstallProxy{
			InstallProxy: &agentpb.InstallProxyRequest{
				ProxyType:   proxyType,
				AccelDomain: accelDomain,
				ProxyUrl:    proxyURL,
				DownloadUrl: downloadURL,
				TagName:     tagName,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("send install_proxy command: %w", err)
	}
	accepted := result.Status == agentpb.TaskStatus_TASK_QUEUED || result.Status == agentpb.TaskStatus_TASK_SUCCESS
	return &agentpb.TaskAccept{Accepted: accepted, TaskId: result.TaskId}, nil
}

// GetProxyStatus queries proxy installation status on a server.
// For local servers, delegates to LocalAgent.
// For remote servers, queries via heartbeat stream.
func (m *NodeServerManager) GetProxyStatus(ctx context.Context, serverID int64) (*agentpb.GetProxyStatusResponse, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.GetProxyStatuses(), nil
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_GetProxyStatus{
			GetProxyStatus: &agentpb.GetProxyStatusRequest{},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.GetProxyStatusResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal get_proxy_status response: %w", err)
	}
	return &resp, nil
}

// ListBinaries lists installed core binaries on a server.
// For local servers, delegates to LocalAgent.
// For remote servers, queries via heartbeat stream.
func (m *NodeServerManager) ListBinaries(ctx context.Context, serverID int64) (*agentpb.ListBinariesResponse, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ListBinaries(), nil
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_ListBinaries{
			ListBinaries: &agentpb.ListBinariesRequest{},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.ListBinariesResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal list_binaries response: %w", err)
	}
	return &resp, nil
}

// InvalidateBinaryCache clears the cached binary info for a server so the next
// GetRemoteState call will re-fetch fresh data from the Agent.
func (m *NodeServerManager) InvalidateBinaryCache(serverID int64) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.cachedBinaries = nil
	ac.cachedBinariesAt = time.Time{}
}

// GetServerPlatform returns the OS and architecture of a remote Agent from cached heartbeat data.
// Returns the panel's own OS/arch as fallback for local servers.
func (m *NodeServerManager) GetServerPlatform(serverID int64) (osName, arch string) {
	if m.isLocalServer(serverID) {
		return runtime.GOOS, runtime.GOARCH
	}
	ac := m.GetConnection(serverID)
	if ac == nil {
		return runtime.GOOS, runtime.GOARCH
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cachedOS != "" && ac.cachedArch != "" {
		return ac.cachedOS, ac.cachedArch
	}
	// Fallback to panel's platform if not yet reported
	return runtime.GOOS, runtime.GOARCH
}

// GetServerDistro returns the Linux distribution of a remote Agent from cached heartbeat data.
// Returns the panel's own distro as fallback for local servers.
func (m *NodeServerManager) GetServerDistro(serverID int64) string {
	if m.isLocalServer(serverID) {
		return detectLocalDistro()
	}
	ac := m.GetConnection(serverID)
	if ac == nil {
		return detectLocalDistro()
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cachedDistro != "" {
		return ac.cachedDistro
	}
	// Fallback to panel's own distro if not yet reported
	return detectLocalDistro()
}

// detectLocalDistro reads /etc/os-release on the panel machine to determine its Linux distribution.
// Returns a normalized lower-case distro identifier or "" if not Linux / not detectable.
func detectLocalDistro() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id := strings.Trim(strings.TrimPrefix(line, "ID="), `"'`)
			id = strings.ToLower(strings.TrimSpace(id))
			switch id {
			case "rhel", "centos", "rocky", "rockylinux", "almalinux", "fedora":
				return "redhat"
			case "ubuntu", "debian", "alpine", "opensuse", "opensuse-leap", "sles", "arch":
				return id
			default:
				if id != "" {
					return id
				}
			}
		}
	}
	return ""
}

// UpdateAgentOnServer sends an update command to a remote Agent via stream.
// The Agent downloads the new binary, verifies checksum, replaces, and restarts.
func (m *NodeServerManager) UpdateAgentOnServer(ctx context.Context, serverID int64, version, downloadURL, checksum string) (*agentpb.TaskAccept, error) {
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_UpdateAgent{
			UpdateAgent: &agentpb.UpdateAgentCommand{
				Version:     version,
				DownloadUrl: downloadURL,
				Checksum:    checksum,
			},
		},
	}
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return nil, fmt.Errorf("send update_agent command: %w", err)
	}
	return &agentpb.TaskAccept{Accepted: true}, nil
}

// SpeedTestOnServer requests a remote Agent to perform a speed test via stream.
// SpeedTestOnServer measures the latency of a remote Agent by timing the round-trip of
// a gRPC command over the existing heartbeat stream (the same channel the Agent already
// uses to talk to the panel). The Agent echoes back immediately (no port probe), so the
// measured round-trip time is the real panel↔node control-plane RTT — exactly what the
// load balancer needs for latency-based sorting. This reuses the heartbeat stream instead
// of opening a separate TCP connection to some arbitrary port.
func (m *NodeServerManager) SpeedTestOnServer(ctx context.Context, serverID int64, targetHost string, targetPort int32, timeoutMs int32, samples int32) (*agentpb.SpeedTestResponse, error) {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_SpeedTest{
			SpeedTest: &agentpb.SpeedTestRequest{
				TargetHost: targetHost,
				TargetPort: targetPort,
				TimeoutMs:  timeoutMs,
				Samples:    samples,
			},
		},
	}
	start := time.Now()
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 60*time.Second)
	rtt := time.Since(start)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.SpeedTestResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal speed_test response: %w", err)
	}
	// Override latency with the measured heartbeat-stream round-trip; the Agent no
	// longer probes a port, so its reported LatencyMs is meaningless.
	resp.Success = true
	resp.LatencyMs = int32(rtt.Milliseconds())
	resp.JitterMs = 0
	resp.PacketLoss = 0
	return &resp, nil
}

// GetMetricsFromServer queries detailed metrics from a remote Agent via stream.
func (m *NodeServerManager) GetMetricsFromServer(ctx context.Context, serverID int64, lastNPoints int32, metricType string) (*agentpb.GetMetricsResponse, error) {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_GetMetrics{
			GetMetrics: &agentpb.GetMetricsRequest{
				LastNPoints: lastNPoints,
				MetricType:  metricType,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.GetMetricsResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal get_metrics response: %w", err)
	}
	return &resp, nil
}

// DownloadGeoRule downloads a geo rule file on a server.
// For local servers, the caller (georule service) handles the download directly.
// For remote servers, sends DownloadGeoRule via heartbeat stream.
// fileHash and fileSize are sent so the Agent can skip download if unchanged.
func (m *NodeServerManager) DownloadGeoRule(ctx context.Context, serverID int64, fileName, downloadURL, fileHash string, fileSize int64, accelDomain, proxyURL string) (bool, int64, string, error) {
	if m.isLocalServer(serverID) {
		// Local: signal that it should be handled by the caller
		return false, 0, "LOCAL", nil
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_DownloadGeoRule{
			DownloadGeoRule: &agentpb.DownloadGeoRuleRequest{
				FileName:    fileName,
				DownloadUrl: downloadURL,
				AccelDomain: accelDomain,
				ProxyUrl:    proxyURL,
				FileHash:    fileHash,
				FileSize:    fileSize,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 120*time.Second)
	if err != nil {
		return false, 0, "", err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return false, 0, result.Error, nil
	}
	var resp agentpb.DownloadGeoRuleResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return false, 0, "", fmt.Errorf("unmarshal download_geo_rule response: %w", err)
	}
	return resp.Success, resp.FileSize, resp.Error, nil
}

// DeleteGeoRule deletes a geo rule file from a remote server.
// For local servers, the caller (georule service) handles the deletion directly.
// For remote servers, sends DeleteGeoRule via heartbeat stream.
func (m *NodeServerManager) DeleteGeoRule(ctx context.Context, serverID int64, fileName string) (bool, string, error) {
	if m.isLocalServer(serverID) {
		// Local: caller handles deletion
		return false, "LOCAL", nil
	}
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_DeleteGeoRule{
			DeleteGeoRule: &agentpb.DeleteGeoRuleRequest{
				FileName: fileName,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 30*time.Second)
	if err != nil {
		return false, "", err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return false, result.Error, nil
	}
	var resp agentpb.DeleteGeoRuleResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return false, "", fmt.Errorf("unmarshal delete_geo_rule response: %w", err)
	}
	return resp.Success, resp.Error, nil
}

// StopGRPCServer gracefully stops the gRPC server.
func (m *NodeServerManager) StopGRPCServer() {
	if m.grpcServer != nil {
		m.grpcServer.GracefulStop()
		log.Println("[NodeServerManager] gRPC server stopped")
	}
}

// GetConnection returns the AgentConnection for the given server ID.
func (m *NodeServerManager) GetConnection(serverID int64) *AgentConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[serverID]
}

// IsOnline checks if a server's Agent is online.
func (m *NodeServerManager) IsOnline(serverID int64) bool {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return false
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()
	return ac.online
}

// ListOnlineServers returns the IDs of all connected servers.
func (m *NodeServerManager) ListOnlineServers() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []int64
	for id, ac := range m.connections {
		ac.mu.Lock()
		if ac.online {
			ids = append(ids, id)
		}
		ac.mu.Unlock()
	}
	return ids
}

// SendCommand sends a PanelCommand to a specific server via the heartbeat stream.
func (m *NodeServerManager) SendCommand(ctx context.Context, serverID int64, cmd *agentpb.PanelCommand) error {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return fmt.Errorf("server %d not connected", serverID)
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if ac.stream == nil {
		return fmt.Errorf("server %d has no active stream", serverID)
	}

	if err := ac.stream.Send(cmd); err != nil {
		return fmt.Errorf("send command to server %d: %w", serverID, err)
	}
	return nil
}

// SendCommandAndWait sends a PanelCommand to a server and waits for the Agent
// to report back a CommandResult. This enables synchronous command execution
// with a timeout, so callers can know whether the command succeeded or failed.
//
// If the command has no command_id, one is generated automatically.
// The timeout controls how long to wait for the result before returning an error.
func (m *NodeServerManager) SendCommandAndWait(ctx context.Context, serverID int64, cmd *agentpb.PanelCommand, timeout time.Duration) (*agentpb.TaskResult, error) {
	// Generate command_id if not set
	if cmd.CommandId == "" {
		cmd.CommandId = generateCommandID()
	}
	cmdID := cmd.CommandId

	// Register pending result channel
	resultCh := make(chan *agentpb.TaskResult, 1)
	m.pendingTasksMu.Lock()
	m.pendingTasks[cmdID] = resultCh
	m.pendingTasksMu.Unlock()
	defer func() {
		m.pendingTasksMu.Lock()
		delete(m.pendingTasks, cmdID)
		m.pendingTasksMu.Unlock()
	}()

	// Send the command
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return nil, fmt.Errorf("send command: %w", err)
	}

	// Wait for result with timeout
	select {
	case result := <-resultCh:
		return result, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("command %s timed out after %v", cmdID, timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("command %s cancelled: %w", cmdID, ctx.Err())
	}
}

// handleTaskResult processes a TaskResult report from an Agent,
// routing it to the waiting goroutine via the pendingTasks map.
func (h *heartbeatServiceImpl) handleTaskResult(ac *AgentConnection, report *agentpb.AgentReport) {
	if report.TaskResult == nil {
		return
	}
	taskID := report.TaskResult.TaskId
	if taskID == "" {
		return
	}
	h.mgr.pendingTasksMu.RLock()
	ch, ok := h.mgr.pendingTasks[taskID]
	h.mgr.pendingTasksMu.RUnlock()
	if ok {
		select {
		case ch <- report.TaskResult:
		default: // channel full, shouldn't happen with buffer size 1
		}
	}
}

// generateCommandID generates a unique command ID for tracking.
// Uses a simple timestamp + random suffix approach.
func generateCommandID() string {
	return fmt.Sprintf("cmd-%d-%04d", time.Now().UnixNano(), time.Now().Nanosecond()%10000)
}

// PushAgentCertificate pushes a renewed Agent certificate via the stream.
// Implements the NodeCertPusher interface for CertRenewer.
// For local servers, this is a no-op (certs are managed locally).
func (m *NodeServerManager) PushAgentCertificate(ctx context.Context, serverID int64, cert *AgentCertResult) error {
	if m.isLocalServer(serverID) {
		return nil // local server: no remote Agent to push to
	}
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_RenewCert{
			RenewCert: &agentpb.RenewCertCommand{
				NewCertPem: cert.CertPEM,
				NewKeyPem:  cert.KeyPEM,
				ExpiresAt:  cert.ExpiresAt.Unix(),
			},
		},
	}
	return m.SendCommand(ctx, serverID, cmd)
}

// DistributeCertificate pushes an external service TLS cert to an Agent.
func (m *NodeServerManager) DistributeCertificate(ctx context.Context, serverID int64, cmd *agentpb.DistributeCertCommand) error {
	if m.isLocalServer(serverID) {
		return nil // local server: certs are managed locally
	}
	panelCmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_DistributeCert{
			DistributeCert: cmd,
		},
	}
	return m.SendCommand(ctx, serverID, panelCmd)
}

// DisconnectServer forcibly disconnects an Agent (e.g., on certificate revocation).
func (m *NodeServerManager) DisconnectServer(ctx context.Context, serverID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ac, ok := m.connections[serverID]; ok {
		ac.mu.Lock()
		ac.online = false
		ac.mu.Unlock()
		delete(m.connections, serverID)
		m.onlineStore.Remove(serverID)
		log.Printf("[NodeServerManager] server %d disconnected", serverID)
	}
}

// UpdateOnlineUsers 用 Agent 上报的数据刷新某节点的在线快照。
func (m *NodeServerManager) UpdateOnlineUsers(serverID int64, users []*agentpb.OnlineUserEntry) {
	m.onlineStore.Update(serverID, users)
}

// GetNodeOnlineUsers 返回某节点当前在线订阅人（未过期）及刷新时间。
func (m *NodeServerManager) GetNodeOnlineUsers(serverID int64) ([]*agentpb.OnlineUserEntry, int64, bool) {
	snap, ok := m.onlineStore.GetFresh(serverID)
	if !ok {
		return nil, 0, false
	}
	return snap.Users, snap.UpdatedAt, true
}

// GetNodeOnlineCount 返回某节点在线订阅数与设备(IP)数。
func (m *NodeServerManager) GetNodeOnlineCount(serverID int64) (subscriptions int, devices int) {
	snap, ok := m.onlineStore.GetFresh(serverID)
	if !ok {
		return 0, 0
	}
	subscriptions = len(snap.Users)
	for _, u := range snap.Users {
		devices += len(u.Sessions)
	}
	return
}

// GetAllOnlineSummary 返回所有在线节点的实时在线汇总（serverID → 汇总）。
func (m *NodeServerManager) GetAllOnlineSummary() map[int64]*NodeOnlineSummary {
	return m.onlineStore.GetAllSummary()
}

// GetAllOnlineUsers 返回所有在线节点的完整在线快照（serverID → 快照），供全局视图使用。
func (m *NodeServerManager) GetAllOnlineUsers() map[int64]*NodeOnlineSnapshot {
	return m.onlineStore.GetAllDetail()
}

// SyncConfigToServer syncs core configuration to a specific server.
// It sends the config via the heartbeat stream (PanelCommand_SyncConfig),
// which is more reliable than the reverse gRPC connection.
func (m *NodeServerManager) SyncConfigToServer(ctx context.Context, serverID int64, coreType string, config []byte, version int64) error {
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_SyncConfig{
			SyncConfig: &agentpb.SyncConfigRequest{
				CoreType:      coreType,
				ConfigContent: config,
				ConfigVersion: version,
			},
		},
	}
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return fmt.Errorf("send SyncConfig command to server %d: %w", serverID, err)
	}
	return nil
}

// ControlCoreOnServer sends a control command to a server's core process.
// It sends the command via the heartbeat stream (PanelCommand_ControlCore),
// which is more reliable than the reverse gRPC connection because the stream
// is always active and verified working (unlike the reverse-dial gRPC client
// which can become stale if the Agent restarts).
func (m *NodeServerManager) ControlCoreOnServer(ctx context.Context, serverID int64, coreType, action string) error {
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_ControlCore{
			ControlCore: &agentpb.ControlCoreRequest{
				CoreType: coreType,
				Action:   action,
			},
		},
	}
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return fmt.Errorf("send ControlCore command to server %d: %w", serverID, err)
	}
	return nil
}

// GetServerStatus queries the status of a server's cores.
// If a direct gRPC connection to the Agent is available, it uses that.
// Otherwise, it returns cached core states from the last heartbeat status report.
func (m *NodeServerManager) GetServerStatus(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}

	// Fallback: return cached core states from last heartbeat status report
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if !ac.statusReported {
		return nil, fmt.Errorf("server %d status not yet reported", serverID)
	}

	cores := make([]*agentpb.CoreStatus, 0, len(ac.coreStates))
	for _, cs := range ac.coreStates {
		cores = append(cores, cs)
	}
	return &agentpb.GetStatusResponse{Cores: cores}, nil
}

// GetRemoteState returns cached state from a remote Agent (core states + binaries).
// This does NOT make a blocking gRPC call for core states — it uses data cached from
// heartbeat status reports. Binaries are refreshed via stream if the cache is stale,
// but the lock is released before the stream call to avoid deadlock (SendCommand also
// acquires ac.mu).
func (m *NodeServerManager) GetRemoteState(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, []*agentpb.BinaryInfo, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, nil, fmt.Errorf("server %d not connected", serverID)
	}

	// Read cached state under lock
	ac.mu.Lock()
	var cores []*agentpb.CoreStatus
	if ac.statusReported {
		for _, cs := range ac.coreStates {
			cores = append(cores, cs)
		}
	}
	binaries := ac.cachedBinaries
	cacheExpired := time.Since(ac.cachedBinariesAt) > 30*time.Second
	needFetch := binaries == nil || cacheExpired
	ac.mu.Unlock()

	// Fetch binaries via stream WITHOUT holding ac.mu (SendCommand acquires it)
	if needFetch {
		fetchCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if resp, err := m.ListBinaries(fetchCtx, serverID); err == nil && resp != nil {
			binaries = resp.Binaries
			// Update cache under lock
			ac.mu.Lock()
			ac.cachedBinaries = binaries
			ac.cachedBinariesAt = time.Now()
			ac.mu.Unlock()
		}
		cancel()
	}

	return &agentpb.GetStatusResponse{Cores: cores}, binaries, nil
}

// TrafficEvents returns the channel for consuming traffic events.
func (m *NodeServerManager) TrafficEvents() <-chan *TrafficEvent {
	return m.onTraffic
}

// StatusEvents returns the channel for consuming status events.
func (m *NodeServerManager) StatusEvents() <-chan *StatusEvent {
	return m.onStatusChg
}

// registerConnection registers a new Agent connection (called by heartbeatServiceImpl).
// All Panel→Agent communication goes through the heartbeat stream — no reverse dial.
func (m *NodeServerManager) registerConnection(serverID int64, hostname string, stream agentpb.HeartbeatService_StreamServer) *AgentConnection {
	ac := &AgentConnection{
		ServerID:      serverID,
		Hostname:      hostname,
		agentVersion:  hostname, // hostname param is actually the agent version from firstReport.AgentVersion
		stream:        stream,
		online:        true,
		lastHeartbeat: time.Now(),
		coreStates:    make(map[string]*agentpb.CoreStatus),
	}

	m.mu.Lock()
	m.connections[serverID] = ac
	m.mu.Unlock()

	// Update server status in DB
	if err := m.serverRepo.UpdateAgentStatus(context.Background(), serverID, "online", hostname); err != nil {
		log.Printf("[NodeServerManager] failed to update agent status for server %d: %v", serverID, err)
	}

	// Notify AgentUpdater about the reconnect so it can verify whether a pending
	// update task has completed (Agent restarts with new version after self-update).
	// This must be called even if there's no pending task — OnAgentReconnect is a
	// no-op when no task exists for this server.
	if m.agentUpdater != nil {
		m.agentUpdater.OnAgentReconnect(serverID, hostname)
	}

	// On Agent reconnect: request immediate status + push SyncData to ensure
	// the Agent's local DB is up-to-date and the core is started.
	//
	// This solves two problems:
	// 1. Status cache is empty after panel restart → UI shows "unknown" state.
	//    GetStatus command populates the cache immediately.
	// 2. Agent's local DB may have stale guardian settings (auto_start=0).
	//    SyncData with ApplyAndStart=true writes fresh DB data and starts the
	//    core if it's not running. If the core is already running, Start is a
	//    no-op (returns nil without killing the process).
	go func() {
		time.Sleep(2 * time.Second) // give the agent a moment to fully initialize

		// 1. Request immediate status report
		cmd := &agentpb.PanelCommand{
			Command: &agentpb.PanelCommand_GetStatus{},
		}
		if err := m.SendCommand(context.Background(), serverID, cmd); err != nil {
			log.Printf("[NodeServerManager] failed to request status from server %d on connect: %v", serverID, err)
		}

		// 2. Push SyncData (DB tables + config + start core)
		//    Skip for local server (server_id=1) — it manages its own DB.
		if serverID != m.localServerID && m.dataSyncProvider != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			resp, err := m.SyncDataToServer(ctx, serverID, true)
			if err != nil {
				log.Printf("[NodeServerManager] auto-SyncData to server %d on connect failed: %v", serverID, err)
			} else {
				log.Printf("[NodeServerManager] auto-SyncData to server %d on connect: success=%v core_started=%v",
					serverID, resp.Success, resp.CoreStarted)
			}
		}

		// 3. Check and renew Agent cert if it's expiring soon.
		//    This ensures the Agent always gets a fresh cert on reconnect,
		//    even if it was offline during the periodic renewal check.
		if m.certRenewer != nil && serverID != m.localServerID {
			m.certRenewer.RenewServerCertIfNeeded(context.Background(), serverID)
		}

		// 4. Push rate limit config if rate limiting is enabled for this server.
		//    On reconnect, the Agent needs to re-initialize its RateLimitManager.
		if m.rateLimitSyncer != nil && serverID != m.localServerID {
			m.rateLimitSyncer.SyncConfigToAgent(context.Background(), serverID)
		}

		// 5. Push WAF rules/configs to the remote node.
		//    On reconnect, the remote node may not have the waf-rules/ directory
		//    (e.g., first connect, or directory was deleted). This ensures the
		//    Agent always has the latest rules, config.lua, config.json, domain.json
		//    from the panel's database.
		if m.wafSyncer != nil && serverID != m.localServerID {
			wafCtx, wafCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer wafCancel()
			if err := m.wafSyncer.SyncToNode(wafCtx, serverID); err != nil {
				log.Printf("[NodeServerManager] WAF sync to server %d on connect failed: %v", serverID, err)
			} else {
				log.Printf("[NodeServerManager] WAF sync to server %d on connect: success", serverID)
			}
		}

		// 6. Push geo rule files (geoip.dat, geosite.dat) to the remote node.
		//    On reconnect, the remote node needs the latest geo rule files for
		//    xray/singbox routing. Files are downloaded from the panel's HTTP endpoint.
		if m.geoRuleSyncer != nil && serverID != m.localServerID {
			geoCtx, geoCancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer geoCancel()
			if err := m.geoRuleSyncer.SyncToNode(geoCtx, serverID); err != nil {
				log.Printf("[NodeServerManager] geo rule sync to server %d on connect failed: %v", serverID, err)
			} else {
				log.Printf("[NodeServerManager] geo rule sync to server %d on connect: success", serverID)
			}
		}
	}()

	log.Printf("[NodeServerManager] server %d (%s) connected", serverID, hostname)
	return ac
}

// markOffline marks a server as offline (called when stream breaks).
func (m *NodeServerManager) markOffline(serverID int64) {
	m.mu.Lock()
	if ac, ok := m.connections[serverID]; ok {
		ac.mu.Lock()
		ac.online = false
		ac.mu.Unlock()
		delete(m.connections, serverID)
		m.onlineStore.Remove(serverID)
	}
	m.mu.Unlock()

	// Update DB
	if err := m.serverRepo.UpdateAgentStatus(context.Background(), serverID, "offline", ""); err != nil {
		log.Printf("[NodeServerManager] failed to mark server %d offline: %v", serverID, err)
	}

	log.Printf("[NodeServerManager] server %d went offline", serverID)

	// Emit status event
	select {
	case m.onStatusChg <- &StatusEvent{ServerID: serverID, Online: false}:
	default: // drop if channel full
	}
}

// ============================================================
// heartbeatServiceImpl — gRPC server implementation
// ============================================================

type heartbeatServiceImpl struct {
	agentpb.UnimplementedHeartbeatServiceServer
	mgr *NodeServerManager
}

func (h *heartbeatServiceImpl) Stream(stream agentpb.HeartbeatService_StreamServer) error {
	// First message must be a HEARTBEAT with server_id
	firstReport, err := stream.Recv()
	if err != nil {
		return err
	}

	if firstReport.Type != agentpb.AgentReport_HEARTBEAT {
		return fmt.Errorf("first report must be HEARTBEAT, got %v", firstReport.Type)
	}

	serverID := firstReport.ServerId
	ac := h.mgr.registerConnection(serverID, firstReport.AgentVersion, stream)

	// Handle first heartbeat
	h.handleHeartbeat(ac, firstReport)

	// Main loop: receive reports
	for {
		report, err := stream.Recv()
		if err != nil {
			h.mgr.markOffline(serverID)
			return err
		}

		ac.mu.Lock()
		ac.lastHeartbeat = time.Now()
		ac.mu.Unlock()

		switch report.Type {
		case agentpb.AgentReport_HEARTBEAT:
			h.handleHeartbeat(ac, report)
		case agentpb.AgentReport_STATUS:
			h.handleStatusReport(ac, report)
		case agentpb.AgentReport_TRAFFIC:
			h.handleTrafficReport(ac, report)
		case agentpb.AgentReport_CRASH:
			h.handleCrashReport(ac, report)
		case agentpb.AgentReport_TASK_RESULT:
			h.handleTaskResult(ac, report)
		case agentpb.AgentReport_ONLINE_USERS:
			h.handleOnlineUsersReport(ac, report)
		}
	}
}

// egressSyncMap tracks the last egress interface name pushed to each Agent so we
// only re-send SetEgressInterface when it changes (or after a panel restart, when
// the Agent has lost its in-memory value). The panel previously never sent this
// command, which is why NIC-level traffic was never accounted.
var egressSyncMap sync.Map

func (h *heartbeatServiceImpl) handleHeartbeat(ac *AgentConnection, report *agentpb.AgentReport) {
	// Cache OS/arch/distro from heartbeat for asset filtering
	if report.Os != "" || report.Arch != "" || report.Distro != "" {
		ac.mu.Lock()
		ac.cachedOS = report.Os
		ac.cachedArch = report.Arch
		ac.cachedDistro = report.Distro
		ac.mu.Unlock()
	}

	// Detect Agent version change between heartbeats (e.g. after self-update
	// without a full reconnect). Notify the AgentUpdater so it can verify
	// and mark any pending update task as done.
	if report.AgentVersion != "" {
		ac.mu.Lock()
		oldVersion := ac.agentVersion
		versionChanged := oldVersion != "" && oldVersion != report.AgentVersion
		if versionChanged {
			ac.agentVersion = report.AgentVersion
		}
		ac.mu.Unlock()

		if versionChanged && h.mgr.agentUpdater != nil {
			log.Printf("[NodeServerManager] agent version changed for server %d: %s → %s",
				ac.ServerID, oldVersion, report.AgentVersion)
			h.mgr.agentUpdater.OnAgentReconnect(ac.ServerID, report.AgentVersion)
		}
	}

	// Update heartbeat in DB (throttled — every 30s, not every heartbeat)
	if err := h.mgr.serverRepo.UpdateHeartbeat(context.Background(), ac.ServerID); err != nil {
		log.Printf("[NodeServerManager] failed to update heartbeat for server %d: %v", ac.ServerID, err)
	}

	// Phase 6: Collect metrics from heartbeat
	if h.mgr.metricsCollector != nil {
		h.mgr.metricsCollector.CollectFromHeartbeat(ac.ServerID, report)
	}

	// Phase 6b: ensure the Agent knows which egress NIC to account, then process
	// NIC-level traffic deltas for quota monitoring.
	if h.mgr.trafficQuotaSvc != nil {
		if server, err := h.mgr.serverRepo.GetByID(context.Background(), ac.ServerID); err == nil && server.EgressInterface != "" {
			if prev, _ := egressSyncMap.Load(ac.ServerID); prev != server.EgressInterface {
				cmd := &agentpb.PanelCommand{
					Command: &agentpb.PanelCommand_SetEgressInterface{
						SetEgressInterface: &agentpb.SetEgressInterfaceRequest{InterfaceName: server.EgressInterface},
					},
				}
				if err := h.mgr.SendCommand(context.Background(), ac.ServerID, cmd); err == nil {
					egressSyncMap.Store(ac.ServerID, server.EgressInterface)
				} else {
					log.Printf("[NodeServerManager] failed to push egress interface %q to server %d: %v",
						server.EgressInterface, ac.ServerID, err)
				}
			}
		}
		h.mgr.trafficQuotaSvc.ProcessHeartbeat(ac.ServerID, report)
	}
}

func (h *heartbeatServiceImpl) handleStatusReport(ac *AgentConnection, report *agentpb.AgentReport) {
	ac.mu.Lock()
	ac.coreStates = make(map[string]*agentpb.CoreStatus, len(report.Cores))
	for _, cs := range report.Cores {
		ac.coreStates[cs.CoreType] = cs
	}
	ac.statusReported = true
	ac.mu.Unlock()

	// Emit event
	select {
	case h.mgr.onStatusChg <- &StatusEvent{
		ServerID: ac.ServerID,
		Online:   true,
		Cores:    report.Cores,
	}:
	default:
	}
}

func (h *heartbeatServiceImpl) handleTrafficReport(ac *AgentConnection, report *agentpb.AgentReport) {
	for _, stat := range report.TrafficStats {
		event := &TrafficEvent{
			ServerID:  ac.ServerID,
			Email:     stat.UserEmail,
			Uplink:    stat.UplinkBytes,
			Downlink:  stat.DownlinkBytes,
			CoreType:  stat.CoreType,
			Timestamp: time.Unix(report.Timestamp, 0),
		}
		select {
		case h.mgr.onTraffic <- event:
		default: // drop if channel full
			log.Printf("[TrafficReport] DROPPED server=%d email=%s up=%d down=%d (channel full)",
				ac.ServerID, stat.UserEmail, stat.UplinkBytes, stat.DownlinkBytes)
		}
	}
}

func (h *heartbeatServiceImpl) handleCrashReport(ac *AgentConnection, report *agentpb.AgentReport) {
	log.Printf("[NodeServerManager] server %d core crash: %s\n%s",
		ac.ServerID, report.CrashedCore, report.CrashLog)
}

// handleOnlineUsersReport 处理 Agent 上报的实时在线订阅人。
func (h *heartbeatServiceImpl) handleOnlineUsersReport(ac *AgentConnection, report *agentpb.AgentReport) {
	users := report.GetOnlineUsers()
	// 空列表也刷新时间戳：表示节点在线但当前无订阅人在线
	h.mgr.onlineStore.Update(ac.ServerID, users)
}

// ============================================================
// registrationServiceImpl — gRPC server implementation for Bootstrap
// ============================================================

type registrationServiceImpl struct {
	agentpb.UnimplementedRegistrationServiceServer
	mgr *NodeServerManager
}

func (r *registrationServiceImpl) Register(ctx context.Context, req *agentpb.RegisterRequest) (*agentpb.RegisterResponse, error) {
	// The BootstrapTokenManager is not directly accessible here.
	// In a full implementation, this would be injected.
	// For now, we return a "not implemented" error.
	// The actual registration flow will be wired up in the integration phase.
	return &agentpb.RegisterResponse{
		Success: false,
		Error:   "registration not yet wired up — use REST API bootstrap-token endpoint",
	}, nil
}

// ListInterfaces lists network interfaces on a server.
// For local servers, queries directly. For remote servers, queries via heartbeat stream.
func (m *NodeServerManager) ListInterfaces(ctx context.Context, serverID int64) ([]string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ListInterfaces()
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_ListInterfaces{
			ListInterfaces: &agentpb.ListInterfacesRequest{},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.ListInterfacesResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal list_interfaces response: %w", err)
	}
	return resp.GetInterfaces(), nil
}

// ExecuteBinaryCommand executes a binary command on a server.
// For local servers, executes directly via LocalAgent.
// For remote servers, sends via heartbeat stream.
func (m *NodeServerManager) ExecuteBinaryCommand(ctx context.Context, serverID int64, coreType string, args ...string) (map[string]string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ExecuteBinaryCommand(ctx, coreType, args...)
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_ExecuteBinary{
			ExecuteBinary: &agentpb.ExecuteBinaryRequest{
				CoreType:  coreType,
				Args:      args,
				RawOutput: false,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.ExecuteBinaryResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal execute_binary response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Parsed, nil
}

// ExecuteBinaryRawOutput executes a binary command and returns raw stdout.
// For local servers, executes directly. For remote servers, sends via heartbeat stream.
func (m *NodeServerManager) ExecuteBinaryRawOutput(ctx context.Context, serverID int64, coreType string, args ...string) (string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return "", fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ExecuteBinaryRawOutput(ctx, coreType, args...)
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_ExecuteBinary{
			ExecuteBinary: &agentpb.ExecuteBinaryRequest{
				CoreType:  coreType,
				Args:      args,
				RawOutput: true,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 60*time.Second)
	if err != nil {
		return "", err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return "", fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.ExecuteBinaryResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return "", fmt.Errorf("unmarshal execute_binary response: %w", err)
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	return resp.Stdout, nil
}

// SyncDataToServer syncs all configuration data from the panel database
// to a remote Agent via heartbeat stream. The Agent writes the data to its local
// SQLite database, generates runtime config files from the local data, and starts
// the core process.
func (m *NodeServerManager) SyncDataToServer(ctx context.Context, serverID int64, applyAndStart bool) (*agentpb.SyncDataResponse, error) {
	if m.isLocalServer(serverID) {
		return nil, fmt.Errorf("SyncData is for remote servers only (server %d is local)", serverID)
	}

	if m.dataSyncProvider == nil {
		return nil, fmt.Errorf("data sync provider not configured")
	}

	req, err := m.dataSyncProvider.BuildSyncDataRequest(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("build sync data: %w", err)
	}
	req.ApplyAndStart = applyAndStart

	// Send via heartbeat stream and wait for result
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_SyncData{
			SyncData: req,
		},
	}

	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 60*time.Second)
	if err != nil {
		// Timeout doesn't mean failure — SyncData is async
		log.Printf("[NodeServerManager] SyncData to server %d sent (async, no result yet)", serverID)
		return &agentpb.SyncDataResponse{Success: true}, nil
	}

	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("SyncData failed: %s", result.Error)
	}

	// Try to deserialize detailed result
	if result.DetailJson != "" {
		var resp agentpb.SyncDataResponse
		if err := json.Unmarshal([]byte(result.DetailJson), &resp); err == nil {
			return &resp, nil
		}
	}

	return &agentpb.SyncDataResponse{Success: true}, nil
}

// XrayAPIOnServer proxies an xray API call to a remote server's Agent via stream.
// For local servers, returns an error (caller should use direct connection).
func (m *NodeServerManager) XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_XrayApi{
			XrayApi: req,
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.XrayAPIResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal xray_api response: %w", err)
	}
	return &resp, nil
}

// ReadConfigFileOnServer reads the runtime config file from a node (local or remote).
// For local nodes, it reads directly from disk via the LocalAgent.
// For remote nodes, it sends ReadConfigFile via heartbeat stream.
func (m *NodeServerManager) ReadConfigFileOnServer(ctx context.Context, serverID int64, coreType string) ([]byte, string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, "", fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ReadConfigFile(coreType)
	}
	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_ReadConfigFile{
			ReadConfigFile: &agentpb.ReadConfigFileRequest{
				CoreType: coreType,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, "", err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, "", fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.ReadConfigFileResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, "", fmt.Errorf("unmarshal read_config_file response: %w", err)
	}
	if !resp.Success {
		return nil, "", fmt.Errorf("%s", resp.Error)
	}
	return resp.Content, resp.FilePath, nil
}

// ReadLogFileOnServer reads the last N lines of a log file from a node (local or remote).
// If logPath is non-empty, it reads that specific file; otherwise it resolves by logType.
func (m *NodeServerManager) ReadLogFileOnServer(ctx context.Context, serverID int64, logType string, lastLines int) ([]string, string, error) {
	return m.ReadLogFileOnServerWithPath(ctx, serverID, logType, "", lastLines)
}

// ReadLogFileOnServerFiltered reads log lines from a remote node with server-side filtering.
// filterEmails: when non-empty, the agent only returns lines containing one of these emails.
// startTime/endTime: Unix timestamps for time-range filtering (0 = no limit).
// This is used for client log packaging to avoid transferring entire log files.
func (m *NodeServerManager) ReadLogFileOnServerFiltered(ctx context.Context, serverID int64, logType string, lastLines int, filterEmails []string, startTime, endTime int64) ([]string, string, error) {
	return m.ReadLogFileOnServerWithPathFiltered(ctx, serverID, logType, "", lastLines, filterEmails, startTime, endTime)
}

// ReadLogFileOnServerWithPath reads the last N lines of a specific log file from a node.
// If logPath is non-empty, the agent reads that file directly instead of resolving by logType.
func (m *NodeServerManager) ReadLogFileOnServerWithPath(ctx context.Context, serverID int64, logType, logPath string, lastLines int) ([]string, string, error) {
	return m.ReadLogFileOnServerWithPathFiltered(ctx, serverID, logType, logPath, lastLines, nil, 0, 0)
}

// ReadLogFileOnServerWithPathFiltered reads the last N lines of a specific log file from a node
// with optional server-side filtering. For remote nodes, filterEmails/startTime/endTime are
// sent to the agent so it can pre-filter lines before transmission, dramatically reducing
// network payload for client log packaging.
func (m *NodeServerManager) ReadLogFileOnServerWithPathFiltered(ctx context.Context, serverID int64, logType, logPath string, lastLines int, filterEmails []string, startTime, endTime int64) ([]string, string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, "", fmt.Errorf("local agent not configured")
		}
		// For local: if logPath is specified, read that file directly
		if logPath != "" {
			resolved := logPath
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(m.localAgent.GetDataDir(), resolved)
			}
			data, err := os.ReadFile(resolved)
			if err != nil {
				return nil, resolved, err
			}
			lines := strings.Split(string(data), "\n")
			if len(lines) > lastLines {
				lines = lines[len(lines)-lastLines:]
			}
			return lines, resolved, nil
		}
		// Fall back to logType-based resolution
		resolved := filepath.Join(m.localAgent.GetDataDir(), "logs", logType+".log")
		switch logType {
		case "nginx":
			resolved = filepath.Join(m.localAgent.GetDataDir(), "logs", "nginx-error.log")
		case "caddy":
			resolved = filepath.Join(m.localAgent.GetDataDir(), "logs", "caddy-access.log")
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, resolved, err
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) > lastLines {
			lines = lines[len(lines)-lastLines:]
		}
		return lines, resolved, nil
	}

	// Remote: send via heartbeat stream
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_ReadLogFile{
			ReadLogFile: &agentpb.ReadLogFileRequest{
				LogType:      logType,
				LogPath:      logPath,
				LastLines:    int32(lastLines),
				FilterEmails: filterEmails,
				StartTime:    startTime,
				EndTime:      endTime,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, "", err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, "", fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.ReadLogFileResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, "", fmt.Errorf("unmarshal read_log_file response: %w", err)
	}
	if !resp.Success {
		return nil, "", fmt.Errorf("%s", resp.Error)
	}
	// Safety: resp.Lines may be nil when the remote node has no data.
	// Return empty slice instead of nil to avoid nil-pointer issues downstream.
	lines := resp.Lines
	if lines == nil {
		lines = []string{}
	}
	return lines, resp.LogPath, nil
}

// RotateLogOnServer triggers log rotation on a node (local or remote).
func (m *NodeServerManager) RotateLogOnServer(ctx context.Context, serverID int64, logType string) (int32, int32, int64, error) {
	if m.isLocalServer(serverID) {
		// For local: we don't have direct access to logmanage.Service here
		// This should be handled by the API handler directly
		return 0, 0, 0, fmt.Errorf("local rotation should be handled by API handler")
	}

	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_RotateLog{
			RotateLog: &agentpb.RotateLogRequest{
				LogType: logType,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return 0, 0, 0, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return 0, 0, 0, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.RotateLogResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return 0, 0, 0, fmt.Errorf("unmarshal rotate_log response: %w", err)
	}
	if !resp.Success {
		return 0, 0, 0, fmt.Errorf("%s", resp.Error)
	}
	return resp.RotatedFiles, resp.CleanedFiles, resp.FreedBytes, nil
}

// PackageLogsOnServer packages logs from a node within a time range.
func (m *NodeServerManager) PackageLogsOnServer(ctx context.Context, serverID int64, logType string, startTime, endTime int64, includeRotated, desensitize bool) (string, int64, string, error) {
	if m.isLocalServer(serverID) {
		return "", 0, "", fmt.Errorf("local packaging should be handled by API handler")
	}

	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_PackageLogs{
			PackageLogs: &agentpb.PackageLogsRequest{
				LogType:        logType,
				StartTime:      startTime,
				EndTime:        endTime,
				IncludeRotated: includeRotated,
				Desensitize:    desensitize,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 60*time.Second)
	if err != nil {
		return "", 0, "", err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return "", 0, "", fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.PackageLogsResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return "", 0, "", fmt.Errorf("unmarshal package_logs response: %w", err)
	}
	if !resp.Success {
		return "", 0, "", fmt.Errorf("%s", resp.Error)
	}
	return resp.PackagePath, resp.PackageSize, resp.DownloadToken, nil
}

// SyncLogConfigToServer syncs log output mode config to a remote node.
func (m *NodeServerManager) SyncLogConfigToServer(ctx context.Context, serverID int64, coreLogConfig, proxyLogConfig, nginxSettings string) error {
	if m.isLocalServer(serverID) {
		return nil // local config is already in the DB
	}

	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_SyncLogConfig{
			SyncLogConfig: &agentpb.SyncLogConfigRequest{
				CoreLogConfig:  coreLogConfig,
				ProxyLogConfig: proxyLogConfig,
				NginxSettings:  nginxSettings,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

// DiscoverLogsOnServer discovers log files on a remote node via gRPC.
func (m *NodeServerManager) DiscoverLogsOnServer(ctx context.Context, serverID int64) ([]*agentpb.DiscoveredLogInfo, error) {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_DiscoverLogs{
			DiscoverLogs: &agentpb.DiscoverLogsRequest{},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp agentpb.DiscoverLogsResponse
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal discover_logs response: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Logs, nil
}

// GetWriterStatusOnServer gets LogWriter status from a remote node via gRPC.
func (m *NodeServerManager) GetWriterStatusOnServer(ctx context.Context, serverID int64) ([]map[string]interface{}, error) {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_GetWriterStatus{
			GetWriterStatus: &agentpb.GetWriterStatusRequest{},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return nil, fmt.Errorf("%s", result.Error)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.DetailJson), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal writer_status response: %w", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		errMsg := "unknown error"
		if e, ok := resp["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	writersRaw, ok := resp["writers"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}
	writers := make([]map[string]interface{}, 0, len(writersRaw))
	for _, w := range writersRaw {
		if wm, ok := w.(map[string]interface{}); ok {
			writers = append(writers, wm)
		}
	}
	return writers, nil
}

// StartWriterOnServer starts a log writer on a remote node via gRPC.
func (m *NodeServerManager) StartWriterOnServer(ctx context.Context, serverID int64, proxyType, mode string) error {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_StartWriter{
			StartWriter: &agentpb.StartWriterRequest{
				ProxyType: proxyType,
				Mode:      mode,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

// StopWriterOnServer stops a log writer on a remote node via gRPC.
func (m *NodeServerManager) StopWriterOnServer(ctx context.Context, serverID int64, proxyType string) error {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_StopWriter{
			StopWriter: &agentpb.StopWriterRequest{
				ProxyType: proxyType,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

// UpdateWriterConfigOnServer updates a log writer's config on a remote node via gRPC.
func (m *NodeServerManager) UpdateWriterConfigOnServer(ctx context.Context, serverID int64, proxyType, mode string, retainDays int32, compress bool, maxSizeMB int32) error {
	cmd := &agentpb.PanelCommand{
		CommandId: generateCommandID(),
		Command: &agentpb.PanelCommand_UpdateWriterConfig{
			UpdateWriterConfig: &agentpb.UpdateWriterConfigRequest{
				ProxyType:  proxyType,
				Mode:       mode,
				RetainDays: retainDays,
				Compress:   compress,
				MaxSizeMb:  maxSizeMB,
			},
		},
	}
	result, err := m.SendCommandAndWait(ctx, serverID, cmd, 10*time.Second)
	if err != nil {
		return err
	}
	if result.Status == agentpb.TaskStatus_TASK_FAILED {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}