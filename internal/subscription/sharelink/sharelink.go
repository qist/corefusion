package sharelink

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
)

// Node holds parsed inbound info for share link generation.
type Node struct {
	Tag            string
	Protocol       string
	Host           string
	Port           int
	Settings       map[string]interface{}
	StreamSettings map[string]interface{}
	FinalMask      map[string]interface{}
}

// Generate generates a share link for the given node (mirrors frontend generateShareLink).
func Generate(node Node) string {
	switch node.Protocol {
	case "vless":
		return generateVlessLink(node)
	case "vmess":
		return generateVmessLink(node)
	case "trojan":
		return generateTrojanLink(node)
	case "hysteria", "hysteria2":
		return generateHysteriaLink(node)
	case "shadowsocks", "shadowsocks2022", "ss":
		return generateShadowsocksLink(node)
	case "tuic":
		return generateTuicLink(node)
	case "http":
		return generateAuthLink("http://", node)
	case "socks", "socks5", "mixed":
		// sing-box 的 mixed = socks + http 同端口，分享用 socks:// 客户端可识别
		return generateAuthLink("socks://", node)
	case "anytls":
		return generateAnyTLSLink(node)
	}
	return ""
}

// --- AnyTLS ---
// anytls 有**官方 URI 规范**（https://github.com/anytls/anytls-go/blob/main/docs/uri_scheme.md）：
//
//	anytls://[auth@]hostname[:port]/?[key=value]...
//
// 官方只定义 sni / insecure 两个参数，并声明"第三方实现可加扩展参数，但不保证被理解"。
// 这里在官方参数之外补 alpn / fp（面板扩展）：mihomo 的 anytls 出站认这两个字段，
// 不认的实现会忽略，不影响连接。Shadowrocket 可直接导入该链接。
func generateAnyTLSLink(n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)
	password := getClientField(clients, "password")
	if password == "" {
		password = getString(n.Settings, "password", "")
	}

	params := url.Values{}
	tls := getMap(ss, "tlsSettings")
	if sni := getString(tls, "serverName", ""); sni != "" {
		params.Set("sni", sni)
	}
	if insecure, ok := tls["allowInsecure"].(bool); ok && insecure {
		params.Set("insecure", "1")
	}
	if fp := getFingerprint(tls); fp != "" {
		params.Set("fp", fp)
	}
	if alpn := getStringSlice(tls, "alpn"); len(alpn) > 0 {
		params.Set("alpn", strings.Join(alpn, ","))
	}

	// 规范里 host 与查询串之间带一个 '/'：anytls://letmein@example.com/?sni=...
	return fmt.Sprintf("anytls://%s@%s:%d/?%s#%s",
		encodeValue(password), formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
}

// --- VLESS ---
func generateVlessLink(n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)
	uuid := getClientField(clients, "id")
	flow := getClientField(clients, "flow")
	encryption := getString(n.Settings, "encryption", "none")
	network := getString(ss, "network", "raw")
	security := getString(ss, "security", "none")

	params := url.Values{}
	params.Set("type", mkcpToKcp(network))
	params.Set("encryption", encryption)

	setNetworkParams(params, ss, network)
	setSecurityParams(params, ss, security)
	if flow != "" {
		params.Set("flow", flow)
	}
	setFinalMaskParams(params, n.FinalMask)

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		uuid, formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
}

