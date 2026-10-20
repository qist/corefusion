package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"gorm.io/gorm"

	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"

	"corefusion/internal/coredata"
	"corefusion/internal/model/entity"
	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/runtime/agent/ratelimit"
	"corefusion/internal/runtime/process"
	"corefusion/internal/store/repository"
	"corefusion/internal/util/safepath"
)

// LocalAgent 本地节点适配器。
//
// 本地服务器（is_local=1）不需要 gRPC 连接到远程 Agent，
// 而是直接复用现有的进程管理基础设施：
//   - 核心进程 (xray/singbox) → process.Manager + healthcheck.Guardian
//   - 代理进程 (caddy/nginx)                → proxy.Manager + healthcheck.Guardian
//
// 所有控制操作都是异步的——方法提交任务后立刻返回，
// 实际执行在 TaskQueue 的 Worker 中进行。
// 结果通过 TaskResultCallback 回传（stream 或本地 channel）。
type LocalAgent struct {
	dataDir          string
	db               *gorm.DB
	serverID         int64
	procMgr          *process.Manager
	proxyMgr         ProxyManager // 代理进程管理器（Caddy/Nginx）
	svcRepo          *repository.ServiceRepository
	userOp           CoreUserOperator // 可选：用户热更新操作器
	limitOp          LimitApplier     // 可选：限速操作器
	binaryInst       BinaryInstaller  // 可选：核心二进制安装器
	statsAddr        string           // xray gRPC API 地址
	singboxStatsAddr string           // sing-box V2Ray API 地址 (experimental.v2ray_api.listen)
	taskQueue        *TaskQueue       // 异步任务队列

	// 限速管理器（nft + tc）
	rateLimitMgr *ratelimit.Manager

	// rateLimitInit 是"配置级 apply"（面板开启/更新限速、agent 重连补推）时用来
	// **落库 + 初始化 nft/tc 后端** 的回调。以前这条路径直接走 AssociateIP(ip="")，
	// 而 Manager 只在 agent 启动时 Init 过 → 面板里开限速在节点上什么都不建
	// （实测 192.168.2.50：journal 只有 `TASK_FAILED err=rate limit manager not initialized`）。
	rateLimitInit func(ctx context.Context, maxDown, maxUp int32) error

	// Persistent xray gRPC connection (lazy-initialized, reused across AlterUsers calls)
	xrayConn   *grpc.ClientConn
	xrayConnMu sync.Mutex
	xrayClient *coredata.XrayClient

	// Persistent sing-box V2Ray API client (lazy-initialized, reused across stats
	// calls)。以前每次查询都新建 XrayClient/连接且从不关闭，metrics 每 30s
	// 一轮会把 sing-box 的 gRPC 连接堆到数千条。
	singboxClient *coredata.XrayClient
}

// NewLocalAgent 创建本地节点适配器，直接复用现有的 process.Manager。
func NewLocalAgent(dataDir string, procMgr *process.Manager, svcRepo *repository.ServiceRepository) *LocalAgent {
	return &LocalAgent{
		dataDir:   dataDir,
		procMgr:   procMgr,
		svcRepo:   svcRepo,
		statsAddr: coredata.DefaultStatsAddr,
		taskQueue: NewTaskQueue(4),
	}
}

// GetDataDir returns the data directory path.
func (a *LocalAgent) GetDataDir() string {
	return a.dataDir
}

// WithProxyManager 注入代理进程管理器（proxy.Manager）。
func (a *LocalAgent) WithProxyManager(pm ProxyManager) *LocalAgent {
	a.proxyMgr = pm
	return a
}

// WithUserOperator 注入用户热更新操作器。
func (a *LocalAgent) WithUserOperator(op CoreUserOperator) *LocalAgent {
	a.userOp = op
	return a
}

// WithLimitApplier 注入限速操作器。
func (a *LocalAgent) WithLimitApplier(la LimitApplier) *LocalAgent {
	a.limitOp = la
	return a
}

// WithStatsAddr 设置 xray gRPC API 地址。
func (a *LocalAgent) WithStatsAddr(addr string) *LocalAgent {
	if addr != "" {
		a.statsAddr = addr
	}
	return a
}

