package subscriptions

import (
	"encoding/json"
	"net"
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
	// 传输名只做命名归一（raw/tcp→tcp、mkcp→kcp），**不把表达不了的传输悄悄改成 tcp**：
	// 一旦改成 tcp，片段就会变成一个"少了全部传输参数、但看起来能用"的坏节点
	// （sing-box 的 kcp、mihomo 的 kcp 都是这么漏出去的）——应该由各格式的
	// 支持性判断整条剔除，并在弹窗里明说原因。
	transport := normalizeNodeTransport(getString(ss, "network", "raw"))
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
		Password:   ss2022ClientPassword(view, cfg.Protocol, getString(client, "password", "")),
		Flow:       getString(client, "flow", ""),
		Encryption: nodeEncryption(cfg.Protocol, settings),
		Extra:      buildExtra(view, ss, cfg.Protocol),
	}
}

// ss2022UserPassword 计算 **Shadowsocks-2022 多用户** 客户端该用的密码。
//
//   - 老式 ss（aes-256-gcm 等）→ 用户密码原样；
//   - SS-2022 单用户（用户密码就是服务端密钥）→ 服务端密钥；
//   - SS-2022 **多用户** → `服务端密钥:用户密钥`（两段 base64 用冒号拼接，SS-2022 的 iPSK 约定）。
//
// 只写"用户密钥"的后果（e2e 实测）：xray 服务端**静默丢弃**连接（shadowsocks 认证失败不回错），
// 表现就是"配置能加载、节点连不上" —— 面板自建的多用户 ss2022 入站（id=70/71）全都中招。
// 实测对照（同一入站同一用户）：冒号形式**立即可通**；只给用户密钥、只给服务端密钥、
// 或把两段密钥按原始字节拼接后再 base64 都不通。
func ss2022UserPassword(method, serverKey, userKey string) string {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(method)), "2022-blake3") {
		return userKey // 老式 ss：密码就是用户密码
	}
	serverKey = strings.TrimSpace(serverKey)
	userKey = strings.TrimSpace(userKey)
	if serverKey == "" {
		return userKey
	}
	if userKey == "" || userKey == serverKey {
		return serverKey // 单用户：服务端密钥即客户端密码
	}
	return serverKey + ":" + userKey
}

// ss2022ClientPassword 从"视图配置"算客户端密码：method 与用户密钥在 settings，
// 服务端密钥在 settings.password（xray 形态）或顶层 password（sing-box 形态，见 singboxViewForUser）。
func ss2022ClientPassword(config map[string]interface{}, protocol, userPassword string) string {
	settings := getMap(config, "settings")
	method := nodeEncryption(protocol, settings)
	if !strings.HasPrefix(strings.ToLower(method), "2022-blake3") {
		return userPassword
	}
	serverKey := getString(settings, "password", "")
	if serverKey == "" {
		serverKey = getString(config, "password", "")
	}
	return ss2022UserPassword(method, serverKey, userPassword)
}

// linkSettingsForUser 构造交给 `sharelink.Generate` 的 settings：
//
//   - ss / ss2022：必须把 **method** 带上（订阅路径的 linkSettings 只挑了 clients/encryption，
//     会让 2022-blake3 的节点在链接里被写成默认的 aes-256-gcm → 客户端连不上）；
//     多用户 ss2022 的密码还要是 `服务端密钥:用户密钥`（链接层从 clients[0].password 取密码，
//     而 mihomo 片段是从**链接**派生的，漏了它 mihomo 也一起连不上）；
//   - 其它协议：原样返回。
func linkSettingsForUser(config, settings map[string]interface{}, protocol, userPassword string) map[string]interface{} {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "shadowsocks", "shadowsocks2022", "ss":
	default:
		return settings
	}
	out := make(map[string]interface{}, len(settings)+2)
	for k, v := range settings {
		out[k] = v
	}
	if method := nodeEncryption(protocol, getMap(config, "settings")); method != "" {
		out["method"] = method
	}
	pwd := ss2022ClientPassword(config, protocol, userPassword)
	clients, _ := out["clients"].([]interface{})
	if len(clients) == 0 {
		out["password"] = pwd
		return out
	}
	copied := make([]interface{}, len(clients))
	copy(copied, clients)
	if first, ok := clients[0].(map[string]interface{}); ok && first != nil {
		one := make(map[string]interface{}, len(first)+1)
		for k, v := range first {
			one[k] = v
		}
		one["password"] = pwd
		copied[0] = one
	}
	out["clients"] = copied
	return out
}

