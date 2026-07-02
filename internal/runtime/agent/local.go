package agent

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"

	agentpb "corefusion/internal/runtime/agent/proto"
	"corefusion/internal/runtime/process"
	"corefusion/internal/store/repository"
)

// LocalAgent 本地节点适配器。
//
// 本地服务器（is_local=1）不需要 gRPC 连接到远程 Agent，
// 而是直接复用现有的进程管理基础设施：
//   - 核心进程 (xray/singbox/hysteria2/tuic) → process.Manager + healthcheck.Guardian
//   - 代理进程 (caddy/nginx)                → proxy.Manager + healthcheck.Guardian
//
// 这不是一个独立的进程管理系统——它是 process.Manager 和 proxy.Manager 的薄包装层，
// 将 NodeOperator 接口适配到已有的本地管理基础设施上。
type LocalAgent struct {
	dataDir       string
	procMgr       *process.Manager
	proxyMgr      ProxyManager      // 代理进程管理器（Caddy/Nginx）
	svcRepo       *repository.ServiceRepository
	userOp        CoreUserOperator  // 可选：用户热更新操作器
	limitOp       LimitApplier      // 可选：限速操作器
	binaryInst    BinaryInstaller   // 可选：核心二进制安装器
	statsAddr     string             // xray gRPC API 地址
}

// NewLocalAgent 创建本地节点适配器，直接复用现有的 process.Manager。
func NewLocalAgent(dataDir string, procMgr *process.Manager, svcRepo *repository.ServiceRepository) *LocalAgent {
	return &LocalAgent{
		dataDir:   dataDir,
		procMgr:   procMgr,
		svcRepo:   svcRepo,
		statsAddr: "127.0.0.1:10085",
	}
}

// WithProxyManager 注入代理进程管理器（proxy.Manager）。
// 注入后，LocalAgent 的代理管理方法将直接复用 proxy.Manager 的
// StartProxy/StopProxy/RestartProxy/ReloadProxy 等方法，
// 这些方法内部已集成 healthcheck.Guardian 自动重启。
func (a *LocalAgent) WithProxyManager(pm ProxyManager) *LocalAgent {
	a.proxyMgr = pm
	return a
}

// WithUserOperator 注入用户热更新操作器。
// 如果不注入，AlterUsers 将使用内置的 xray gRPC 客户端（与 xraycontrol.Handler 相同的连接方式）。
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

// WithBinaryInstaller 注入核心二进制安装器。
// 注入后，LocalAgent 的 InstallBinary 和 ListBinaries 方法将委托给安装器。
func (a *LocalAgent) WithBinaryInstaller(bi BinaryInstaller) *LocalAgent {
	a.binaryInst = bi
	return a
}

// SyncConfig 写入配置文件到本地 runtime 目录。
// 这与 configsync.Service.syncCore() 写入的路径完全一致：
//   data/runtime/<coreType>.jsonc
//
// 远程 Agent 收到 SyncConfig 后也是写入本地 runtime 目录，
// 本地服务器只是省去了 gRPC 传输，直接写文件。
func (a *LocalAgent) SyncConfig(ctx context.Context, coreType string, config []byte, version int64) error {
	runtimeDir := filepath.Join(a.dataDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0750); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	configPath := filepath.Join(runtimeDir, coreType+".jsonc")
	if err := os.WriteFile(configPath, config, 0640); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	log.Printf("[LocalAgent] config synced: %s (version=%d, %d bytes)", coreType, version, len(config))
	return nil
}

// ControlCore 控制本地核心进程。
// 直接委托给 process.Manager 的 Start/Stop/Restart，
// process.Manager 内部已经集成了 healthcheck.Guardian 的自动重启守护。
func (a *LocalAgent) ControlCore(ctx context.Context, coreType, action string) (*agentpb.ControlCoreResponse, error) {
	serviceID, err := a.resolveServiceID(coreType)
	if err != nil {
		return &agentpb.ControlCoreResponse{Success: false, Error: err.Error()}, err
	}

	switch action {
	case "start":
		err = a.procMgr.Start(ctx, serviceID, coreType)
	case "stop":
		err = a.procMgr.Stop(ctx, serviceID)
	case "restart":
		err = a.procMgr.Restart(ctx, serviceID, coreType)
	default:
		return &agentpb.ControlCoreResponse{Success: false, Error: fmt.Sprintf("unknown action: %s", action)},
			fmt.Errorf("unknown action: %s", action)
	}

	if err != nil {
		return &agentpb.ControlCoreResponse{Success: false, Error: err.Error()}, err
	}

	// 获取当前状态
	state := "stopped"
	pid := int32(0)
	if a.procMgr.IsRunning(serviceID) {
		state = "running"
	}

	return &agentpb.ControlCoreResponse{
		Success: true,
		State:   state,
		Pid:     pid,
	}, nil
}

