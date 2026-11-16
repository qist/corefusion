package router

import (
	"context"
	"encoding/json"
	"fmt"

	"corefusion/internal/coredata"
	agentrt "corefusion/internal/runtime/agent"
	agentpb "corefusion/internal/runtime/agent/proto"
	coreconfigSvc "corefusion/internal/service/coreconfig"
	endpointconfigSvc "corefusion/internal/service/endpointconfig"
	inboundconfigSvc "corefusion/internal/service/inboundconfig"
	outboundconfigSvc "corefusion/internal/service/outboundconfig"
)

// 本文件负责「控制面不可达期间的热更改动，Agent 重连后自动补下发」：
//
//	xrayPendingRemote（标记层） ──失败时──> NodeServerManager.MarkXrayPending
//	                                               │
//	                              Agent 重连 ──Take──┘
//	                                               ↓
//	                            xrayPendingReplayer（按数据库期望状态下发）
//
// 为什么需要：Agent 断连时面板只能写库（连配置文件都下发不了），运行中的 xray
// 仍是旧配置；而周期同步对已运行的核心只写文件、不重启（reload_strategy=NONE），
// 于是「磁盘是新配置、进程跑的是旧配置」会一直持续到下次重启核心。

// xrayRemoteAPI 是「远端 xray API 转发」的最小接口（*nodeserver.Service 实现）。
type xrayRemoteAPI interface {
	XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error)
}

// xrayPendingRemote 包一层远端 xray API 转发：只在「控制面不可达」时记录待补下发。
//
// 只记这类失败，是因为 xray 的业务错误（handler not found、参数不合法）说明指令
// 已经送达，重放也不会成功；若把它们也记下来，重连后会做无谓的删除/重建。
type xrayPendingRemote struct {
	inner xrayRemoteAPI
	mgr   *agentrt.NodeServerManager
}

func (w *xrayPendingRemote) XrayAPIOnServer(ctx context.Context, serverID int64, req *agentpb.XrayAPIRequest) (*agentpb.XrayAPIResponse, error) {
	resp, err := w.inner.XrayAPIOnServer(ctx, serverID, req)
	if err != nil && w.mgr != nil && agentrt.IsXrayControlUnavailable(err) {
		if op, ok := xrayPendingOpOf(req.GetService(), req.GetMethod(), req.GetPayload()); ok {
			w.mgr.MarkXrayPending(serverID, op)
		}
	}
	return resp, err
}

// xrayPendingOpOf 把一次 xray 控制调用映射成「待补下发的目标」。
//
// 入站/出站的增删改（含用户增删）都归到「把该 tag 同步到数据库期望状态」，
// 因为补下发时不关心中间改了几次，只看最终该 tag 应该长什么样。
func xrayPendingOpOf(service, method string, payload []byte) (agentrt.XrayPendingOp, bool) {
	switch service {
	case "router":
		// 路由/负载均衡都是整段重放，没有按 tag 增删的语义。
		return agentrt.XrayPendingOp{Kind: agentrt.XrayPendingRouting}, true
	case "proxyman":
		var p struct {
			Tag    string `json:"tag"`
			Config struct {
				Tag string `json:"tag"`
			} `json:"config"`
		}
		_ = json.Unmarshal(payload, &p)
		tag := p.Tag
		if tag == "" {
			tag = p.Config.Tag
		}
		if tag == "" {
			return agentrt.XrayPendingOp{}, false
		}
		switch method {
		case "addInbound", "removeInbound", "alterInbound", "addUser", "removeUser":
			return agentrt.XrayPendingOp{Kind: agentrt.XrayPendingInbound, Tag: tag}, true
		case "addOutbound", "removeOutbound", "alterOutbound":
			return agentrt.XrayPendingOp{Kind: agentrt.XrayPendingOutbound, Tag: tag}, true
		}
	}
	return agentrt.XrayPendingOp{}, false
}

// xrayRuntimeControl 是补下发用到的控制面操作（*coredata.ControlProvider 实现）。
type xrayRuntimeControl interface {
	AddInbound(ctx context.Context, serverID int64, configJSON []byte) error
	RemoveInbound(ctx context.Context, serverID int64, tag string) error
	AddOutbound(ctx context.Context, serverID int64, configJSON []byte) error
	RemoveOutbound(ctx context.Context, serverID int64, tag string) error
	ApplyRouting(ctx context.Context, serverID int64, routingJSON []byte) error
}

// xrayDesiredRuntime 是数据库里「该节点运行态应该是什么」的索引。
type xrayDesiredRuntime struct {
	Inbounds  map[string]json.RawMessage
	Outbounds map[string]json.RawMessage
	Routing   json.RawMessage
}

// xrayPendingReplayer 读数据库生成完整运行时配置，再按待补队列下发到节点。
type xrayPendingReplayer struct {
	coreConfigSvc *coreconfigSvc.Service
	inboundSvc    *inboundconfigSvc.Service
	outboundSvc   *outboundconfigSvc.Service
	endpointSvc   *endpointconfigSvc.Service
	chainLinkSvc  coreconfigSvc.ChainLinkProvider
	control       *coredata.ControlProvider
}

