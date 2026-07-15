package formatter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Format represents the subscription output format.
type Format string

const (
	FormatSingBoxJSON     Format = "singbox_json"
	FormatXrayJSON        Format = "xray_json"
	FormatMihomoYAML      Format = "mihomo_yaml"
	FormatRawURI          Format = "raw_uri"
	FormatBase64Aggregate Format = "base64_uri"
)

// Node represents a proxy node for formatting.
type Node struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Transport  string `json:"transport"`
	TLS        bool   `json:"tls"`
	ServerName string `json:"server_name,omitempty"`
	UUID       string `json:"uuid,omitempty"`
	Password   string `json:"password,omitempty"`
	AlterID    int    `json:"alter_id,omitempty"`
	Encryption string `json:"encryption,omitempty"` // VLESS encryption or SS cipher method
	Flow       string `json:"flow,omitempty"`       // XTLS Vision flow
	// Extra holds all additional fields for mihomo/clash output.
	// Keys are mihomo YAML field names. Only non-nil values are rendered.
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// Formatter formats subscription content.
type Formatter interface {
	Format(nodes []Node) ([]byte, error)
	ContentType() string
	FileExtension() string
}

// GetFormatter returns the appropriate formatter for the given format.
func GetFormatter(format Format) Formatter {
	switch format {
	case FormatSingBoxJSON:
		return &SingBoxFormatter{}
	case FormatXrayJSON:
		return &XrayFormatter{}
	case FormatMihomoYAML:
		return &MihomoFormatter{}
	case FormatRawURI:
		return &RawURIFormatter{}
	case FormatBase64Aggregate:
		return &Base64AggregateFormatter{}
	default:
		return &RawURIFormatter{}
	}
}

// SingBoxFormatter formats nodes as sing-box JSON.
type SingBoxFormatter struct{}

func (f *SingBoxFormatter) ContentType() string   { return "application/json" }
func (f *SingBoxFormatter) FileExtension() string { return "json" }

func (f *SingBoxFormatter) Format(nodes []Node) ([]byte, error) {
	outbounds := make([]map[string]interface{}, 0, len(nodes)+2)

	for _, node := range nodes {
		ob := map[string]interface{}{
			"type":        node.Protocol,
			"tag":         node.Name,
			"server":      node.Host,
			"server_port": node.Port,
		}

		if node.UUID != "" {
			ob["uuid"] = node.UUID
		}
		if node.Password != "" {
			ob["password"] = node.Password
		}

		// Add flow for VLESS with XTLS Vision
		if node.Protocol == "vless" && node.Flow != "" {
			ob["flow"] = node.Flow
		}

		// Add VLESS encryption if configured
		if node.Protocol == "vless" && node.Encryption != "" && node.Encryption != "none" {
			ob["packet_encoding"] = "xudp" // sing-box uses packet_encoding for VLESS encryption
		}

		if node.TLS {
			ob["tls"] = map[string]interface{}{
				"enabled":     true,
				"server_name": node.ServerName,
			}
		}

		if node.Transport != "" && node.Transport != "tcp" {
			ob["transport"] = map[string]interface{}{
				"type": node.Transport,
			}
		}

		outbounds = append(outbounds, ob)
	}

	// Add direct and block outbounds
	outbounds = append(outbounds,
		map[string]interface{}{"type": "direct", "tag": "direct"},
		map[string]interface{}{"type": "block", "tag": "block"},
	)

	result := map[string]interface{}{
		"outbounds": outbounds,
	}

	return json.MarshalIndent(result, "", "  ")
}

// XrayFormatter formats nodes as Xray JSON.
type XrayFormatter struct{}

func (f *XrayFormatter) ContentType() string   { return "application/json" }
func (f *XrayFormatter) FileExtension() string { return "json" }

