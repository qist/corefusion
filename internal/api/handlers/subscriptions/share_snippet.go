package subscriptions

import (
	"encoding/json"
	"strings"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/formatter"
)

// 编辑器"分享"弹窗的片段渲染。
//
// 设计原则：**优先从节点（formatter.Node）生成，而不是从分享链接生成**。
// 原因：不少协议根本没有标准 URI（naive / snell / ssh / 无认证的 http…），
// 只从链接生成时它们会全部"无法生成片段"（正是用户报的问题）；而节点是从入站配置
// 直接构造的，天然覆盖全部协议。
//
// 支持矩阵刻意复用**订阅同一份** formatSupportedProtocols —— 否则又会出现
// "订阅里没有、弹窗里能粘"的两套口径。

// shareFormat 归一化前端传来的片段格式。
//   - uri     : 分享链接（可扫码）
//   - mihomo  : mihomo(clash) proxies 片段
//   - singbox : sing-box 出站 JSON 片段
//   - xray    : xray 出站 JSON 片段
func normalizeShareFormat(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "mihomo", "mihomo_yaml", "clash", "yaml":
		return "mihomo"
	case "singbox", "sing-box", "singbox_json":
		return "singbox"
	case "xray", "xray_json":
		return "xray"
	default:
		return "uri"
	}
}

// snippetSupportMatrix 返回该片段格式的协议支持矩阵（与对应订阅格式同一份）。
func snippetSupportMatrix(format string) map[string]bool {
	switch format {
	case "mihomo":
		return formatSupportedProtocols("mihomo_yaml")
	case "singbox":
		return formatSupportedProtocols("singbox_json")
	case "xray":
		return formatSupportedProtocols("xray_json")
	default:
		return formatSupportedProtocols("base64_uri")
	}
}

// formatterNodeForShare 从入站配置 + 单个用户构造 formatter.Node。
// 与订阅的节点构造保持同一口径（协议名归一、传输归一、TLS 取 streamSettings），
// 这样"弹窗里复制的片段"与"订阅里的节点"不会各说各话。
func formatterNodeForShare(name string, cfg *entity.InboundConfig, view, client map[string]interface{}, host string, port int) *formatter.Node {
	ss := getMap(view, "streamSettings")
	protocol := cfg.Protocol
	if protocol == "hysteria" {
		protocol = "hysteria2"
	}
	transport := getString(ss, "network", "raw")
	switch transport {
	case "raw", "tcp", "":
		transport = "tcp"
	case "ws", "grpc", "httpupgrade", "xhttp", "h2", "http":
		// 保留
	default:
		transport = "tcp"
	}
	settings := getMap(view, "settings")
	uuid := getString(client, "id", "")
	if uuid == "" {
		uuid = getString(client, "uuid", "")
	}
	return &formatter.Node{
		Name:       name,
		Protocol:   protocol,
		Host:       host,
		Port:       port,
		Transport:  transport,
		TLS:        getString(ss, "security", "none") != "none",
		ServerName: getTLSServerName(ss),
		UUID:       uuid,
		Password:   getString(client, "password", ""),
		Flow:       getString(client, "flow", ""),
		Encryption: getString(settings, "method", ""),
		Extra:      buildExtra(view, ss, cfg.Protocol),
	}
}

// renderShareSnippet 渲染单个用户的可粘贴片段。
// 返回 ok=false 表示"该格式下这条记录无从渲染"（协议不支持 / 链接与节点都失败）。
func renderShareSnippet(format, link string, node *formatter.Node) (string, bool) {
	switch format {
	case "mihomo":
		// 优先走链接（与 mihomo 订阅**逐字段一致**）…
		if snippet := mihomoProxySnippet(link); snippet != "" {
			return snippet, true
		}
		// …链接拿不到（无标准 URI 的协议）时退化为节点直出。
		if snippet := mihomoProxySnippetFromNode(node); snippet != "" {
			return snippet, true
		}
		return "", false
	case "singbox":
		// 与订阅同一口径：sing-box 表达不了的传输（xhttp / splithttp / kcp）必须剔除，
		// 否则客户端会因 `unknown transport type: xhttp` 拒绝加载**整份**配置。
		if node != nil && singboxNodeDropReason(node) != "" {
			return "", false
		}
		return outboundJSONSnippet(formatter.FormatSingBoxJSON, node)
	case "xray":
		return outboundJSONSnippet(formatter.FormatXrayJSON, node)
	default:
		return link, link != ""
	}
}

