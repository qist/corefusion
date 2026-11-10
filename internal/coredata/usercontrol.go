package coredata

import (
	"context"
	"fmt"
	"strings"

	proxymanCommand "github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	hyAccount "github.com/xtls/xray-core/proxy/hysteria/account"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	ss2022 "github.com/xtls/xray-core/proxy/shadowsocks_2022"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
)

// NormalizeProtocol 归一协议名（去空格、转小写）。
func NormalizeProtocol(protocol string) string {
	return strings.ToLower(strings.TrimSpace(protocol))
}

// ProtocolHasUsers 报告该协议是否按账号鉴权。
//
// socks/http 这类入站的 settings 里没有 clients/users，往其中写用户既无效、
// 又会让配置在下次重启时校验失败（整个核心起不来），因此必须显式跳过。
func ProtocolHasUsers(protocol string) bool {
	switch NormalizeProtocol(protocol) {
	case "socks", "http", "dokodemo-door", "wireguard", "tunnel":
		return false
	}
	return true
}

// BuildXrayUserAccount 按协议构造 xray 用户账户（AlterInbound AddUser 用）。
//
// 必须由调用方显式给出协议：入站 tag 不保证含协议名（线上实际是
// inbound-59026 这种"端口 tag"），仅凭 tag 推断会把 vless 当成 trojan，
// 构造出的账号类型不匹配 → 热加失败 → 触发核心重启。
func BuildXrayUserAccount(protocol, id, password, flow, method string) *serial.TypedMessage {
	switch NormalizeProtocol(protocol) {
	case "vless":
		return serial.ToTypedMessage(&vless.Account{Id: id, Flow: flow})
	case "vmess":
		return serial.ToTypedMessage(&vmess.Account{Id: id})
	case "trojan":
		return serial.ToTypedMessage(&trojan.Account{Password: password})
	case "hysteria":
		// xray 的 hysteria 入站用 auth 字段鉴权
		return serial.ToTypedMessage(&hyAccount.Account{Auth: password})
	case "shadowsocks", "shadowsocks2022", "ss", "ss2022":
		if strings.HasPrefix(method, "2022-blake3") {
			return serial.ToTypedMessage(&ss2022.Account{Key: password})
		}
		cipherType := shadowsocks.CipherType_AES_256_GCM
		switch method {
		case "aes-128-gcm":
			cipherType = shadowsocks.CipherType_AES_128_GCM
		case "chacha20-poly1305":
			cipherType = shadowsocks.CipherType_CHACHA20_POLY1305
		case "xchacha20-ietf-poly1305":
			cipherType = shadowsocks.CipherType_XCHACHA20_POLY1305
		case "none":
			cipherType = shadowsocks.CipherType_UNKNOWN
		}
		return serial.ToTypedMessage(&shadowsocks.Account{Password: password, CipherType: cipherType})
	}
	return nil
}

// InferProtocolFromTag 从 tag 里猜协议，仅用于"调用方没给协议"的兜底。
// 新入站 tag 往往只有端口（inbound-59026），此时返回空串，调用方应显式传协议。
func InferProtocolFromTag(tag string) string {
	t := strings.ToLower(tag)
	switch {
	case strings.Contains(t, "vless"):
		return "vless"
	case strings.Contains(t, "vmess"):
		return "vmess"
	case strings.Contains(t, "trojan"):
		return "trojan"
	case strings.Contains(t, "hysteria"):
		return "hysteria"
	case strings.Contains(t, "shadowsocks-2022"), strings.Contains(t, "ss2022"), strings.Contains(t, "shadowsocks"), strings.Contains(t, "ss-"):
		return "shadowsocks"
	case strings.Contains(t, "socks"):
		return "socks"
	case strings.Contains(t, "http"):
		return "http"
	}
	return ""
}

// AddUser 在指定节点的运行中 xray 上热添加用户：
//   - 本地节点：面板进程内直连 xray gRPC（不再走面板自身的 HTTP 回环，
//     否则面板开启 TLS 时回环请求会被 HTTPS 监听器拒绝）；
//   - 远程节点：经 Agent gRPC 转发，由 Agent 调本机 xray gRPC。
func (p *ControlProvider) AddUser(ctx context.Context, serverID int64, tag, protocolName, email, id, password, flow, method string) error {
	protocolName = NormalizeProtocol(protocolName)
	if protocolName == "" {
		protocolName = InferProtocolFromTag(tag)
	}
	if protocolName == "" {
		return fmt.Errorf("protocol is required for tag %q (tag carries no protocol name)", tag)
	}
	if !ProtocolHasUsers(protocolName) {
		return fmt.Errorf("protocol %q does not support users", protocolName)
	}

	if p.isLocal != nil && p.isLocal(serverID) {
		account := BuildXrayUserAccount(protocolName, id, password, flow, method)
		if account == nil {
			return fmt.Errorf("unsupported protocol %q for xray user account", protocolName)
		}
		conn, err := p.LocalConn(ctx, serverID)
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = proxymanCommand.NewHandlerServiceClient(conn).AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
			Tag: tag,
			Operation: serial.ToTypedMessage(&proxymanCommand.AddUserOperation{
				User: &protocol.User{Email: email, Account: account},
			}),
		})
		if err != nil {
			return fmt.Errorf("add user %s to %s: %w", email, tag, err)
		}
		return nil
	}

	_, err := p.RemoteCall(ctx, serverID, "proxyman", "addUser", map[string]interface{}{
		"tag":      tag,
		"protocol": protocolName,
		"email":    email,
		"id":       id,
		"password": password,
		"flow":     flow,
		"method":   method,
	})
	if err != nil {
		return fmt.Errorf("add user %s to %s (server %d): %w", email, tag, serverID, err)
	}
	return nil
}

// RemoveUser 在指定节点的运行中 xray 上热移除用户（本地直连 / 远程经 Agent）。
func (p *ControlProvider) RemoveUser(ctx context.Context, serverID int64, tag, email string) error {
	if p.isLocal != nil && p.isLocal(serverID) {
		conn, err := p.LocalConn(ctx, serverID)
		if err != nil {
			return err
		}
		defer conn.Close()
		_, err = proxymanCommand.NewHandlerServiceClient(conn).AlterInbound(ctx, &proxymanCommand.AlterInboundRequest{
			Tag: tag,
			Operation: serial.ToTypedMessage(&proxymanCommand.RemoveUserOperation{
				Email: email,
			}),
		})
		if err != nil {
			return fmt.Errorf("remove user %s from %s: %w", email, tag, err)
		}
		return nil
	}

	_, err := p.RemoteCall(ctx, serverID, "proxyman", "removeUser", map[string]interface{}{
		"tag":   tag,
		"email": email,
	})
	if err != nil {
		return fmt.Errorf("remove user %s from %s (server %d): %w", email, tag, serverID, err)
	}
	return nil
}