func (f *XrayFormatter) Format(nodes []Node) ([]byte, error) {
	outbounds := make([]map[string]interface{}, 0, len(nodes)+2)

	for _, node := range nodes {
		protocol := node.Protocol
		if protocol == "hysteria2" {
			protocol = "hysteria"
		}
		ob := map[string]interface{}{
			"protocol": protocol,
			"tag":      node.Name,
		}

		settings := map[string]interface{}{}
		switch node.Protocol {
		case "vless", "vmess":
			user := map[string]interface{}{
				"id": node.UUID,
			}
			// Add flow for VLESS with XTLS Vision
			if node.Protocol == "vless" && node.Flow != "" {
				user["flow"] = node.Flow
			}
			settings["vnext"] = []map[string]interface{}{{
				"address": node.Host,
				"port":    node.Port,
				"users":   []map[string]interface{}{user},
			}}
			// Add VLESS encryption if configured
			if node.Protocol == "vless" && node.Encryption != "" && node.Encryption != "none" {
				settings["encryption"] = node.Encryption
			}
		case "trojan":
			settings["servers"] = []map[string]interface{}{{
				"address":  node.Host,
				"port":     node.Port,
				"password": node.Password,
			}}
		case "hysteria2":
			settings["version"] = 2
			settings["address"] = node.Host
			settings["port"] = node.Port
		}
		ob["settings"] = settings

		streamSettings := map[string]interface{}{
			"network":  "tcp",
			"security": "none",
		}
		if node.Protocol == "hysteria2" {
			streamSettings["network"] = "hysteria"
		} else if node.Transport != "" {
			streamSettings["network"] = node.Transport
		}
		if node.TLS {
			streamSettings["security"] = "tls"
			tlsSettings := map[string]interface{}{}
			if node.ServerName != "" {
				tlsSettings["serverName"] = node.ServerName
			}
			// Copy TLS-related fields from Extra
			if fp, ok := node.Extra["fingerprint"].(string); ok && fp != "" {
				tlsSettings["fingerprint"] = fp
			}
			if alpn, ok := node.Extra["alpn"].([]string); ok && len(alpn) > 0 {
				tlsSettings["alpn"] = alpn
			}
			if insecure, ok := node.Extra["skip-cert-verify"].(bool); ok && insecure {
				tlsSettings["allowInsecure"] = true
			}
			streamSettings["tlsSettings"] = tlsSettings
		}
		// Reality
		if ro, ok := node.Extra["reality-opts"].(map[string]interface{}); ok {
			streamSettings["security"] = "reality"
			realitySettings := map[string]interface{}{}
			if pk, ok := ro["public-key"].(string); ok {
				realitySettings["serverNames"] = []string{node.ServerName}
				realitySettings["publicKey"] = pk
			}
			if sid, ok := ro["short-id"].(string); ok {
				realitySettings["shortId"] = sid
			}
			if fp, ok := node.Extra["fingerprint"].(string); ok {
				realitySettings["fingerprint"] = fp
			}
			streamSettings["realitySettings"] = realitySettings
		}
		// Transport sub-options from Extra
		if wsOpts, ok := node.Extra["ws-opts"].(map[string]interface{}); ok {
			wsSettings := map[string]interface{}{}
			if path, ok := wsOpts["path"].(string); ok {
				wsSettings["path"] = path
			}
			if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
				wsSettings["headers"] = headers
			}
			if ed, ok := wsOpts["max-early-data"].(int); ok {
				wsSettings["maxEarlyData"] = ed
			}
			streamSettings["wsSettings"] = wsSettings
		}
		if grpcOpts, ok := node.Extra["grpc-opts"].(map[string]interface{}); ok {
			grpcSettings := map[string]interface{}{}
			if sn, ok := grpcOpts["grpc-service-name"].(string); ok {
				grpcSettings["serviceName"] = sn
			}
			// grpc-mode/multiMode not supported by mihomo
			streamSettings["grpcSettings"] = grpcSettings
		}
		if xhttpOpts, ok := node.Extra["xhttp-opts"].(map[string]interface{}); ok {
			xhttpSettings := map[string]interface{}{}
			if mode, ok := xhttpOpts["mode"].(string); ok {
				xhttpSettings["mode"] = mode
			}
			if path, ok := xhttpOpts["path"].(string); ok {
				xhttpSettings["path"] = path
			}
			if host, ok := xhttpOpts["host"].(string); ok {
				xhttpSettings["host"] = host
			}
			streamSettings["xhttpSettings"] = xhttpSettings
		}
		if kcpOpts, ok := node.Extra["kcp-settings"].(map[string]interface{}); ok {
			kcpSettings := map[string]interface{}{}
			for _, key := range []string{"mtu", "tti", "uplinkCapacity", "downlinkCapacity", "congestion", "readBufferSize", "writeBufferSize", "seed"} {
				if value, exists := kcpOpts[key]; exists {
					kcpSettings[key] = value
				}
			}
			if headerType, ok := kcpOpts["headerType"].(string); ok && headerType != "" {
				kcpSettings["header"] = map[string]interface{}{"type": headerType}
			}
			if len(kcpSettings) > 0 {
				streamSettings["kcpSettings"] = kcpSettings
			}
		}
		if h2Opts, ok := node.Extra["h2-opts"].(map[string]interface{}); ok {
			h2Settings := map[string]interface{}{}
			if path, ok := h2Opts["path"].(string); ok {
				h2Settings["path"] = path
			}
			if host, ok := h2Opts["host"].([]string); ok {
				h2Settings["host"] = host
			}
			streamSettings["h2Settings"] = h2Settings
		}
		if httpOpts, ok := node.Extra["http-opts"].(map[string]interface{}); ok {
			httpSettings := map[string]interface{}{}
			if path, ok := httpOpts["path"].([]string); ok {
				httpSettings["path"] = path
			}
			if host, ok := httpOpts["host"].([]string); ok {
				httpSettings["host"] = host
			}
			if headers, ok := httpOpts["headers"].(map[string]interface{}); ok {
				httpSettings["headers"] = headers
			}
			streamSettings["tcpSettings"] = map[string]interface{}{
				"header": map[string]interface{}{
					"type":    "http",
					"request": httpSettings,
				},
			}
		}
		// Sockopt
		if tfo, ok := node.Extra["tfo"].(bool); ok && tfo {
			streamSettings["sockopt"] = map[string]interface{}{"tcpFastOpen": true}
		}
		if finalmask, ok := node.Extra["finalmask"].(map[string]interface{}); ok && len(finalmask) > 0 {
			streamSettings["finalmask"] = finalmask
		}
		if node.Protocol == "hysteria2" {
			hysteriaSettings := map[string]interface{}{"version": 2}
			if node.Password != "" {
				hysteriaSettings["auth"] = node.Password
			}
			if obfs, ok := node.Extra["obfs"].(string); ok && obfs != "" {
				hysteriaSettings["obfs"] = obfs
			}
			if obfsPassword, ok := node.Extra["obfs-password"].(string); ok && obfsPassword != "" {
				hysteriaSettings["obfsPassword"] = obfsPassword
			}
			if up, ok := node.Extra["up"].(string); ok && up != "" {
				hysteriaSettings["up"] = up
			}
			if down, ok := node.Extra["down"].(string); ok && down != "" {
				hysteriaSettings["down"] = down
			}
			if ports, ok := node.Extra["ports"].(string); ok && ports != "" {
				hysteriaSettings["hopPorts"] = ports
			}
			if hopInterval, ok := node.Extra["hop-interval"].(string); ok && hopInterval != "" {
				hysteriaSettings["hopInterval"] = hopInterval
			}
			streamSettings["hysteriaSettings"] = hysteriaSettings
		}
		ob["streamSettings"] = streamSettings

		outbounds = append(outbounds, ob)
	}

	outbounds = append(outbounds,
		map[string]interface{}{"protocol": "freedom", "tag": "direct"},
		map[string]interface{}{"protocol": "blackhole", "tag": "block"},
	)

	result := map[string]interface{}{"outbounds": outbounds}
	return json.MarshalIndent(result, "", "  ")
}

// MihomoFormatter formats nodes as Mihomo YAML.
type MihomoFormatter struct{}

func (f *MihomoFormatter) ContentType() string   { return "text/yaml" }
func (f *MihomoFormatter) FileExtension() string { return "yaml" }

func (f *MihomoFormatter) Format(nodes []Node) ([]byte, error) {
	yaml := "proxies:\n"
	for _, node := range nodes {
		yaml += fmt.Sprintf("  - name: %s\n", node.Name)
		proto := node.Protocol
		if proto == "hysteria" {
			proto = "hysteria2"
		}
		yaml += fmt.Sprintf("    type: %s\n", proto)
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		// Resolve port: when port=0 (port hopping), extract first port from range
		port := node.Port
		if port == 0 {
			if portsStr, ok := node.Extra["ports"].(string); ok && portsStr != "" {
				port = extractFirstPort(portsStr)
			}
		}
		if port == 0 {
			port = 443
		}
		yaml += fmt.Sprintf("    port: %d\n", port)
		if node.UUID != "" {
			yaml += fmt.Sprintf("    uuid: %s\n", node.UUID)
		}
		if node.Password != "" {
			yaml += fmt.Sprintf("    password: %s\n", node.Password)
		}
		if node.Protocol == "vless" && node.Flow != "" {
			yaml += fmt.Sprintf("    flow: %s\n", node.Flow)
		}
		if node.Encryption != "" && node.Encryption != "none" {
			yaml += fmt.Sprintf("    encryption: %s\n", node.Encryption)
		}
		if node.TLS {
			yaml += "    tls: true\n"
			if node.ServerName != "" {
				// trojan/hysteria2/tuic use "sni", vmess/vless use "servername"
				if node.Protocol == "trojan" || node.Protocol == "hysteria2" || node.Protocol == "tuic" {
					yaml += fmt.Sprintf("    sni: %s\n", node.ServerName)
				} else {
					yaml += fmt.Sprintf("    servername: %s\n", node.ServerName)
				}
			}
		}
		transport := node.Transport
		if (transport == "tcp" || transport == "raw" || transport == "") && node.Extra != nil {
			if _, ok := node.Extra["http-opts"]; ok {
				transport = "http"
			}
		}
		if transport != "" && transport != "raw" && transport != "tcp" && transport != "kcp" && transport != "mkcp" && transport != "quic" {
			yaml += fmt.Sprintf("    network: %s\n", transport)
		}
		// Render all Extra fields
		yaml += renderExtra(node.Extra, 4)
	}
	return []byte(yaml), nil
}

// renderExtra renders Extra map fields as YAML lines with given indent.
func renderExtra(extra map[string]interface{}, indent int) string {
	if len(extra) == 0 {
		return ""
	}
	prefix := strings.Repeat(" ", indent)
	var result string
	// Render in deterministic key order
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	// Simple sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		v := extra[k]
		switch val := v.(type) {
		case map[string]interface{}:
			if len(val) == 0 {
				continue
			}
			result += prefix + k + ":\n"
			result += renderMap(val, indent+2)
		case []string:
			result += prefix + k + ":\n"
			for _, s := range val {
				result += prefix + "  - " + s + "\n"
			}
		case []interface{}:
			result += prefix + k + ":\n"
			for _, item := range val {
				result += prefix + "  - " + fmt.Sprintf("%v", item) + "\n"
			}
		case bool:
			if val {
				result += prefix + k + ": true\n"
			}
		case int:
			result += prefix + k + ": " + fmt.Sprintf("%d", val) + "\n"
		case float64:
			result += prefix + k + ": " + fmt.Sprintf("%v", val) + "\n"
		case string:
			if val != "" {
				result += prefix + k + ": " + val + "\n"
			}
		}
	}
	return result
}