// WithSingboxStatsAddr 设置 sing-box V2Ray API 地址。
// sing-box 的 V2Ray API 与 xray StatsService 协议兼容，但监听地址不同
// （从 experimental.v2ray_api.listen 获取，默认 127.0.0.1:8080）。
func (a *LocalAgent) WithSingboxStatsAddr(addr string) *LocalAgent {
	if addr != "" {
		a.singboxStatsAddr = addr
	}
	return a
}

// WithBinaryInstaller 注入核心二进制安装器。
func (a *LocalAgent) WithBinaryInstaller(bi BinaryInstaller) *LocalAgent {
	a.binaryInst = bi
	return a
}

// WithRateLimitManager 注入限速管理器。
func (a *LocalAgent) WithRateLimitManager(mgr *ratelimit.Manager) *LocalAgent {
	a.rateLimitMgr = mgr
	return a
}

// WithRateLimitInitFunc 注入"配置级 apply 时初始化限速后端"的回调（见 rateLimitInit 字段）。
func (a *LocalAgent) WithRateLimitInitFunc(fn func(ctx context.Context, maxDown, maxUp int32) error) *LocalAgent {
	a.rateLimitInit = fn
	return a
}

// WithDB injects the agent's local database for config snapshot persistence.
func (a *LocalAgent) WithDB(db *gorm.DB) *LocalAgent {
	a.db = db
	return a
}

// WithServerID sets the agent's own server ID (used as the snapshot key).
func (a *LocalAgent) WithServerID(id int64) *LocalAgent {
	a.serverID = id
	return a
}

// GetRateLimitManager 返回限速管理器。
func (a *LocalAgent) GetRateLimitManager() *ratelimit.Manager {
	return a.rateLimitMgr
}

// GetTaskQueue 返回 TaskQueue 供外部查询任务状态。
func (a *LocalAgent) GetTaskQueue() *TaskQueue {
	return a.taskQueue
}

// RestoreFromSnapshot re-applies the latest persisted config snapshot for each
// supported core type from the local database. Called at agent startup so the node
// can serve traffic even before the panel is reachable (data localization).
func (a *LocalAgent) RestoreFromSnapshot(ctx context.Context) {
	if a.db == nil {
		return
	}
	for _, coreType := range []string{"xray", "singbox"} {
		var snap entity.ConfigSnapshot
		if err := a.db.Where("server_id = ? AND core_type = ?", a.serverID, coreType).
			Order("version DESC").First(&snap).Error; err != nil {
			continue
		}
		if err := a.procMgr.WriteRuntimeConfig(coreType, []byte(snap.Config)); err != nil {
			log.Printf("[LocalAgent] restore snapshot %s failed: %v", coreType, err)
			continue
		}
		log.Printf("[LocalAgent] restored config snapshot: %s (version=%d, %d bytes)", coreType, snap.Version, len(snap.Config))
	}
}

// ============================================================
// 异步控制操作 — 提交到 TaskQueue，立刻返回
// ============================================================

// SyncConfig 写入配置文件到本地 runtime 目录（异步）。
// 提交到 TaskQueue 执行，通过 TaskResult 回传结果。
func (a *LocalAgent) SyncConfig(ctx context.Context, coreType string, config []byte, version int64) error {
	// coreType 会被拼成落盘路径（runtime/<coreType>.jsonc）✗ —— Agent 以 root 运行，
	// 面板下发的 `../../..` 会逃出数据目录 → 必须只接受"单纯名字"（安全审计 v5 H3/L11）。
	if err := safepath.CheckName("core type", coreType); err != nil {
		return err
	}
	taskID := GenerateTaskID()
	// Use content hash for idempotency key so that config changes always get applied.
	// Using version alone (e.g. sync-config-xray-v0) causes idempotent hits when
	// the same version number is reused, preventing new configs from being written.
	idempotencyKey := fmt.Sprintf("sync-config-%s-v%d-%s", coreType, version, hashConfig(config))

	a.taskQueue.Submit(taskID, "sync_config", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := a.procMgr.WriteRuntimeConfig(coreType, config); err != nil {
				return nil, fmt.Errorf("write config: %w", err)
			}
			// Persist a local config snapshot so the node can self-heal when the
			// panel is unreachable on restart.
			if a.db != nil {
				if err := a.db.Create(&entity.ConfigSnapshot{
					ServerID:  a.serverID,
					CoreType:  coreType,
					Version:   version,
					Config:    string(config),
					CreatedAt: time.Now(),
				}).Error; err != nil {
					log.Printf("[LocalAgent] save config snapshot %s failed: %v", coreType, err)
				}
			}
			log.Printf("[LocalAgent] config synced: %s (version=%d, %d bytes)", coreType, version, len(config))
			return map[string]interface{}{"core_type": coreType, "version": version}, nil
		},
		nil, // callback set by caller if needed
	)
	return nil
}

