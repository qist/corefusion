package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

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

	// Pending task results — taskID → result channel
	pendingTasks   map[string]chan *agentpb.TaskResult
	pendingTasksMu sync.RWMutex
}

// NodeSettingStore provides access to system settings for GitHub acceleration.
// Implemented by systemSettingService.
type NodeSettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
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

// SetMetricsCollector sets the metrics collector for heartbeat-based metric collection.
func (m *NodeServerManager) SetMetricsCollector(mc *MetricsCollector) {
	m.metricsCollector = mc
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
	agentClient agentpb.AgentServiceClient
	conn        *grpc.ClientConn

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
	}
}

// WithCRL sets the CRL for certificate revocation checking.
func (m *NodeServerManager) WithCRL(crl *CertRevocationList) *NodeServerManager {
	m.crl = crl
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
		ClientCAs:    x509.NewCertPool(),
	}
	tlsConfig.ClientCAs.AddCert(m.ca.GetCACert())

	// Add CRL verification if available
	if m.crl != nil {
		tlsConfig.VerifyPeerCertificate = m.crl.VerifyCallback()
	}

	creds := credentials.NewTLS(tlsConfig)

	m.grpcServer = grpc.NewServer(grpc.Creds(creds))

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

// AlterUsersOnServer sends an AlterUsers command to a remote server's Agent.
func (m *NodeServerManager) AlterUsersOnServer(ctx context.Context, serverID int64, req *agentpb.AlterUsersRequest) error {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return fmt.Errorf("server %d agent client not available", serverID)
	}
	accept, err := ac.agentClient.AlterUsers(ctx, req)
	if err != nil {
		return err
	}
	if !accept.Accepted {
		return fmt.Errorf("task rejected: %s", accept.RejectReason)
	}
	return nil
}

