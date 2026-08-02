package coredata

import (
	"context"
	"encoding/json"
	"fmt"

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
type Provider struct {
	serverID    int64
	local       *localBackend
	remote      *remoteBackend
	onlineUsers OnlineUsersProvider
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

// NewProvider 构造 Provider。本地/远程后端按需惰性创建。
func NewProvider(serverID int64, opts Options) *Provider {
	p := &Provider{serverID: serverID, onlineUsers: opts.OnlineUsers}

	if opts.IsLocal == nil || opts.IsLocal(serverID) {
		var addr string
		if opts.AddrResolver != nil {
			if a, err := opts.AddrResolver(context.Background(), serverID); err == nil {
				addr = a
			}
		}
		p.local = &localBackend{client: NewXrayClient(addr)}
	}
	if opts.Remote != nil && (opts.IsLocal == nil || !opts.IsLocal(serverID)) {
		p.remote = &remoteBackend{caller: opts.Remote, serverID: serverID}
	}
	return p
}

func (p *Provider) backend() (CoreDataProvider, error) {
	if p.local != nil {
		return p.local, nil
	}
	if p.remote != nil {
		return p.remote, nil
	}
	return nil, fmt.Errorf("no backend available for server %d", p.serverID)
}

// GetUserTraffic 返回用户累计流量。
func (p *Provider) GetUserTraffic(ctx context.Context, serverID int64) ([]UserTraffic, error) {
	b, err := p.backend()
	if err != nil {
		return nil, err
	}
	return b.GetUserTraffic(ctx, serverID)
}

// GetInboundTraffic 返回入站累计流量。
func (p *Provider) GetInboundTraffic(ctx context.Context, serverID int64) ([]InboundTraffic, error) {
	b, err := p.backend()
	if err != nil {
		return nil, err
	}
	return b.GetInboundTraffic(ctx, serverID)
}

// GetSysStats 返回系统统计。
func (p *Provider) GetSysStats(ctx context.Context, serverID int64) (*SysStats, error) {
	b, err := p.backend()
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
	b, err := p.backend()
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
	_ = resp
	return nil
}