// hashConfig returns a short hash of the config content for idempotency.
func hashConfig(data []byte) string {
	if len(data) == 0 {
		return "empty"
	}
	// Use first 8 chars of SHA256 as a short content hash
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])[:8]
}

// SyncConfigSync 与 SyncConfig 相同（写运行时配置 + 落一份本地快照），但**同步**执行。
//
// ⚠ 为什么需要同步版本 ✗：sing-box 等**不支持热应用**的核心，配置下发后必须重启核心才能生效。
// 异步任务队列会让"写盘"与"重启读取配置"发生竞态（重启可能读到旧文件）→ 故这条路径要求
// 写盘先完成，调用方随后才重启（见 cmd/agent 的 applySyncedConfig）。
func (a *LocalAgent) SyncConfigSync(ctx context.Context, coreType string, config []byte, version int64) error {
	if err := safepath.CheckName("core type", coreType); err != nil {
		return err
	}
	if err := a.procMgr.WriteRuntimeConfig(coreType, config); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if a.db != nil {
		if err := a.db.Create(&entity.ConfigSnapshot{
			ServerID:  a.serverID,
			CoreType:  coreType,
			Version:   version,
			Config:    string(config),
			CreatedAt: time.Now(),
		}).Error; err != nil {
			log.Printf("[LocalAgent] save config snapshot %s failed: %v", coreType, err)
		}
	}
	return nil
}

// ControlCore 控制本地核心进程（异步）。
// 提交到 TaskQueue 执行，Worker 中调用 process.Manager。
func (a *LocalAgent) ControlCore(ctx context.Context, coreType, action string) error {
	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("control-core-%s-%s", coreType, action)

	a.taskQueue.Submit(taskID, "control_core", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			serviceID, err := a.resolveServiceID(coreType)
			if err != nil {
				return nil, err
			}
			switch action {
			case "start":
				return map[string]interface{}{"state": "starting"}, a.procMgr.Start(ctx, serviceID, coreType)
			case "stop":
				return map[string]interface{}{"state": "stopping"}, a.procMgr.Stop(ctx, serviceID)
			case "restart":
				return map[string]interface{}{"state": "restarting"}, a.procMgr.Restart(ctx, serviceID, coreType)
			default:
				return nil, fmt.Errorf("unknown action: %s", action)
			}
		},
		nil,
	)
	return nil
}

// SyncProxyConfig 写入代理配置文件到本地磁盘（异步）。
func (a *LocalAgent) SyncProxyConfig(ctx context.Context, proxyType string, config []byte) error {
	if a.proxyMgr == nil {
		return fmt.Errorf("proxy manager not configured")
	}

	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("sync-proxy-config-%s", proxyType)

	a.taskQueue.Submit(taskID, "sync_proxy_config", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if err := a.proxyMgr.WriteConfigFile(proxyType, string(config)); err != nil {
				return nil, err
			}
			log.Printf("[LocalAgent] proxy config synced: %s (%d bytes)", proxyType, len(config))
			return map[string]interface{}{"proxy_type": proxyType}, nil
		},
		nil,
	)
	return nil
}

// ControlProxy 控制本地代理进程（Caddy/Nginx）（异步）。
func (a *LocalAgent) ControlProxy(ctx context.Context, proxyType, action string) error {
	if a.proxyMgr == nil {
		return fmt.Errorf("proxy manager not configured")
	}

	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("control-proxy-%s-%s", proxyType, action)

	a.taskQueue.Submit(taskID, "control_proxy", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			switch action {
			case "start":
				return map[string]interface{}{"state": "running"}, a.proxyMgr.StartProxy(ctx, proxyType)
			case "stop":
				return map[string]interface{}{"state": "stopped"}, a.proxyMgr.StopProxy(ctx, proxyType)
			case "restart":
				return map[string]interface{}{"state": "restarting"}, a.proxyMgr.RestartProxy(ctx, proxyType)
			case "reload":
				return map[string]interface{}{"state": "running"}, a.proxyMgr.ReloadProxy(ctx, proxyType)
			default:
				return nil, fmt.Errorf("unknown action: %s", action)
			}
		},
		nil,
	)
	return nil
}

