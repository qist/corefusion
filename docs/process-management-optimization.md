# 进程管理架构优化方案

> 版本：v1.0 | 日期：2026-07-03
> 影响范围：`internal/runtime/agent/`, `internal/runtime/process/`, `internal/service/proxy/`, `cmd/agent/`, `cmd/corefusion/`

---

## 一、现状问题

### P0-1: 命令下发无回传（Fire-and-Forget）

**文件**：`internal/runtime/agent/manager.go` `SendCommand()`、`cmd/agent/main.go` `handlePanelCommand()`

**现状**：面板通过 heartbeat stream 向远程 Agent 下发 `ControlCore`/`ControlProxy` 命令后，`SendCommand()` 立即返回 nil。Agent 在 goroutine 中异步执行，成功/失败只打日志，**不回传任何结果**。

**影响**：
- HTTP API 返回 200 OK，但实际 Agent 可能执行失败
- 用户看到"重启成功"，进程可能根本没启起来
- 前端无法展示真实操作结果，只能事后轮询状态

**方案**：在 proto 中增加 `CommandResult` 消息类型，Agent 执行后通过 stream 回传。面板侧维护 `map[commandID]chan *CommandResult`，API handler 等待结果（带超时）后返回。

**改动文件**：
- `internal/runtime/agent/proto/corefusion_agent.proto` — 增加 `command_id` 字段和 `CommandResult` 消息
- `internal/runtime/agent/manager.go` — `SendCommand` 改为 `SendCommandAndWait`，维护 pending map
- `cmd/agent/main.go` — `handlePanelCommand` 中回传 `CommandResult`
- `internal/api/handlers/nodeserver/protocol_proxy.go` — 等待远程结果

---

### P0-2: `isLocalServer()` 每次查 DB

**文件**：`internal/runtime/agent/manager.go` `isLocalServer()`

**现状**：`NodeOperator` 的 11+ 个方法每个都调用 `isLocalServer(serverID)`，而该方法每次都查询 `nodeServerRepo.GetByID()`。本地节点 server_id=1 永远不变。

**影响**：不必要的 DB 查询，高频操作（如用户同步）会产生大量冗余 IO。

**方案**：启动时从 DB 加载一次 `localServerID`，缓存到 `NodeServerManager` 中。

**改动文件**：
- `internal/runtime/agent/manager.go` — 增加 `localServerID int64` 字段，`isLocalServer` 改为内存比较

---

### P1-1: `ControlCore`/`ControlProxy` 返回状态不准确

**文件**：`internal/runtime/agent/local.go` `ControlCore()`、`ControlProxy()`

**现状**：`process.Manager.Restart()` 和 `Stop()` 都是异步的（goroutine 执行），方法返回时操作尚未完成。但 `ControlCore` 在调用后立即查询 `IsRunning()`，返回的 state 可能是旧的。

**影响**：前端展示的状态不能反映实际操作进度。比如 restart 后显示 "running"（旧进程还没退出）。

**方案**：异步操作返回中间状态 `"restarting"` / `"stopping"`，让前端知道操作进行中。后续通过 P0-1 的回传机制或轮询获取最终状态。

**改动文件**：
- `internal/runtime/agent/local.go` — `ControlCore`、`ControlProxy` 返回中间状态

---

### P1-2: LocalAgent 被创建了两次

**文件**：`cmd/corefusion/main.go` 第 192 行、`internal/api/router/router.go` 第 801 行

**现状**：
1. `main.go` 中创建 `localAgent`（只有 processMgr，**没有** ProxyManager 和 BinaryInstaller），注入 `nodeMgr`
2. `router.go` 中又创建 `localAgent`（完整版，有 ProxyManager 和 BinaryInstaller），注入 `nodeMgrForCRL`

两个实例指向同一个 `nodeMgr`（后者覆盖前者），如果 router 中的 `nodeMgrForCRL` 和 main.go 中的 `nodeMgr` 不是同一个实例，第一个 `localAgent` 的代理操作会返回 "proxy manager not configured"。

**方案**：只创建一次 LocalAgent，在 main.go 中注入全部依赖后传给 router。

**改动文件**：
- `cmd/corefusion/main.go` — 调整初始化顺序
- `internal/api/router/router.go` — 接收外部 LocalAgent 而非自行创建

---

### P1-3: `RestartProxy` 同步阻塞 3s+

**文件**：`internal/service/proxy/manager.go` `RestartProxy()`