func renderMap(m map[string]interface{}, indent int) string {
	prefix := strings.Repeat(" ", indent)
	var result string
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		v := m[k]
		switch val := v.(type) {
		case map[string]interface{}:
			result += prefix + k + ":\n"
			result += renderMap(val, indent+2)
		case []string:
			result += prefix + k + ":\n"
			for _, s := range val {
				result += prefix + "  - " + s + "\n"
			}
		case []interface{}:
			result += prefix + k + ":\n"
			for _, item := range val {
				result += prefix + "  - " + fmt.Sprintf("%v", item) + "\n"
			}
		case bool:
			if val {
				result += prefix + k + ": true\n"
			}
		case int:
			result += prefix + k + ": " + fmt.Sprintf("%d", val) + "\n"
		case float64:
			result += prefix + k + ": " + fmt.Sprintf("%v", val) + "\n"
		case string:
			if val != "" {
				result += prefix + k + ": " + val + "\n"
			}
		}
	}
	return result
}

// RawURIFormatter formats nodes as raw URI list.
type RawURIFormatter struct{}

func (f *RawURIFormatter) ContentType() string   { return "text/plain" }
func (f *RawURIFormatter) FileExtension() string { return "txt" }

func (f *RawURIFormatter) Format(nodes []Node) ([]byte, error) {
	var lines []string
	for _, node := range nodes {
		uri := formatURI(node)
		lines = append(lines, uri)
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// extractFirstPort extracts the first port number from a port range string.
// Supports formats: "20000-50000", "20000,30000,40000", "443".
func extractFirstPort(portsStr string) int {
	portsStr = strings.TrimSpace(portsStr)
	if portsStr == "" {
		return 0
	}
	// Try range format "20000-50000"
	if idx := strings.Index(portsStr, "-"); idx > 0 {
		first := strings.TrimSpace(portsStr[:idx])
		if p, err := strconv.Atoi(first); err == nil && p > 0 {
			return p
		}
	}
	// Try comma-separated "20000,30000"
	if idx := strings.Index(portsStr, ","); idx > 0 {
		first := strings.TrimSpace(portsStr[:idx])
		if p, err := strconv.Atoi(first); err == nil && p > 0 {
			return p
		}
	}
	// Single port
	if p, err := strconv.Atoi(portsStr); err == nil && p > 0 {
		return p
	}
	return 0
}

// serializeFinalMaskJSON converts a finalmask map to compact JSON string.
func serializeFinalMaskJSON(fm map[string]interface{}) string {
	if len(fm) == 0 {
		return ""
	}
	data := make(map[string]interface{})
	// TCP masks
	if tcp, ok := fm["tcp"]; ok {
		data["tcp"] = tcp
	}
	// UDP masks
	if udp, ok := fm["udp"]; ok {
		data["udp"] = udp
	}
	// QUIC params
	if qp, ok := fm["quicParams"]; ok {
		data["quicParams"] = qp
	}
	if len(data) == 0 {
		return ""
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// buildFinalMaskFromMihomo reconstructs the fm (finalmask) JSON from
// mihomo-style flat keys stored in Extra.
// Mapping: mihomo flat key → finalmask JSON path
//   up                          → quicParams.brutalUp
//   down                        → quicParams.brutalDown
//   ports                       → quicParams.udpHop.ports
//   hop-interval                → quicParams.udpHop.interval
//   bbr-profile                 → quicParams.bbrProfile
//   obfs + obfs-password        → udp[0].type + udp[0].settings.password
//   initial-stream-receive-window    → quicParams.initStreamReceiveWindow
//   max-stream-receive-window        → quicParams.maxStreamReceiveWindow
//   initial-connection-receive-window → quicParams.initConnectionReceiveWindow
//   max-connection-receive-window    → quicParams.maxConnectionReceiveWindow
//   max-idle-timeout                → quicParams.maxIdleTimeout
//   keep-alive-period               → quicParams.keepAlivePeriod
//   disable-path-mtu-discovery      → quicParams.disablePathMTUDiscovery
//   max-incoming-streams            → quicParams.maxIncomingStreams
func buildFinalMaskFromMihomo(extra map[string]interface{}) string {
	if len(extra) == 0 {
		return ""
	}

	data := make(map[string]interface{})
	quicParams := make(map[string]interface{})
	hasQP := false

	// bbr-profile → quicParams.bbrProfile
	if v, ok := extra["bbr-profile"].(string); ok && v != "" {
		quicParams["bbrProfile"] = v
		hasQP = true
	}

	// up → quicParams.brutalUp
	if v, ok := extra["up"].(string); ok && v != "" {
		quicParams["brutalUp"] = v
		hasQP = true
	}

	// down → quicParams.brutalDown
	if v, ok := extra["down"].(string); ok && v != "" {
		quicParams["brutalDown"] = v
		hasQP = true
	}

	// QUIC window settings
	if v := extraToInt(extra, "initial-stream-receive-window"); v > 0 {
		quicParams["initStreamReceiveWindow"] = v
		hasQP = true
	}
	if v := extraToInt(extra, "max-stream-receive-window"); v > 0 {
		quicParams["maxStreamReceiveWindow"] = v
		hasQP = true
	}
	if v := extraToInt(extra, "initial-connection-receive-window"); v > 0 {
		quicParams["initConnectionReceiveWindow"] = v
		hasQP = true
	}
	if v := extraToInt(extra, "max-connection-receive-window"); v > 0 {
		quicParams["maxConnectionReceiveWindow"] = v
		hasQP = true
	}
	if v := extraToInt(extra, "max-idle-timeout"); v > 0 {
		quicParams["maxIdleTimeout"] = v
		hasQP = true
	}
	if v := extraToInt(extra, "keep-alive-period"); v > 0 {
		quicParams["keepAlivePeriod"] = v
		hasQP = true
	}
	if v := extraToInt(extra, "max-incoming-streams"); v > 0 {
		quicParams["maxIncomingStreams"] = v
		hasQP = true
	}
	if v, ok := extra["disable-path-mtu-discovery"].(bool); ok && v {
		quicParams["disablePathMTUDiscovery"] = true
		hasQP = true
	}

	// udpHop: ports + interval
	udpHop := make(map[string]interface{})
	hasHop := false
	if v, ok := extra["ports"].(string); ok && v != "" {
		udpHop["ports"] = v
		hasHop = true
	}
	if v, ok := extra["hop-interval"].(string); ok && v != "" {
		udpHop["interval"] = v
		hasHop = true
	}
	if hasHop {
		quicParams["udpHop"] = udpHop
		hasQP = true
	}

	if hasQP {
		data["quicParams"] = quicParams
	}

	// udp: obfs + obfs-password → salamander entry
	if obfs, ok := extra["obfs"].(string); ok && obfs != "" {
		udpEntry := map[string]interface{}{
			"type": obfs,
		}
		if obfsPwd, ok := extra["obfs-password"].(string); ok && obfsPwd != "" {
			udpEntry["settings"] = map[string]interface{}{
				"password": obfsPwd,
			}
		}
		data["udp"] = []interface{}{udpEntry}
	}

	if len(data) == 0 {
		return ""
	}

	b, _ := json.Marshal(data)
	return string(b)
}

// extraToInt extracts an int value from Extra, handling int, float64, and string types.
func extraToInt(extra map[string]interface{}, key string) int {
	switch v := extra[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// FormatNodeURI exports the URI formatting for a single node.
func FormatNodeURI(node Node) string {
	return formatURI(node)
}

// URIToMihomo converts a xray-style URI directly to mihomo proxy YAML lines.
func URIToMihomo(uri string) string {
	node := ParseURIToNode(uri)
	if node == nil {
		return ""
	}
	return NodeToMihomoProxy(*node)
}

// MihomoToURI converts a mihomo proxy YAML map to a xray-style URI.
func MihomoToURI(proxy map[string]interface{}) string {
	node := mihomoProxyToNode(proxy)
	if node == nil {
		return ""
	}
	return formatURI(*node)
}

// NodeToMihomoProxy renders a single node as mihomo proxy YAML lines.
func NodeToMihomoProxy(node Node) string {
	var b strings.Builder
	b.WriteString("  - name: " + node.Name + "\n")
	proto := node.Protocol
	if proto == "hysteria" {
		proto = "hysteria2"
	}
	b.WriteString("    type: " + proto + "\n")
	b.WriteString("    server: " + node.Host + "\n")
	// Resolve port: when port=0 (port hopping), extract first port from range
	port := node.Port
	if port == 0 {
		if portsStr, ok := node.Extra["ports"].(string); ok && portsStr != "" {
			port = extractFirstPort(portsStr)
		}
	}
	if port == 0 {
		port = 443
	}
	b.WriteString(fmt.Sprintf("    port: %d\n", port))
	if node.UUID != "" {
		b.WriteString("    uuid: " + node.UUID + "\n")
	}
	if node.Password != "" {
		b.WriteString("    password: " + node.Password + "\n")
	}
	if node.Protocol == "vless" && node.Flow != "" {
		b.WriteString("    flow: " + node.Flow + "\n")
	}
	if node.Encryption != "" && node.Encryption != "none" {
		b.WriteString("    encryption: " + node.Encryption + "\n")
	}
	if node.TLS {
		b.WriteString("    tls: true\n")
		if node.ServerName != "" {
			if proto == "trojan" || proto == "hysteria2" || proto == "tuic" {
				b.WriteString("    sni: " + node.ServerName + "\n")
			} else {
				b.WriteString("    servername: " + node.ServerName + "\n")
			}
		}
	}
	transport := node.Transport
	// TCP with http-opts → mihomo uses network: http
	if transport == "tcp" || transport == "raw" || transport == "" {
		if _, ok := node.Extra["http-opts"]; ok {
			transport = "http"
		}
	}
	if transport != "" && transport != "raw" && transport != "tcp" && transport != "kcp" && transport != "mkcp" && transport != "quic" {
		b.WriteString("    network: " + transport + "\n")
	}
	b.WriteString(renderExtra(node.Extra, 4))
	return b.String()
}

// mihomoProxyToNode converts a mihomo proxy map back to a Node.
func mihomoProxyToNode(p map[string]interface{}) *Node {
	name, _ := p["name"].(string)
	proto, _ := p["type"].(string)
	server, _ := p["server"].(string)
	port := 0
	if v, ok := p["port"].(float64); ok {
		port = int(v)
	} else if v, ok := p["port"].(int); ok {
		port = v
	}
	uuid, _ := p["uuid"].(string)
	password, _ := p["password"].(string)
	flow, _ := p["flow"].(string)
	encryption, _ := p["encryption"].(string)

	// TLS
	tls := false
	if v, ok := p["tls"].(bool); ok {
		tls = v
	}
	sni := ""
	if v, ok := p["sni"].(string); ok && v != "" {
		sni = v
	} else if v, ok := p["servername"].(string); ok && v != "" {
		sni = v
	}

	// Transport
	transport := "tcp"
	if v, ok := p["network"].(string); ok && v != "" {
		transport = v
	}

	// Build Extra from remaining fields
	extra := make(map[string]interface{})
	copyKeys := []string{
		"alterId", "cipher", "alpn", "fingerprint", "client-fingerprint",
		"skip-cert-verify", "tfo", "mptcp", "routing-mark", "udp", "xudp",
		"reality-opts", "ech-opts", "support-x25519mlkem768",
		"ws-opts", "ws-headers", "grpc-opts", "xhttp-opts", "h2-opts", "http-opts",
		"obfs", "obfs-password", "up", "down", "ports", "hop-interval",
		"bbr-profile", "max-idle-timeout", "keep-alive-period",
		"disable-path-mtu-discovery", "max-incoming-streams",
		"initial-stream-receive-window", "max-stream-receive-window",
		"initial-connection-receive-window", "max-connection-receive-window",
		"realm-opts", "packet-addr", "packet-encoding",
		"plugin", "plugin-opts", "udp-over-tcp", "udp-over-tcp-version",
		// TUIC specific
		"token", "congestion-controller", "disable-sni", "udp-relay-mode",
		// Hysteria v1 specific
		"auth_str", "protocol", "auth-str",
		// Socks5/HTTP specific
		"username",
		// SS specific
		"obfs-min-packet-size", "obfs-max-packet-size",
	}
	for _, k := range copyKeys {
		if v, ok := p[k]; ok {
			extra[k] = v
		}
	}
	if len(extra) == 0 {
		extra = nil
	}

	if proto == "hysteria" {
		proto = "hysteria2"
	}

	return &Node{
		Name:       name,
		Protocol:   proto,
		Host:       server,
		Port:       port,
		Transport:  transport,
		TLS:        tls,
		ServerName: sni,
		UUID:       uuid,
		Password:   password,
		Flow:       flow,
		Encryption: encryption,
		Extra:      extra,
	}
}

func formatURI(node Node) string {
	switch node.Protocol {
	case "vmess":
		// vmess://base64(json)
		config := map[string]interface{}{
			"v":    "2",
			"ps":   node.Name,
			"add":  node.Host,
			"port": node.Port,
			"id":   node.UUID,
			"aid":  node.AlterID,
			"net":  node.Transport,
			"type": "none",
			"host": "",
			"path": "/",
			"tls":  "",
			"sni":  "",
			"alpn": "",
			"fp":   "",
		}
		if node.TLS {
			config["tls"] = "tls"
		}
		if node.ServerName != "" {
			config["sni"] = node.ServerName
		}
		if scy, ok := node.Extra["cipher"].(string); ok && scy != "" {
			config["scy"] = scy
		}
		if fp, ok := node.Extra["client-fingerprint"].(string); ok && fp != "" {
			config["fp"] = fp
		} else if fp, ok := node.Extra["fingerprint"].(string); ok && fp != "" {
			config["fp"] = fp
		}
		if alpnStr := extraALPN(node.Extra); alpnStr != "" {
			config["alpn"] = alpnStr
		}
		// Transport sub-options from Extra
		if wsOpts, ok := node.Extra["ws-opts"].(map[string]interface{}); ok {
			if path, ok := wsOpts["path"].(string); ok && path != "" {
				config["path"] = path
			}
			if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok {
					config["host"] = host
				}
			}
		}
		if grpcOpts, ok := node.Extra["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
				config["path"] = sn
			}
		}
		if h2Opts, ok := node.Extra["h2-opts"].(map[string]interface{}); ok {
			if path, ok := h2Opts["path"].(string); ok && path != "" {
				config["path"] = path
			}
			if hosts, ok := h2Opts["host"].([]string); ok && len(hosts) > 0 {
				config["host"] = hosts[0]
			} else if hosts, ok := h2Opts["host"].([]interface{}); ok && len(hosts) > 0 {
				if h, ok := hosts[0].(string); ok {
					config["host"] = h
				}
			}
		}
		if httpOpts, ok := node.Extra["http-opts"].(map[string]interface{}); ok {
			if method, ok := httpOpts["method"].(string); ok && method != "" {
				config["type"] = method
			}
			if paths, ok := httpOpts["path"].([]string); ok && len(paths) > 0 {
				config["path"] = paths[0]
			} else if paths, ok := httpOpts["path"].([]interface{}); ok && len(paths) > 0 {
				if p, ok := paths[0].(string); ok {
					config["path"] = p
				}
			} else if path, ok := httpOpts["path"].(string); ok && path != "" {
				config["path"] = path
			}
			if headers, ok := httpOpts["headers"].(map[string]interface{}); ok {
				if hostVal, ok := headers["Host"]; ok {
					switch h := hostVal.(type) {
					case string:
						if h != "" {
							config["host"] = h
						}
					case []string:
						if len(h) > 0 {
							config["host"] = h[0]
						}
					case []interface{}:
						if len(h) > 0 {
							if s, ok := h[0].(string); ok {
								config["host"] = s
							}
						}
					}
				}
			}
		}
		data, _ := json.Marshal(config)
		return "vmess://" + base64.StdEncoding.EncodeToString(data)

	case "vless":
		uri := fmt.Sprintf("vless://%s@%s:%d", node.UUID, node.Host, node.Port)
		params := url.Values{}
		if node.Transport != "" && node.Transport != "tcp" {
			params.Set("type", node.Transport)
		}
		if node.Encryption != "" && node.Encryption != "none" {
			params.Set("encryption", node.Encryption)
		}
		if node.Flow != "" {
			params.Set("flow", node.Flow)
		}
		// TLS / Reality
		if ro, ok := node.Extra["reality-opts"].(map[string]interface{}); ok {
			params.Set("security", "reality")
			if pk, ok := ro["public-key"].(string); ok && pk != "" {
				params.Set("pbk", pk)
			}
			if sid, ok := ro["short-id"].(string); ok && sid != "" {
				params.Set("sid", sid)
			}
		} else if node.TLS {
			params.Set("security", "tls")
		} else {
			params.Set("security", "none")
		}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		if fp, ok := node.Extra["client-fingerprint"].(string); ok && fp != "" {
			params.Set("fp", fp)
		} else if fp, ok := node.Extra["fingerprint"].(string); ok && fp != "" {
			params.Set("fp", fp)
		}
		if alpnStr := extraALPN(node.Extra); alpnStr != "" {
			params.Set("alpn", alpnStr)
		}
		if insecure, ok := node.Extra["skip-cert-verify"].(bool); ok && insecure {
			params.Set("allowInsecure", "1")
		}
		// Transport sub-options
		if wsOpts, ok := node.Extra["ws-opts"].(map[string]interface{}); ok {
			if path, ok := wsOpts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok && host != "" {
					params.Set("host", host)
				}
			}
			if ed, ok := wsOpts["max-early-data"].(int); ok && ed > 0 {
				params.Set("ed", strconv.Itoa(ed))
			} else if ed, ok := wsOpts["max-early-data"].(float64); ok && ed > 0 {
				params.Set("ed", strconv.Itoa(int(ed)))
			}
		}
		if grpcOpts, ok := node.Extra["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
				params.Set("serviceName", sn)
			}
		}
		if xhttpOpts, ok := node.Extra["xhttp-opts"].(map[string]interface{}); ok {
			if path, ok := xhttpOpts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if host, ok := xhttpOpts["host"].(string); ok && host != "" {
				params.Set("host", host)
			}
			if mode, ok := xhttpOpts["mode"].(string); ok && mode != "" {
				params.Set("mode", mode)
			}
		}
		if h2Opts, ok := node.Extra["h2-opts"].(map[string]interface{}); ok {
			if path, ok := h2Opts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if hosts, ok := h2Opts["host"].([]string); ok && len(hosts) > 0 {
				params.Set("host", hosts[0])
			} else if hosts, ok := h2Opts["host"].([]interface{}); ok && len(hosts) > 0 {
				if h, ok := hosts[0].(string); ok {
					params.Set("host", h)
				}
			}
		}
		if httpOpts, ok := node.Extra["http-opts"].(map[string]interface{}); ok {
			if method, ok := httpOpts["method"].(string); ok && method != "" {
				params.Set("method", method)
			}
			if paths, ok := httpOpts["path"].([]string); ok && len(paths) > 0 {
				params.Set("path", paths[0])
			} else if paths, ok := httpOpts["path"].([]interface{}); ok && len(paths) > 0 {
				if p, ok := paths[0].(string); ok {
					params.Set("path", p)
				}
			} else if path, ok := httpOpts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if headers, ok := httpOpts["headers"].(map[string]interface{}); ok {
				if hostVal, ok := headers["Host"]; ok {
					switch h := hostVal.(type) {
					case string:
						if h != "" {
							params.Set("host", h)
						}
					case []string:
						if len(h) > 0 {
							params.Set("host", h[0])
						}
					case []interface{}:
						if len(h) > 0 {
							if s, ok := h[0].(string); ok {
								params.Set("host", s)
							}
						}
					}
				}
			}
		}
		if fm, ok := node.Extra["finalmask"].(map[string]interface{}); ok && len(fm) > 0 {
			if fmJSON := serializeFinalMaskJSON(fm); fmJSON != "" {
				params.Set("fm", fmJSON)
			}
		}
		encoded := strings.ReplaceAll(params.Encode(), "+", "%20")
		if encoded != "" {
			uri += "?" + encoded
		}
		uri += "#" + url.PathEscape(node.Name)
		return uri

	case "trojan":
		uri := fmt.Sprintf("trojan://%s@%s:%d", url.QueryEscape(node.Password), node.Host, node.Port)
		params := url.Values{}
		if node.Transport != "" && node.Transport != "tcp" {
			params.Set("type", node.Transport)
		}
		// TLS / Reality
		if ro, ok := node.Extra["reality-opts"].(map[string]interface{}); ok {
			params.Set("security", "reality")
			if pk, ok := ro["public-key"].(string); ok && pk != "" {
				params.Set("pbk", pk)
			}
			if sid, ok := ro["short-id"].(string); ok && sid != "" {
				params.Set("sid", sid)
			}
		} else if node.TLS {
			params.Set("security", "tls")
		} else {
			params.Set("security", "none")
		}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		if fp, ok := node.Extra["client-fingerprint"].(string); ok && fp != "" {
			params.Set("fp", fp)
		} else if fp, ok := node.Extra["fingerprint"].(string); ok && fp != "" {
			params.Set("fp", fp)
		}
		if alpnStr := extraALPN(node.Extra); alpnStr != "" {
			params.Set("alpn", alpnStr)
		}
		if insecure, ok := node.Extra["skip-cert-verify"].(bool); ok && insecure {
			params.Set("allowInsecure", "1")
		}
		// Transport sub-options
		if grpcOpts, ok := node.Extra["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
				params.Set("serviceName", sn)
			}
		}
		if wsOpts, ok := node.Extra["ws-opts"].(map[string]interface{}); ok {
			if path, ok := wsOpts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok && host != "" {
					params.Set("host", host)
				}
			}
		}
		if xhttpOpts, ok := node.Extra["xhttp-opts"].(map[string]interface{}); ok {
			if path, ok := xhttpOpts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if host, ok := xhttpOpts["host"].(string); ok && host != "" {
				params.Set("host", host)
			}
			if mode, ok := xhttpOpts["mode"].(string); ok && mode != "" {
				params.Set("mode", mode)
			}
		}
		if h2Opts, ok := node.Extra["h2-opts"].(map[string]interface{}); ok {
			if path, ok := h2Opts["path"].(string); ok && path != "" {
				params.Set("path", path)
			}
			if hosts, ok := h2Opts["host"].([]string); ok && len(hosts) > 0 {
				params.Set("host", hosts[0])
			} else if hosts, ok := h2Opts["host"].([]interface{}); ok && len(hosts) > 0 {
				if h, ok := hosts[0].(string); ok {
					params.Set("host", h)
				}
			}
		}
		encoded := strings.ReplaceAll(params.Encode(), "+", "%20")
		if encoded != "" {
			uri += "?" + encoded
		}
		uri += "#" + url.PathEscape(node.Name)
		return uri

	case "tuic":
		// tuic://uuid:password@host:port?params#name (v5) or tuic://token@host:port (legacy)
		var userInfo string
		if node.UUID != "" {
			if node.Password != "" {
				userInfo = node.UUID + ":" + node.Password
			} else {
				userInfo = node.UUID
			}
		} else if token, ok := node.Extra["token"].(string); ok && token != "" {
			userInfo = token
		}
		uri := fmt.Sprintf("tuic://%s@%s:%d", url.QueryEscape(userInfo), node.Host, node.Port)
		params := url.Values{}
		if node.UUID != "" {
			params.Set("uuid", node.UUID)
		}
		if node.Password != "" {
			params.Set("password", node.Password)
		}
		if cc, ok := node.Extra["congestion-controller"].(string); ok && cc != "" {
			params.Set("congestion_control", cc)
		}
		if alpnStr := extraALPN(node.Extra); alpnStr != "" {
			params.Set("alpn", alpnStr)
		}
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}
		if v, ok := node.Extra["disable-sni"].(bool); ok && v {
			params.Set("disable_sni", "1")
		}
		if v, ok := node.Extra["udp-relay-mode"].(string); ok && v != "" {
			params.Set("udp_relay_mode", v)
		}
		if insecure, ok := node.Extra["skip-cert-verify"].(bool); ok && insecure {
			params.Set("allowInsecure", "1")
		}
		encoded := strings.ReplaceAll(params.Encode(), "+", "%20")
		if encoded != "" {
			uri += "?" + encoded
		}
		uri += "#" + url.PathEscape(node.Name)
		return uri

	case "shadowsocks", "shadowsocks2022", "ss":
		// ss://base64(method:password)@host:port#name
		method := node.Encryption
		if method == "" {
			if c, ok := node.Extra["cipher"].(string); ok {
				method = c
			}
		}
		userinfo := method + ":" + node.Password
		uri := fmt.Sprintf("ss://%s@%s:%d", base64.StdEncoding.EncodeToString([]byte(userinfo)), node.Host, node.Port)
		uri += "#" + url.PathEscape(node.Name)
		return uri

	case "hysteria2":
		// Resolve port: when port=0 (port hopping), extract first port from range
		port := node.Port
		portsStr := ""
		if v, ok := node.Extra["ports"].(string); ok && v != "" {
			portsStr = v
		}
		if port == 0 && portsStr != "" {
			port = extractFirstPort(portsStr)
		}
		if port == 0 {
			port = 443 // fallback
		}

		params := url.Values{}

		// TLS (hysteria2 always uses TLS)
		params.Set("security", "tls")
		if node.ServerName != "" {
			params.Set("sni", node.ServerName)
		}

		// Fingerprint
		if fp, ok := node.Extra["client-fingerprint"].(string); ok && fp != "" {
			params.Set("fp", fp)
		} else if fp, ok := node.Extra["fingerprint"].(string); ok && fp != "" {
			params.Set("fp", fp)
		}

		// ALPN
		if alpn, ok := node.Extra["alpn"].([]string); ok && len(alpn) > 0 {
			params.Set("alpn", strings.Join(alpn, ","))
		} else if alpn, ok := node.Extra["alpn"].([]interface{}); ok && len(alpn) > 0 {
			strs := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s, ok := a.(string); ok {
					strs = append(strs, s)
				}
			}
			if len(strs) > 0 {
				params.Set("alpn", strings.Join(strs, ","))
			}
		} else if alpn, ok := node.Extra["alpn"].(string); ok && alpn != "" {
			params.Set("alpn", alpn)
		}

		// Obfs
		if obfs, ok := node.Extra["obfs"].(string); ok && obfs != "" {
			params.Set("obfs", obfs)
		}
		if obfsPwd, ok := node.Extra["obfs-password"].(string); ok && obfsPwd != "" {
			params.Set("obfs-password", obfsPwd)
		}

		// Port hopping range
		if portsStr != "" {
			params.Set("mport", portsStr)
		}

		// Build fm (finalmask) parameter
		// If finalmask already exists in Extra (from parsed URI), use it directly
		// Otherwise, reconstruct from mihomo-style flat keys
		if fm, ok := node.Extra["finalmask"].(map[string]interface{}); ok && len(fm) > 0 {
			if fmJSON := serializeFinalMaskJSON(fm); fmJSON != "" {
				params.Set("fm", fmJSON)
			}
		} else {
			if fmJSON := buildFinalMaskFromMihomo(node.Extra); fmJSON != "" {
				params.Set("fm", fmJSON)
			}
		}

		// Encode params (sorted alphabetically, %20 for spaces)
		encoded := strings.ReplaceAll(params.Encode(), "+", "%20")

		uri := fmt.Sprintf("hysteria2://%s@%s:%d", url.QueryEscape(node.Password), node.Host, port)
		if encoded != "" {
			uri += "?" + encoded
		}
		uri += "#" + url.PathEscape(node.Name)
		return uri

	default:
		return fmt.Sprintf("%s://%s:%d#%s", node.Protocol, node.Host, node.Port, node.Name)
	}
}