// AlterUsers 通过本地 gRPC AlterInbound 热更新用户（异步）。
func (a *LocalAgent) AlterUsers(ctx context.Context, req *agentpb.AlterUsersRequest) error {
	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("alter-users-%s-%s", req.InboundTag, req.Action)

	a.taskQueue.Submit(taskID, "alter_users", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if a.userOp != nil {
				return nil, a.userOp.AlterUsers(ctx, req)
			}
			return nil, a.alterUsersViaXrayGRPC(ctx, req)
		},
		nil,
	)
	return nil
}

// ApplyLimits 应用本地限速规则（异步）。
// 优先使用 ratelimit.Manager（nft + tc 完整限速），
// 如果未配置则回退到旧版 LimitApplier。
func (a *LocalAgent) ApplyLimits(ctx context.Context, req *agentpb.ApplyLimitsRequest) error {
	taskID := GenerateTaskID()

	// 优先使用新的 ratelimit.Manager
	if a.rateLimitMgr != nil {
		idempotencyKey := fmt.Sprintf("apply-limits-%s-%s", req.Action, req.Ip)
		a.taskQueue.Submit(taskID, "apply_limits", idempotencyKey,
			func(ctx context.Context) (interface{}, error) {
				return nil, a.applyLimitsViaManager(ctx, req)
			},
			nil,
		)
		return nil
	}

	// 回退到旧版 LimitApplier
	if a.limitOp == nil {
		return fmt.Errorf("limit applier not configured")
	}

	idempotencyKey := fmt.Sprintf("apply-limits-%s-%s", req.Action, req.Ip)
	a.taskQueue.Submit(taskID, "apply_limits", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			return nil, a.limitOp.ApplyLimits(ctx, req)
		},
		nil,
	)
	return nil
}

// AssociateIP 将 IP 关联到订阅并应用限速规则（异步）。
// 这是 Panel 推送 IP 归属的 Agent 侧入口。
func (a *LocalAgent) AssociateIP(ctx context.Context, req *agentpb.AssociateIPRequest) error {
	if a.rateLimitMgr == nil {
		return fmt.Errorf("rate limit manager not configured")
	}

	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("associate-ip-%s-%d", req.Ip, req.SubscriptionId)

	a.taskQueue.Submit(taskID, "associate_ip", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			rate := &ratelimit.RateConfig{
				MaxDownMbps: req.MaxDownMbps,
				MaxUpMbps:   req.MaxUpMbps,
				ConnLimit:   req.ConnLimit,
				DeviceLimit: req.DeviceLimit,
			}
			return nil, a.rateLimitMgr.AssociateIP(ctx, req.Ip, req.SubscriptionId, rate)
		},
		nil,
	)
	return nil
}

// applyLimitsViaManager 使用 ratelimit.Manager 执行限速操作。
func (a *LocalAgent) applyLimitsViaManager(ctx context.Context, req *agentpb.ApplyLimitsRequest) error {
	// 配置级 apply（不带 IP）：面板开启/更新限速、或 agent 重连补推时走这里。
	// 语义是"初始化/更新后端配置"，**不是**给某个 IP 套规则 —— 以前传空 IP 进 Manager.AssociateIP
	// 会直接报 `rate limit manager not initialized`，导致面板开了限速而节点上 nft/tc/IFB 全无。
	if req.Action == "apply" && req.Ip == "" {
		if a.rateLimitInit == nil {
			return fmt.Errorf("rate limit init callback not configured")
		}
		return a.rateLimitInit(ctx, req.MaxDownMbps, req.MaxUpMbps)
	}

	if a.rateLimitMgr == nil {
		return fmt.Errorf("rate limit manager not configured")
	}

	switch req.Action {
	case "apply":
		rate := &ratelimit.RateConfig{
			MaxDownMbps: req.MaxDownMbps,
			MaxUpMbps:   req.MaxUpMbps,
			ConnLimit:   req.ConnLimit,
			DeviceLimit: req.DeviceLimit,
		}
		return a.rateLimitMgr.AssociateIP(ctx, req.Ip, req.SubscriptionId, rate)

	case "remove":
		if req.Ip != "" {
			return a.rateLimitMgr.RemoveIP(ctx, req.Ip)
		}
		return a.rateLimitMgr.RemoveSubscription(ctx, req.SubscriptionId)

	case "update":
		rate := &ratelimit.RateConfig{
			MaxDownMbps: req.MaxDownMbps,
			MaxUpMbps:   req.MaxUpMbps,
			ConnLimit:   req.ConnLimit,
			DeviceLimit: req.DeviceLimit,
		}
		return a.rateLimitMgr.UpdateRate(ctx, req.SubscriptionId, rate)

	default:
		return fmt.Errorf("unknown apply-limits action: %s", req.Action)
	}
}