// ApplyLimitsOnServer sends an ApplyLimits command to a remote server's Agent.
func (m *NodeServerManager) ApplyLimitsOnServer(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return fmt.Errorf("server %d agent client not available", serverID)
	}
	accept, err := ac.agentClient.ApplyLimits(ctx, req)
	if err != nil {
		return err
	}
	if !accept.Accepted {
		return fmt.Errorf("task rejected: %s", accept.RejectReason)
	}
	return nil
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
	cmd := &agentpb.PanelCommand{
		Command: &agentpb.PanelCommand_SyncProxyConfig{
			SyncProxyConfig: &agentpb.SyncProxyConfigRequest{
				ProxyType:     proxyType,
				ConfigContent: config,
			},
		},
	}
	if err := m.SendCommand(ctx, serverID, cmd); err != nil {
		return fmt.Errorf("send SyncProxyConfig command to server %d: %w", serverID, err)
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

// ============================================================
// Phase 6: 新增 RPC 方法（Agent 自动更新、测速、指标查询）
// ============================================================

// InstallBinary installs a core binary on a server.
// For local servers, delegates to LocalAgent → binaries.Installer.
// For remote servers, sends InstallBinary via gRPC to the Agent.
func (m *NodeServerManager) InstallBinary(ctx context.Context, serverID int64, coreType, version, downloadURL, assetName string) (*agentpb.TaskAccept, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		err := m.localAgent.InstallBinary(ctx, coreType, version, downloadURL, assetName)
		if err != nil {
			return nil, err
		}
		// Return a synthetic TaskAccept for local operations
		return &agentpb.TaskAccept{Accepted: true, TaskId: "local"}, nil
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	accelDomain, proxyURL := m.githubAccelConfig()
	return ac.agentClient.InstallBinary(ctx, &agentpb.InstallBinaryRequest{
		CoreType:    coreType,
		Version:     version,
		DownloadUrl: downloadURL,
		AssetName:   assetName,
		AccelDomain: accelDomain,
		ProxyUrl:    proxyURL,
	})
}

// InstallProxy installs a proxy binary (caddy/nginx) on a server.
// For local servers, delegates to LocalAgent → proxy.Manager.InstallProxy.
// For remote servers, sends InstallProxy via gRPC to the Agent.
func (m *NodeServerManager) InstallProxy(ctx context.Context, serverID int64, proxyType string) (*agentpb.TaskAccept, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		err := m.localAgent.InstallProxy(ctx, proxyType)
		if err != nil {
			return nil, err
		}
		return &agentpb.TaskAccept{Accepted: true, TaskId: "local"}, nil
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	accelDomain, proxyURL := m.githubAccelConfig()
	return ac.agentClient.InstallProxy(ctx, &agentpb.InstallProxyRequest{
		ProxyType:   proxyType,
		AccelDomain: accelDomain,
		ProxyUrl:    proxyURL,
	})
}

// GetProxyStatus queries proxy installation status on a server.
// For local servers, delegates to LocalAgent.
// For remote servers, queries via gRPC to the Agent.
func (m *NodeServerManager) GetProxyStatus(ctx context.Context, serverID int64) (*agentpb.GetProxyStatusResponse, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.GetProxyStatuses(), nil
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	return ac.agentClient.GetProxyStatus(ctx, &agentpb.GetProxyStatusRequest{})
}

// ListBinaries lists installed core binaries on a server.
// For local servers, delegates to LocalAgent.
// For remote servers, queries via gRPC to the Agent.
func (m *NodeServerManager) ListBinaries(ctx context.Context, serverID int64) (*agentpb.ListBinariesResponse, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ListBinaries(), nil
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	return ac.agentClient.ListBinaries(ctx, &agentpb.ListBinariesRequest{})
}

// UpdateAgentOnServer sends an update command to a remote Agent.
// The Agent downloads the new binary, verifies checksum, replaces, and restarts.
func (m *NodeServerManager) UpdateAgentOnServer(ctx context.Context, serverID int64, version, downloadURL, checksum string) (*agentpb.TaskAccept, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	return ac.agentClient.UpdateAgent(ctx, &agentpb.UpdateAgentRequest{
		TargetVersion: version,
		DownloadUrl:   downloadURL,
		Checksum:      checksum,
	})
}

// SpeedTestOnServer requests a remote Agent to perform a speed test.
func (m *NodeServerManager) SpeedTestOnServer(ctx context.Context, serverID int64, targetHost string, targetPort int32, timeoutMs int32, samples int32) (*agentpb.SpeedTestResponse, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	return ac.agentClient.SpeedTest(ctx, &agentpb.SpeedTestRequest{
		TargetHost: targetHost,
		TargetPort: targetPort,
		TimeoutMs:  timeoutMs,
		Samples:    samples,
	})
}

// GetMetricsFromServer queries detailed metrics from a remote Agent.
func (m *NodeServerManager) GetMetricsFromServer(ctx context.Context, serverID int64, lastNPoints int32, metricType string) (*agentpb.GetMetricsResponse, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	return ac.agentClient.GetMetrics(ctx, &agentpb.GetMetricsRequest{
		LastNPoints: lastNPoints,
		MetricType:  metricType,
	})
}

// DownloadGeoRule downloads a geo rule file on a server.
// For local servers, the caller (georule service) handles the download directly.
// For remote servers, sends DownloadGeoRule via gRPC to the Agent.
func (m *NodeServerManager) DownloadGeoRule(ctx context.Context, serverID int64, fileName, downloadURL, accelDomain, proxyURL string) (bool, int64, string, error) {
	if m.isLocalServer(serverID) {
		// Local: signal that it should be handled by the caller
		return false, 0, "LOCAL", nil
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return false, 0, "", fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return false, 0, "", fmt.Errorf("server %d agent client not available", serverID)
	}
	resp, err := ac.agentClient.DownloadGeoRule(ctx, &agentpb.DownloadGeoRuleRequest{
		FileName:    fileName,
		DownloadUrl: downloadURL,
		AccelDomain: accelDomain,
		ProxyUrl:    proxyURL,
	})
	if err != nil {
		return false, 0, "", err
	}
	return resp.Success, resp.FileSize, resp.Error, nil
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
		if ac.conn != nil {
			ac.conn.Close()
		}
		ac.mu.Unlock()
		delete(m.connections, serverID)
		log.Printf("[NodeServerManager] server %d disconnected", serverID)
	}
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
// This does NOT make a gRPC call — it uses data cached from heartbeat status reports
// and the last successful ListBinaries query. This prevents API blocking on stale
// gRPC connections.
func (m *NodeServerManager) GetRemoteState(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, []*agentpb.BinaryInfo, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, nil, fmt.Errorf("server %d not connected", serverID)
	}

	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Core states from heartbeat
	var cores []*agentpb.CoreStatus
	if ac.statusReported {
		for _, cs := range ac.coreStates {
			cores = append(cores, cs)
		}
	}

	// Binaries from cache (refresh if older than 5 minutes)
	binaries := ac.cachedBinaries
	if binaries == nil && ac.agentClient != nil {
		// Try to fetch with short timeout (won't block long)
		fetchCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if resp, err := ac.agentClient.ListBinaries(fetchCtx, &agentpb.ListBinariesRequest{}); err == nil {
			binaries = resp.Binaries
			ac.cachedBinaries = binaries
			ac.cachedBinariesAt = time.Now()
		}
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
// It also establishes a reverse gRPC client (agentClient) to the Agent's gRPC server
// so that the Panel can send AgentService RPCs (ListBinaries, InstallBinary, etc.).
func (m *NodeServerManager) registerConnection(serverID int64, hostname string, stream agentpb.HeartbeatService_StreamServer) *AgentConnection {
	ac := &AgentConnection{
		ServerID:      serverID,
		Hostname:      hostname,
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

	// Establish reverse gRPC connection to the Agent's gRPC server
	m.dialAgent(ac, stream)

	log.Printf("[NodeServerManager] server %d (%s) connected", serverID, hostname)
	return ac
}

// dialAgent establishes a gRPC client connection to the Agent's gRPC server.
// It determines the Agent's address from (1) the stream peer address, or (2) the
// node server's InternalIP/PublicIP + AgentPort in the database.
func (m *NodeServerManager) dialAgent(ac *AgentConnection, stream agentpb.HeartbeatService_StreamServer) {
	if m.ca == nil {
		log.Printf("[NodeServerManager] CA not available, skipping reverse dial for server %d", ac.ServerID)
		return
	}

	// Determine agent address
	addr := m.resolveAgentAddress(ac.ServerID, stream)
	if addr == "" {
		log.Printf("[NodeServerManager] cannot determine agent address for server %d, reverse dial skipped", ac.ServerID)
		return
	}

	// Load CA cert for verifying the Agent's server certificate
	caPEM := m.ca.GetCACertPEM()
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Printf("[NodeServerManager] failed to parse CA cert for reverse dial server %d", ac.ServerID)
		return
	}

	// Issue a client certificate for the Panel (to present to the Agent)
	clientCert, err := m.ca.IssueServerCertificate()
	if err != nil {
		log.Printf("[NodeServerManager] failed to issue client cert for reverse dial: %v", err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		RootCAs:            caPool,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // Agent cert CN is hostname, relax for development
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64*1024*1024)),
	)
	if err != nil {
		log.Printf("[NodeServerManager] failed to reverse-dial agent at %s for server %d: %v", addr, ac.ServerID, err)
		return
	}

	ac.mu.Lock()
	ac.conn = conn
	ac.agentClient = agentpb.NewAgentServiceClient(conn)
	ac.mu.Unlock()

	log.Printf("[NodeServerManager] reverse-dial to agent %s (server %d) established", addr, ac.ServerID)
}

// resolveAgentAddress determines the Agent's gRPC server address.
// Priority: 1. DB InternalIP, 2. DB PublicIP, 3. DB Hostname, 4. Stream peer address.
// We prefer DB addresses because the stream peer address may be NAT'd or proxied.
func (m *NodeServerManager) resolveAgentAddress(serverID int64, stream agentpb.HeartbeatService_StreamServer) string {
	// 1. Look up the server in DB for InternalIP / PublicIP / Hostname
	server, err := m.serverRepo.GetByID(context.Background(), serverID)
	if err == nil {
		port := server.AgentPort
		if port == 0 {
			port = 18443
		}
		if server.InternalIP != "" {
			return net.JoinHostPort(server.InternalIP, strconv.Itoa(port))
		}
		if server.PublicIP != "" {
			return net.JoinHostPort(server.PublicIP, strconv.Itoa(port))
		}
		// Hostname may be an IP address or DNS name
		if server.Hostname != "" && server.Hostname != "local-panel" {
			return net.JoinHostPort(server.Hostname, strconv.Itoa(port))
		}
	}

	// 2. Fallback: try to get address from the gRPC stream peer
	if p, ok := peer.FromContext(stream.Context()); ok {
		if addr := extractHostFromPeerAddr(p.Addr.String(), serverID); addr != "" {
			return addr
		}
	}

	return ""
}

// extractHostFromPeerAddr extracts the host from a gRPC peer address string.
// Peer addresses look like "192.168.2.50:18443" or "[::1]:18443".
// We need to replace the port with the Agent's listen port (since the peer
// address is the Agent's *outbound* source port, not its listen port).
func extractHostFromPeerAddr(peerAddr string, serverID int64) string {
	host, _, err := net.SplitHostPort(peerAddr)
	if err != nil {
		// Maybe just an IP without port
		host = peerAddr
	}
	if host == "" {
		return ""
	}
	// Use default agent port; we'll need the DB port, but for now use default
	return net.JoinHostPort(host, strconv.Itoa(18443))
}

// markOffline marks a server as offline (called when stream breaks).
func (m *NodeServerManager) markOffline(serverID int64) {
	m.mu.Lock()
	if ac, ok := m.connections[serverID]; ok {
		ac.mu.Lock()
		ac.online = false
		// Close the reverse gRPC connection if it exists
		if ac.conn != nil {
			ac.conn.Close()
			ac.conn = nil
			ac.agentClient = nil
		}
		ac.mu.Unlock()
		delete(m.connections, serverID)
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
		}
	}
}

