// CoreFusion Agent — 远程节点守护进程
//
// 设计原则（plan-design.md §19.4）：
//   Agent 不重新实现进程管理。它直接复用面板现有的：
//     - process.Manager（核心进程管理 + healthcheck.Guardian 自动重启）
//     - proxy.Manager（Caddy/Nginx 代理进程管理 + Guardian）
//     - LocalAgent（NodeOperator 接口的本地实现，作为 gRPC 服务端委托层）
//
// Agent 职责：
//   1. Bootstrap：用 Bootstrap Token 向面板换取 mTLS 证书
//   2. 反向连接面板 gRPC Server，建立 HeartbeatService 双向 stream
//   3. 定时上报：进程状态、健康指标
//   4. 启动 AgentService gRPC Server，接收面板指令（委托给 LocalAgent → process.Manager / proxy.Manager）
//   5. 本地守护：核心/代理崩溃自动重启（Guardian，独立于面板连接）
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	agent "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/runtime/process"
	binariesSvc "corefusion/internal/service/binaries"
	proxySvc "corefusion/internal/service/proxy"
	"corefusion/internal/model/entity"
	"corefusion/internal/store/db"
	"corefusion/internal/store/repository"
)

const agentVersion = "1.0.0-dev"

// ============================================================
// Config
// ============================================================

