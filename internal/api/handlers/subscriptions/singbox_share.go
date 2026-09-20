package subscriptions

import (
	"fmt"
	"log"
	"strings"

	realityutil "corefusion/internal/reality"
)

// ─────────────────────────────────────────────────────────────────────────────
// sing-box 入站 → 订阅渲染（xray 形态归一）
//
// 订阅渲染链路（分享链接 sharelink.Generate / formatter.Node）原本只认 xray 形态：
//
//	xray    : settings.clients[]（用户凭证，email 标识）+ streamSettings（network/security）
//	sing-box: 顶层 users[]（name 标识）+ transport/tls
//
// 结果就是 sing-box 入站**永远不会出现在订阅里**（用户凭证在 settings 里找不到）。
//
// 这里把 sing-box 入站归一成 xray 形态（settings.clients 只保留该订阅用户，
// streamSettings 由 transport/tls 反推），下游 sharelink / formatter 即可原样复用；
// 字段口径与前端 web/admin/.../singbox/utils/singboxShareLink.js 保持一致。
// ─────────────────────────────────────────────────────────────────────────────

// isSingboxInboundConfig 判断入站配置是否为 sing-box 形态。
func isSingboxInboundConfig(config map[string]interface{}) bool {
	if config == nil {
		return false
	}
	if _, hasSettings := config["settings"]; hasSettings {
		return false // xray 形态
	}
	_, hasType := config["type"]
	return hasType
}

// finalMaskWithPortHop 返回订阅渲染用的 finalmask：原本取 inbound 的 finalmask，
// 但如果入站只配了 _portHop（端口跳跃）而没有 finalmask，就用 _portHop 合成一个
// 只含 udpHop 的 finalmask，让分享链接带上 mport（客户端需要它才能在端口段内跳跃）。
//
// 为什么不在入站配置里直接写 finalmask：sing-box 入站是严格解码，finalmask /
// quicParams 不是它的字段，写进去会 FATAL —— 因此端口跳跃对 sing-box 靠
// 「防火墙 DNAT 端口段 → 单端口」实现，客户端跳跃范围则通过链接的 mport 下发。
func finalMaskWithPortHop(config map[string]interface{}) map[string]interface{} {
	fm := getFinalMask(config)
	if fm != nil {
		if qp, ok := fm["quicParams"].(map[string]interface{}); ok {
			if hop, ok := qp["udpHop"].(map[string]interface{}); ok {
				if ports, _ := hop["ports"].(string); ports != "" {
					return fm
				}
			}
		}
	}
	ports := portHopRange(config)
	if ports == "" {
		return fm
	}
	hop := map[string]interface{}{"ports": ports}
	if fm == nil {
		fm = map[string]interface{}{}
	}
	qp, _ := fm["quicParams"].(map[string]interface{})
	if qp == nil {
		qp = map[string]interface{}{}
		fm["quicParams"] = qp
	}
	qp["udpHop"] = hop
	return fm
}

// portHopRange 从 _portHop 取端口段（预设用 range，手动用 start-end）。
func portHopRange(config map[string]interface{}) string {
	ph, ok := config["_portHop"].(map[string]interface{})
	if !ok {
		return ""
	}
	if enabled, _ := ph["enabled"].(bool); !enabled {
		return ""
	}
	if mode, _ := ph["mode"].(string); mode == "manual" {
		start, _ := ph["start"].(float64)
		end, _ := ph["end"].(float64)
		if start > 0 && end > 0 {
			return fmt.Sprintf("%d-%d", int(start), int(end))
		}
	}
	if r, _ := ph["range"].(string); r != "" {
		return r
	}
	return ""
}