// SetIPBlacklist 应用面板级 IP 黑名单到本机 nft（同步执行，nft 命令毫秒级）。
// 不依赖 ratelimit.Manager 初始化 —— 限速未启用的节点同样要能拦黑名单。
func (a *LocalAgent) SetIPBlacklist(ctx context.Context, entries []string) error {
	return ratelimit.ApplyIPBlacklist(entries)
}

// ============================================================
// 同步查询操作 — 纯读，不走 TaskQueue
// ============================================================

// GetStatus 获取本地核心状态（同步查询）。
func (a *LocalAgent) GetStatus(ctx context.Context) (*agentpb.GetStatusResponse, error) {
	allProcs := a.procMgr.ListAll()
	cores := make([]*agentpb.CoreStatus, 0, len(allProcs))
	for _, p := range allProcs {
		cores = append(cores, &agentpb.CoreStatus{
			CoreType: p.CoreType,
			State:    p.State,
			Pid:      int32(p.PID),
			// Unix 秒：前端协议中心状态栏「启动时间」用（p.startedAt 由进程启动时记录，
			// 面板/Agent 重启恢复的进程可能是零值，退回 /proc/<pid>/stat 的 starttime）
			StartedAt:     startedAtSeconds(p),
			UptimeSeconds: p.Uptime,
			CpuUsage:      p.CPU,
			MemUsageBytes: p.Mem,
		})
	}
	return &agentpb.GetStatusResponse{Cores: cores}, nil
}

// startedAtSeconds 返回进程启动时间（Unix 秒）。
// p.startedAt 在正常启动时已记录；但 Agent 重启后从 guardian 恢复的进程
// 可能是零值，此时退回读 /proc/<pid>/stat（Linux）算真实启动时间，
// 避免状态栏「启动时间」显示 1970 或空。
func startedAtSeconds(p process.ProcessInfo) int64 {
	if !p.StartedAt.IsZero() {
		return p.StartedAt.Unix()
	}
	if t := process.GetProcessStartTime(p.PID); !t.IsZero() {
		return t.Unix()
	}
	return 0
}

// IsOnline 本地服务器永远在线。
func (a *LocalAgent) IsOnline() bool {
	return true
}

// GetProxyStatus 获取本地代理进程状态。
func (a *LocalAgent) GetProxyStatus(proxyType string) *ProxyInfo {
	if a.proxyMgr == nil {
		return nil
	}
	info := a.proxyMgr.DetectProxy(proxyType)
	return &ProxyInfo{
		Type:      info.Type,
		Name:      info.Name,
		Installed: info.Installed,
		Version:   info.Version,
		Path:      info.Path,
		Status:    info.Status,
		Managed:   info.Managed,
	}
}

// GetProxyStatuses 获取所有代理的安装和运行状态。
func (a *LocalAgent) GetProxyStatuses() *agentpb.GetProxyStatusResponse {
	resp := &agentpb.GetProxyStatusResponse{
		GuardianStatus: make(map[string]string),
	}
	if a.proxyMgr == nil {
		return resp
	}
	for _, pt := range []string{"caddy", "nginx"} {
		info := a.proxyMgr.DetectProxy(pt)
		resp.Proxies = append(resp.Proxies, &agentpb.ProxyInfo{
			Type:      info.Type,
			Name:      info.Name,
			Installed: info.Installed,
			Version:   info.Version,
			Path:      info.Path,
			Status:    info.Status,
			Managed:   info.Managed,
		})
		// Get guardian status and serialize to JSON
		guardianStatus := a.proxyMgr.GetGuardianStatus(pt)
		if statusJSON, err := json.Marshal(guardianStatus); err == nil {
			resp.GuardianStatus[pt] = string(statusJSON)
		}
	}
	return resp
}