type AgentConfig struct {
	Panel struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`      // Panel HTTP port (for bootstrap REST API)
		GRPCPort int    `yaml:"grpc_port"` // Panel gRPC port (for mTLS heartbeat)
		ServerID int64  `yaml:"server_id"`
	} `yaml:"panel"`
	Bootstrap struct {
		Token string `yaml:"token"`
	} `yaml:"bootstrap"`
	Agent struct {
		ListenPort int    `yaml:"listen_port"` // Agent gRPC server port (for Panel→Agent direct RPC)
		LogLevel   string `yaml:"log_level"`
		DataDir    string `yaml:"data_dir"`
	} `yaml:"agent"`
}

func loadConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Agent.DataDir == "" {
		cfg.Agent.DataDir = "/opt/corefusion-agent/data"
	}
	if cfg.Agent.ListenPort == 0 {
		cfg.Agent.ListenPort = 18444
	}
	if cfg.Panel.GRPCPort == 0 {
		cfg.Panel.GRPCPort = 18443
	}
	if cfg.Panel.Port == 0 {
		cfg.Panel.Port = 8080
	}
	return &cfg, nil
}

// ============================================================
// Main
// ============================================================

func main() {
	configPath := "config.yaml"
	for i, arg := range os.Args[1:] {
		if arg == "-config" && i+1 < len(os.Args[1:]) {
			configPath = os.Args[i+2]
		} else if strings.HasPrefix(arg, "-config=") {
			configPath = strings.TrimPrefix(arg, "-config=")
		}
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Ensure data directory structure
	dataDir := cfg.Agent.DataDir
	initDataDir(dataDir)
	certsDir := filepath.Join(dataDir, "certs")
	os.MkdirAll(certsDir, 0750)

	log.Printf("[Agent] version=%s, server_id=%d, panel=%s:%d, data_dir=%s",
		agentVersion, cfg.Panel.ServerID, cfg.Panel.Host, cfg.Panel.GRPCPort, dataDir)

	app, err := NewAgentApp(cfg)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Phase 1: Bootstrap (get mTLS cert from Panel) or load existing cert
	if err := app.EnsureCert(); err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	// Phase 2: Start process.Manager + Guardian (local daemon, independent of panel connection)
	app.StartLocalDaemons(ctx)

	// Phase 3: Start AgentService gRPC server (Panel→Agent direct RPC, mTLS)
	go app.StartGRPCServer(ctx)

	// Phase 4: Connect to Panel and run heartbeat loop (auto-reconnect)
	go app.RunHeartbeatLoop(ctx)

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[Agent] shutting down...")
	cancel()
	time.Sleep(2 * time.Second)
}

// ============================================================
// AgentApp
// ============================================================

type AgentApp struct {
	cfg      *AgentConfig
	dataDir  string
	certPath string
	keyPath  string
	caPath   string

	// Reused infrastructure (same as Panel)
	procMgr   *process.Manager
	proxyMgr  *proxySvc.Manager
	localAgent *agent.LocalAgent
	svcRepo   *repository.ServiceRepository
	database  *gorm.DB

	// gRPC connection to Panel
	grpcConn *grpc.ClientConn
}

func NewAgentApp(cfg *AgentConfig) (*AgentApp, error) {
	dataDir := cfg.Agent.DataDir

	// Phase 1: Initialize local SQLite database (same as Panel)
	dbPath := filepath.Join(dataDir, "sqlite", "agent.db")
	dbCfg := db.DefaultConfig()
	dbCfg.Path = dbPath
	if err := db.Init(dbCfg); err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	// Run migrations for process_guardians, inbound_services, etc.
	if err := migrateAgentTables(db.DB); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Phase 2: Create repositories (same as Panel)
	svcRepo := repository.NewServiceRepository(db.DB)
	guardianRepo := repository.NewProcessGuardianRepository(db.DB)
	crashLogRepo := repository.NewCrashLogRepository(db.DB)
	settingRepo := repository.NewSystemSettingRepository(db.DB)
	proxySiteRepo := repository.NewProxySiteRepository(db.DB)

	// Phase 3: Create binary resolver + process.Manager (same as Panel)
	binaryResolver := binariesSvc.NewResolver(dataDir)
	procMgr := process.NewManager(dataDir, svcRepo, guardianRepo, crashLogRepo, binaryResolver)

	// Phase 4: Create proxy.Manager (same as Panel)
	proxyMgr := proxySvc.NewManager(dataDir, proxySiteRepo, &agentSettingStore{repo: settingRepo})

	// Phase 5: Create LocalAgent — wraps process.Manager + proxy.Manager
	// This is the SAME LocalAgent used by the Panel for is_local=1 servers.
	// It implements NodeOperator by delegating to process.Manager / proxy.Manager.
	localAgent := agent.NewLocalAgent(dataDir, procMgr, svcRepo)
	localAgent = localAgent.WithProxyManager(agent.NewProxyManagerAdapter(proxyMgr))

	app := &AgentApp{
		cfg:        cfg,
		dataDir:    dataDir,
		certPath:   filepath.Join(dataDir, "certs", "agent-cert.pem"),
		keyPath:    filepath.Join(dataDir, "certs", "agent-key.pem"),
		caPath:     filepath.Join(dataDir, "certs", "ca-cert.pem"),
		procMgr:    procMgr,
		proxyMgr:   proxyMgr,
		localAgent: localAgent,
		svcRepo:    svcRepo,
		database:   db.DB,
	}

	return app, nil
}

// agentSettingStore adapts SystemSettingRepository to proxy.SettingStore interface.
type agentSettingStore struct {
	repo *repository.SystemSettingRepository
}

func (s *agentSettingStore) GetByKey(ctx context.Context, key string) (string, error) {
	return s.repo.GetByKey(ctx, key)
}

func (s *agentSettingStore) Set(ctx context.Context, key, value string) error {
	return s.repo.Set(ctx, key, value)
}

// StartLocalDaemons starts process.Manager's seed + reconcile, which provides
// independent local crash-recovery (Guardian) even if the panel is unreachable.
func (a *AgentApp) StartLocalDaemons(ctx context.Context) {
	a.procMgr.SeedCores(ctx)
	go a.procMgr.Reconcile(ctx)
	log.Println("[Agent] process.Manager started (Guardian auto-restart active)")
}

// ============================================================
// Bootstrap — exchange Bootstrap Token for mTLS certificate
// ============================================================

func (a *AgentApp) EnsureCert() error {
	if a.hasValidCert() {
		log.Println("[Agent] using existing mTLS certificate")
		return nil
	}

	if a.cfg.Bootstrap.Token == "" {
		return fmt.Errorf("no bootstrap token and no valid cert — configure bootstrap.token in config.yaml")
	}

	log.Println("[Agent] bootstrapping with panel...")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "agent-" + strconv.FormatInt(a.cfg.Panel.ServerID, 10)
	}

	bootstrapURL := fmt.Sprintf("http://%s:%d/api/v1/agent/bootstrap", a.cfg.Panel.Host, a.cfg.Panel.Port)
	reqBody, _ := json.Marshal(map[string]interface{}{
		"server_id":       a.cfg.Panel.ServerID,
		"bootstrap_token": a.cfg.Bootstrap.Token,
		"hostname":        hostname,
		"agent_version":   agentVersion,
	})

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(bootstrapURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("bootstrap request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("bootstrap failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Code string `json:"code"`
		Data struct {
			CertPEM   string `json:"cert_pem"`
			KeyPEM    string `json:"key_pem"`
			CACertPEM string `json:"ca_cert_pem"`
			ExpiresAt int64  `json:"cert_expires_at"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse bootstrap response: %w", err)
	}
	if apiResp.Code != "" && apiResp.Code != "OK" {
		return fmt.Errorf("bootstrap error: %s - %s", apiResp.Code, apiResp.Message)
	}

	if err := os.WriteFile(a.certPath, []byte(apiResp.Data.CertPEM), 0640); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(a.keyPath, []byte(apiResp.Data.KeyPEM), 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	if err := os.WriteFile(a.caPath, []byte(apiResp.Data.CACertPEM), 0640); err != nil {
		return fmt.Errorf("write CA cert: %w", err)
	}

	log.Printf("[Agent] bootstrap successful, cert expires at %v", time.Unix(apiResp.Data.ExpiresAt, 0))
	return nil
}

