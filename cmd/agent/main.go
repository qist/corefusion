// CoreFusion Agent — 远程节点守护进程
//
// 设计原则（plan-design.md §19.4）：
//
//	Agent 不重新实现进程管理。它直接复用面板现有的：
//	  - process.Manager（核心进程管理 + healthcheck.Guardian 自动重启）
//	  - proxy.Manager（Caddy/Nginx 代理进程管理 + Guardian）
//	  - LocalAgent（NodeOperator 接口的本地实现，作为 gRPC 服务端委托层）
//
// Agent 职责：
//  1. Bootstrap：用 Bootstrap Token 向面板换取 mTLS 证书
//  2. 反向连接面板 gRPC Server，建立 HeartbeatService 双向 stream
//  3. 定时上报：进程状态、健康指标
//  4. 通过 stream 接收面板指令（委托给 LocalAgent → process.Manager / proxy.Manager）
//  5. 本地守护：核心/代理崩溃自动重启（Guardian，独立于面板连接）
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"corefusion/internal/constants"
	agent "corefusion/internal/runtime/agent"
)

func main() {
	// 证书子命令（在**节点本机**做 ACME 签发）—— 不需要面板配置，先于配置加载处理 ✓。
	// 面板常在 SNAT 后面（只有出站）→ 公信 CA 验证不了面板；节点自己有公网/端口映射，
	// 所以"签发"必须在节点上执行 ✓。
	if len(os.Args) > 1 && os.Args[1] == "cert" {
		os.Exit(runCertSubcommand(os.Args[2:]))
	}

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
		constants.Version, cfg.Panel.ServerID, cfg.Panel.Host, cfg.Panel.GRPCPort, dataDir)

	app, err := NewAgentApp(cfg)
	if err != nil {
		log.Fatalf("init failed: %v", err)
	}

	// Check for crash recovery before starting (rolls back if new binary crashed)
	selfUpdater := agent.NewSelfUpdater(dataDir, app.settingRepo)
	selfUpdater.CheckCrashRecovery()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Phase 1: Bootstrap (get mTLS cert from Panel) or load existing cert
	if err := app.EnsureCert(); err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	// Phase 2: Start process.Manager + Guardian (local daemon, independent of panel connection)
	// This starts the Guardian which will auto-restart crashed cores.
	// At this point, if the Agent has local DB data from a previous SyncData,
	// the config generator will use it to generate the runtime config.
	app.StartLocalDaemons(ctx)

	// Phase 3: Connect to Panel and run heartbeat loop (auto-reconnect)
	// All Panel→Agent communication goes through the heartbeat stream.
	// No reverse-dial gRPC server is needed — this simplifies container/NAT deployment.
	// On first successful connection, the Agent will request SyncData from the panel
	// to ensure its local database is up-to-date.
	go app.RunHeartbeatLoop(ctx)

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[Agent] shutting down...")
	cancel()
	time.Sleep(2 * time.Second)
}