// ensureAccessDomain 把历史遗留的扁平字段（_accessDomainEnabled / _directDomain …）
// 归一成订阅端真正读取的 _accessDomain 对象。
//
// 背景：xray 编辑器一直写的是 _accessDomain 对象，而 sing-box 编辑器此前只写扁平
// 字段（且这些扁平字段会被后端当内部字段剔除），导致 sing-box 入站在订阅里
// 拿不到域名与端口 —— 客户端就算拿到链接也访问不了。这里做一次归一，
// 老数据无需重新保存即可生效。
func ensureAccessDomain(config map[string]interface{}) {
	if config == nil {
		return
	}
	if _, ok := config["_accessDomain"].(map[string]interface{}); ok {
		return
	}
	enabled, _ := config["_accessDomainEnabled"].(bool)
	adType, _ := config["_accessDomainType"].(string)
	if !enabled && adType == "" {
		return // 未配置接入域名
	}
	str := func(key string) string {
		v, _ := config[key].(string)
		return v
	}
	port := 443
	if v, ok := config["_accessCustomPort"].(float64); ok && v > 0 {
		port = int(v)
	}
	ad := map[string]interface{}{
		"enabled":        enabled,
		"type":           adType,
		"directDomain":   str("_directDomain"),
		"directPortMode": str("_directPortMode"),
		"cfDomain":       str("_cfDomain"),
		"cfIP":           str("_cfIP"),
		"customDomain":   str("_customDomain"),
		"customPortMode": str("_customPortMode"),
		"customPort":     port,
	}
	if ad["type"] == "" {
		ad["type"] = "direct"
	}
	if ad["directPortMode"] == "" {
		ad["directPortMode"] = "config"
	}
	if ad["customPortMode"] == "" {
		ad["customPortMode"] = "custom"
	}
	config["_accessDomain"] = ad
	log.Printf("[subscriptions] 入站 %v 使用扁平接入域名字段，已归一为 _accessDomain 对象", config["tag"])
}

// subscriptionView 把某个入站配置归一成「订阅渲染用的 xray 形态」。
//
// 返回值 skip=true 表示该节点不该出现在这个订阅里（例如该订阅用户在此节点没有账号，
// 绝不复用他人凭证）。xray 入站原样返回。
func subscriptionView(tag string, config map[string]interface{}, clientEmail string) (map[string]interface{}, bool) {
	// 接入域名归一（xray 与 sing-box 都走一次，历史数据也能生效）
	ensureAccessDomain(config)
	if !isSingboxInboundConfig(config) {
		return config, false
	}
	user := findSingboxUserForSubscription(config, clientEmail, tag)
	if user == nil {
		// 该订阅用户在此 sing-box 入站没有账号 → 跳过（安全：不使用他人凭证）
		return nil, true
	}
	return singboxViewForUser(config, user, getString(user, "name", "")), false
}

// singboxViewForUser 把 sing-box 入站 + 一条 users[] 记录归一成 xray 形态
// （settings.clients 只保留该用户，streamSettings 由 transport/tls 反推）。
// clientEmail 为该记录对外的标识：订阅链路传 users[].name，面板二维码链路传原始记录名。
func singboxViewForUser(config map[string]interface{}, user map[string]interface{}, clientEmail string) map[string]interface{} {
	client := map[string]interface{}{}
	if clientEmail != "" {
		client["email"] = clientEmail
	} else if v, ok := user["name"].(string); ok {
		client["email"] = v
	}
	// sing-box: uuid（vless/vmess/tuic），password（trojan/hysteria2/ss/anytls），psk（snell），username（naive/mixed/http/socks）
	if v, ok := user["uuid"].(string); ok && v != "" {
		client["id"] = v
		client["uuid"] = v // sharelink 的 tuic 分支读 clients[].uuid
	}
	if v, ok := user["password"].(string); ok && v != "" {
		client["password"] = v
		// xray 形态里 hysteria/hysteria2 的密码字段叫 auth，sharelink 的 hysteria 分支读它
		client["auth"] = v
	}
	if v, ok := user["flow"].(string); ok && v != "" {
		client["flow"] = v
	}
	if v, ok := user["method"].(string); ok && v != "" {
		client["method"] = v
	}
	if v, ok := user["psk"].(string); ok && v != "" {
		client["psk"] = v
	}
	if v, ok := user["username"].(string); ok && v != "" {
		client["username"] = v
		client["email"] = v
	}

	protocol, _ := config["type"].(string)
	encryption := "none"
	if protocol == "shadowsocks" {
		encryption = "" // ss 用 method，不需要 encryption
	}

	out := make(map[string]interface{}, len(config)+2)
	for k, v := range config {
		out[k] = v
	}
	out["settings"] = map[string]interface{}{
		"clients":    []interface{}{client},
		"encryption": encryption,
	}
	if method, ok := config["method"].(string); ok && method != "" {
		out["method"] = method // ss：sharelink 从 settings/clients 之外取 method
	}
	out["streamSettings"] = singboxToXrayStreamSettings(config)
	return out
}