// --- VMess ---
func generateVmessLink(n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)
	network := getString(ss, "network", "raw")
	security := getString(ss, "security", "none")

	obj := map[string]interface{}{
		"v":    "2",
		"ps":   n.Tag,
		"add":  n.Host,
		"port": n.Port,
		"id":   getClientField(clients, "id"),
		"aid":  0,
		"scy":  "auto",
		"net":  mkcpToKcp(network),
		"type": "none",
		"host": "",
		"path": "",
		"tls":  "",
		"sni":  "",
		"alpn": "",
		"fp":   "",
	}

	// Network-specific
	switch network {
	case "tcp", "raw":
		tcp := getMap(ss, "raw")
		if getString(tcp, "headerType", "") == "http" {
			obj["type"] = "http"
			obj["host"] = getHeaderHost(tcp)
			obj["path"] = getTCPPath(tcp)
		}
	case "kcp", "mkcp":
		kcp := getMap(ss, "kcp")
		obj["mtu"] = getInt(kcp, "mtu", 1350)
		obj["tti"] = getInt(kcp, "tti", 50)
		header, seed := getMkcpLegacyHeader(n.FinalMask)
		if header != "" {
			obj["headerType"] = header
		}
		if seed != "" {
			obj["seed"] = seed
		}
	case "ws":
		ws := getMap(ss, "ws")
		path := getString(ws, "path", "/")
		// Detect early data in path (?ed=2560), keep path as-is
		if idx := strings.Index(path, "?ed="); idx >= 0 {
			edStr := path[idx+4:]
			if ampIdx := strings.Index(edStr, "&"); ampIdx >= 0 {
				edStr = edStr[:ampIdx]
			}
			if _, err := strconv.Atoi(edStr); err == nil {
				obj["max-early-data"] = 4096
				obj["early-data-header-name"] = "Sec-WebSocket-Protocol"
			}
		}
		if path == "" {
			path = "/"
		}
		obj["path"] = path
		obj["host"] = getWSHost(ws)
	case "grpc":
		grpc := getMap(ss, "grpc")
		obj["path"] = getString(grpc, "serviceName", "")
		obj["authority"] = getString(grpc, "authority", "")
		if getBool(getMap(grpc, "settings"), "multiMode") {
			obj["type"] = "multi"
		}
	case "httpupgrade":
		hu := getMap(ss, "httpupgrade")
		obj["path"] = getString(hu, "path", "/")
		obj["host"] = getString(hu, "host", "")
	case "xhttp":
		xhttp := getMap(ss, "xhttp")
		obj["path"] = getString(xhttp, "path", "/")
		obj["host"] = getString(xhttp, "host", "")
		obj["type"] = getString(xhttp, "mode", "auto")
	}

	// TLS
	if security == "tls" {
		tls := getMap(ss, "tlsSettings")
		obj["tls"] = "tls"
		obj["sni"] = getString(tls, "serverName", "")
		obj["alpn"] = strings.Join(getStringSlice(tls, "alpn"), ",")
		obj["fp"] = getFingerprint(tls)
	}

	// FinalMask
	fmData := serializeFinalMask(n.FinalMask)
	if fmData != "" {
		obj["fm"] = fmData
	}

	data, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(data)
}

// --- Trojan ---
func generateTrojanLink(n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)
	password := getClientField(clients, "password")
	network := getString(ss, "network", "raw")
	security := getString(ss, "security", "tls")

	params := url.Values{}
	params.Set("type", mkcpToKcp(network))

	setNetworkParams(params, ss, network)
	setSecurityParams(params, ss, security)

	fmData := serializeFinalMask(n.FinalMask)
	if fmData != "" {
		params.Set("fm", fmData)
	}

	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s",
		encodeValue(password), formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
}