// ParseURIToNode parses a xray-style URI (vless://, vmess://, trojan://, hysteria2://)
// back into a Node with all transport parameters in Extra.
func ParseURIToNode(uri string) *Node {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil
	}

	// Extract fragment (#name)
	name := ""
	if idx := strings.LastIndex(uri, "#"); idx >= 0 {
		name = uri[idx+1:]
		uri = uri[:idx]
	}

	// Split scheme
	parts := strings.SplitN(uri, "://", 2)
	if len(parts) != 2 {
		return nil
	}
	scheme := parts[0]
	body := parts[1]

	switch scheme {
	case "vmess":
		return parseVmessURI(body, name)
	case "vless":
		return parseVlessURI(body, name)
	case "trojan":
		return parseTrojanURI(body, name)
	case "hysteria2":
		return parseHysteria2URI(body, name)
	case "tuic":
		return parseTuicURI(body, name)
	case "ss":
		return parseSSURI(body, name)
	}
	return nil
}

func parseVmessURI(body, name string) *Node {
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(body)
		if err != nil {
			return nil
		}
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	host, _ := cfg["add"].(string)
	port := 0
	if p, ok := cfg["port"].(float64); ok {
		port = int(p)
	}
	uuid, _ := cfg["id"].(string)
	aid := 0
	if a, ok := cfg["aid"].(float64); ok {
		aid = int(a)
	}
	net, _ := cfg["net"].(string)
	tls, _ := cfg["tls"].(string)
	sni, _ := cfg["sni"].(string)
	if ps, ok := cfg["ps"].(string); ok && name == "" {
		name = ps
	}

	extra := make(map[string]interface{})
	if aid > 0 {
		extra["alterId"] = aid
	}
	if cipher, ok := cfg["scy"].(string); ok && cipher != "" {
		extra["cipher"] = cipher
	}
	// Transport sub-options from vmess JSON
	if path, ok := cfg["path"].(string); ok && path != "" {
		switch net {
		case "ws":
			wsOpts := map[string]interface{}{"path": path}
			if host, ok := cfg["host"].(string); ok && host != "" {
				wsOpts["headers"] = map[string]interface{}{"Host": host}
			}
			extra["ws-opts"] = wsOpts
		case "h2":
			h2Opts := map[string]interface{}{"path": path}
			if host, ok := cfg["host"].(string); ok && host != "" {
				h2Opts["host"] = []string{host}
			}
			extra["h2-opts"] = h2Opts
		case "http", "tcp":
			if path != "" {
				httpOpts := map[string]interface{}{"path": path}
				if host, ok := cfg["host"].(string); ok && host != "" {
					httpOpts["host"] = []string{host}
				}
				extra["http-opts"] = httpOpts
			}
		}
	}
	if net == "grpc" {
		if sn, ok := cfg["path"].(string); ok && sn != "" {
			extra["grpc-opts"] = map[string]interface{}{"grpc-service-name": sn}
		}
	}
	if tls == "tls" {
		if fp, ok := cfg["fp"].(string); ok && fp != "" {
			extra["fingerprint"] = fp
		}
		if alpn, ok := cfg["alpn"].(string); ok && alpn != "" {
			extra["alpn"] = strings.Split(alpn, ",")
		}
	}
	if tfo, ok := cfg["tfo"].(bool); ok && tfo {
		extra["tfo"] = true
	}

	return &Node{
		Name:       name,
		Protocol:   "vmess",
		Host:       host,
		Port:       port,
		Transport:  net,
		TLS:        tls == "tls",
		ServerName: sni,
		UUID:       uuid,
		AlterID:    aid,
		Extra:      extra,
	}
}