// findSingboxUserForSubscription 在 sing-box 顶层 users[] 中按 name 匹配订阅用户。
// 复用与 xray 相同的邮箱规则：{client_email}_{tag 前缀+末段}。
func findSingboxUserForSubscription(config map[string]interface{}, clientEmail, tag string) map[string]interface{} {
	if clientEmail == "" {
		return nil
	}
	inboundEmail := generateInboundEmail(clientEmail, tag)
	users, _ := config["users"].([]interface{})
	for _, u := range users {
		user, ok := u.(map[string]interface{})
		if !ok {
			continue
		}
		if name, _ := user["name"].(string); name == inboundEmail {
			return user
		}
	}
	return nil
}

// singboxToXrayStreamSettings 由 sing-box 的 transport / tls 反推 xray 的 streamSettings，
// 字段口径与前端 singboxShareLink.js 一致（network / path / host / serviceName /
// tlsSettings / realitySettings）。
func singboxToXrayStreamSettings(config map[string]interface{}) map[string]interface{} {
	ss := map[string]interface{}{"network": "tcp", "security": "none"}

	// transport：sing-box 的 "http" 表示 HTTP/2（xray 里叫 h2）
	if tr, ok := config["transport"].(map[string]interface{}); ok {
		switch t, _ := tr["type"].(string); t {
		// 注意：sharelink.Generate 读的是 xray 形态的**嵌套**设置
		// （wsSettings / grpcSettings / httpSettings / httpupgradeSettings），
		// 只写顶层的 path/host/serviceName 会被忽略 —— 表现就是分享链接里
		// serviceName 为空、ws 的 path/host 丢失。所以两种形态都写一份。
		case "ws":
			ss["network"] = "ws"
			path, host := getString(tr, "path", ""), firstString(tr["host"])
			setIfNotEmpty(ss, "path", path)
			setIfNotEmpty(ss, "host", host)
			ss["wsSettings"] = map[string]interface{}{"path": path, "host": host}
		case "grpc":
			ss["network"] = "grpc"
			svc := getString(tr, "service_name", "")
			setIfNotEmpty(ss, "serviceName", svc)
			ss["grpcSettings"] = map[string]interface{}{"serviceName": svc}
		case "http":
			ss["network"] = "h2"
			path, host := getString(tr, "path", ""), firstString(tr["host"])
			setIfNotEmpty(ss, "path", path)
			setIfNotEmpty(ss, "host", host)
			ss["httpSettings"] = map[string]interface{}{"path": path, "host": host}
		case "httpupgrade":
			ss["network"] = "httpupgrade"
			path, host := getString(tr, "path", ""), firstString(tr["host"])
			setIfNotEmpty(ss, "path", path)
			setIfNotEmpty(ss, "host", host)
			ss["httpupgradeSettings"] = map[string]interface{}{"path": path, "host": host}
		case "quic":
			ss["network"] = "quic"
		case "":
			// 无 transport
		default:
			ss["network"] = t
		}
	}

	// Hysteria2 / Hysteria：sing-box 把这些放在**顶层**（obfs / up_mbps / down_mbps /
	// bbr_profile / ignore_client_bandwidth），xray 口径在 streamSettings.hysteriaSettings。
	// 分享链接的 obfs / obfs-password / bbr-profile / insecure 都从这里取 —— 少了它们，
	// 服务端开了混淆或指定 BBR 时，客户端拿到的链接就是"能连但参数不全"。
	if hy := hysteriaViewSettings(config); len(hy) > 0 {
		ss["hysteriaSettings"] = hy
	}

	tlsCfg, _ := config["tls"].(map[string]interface{})
	if tlsCfg == nil {
		return ss
	}
	if enabled, _ := tlsCfg["enabled"].(bool); !enabled {
		return ss
	}

	ss["security"] = "tls"
	tlsSettings := map[string]interface{}{}
	if sni := getString(tlsCfg, "server_name", ""); sni != "" {
		tlsSettings["serverName"] = sni
	}
	if alpn := getStringSlice(tlsCfg, "alpn"); len(alpn) > 0 {
		tlsSettings["alpn"] = alpn
	}
	if insecure, _ := tlsCfg["insecure"].(bool); insecure {
		tlsSettings["allowInsecure"] = true
		if hy, ok := ss["hysteriaSettings"].(map[string]interface{}); ok {
			hy["insecure"] = true
		}
	}
	if utls, ok := tlsCfg["utls"].(map[string]interface{}); ok {
		if fp := getString(utls, "fingerprint", ""); fp != "" {
			tlsSettings["fingerprint"] = fp
		}
	}
	if _, has := tlsSettings["fingerprint"]; !has {
		if fp := getString(config, "_utlsFingerprint", ""); fp != "" {
			tlsSettings["fingerprint"] = fp
		}
	}
	ss["tlsSettings"] = tlsSettings

	// Reality：分享链接需要 pbk（公钥）。sing-box 服务端只存 private_key，
	// 公钥的取用顺序（重要，缺 pbk 客户端根本连不上）：
	//   1) tls.reality.public_key（历史/手工填写）
	//   2) 顶层 _realityPublicKey（编辑器保存的，生成核心配置时会被剥离，不会进 sing-box）
	//   3) 由 private_key 推导（x25519 私钥可推导公钥，覆盖只存了私钥的老数据）
	if reality, ok := tlsCfg["reality"].(map[string]interface{}); ok && reality != nil {
		if on, has := reality["enabled"].(bool); !has || on {
			ss["security"] = "reality"
			rs := map[string]interface{}{}
			pbk := getString(reality, "public_key", "")
			if pbk == "" {
				pbk = getString(config, "_realityPublicKey", "")
			}
			if pbk == "" {
				pbk = realityutil.PublicKeyFromPrivate(getString(reality, "private_key", ""))
			}
			if pbk != "" {
				rs["_password"] = pbk
			}
			if sids := getStringSlice(reality, "short_id"); len(sids) > 0 {
				rs["shortIds"] = sids
			} else if sids := getStringSlice(reality, "short_ids"); len(sids) > 0 {
				rs["shortIds"] = sids
			}
			// sni：优先 handshake.server（Reality 伪装目标），其次 tls.server_name
			if hs, ok := reality["handshake"].(map[string]interface{}); ok {
				if s := getString(hs, "server", ""); s != "" {
					rs["serverNames"] = []string{s}
				}
			}
			if _, has := rs["serverNames"]; !has {
				if sni := getString(tlsCfg, "server_name", ""); sni != "" {
					rs["serverNames"] = []string{sni}
				}
			}
			// 客户端 uTLS 指纹：优先 tls.utls.fingerprint，其次面板内部字段 _utlsFingerprint
			//（sing-box 入站没有 utls 字段，编辑器只能把它存成内部字段）。
			// 这里**不补默认值** —— 编辑器下拉是 clearable 的，用户清空就该真的不带 fp
			//（xray 侧同样由编辑器表单的默认值决定，不在订阅端硬编码）。
			fp := ""
			if v, ok := tlsSettings["fingerprint"].(string); ok {
				fp = v
			}
			if fp == "" {
				fp = getString(config, "_utlsFingerprint", "")
			}
			if fp != "" {
				rs["fingerprint"] = fp
			}
			if spx := getString(reality, "spider_x", ""); spx != "" {
				rs["spiderX"] = spx
			}
			ss["realitySettings"] = rs
			if pbk, _ := rs["_password"].(string); pbk == "" {
				log.Printf("[subscriptions] sing-box reality 入站既无 public_key 也无法由 private_key 推导，分享链接将不含 pbk（请重新生成密钥对）")
			}
		}
	}
	return ss
}