**现状**：`RestartProxy` = `StopProxy`（pkill + 1.5s 等待）+ `sleep(1s)` + `StartProxy`（生成配置 + 启动 + 0.5s 验证），总计 3+ 秒。HTTP handler 被阻塞。

对比：`process.Manager.Restart()` 是异步的（goroutine 执行），两者行为不一致。

此外，`StopProxy` 中的 `pkill -f caddy` + `pkill -9 -f caddy` 可能杀掉刚启动的新进程（竞态）。

**方案**：改为异步执行（与 `process.Manager.Restart()` 一致）：先停 Guardian，goroutine 中执行 stop + start。

**改动文件**：
- `internal/service/proxy/manager.go` — `RestartProxy` 改为异步

---

### P1-4: proxy.Manager Guardian 缺少指数退避和崩溃日志

**文件**：`internal/service/proxy/manager.go` `StartProxy()`

**现状**：`process.Manager` 的 Guardian 有指数退避、crash log 写入 DB、重启计数等功能。`proxy.Manager` 的 Guardian 配置为默认值，没有退避策略——如果配置文件错误，会快速无限循环重启，没有崩溃记录。

**方案**：为 proxy.Manager 增加退避配置，并记录崩溃日志到 DB（复用 `core_crash_logs` 表或新建 `proxy_crash_logs` 表）。

**改动文件**：
- `internal/service/proxy/manager.go` — Guardian 使用自定义 Config（带退避）
- `internal/model/entity/` — 可选：新增 `ProxyCrashLog` 实体

---

### P2-1: xray gRPC 每次新建连接

**文件**：`internal/runtime/agent/local.go` `alterUsersViaXrayGRPC()`

**现状**：每次 `AlterUsers` 都 `grpc.DialContext()` 建立新连接然后 `Close()`。频繁的用户同步操作产生大量短连接。

**方案**：使用持久连接，Lazy 初始化 + 连接健康检查。

**改动文件**：
- `internal/runtime/agent/local.go` — 增加 `xrayConn` 字段，懒初始化

---

### P2-2: `killProxyProcess` 使用 `pkill -f` 太粗暴

**文件**：`internal/service/proxy/manager.go` `killProxyProcess()`、`stopExisting()`

**现状**：`pkill -f caddy` 可能杀掉路径中包含 "caddy" 的其他进程。`StartProxy()` 已经启动了进程并记录 `cmd.Process.Pid`，但没有保存下来用于精确 kill。

**方案**：在 Manager 中保存 PID map，优先精确 kill，`pkill` 仅作 fallback。

**改动文件**：
- `internal/service/proxy/manager.go` — 增加 `processPIDs map[ProxyType]int`，kill 时优先用 PID

---

### P2-3: 配置写入职责不清，存在竞态写

**文件**：`internal/service/configsync/service.go`、`internal/runtime/agent/local.go` `SyncConfig()`、`internal/runtime/process/manager.go` `Start()` 的 ConfigGenerator

**现状**：三个地方都可以写 `runtime/<coreType>.jsonc`：
1. `configsync.Service.syncCore()` — 定时（5 分钟）从 DB 读配置 → `PrepareFullConfig` → 写文件
2. `LocalAgent.SyncConfig()` — 面板/远程命令写入原始 bytes → 写文件
3. `process.Manager.Start()` — `ConfigGenerator` → `PrepareFullConfig` → 写文件

如果 `configsync` 正在写文件，同时 `SyncConfig` 也在写，可能导致配置文件损坏或不一致。

**方案**：统一配置写入入口。`LocalAgent.SyncConfig()` 和 `configsync` 都通过 `process.Manager` 的 `WriteConfig()` 方法写入，加互斥锁保护。

**改动文件**：
- `internal/runtime/process/manager.go` — 增加带锁的 `WriteConfig()` 方法
- `internal/runtime/agent/local.go` — `SyncConfig` 委托给 processMgr
- `internal/service/configsync/service.go` — 同上

---

## 二、实施顺序

```
Phase 1（P0，核心体验）
  ├─ P0-2: isLocalServer 缓存     ← 最简单，5分钟改完
  ├─ P0-1: 命令回传机制            ← 核心，需要 proto 变更
  └─ P1-1: 返回中间状态           ← 依赖 P0-1

Phase 2（P1，架构对齐）
  ├─ P1-2: LocalAgent 单例        ← 依赖初始化顺序调整
  ├─ P1-3: RestartProxy 异步化    ← 与 process.Manager 对齐
  └─ P1-4: Guardian 退避+日志    ← 与 process.Manager 对齐

Phase 3（P2，锦上添花）
  ├─ P2-1: xray gRPC 连接池化
  ├─ P2-2: 精确 PID kill
  └─ P2-3: 配置写入统一入口
```

