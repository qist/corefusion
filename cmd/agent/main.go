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
	"corefusion/pkg/netutil"
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
	settingRepo *repository.SystemSettingRepository

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

	// Set config generator: generates minimal default configs for each core type.
	// The Agent does NOT have access to the panel's core_configs DB table,
	// so it generates minimal valid configs locally. When the panel syncs a
	// full config via SyncConfig, it overwrites these defaults.
	procMgr.SetConfigGenerator(func(ctx context.Context, coreType string, configPath string) error {
		config := generateDefaultConfig(coreType, dataDir)
		if config == nil {
			return fmt.Errorf("no default config for core type: %s", coreType)
		}
		data, _ := json.MarshalIndent(config, "", "  ")
		return os.WriteFile(configPath, data, 0640)
	})

	// Phase 3b: Create binary installer (for remote core binary installation)
	binaryInstaller := binariesSvc.NewInstaller(dataDir, binaryResolver, &agentSettingStore{repo: settingRepo})

	// Phase 4: Create proxy.Manager (same as Panel)
	proxyMgr := proxySvc.NewManager(dataDir, proxySiteRepo, &agentSettingStore{repo: settingRepo})

	// Phase 5: Create LocalAgent — wraps process.Manager + proxy.Manager
	// This is the SAME LocalAgent used by the Panel for is_local=1 servers.
	// It implements NodeOperator by delegating to process.Manager / proxy.Manager.
	localAgent := agent.NewLocalAgent(dataDir, procMgr, svcRepo)
	localAgent = localAgent.WithProxyManager(agent.NewProxyManagerAdapter(proxyMgr))
	localAgent = localAgent.WithBinaryInstaller(agent.NewBinaryInstallerAdapter(binaryInstaller))

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
		settingRepo: settingRepo,
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