func (a *AgentApp) hasValidCert() bool {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil {
		return false
	}
	if _, err := os.ReadFile(a.caPath); err != nil {
		return false
	}
	for _, c := range cert.Certificate {
		parsed, err := x509.ParseCertificate(c)
		if err != nil {
			continue
		}
		if time.Now().After(parsed.NotAfter) {
			return false
		}
	}
	return true
}

// ============================================================
// Heartbeat Loop — connect to Panel gRPC, send reports, receive commands
// ============================================================

func (a *AgentApp) RunHeartbeatLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := a.connectAndStream(ctx); err != nil {
			log.Printf("[Agent] panel connection error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *AgentApp) dialPanel() (*grpc.ClientConn, error) {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}

	caPEM, err := os.ReadFile(a.caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caPool,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // Panel cert CN is hostname, relax for development
	}

	addr := fmt.Sprintf("%s:%d", a.cfg.Panel.Host, a.cfg.Panel.GRPCPort)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("dial panel: %w", err)
	}
	return conn, nil
}

func (a *AgentApp) connectAndStream(ctx context.Context) error {
	conn, err := a.dialPanel()
	if err != nil {
		return err
	}
	defer conn.Close()

	hbClient := agentpb.NewHeartbeatServiceClient(conn)
	stream, err := hbClient.Stream(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	// Send first heartbeat
	if err := a.sendHeartbeat(stream); err != nil {
		return fmt.Errorf("first heartbeat: %w", err)
	}
	log.Printf("[Agent] connected to panel, heartbeat stream active")

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()
	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()

	// Receive loop (Panel commands via stream)
	recvErr := make(chan error, 1)
	go func() {
		for {
			cmd, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			a.handlePanelCommand(cmd, stream)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvErr:
			return err
		case <-heartbeatTicker.C:
			if err := a.sendHeartbeat(stream); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
		case <-statusTicker.C:
			if err := a.sendStatusReport(stream); err != nil {
				return fmt.Errorf("status report: %w", err)
			}
		}
	}
}

func (a *AgentApp) sendHeartbeat(stream agentpb.HeartbeatService_StreamClient) error {
	cpu, mem := getSystemMetrics()
	return stream.Send(&agentpb.AgentReport{
		Type:          agentpb.AgentReport_HEARTBEAT,
		ServerId:      a.cfg.Panel.ServerID,
		Timestamp:     time.Now().Unix(),
		AgentVersion:  agentVersion,
		CpuUsage:      cpu,
		MemUsageBytes: mem,
	})
}

func (a *AgentApp) sendStatusReport(stream agentpb.HeartbeatService_StreamClient) error {
	// Get core states from LocalAgent → process.Manager.ListAll()
	resp, err := a.localAgent.GetStatus(context.Background())
	if err != nil {
		return err
	}
	return stream.Send(&agentpb.AgentReport{
		Type:      agentpb.AgentReport_STATUS,
		ServerId:  a.cfg.Panel.ServerID,
		Timestamp: time.Now().Unix(),
		Cores:     resp.Cores,
	})
}

// ============================================================
// Handle Panel Commands (received via heartbeat stream)
// Delegates to LocalAgent → process.Manager / proxy.Manager
// ============================================================

func (a *AgentApp) handlePanelCommand(cmd *agentpb.PanelCommand, stream agentpb.HeartbeatService_StreamClient) {
	ctx := context.Background()
	switch c := cmd.Command.(type) {
	case *agentpb.PanelCommand_SyncConfig:
		// LocalAgent.SyncConfig → os.WriteFile (data/runtime/<coreType>.jsonc)
		if err := a.localAgent.SyncConfig(ctx, c.SyncConfig.CoreType, c.SyncConfig.ConfigContent, c.SyncConfig.ConfigVersion); err != nil {
			log.Printf("[Agent] SyncConfig error: %v", err)
		} else {
			log.Printf("[Agent] config synced: %s (version=%d)", c.SyncConfig.CoreType, c.SyncConfig.ConfigVersion)
		}

	case *agentpb.PanelCommand_ControlCore:
		// LocalAgent.ControlCore → process.Manager.Start/Stop/Restart
		resp, err := a.localAgent.ControlCore(ctx, c.ControlCore.CoreType, c.ControlCore.Action)
		if err != nil {
			log.Printf("[Agent] ControlCore %s %s error: %v", c.ControlCore.CoreType, c.ControlCore.Action, err)
		} else {
			log.Printf("[Agent] ControlCore %s %s → state=%s", c.ControlCore.CoreType, c.ControlCore.Action, resp.State)
		}

	case *agentpb.PanelCommand_SyncProxyConfig:
		// LocalAgent.SyncProxyConfig → proxy.Manager.WriteConfigFile
		resp, err := a.localAgent.SyncProxyConfig(ctx, c.SyncProxyConfig.ProxyType, c.SyncProxyConfig.ConfigContent)
		if err != nil {
			log.Printf("[Agent] SyncProxyConfig error: %v", err)
		} else if resp.Success {
			log.Printf("[Agent] proxy config synced: %s", c.SyncProxyConfig.ProxyType)
		}

	case *agentpb.PanelCommand_ControlProxy:
		// LocalAgent.ControlProxy → proxy.Manager.StartProxy/StopProxy/...
		resp, err := a.localAgent.ControlProxy(ctx, c.ControlProxy.ProxyType, c.ControlProxy.Action)
		if err != nil {
			log.Printf("[Agent] ControlProxy error: %v", err)
		} else {
			log.Printf("[Agent] ControlProxy %s %s → state=%s", c.ControlProxy.ProxyType, c.ControlProxy.Action, resp.State)
		}

	case *agentpb.PanelCommand_AlterUsers:
		// LocalAgent.AlterUsers → xray gRPC AlterInbound (hot reload, no restart)
		resp, err := a.localAgent.AlterUsers(ctx, c.AlterUsers)
		if err != nil {
			log.Printf("[Agent] AlterUsers error: %v", err)
		} else {
			log.Printf("[Agent] AlterUsers %s: affected=%d", c.AlterUsers.Action, resp.AffectedCount)
		}

	case *agentpb.PanelCommand_RenewCert:
		// Panel pushes renewed mTLS cert — save to disk (hot swap)
		log.Printf("[Agent] renewing mTLS certificate (expires %v)", time.Unix(c.RenewCert.ExpiresAt, 0))
		if err := os.WriteFile(a.certPath, c.RenewCert.NewCertPem, 0640); err != nil {
			log.Printf("[Agent] write renewed cert: %v", err)
		} else if err := os.WriteFile(a.keyPath, c.RenewCert.NewKeyPem, 0600); err != nil {
			log.Printf("[Agent] write renewed key: %v", err)
		} else {
			log.Println("[Agent] mTLS certificate renewed successfully")
		}

	case *agentpb.PanelCommand_DistributeCert:
		// Panel distributes external service TLS cert (e.g. Let's Encrypt)
		a.handleDistributeCert(c.DistributeCert)

	case *agentpb.PanelCommand_GetStatus:
		// Panel requests immediate status — send via stream
		a.sendStatusReport(stream)

	case *agentpb.PanelCommand_UpdateAgent:
		log.Printf("[Agent] UpdateAgent command received: version=%s (not implemented)", c.UpdateAgent.Version)

	default:
		log.Printf("[Agent] unknown command: %T", cmd.Command)
	}
}

func (a *AgentApp) handleDistributeCert(cmd *agentpb.DistributeCertCommand) {
	if cmd.TargetPath == "" {
		log.Printf("[Agent] DistributeCert: no target_path for %s", cmd.CertName)
		return
	}
	dir := filepath.Dir(cmd.TargetPath)
	os.MkdirAll(dir, 0750)

	if err := os.WriteFile(cmd.TargetPath, cmd.CertPem, 0640); err != nil {
		log.Printf("[Agent] distribute cert write error: %v", err)
		return
	}
	// Write key alongside (same dir, key.pem)
	keyPath := filepath.Join(dir, "key.pem")
	os.WriteFile(keyPath, cmd.KeyPem, 0600)
	if len(cmd.FullchainPem) > 0 {
		fullchainPath := filepath.Join(dir, "fullchain.pem")
		os.WriteFile(fullchainPath, cmd.FullchainPem, 0640)
	}

	log.Printf("[Agent] certificate %s distributed to %s", cmd.CertName, dir)

	if cmd.ReloadCore {
		log.Printf("[Agent] reload core requested after cert distribution")
		// Core reload would be handled by process.Manager.Restart or Guardian
	}
}

// ============================================================
// AgentService gRPC Server (Panel→Agent direct RPC)
// Delegates to LocalAgent — same as how Panel handles is_local servers
// ============================================================

// agentServiceServer implements AgentService by delegating to LocalAgent.
// This is the remote equivalent of how the Panel's NodeServerManager
// delegates to LocalAgent for is_local=1 servers.
type agentServiceServer struct {
	agentpb.UnimplementedAgentServiceServer
	localAgent *agent.LocalAgent
}

func (s *agentServiceServer) SyncConfig(ctx context.Context, req *agentpb.SyncConfigRequest) (*agentpb.SyncConfigResponse, error) {
	if err := s.localAgent.SyncConfig(ctx, req.CoreType, req.ConfigContent, req.ConfigVersion); err != nil {
		return &agentpb.SyncConfigResponse{Success: false, Error: err.Error()}, nil
	}
	return &agentpb.SyncConfigResponse{Success: true, AppliedVersion: fmt.Sprintf("%d", req.ConfigVersion)}, nil
}

func (s *agentServiceServer) ControlCore(ctx context.Context, req *agentpb.ControlCoreRequest) (*agentpb.ControlCoreResponse, error) {
	return s.localAgent.ControlCore(ctx, req.CoreType, req.Action)
}

func (s *agentServiceServer) GetStatus(ctx context.Context, req *agentpb.GetStatusRequest) (*agentpb.GetStatusResponse, error) {
	return s.localAgent.GetStatus(ctx)
}

func (s *agentServiceServer) SyncProxyConfig(ctx context.Context, req *agentpb.SyncProxyConfigRequest) (*agentpb.SyncProxyConfigResponse, error) {
	return s.localAgent.SyncProxyConfig(ctx, req.ProxyType, req.ConfigContent)
}

func (s *agentServiceServer) ControlProxy(ctx context.Context, req *agentpb.ControlProxyRequest) (*agentpb.ControlProxyResponse, error) {
	return s.localAgent.ControlProxy(ctx, req.ProxyType, req.Action)
}

func (s *agentServiceServer) AlterUsers(ctx context.Context, req *agentpb.AlterUsersRequest) (*agentpb.AlterUsersResponse, error) {
	return s.localAgent.AlterUsers(ctx, req)
}

func (s *agentServiceServer) ApplyLimits(ctx context.Context, req *agentpb.ApplyLimitsRequest) (*agentpb.ApplyLimitsResponse, error) {
	return s.localAgent.ApplyLimits(ctx, req)
}

func (s *agentServiceServer) UpdateAgent(ctx context.Context, req *agentpb.UpdateAgentRequest) (*agentpb.UpdateAgentResponse, error) {
	return &agentpb.UpdateAgentResponse{Success: false, Error: "not implemented"}, nil
}

func (s *agentServiceServer) SpeedTest(ctx context.Context, req *agentpb.SpeedTestRequest) (*agentpb.SpeedTestResponse, error) {
	return &agentpb.SpeedTestResponse{Success: false, Error: "not implemented"}, nil
}

func (s *agentServiceServer) GetMetrics(ctx context.Context, req *agentpb.GetMetricsRequest) (*agentpb.GetMetricsResponse, error) {
	cpu, mem := getSystemMetrics()
	return &agentpb.GetMetricsResponse{
		Points: []*agentpb.MetricPoint{
			{
				Timestamp:     time.Now().Unix(),
				CpuUsage:      cpu,
				MemUsageBytes: mem,
			},
		},
	}, nil
}

func (a *AgentApp) StartGRPCServer(ctx context.Context) {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil {
		log.Printf("[Agent] failed to load cert for gRPC server: %v", err)
		return
	}

	caPEM, err := os.ReadFile(a.caPath)
	if err != nil {
		log.Printf("[Agent] failed to read CA for gRPC server: %v", err)
		return
	}
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.cfg.Agent.ListenPort))
	if err != nil {
		log.Printf("[Agent] failed to listen on :%d: %v", a.cfg.Agent.ListenPort, err)
		return
	}

	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
	agentpb.RegisterAgentServiceServer(server, &agentServiceServer{localAgent: a.localAgent})

	log.Printf("[Agent] gRPC server listening on :%d (mTLS)", a.cfg.Agent.ListenPort)
	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()

	if err := server.Serve(lis); err != nil {
		log.Printf("[Agent] gRPC server error: %v", err)
	}
}

