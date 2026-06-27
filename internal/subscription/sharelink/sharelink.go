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
	Tag          string
	Protocol     string
	Host         string
	Port         int
	Settings     map[string]interface{}
	StreamSettings map[string]interface{}
	FinalMask    map[string]interface{}
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
	case "hysteria":
		return generateHysteriaLink(node)
	case "shadowsocks":
		return generateShadowsocksLink(node)
	}
	return ""
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
		// Parse early data from path (?ed=2560)
		if idx := strings.Index(path, "?ed="); idx >= 0 {
			edStr := path[idx+4:]
			// Remove any trailing params
			if ampIdx := strings.Index(edStr, "&"); ampIdx >= 0 {
				edStr = edStr[:ampIdx]
			}
			if ed, err := strconv.Atoi(edStr); err == nil && ed > 0 {
				obj["max-early-data"] = ed
				obj["early-data-header-name"] = "Sec-WebSocket-Protocol"
				path = path[:idx]
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
		url.QueryEscape(password), formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
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

	// Obfs from finalmask salamander
	if fm := n.FinalMask; fm != nil {
		if udp, ok := fm["udp"].([]interface{}); ok {
			for _, item := range udp {
				if m, ok := item.(map[string]interface{}); ok {
					if m["type"] == "salamander" {
						if s, ok := m["settings"].(map[string]interface{}); ok {
							if pwd, ok := s["password"].(string); ok && pwd != "" {
								params.Set("obfs", "salamander")
								params.Set("obfs-password", pwd)
							}
						}
					}
				}
			}
		}
		// mport from quicParams.udpHop
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

	// Hysteria specific settings
	hy := getMap(ss, "hysteriaSettings")
	if upMbps, ok := hy["up_mbps"]; ok {
		params.Set("upmbps", fmt.Sprintf("%v", upMbps))
	}
	if downMbps, ok := hy["down_mbps"]; ok {
		params.Set("downmbps", fmt.Sprintf("%v", downMbps))
	}

	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s",
		url.QueryEscape(auth), formatHost(n.Host), n.Port, encodeParams(params), url.PathEscape(n.Tag))
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
	v, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range v {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func getFingerprint(tls map[string]interface{}) string {
	if tls == nil {
		return ""
	}
	// settings[0].fingerprint
	if settings, ok := tls["settings"].([]interface{}); ok && len(settings) > 0 {
		if s, ok := settings[0].(map[string]interface{}); ok {
			if fp, ok := s["fingerprint"].(string); ok {
				return fp
			}
		}
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
