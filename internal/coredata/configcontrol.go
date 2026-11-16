package coredata

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

// BuildInboundConfig 把入站 JSON 解析成 xray 的入站配置，并返回其 tag。
// xray 的 AddInbound 对重复 tag 会报 "existing tag found"，调用方需先按 tag 移除。
func BuildInboundConfig(configJSON []byte) (*xraycore.InboundHandlerConfig, string, error) {
	cfg := new(conf.InboundDetourConfig)
	if err := json.Unmarshal(configJSON, cfg); err != nil {
		return nil, "", fmt.Errorf("parse inbound config: %w", err)
	}
	tag := cfg.Tag
	inbound, err := cfg.Build()
	if err != nil {
		return nil, tag, fmt.Errorf("build inbound config: %w", err)
	}
	return inbound, tag, nil
}

// AddInbound 把一条入站热下发到运行中的 xray（本地直连 gRPC / 远端经 Agent）。
//
// 幂等 + 竞态兜底：同 tag 已存在时先移除再添加（xray 的 AddHandler 拒绝重复 tag）。
// 面板保存时前端会先 DELETE 再 POST，而 Agent 的热重载（SyncData HOT_RELOAD）也可能
// 同时对同一个 tag 做「先删后加」——两条链路交叉时 add 会撞上 xray 的
// `existing tag found`（真机踩到：用户看到「控制接口不可用」）。所以每次尝试都先删，
// 撞上重复 tag 就再试一次。
func (p *ControlProvider) AddInbound(ctx context.Context, serverID int64, configJSON []byte) error {
	const attempts = 2
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = p.addInboundOnce(ctx, serverID, configJSON)
		if lastErr == nil {
			return nil
		}
		if !IsExistingTagErr(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

// addInboundOnce 执行一次「先删同 tag，再添加」。
func (p *ControlProvider) addInboundOnce(ctx context.Context, serverID int64, configJSON []byte) error {
	if p.isLocal != nil && p.isLocal(serverID) {
		inbound, tag, err := BuildInboundConfig(configJSON)
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
			_, _ = client.RemoveInbound(ctx, &proxymanCommand.RemoveInboundRequest{Tag: tag})
		}
		if _, err := client.AddInbound(ctx, &proxymanCommand.AddInboundRequest{Inbound: inbound}); err != nil {
			return fmt.Errorf("add inbound %s: %w", tag, err)
		}
		return nil
	}

	if _, tag, err := BuildInboundConfig(configJSON); err == nil && tag != "" {
		_, _ = p.RemoteCall(ctx, serverID, "proxyman", "removeInbound", map[string]interface{}{"tag": tag})
	}
	if _, err := p.RemoteCall(ctx, serverID, "proxyman", "addInbound", map[string]interface{}{
		"config": json.RawMessage(configJSON),
	}); err != nil {
		return fmt.Errorf("add inbound on server %d: %w", serverID, err)
	}
	return nil
}

// IsExistingTagErr 判断错误是否为「该 tag 已存在」——这种错误重试（先删后加）可以恢复。
func IsExistingTagErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "existing tag") || strings.Contains(msg, "existing handler")
}

// IsMissingHandlerErr 判断错误是否只是「该对象已经不存在」。
//
// xray 删除不存在的入站/出站时返回 common.ErrNoClue：
//   - 入站/出站：`not enough information for making a decision`
//   - 用户：`user ... not found` / `not found`
//
// 这类错误语义上等价于「已经删掉了」：面板保存/关闭入口时，前端会主动 DELETE，
// 而 Agent 的热重载可能已经先删过同一个 tag（两条链路并发），此时再删就会拿到它。
// 必须按幂等成功处理，否则用户会看到「已保存，但控制接口不可用」的假告警
// （真机踩到：关闭入口后端口确实已关闭、操作其实成功）。
func IsMissingHandlerErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"not enough information for making a decision",
		"handler not found",
		"not found",
		"does not exist",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

// RemoveInbound 从运行中的 xray 热移除一条入站。
// 不存在视为成功（面板与 Agent 热重载可能都在删同一个 tag）；其它错误照常返回。
func (p *ControlProvider) RemoveInbound(ctx context.Context, serverID int64, tag string) error {
	if tag == "" {
		return nil
	}
	if p.isLocal != nil && p.isLocal(serverID) {
		conn, err := p.LocalConn(ctx, serverID)
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = proxymanCommand.NewHandlerServiceClient(conn).RemoveInbound(ctx,
			&proxymanCommand.RemoveInboundRequest{Tag: tag})
		if err != nil && !IsMissingHandlerErr(err) {
			return fmt.Errorf("remove inbound %s: %w", tag, err)
		}
		return nil
	}

	if _, err := p.RemoteCall(ctx, serverID, "proxyman", "removeInbound", map[string]interface{}{"tag": tag}); err != nil && !IsMissingHandlerErr(err) {
		return fmt.Errorf("remove inbound %s on server %d: %w", tag, serverID, err)
	}
	return nil
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

// RemoveOutbound 从运行中的 xray 热移除一条出站。
// 不存在视为成功（面板与 Agent 热重载可能都在删同一个 tag）；其它错误照常返回。
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
		_, err = proxymanCommand.NewHandlerServiceClient(conn).RemoveOutbound(ctx,
			&proxymanCommand.RemoveOutboundRequest{Tag: tag})
		if err != nil && !IsMissingHandlerErr(err) {
			return fmt.Errorf("remove outbound %s: %w", tag, err)
		}
		return nil
	}

	if _, err := p.RemoteCall(ctx, serverID, "proxyman", "removeOutbound", map[string]interface{}{
		"tag": tag,
	}); err != nil && !IsMissingHandlerErr(err) {
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