---

## 三、详细实现规范

### P0-1: 命令回传机制

#### Proto 变更

```protobuf
// corefusion_agent.proto

message PanelCommand {
  string command_id = 1;  // 新增：命令追踪 ID（UUID）
  oneof command { ... }
}

message AgentReport {
  enum ReportType {
    HEARTBEAT = 0; STATUS = 1; TRAFFIC = 2; CRASH = 3;
    COMMAND_RESULT = 4;  // 新增
  }
  ReportType type = 1;
  // ...existing fields...

  // COMMAND_RESULT
  CommandResult command_result = 50;
}

message CommandResult {
  string command_id = 1;
  bool   success = 2;
  string error = 3;
  int64  timestamp = 4;
  oneof detail {
    ControlCoreResponse core_detail = 5;
    ControlProxyResponse proxy_detail = 6;
  }
}
```

#### NodeServerManager 变更

```go
type NodeServerManager struct {
    // ...existing...
    pendingCmds   map[string]chan *agentpb.CommandResult  // commandID → result channel
    pendingCmdsMu sync.RWMutex
}

// SendCommandAndWait 发送命令并等待结果（带超时）。
func (m *NodeServerManager) SendCommandAndWait(
    ctx context.Context, serverID int64, cmd *agentpb.PanelCommand, timeout time.Duration,
) (*agentpb.CommandResult, error) {
    cmdID := uuid.NewString()
    cmd.CommandId = cmdID

    resultCh := make(chan *agentpb.CommandResult, 1)
    m.pendingCmdsMu.Lock()
    m.pendingCmds[cmdID] = resultCh
    m.pendingCmdsMu.Unlock()
    defer func() {
        m.pendingCmdsMu.Lock()
        delete(m.pendingCmds, cmdID)
        m.pendingCmdsMu.Unlock()
    }()

    if err := m.SendCommand(ctx, serverID, cmd); err != nil {
        return nil, err
    }

    select {
    case result := <-resultCh:
        return result, nil
    case <-time.After(timeout):
        return nil, fmt.Errorf("command %s timed out after %v", cmdID, timeout)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

#### Agent 侧变更

```go
// cmd/agent/main.go handlePanelCommand
func (a *AgentApp) handlePanelCommand(cmd *agentpb.PanelCommand, stream ...) {
    ctx := context.Background()
    cmdID := cmd.CommandId  // 保存 command_id

    switch c := cmd.Command.(type) {
    case *agentpb.PanelCommand_ControlCore:
        go func(coreType, action string) {
            resp, err := a.localAgent.ControlCore(ctx, coreType, action)
            result := &agentpb.CommandResult{
                CommandId: cmdID,
                Success:   err == nil && resp.Success,
                Error:     "",
                Timestamp: time.Now().Unix(),
                Detail:    &agentpb.CommandResult_CoreDetail{CoreDetail: resp},
            }
            if err != nil {
                result.Error = err.Error()
            }
            a.sendCommandResult(stream, result)
        }(c.ControlCore.CoreType, c.ControlCore.Action)
    // ...similar for ControlProxy...
    }
}

func (a *AgentApp) sendCommandResult(stream agentpb.HeartbeatService_StreamClient, result *agentpb.CommandResult) {
    report := &agentpb.AgentReport{
        Type:          agentpb.AgentReport_COMMAND_RESULT,
        ServerId:      a.cfg.Panel.ServerID,
        Timestamp:     time.Now().Unix(),
        CommandResult: result,
    }
    if err := stream.Send(report); err != nil {
        log.Printf("[Agent] failed to send command result: %v", err)
    }
}
```

#### 面板侧接收

```go
// manager.go handleCommandResult
func (h *heartbeatServiceImpl) handleCommandResult(ac *AgentConnection, report *agentpb.AgentReport) {
    if report.CommandResult == nil {
        return
    }
    cmdID := report.CommandResult.CommandId
    h.mgr.pendingCmdsMu.RLock()
    ch, ok := h.mgr.pendingCmds[cmdID]
    h.mgr.pendingCmdsMu.RUnlock()
    if ok {
        ch <- report.CommandResult
    }
}
```

---

### P0-2: isLocalServer 缓存

```go
type NodeServerManager struct {
    // ...existing...
    localServerID int64  // 缓存本地 server ID，0 表示未初始化
}