// ListBinaries 列出已安装的核心二进制。
func (a *LocalAgent) ListBinaries() *agentpb.ListBinariesResponse {
	if a.binaryInst == nil {
		return &agentpb.ListBinariesResponse{}
	}
	items := a.binaryInst.ListInstalled()
	binaries := make([]*agentpb.BinaryInfo, 0, len(items))
	for _, item := range items {
		binaries = append(binaries, &agentpb.BinaryInfo{
			CoreType:   item.CoreType,
			Version:    item.Version,
			BinaryPath: item.BinaryPath,
			Installed:  item.Installed,
		})
	}
	return &agentpb.ListBinariesResponse{Binaries: binaries}
}

// InstallProxy 安装代理二进制（caddy/nginx）（异步）。
func (a *LocalAgent) InstallProxy(ctx context.Context, proxyType string) error {
	if a.proxyMgr == nil {
		return fmt.Errorf("proxy manager not configured")
	}
	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("install-proxy-%s", proxyType)
	a.taskQueue.Submit(taskID, "install_proxy", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			return nil, a.proxyMgr.InstallProxy(ctx, proxyType)
		},
		nil,
	)
	return nil
}

// InstallProxyVersion 安装指定版本的代理二进制（caddy/nginx）（异步）。
// downloadURL 为空时回退到自动获取最新版本。
func (a *LocalAgent) InstallProxyVersion(ctx context.Context, proxyType, downloadURL, tagName string) error {
	if a.proxyMgr == nil {
		return fmt.Errorf("proxy manager not configured")
	}
	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("install-proxy-%s-%s", proxyType, tagName)
	a.taskQueue.Submit(taskID, "install_proxy", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			if downloadURL != "" {
				if err := a.proxyMgr.DownloadAndInstallProxy(ctx, proxyType, downloadURL); err != nil {
					return nil, err
				}
			} else {
				if err := a.proxyMgr.InstallProxy(ctx, proxyType); err != nil {
					return nil, err
				}
			}
			return map[string]interface{}{"proxy_type": proxyType, "version": tagName}, nil
		},
		nil,
	)
	return nil
}

// InstallBinary 安装核心二进制（xray/singbox）（异步）。
func (a *LocalAgent) InstallBinary(ctx context.Context, coreType, version, downloadURL, assetName string) error {
	if a.binaryInst == nil {
		return fmt.Errorf("binary installer not configured")
	}
	taskID := GenerateTaskID()
	idempotencyKey := fmt.Sprintf("install-binary-%s-%s", coreType, version)
	a.taskQueue.Submit(taskID, "install_binary", idempotencyKey,
		func(ctx context.Context) (interface{}, error) {
			result, err := a.binaryInst.Install(ctx, coreType, version, downloadURL, assetName)
			if err != nil {
				return nil, err
			}
			return result, nil
		},
		nil,
	)
	return nil
}

// resolveServiceID 根据 coreType 查找对应的 service ID。
func (a *LocalAgent) resolveServiceID(coreType string) (int64, error) {
	if a.svcRepo == nil {
		return 0, fmt.Errorf("service repository not configured")
	}
	services, _, err := a.svcRepo.List(context.Background(), 1, coreType, 1, 0)
	if err != nil {
		return 0, fmt.Errorf("query service for %s: %w", coreType, err)
	}
	if len(services) == 0 {
		return 0, fmt.Errorf("no service found for core type %s", coreType)
	}
	return services[0].ID, nil
}

// ============================================================
// 内置 xray gRPC 客户端
// ============================================================

// alterUsersViaXrayGRPC 通过 xray gRPC API 执行用户增删改。
// 使用持久连接（lazy-initialized）避免频繁创建/销毁 gRPC 连接。
func (a *LocalAgent) alterUsersViaXrayGRPC(ctx context.Context, req *agentpb.AlterUsersRequest) error {
	client, err := a.getXrayClient(ctx)
	if err != nil {
		return fmt.Errorf("connect to xray: %w", err)
	}

	for _, user := range req.Users {
		switch req.Action {
		case "add":
			if err := a.addUserViaGRPC(ctx, client, req.InboundTag, user); err != nil {
				return err
			}
		case "remove":
			if err := a.removeUserViaGRPC(ctx, client, req.InboundTag, user.Email); err != nil {
				return err
			}
		case "update":
			_ = a.removeUserViaGRPC(ctx, client, req.InboundTag, user.Email)
			if err := a.addUserViaGRPC(ctx, client, req.InboundTag, user); err != nil {
				return err
			}
		}
	}
	return nil
}

