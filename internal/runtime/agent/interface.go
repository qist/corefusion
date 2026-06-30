package agent

import (
	"context"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// NodeOperator 是对所有节点服务器操作的统一抽象。
// 上层业务代码（用户同步、配置同步、流量聚合等）不需要区分本地和远程，
// 只需通过 NodeOperator 接口操作即可。
//
// 对于 is_local=1 的服务器，操作委托给 LocalAgent：
//   - 核心进程 (xray/singbox/hysteria2/tuic) → 复用 process.Manager + healthcheck.Guardian
//   - 代理进程 (caddy/nginx)                → 复用 proxy.Manager + healthcheck.Guardian
//
// 对于远程服务器，操作通过 gRPC 调用 Agent。
type NodeOperator interface {
	// 核心配置管理
	SyncConfig(ctx context.Context, serverID int64, coreType string, config []byte, version int64) error
	ControlCore(ctx context.Context, serverID int64, coreType, action string) error

	// 代理配置管理（Caddy/Nginx）
	SyncProxyConfig(ctx context.Context, serverID int64, proxyType string, config []byte) error
	ControlProxy(ctx context.Context, serverID int64, proxyType, action string) error

	// 用户管理（热生效，不重启核心）
	AlterUsers(ctx context.Context, serverID int64, req *agentpb.AlterUsersRequest) error

	// 限速/并发
	ApplyLimits(ctx context.Context, serverID int64, req *agentpb.ApplyLimitsRequest) error

	// 状态查询
	GetStatus(ctx context.Context, serverID int64) (*agentpb.GetStatusResponse, error)
	IsOnline(serverID int64) bool
}

// CoreUserOperator 处理本地核心的用户热更新操作。
// 由现有的 xraycontrol / singbox / hysteria2 / tuic 适配器实现，
// LocalAgent 通过此接口委托，不重新实现 gRPC 调用逻辑。
type CoreUserOperator interface {
	// AlterUsers 对本地核心执行用户增删改操作。
	AlterUsers(ctx context.Context, req *agentpb.AlterUsersRequest) (*agentpb.AlterUsersResponse, error)
}

// LimitApplier 处理本地限速规则的应用。
// 由现有的 firewall/nftables 逻辑实现。
type LimitApplier interface {
	ApplyLimits(ctx context.Context, req *agentpb.ApplyLimitsRequest) (*agentpb.ApplyLimitsResponse, error)
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
}

// ProxyInfo 包含代理进程的安装和运行状态信息。
// 与 proxy.ProxyInfo 结构对应，但使用接口类型避免循环依赖。
type ProxyInfo struct {
	Type       string `json:"type"`
	Name       string `json:"name"`
	Installed  bool   `json:"installed"`
	Version    string `json:"version"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Managed    bool   `json:"managed"`
}