// InitLocalServerID 从 DB 加载本地 server ID，启动时调用一次。
func (m *NodeServerManager) InitLocalServerID(ctx context.Context) {
    servers, err := m.serverRepo.ListLocal(ctx)
    if err == nil && len(servers) > 0 {
        m.localServerID = servers[0].ID
    }
}

func (m *NodeServerManager) isLocalServer(serverID int64) bool {
    if m.localServerID > 0 {
        return serverID == m.localServerID
    }
    return serverID == 0 || serverID == 1  // fallback
}
```

---

### P1-1: 返回中间状态

```go
// local.go ControlCore
func (a *LocalAgent) ControlCore(ctx context.Context, coreType, action string) (*agentpb.ControlCoreResponse, error) {
    // ...
    switch action {
    case "start":
        err = a.procMgr.Start(ctx, serviceID, coreType)
        if err == nil {
            return &agentpb.ControlCoreResponse{Success: true, State: "starting"}, nil
        }
    case "stop":
        err = a.procMgr.Stop(ctx, serviceID)
        if err == nil {
            return &agentpb.ControlCoreResponse{Success: true, State: "stopping"}, nil
        }
    case "restart":
        err = a.procMgr.Restart(ctx, serviceID, coreType)
        if err == nil {
            return &agentpb.ControlCoreResponse{Success: true, State: "restarting"}, nil
        }
    }
    // ...
}

// local.go ControlProxy
func (a *LocalAgent) ControlProxy(ctx context.Context, proxyType, action string) (*agentpb.ControlProxyResponse, error) {
    // ...
    switch action {
    case "restart":
        err = a.proxyMgr.RestartProxy(ctx, proxyType)
        if err == nil {
            return &agentpb.ControlProxyResponse{Success: true, State: "restarting"}, nil
        }
    case "reload":
        err = a.proxyMgr.ReloadProxy(ctx, proxyType)
        if err == nil {
            return &agentpb.ControlProxyResponse{Success: true, State: "running"}, nil  // reload 完成后仍是 running
        }
    // ...
    }
}
```

---

### P1-2: LocalAgent 单例

```go
// main.go — 调整初始化顺序
func runServer(args []string) {
    // ...Phase 1-2...

    processMgr := process.NewManager(...)
    nodeMgr := agent.NewNodeServerManager(...)

    // 先创建 proxyMgr（提前到这里）
    proxySiteRepo := repository.NewProxySiteRepository(db.DB)
    proxyMgrInst := proxySvc.NewManager(dataDir, proxySiteRepo, systemSettingRepo)

    // 创建完整的 LocalAgent（一次）
    localAgent := agent.NewLocalAgent(dataDir, processMgr, svcRepo).
        WithProxyManager(agent.NewProxyManagerAdapter(proxyMgrInst)).
        WithBinaryInstaller(agent.NewBinaryInstallerAdapter(binaryInstaller))
    nodeMgr.SetLocalAgent(localAgent)

    // 传给 router
    router.SetLocalAgent(localAgent)
    router.SetProcessManager(processMgr)
    router.SetProxyManager(proxyMgrInst)
    // ...
}

// router.go — 接收外部 LocalAgent
var sharedLocalAgent *agent.LocalAgent

func SetLocalAgent(la *agent.LocalAgent) {
    sharedLocalAgent = la
}
```

---

### P1-3: RestartProxy 异步化

```go
func (m *Manager) RestartProxy(ctx context.Context, proxyType ProxyType) error {
    // 停止 Guardian 防止自动重启干扰
    m.mu.Lock()
    if g, ok := m.guardians[proxyType]; ok {
        g.Stop()
        delete(m.guardians, proxyType)
    }
    m.mu.Unlock()

    // 异步执行 stop + start
    go func() {
        m.stopExisting(proxyType)
        time.Sleep(time.Second)
        if err := m.StartProxy(context.Background(), proxyType); err != nil {
            log.Printf("proxy %s: async restart failed: %v", proxyType, err)
        } else {
            log.Printf("proxy %s: async restart completed", proxyType)
        }
    }()
    return nil
}
```

---

### P1-4: Guardian 退避 + 崩溃日志

```go
func (m *Manager) StartProxy(ctx context.Context, proxyType ProxyType) error {
    // ...existing code...

    // 使用带退避的 Guardian 配置
    cfg := healthcheck.DefaultConfig(string(proxyType), 0)
    cfg.MaxRestartCount = 5
    cfg.RestartWindowSec = 600    // 10 分钟窗口
    cfg.StableThresholdSec = 300  // 5 分钟稳定阈值

    guardian := healthcheck.New(cfg,
        func() bool {
            info := m.DetectProxy(proxyType)
            return info.Status == "running"
        },
        func(ctx context.Context) error {
            log.Printf("proxy %s: Guardian triggered restart (backoff)", proxyType)
            return m.StartProxy(ctx, proxyType)
        },
    )

    // ...existing code...
}
```

---

### P2-1: xray gRPC 连接持久化

```go
type LocalAgent struct {
    // ...existing...
    xrayConn    *grpc.ClientConn
    xrayConnMu  sync.Mutex
}