func (r *xrayPendingReplayer) ReplayXrayPending(ctx context.Context, serverID int64, ops []agentrt.XrayPendingOp) error {
	if len(ops) == 0 {
		return nil
	}
	if r == nil || r.control == nil || r.coreConfigSvc == nil {
		return fmt.Errorf("xray pending replayer not configured")
	}
	desired, err := r.desired(ctx, serverID)
	if err != nil {
		return err
	}
	return applyXrayPending(ctx, serverID, ops, desired, r.control)
}

// desired 用「与下发节点完全相同」的生成链路（PrepareFullConfig）产出最终运行时配置，
// 再按 tag 建立索引：补下发时直接用这份以数据库为准的对象，避免拼装第二套配置逻辑。
func (r *xrayPendingReplayer) desired(ctx context.Context, serverID int64) (*xrayDesiredRuntime, error) {
	full, err := r.coreConfigSvc.GetFull(ctx, serverID, "xray")
	if err != nil {
		return nil, fmt.Errorf("load xray sections for server %d: %w", serverID, err)
	}
	raw, err := coreconfigSvc.PrepareFullConfig(coreconfigSvc.PrepareFullConfigOptions{
		Ctx:              ctx,
		ServerID:         serverID,
		CoreType:         "xray",
		FullConfig:       full,
		InboundProvider:  r.inboundSvc,
		OutboundProvider: r.outboundSvc,
		EndpointProvider: r.endpointSvc,
		// 链式出站/规则由生成器产出，不传的话期望状态里会缺这些 tag，
		// 补下发可能把仍在使用的链式出站当成「已删除」移除掉。
		ChainLinkProvider: r.chainLinkSvc,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare xray config for server %d: %w", serverID, err)
	}

	return buildDesiredRuntime(raw, serverID)
}

// buildDesiredRuntime 从生成的完整配置里按 tag 建立索引。
func buildDesiredRuntime(raw []byte, serverID int64) (*xrayDesiredRuntime, error) {
	var parsed struct {
		Inbounds  []json.RawMessage `json:"inbounds"`
		Outbounds []json.RawMessage `json:"outbounds"`
		Routing   json.RawMessage   `json:"routing"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse generated xray config for server %d: %w", serverID, err)
	}

	desired := &xrayDesiredRuntime{
		Inbounds:  make(map[string]json.RawMessage, len(parsed.Inbounds)),
		Outbounds: make(map[string]json.RawMessage, len(parsed.Outbounds)),
		Routing:   parsed.Routing,
	}
	for _, item := range parsed.Inbounds {
		if tag := jsonTagOf(item); tag != "" {
			desired.Inbounds[tag] = item
		}
	}
	for _, item := range parsed.Outbounds {
		if tag := jsonTagOf(item); tag != "" {
			desired.Outbounds[tag] = item
		}
	}
	return desired, nil
}

func jsonTagOf(raw json.RawMessage) string {
	var obj struct {
		Tag string `json:"tag"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return obj.Tag
}

// applyXrayPending 按待补队列把节点运行态同步到期望状态：
//   - 期望里还有该 tag → 覆盖下发（同 tag 先删后加，幂等）
//   - 期望里没有该 tag → 从运行态移除（离线期间被删掉的入站/出站）
func applyXrayPending(ctx context.Context, serverID int64, ops []agentrt.XrayPendingOp, desired *xrayDesiredRuntime, ctl xrayRuntimeControl) error {
	if desired == nil {
		return fmt.Errorf("desired runtime is empty")
	}
	for _, op := range ops {
		switch op.Kind {
		case agentrt.XrayPendingInbound:
			if cfg, ok := desired.Inbounds[op.Tag]; ok {
				if err := ctl.AddInbound(ctx, serverID, cfg); err != nil {
					return fmt.Errorf("apply inbound %s: %w", op.Tag, err)
				}
				continue
			}
			if err := ctl.RemoveInbound(ctx, serverID, op.Tag); err != nil {
				return fmt.Errorf("remove inbound %s: %w", op.Tag, err)
			}
		case agentrt.XrayPendingOutbound:
			if cfg, ok := desired.Outbounds[op.Tag]; ok {
				if err := ctl.AddOutbound(ctx, serverID, cfg); err != nil {
					return fmt.Errorf("apply outbound %s: %w", op.Tag, err)
				}
				continue
			}
			if err := ctl.RemoveOutbound(ctx, serverID, op.Tag); err != nil {
				return fmt.Errorf("remove outbound %s: %w", op.Tag, err)
			}
		case agentrt.XrayPendingRouting:
			if len(desired.Routing) == 0 {
				continue
			}
			if err := ctl.ApplyRouting(ctx, serverID, desired.Routing); err != nil {
				return fmt.Errorf("apply routing: %w", err)
			}
		}
	}
	return nil
}