// syncGithubAccelSettings writes GitHub acceleration config to the Agent's local
// settings DB so that installer and proxyMgr can use it for downloads.
func (s *agentServiceServer) syncGithubAccelSettings(ctx context.Context, accelDomain, proxyURL string) {
	if s.settingRepo == nil {
		return
	}
	if accelDomain == "" && proxyURL == "" {
		return
	}
	// Build config JSON matching the panel's github.acceleration setting format
	cfg := map[string]interface{}{
		"enabled":   true,
		"proxy_url": proxyURL,
		"domains":   []interface{}{},
	}
	if accelDomain != "" {
		cfg["domains"] = []map[string]interface{}{
			{"name": "panel-synced", "url": accelDomain, "enabled": true},
		}
	}
	data, _ := json.Marshal(cfg)
	s.settingRepo.Set(ctx, "github.acceleration", string(data))
	log.Printf("[Agent] synced github.acceleration settings (accel=%s, proxy=%s)", accelDomain, proxyURL)
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
	cmdID := cmd.CommandId
	idempotencyKey := cmd.IdempotencyKey

	// Helper: submit a task to the LocalAgent's TaskQueue with a callback
	// that sends the TaskResult back to the panel via the heartbeat stream.
	submitTask := func(taskType string, fn agent.TaskFunc) {
		taskID := cmdID
		if taskID == "" {
			taskID = agent.GenerateTaskID()
		}
		a.localAgent.GetTaskQueue().Submit(taskID, taskType, idempotencyKey, fn, func(result *agentpb.TaskResult) {
			a.sendTaskResult(stream, result)
		})
	}

	switch c := cmd.Command.(type) {
	case *agentpb.PanelCommand_SyncConfig:
		req := c.SyncConfig
		submitTask("sync_config", func(ctx context.Context) (interface{}, error) {
			if err := a.localAgent.SyncConfig(ctx, req.CoreType, req.ConfigContent, req.ConfigVersion); err != nil {
				return nil, err
			}
			log.Printf("[Agent] config synced: %s (version=%d)", req.CoreType, req.ConfigVersion)
			return map[string]interface{}{"core_type": req.CoreType, "version": req.ConfigVersion}, nil
		})

	case *agentpb.PanelCommand_ControlCore:
		req := c.ControlCore
		submitTask("control_core", func(ctx context.Context) (interface{}, error) {
			if err := a.localAgent.ControlCore(ctx, req.CoreType, req.Action); err != nil {
				return nil, err
			}
			log.Printf("[Agent] ControlCore %s %s accepted", req.CoreType, req.Action)
			return map[string]interface{}{"core_type": req.CoreType, "action": req.Action}, nil
		})

	case *agentpb.PanelCommand_SyncProxyConfig:
		req := c.SyncProxyConfig
		submitTask("sync_proxy_config", func(ctx context.Context) (interface{}, error) {
			if err := a.localAgent.SyncProxyConfig(ctx, req.ProxyType, req.ConfigContent); err != nil {
				return nil, err
			}
			log.Printf("[Agent] proxy config synced: %s", req.ProxyType)
			return map[string]interface{}{"proxy_type": req.ProxyType}, nil
		})

	case *agentpb.PanelCommand_ControlProxy:
		req := c.ControlProxy
		submitTask("control_proxy", func(ctx context.Context) (interface{}, error) {
			if err := a.localAgent.ControlProxy(ctx, req.ProxyType, req.Action); err != nil {
				return nil, err
			}
			log.Printf("[Agent] ControlProxy %s %s accepted", req.ProxyType, req.Action)
			return map[string]interface{}{"proxy_type": req.ProxyType, "action": req.Action}, nil
		})

	case *agentpb.PanelCommand_AlterUsers:
		req := c.AlterUsers
		submitTask("alter_users", func(ctx context.Context) (interface{}, error) {
			if err := a.localAgent.AlterUsers(ctx, req); err != nil {
				return nil, err
			}
			log.Printf("[Agent] AlterUsers %s accepted", req.Action)
			return map[string]interface{}{"action": req.Action}, nil
		})

	case *agentpb.PanelCommand_RenewCert:
		log.Printf("[Agent] renewing mTLS certificate (expires %v)", time.Unix(c.RenewCert.ExpiresAt, 0))
		if err := os.WriteFile(a.certPath, c.RenewCert.NewCertPem, 0640); err != nil {
			log.Printf("[Agent] write renewed cert: %v", err)
		} else if err := os.WriteFile(a.keyPath, c.RenewCert.NewKeyPem, 0600); err != nil {
			log.Printf("[Agent] write renewed key: %v", err)
		} else {
			log.Println("[Agent] mTLS certificate renewed successfully")
		}

	case *agentpb.PanelCommand_DistributeCert:
		a.handleDistributeCert(c.DistributeCert)

	case *agentpb.PanelCommand_GetStatus:
		a.sendStatusReport(stream)

	case *agentpb.PanelCommand_UpdateAgent:
		log.Printf("[Agent] UpdateAgent command received: version=%s (not implemented)", c.UpdateAgent.Version)

	default:
		log.Printf("[Agent] unknown command: %T", cmd.Command)
	}
}

// sendTaskResult sends a TaskResult back to the panel via the heartbeat stream.
func (a *AgentApp) sendTaskResult(stream agentpb.HeartbeatService_StreamClient, result *agentpb.TaskResult) {
	report := &agentpb.AgentReport{
		Type:       agentpb.AgentReport_TASK_RESULT,
		ServerId:   a.cfg.Panel.ServerID,
		Timestamp:  time.Now().Unix(),
		TaskResult: result,
	}
	if err := stream.Send(report); err != nil {
		log.Printf("[Agent] failed to send task result (id=%s): %v", result.TaskId, err)
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
	dataDir    string
	settingRepo *repository.SystemSettingRepository
}

func (s *agentServiceServer) SyncConfig(ctx context.Context, req *agentpb.SyncConfigRequest) (*agentpb.TaskAccept, error) {
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("sync-config-%s-v%d", req.CoreType, req.ConfigVersion)
	return s.localAgent.GetTaskQueue().Submit(taskID, "sync_config", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.SyncConfig(ctx, req.CoreType, req.ConfigContent, req.ConfigVersion); err != nil {
				return nil, err
			}
			log.Printf("[Agent] config synced: %s (version=%d)", req.CoreType, req.ConfigVersion)
			return map[string]interface{}{"core_type": req.CoreType, "version": req.ConfigVersion}, nil
		},
		nil, // no stream callback for direct RPC
	), nil
}