func setIfNotEmpty(m map[string]interface{}, key, val string) {
	if val != "" {
		m[key] = val
	}
}

// firstString 取字符串或字符串列表的第一个元素（sing-box 的 host 两种形态都有）。
func firstString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []interface{}:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// 订阅格式 ↔ 协议支持矩阵
//
// 不同客户端/格式能表达的协议不同：把某个格式不支持的协议塞进去，客户端要么忽略、
// 要么直接报错解析失败。因此每种格式都只保留它真正支持的协议。
// ─────────────────────────────────────────────────────────────────────────────

// formatSupportedProtocols 返回某订阅格式支持的协议集合；返回 nil 表示不做限制。
func formatSupportedProtocols(format string) map[string]bool {
	switch strings.ToLower(format) {
	case "base64_uri", "raw_uri":
		// 分享链接（URI）形态：标准 scheme，任何客户端都能解析
		return map[string]bool{
			"vless": true, "vmess": true, "trojan": true,
			"shadowsocks": true, "shadowsocks2022": true,
			"hysteria": true, "hysteria2": true, "tuic": true,
			"socks": true, "http": true,
		}
	case "xray_json":
		// xray 客户端：不支持 anytls / naive / snell / mixed 等 sing-box 专属协议
		return map[string]bool{
			"vless": true, "vmess": true, "trojan": true,
			"shadowsocks": true, "shadowsocks2022": true,
			"hysteria": true, "hysteria2": true, "tuic": true,
			"socks": true, "http": true,
		}
	case "singbox_json":
		// sing-box 客户端：支持面最广
		return map[string]bool{
			"vless": true, "vmess": true, "trojan": true,
			"shadowsocks": true, "shadowsocks2022": true,
			"hysteria": true, "hysteria2": true, "tuic": true,
			"socks": true, "http": true, "mixed": true,
			"anytls": true, "naive": true, "snell": true,
		}
	case "mihomo_yaml":
		// mihomo(clash.meta)：实测能渲染的是这些；anytls / snell / naive 目前
		// 拿不到 ProxyMap 也生成不出链接，塞进去只会得到一个空/无效条目，故不列入。
		return map[string]bool{
			"vless": true, "vmess": true, "trojan": true,
			"shadowsocks": true, "shadowsocks2022": true,
			"hysteria": true, "hysteria2": true, "tuic": true,
			"socks": true, "http": true,
		}
	default:
		return nil
	}
}