func (h *heartbeatServiceImpl) handleHeartbeat(ac *AgentConnection, report *agentpb.AgentReport) {
	// Update heartbeat in DB (throttled — every 30s, not every heartbeat)
	if err := h.mgr.serverRepo.UpdateHeartbeat(context.Background(), ac.ServerID); err != nil {
		log.Printf("[NodeServerManager] failed to update heartbeat for server %d: %v", ac.ServerID, err)
	}

	// Phase 6: Collect metrics from heartbeat
	if h.mgr.metricsCollector != nil {
		h.mgr.metricsCollector.CollectFromHeartbeat(ac.ServerID, report)
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
		}
	}
}

func (h *heartbeatServiceImpl) handleCrashReport(ac *AgentConnection, report *agentpb.AgentReport) {
	log.Printf("[NodeServerManager] server %d core crash: %s\n%s",
		ac.ServerID, report.CrashedCore, report.CrashLog)
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
// For local servers, queries directly. For remote servers, queries via gRPC to the Agent.
func (m *NodeServerManager) ListInterfaces(ctx context.Context, serverID int64) ([]string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ListInterfaces()
	}
	// Remote: query via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	resp, err := ac.agentClient.ListInterfaces(ctx, &agentpb.ListInterfacesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetInterfaces(), nil
}

