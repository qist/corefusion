package proxy

import (
	"context"
	"time"
)

// 代理能力标记，与 Agent 上报的 proto ProxyInfo.capabilities 取值一致。
const (
	// NginxCapStreamPPVar 表示 nginx 打了 `-DNGX_STREAM_PP_VAR` 补丁：
	// proxy_protocol 可以取变量，按 SNI 给不同后端发 v1 / v2 / 不发。
	NginxCapStreamPPVar = "stream_pp_var"
	// NginxCapStreamSSLPreread 表示编译了 stream_ssl_preread_module。
	NginxCapStreamSSLPreread = "stream_ssl_preread"
)

// nginxCapabilityTTL 是能力缓存时长。节点升级 nginx 后最多 1 分钟生效，
// 避免每次生成配置都去问一遍 Agent（远程节点是一次 gRPC 往返）。
const nginxCapabilityTTL = 60 * time.Second

// NginxCapabilityFunc 返回节点 nginx 二进制的可选能力（由上层注入）。
// 返回 error 表示探测失败（节点离线等），调用方按「不支持」回退旧行为。
type NginxCapabilityFunc func(ctx context.Context, serverID int64) ([]string, error)

type nginxCapabilityEntry struct {
	caps map[string]bool
	at   time.Time
}

// SetNginxCapabilityFunc 注入能力探测函数（节点代理状态查询）。
func (m *NginxManager) SetNginxCapabilityFunc(fn NginxCapabilityFunc) {
	m.capMu.Lock()
	m.capabilityFunc = fn
	m.capCache = nil
	m.capMu.Unlock()
}

// NginxCapabilities 返回节点 nginx 的能力集合（带缓存）。
//
// 探测失败或未注入探测函数时返回空集合：调用方据此回退旧行为
// （整组 proxy_protocol on + 跳过 sing-box 规则），不会生成无法启动的配置。
func (m *NginxManager) NginxCapabilities(ctx context.Context, serverID int64) map[string]bool {
	if m.capabilityFunc == nil {
		return nil
	}

	m.capMu.Lock()
	if entry, ok := m.capCache[serverID]; ok && time.Since(entry.at) < nginxCapabilityTTL {
		caps := entry.caps
		m.capMu.Unlock()
		return caps
	}
	m.capMu.Unlock()

	list, err := m.capabilityFunc(ctx, serverID)
	if err != nil {
		// 探测失败（节点离线/超时）也写负缓存：否则每次生成配置都会再等一轮
		// gRPC 超时，把「预览配置」拖成十几秒。60s 后再试。
		m.capMu.Lock()
		if m.capCache == nil {
			m.capCache = make(map[int64]nginxCapabilityEntry)
		}
		m.capCache[serverID] = nginxCapabilityEntry{caps: nil, at: time.Now()}
		m.capMu.Unlock()
		return nil
	}
	caps := make(map[string]bool, len(list))
	for _, c := range list {
		caps[c] = true
	}

	m.capMu.Lock()
	if m.capCache == nil {
		m.capCache = make(map[int64]nginxCapabilityEntry)
	}
	m.capCache[serverID] = nginxCapabilityEntry{caps: caps, at: time.Now()}
	m.capMu.Unlock()
	return caps
}

// SupportsStreamProxyProtocolVar 判断该节点的 nginx 能否按域名分别发送
// PROXY Protocol（`proxy_protocol $var`）。
//
// 不支持时生成器必须回退旧模式；支持时才能把 sing-box 的 REALITY 域名也
// 映射到 443（sing-box 不能收 PROXY 头）。
func (m *NginxManager) SupportsStreamProxyProtocolVar(ctx context.Context, serverID int64) bool {
	return m.NginxCapabilities(ctx, serverID)[NginxCapStreamPPVar]
}