// AlterUsers 通过本地 gRPC AlterInbound 热更新用户。
//
// 如果注入了 CoreUserOperator（如 xraycontrol 的服务层），直接委托。
// 否则使用内置的 xray gRPC 客户端——与 xraycontrol.Handler.AddUser/RemoveUser
// 使用完全相同的 xray-core gRPC 库和连接方式。
func (a *LocalAgent) AlterUsers(ctx context.Context, req *agentpb.AlterUsersRequest) (*agentpb.AlterUsersResponse, error) {
	if a.userOp != nil {
		return a.userOp.AlterUsers(ctx, req)
	}
	return a.alterUsersViaXrayGRPC(ctx, req)
}

// ApplyLimits 应用本地限速规则。
// 如果注入了 LimitApplier（如 firewall 服务），直接委托。
func (a *LocalAgent) ApplyLimits(ctx context.Context, req *agentpb.ApplyLimitsRequest) (*agentpb.ApplyLimitsResponse, error) {
	if a.limitOp != nil {
		return a.limitOp.ApplyLimits(ctx, req)
	}
	return &agentpb.ApplyLimitsResponse{Success: false, Error: "limit applier not configured"}, nil
}

// GetStatus 获取本地核心状态。
// 直接调用 process.Manager.ListAll()，该方法已包含 PID、状态、uptime、CPU、内存等信息。
func (a *LocalAgent) GetStatus(ctx context.Context) (*agentpb.GetStatusResponse, error) {
	allProcs := a.procMgr.ListAll()
	cores := make([]*agentpb.CoreStatus, 0, len(allProcs))
	for _, p := range allProcs {
		cores = append(cores, &agentpb.CoreStatus{
			CoreType:       p.CoreType,
			State:          p.State,
			Pid:           int32(p.PID),
			UptimeSeconds: p.Uptime,
			CpuUsage:      p.CPU,
			MemUsageBytes: p.Mem,
		})
	}
	return &agentpb.GetStatusResponse{Cores: cores}, nil
}

// IsOnline 本地服务器永远在线。
func (a *LocalAgent) IsOnline() bool {
	return true
}

// ============================================================
// 代理进程管理（Caddy/Nginx）— 直接复用 proxy.Manager
// ============================================================

// SyncProxyConfig 写入代理配置文件到本地磁盘。
// 直接委托给 proxy.Manager.WriteConfigFile，写入路径为：
//   data/proxy/Caddyfile  (caddy)
//   data/proxy/nginx.conf (nginx)
//
// 这与 proxy.Manager.SaveAndApply() 写入的路径完全一致。
func (a *LocalAgent) SyncProxyConfig(ctx context.Context, proxyType string, config []byte) (*agentpb.SyncProxyConfigResponse, error) {
	if a.proxyMgr == nil {
		return &agentpb.SyncProxyConfigResponse{Success: false, Error: "proxy manager not configured"}, nil
	}
	if err := a.proxyMgr.WriteConfigFile(proxyType, string(config)); err != nil {
		return &agentpb.SyncProxyConfigResponse{Success: false, Error: err.Error()}, err
	}
	log.Printf("[LocalAgent] proxy config synced: %s (%d bytes)", proxyType, len(config))
	return &agentpb.SyncProxyConfigResponse{Success: true}, nil
}