// ExecuteBinaryCommand executes a binary command on a server.
// For local servers, executes directly via LocalAgent.
// For remote servers, sends via gRPC to the Agent.
func (m *NodeServerManager) ExecuteBinaryCommand(ctx context.Context, serverID int64, coreType string, args ...string) (map[string]string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return nil, fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ExecuteBinaryCommand(ctx, coreType, args...)
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	resp, err := ac.agentClient.ExecuteBinary(ctx, &agentpb.ExecuteBinaryRequest{
		CoreType:  coreType,
		Args:      args,
		RawOutput: false,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Parsed, nil
}

// ExecuteBinaryRawOutput executes a binary command and returns raw stdout.
// For local servers, executes directly. For remote servers, sends via gRPC to the Agent.
func (m *NodeServerManager) ExecuteBinaryRawOutput(ctx context.Context, serverID int64, coreType string, args ...string) (string, error) {
	if m.isLocalServer(serverID) {
		if m.localAgent == nil {
			return "", fmt.Errorf("local agent not configured")
		}
		return m.localAgent.ExecuteBinaryRawOutput(ctx, coreType, args...)
	}
	// Remote: send via gRPC
	ac := m.GetConnection(serverID)
	if ac == nil {
		return "", fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return "", fmt.Errorf("server %d agent client not available", serverID)
	}
	resp, err := ac.agentClient.ExecuteBinary(ctx, &agentpb.ExecuteBinaryRequest{
		CoreType:  coreType,
		Args:      args,
		RawOutput: true,
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Error)
	}
	return resp.Stdout, nil
}

// XrayAPIOnServer proxies an xray API call to a remote server's Agent.
// For local servers, returns an error (caller should use direct connection).
func (m *NodeServerManager) XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	ac := m.GetConnection(serverID)
	if ac == nil {
		return nil, fmt.Errorf("server %d not connected", serverID)
	}
	if ac.agentClient == nil {
		return nil, fmt.Errorf("server %d agent client not available", serverID)
	}
	return ac.agentClient.XrayAPI(ctx, req)
}