// getXrayClient returns a persistent xray gRPC client, creating the connection
// lazily on first call. 建连逻辑复用 coredata.XrayClient，避免在写操作层重复
// grpc.DialContext + insecure + WithBlock。
func (a *LocalAgent) getXrayClient(ctx context.Context) (proxymanCommand.HandlerServiceClient, error) {
	a.xrayConnMu.Lock()
	defer a.xrayConnMu.Unlock()

	if a.xrayClient == nil {
		a.xrayClient = coredata.NewXrayClient(a.statsAddr)
	}
	conn, err := a.xrayClient.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to xray: %w", err)
	}
	a.xrayConn = conn
	return proxymanCommand.NewHandlerServiceClient(conn), nil
}

// Close releases resources held by the LocalAgent.
func (a *LocalAgent) Close() {
	if a.taskQueue != nil {
		a.taskQueue.Stop()
	}
	a.xrayConnMu.Lock()
	defer a.xrayConnMu.Unlock()
	if a.xrayConn != nil {
		a.xrayConn.Close()
		a.xrayConn = nil
	}
	if a.xrayClient != nil {
		a.xrayClient.Close()
		a.xrayClient = nil
	}
}

func (a *LocalAgent) addUserViaGRPC(ctx context.Context, client proxymanCommand.HandlerServiceClient, tag string, user *agentpb.UserOp) error {
	account := buildXrayAccount(tag, user)
	if account == nil {
		return fmt.Errorf("unsupported protocol for tag: %s", tag)
	}

	_, err := client.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(&proxymanCommand.AddUserOperation{User: &protocol.User{Email: user.Email, Account: account}}),
	})
	return err
}

func (a *LocalAgent) removeUserViaGRPC(ctx context.Context, client proxymanCommand.HandlerServiceClient, tag, email string) error {
	_, err := client.AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
		Tag:       tag,
		Operation: serial.ToTypedMessage(&proxymanCommand.RemoveUserOperation{Email: email}),
	})
	return err
}

// buildXrayAccount 根据入站标签构建 xray 账户。
func buildXrayAccount(tag string, user *agentpb.UserOp) *serial.TypedMessage {
	tagLower := strings.ToLower(tag)

	if strings.Contains(tagLower, "vless") {
		return serial.ToTypedMessage(&vless.Account{Id: user.Uuid, Flow: user.Flow})
	}
	if strings.Contains(tagLower, "vmess") {
		return serial.ToTypedMessage(&vmess.Account{Id: user.Uuid})
	}
	if strings.Contains(tagLower, "hysteria") {
		return serial.ToTypedMessage(&account.Account{Auth: user.Password})
	}
	if strings.Contains(tagLower, "shadowsocks") || strings.Contains(tagLower, "ss-") {
		if strings.HasPrefix(user.Encryption, "2022-blake3") {
			return serial.ToTypedMessage(&ss2022.Account{Key: user.Password})
		}
		cipherType := shadowsocks.CipherType_AES_256_GCM
		switch user.Encryption {
		case "aes-128-gcm":
			cipherType = shadowsocks.CipherType_AES_128_GCM
		case "chacha20-poly1305":
			cipherType = shadowsocks.CipherType_CHACHA20_POLY1305
		case "xchacha20-ietf-poly1305":
			cipherType = shadowsocks.CipherType_XCHACHA20_POLY1305
		case "none":
			cipherType = shadowsocks.CipherType_UNKNOWN
		}
		return serial.ToTypedMessage(&shadowsocks.Account{Password: user.Password, CipherType: cipherType})
	}
	if user.Password != "" {
		return serial.ToTypedMessage(&trojan.Account{Password: user.Password})
	}
	return nil
}

// ExecuteBinaryCommand executes a binary command and returns the parsed result.
func (a *LocalAgent) ExecuteBinaryCommand(ctx context.Context, coreType string, args ...string) (map[string]string, error) {
	binaryPath, err := a.resolveBinaryPath(coreType)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}

	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			result[key] = val
		}
	}
	return result, nil
}

// ExecuteBinaryRawOutput executes a binary command and returns raw stdout.
func (a *LocalAgent) ExecuteBinaryRawOutput(ctx context.Context, coreType string, args ...string) (string, error) {
	binaryPath, err := a.resolveBinaryPath(coreType)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}
	return string(out), nil
}

// ListInterfaces returns a list of network interface names.
func (a *LocalAgent) ListInterfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		names = append(names, iface.Name)
	}
	return names, nil
}