// ============================================================
// System Metrics (for heartbeat reports)
// ============================================================

func getSystemMetrics() (cpuUsage float64, memUsageBytes int64) {
	if runtime.GOOS == "linux" {
		memUsageBytes = getLinuxMemUsage()
	}
	cpuUsage = getLinuxCPUUsage()
	return
}

func getLinuxMemUsage() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var memTotal, memAvailable int64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMemInfoLine(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = parseMemInfoLine(line)
		}
	}
	if memTotal > 0 {
		return (memTotal - memAvailable) * 1024
	}
	return 0
}

func parseMemInfoLine(line string) int64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	val, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func getLinuxCPUUsage() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return 0
	}
	load1, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	ncpu := runtime.NumCPU()
	if ncpu == 0 {
		ncpu = 1
	}
	usage := (load1 / float64(ncpu)) * 100
	if usage > 100 {
		usage = 100
	}
	return usage
}

// ============================================================
// Utility
// ============================================================

func initDataDir(dataDir string) {
	dirs := []string{
		filepath.Join(dataDir, "sqlite"),
		filepath.Join(dataDir, "certs"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "runtime"),
		filepath.Join(dataDir, "proxy"),
		filepath.Join(dataDir, "binaries", "active"),
		filepath.Join(dataDir, "binaries", "proxy"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0750)
	}
}

// migrateAgentTables runs migrations for the tables that process.Manager
// and proxy.Manager need (inbound_services, process_guardians, core_crash_logs,
// proxy_sites, system_settings).
func migrateAgentTables(database *gorm.DB) error {
	// Only migrate the tables needed by the Agent, not the full Panel schema.
	// We use AutoMigrate with a subset of entities.
	return database.AutoMigrate(
		&entity.InboundService{},
		&entity.ProcessGuardian{},
		&entity.CoreCrashLog{},
		&entity.ProxySite{},
		&entity.SystemSetting{},
	)
}