// nodeEncryption 返回客户端配置里该协议的"加密"字段（协议相关，别混用）：
//
//   - shadowsocks / 2022 → `settings.method`（mihomo `cipher`、sing-box `method`、xray `method`）；
//   - vless → `settings.encryption`（**VLESS 加密**，形如
//     `mlkem768x25519plus.native.0rtt.<key>`）：xray 写进 users[].encryption（同时写 settings.encryption
//     兼容旧客户端），mihomo 写 `encryption`。**以前这里只读 ss 的 method，于是 vless 节点的加密
//     一路丢失** —— xray 片段里成了 `encryption: "none"`（服务端在等加密握手，连不上），
//     还刚好撞上 xray 26 的"明文 vless 不许连公网地址"限制。
//   - 其它协议（或者值为空/none）→ 空串。
func nodeEncryption(protocol string, settings map[string]interface{}) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "vless":
		if enc := strings.TrimSpace(getString(settings, "encryption", "")); enc != "" && !strings.EqualFold(enc, "none") {
			return enc
		}
		return ""
	case "shadowsocks", "shadowsocks2022", "ss":
		return getString(settings, "method", "")
	default:
		// 其它协议没有"加密/method"这个客户端字段：别把 settings 里的杂项当成加密写出去。
		return ""
	}
}

// renderShareSnippet 渲染单个用户的可粘贴片段。
// 返回 ok=false 表示"该格式下这条记录无从渲染"（协议不支持 / 链接与节点都失败）。
func renderShareSnippet(format, link string, node *formatter.Node) (string, bool) {
	switch format {
	case "mihomo":
		// mihomo 表达不了的传输（kcp/mkcp/quic/splithttp）必须整条剔除：
		// 写进去客户端只会静默回退成普通 TCP —— 参数全丢、还装作能用。
		if mihomoNodeDropReason(node) != "" {
			return "", false
		}
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
		// xray 26 起对**明文 vless / trojan** 有硬性限制（见 xrayNodeDropReason）：
		// 写进去会让客户端**整份配置加载失败**（订阅里就是一个节点废掉整份），
		// 所以整条剔除并在弹窗里说明原因。
		if node != nil && xrayNodeDropReason(node) != "" {
			return "", false
		}
		return outboundJSONSnippet(formatter.FormatXrayJSON, node)
	default:
		return link, link != ""
	}
}

// xrayNodeDropReason 返回"该节点会被 xray 客户端**拒绝加载**"的原因（空串 = 可以生成）。
//
// 实测 xray 26.9.9：明文 vless / trojan（没有 TLS/REALITY，vless 也没有 VLESS 加密）时，
// 只允许连**私有地址（含 localhost）**，公网 IP 与域名一律拒载：
//
//	vless without TLS or other encryption is prohibited unless the server address
//	is a private IP or domain      ← 注意：实测域名同样被拒，报错文案有误导
//	trojan without TLS is prohibited unless the server address is a private IP...
//
// 这类节点一旦写进客户端配置，xray 是**整份拒绝**（不是跳过这一条），
// 所以必须在生成阶段整条剔除 —— 与 kcp/xhttp 的处理口径一致。
// 其它协议（vmess/ss/hysteria…）或走 TLS 的节点不受影响。
func xrayNodeDropReason(node *formatter.Node) string {
	if node == nil {
		return ""
	}
	proto := strings.ToLower(node.Protocol)
	if proto != "vless" && proto != "trojan" {
		return ""
	}
	// TLS 与 REALITY 都算"加密"
	if node.TLS || node.Extra["reality-opts"] != nil {
		return ""
	}
	if proto == "vless" {
		// VLESS 加密（mlkem768x25519plus…）也算"other encryption"，此时允许公网地址
		if enc := strings.TrimSpace(node.Encryption); enc != "" && !strings.EqualFold(enc, "none") {
			return ""
		}
	}
	if isLocalOrPrivateHost(node.Host) {
		return ""
	}
	return "plain-" + proto
}

// isLocalOrPrivateHost 判断分享地址是否是"明文 vless/trojan 仍然允许"的本地/私有地址。
// 注意：域名**不算**（xray 实测会拒），所以这里只认 IP 段。
func isLocalOrPrivateHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "" {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// shareUnsupportedReason 说明"该格式下为什么生不出片段"（供前端把原因显示出来）。
// 空串 = 能生成。
func shareUnsupportedReason(format string, node *formatter.Node) string {
	if node == nil {
		return ""
	}
	switch format {
	case "singbox":
		return singboxNodeDropReason(node)
	case "mihomo":
		return mihomoNodeDropReason(node)
	case "xray":
		return xrayNodeDropReason(node)
	default:
		return ""
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
