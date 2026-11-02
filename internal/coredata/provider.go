package coredata

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	statsService "github.com/xtls/xray-core/app/stats/command"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// CoreDataProvider 是前端/面板的唯一依赖。实现按 serverID 路由：
// 本地节点直连 xray gRPC，远程节点经 Agent RPC，对调用方透明。
type CoreDataProvider interface {
	GetUserTraffic(ctx context.Context, serverID int64) ([]UserTraffic, error)
	GetInboundTraffic(ctx context.Context, serverID int64) ([]InboundTraffic, error)
	GetSysStats(ctx context.Context, serverID int64) (*SysStats, error)
	GetOnlineUsers(ctx context.Context, serverID int64) ([]OnlineUser, error)
	ResetUserTraffic(ctx context.Context, serverID int64, email string) error
}

// AddrResolver 解析某节点的 xray stats 地址（本地直连用）。
type AddrResolver func(ctx context.Context, serverID int64) (string, error)

// RemoteCaller 向远程节点的 Agent 发起 xray API 调用（与 nodeserver.Service 对齐）。
type RemoteCaller interface {
	XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error)
}

// OnlineUsersProvider 在线用户聚合（节点级，非 xray API 直读），
// 由 nodeserver.Service.GetOnlineUsers 等实现注入，避免 provider 反向依赖。
type OnlineUsersProvider func(ctx context.Context, serverID int64) ([]OnlineUser, error)

// Provider 组合本地/远程两种后端。
//
// ⚠ 路由必须按「每次调用的 serverID」决定，不能用构造时的 serverID 预建单一后端：
// 此前 Handler 用面板本机 localServerID 构造 → backend() 恒返回 local 后端，
// 查询任何远程节点都在拨面板本机的 127.0.0.1:10085（远程后端根本没建出来），
// 面板本机核心没跑时远程节点也被误报 dial 失败。
type Provider struct {
	mu           sync.Mutex
	isLocalFn    func(serverID int64) bool
	addrResolver AddrResolver
	remoteCaller RemoteCaller
	onlineUsers  OnlineUsersProvider
	locals       map[int64]*localBackend
	remotes      map[int64]*remoteBackend
}

// Options 构造 Provider 所需依赖。
type Options struct {
	// IsLocal 返回该 server 是否为本地节点（直连 gRPC）。
	IsLocal func(serverID int64) bool
	// AddrResolver 解析本地 stats 地址。
	AddrResolver AddrResolver
	// Remote 远程 Agent RPC 调用方（仅远程节点需要）。
	Remote RemoteCaller
	// OnlineUsers 在线用户聚合实现（可选，缺省返回空）。
	OnlineUsers OnlineUsersProvider
}

// NewProvider 构造 Provider。本地/远程后端按每次调用的 serverID 惰性创建并缓存。
// serverID 参数保留仅为兼容旧签名，路由不再依赖它。
func NewProvider(serverID int64, opts Options) *Provider {
	_ = serverID
	return &Provider{
		isLocalFn:    opts.IsLocal,
		addrResolver: opts.AddrResolver,
		remoteCaller: opts.Remote,
		onlineUsers:  opts.OnlineUsers,
		locals:       make(map[int64]*localBackend),
		remotes:      make(map[int64]*remoteBackend),
	}
}