// --- Hysteria ---
func generateHysteriaLink(n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)
	auth := getClientField(clients, "auth")
	security := getString(ss, "security", "tls")

	params := url.Values{}

	// TLS params
	if security == "tls" {
		params.Set("security", "tls")
		tls := getMap(ss, "tlsSettings")
		if fp := getFingerprint(tls); fp != "" {
			params.Set("fp", fp)
		}
		if alpn := getStringSlice(tls, "alpn"); len(alpn) > 0 {
			params.Set("alpn", strings.Join(alpn, ","))
		}
		if sni := getString(tls, "serverName", ""); sni != "" {
			params.Set("sni", sni)
		}
	}

	// Obfs / bbr-profile / insecure：sing-box 侧由 singbox_share 归一化进 hysteriaSettings
	//（sing-box 把混淆放在顶层 obfs 结构体、拥塞控制叫 bbr_profile），xray 侧仍写在 finalmask。
	// 少了它们，服务端开了混淆/指定 BBR 时客户端拿到的链接就是"参数不全"。
	hy := getMap(ss, "hysteriaSettings")
	obfsType := getString(hy, "obfs", "")
	obfsPwd := getString(hy, "obfs_password", "")
	if obfsPwd == "" {
		obfsPwd = getString(hy, "obfsPassword", "")
	}
	if bbr := getString(hy, "bbr_profile", ""); bbr != "" {
		params.Set("bbr-profile", bbr)
	}
	if insecure, ok := hy["insecure"].(bool); ok && insecure {
		params.Set("insecure", "1")
	}

	if fm := n.FinalMask; fm != nil {
		// xray 口径：salamander 写在 finalmask.udp[] 里，仅当上面没拿到 obfs 时使用
		if obfsType == "" {
			if udp, ok := fm["udp"].([]interface{}); ok {
				for _, item := range udp {
					if m, ok := item.(map[string]interface{}); ok {
						if m["type"] == "salamander" {
							if st, ok := m["settings"].(map[string]interface{}); ok {
								if pwd, ok := st["password"].(string); ok && pwd != "" {
									obfsType = "salamander"
									obfsPwd = pwd
								}
							}
						}
					}
				}
			}
		}
		// mport from quicParams.udpHop（sing-box 的 _portHop 由 finalMaskWithPortHop 合成到此）
		if qp, ok := fm["quicParams"].(map[string]interface{}); ok {
			if hop, ok := qp["udpHop"].(map[string]interface{}); ok {
				if ports, ok := hop["ports"].(string); ok && ports != "" {
					params.Set("mport", ports)
				}
			}
		}
		// Full finalmask JSON
		fmData := serializeFinalMask(fm)
		if fmData != "" {
			params.Set("fm", fmData)
		}
	}
	if obfsType != "" && obfsPwd != "" {
		params.Set("obfs", obfsType)
		params.Set("obfs-password", obfsPwd)
	}

	// Hysteria specific settings（hy 已在上面取过 hysteriaSettings）
	if upMbps, ok := hy["up_mbps"]; ok {
		params.Set("upmbps", fmt.Sprintf("%v", upMbps))
	}
	if downMbps, ok := hy["down_mbps"]; ok {
		params.Set("downmbps", fmt.Sprintf("%v", downMbps))
	}

	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s",
		encodeValue(auth), formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
}

// --- Shadowsocks ---
func generateShadowsocksLink(n Node) string {
	clients := getClients(n.Settings)
	password := getClientField(clients, "password")
	method := getString(n.Settings, "method", "aes-256-gcm")

	if method == "" {
		method = "aes-256-gcm"
	}

	// ss://base64(method:password)@host:port#tag
	auth := base64.StdEncoding.EncodeToString([]byte(method + ":" + password))
	return fmt.Sprintf("shadowsocks://%s@%s:%d#%s",
		auth, formatHost(n.Host), n.Port, url.PathEscape(n.Tag))
}

// --- TUIC ---
func generateTuicLink(n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)

	params := url.Values{}
	uuid := getClientField(clients, "uuid")
	password := getClientField(clients, "password")
	// Also allow settings-level id/password (sing-box tuic uses uuid+password)
	if uuid == "" {
		uuid = getString(n.Settings, "id", "")
	}
	if password == "" {
		password = getString(n.Settings, "password", "")
	}

	userinfo := uuid
	if password != "" {
		userinfo += ":" + password
	}

	// TLS params
	tls := getMap(ss, "tlsSettings")
	if fp := getFingerprint(tls); fp != "" {
		params.Set("fp", fp)
	}
	if alpn := getStringSlice(tls, "alpn"); len(alpn) > 0 {
		params.Set("alpn", strings.Join(alpn, ","))
	}
	if sni := getString(tls, "serverName", ""); sni != "" {
		params.Set("sni", sni)
	}

	// sing-box 把 tuic 的拥塞控制 / UDP 中继模式放在**顶层**，由
	// singboxToXrayStreamSettings 归一进 streamSettings.tuicSettings；xray 形态没有这两个字段。
	tuicSettings := getMap(ss, "tuicSettings")
	if cc := getString(tuicSettings, "congestion_control", ""); cc != "" {
		params.Set("congestion_control", cc)
	} else if cc, ok := n.Settings["congestionControl"].(string); ok && cc != "" {
		params.Set("congestion_control", cc)
	} else if cc, ok := n.Settings["congestion_control"].(string); ok && cc != "" {
		params.Set("congestion_control", cc)
	}
	if m := getString(tuicSettings, "udp_relay_mode", ""); m != "" {
		params.Set("udp_relay_mode", m)
	}
	if z, ok := tuicSettings["zero_rtt_handshake"].(bool); ok && z {
		params.Set("zero_rtt_handshake", "1")
	}

	return fmt.Sprintf("tuic://%s@%s:%d?%s#%s",
		encodeValue(userinfo), formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
}