func parseVlessURI(body, name string) *Node {
	// vless://uuid@host:port?params#name
	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	// body = uuid@host:port
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return nil
	}
	uuid := body[:atIdx]
	hostPort := body[atIdx+1:]
	host, portStr, _ := netSplitHostPort(hostPort)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	params := parseQueryParams(query)
	transport := params["type"]
	if transport == "" || transport == "raw" {
		transport = "tcp"
	}
	security := params["security"]
	tls := security == "tls" || security == "reality"
	sni := params["sni"]
	flow := params["flow"]
	encryption := params["encryption"]

	extra := make(map[string]interface{})
	if fp := params["fp"]; fp != "" {
		extra["fingerprint"] = fp
	}
	if alpn := params["alpn"]; alpn != "" {
		extra["alpn"] = strings.Split(alpn, ",")
	}
	if pbk := params["pbk"]; pbk != "" {
		ro := map[string]interface{}{"public-key": pbk}
		if sid := params["sid"]; sid != "" {
			ro["short-id"] = sid
		}
		extra["reality-opts"] = ro
	}
	if insecure := params["allowInsecure"]; insecure == "1" || insecure == "true" {
		extra["skip-cert-verify"] = true
	}
	if finalMask := parseFinalMaskParam(params); len(finalMask) > 0 {
		extra["finalmask"] = finalMask
	}

	// Transport sub-options
	switch transport {
	case "tcp":
		// headerType=http means TCP disguised as HTTP → store as http-opts
		if headerType := params["headerType"]; headerType == "http" {
			httpOpts := make(map[string]interface{})
			if path := params["path"]; path != "" {
				httpOpts["path"] = []string{path}
			}
			if host := params["host"]; host != "" {
				httpOpts["host"] = []string{host}
			}
			if len(httpOpts) > 0 {
				extra["http-opts"] = httpOpts
			}
		}
	case "ws":
		wsOpts := make(map[string]interface{})
		if path := params["path"]; path != "" {
			wsOpts["path"] = path
		}
		if host := params["host"]; host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		if ed := params["ed"]; ed != "" {
			if v, err := strconv.Atoi(ed); err == nil {
				wsOpts["max-early-data"] = v
			}
		}
		if len(wsOpts) > 0 {
			extra["ws-opts"] = wsOpts
		}
	case "grpc":
		grpcOpts := make(map[string]interface{})
		if sn := params["serviceName"]; sn != "" {
			grpcOpts["grpc-service-name"] = sn
		}
		if mode := params["mode"]; mode != "" {
			// grpc-mode not supported by mihomo, skip
		}
		if len(grpcOpts) > 0 {
			extra["grpc-opts"] = grpcOpts
		}
	case "xhttp":
		xhttpOpts := make(map[string]interface{})
		if mode := params["mode"]; mode != "" {
			xhttpOpts["mode"] = mode
		}
		if path := params["path"]; path != "" {
			xhttpOpts["path"] = path
		}
		if host := params["host"]; host != "" {
			xhttpOpts["host"] = host
		}
		if len(xhttpOpts) > 0 {
			extra["xhttp-opts"] = xhttpOpts
		}
	case "kcp":
		kcpOpts := make(map[string]interface{})
		if headerType := params["headerType"]; headerType != "" {
			kcpOpts["headerType"] = headerType
		}
		if seed := params["seed"]; seed != "" {
			kcpOpts["seed"] = seed
		}
		if mtu := params["mtu"]; mtu != "" {
			if v, err := strconv.Atoi(mtu); err == nil {
				kcpOpts["mtu"] = v
			}
		}
		if tti := params["tti"]; tti != "" {
			if v, err := strconv.Atoi(tti); err == nil {
				kcpOpts["tti"] = v
			}
		}
		if len(kcpOpts) > 0 {
			extra["kcp-settings"] = kcpOpts
		}
	case "h2":
		h2Opts := make(map[string]interface{})
		if path := params["path"]; path != "" {
			h2Opts["path"] = path
		}
		if host := params["host"]; host != "" {
			h2Opts["host"] = []string{host}
		}
		if len(h2Opts) > 0 {
			extra["h2-opts"] = h2Opts
		}
	case "http":
		httpOpts := make(map[string]interface{})
		if path := params["path"]; path != "" {
			httpOpts["path"] = path
		}
		if host := params["host"]; host != "" {
			httpOpts["host"] = []string{host}
		}
		if len(httpOpts) > 0 {
			extra["http-opts"] = httpOpts
		}
	}
	if tfo := params["tfo"]; tfo == "1" || tfo == "true" {
		extra["tfo"] = true
	}
	extra["udp"] = true

	return &Node{
		Name:       name,
		Protocol:   "vless",
		Host:       host,
		Port:       port,
		Transport:  transport,
		TLS:        tls,
		ServerName: sni,
		UUID:       uuid,
		Flow:       flow,
		Encryption: encryption,
		Extra:      extra,
	}
}

