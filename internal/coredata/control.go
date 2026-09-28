package coredata

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// CoreControl 是写操作（控制面）的公共层抽象，与 CoreDataProvider（读操作）
// 对称分层。它不承载业务编排（协议识别、config Build 等保留在调用方 handler），
// 只负责"把一次控制请求送达正确的核心"：
//   - 本地节点：直连 xray gRPC（每次新建连接，因 xray 可能独立重启）
//   - 远程节点：经 Agent RPC 转发
//
// 上层可按 serverID 路由，对调用方透明，与 CoreDataProvider 风格一致。
type CoreControl interface {
	// LocalConn 返回本地节点 serverID 的 xray gRPC 连接（每次新建，不缓存）。
	LocalConn(ctx context.Context, serverID int64) (*grpc.ClientConn, error)
	// RemoteCall 经 Agent 转发一次写操作，返回远程 JSON 响应。
	RemoteCall(ctx context.Context, serverID int64, service, method string, payload any) (json.RawMessage, error)
}

// ControlProvider 组合本地/远程两种后端，按 serverID 路由。
type ControlProvider struct {
	isLocal func(serverID int64) bool
	addr    AddrResolver
	remote  RemoteCaller
}

// ControlOptions 构造 ControlProvider 所需依赖。
type ControlOptions struct {
	// IsLocal 返回该 server 是否为本地节点（直连 gRPC）。
	IsLocal func(serverID int64) bool
	// AddrResolver 解析本地 xray 地址。
	AddrResolver AddrResolver
	// Remote 远程 Agent RPC 调用方（仅远程节点需要）。
	Remote RemoteCaller
}

// NewControlProvider 构造 ControlProvider。
func NewControlProvider(opts ControlOptions) *ControlProvider {
	return &ControlProvider{
		isLocal: opts.IsLocal,
		addr:    opts.AddrResolver,
		remote:  opts.Remote,
	}
}

// LocalConn 返回本地节点的 xray gRPC 连接。
// xray 可能独立于面板进程重启，故每次新建连接（不复用缓存），与历史行为一致。
func (p *ControlProvider) LocalConn(ctx context.Context, serverID int64) (*grpc.ClientConn, error) {
	if p.isLocal != nil && !p.isLocal(serverID) {
		return nil, fmt.Errorf("server %d is not local; use RemoteCall", serverID)
	}
	var addr string
	if p.addr != nil {
		var err error
		addr, err = p.addr(ctx, serverID)
		if err != nil {
			return nil, fmt.Errorf("resolve stats addr: %w", err)
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("xray API not available for server %d", serverID)
	}
	client := NewXrayClient(addr)
	return client.Conn(ctx)
}

// RemoteCall 经 Agent 转发一次写操作，返回远程 JSON 响应。
func (p *ControlProvider) RemoteCall(ctx context.Context, serverID int64, service, method string, payload any) (json.RawMessage, error) {
	if p.remote == nil {
		return nil, fmt.Errorf("node operator not configured for remote server %d", serverID)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := p.remote.XrayAPIOnServer(ctx, serverID, &agentpb.XrayAPIRequest{
		Service: service,
		Method:  method,
		Payload: payloadBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("remote call: %w", err)
	}
	if !resp.GetSuccess() {
		return nil, fmt.Errorf("remote error: %s", resp.GetError())
	}
	return json.RawMessage(resp.GetResponse()), nil
}