// generateAuthLink generates http:// or socks:// userinfo-based share links.
func generateAuthLink(scheme string, n Node) string {
	ss := n.StreamSettings
	clients := getClients(n.Settings)
	username := getClientField(clients, "username")
	password := getClientField(clients, "password")
	if username == "" {
		username = getString(n.Settings, "username", "")
	}
	if password == "" {
		password = getString(n.Settings, "password", "")
	}

	params := url.Values{}
	network := getString(ss, "network", "")
	if network != "" && network != "tcp" && network != "raw" {
		params.Set("network", network)
	}
	security := getString(ss, "security", "none")
	params.Set("security", security)
	tls := getMap(ss, "tlsSettings")
	if sni := getString(tls, "serverName", ""); sni != "" {
		params.Set("sni", sni)
	}
	setFinalMaskParams(params, n.FinalMask)

	// 用户名 / 密码**分别**转义：此前把 "user:pass" 整体 QueryEscape 会把分隔用的
	// ':' 编成 %3A，很多客户端按第一个 ':' 切分 userinfo → 认证直接失败。
	userinfo := encodeValue(username)
	if password != "" {
		userinfo += ":" + encodeValue(password)
	}

	return fmt.Sprintf("%s%s@%s:%d?%s#%s",
		scheme, userinfo, formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
}

// --- helpers ---

func mkcpToKcp(network string) string {
	if network == "mkcp" {
		return "kcp"
	}
	return network
}

// encodeParams encodes URL params with %20 for spaces instead of +.
func encodeParams(params url.Values) string {
	s := params.Encode()
	return strings.ReplaceAll(s, "+", "%20")
}

// encodeValue encodes a single value with %20 for spaces instead of +.
func encodeValue(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func formatHost(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func getClients(settings map[string]interface{}) []interface{} {
	if settings == nil {
		return nil
	}
	if clients, ok := settings["clients"].([]interface{}); ok {
		return clients
	}
	return nil
}

func getClientField(clients []interface{}, field string) string {
	if len(clients) == 0 {
		return ""
	}
	c, ok := clients[0].(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := c[field].(string)
	return v
}

func getString(m map[string]interface{}, key, def string) string {
	if m == nil {
		return def
	}
	v, ok := m[key].(string)
	if !ok || v == "" {
		return def
	}
	return v
}

func getInt(m map[string]interface{}, key string, def int) int {
	if m == nil {
		return def
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

func getBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	v, ok := m[key].(map[string]interface{})
	if !ok {
		return nil
	}
	return v
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	// 同时支持 []string（面板内部构造的视图，如 singboxToXrayStreamSettings 的
	// realmSettings.serverNames/shortIds）与 []interface{}（从 JSON 反序列化的 xray 配置）。
	// 只认 []interface{} 会让前者静默丢空 —— 表现就是 REALITY 分享链接缺 sni / sid。
	switch v := m[key].(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func getFingerprint(tls map[string]interface{}) string {
	if tls == nil {
		return ""
	}
	// settings[0].fingerprint（xray 形态）
	if settings, ok := tls["settings"].([]interface{}); ok && len(settings) > 0 {
		if s, ok := settings[0].(map[string]interface{}); ok {
			if fp, ok := s["fingerprint"].(string); ok && fp != "" {
				return fp
			}
		}
	}
	// 顶层 fingerprint（面板内部构造的视图，如 sing-box 反推的 realitySettings）
	if fp, ok := tls["fingerprint"].(string); ok && fp != "" {
		return fp
	}
	return ""
}

func getHeaderHost(tcp map[string]interface{}) string {
	if tcp == nil {
		return ""
	}
	// Try tcp.request (frontend format) first
	req, ok := tcp["request"].(map[string]interface{})
	if !ok {
		// xray config: tcp.header.request
		if header := getMap(tcp, "header"); header != nil {
			req, _ = header["request"].(map[string]interface{})
		}
	}
	if req == nil {
		return ""
	}
	// xray config format: {"Host": ["www.baidu.com"]}
	if headers, ok := req["headers"].(map[string]interface{}); ok {
		if hostArr, ok := headers["Host"].([]interface{}); ok && len(hostArr) > 0 {
			if h, ok := hostArr[0].(string); ok {
				return h
			}
		}
		if hostArr, ok := headers["host"].([]interface{}); ok && len(hostArr) > 0 {
			if h, ok := hostArr[0].(string); ok {
				return h
			}
		}
	}
	// Frontend format: [{"key":"Host","value":"www.baidu.com"}]
	if headers, ok := req["headers"].([]interface{}); ok {
		for _, h := range headers {
			if m, ok := h.(map[string]interface{}); ok {
				if strings.EqualFold(getString(m, "key", ""), "host") {
					return getString(m, "value", "")
				}
			}
		}
	}
	return ""
}

func getTCPPath(tcp map[string]interface{}) string {
	if tcp == nil {
		return "/"
	}
	// Try tcp.request (frontend format) first
	req, ok := tcp["request"].(map[string]interface{})
	if !ok {
		// xray config: tcp.header.request
		if header := getMap(tcp, "header"); header != nil {
			req, _ = header["request"].(map[string]interface{})
		}
	}
	if req == nil {
		return "/"
	}
	if p, ok := req["_pathsStr"].(string); ok && p != "" {
		return p
	}
	if paths, ok := req["path"].([]interface{}); ok && len(paths) > 0 {
		if p, ok := paths[0].(string); ok {
			return p
		}
	}
	return "/"
}

func getWSHost(ws map[string]interface{}) string {
	if ws == nil {
		return ""
	}
	if h, ok := ws["host"].(string); ok && h != "" {
		return h
	}
	// xray config format: ws.headers.Host
	if headers, ok := ws["headers"].(map[string]interface{}); ok {
		if hostArr, ok := headers["Host"].([]interface{}); ok && len(hostArr) > 0 {
			if h, ok := hostArr[0].(string); ok {
				return h
			}
		}
	}
	// Frontend format: ws.settings._headers
	if s, ok := ws["settings"].(map[string]interface{}); ok {
		if headers, ok := s["_headers"].([]interface{}); ok {
			for _, h := range headers {
				if m, ok := h.(map[string]interface{}); ok {
					if strings.EqualFold(getString(m, "key", ""), "host") {
						return getString(m, "value", "")
					}
				}
			}
		}
	}
	return ""
}

func getMkcpLegacyHeader(fm map[string]interface{}) (header, seed string) {
	if fm == nil {
		return
	}
	udp, ok := fm["udp"].([]interface{})
	if !ok {
		return
	}
	for _, item := range udp {
		m, ok := item.(map[string]interface{})
		if !ok || m["type"] != "mkcp-legacy" {
			continue
		}
		s, ok := m["settings"].(map[string]interface{})
		if !ok {
			continue
		}
		if h, ok := s["header"].(string); ok && h != "" {
			header = h
		}
		if v, ok := s["value"].(string); ok && v != "" {
			seed = v
		}
	}
	return
}

// setNetworkParams sets transport-specific query params (mirrors frontend).
func setNetworkParams(params url.Values, ss map[string]interface{}, network string) {
	switch network {
	case "tcp", "raw":
		// Try rawSettings first (xray config format), then raw (frontend format)
		tcp := getMap(ss, "rawSettings")
		if tcp == nil {
			tcp = getMap(ss, "raw")
		}
		// xray config: rawSettings.header.type = "http"
		headerType := getString(tcp, "headerType", "")
		if headerType == "" {
			if header := getMap(tcp, "header"); header != nil {
				headerType = getString(header, "type", "")
			}
		}
		if headerType == "http" {
			params.Set("headerType", "http")
			params.Set("path", getTCPPath(tcp))
			if h := getHeaderHost(tcp); h != "" {
				params.Set("host", h)
			}
		}
	case "kcp", "mkcp":
		kcp := getMap(ss, "kcp")
		params.Set("mtu", fmt.Sprintf("%d", getInt(kcp, "mtu", 1350)))
		params.Set("tti", fmt.Sprintf("%d", getInt(kcp, "tti", 50)))
		header, seed := getMkcpLegacyHeader(getMap(ss, "finalmask"))
		if header != "" {
			params.Set("headerType", header)
		}
		if seed != "" {
			params.Set("seed", seed)
		}
	case "h2", "http":
		// h2（sing-box 的 transport.type=http / xray 的 network=h2）
		http := getMap(ss, "httpSettings")
		if http == nil {
			http = getMap(ss, "http")
		}
		params.Set("path", getString(http, "path", "/"))
		if h := getString(http, "host", ""); h != "" {
			params.Set("host", h)
		}
	case "ws":
		ws := getMap(ss, "wsSettings")
		if ws == nil {
			ws = getMap(ss, "ws")
		}
		params.Set("path", getString(ws, "path", "/"))
		params.Set("host", getWSHost(ws))
	case "grpc":
		grpc := getMap(ss, "grpcSettings")
		if grpc == nil {
			grpc = getMap(ss, "grpc")
		}
		params.Set("serviceName", getString(grpc, "serviceName", ""))
		if auth := getString(grpc, "authority", ""); auth != "" {
			params.Set("authority", auth)
		}
		if getBool(getMap(grpc, "settings"), "multiMode") {
			params.Set("mode", "multi")
		}
	case "httpupgrade":
		hu := getMap(ss, "httpupgradeSettings")
		if hu == nil {
			hu = getMap(ss, "httpupgrade")
		}
		params.Set("path", getString(hu, "path", "/"))
		params.Set("host", getString(hu, "host", ""))
	case "xhttp":
		xhttp := getMap(ss, "xhttpSettings")
		if xhttp == nil {
			xhttp = getMap(ss, "xhttp")
		}
		params.Set("path", getString(xhttp, "path", "/"))
		params.Set("host", getString(xhttp, "host", ""))
		params.Set("mode", getString(xhttp, "mode", "auto"))
		if xpb := getString(xhttp, "xPaddingBytes", ""); xpb != "" {
			params.Set("x_padding_bytes", xpb)
		}
		// Serialize xhttp extra fields
		extra := buildXhttpExtra(xhttp)
		if extra != "" {
			params.Set("extra", extra)
		}
	}
}

// setSecurityParams sets security-specific query params (mirrors frontend).
func setSecurityParams(params url.Values, ss map[string]interface{}, security string) {
	switch security {
	case "tls":
		params.Set("security", "tls")
		tls := getMap(ss, "tlsSettings")
		if fp := getFingerprint(tls); fp != "" {
			params.Set("fp", fp)
		}
		if alpn := getStringSlice(tls, "alpn"); len(alpn) > 0 {
			params.Set("alpn", strings.Join(alpn, ","))
		}
		if sni := getString(tls, "serverName", ""); sni != "" {
			params.Set("sni", sni)
		}
	case "reality":
		params.Set("security", "reality")
		// REALITY 的 ALPN 也要下发（v2ray 系客户端会按它做 TLS 握手，
		// 缺了可能与服务端协商到不同协议；前端生成器一直有，后端此前漏了）。
		if tls := getMap(ss, "tlsSettings"); tls != nil {
			if alpn := getStringSlice(tls, "alpn"); len(alpn) > 0 {
				params.Set("alpn", strings.Join(alpn, ","))
			}
		}
		reality := getMap(ss, "realitySettings")
		// pbk from settings[0].password or _password
		if pwd, ok := reality["_password"].(string); ok && pwd != "" {
			params.Set("pbk", pwd)
		} else if settings, ok := reality["settings"].([]interface{}); ok && len(settings) > 0 {
			if s, ok := settings[0].(map[string]interface{}); ok {
				if pwd, ok := s["password"].(string); ok && pwd != "" {
					params.Set("pbk", pwd)
				}
			}
		}
		// fp
		if fp := getFingerprint(reality); fp != "" {
			params.Set("fp", fp)
		}
		// sni from serverNames
		if sniList := getStringSlice(reality, "serverNames"); len(sniList) > 0 {
			params.Set("sni", sniList[rand.Intn(len(sniList))])
		}
		// sid from shortIds
		if sidList := getStringSlice(reality, "shortIds"); len(sidList) > 0 {
			params.Set("sid", sidList[rand.Intn(len(sidList))])
		}
		if spx, ok := reality["spiderX"].(string); ok && spx != "" {
			params.Set("spx", spx)
		}
		// pqv (post-quantum verification) from settings[0].mldsa65Verify
		if settings, ok := reality["settings"].([]interface{}); ok && len(settings) > 0 {
			if s, ok := settings[0].(map[string]interface{}); ok {
				if pqv, ok := s["mldsa65Verify"].(string); ok && pqv != "" {
					params.Set("pqv", pqv)
				}
			}
		}
	default:
		params.Set("security", "none")
	}
}

// setFinalMaskParams sets the fm query param from finalmask config.
func setFinalMaskParams(params url.Values, fm map[string]interface{}) {
	fmData := serializeFinalMask(fm)
	if fmData != "" {
		params.Set("fm", fmData)
	}
}

// buildXhttpExtra serializes xhttp extra fields to JSON string.
func buildXhttpExtra(xhttp map[string]interface{}) string {
	if xhttp == nil {
		return ""
	}
	// Merge xhttp.extra sub-object into the main xhttp map for unified access
	xhttpExtra := getMap(xhttp, "extra")
	merged := xhttp
	if xhttpExtra != nil {
		merged = make(map[string]interface{})
		for k, v := range xhttp {
			merged[k] = v
		}
		for k, v := range xhttpExtra {
			if _, exists := merged[k]; !exists {
				merged[k] = v
			}
		}
	}
	extra := make(map[string]interface{})
	// Direct extra fields on xhttp object (merged with xhttp.extra)
	setStr := func(src, dst string) {
		if v, ok := merged[src].(string); ok && v != "" {
			extra[dst] = v
		}
	}
	setBool := func(src, dst string) {
		if v, ok := merged[src].(bool); ok && v {
			extra[dst] = true
		}
	}
	setInt := func(src, dst string) {
		if v, ok := merged[src].(float64); ok && v > 0 {
			extra[dst] = int(v)
		}
	}
	setStr("noGRPCHeader", "noGRPCHeader")
	setStr("xPaddingBytes", "xPaddingBytes")
	setBool("xPaddingObfsMode", "xPaddingObfsMode")
	setStr("xPaddingKey", "xPaddingKey")
	setStr("xPaddingHeader", "xPaddingHeader")
	setStr("xPaddingPlacement", "xPaddingPlacement")
	setStr("xPaddingMethod", "xPaddingMethod")
	setStr("uplinkHttpMethod", "uplinkHTTPMethod")
	setStr("sessionIDPlacement", "sessionIDPlacement")
	setStr("sessionPlacement", "sessionPlacement")
	setStr("sessionIDKey", "sessionIDKey")
	setStr("sessionKey", "sessionKey")
	setStr("seqPlacement", "seqPlacement")
	setStr("seqKey", "seqKey")
	setStr("uplinkDataPlacement", "uplinkDataPlacement")
	setStr("uplinkDataKey", "uplinkDataKey")
	setInt("uplinkChunkSize", "uplinkChunkSize")
	setInt("scMaxEachPostBytes", "scMaxEachPostBytes")
	setInt("scMinPostsIntervalMs", "scMinPostsIntervalMs")
	setInt("scMaxBufferedPosts", "scMaxBufferedPosts")
	setStr("scStreamUpServerSecs", "scStreamUpServerSecs")
	// xmux
	if xmux := getMap(merged, "xmux"); xmux != nil && len(xmux) > 0 {
		extra["xmux"] = xmux
	}
	if len(extra) == 0 {
		return ""
	}
	b, _ := json.Marshal(extra)
	return string(b)
}

// serializeFinalMask converts finalmask config to JSON string (mirrors frontend serializeFinalMask).
func serializeFinalMask(fm map[string]interface{}) string {
	if fm == nil {
		return ""
	}
	data := make(map[string]interface{})

	// TCP masks
	if tcp, ok := fm["tcp"].([]interface{}); ok && len(tcp) > 0 {
		data["tcp"] = tcp
	}

	// UDP masks
	if udp, ok := fm["udp"].([]interface{}); ok && len(udp) > 0 {
		data["udp"] = udp
	}

	// QUIC params
	if qp, ok := fm["quicParams"].(map[string]interface{}); ok {
		data["quicParams"] = qp
	}

	if len(data) == 0 {
		return ""
	}
	b, _ := json.Marshal(data)
	return string(b)
}