func (a *LocalAgent) getXrayConn() (proxymanCommand.HandlerServiceClient, error) {
    a.xrayConnMu.Lock()
    defer a.xrayConnMu.Unlock()

    if a.xrayConn != nil {
        // 健康检查
        if _, err := a.xrayConn.ResetConnectBackoff(); err == nil {
            return proxymanCommand.NewHandlerServiceClient(a.xrayConn), nil
        }
        a.xrayConn.Close()
        a.xrayConn = nil
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    conn, err := grpc.DialContext(ctx, a.statsAddr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithBlock(),
    )
    if err != nil {
        return nil, fmt.Errorf("connect to xray: %w", err)
    }
    a.xrayConn = conn
    return proxymanCommand.NewHandlerServiceClient(conn), nil
}

// Close 释放持久连接
func (a *LocalAgent) Close() {
    a.xrayConnMu.Lock()
    defer a.xrayConnMu.Unlock()
    if a.xrayConn != nil {
        a.xrayConn.Close()
    }
}
```

---

### P2-2: 精确 PID kill

```go
type Manager struct {
    // ...existing...
    processPIDs map[ProxyType]int  // 新增：记录启动的进程 PID
}

func (m *Manager) StartProxy(ctx context.Context, proxyType ProxyType) error {
    // ...existing code...
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("start: %w", err)
    }

    // 保存 PID
    m.mu.Lock()
    m.processPIDs[proxyType] = cmd.Process.Pid
    m.mu.Unlock()
    // ...
}

func (m *Manager) killProxyProcess(proxyType ProxyType) error {
    m.mu.Lock()
    pid, hasPID := m.processPIDs[proxyType]
    if hasPID {
        delete(m.processPIDs, proxyType)
    }
    m.mu.Unlock()

    // 优先精确 kill
    if hasPID && pid > 0 {
        if isProcessAlive(pid) {
            syscall.Kill(pid, syscall.SIGTERM)
            // 等待优雅退出
            deadline := time.Now().Add(5 * time.Second)
            for isProcessAlive(pid) && time.Now().Before(deadline) {
                time.Sleep(100 * time.Millisecond)
            }
            if isProcessAlive(pid) {
                syscall.Kill(pid, syscall.SIGKILL)
            }
            return nil
        }
    }

    // Fallback: pkill
    return exec.Command("pkill", "-f", string(proxyType)).Run()
}
```

---

### P2-3: 配置写入统一入口

```go
// process/manager.go — 新增带锁的配置写入方法
func (m *Manager) WriteRuntimeConfig(coreType string, data []byte) error {
    m.configMu.Lock()
    defer m.configMu.Unlock()

    runtimeDir := filepath.Join(m.dataDir, "runtime")
    os.MkdirAll(runtimeDir, 0750)
    configPath := filepath.Join(runtimeDir, coreType+".jsonc")
    return os.WriteFile(configPath, data, 0640)
}

// local.go SyncConfig — 委托给 processMgr
func (a *LocalAgent) SyncConfig(ctx context.Context, coreType string, config []byte, version int64) error {
    if err := a.procMgr.WriteRuntimeConfig(coreType, config); err != nil {
        return fmt.Errorf("write config: %w", err)
    }
    log.Printf("[LocalAgent] config synced: %s (version=%d)", coreType, version)
    return nil
}

// configsync/service.go syncCore — 也委托给 processMgr
// 需要在 Service 中注入 processMgr 或 WriteRuntimeConfig 函数
```
