package coredata

import (
	"context"
	"encoding/json"
	"fmt"

	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	routercore "github.com/xtls/xray-core/app/router"
	routerCommand "github.com/xtls/xray-core/app/router/command"
	"github.com/xtls/xray-core/common/serial"
	xraycore "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
)

// BuildOutboundConfig 把出站 JSON 解析成 xray 的出站配置，并返回其 tag。
//
// xray 的 AddOutbound 对重复 tag 会报 "existing tag found"：调用方需要先按 tag
// 移除旧出站再添加（见 ControlProvider.AddOutbound）。
func BuildOutboundConfig(configJSON []byte) (*xraycore.OutboundHandlerConfig, string, error) {
	cfg := new(conf.OutboundDetourConfig)
	if err := json.Unmarshal(configJSON, cfg); err != nil {
		return nil, "", fmt.Errorf("parse outbound config: %w", err)
	}
	tag := cfg.Tag
	outbound, err := cfg.Build()
	if err != nil {
		return nil, tag, fmt.Errorf("build outbound config: %w", err)
	}
	return outbound, tag, nil
}

// BuildRoutingConfig 把路由 JSON 解析成 xray 的路由配置。
// 会自动补一条隐式 api 规则（缺失时 xray API 自身的出站就不可达）。
func BuildRoutingConfig(routingJSON []byte) (*routercore.Config, error) {
	routerConf := new(conf.RouterConfig)
	if err := json.Unmarshal(routingJSON, routerConf); err != nil {
		return nil, fmt.Errorf("parse routing config: %w", err)
	}
	EnsureImplicitAPIRule(routerConf)
	built, err := routerConf.Build()
	if err != nil {
		return nil, fmt.Errorf("build routing config: %w", err)
	}
	return built, nil
}

// EnsureImplicitAPIRule 保证路由规则里存在 "inboundTag:[api] → outboundTag:api"。
// 缺了它，xray 的 API 出站不可达（面板后续所有热下发都会失败）。
func EnsureImplicitAPIRule(routerConf *conf.RouterConfig) {
	if routerConf == nil {
		return
	}
	for _, rawRule := range routerConf.RuleList {
		var rule struct {
			InboundTag  []string `json:"inboundTag"`
			OutboundTag string   `json:"outboundTag"`
		}
		if err := json.Unmarshal(rawRule, &rule); err != nil {
			continue
		}
		if rule.OutboundTag != "api" {
			continue
		}
		for _, inboundTag := range rule.InboundTag {
			if inboundTag == "api" {
				return
			}
		}
	}

	apiRule := json.RawMessage(`{"type":"field","inboundTag":["api"],"outboundTag":"api"}`)
	routerConf.RuleList = append([]json.RawMessage{apiRule}, routerConf.RuleList...)
}

// AddOutbound 把一条出站热下发到运行中的 xray（本地直连 gRPC / 远端经 Agent）。
//
// 幂等语义：同 tag 已存在时先移除再添加（xray 的 AddHandler 拒绝重复 tag，
// 而 RemoveHandler 对不存在的 tag 是静默成功）。远端 Agent 版本较低时也兼容，
// 因为这里先调 removeOutbound 再调 addOutbound。
func (p *ControlProvider) AddOutbound(ctx context.Context, serverID int64, configJSON []byte) error {
	if p.isLocal != nil && p.isLocal(serverID) {
		outbound, tag, err := BuildOutboundConfig(configJSON)
		if err != nil {
			return err
		}
		conn, err := p.LocalConn(ctx, serverID)
		if err != nil {
			return err
		}
		defer conn.Close()

		client := proxymanCommand.NewHandlerServiceClient(conn)
		if tag != "" {
			// 先删同 tag（不存在时静默成功），保证“改出站”可重复下发。
			_, _ = client.RemoveOutbound(ctx, &proxymanCommand.RemoveOutboundRequest{Tag: tag})
		}
		if _, err := client.AddOutbound(ctx, &proxymanCommand.AddOutboundRequest{Outbound: outbound}); err != nil {
			return fmt.Errorf("add outbound %s: %w", tag, err)
		}
		return nil
	}

	// 远端：先按 tag 移除旧出站（不存在时 Agent 侧也是静默成功）。
	if _, tag, err := BuildOutboundConfig(configJSON); err == nil && tag != "" {
		_, _ = p.RemoteCall(ctx, serverID, "proxyman", "removeOutbound", map[string]interface{}{"tag": tag})
	}
	if _, err := p.RemoteCall(ctx, serverID, "proxyman", "addOutbound", map[string]interface{}{
		"config": json.RawMessage(configJSON),
	}); err != nil {
		return fmt.Errorf("add outbound on server %d: %w", serverID, err)
	}
	return nil
}

// RemoveOutbound 从运行中的 xray 热移除一条出站（不存在视为成功）。
func (p *ControlProvider) RemoveOutbound(ctx context.Context, serverID int64, tag string) error {
	if tag == "" {
		return nil
	}
	if p.isLocal != nil && p.isLocal(serverID) {
		conn, err := p.LocalConn(ctx, serverID)
		if err != nil {
			return err
		}
		defer conn.Close()
		if _, err := proxymanCommand.NewHandlerServiceClient(conn).RemoveOutbound(ctx,
			&proxymanCommand.RemoveOutboundRequest{Tag: tag}); err != nil {
			return fmt.Errorf("remove outbound %s: %w", tag, err)
		}
		return nil
	}

	if _, err := p.RemoteCall(ctx, serverID, "proxyman", "removeOutbound", map[string]interface{}{
		"tag": tag,
	}); err != nil {
		return fmt.Errorf("remove outbound %s on server %d: %w", tag, serverID, err)
	}
	return nil
}

// ApplyRouting 用给定路由配置**全量替换**运行中 xray 的路由规则
// （RouterService.AddRule + ShouldAppend=false：xray 会清空旧 rules 与 balancers 再装载）。
func (p *ControlProvider) ApplyRouting(ctx context.Context, serverID int64, routingJSON []byte) error {
	if p.isLocal != nil && p.isLocal(serverID) {
		config, err := BuildRoutingConfig(routingJSON)
		if err != nil {
			return err
		}
		conn, err := p.LocalConn(ctx, serverID)
		if err != nil {
			return err
		}
		defer conn.Close()

		if _, err := routerCommand.NewRoutingServiceClient(conn).AddRule(ctx, &routerCommand.AddRuleRequest{
			ShouldAppend: false,
			Config:       serial.ToTypedMessage(config),
		}); err != nil {
			return fmt.Errorf("apply routing: %w", err)
		}
		return nil
	}

	if _, err := p.RemoteCall(ctx, serverID, "router", "applyRouting", map[string]interface{}{
		"routing": json.RawMessage(routingJSON),
	}); err != nil {
		return fmt.Errorf("apply routing on server %d: %w", serverID, err)
	}
	return nil
}