func (s *agentServiceServer) ControlCore(ctx context.Context, req *agentpb.ControlCoreRequest) (*agentpb.TaskAccept, error) {
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("control-core-%s-%s", req.CoreType, req.Action)
	return s.localAgent.GetTaskQueue().Submit(taskID, "control_core", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.ControlCore(ctx, req.CoreType, req.Action); err != nil {
				return nil, err
			}
			return map[string]interface{}{"core_type": req.CoreType, "action": req.Action}, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) GetStatus(ctx context.Context, req *agentpb.GetStatusRequest) (*agentpb.GetStatusResponse, error) {
	return s.localAgent.GetStatus(ctx)
}

func (s *agentServiceServer) SyncProxyConfig(ctx context.Context, req *agentpb.SyncProxyConfigRequest) (*agentpb.TaskAccept, error) {
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("sync-proxy-config-%s", req.ProxyType)
	return s.localAgent.GetTaskQueue().Submit(taskID, "sync_proxy_config", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.SyncProxyConfig(ctx, req.ProxyType, req.ConfigContent); err != nil {
				return nil, err
			}
			return map[string]interface{}{"proxy_type": req.ProxyType}, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) ControlProxy(ctx context.Context, req *agentpb.ControlProxyRequest) (*agentpb.TaskAccept, error) {
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("control-proxy-%s-%s", req.ProxyType, req.Action)
	return s.localAgent.GetTaskQueue().Submit(taskID, "control_proxy", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.ControlProxy(ctx, req.ProxyType, req.Action); err != nil {
				return nil, err
			}
			return map[string]interface{}{"proxy_type": req.ProxyType, "action": req.Action}, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) AlterUsers(ctx context.Context, req *agentpb.AlterUsersRequest) (*agentpb.TaskAccept, error) {
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("alter-users-%s-%s", req.InboundTag, req.Action)
	return s.localAgent.GetTaskQueue().Submit(taskID, "alter_users", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.AlterUsers(ctx, req); err != nil {
				return nil, err
			}
			return map[string]interface{}{"action": req.Action}, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) ApplyLimits(ctx context.Context, req *agentpb.ApplyLimitsRequest) (*agentpb.TaskAccept, error) {
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("apply-limits-%s-%s", req.Action, req.Ip)
	return s.localAgent.GetTaskQueue().Submit(taskID, "apply_limits", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.ApplyLimits(ctx, req); err != nil {
				return nil, err
			}
			return nil, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) UpdateAgent(ctx context.Context, req *agentpb.UpdateAgentRequest) (*agentpb.TaskAccept, error) {
	return &agentpb.TaskAccept{Accepted: false, RejectReason: "not implemented"}, nil
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

// GetTask queries the status of an async task.
func (s *agentServiceServer) GetTask(ctx context.Context, req *agentpb.GetTaskRequest) (*agentpb.GetTaskResponse, error) {
	return s.localAgent.GetTaskQueue().GetTask(req.TaskId), nil
}

// XrayAPI proxies xray gRPC API calls to the local xray process.
// This allows the panel to control xray on remote nodes via the Agent.
func (s *agentServiceServer) XrayAPI(ctx context.Context, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	return s.localAgent.XrayAPIProxy(ctx, req)
}

// ListInterfaces returns the network interface names on the Agent's host.
func (s *agentServiceServer) ListInterfaces(ctx context.Context, req *agentpb.ListInterfacesRequest) (*agentpb.ListInterfacesResponse, error) {
	names, err := s.localAgent.ListInterfaces()
	if err != nil {
		return nil, err
	}
	return &agentpb.ListInterfacesResponse{Interfaces: names}, nil
}

// ExecuteBinary executes a core binary command on the Agent's host.
func (s *agentServiceServer) ExecuteBinary(ctx context.Context, req *agentpb.ExecuteBinaryRequest) (*agentpb.ExecuteBinaryResponse, error) {
	if req.RawOutput {
		out, err := s.localAgent.ExecuteBinaryRawOutput(ctx, req.CoreType, req.Args...)
		if err != nil {
			return &agentpb.ExecuteBinaryResponse{Success: false, Error: err.Error()}, nil
		}
		return &agentpb.ExecuteBinaryResponse{Success: true, Stdout: out}, nil
	}
	parsed, err := s.localAgent.ExecuteBinaryCommand(ctx, req.CoreType, req.Args...)
	if err != nil {
		return &agentpb.ExecuteBinaryResponse{Success: false, Error: err.Error()}, nil
	}
	return &agentpb.ExecuteBinaryResponse{Success: true, Parsed: parsed}, nil
}

func (s *agentServiceServer) InstallBinary(ctx context.Context, req *agentpb.InstallBinaryRequest) (*agentpb.TaskAccept, error) {
	// Sync GitHub acceleration settings to local DB before install
	s.syncGithubAccelSettings(ctx, req.AccelDomain, req.ProxyUrl)
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("install-binary-%s-%s", req.CoreType, req.Version)
	return s.localAgent.GetTaskQueue().Submit(taskID, "install_binary", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.InstallBinary(ctx, req.CoreType, req.Version, req.DownloadUrl, req.AssetName); err != nil {
				return nil, err
			}
			return map[string]interface{}{"core_type": req.CoreType, "version": req.Version}, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) InstallProxy(ctx context.Context, req *agentpb.InstallProxyRequest) (*agentpb.TaskAccept, error) {
	// Sync GitHub acceleration settings to local DB before install
	s.syncGithubAccelSettings(ctx, req.AccelDomain, req.ProxyUrl)
	taskID := agent.GenerateTaskID()
	idempotencyKey := fmt.Sprintf("install-proxy-%s", req.ProxyType)
	return s.localAgent.GetTaskQueue().Submit(taskID, "install_proxy", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := s.localAgent.InstallProxy(ctx, req.ProxyType); err != nil {
				return nil, err
			}
			return map[string]interface{}{"proxy_type": req.ProxyType}, nil
		},
		nil,
	), nil
}

func (s *agentServiceServer) GetProxyStatus(ctx context.Context, req *agentpb.GetProxyStatusRequest) (*agentpb.GetProxyStatusResponse, error) {
	return s.localAgent.GetProxyStatuses(), nil
}

func (s *agentServiceServer) ListBinaries(ctx context.Context, req *agentpb.ListBinariesRequest) (*agentpb.ListBinariesResponse, error) {
	return s.localAgent.ListBinaries(), nil
}

func (s *agentServiceServer) ListReleases(ctx context.Context, req *agentpb.ListReleasesRequest) (*agentpb.ListBinariesResponse, error) {
	return &agentpb.ListBinariesResponse{}, nil
}

// DownloadGeoRule downloads a geo rule file to the Agent's local data/rules/ directory.
func (s *agentServiceServer) DownloadGeoRule(ctx context.Context, req *agentpb.DownloadGeoRuleRequest) (*agentpb.DownloadGeoRuleResponse, error) {
	if req.FileName == "" || req.DownloadUrl == "" {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: "file_name and download_url are required"}, nil
	}

	rulesDir := filepath.Join(s.dataDir, "rules")
	tmpDir := filepath.Join(s.dataDir, "rules_tmp")
	if err := os.MkdirAll(rulesDir, 0750); err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: fmt.Sprintf("create rules dir: %v", err)}, nil
	}
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: fmt.Sprintf("create tmp dir: %v", err)}, nil
	}

	filePath := filepath.Join(rulesDir, req.FileName)
	tmpPath := filepath.Join(tmpDir, req.FileName)

	// Apply GitHub acceleration domain if provided
	downloadURL := req.DownloadUrl
	if req.AccelDomain != "" {
		downloadURL = netutil.AccelerateGitHubURL(downloadURL, req.AccelDomain)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}, nil
	}

	// Use HTTP client with GitHub acceleration and/or proxy support
	client := netutil.GitHubHTTPClientWithProxy(120*time.Second, req.AccelDomain, req.ProxyUrl)
	resp, err := client.Do(httpReq)
	if err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: fmt.Sprintf("download failed: HTTP %d", resp.StatusCode)}, nil
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}, nil
	}

	size, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}, nil
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return &agentpb.DownloadGeoRuleResponse{Success: false, Error: err.Error()}, nil
	}

	log.Printf("[Agent] geo rule downloaded: %s (%d bytes) → %s", req.FileName, size, filePath)
	return &agentpb.DownloadGeoRuleResponse{
		Success:  true,
		FileSize: size,
		FilePath: filePath,
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
	agentpb.RegisterAgentServiceServer(server, &agentServiceServer{localAgent: a.localAgent, dataDir: a.dataDir, settingRepo: a.settingRepo})

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

// generateDefaultConfig generates a minimal valid config for a core type.
// The Agent uses this when no config has been synced from the panel yet.
// Once the panel syncs a full config via SyncConfig, it overwrites these defaults.
func generateDefaultConfig(coreType string, dataDir string) map[string]interface{} {
	rulesDir := filepath.Join(dataDir, "rules")
	switch coreType {
	case "xray":
		return map[string]interface{}{
			"log": map[string]interface{}{
				"loglevel": "warning",
			},
			"api": map[string]interface{}{
				"tag":      "api",
				"services": []string{"HandlerService", "LoggerService", "StatsService", "RoutingService"},
			},
			"stats": map[string]interface{}{},
			"inbounds": []interface{}{
				map[string]interface{}{
					"tag":      "api",
					"listen":   "127.0.0.1",
					"port":     10085,
					"protocol": "dokodemo-door",
					"settings": map[string]interface{}{"address": "127.0.0.1"},
				},
			},
			"outbounds": []interface{}{
				map[string]interface{}{"protocol": "freedom", "tag": "direct"},
				map[string]interface{}{"protocol": "blackhole", "tag": "blocked"},
			},
			"routing": map[string]interface{}{
				"domainStrategy": "IPIfNonMatch",
				"rules": []interface{}{
					map[string]interface{}{
						"type":        "field",
						"inboundTag":  []string{"api"},
						"outboundTag": "api",
					},
				},
			},
			"policy": map[string]interface{}{
				"levels": map[string]interface{}{
					"0": map[string]interface{}{
						"statsUserUplink":   true,
						"statsUserDownlink": true,
					},
				},
				"system": map[string]interface{}{
					"statsInboundUplink":   true,
					"statsInboundDownlink": true,
				},
			},
		}
	case "singbox":
		return map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
			"inbounds":  []interface{}{},
			"outbounds": []interface{}{},
		}
	case "hysteria2":
		return map[string]interface{}{
			"log": map[string]interface{}{
				"level": "info",
			},
			"listen":  ":0",
			"masquerade": map[string]interface{}{
				"type": "404",
			},
		}
	case "tuic":
		return map[string]interface{}{
			"server":   "0.0.0.0:0",
			"users":    []interface{}{},
		}
	}
	_ = rulesDir // rulesDir used for future geo file path injection
	return nil
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