// ControlProxy 控制本地代理进程（Caddy/Nginx）。
// 直接委托给 proxy.Manager 的 StartProxy/StopProxy/RestartProxy/ReloadProxy，
// proxy.Manager 内部已集成 healthcheck.Guardian 的自动重启守护。
func (a *LocalAgent) ControlProxy(ctx context.Context, proxyType, action string) (*agentpb.ControlProxyResponse, error) {
	if a.proxyMgr == nil {
		return &agentpb.ControlProxyResponse{Success: false, Error: "proxy manager not configured"}, nil
	}

	var err error
	switch action {
	case "start":
		err = a.proxyMgr.StartProxy(ctx, proxyType)
	case "stop":
		err = a.proxyMgr.StopProxy(ctx, proxyType)
	case "restart":
		err = a.proxyMgr.RestartProxy(ctx, proxyType)
	case "reload":
		err = a.proxyMgr.ReloadProxy(ctx, proxyType)
	default:
		return &agentpb.ControlProxyResponse{Success: false, Error: fmt.Sprintf("unknown action: %s", action)},
			fmt.Errorf("unknown action: %s", action)
	}

	if err != nil {
		return &agentpb.ControlProxyResponse{Success: false, Error: err.Error()}, err
	}

	// 获取当前状态
	state := "stopped"
	info := a.proxyMgr.DetectProxy(proxyType)
	if info.Status == "running" {
		state = "running"
	}

	return &agentpb.ControlProxyResponse{Success: true, State: state}, nil
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

// ============================================================
// 二进制安装（核心 + 代理）— 委托给 BinaryInstaller / ProxyManager
// ============================================================

// InstallBinary 安装核心二进制（xray/singbox/hysteria2/tuic）。
func (a *LocalAgent) InstallBinary(ctx context.Context, coreType, version, downloadURL, assetName string) (*agentpb.InstallBinaryResponse, error) {
	if a.binaryInst == nil {
		return &agentpb.InstallBinaryResponse{Success: false, Error: "binary installer not configured"}, nil
	}
	result, err := a.binaryInst.Install(ctx, coreType, version, downloadURL, assetName)
	if err != nil {
		return &agentpb.InstallBinaryResponse{Success: false, Error: err.Error()}, nil
	}
	log.Printf("[LocalAgent] binary installed: %s (version=%s, path=%s)", coreType, result.Version, result.BinaryPath)
	return &agentpb.InstallBinaryResponse{
		Success:    true,
		CoreType:   result.CoreType,
		Version:    result.Version,
		BinaryPath: result.BinaryPath,
	}, nil
}

// InstallProxy 安装代理二进制（caddy/nginx）。
func (a *LocalAgent) InstallProxy(ctx context.Context, proxyType string) (*agentpb.InstallProxyResponse, error) {
	if a.proxyMgr == nil {
		return &agentpb.InstallProxyResponse{Success: false, Error: "proxy manager not configured"}, nil
	}
	if err := a.proxyMgr.InstallProxy(ctx, proxyType); err != nil {
		return &agentpb.InstallProxyResponse{Success: false, Error: err.Error()}, nil
	}
	// Get updated status
	info := a.proxyMgr.DetectProxy(proxyType)
	log.Printf("[LocalAgent] proxy installed: %s (version=%s)", proxyType, info.Version)
	return &agentpb.InstallProxyResponse{
		Success:    true,
		ProxyType:  proxyType,
		Version:    info.Version,
		BinaryPath: info.Path,
		Installed:  info.Installed,
	}, nil
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

// GetProxyStatuses 获取所有代理的安装和运行状态。
func (a *LocalAgent) GetProxyStatuses() *agentpb.GetProxyStatusResponse {
	resp := &agentpb.GetProxyStatusResponse{}
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
	}
	return resp
}

// resolveServiceID 根据 coreType 查找对应的 service ID。
// process.Manager 通过 serviceID 管理进程，需要做一次 DB 查询。
func (a *LocalAgent) resolveServiceID(coreType string) (int64, error) {
	if a.svcRepo == nil {
		return 0, fmt.Errorf("service repository not configured")
	}
	services, _, err := a.svcRepo.List(context.Background(), 1, coreType, 1, 0) // server_id=1 (local)
	if err != nil {
		return 0, fmt.Errorf("query service for %s: %w", coreType, err)
	}
	if len(services) == 0 {
		return 0, fmt.Errorf("no service found for core type %s", coreType)
	}
	return services[0].ID, nil
}

// ============================================================
// 内置 xray gRPC 客户端 — 与 xraycontrol.Handler 使用相同的连接方式
// ============================================================

// alterUsersViaXrayGRPC 通过 xray gRPC API 执行用户增删改。
// 这与 xraycontrol.Handler.AddUser/RemoveUser 使用完全相同的：
//   - grpc.DialContext 连接到 127.0.0.1:10085
//   - proxymanCommand.NewHandlerServiceClient
//   - client.AlterInbound with AddUserOperation/RemoveUserOperation
//
// 不是新系统——是对 xray-core gRPC API 的标准调用。
func (a *LocalAgent) alterUsersViaXrayGRPC(ctx context.Context, req *agentpb.AlterUsersRequest) (*agentpb.AlterUsersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, a.statsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return &agentpb.AlterUsersResponse{Success: false, Error: fmt.Sprintf("connect to xray: %v", err)}, err
	}
	defer conn.Close()

	client := proxymanCommand.NewHandlerServiceClient(conn)
	affected := int32(0)

	for _, user := range req.Users {
		switch req.Action {
		case "add":
			if err := a.addUserViaGRPC(ctx, client, req.InboundTag, user); err != nil {
				return &agentpb.AlterUsersResponse{Success: false, Error: err.Error()}, err
			}
			affected++
		case "remove":
			if err := a.removeUserViaGRPC(ctx, client, req.InboundTag, user.Email); err != nil {
				return &agentpb.AlterUsersResponse{Success: false, Error: err.Error()}, err
			}
			affected++
		case "update":
			// update = remove + add
			_ = a.removeUserViaGRPC(ctx, client, req.InboundTag, user.Email)
			if err := a.addUserViaGRPC(ctx, client, req.InboundTag, user); err != nil {
				return &agentpb.AlterUsersResponse{Success: false, Error: err.Error()}, err
			}
			affected++
		}
	}

	return &agentpb.AlterUsersResponse{Success: true, AffectedCount: affected}, nil
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
// 协议检测逻辑与 xraycontrol.Handler.AddUser 完全一致。
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
			cipherType = shadowsocks.CipherType_NONE
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
	// Resolve binary path
	binaryPath, err := a.resolveBinaryPath(coreType)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}

	// Parse output as key:value pairs
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