func parseTrojanURI(body, name string) *Node {
	// trojan://password@host:port?params#name
	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return nil
	}
	password := body[:atIdx]
	hostPort := body[atIdx+1:]
	host, portStr, _ := netSplitHostPort(hostPort)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	params := parseQueryParams(query)
	transport := params["type"]
	if transport == "" || transport == "raw" {
		transport = "tcp"
	}
	sni := params["sni"]

	extra := make(map[string]interface{})
	if fp := params["fp"]; fp != "" {
		extra["fingerprint"] = fp
	}
	if alpn := params["alpn"]; alpn != "" {
		extra["alpn"] = strings.Split(alpn, ",")
	}
	if insecure := params["allowInsecure"]; insecure == "1" || insecure == "true" {
		extra["skip-cert-verify"] = true
	}
	switch transport {
	case "ws":
		wsOpts := make(map[string]interface{})
		if path := params["path"]; path != "" {
			wsOpts["path"] = path
		}
		if host := params["host"]; host != "" {
			wsOpts["headers"] = map[string]interface{}{"Host": host}
		}
		if len(wsOpts) > 0 {
			extra["ws-opts"] = wsOpts
		}
	case "grpc":
		grpcOpts := make(map[string]interface{})
		if sn := params["serviceName"]; sn != "" {
			grpcOpts["grpc-service-name"] = sn
		}
		if len(grpcOpts) > 0 {
			extra["grpc-opts"] = grpcOpts
		}
	}
	extra["udp"] = true

	return &Node{
		Name:       name,
		Protocol:   "trojan",
		Host:       host,
		Port:       port,
		Transport:  transport,
		TLS:        true,
		ServerName: sni,
		Password:   password,
		Extra:      extra,
	}
}