// resolveBinaryPath resolves the binary path for a core type.
func (a *LocalAgent) resolveBinaryPath(coreType string) (string, error) {
	binaryName := coreType + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(a.dataDir, "binaries", "active", coreType, binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("binary not installed for core type %s", coreType)
	}
	return binaryPath, nil
}

// ReadConfigFile reads the runtime config file for the given core type
// from the agent's data directory (runtime/{coreType}.jsonc).
// Also supports WAF file types (all read from the unified proxy/waf-rules/ dir):
//   - "waf_config"            → proxy/waf-rules/config.lua
//   - "waf_domain"            → proxy/waf-rules/domain.json
//   - "waf_rule:xxx.rule"     → proxy/waf-rules/xxx.rule
//   - "waf_rule_list"         → JSON list of .rule files in waf-rules/
//   - "caddy_waf_config"      → proxy/waf-rules/config.json
//   - "caddy_waf_domain"      → proxy/waf-rules/domain.json
//   - "caddy_waf_rule:xxx.rule"→ proxy/waf-rules/xxx.rule
//   - "caddy_waf_rule_list"   → JSON list of .rule files in waf-rules/
func (a *LocalAgent) ReadConfigFile(coreType string) ([]byte, string, error) {
	wafDir := filepath.Join(a.dataDir, "proxy", "waf-rules")
	// WAF file types
	switch {
	// --- NginxGuard WAF ---
	// config.lua lives with the Lua source at proxy/lua/waf/config.lua
	// (loaded by NginxGuard's Lua engine via lua_package_path).
	case coreType == "waf_config":
		p := filepath.Join(a.dataDir, "proxy", "lua", "waf", "config.lua")
		data, err := os.ReadFile(p)
		return data, p, err
	case coreType == "waf_domain":
		p := filepath.Join(wafDir, "domain.json")
		data, err := os.ReadFile(p)
		return data, p, err
	case strings.HasPrefix(coreType, "waf_rule:"):
		ruleName := strings.TrimPrefix(coreType, "waf_rule:")
		// ⚠ ruleName 直接 Join 会逃出 wafDir（`waf_rule:../../../etc/shadow`）✗ —— 见审计 v5 H3。
		if err := safepath.CheckName("waf rule name", ruleName); err != nil {
			return nil, "", err
		}
		p := filepath.Join(wafDir, ruleName)
		data, err := os.ReadFile(p)
		return data, p, err
	case coreType == "waf_rule_list":
		return a.listWAFRules()
	// --- CaddyGuard WAF ---
	case coreType == "caddy_waf_config":
		p := filepath.Join(wafDir, "config.json")
		data, err := os.ReadFile(p)
		return data, p, err
	case coreType == "caddy_waf_domain":
		p := filepath.Join(wafDir, "domain.json")
		data, err := os.ReadFile(p)
		return data, p, err
	case strings.HasPrefix(coreType, "caddy_waf_rule:"):
		ruleName := strings.TrimPrefix(coreType, "caddy_waf_rule:")
		if err := safepath.CheckName("caddy waf rule name", ruleName); err != nil {
			return nil, "", err
		}
		p := filepath.Join(wafDir, ruleName)
		data, err := os.ReadFile(p)
		return data, p, err
	case coreType == "caddy_waf_rule_list":
		return a.listWAFRules()
	}
	// Default: runtime config file
	if err := safepath.CheckName("core type", coreType); err != nil {
		return nil, "", err
	}
	configPath := filepath.Join(a.dataDir, "runtime", coreType+".jsonc")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, configPath, err
	}
	return data, configPath, nil
}

// listWAFRules returns a JSON-encoded list of .rule files in the unified WAF rules directory.
// Path: proxy/waf-rules/
func (a *LocalAgent) listWAFRules() ([]byte, string, error) {
	ruleDir := filepath.Join(a.dataDir, "proxy", "waf-rules")
	entries, err := os.ReadDir(ruleDir)
	if err != nil {
		// Return empty list if directory doesn't exist
		return []byte("[]"), ruleDir, nil
	}
	type ruleFile struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	var files []ruleFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".rule") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, ruleFile{Name: name, Size: info.Size()})
	}
	if files == nil {
		files = []ruleFile{}
	}
	data, err := json.Marshal(files)
	if err != nil {
		return nil, ruleDir, err
	}
	return data, ruleDir, nil
}