// singboxUnsupportedTransports 是 sing-box 出站**表达不了**的传输（xray 专属）。
// 只要订阅里混进一个，sing-box 客户端会因 "unknown transport type: xhttp" 拒绝
// 加载**整个**订阅 —— 所以必须整条剔除，而不是原样下发。
// 已用本机 sing-box（1.15）实测：xhttp / splithttp 报 unknown transport type；
// mKCP 系列 sing-box 没有对应传输。
var singboxUnsupportedTransports = map[string]bool{
	"xhttp": true, "splithttp": true, "kcp": true, "mkcp": true,
}

// filterRenderedNodesForFormat 按目标格式过滤渲染结果：不支持的协议 / 传输直接剔除，
// 避免客户端拿到无法解析的条目（甚至因此拒绝加载整个订阅）。
func filterRenderedNodesForFormat(items []renderedSubscriptionNode, format string) []renderedSubscriptionNode {
	supported := formatSupportedProtocols(format)
	if supported == nil {
		return items
	}
	isSingbox := strings.ToLower(format) == "singbox_json"
	kept := make([]renderedSubscriptionNode, 0, len(items))
	dropped := make([]string, 0, 4)
	for _, item := range items {
		proto := ""
		name := ""
		if item.Node != nil {
			proto = strings.ToLower(item.Node.Protocol)
			name = item.Node.Name
		}
		if proto != "" && !supported[proto] {
			dropped = append(dropped, fmt.Sprintf("%s(%s)", name, proto))
			continue
		}
		if isSingbox && item.Node != nil && singboxUnsupportedTransports[strings.ToLower(item.Node.Transport)] {
			dropped = append(dropped, fmt.Sprintf("%s(transport=%s)", name, item.Node.Transport))
			continue
		}
		kept = append(kept, item)
	}
	if len(dropped) > 0 {
		log.Printf("[subscriptions] format=%s 不支持以下条目，已从订阅中剔除: %s", format, strings.Join(dropped, ", "))
	}
	return kept
}

// hysteriaViewSettings 把 sing-box hysteria2 / hysteria 的顶层字段整理成 xray 口径的
// hysteriaSettings，供 sharelink.generateHysteriaLink 生成 obfs / obfs-password /
// bbr-profile / insecure 等链接参数。非 hysteria 入站返回空 map，调用方据此跳过。
func hysteriaViewSettings(config map[string]interface{}) map[string]interface{} {
	hy := map[string]interface{}{}
	if v, ok := config["up_mbps"]; ok && v != nil {
		hy["up_mbps"] = v
	}
	if v, ok := config["down_mbps"]; ok && v != nil {
		hy["down_mbps"] = v
	}
	// obfs：sing-box 是 {type, password} 结构体（服务端开 salamander 时客户端必须带上）
	if obfs, ok := config["obfs"].(map[string]interface{}); ok {
		if obfsType := getString(obfs, "type", ""); obfsType != "" {
			hy["obfs"] = obfsType
		}
		if pwd := getString(obfs, "password", ""); pwd != "" {
			hy["obfs_password"] = pwd
		}
	}
	if bbr := getString(config, "bbr_profile", ""); bbr != "" {
		hy["bbr_profile"] = bbr
	}
	if icb, _ := config["ignore_client_bandwidth"].(bool); icb {
		hy["ignore_client_bandwidth"] = true
	}
	return hy
}