func parseHysteria2URI(body, name string) *Node {
	// hysteria2://password@host:port?params#name
	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return nil
	}
	password := body[:atIdx]
	if decodedPassword, err := url.QueryUnescape(password); err == nil {
		password = decodedPassword
	}
	hostPort := body[atIdx+1:]
	host, portStr, _ := netSplitHostPort(hostPort)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	params := parseQueryParams(query)
	sni := params["sni"]

	extra := make(map[string]interface{})
	if obfs := params["obfs"]; obfs != "" {
		extra["obfs"] = obfs
		if obfsPassword := params["obfs-password"]; obfsPassword != "" {
			extra["obfs-password"] = obfsPassword
		}
	}
	if insecure := params["insecure"]; insecure == "1" || insecure == "true" {
		extra["skip-cert-verify"] = true
	}
	if fp := params["fp"]; fp != "" {
		extra["fingerprint"] = fp
	}
	if alpn := params["alpn"]; alpn != "" {
		extra["alpn"] = strings.Split(alpn, ",")
	}
	if up := params["up"]; up != "" {
		extra["up"] = up
	}
	if upMbps := params["upmbps"]; upMbps != "" {
		extra["up"] = upMbps
	}
	if down := params["down"]; down != "" {
		extra["down"] = down
	}
	if downMbps := params["downmbps"]; downMbps != "" {
		extra["down"] = downMbps
	}
	if ports := params["ports"]; ports != "" {
		extra["ports"] = ports
	}
	if mport := params["mport"]; mport != "" {
		extra["ports"] = mport
	}
	if finalMask := parseFinalMaskParam(params); len(finalMask) > 0 {
		extra["finalmask"] = finalMask
		if qp, ok := finalMask["quicParams"].(map[string]interface{}); ok {
			if hop, ok := qp["udpHop"].(map[string]interface{}); ok {
				if ports, ok := hop["ports"].(string); ok && ports != "" {
					extra["ports"] = ports
				}
				if interval, ok := hop["interval"].(string); ok && interval != "" {
					extra["hop-interval"] = interval
				}
			}
		}
		if udp, ok := finalMask["udp"].([]interface{}); ok {
			for _, item := range udp {
				entry, ok := item.(map[string]interface{})
				if !ok || entry["type"] != "salamander" {
					continue
				}
				extra["obfs"] = "salamander"
				if settings, ok := entry["settings"].(map[string]interface{}); ok {
					if pwd, ok := settings["password"].(string); ok && pwd != "" {
						extra["obfs-password"] = pwd
					}
				}
			}
		}
	}
	extra["udp"] = true

	return &Node{
		Name:       name,
		Protocol:   "hysteria2",
		Host:       host,
		Port:       port,
		TLS:        true,
		ServerName: sni,
		Password:   password,
		Extra:      extra,
	}
}