// outboundJSONSnippet 用与订阅同一个 formatter 渲染单节点，再取出该出站对象。
// 只保留出站本身（订阅输出的是整份配置），便于用户直接粘进自己的 outbounds 数组。
func outboundJSONSnippet(fmtName formatter.Format, node *formatter.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	raw, err := formatter.GetFormatter(fmtName).Format([]formatter.Node{*node})
	if err != nil {
		return "", false
	}
	var full map[string]interface{}
	if err := json.Unmarshal(raw, &full); err != nil {
		return "", false
	}
	outbounds, _ := full["outbounds"].([]interface{})
	if len(outbounds) == 0 {
		return "", false
	}
	out, ok := outbounds[0].(map[string]interface{})
	if !ok {
		return "", false
	}
	pretty, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", false
	}
	return string(pretty), true
}

// mihomoProxySnippetFromNode 在拿不到链接时，直接从节点输出 mihomo proxies 片段。
//
// 只覆盖"mihomo 支持、但没有标准分享链接"的协议（http / socks5 / snell / ssh / ss）。
// 这些情况下不存在"另一条链接口径"，因此这里不会引入两套口径的问题。
// 字段名以 mihomo 源码 /opt/mihomo/adapter/outbound/*.go 的 proxy tag 为准。
func mihomoProxySnippetFromNode(node *formatter.Node) string {
	if node == nil {
		return ""
	}
	proto := strings.ToLower(node.Protocol)
	if proto == "hysteria" {
		proto = "hysteria2"
	}
	extra := node.Extra
	m := map[string]interface{}{"name": node.Name, "server": node.Host, "port": node.Port}
	switch proto {
	case "shadowsocks", "shadowsocks2022", "ss":
		m["type"] = "ss"
		m["cipher"] = node.Encryption
		m["password"] = node.Password
		m["udp"] = true
	case "http":
		m["type"] = "http"
		setIfNotEmpty(m, "username", extraString(extra, "username", "user"))
		setIfNotEmpty(m, "password", extraString(extra, "password", "pass"))
	case "socks", "socks5", "mixed":
		m["type"] = "socks5"
		setIfNotEmpty(m, "username", extraString(extra, "username", "user"))
		setIfNotEmpty(m, "password", extraString(extra, "password", "pass"))
		m["udp"] = true
	case "snell":
		m["type"] = "snell"
		// mihomo 的 snell 用 psk（不是 password）
		psk := node.Password
		if v := extraString(extra, "psk"); v != "" {
			psk = v
		}
		setIfNotEmpty(m, "psk", psk)
		if v, ok := extra["version"]; ok {
			m["version"] = v
		}
		if v, ok := extra["obfs-opts"]; ok {
			m["obfs-opts"] = v
		}
		m["udp"] = true
	case "ssh":
		m["type"] = "ssh"
		setIfNotEmpty(m, "username", extraString(extra, "username", "user"))
		setIfNotEmpty(m, "password", extraString(extra, "password", "pass"))
		setIfNotEmpty(m, "private-key", extraString(extra, "private-key", "privateKey"))
	default:
		return ""
	}
	if node.TLS {
		m["tls"] = true
		setIfNotEmpty(m, "sni", node.ServerName)
		if v, ok := extra["skip-cert-verify"]; ok {
			m["skip-cert-verify"] = v
		}
		if v, ok := extra["client-fingerprint"]; ok {
			m["client-fingerprint"] = v
		}
	}
	if len(m) <= 3 { // 只有 name/server/port 说明参数缺失，宁可不给
		return ""
	}
	var b strings.Builder
	b.WriteString("proxies:\n")
	writeMihomoProxyMap(&b, m)
	return b.String()
}

// extraString 依次取 Extra 里第一个非空字符串（不同来源的键名不一致）。
func extraString(extra map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := extra[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
