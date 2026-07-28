package agent

import (
	"context"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// NodeOperator 是对所有节点服务器操作的统一抽象。
// 上层业务代码（用户同步、配置同步、流量聚合等）不需要区分本地和远程，
// 只需通过 NodeOperator 接口操作即可。
//
// 所有控制操作都是异步的——方法立刻返回 error，实际执行在后台进行。
// 调用方通过 NodeServerManager.GetTask() 或 stream 回传的 TaskResult
// 获取最终执行状态。
//
// 对于 is_local=1 的服务器，操作委托给 LocalAgent + TaskQueue：
//   - 核心进程 (xray/singbox/hysteria2/tuic) → process.Manager + Guardian
//   - 代理进程 (caddy/nginx)                → proxy.Manager + Guardian
//
// 对于远程服务器，操作通过 gRPC 调用 Agent（也是异步 TaskAccept）。
type NodeOperator interface {
	// 核心配置管理（异步）
	SyncConfig(ctx context.Context, serverID int64, coreType string, config []byte, version int64) error
	ControlCore(ctx context.Context, serverID int64, coreType, action string) error

	// 代理配置管理（Caddy/Nginx）（异步）
	SyncProxyConfig(ctx context.Context, serverID int64, proxyType string, config []byte) error
	ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error

	// 用户管理（热生效，不重启核心）（异步）
	AlterUsers(ctx context.Context, serverID int64, req *agentpb.AlterUsersRequest) error

	// 限速/并发（异步）
	ApplyLimits(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error

	// IP 归属推送（异步）— Panel 通知 Agent 某个 IP 属于哪个订阅
	AssociateIP(ctx context.Context, serverID int64, req *agentpb.AssociateIPRequest) error

	// 状态查询（同步，纯读）
	GetStatus(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error)
	IsOnline(serverID int64) bool
}

// CoreUserOperator 处理本地核心的用户热更新操作。
// 由现有的 xraycontrol / singbox / hysteria2 / tuic 适配器实现，
// LocalAgent 通过此接口委托，不重新实现 gRPC 调用逻辑。
type CoreUserOperator interface {
	// AlterUsers 对本地核心执行用户增删改操作。
	AlterUsers(ctx context.Context, req *agentpb.AlterUsersRequest) error
}

// LimitApplier 处理本地限速规则的应用。
// 由现有的 firewall/nftables 逻辑实现。
type LimitApplier interface {
	ApplyLimits(ctx context.Context, req *agentpb.ApplyLimitsRequest) error
}

// TrafficQuotaProcessor processes egress interface traffic data from heartbeats
// for monthly traffic quota monitoring. Implemented by nodeserver.NodeTrafficQuotaService.
type TrafficQuotaProcessor interface {
	ProcessHeartbeat(serverID int64, report *agentpb.AgentReport)
}

// ProxyManager 接口抽象代理进程管理能力。
// 由现有的 proxy.Manager 实现，LocalAgent 通过此接口委托，
// 不重新实现 Caddy/Nginx 的启动、停止、重启、配置生成等逻辑。
type ProxyManager interface {
	// StartProxy 启动代理进程（内部已集成 Guardian 自动重启）
	StartProxy(ctx context.Context, proxyType string) error
	// StopProxy 停止代理进程
	StopProxy(ctx context.Context, proxyType string) error
	// RestartProxy 重启代理进程
	RestartProxy(ctx context.Context, proxyType string) error
	// ReloadProxy 热重载配置（不重启进程）
	ReloadProxy(ctx context.Context, proxyType string) error
	// WriteConfigFile 写入配置文件到磁盘
	WriteConfigFile(proxyType string, config string) error
	// DetectProxy 检测代理安装状态和运行状态
	DetectProxy(proxyType string) ProxyInfo
	// InstallProxy 下载并安装代理二进制（Caddy/Nginx）
	InstallProxy(ctx context.Context, proxyType string) error
	// DownloadAndInstallProxy 下载指定版本的代理二进制并安装，如果代理正在运行则重启
	DownloadAndInstallProxy(ctx context.Context, proxyType, downloadURL string) error
	// GetGuardianStatus 获取 Guardian 状态（包含 CPU、内存等监控数据）
	GetGuardianStatus(proxyType string) map[string]interface{}
}

// BinaryInstaller 处理核心二进制的下载和安装。
// 由 binaries.Installer 实现，LocalAgent 通过此接口委托。
type BinaryInstaller interface {
	// Install 下载并安装核心二进制
	Install(ctx context.Context, coreType, version, downloadURL, assetName string) (*BinaryInstallResult, error)
	// ListInstalled 列出已安装的核心二进制
	ListInstalled() []BinaryInstallResult
}

// BinaryInstallResult 包含核心二进制安装结果信息。
type BinaryInstallResult struct {
	CoreType   string `json:"core_type"`
	Version    string `json:"version"`
	BinaryPath string `json:"binary_path"`
	Installed  bool   `json:"installed"`
}

// ProxyInfo 包含代理进程的安装和运行状态信息。
// 与 proxy.ProxyInfo 结构对应，但使用接口类型避免循环依赖。
type ProxyInfo struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Status    string `json:"status"`
	Managed   bool   `json:"managed"`
}