// extraALPN extracts ALPN as comma-separated string from Extra, handling []string, []interface{}, and string types.
func extraALPN(extra map[string]interface{}) string {
	switch alpn := extra["alpn"].(type) {
	case []string:
		if len(alpn) > 0 {
			return strings.Join(alpn, ",")
		}
	case []interface{}:
		strs := make([]string, 0, len(alpn))
		for _, a := range alpn {
			if s, ok := a.(string); ok {
				strs = append(strs, s)
			}
		}
		if len(strs) > 0 {
			return strings.Join(strs, ",")
		}
	case string:
		if alpn != "" {
			return alpn
		}
	}
	return ""
}

func parseTuicURI(body, name string) *Node {
	// tuic://uuid:password@host:port?params#name
	query := ""
	if idx := strings.Index(body, "?"); idx >= 0 {
		query = body[idx+1:]
		body = body[:idx]
	}
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return nil
	}
	userInfo := body[:atIdx]
	if decoded, err := url.QueryUnescape(userInfo); err == nil {
		userInfo = decoded
	}
	hostPort := body[atIdx+1:]
	host, portStr, _ := netSplitHostPort(hostPort)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	params := parseQueryParams(query)
	uuid := params["uuid"]
	password := params["password"]
	if uuid == "" {
		// Legacy token format
		uuid = userInfo
	}

	extra := make(map[string]interface{})
	if cc := params["congestion_control"]; cc != "" {
		extra["congestion-controller"] = cc
	}
	if alpn := params["alpn"]; alpn != "" {
		extra["alpn"] = strings.Split(alpn, ",")
	}
	if insecure := params["allowInsecure"]; insecure == "1" || insecure == "true" {
		extra["skip-cert-verify"] = true
	}
	if v := params["disable_sni"]; v == "1" || v == "true" {
		extra["disable-sni"] = true
	}
	if v := params["udp_relay_mode"]; v != "" {
		extra["udp-relay-mode"] = v
	}
	extra["udp"] = true

	return &Node{
		Name:       name,
		Protocol:   "tuic",
		Host:       host,
		Port:       port,
		TLS:        true,
		ServerName: params["sni"],
		UUID:       uuid,
		Password:   password,
		Extra:      extra,
	}
}

func parseSSURI(body, name string) *Node {
	// ss://base64(method:password)@host:port#name
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return nil
	}
	encoded := body[:atIdx]
	hostPort := body[atIdx+1:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return nil
		}
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil
	}
	method, password := parts[0], parts[1]
	host, portStr, _ := netSplitHostPort(hostPort)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	return &Node{
		Name:       name,
		Protocol:   "shadowsocks",
		Host:       host,
		Port:       port,
		Encryption: method,
		Password:   password,
	}
}

// parseQueryParams parses URL query string into map with URL decoding.
func parseQueryParams(query string) map[string]string {
	params := make(map[string]string)
	for _, part := range strings.Split(query, "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			k, _ := url.QueryUnescape(kv[0])
			v, _ := url.QueryUnescape(kv[1])
			params[k] = v
		}
	}
	return params
}

func parseFinalMaskParam(params map[string]string) map[string]interface{} {
	fm := params["fm"]
	if fm == "" {
		return nil
	}
	var finalMask map[string]interface{}
	if err := json.Unmarshal([]byte(fm), &finalMask); err != nil || len(finalMask) == 0 {
		return nil
	}
	return finalMask
}

// netSplitHostPort splits host:port, handling IPv6 brackets.
func netSplitHostPort(hostPort string) (string, string, error) {
	if strings.HasPrefix(hostPort, "[") {
		// IPv6
		end := strings.Index(hostPort, "]")
		if end < 0 {
			return hostPort, "", nil
		}
		host := hostPort[1:end]
		port := ""
		if end+1 < len(hostPort) && hostPort[end+1] == ':' {
			port = hostPort[end+2:]
		}
		return host, port, nil
	}
	// IPv4 or hostname
	idx := strings.LastIndex(hostPort, ":")
	if idx < 0 {
		return hostPort, "", nil
	}
	return hostPort[:idx], hostPort[idx+1:], nil
}

// Base64AggregateFormatter formats nodes as base64-encoded aggregate.
type Base64AggregateFormatter struct{}

func (f *Base64AggregateFormatter) ContentType() string   { return "text/plain" }
func (f *Base64AggregateFormatter) FileExtension() string { return "txt" }

func (f *Base64AggregateFormatter) Format(nodes []Node) ([]byte, error) {
	raw := &RawURIFormatter{}
	rawBytes, err := raw.Format(nodes)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(rawBytes)
	return []byte(encoded), nil
}