func (p *Provider) backend(serverID int64) (CoreDataProvider, error) {
	if p.isLocalFn == nil || p.isLocalFn(serverID) {
		p.mu.Lock()
		defer p.mu.Unlock()
		lb, ok := p.locals[serverID]
		if !ok {
			var addr string
			if p.addrResolver != nil {
				if a, err := p.addrResolver(context.Background(), serverID); err == nil {
					addr = a
				}
			}
			lb = &localBackend{client: NewXrayClient(addr)}
			p.locals[serverID] = lb
		}
		return lb, nil
	}
	if p.remoteCaller == nil {
		return nil, fmt.Errorf("no remote caller available for server %d", serverID)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rb, ok := p.remotes[serverID]
	if !ok {
		rb = &remoteBackend{caller: p.remoteCaller, serverID: serverID}
		p.remotes[serverID] = rb
	}
	return rb, nil
}

// GetUserTraffic 返回用户累计流量。
func (p *Provider) GetUserTraffic(ctx context.Context, serverID int64) ([]UserTraffic, error) {
	b, err := p.backend(serverID)
	if err != nil {
		return nil, err
	}
	return b.GetUserTraffic(ctx, serverID)
}

// GetInboundTraffic 返回入站累计流量。
func (p *Provider) GetInboundTraffic(ctx context.Context, serverID int64) ([]InboundTraffic, error) {
	b, err := p.backend(serverID)
	if err != nil {
		return nil, err
	}
	return b.GetInboundTraffic(ctx, serverID)
}

// GetSysStats 返回系统统计。
func (p *Provider) GetSysStats(ctx context.Context, serverID int64) (*SysStats, error) {
	b, err := p.backend(serverID)
	if err != nil {
		return nil, err
	}
	return b.GetSysStats(ctx, serverID)
}

// GetOnlineUsers 返回在线用户（委托节点级聚合）。
func (p *Provider) GetOnlineUsers(ctx context.Context, serverID int64) ([]OnlineUser, error) {
	if p.onlineUsers == nil {
		return nil, nil
	}
	return p.onlineUsers(ctx, serverID)
}

// ResetUserTraffic 复位用户流量计数器。
func (p *Provider) ResetUserTraffic(ctx context.Context, serverID int64, email string) error {
	b, err := p.backend(serverID)
	if err != nil {
		return err
	}
	return b.ResetUserTraffic(ctx, serverID, email)
}

// ---- 本地后端：直连 xray gRPC ----

type localBackend struct {
	client *XrayClient
}

func (l *localBackend) GetUserTraffic(ctx context.Context, _ int64) ([]UserTraffic, error) {
	resp, err := l.client.RawStats(ctx, false)
	if err != nil {
		return nil, err
	}
	sr, err := ParseStatsResponse(resp)
	if err != nil {
		return nil, err
	}
	return sr.Users, nil
}

func (l *localBackend) GetInboundTraffic(ctx context.Context, _ int64) ([]InboundTraffic, error) {
	resp, err := l.client.RawStats(ctx, false)
	if err != nil {
		return nil, err
	}
	sr, err := ParseStatsResponse(resp)
	if err != nil {
		return nil, err
	}
	return sr.Inbounds, nil
}

func (l *localBackend) GetSysStats(ctx context.Context, _ int64) (*SysStats, error) {
	conn, err := l.client.dial(ctx)
	if err != nil {
		return nil, err
	}
	client := statsService.NewStatsServiceClient(conn)
	r, err := client.GetSysStats(ctx, &statsService.SysStatsRequest{})
	if err != nil {
		return nil, fmt.Errorf("get xray sys stats: %w", err)
	}
	return &SysStats{NumGoroutine: r.GetNumGoroutine(), Alloc: int64(r.GetAlloc())}, nil
}

func (l *localBackend) GetOnlineUsers(ctx context.Context, _ int64) ([]OnlineUser, error) {
	// 本地在线用户仍交由节点级聚合（见 Options.OnlineUsers）。
	return nil, fmt.Errorf("local online users must be supplied via Options.OnlineUsers")
}

func (l *localBackend) ResetUserTraffic(ctx context.Context, _ int64, email string) error {
	return l.client.ResetUserStats(ctx, email)
}

// ---- 远程后端：经 Agent RPC ----

type remoteBackend struct {
	caller   RemoteCaller
	serverID int64
}

func (r *remoteBackend) rawStats(ctx context.Context, reset bool) (*StatsResponse, error) {
	arg, _ := json.Marshal(map[string]any{"reset": reset})
	resp, err := r.caller.XrayAPIOnServer(ctx, r.serverID, &agentpb.XrayAPIRequest{
		Service: "stats",
		Method:  "QueryStats",
		Payload: arg,
	})
	if err != nil {
		return nil, fmt.Errorf("remote QueryStats: %w", err)
	}
	if !resp.GetSuccess() {
		return nil, fmt.Errorf("remote QueryStats error: %s", resp.GetError())
	}
	var qr statsService.QueryStatsResponse
	if err := json.Unmarshal([]byte(resp.GetResponse()), &qr); err != nil {
		return nil, fmt.Errorf("unmarshal remote stats: %w", err)
	}
	return ParseStatsResponse(&qr)
}

func (r *remoteBackend) GetUserTraffic(ctx context.Context, _ int64) ([]UserTraffic, error) {
	sr, err := r.rawStats(ctx, false)
	if err != nil {
		return nil, err
	}
	return sr.Users, nil
}

func (r *remoteBackend) GetInboundTraffic(ctx context.Context, _ int64) ([]InboundTraffic, error) {
	sr, err := r.rawStats(ctx, false)
	if err != nil {
		return nil, err
	}
	return sr.Inbounds, nil
}

func (r *remoteBackend) GetSysStats(ctx context.Context, _ int64) (*SysStats, error) {
	resp, err := r.caller.XrayAPIOnServer(ctx, r.serverID, &agentpb.XrayAPIRequest{
		Service: "stats",
		Method:  "GetSysStats",
		Payload: []byte("{}"),
	})
	if err != nil {
		return nil, fmt.Errorf("remote GetSysStats: %w", err)
	}
	if !resp.GetSuccess() {
		return nil, fmt.Errorf("remote GetSysStats error: %s", resp.GetError())
	}
	var sr statsService.SysStatsResponse
	if err := json.Unmarshal([]byte(resp.GetResponse()), &sr); err != nil {
		return nil, fmt.Errorf("unmarshal remote sys stats: %w", err)
	}
	return &SysStats{NumGoroutine: sr.GetNumGoroutine(), Alloc: int64(sr.GetAlloc())}, nil
}

func (r *remoteBackend) GetOnlineUsers(ctx context.Context, _ int64) ([]OnlineUser, error) {
	return nil, fmt.Errorf("remote online users must be supplied via Options.OnlineUsers")
}

func (r *remoteBackend) ResetUserTraffic(ctx context.Context, _ int64, email string) error {
	arg, _ := json.Marshal(map[string]any{"reset": true, "pattern": "user>>>" + email + ">>>traffic>>>"})
	resp, err := r.caller.XrayAPIOnServer(ctx, r.serverID, &agentpb.XrayAPIRequest{
		Service: "stats",
		Method:  "QueryStats",
		Payload: arg,
	})
	if err != nil {
		return fmt.Errorf("remote reset: %w", err)
	}
	if !resp.GetSuccess() {
		return fmt.Errorf("remote reset error: %s", resp.GetError())
	}
	return nil
}
