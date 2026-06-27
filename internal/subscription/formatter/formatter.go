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

func (f *SingBoxFormatter) ContentType() string  { return "application/json" }
func (f *SingBoxFormatter) FileExtension() string { return "json" }

func (f *SingBoxFormatter) Format(nodes []Node) ([]byte, error) {
	outbounds := make([]map[string]interface{}, 0, len(nodes)+2)

	for _, node := range nodes {
		ob := map[string]interface{}{
			"type":    node.Protocol,
			"tag":     node.Name,
			"server":  node.Host,
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

func (f *XrayFormatter) ContentType() string  { return "application/json" }
func (f *XrayFormatter) FileExtension() string { return "json" }

func (f *XrayFormatter) Format(nodes []Node) ([]byte, error) {
	outbounds := make([]map[string]interface{}, 0, len(nodes)+2)

	for _, node := range nodes {
		ob := map[string]interface{}{
			"protocol": node.Protocol,
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
		}
		ob["settings"] = settings

		streamSettings := map[string]interface{}{
			"network":  "tcp",
			"security": "none",
		}
		if node.Transport != "" {
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

func (f *MihomoFormatter) ContentType() string  { return "text/yaml" }
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
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
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
		if node.Transport != "" && node.Transport != "raw" && node.Transport != "tcp" && node.Transport != "kcp" && node.Transport != "mkcp" && node.Transport != "quic" {
			yaml += fmt.Sprintf("    network: %s\n", node.Transport)
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

func (f *RawURIFormatter) ContentType() string  { return "text/plain" }
func (f *RawURIFormatter) FileExtension() string { return "txt" }

func (f *RawURIFormatter) Format(nodes []Node) ([]byte, error) {
	var lines []string
	for _, node := range nodes {
		uri := formatURI(node)
		lines = append(lines, uri)
	}
	return []byte(strings.Join(lines, "\n")), nil
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
	b.WriteString(fmt.Sprintf("    port: %d\n", node.Port))
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
		"alterId", "cipher", "alpn", "fingerprint", "skip-cert-verify",
		"tfo", "mptcp", "routing-mark", "udp",
		"reality-opts", "ech-opts",
		"ws-opts", "ws-headers", "grpc-opts", "xhttp-opts", "h2-opts", "http-opts",
		"obfs", "obfs-password", "up", "down", "ports",
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
		data, _ := json.Marshal(config)
		return "vmess://" + base64.StdEncoding.EncodeToString(data)

	case "vless":
		uri := fmt.Sprintf("vless://%s@%s:%d", node.UUID, node.Host, node.Port)
		params := []string{}
		if node.Transport != "" && node.Transport != "tcp" {
			params = append(params, "type="+node.Transport)
		}
		if node.Encryption != "" && node.Encryption != "none" {
			params = append(params, "encryption="+node.Encryption)
		}
		if node.Flow != "" {
			params = append(params, "flow="+node.Flow)
		}
		// TLS / Reality
		if ro, ok := node.Extra["reality-opts"].(map[string]interface{}); ok {
			params = append(params, "security=reality")
			if pk, ok := ro["public-key"].(string); ok && pk != "" {
				params = append(params, "pbk="+pk)
			}
			if sid, ok := ro["short-id"].(string); ok && sid != "" {
				params = append(params, "sid="+sid)
			}
		} else if node.TLS {
			params = append(params, "security=tls")
		} else {
			params = append(params, "security=none")
		}
		if node.ServerName != "" {
			params = append(params, "sni="+node.ServerName)
		}
		if fp, ok := node.Extra["client-fingerprint"].(string); ok && fp != "" {
			params = append(params, "fp="+fp)
		}
		if alpn, ok := node.Extra["alpn"].([]string); ok && len(alpn) > 0 {
			params = append(params, "alpn="+strings.Join(alpn, ","))
		}
		// Transport sub-options
		if wsOpts, ok := node.Extra["ws-opts"].(map[string]interface{}); ok {
			if path, ok := wsOpts["path"].(string); ok && path != "" {
				params = append(params, "path="+path)
			}
			if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok && host != "" {
					params = append(params, "host="+host)
				}
			}
			if ed, ok := wsOpts["max-early-data"].(int); ok && ed > 0 {
				params = append(params, "ed="+strconv.Itoa(ed))
			}
		}
		if grpcOpts, ok := node.Extra["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
				params = append(params, "serviceName="+sn)
			}
		}
		if xhttpOpts, ok := node.Extra["xhttp-opts"].(map[string]interface{}); ok {
			if path, ok := xhttpOpts["path"].(string); ok && path != "" {
				params = append(params, "path="+path)
			}
			if host, ok := xhttpOpts["host"].(string); ok && host != "" {
				params = append(params, "host="+host)
			}
			if mode, ok := xhttpOpts["mode"].(string); ok && mode != "" {
				params = append(params, "mode="+mode)
			}
		}
		if len(params) > 0 {
			uri += "?" + strings.Join(params, "&")
		}
		uri += "#" + node.Name
		return uri

	case "trojan":
		uri := fmt.Sprintf("trojan://%s@%s:%d", url.QueryEscape(node.Password), node.Host, node.Port)
		params := []string{}
		if node.Transport != "" && node.Transport != "tcp" {
			params = append(params, "type="+node.Transport)
		}
		// TLS / Reality
		if ro, ok := node.Extra["reality-opts"].(map[string]interface{}); ok {
			params = append(params, "security=reality")
			if pk, ok := ro["public-key"].(string); ok && pk != "" {
				params = append(params, "pbk="+pk)
			}
			if sid, ok := ro["short-id"].(string); ok && sid != "" {
				params = append(params, "sid="+sid)
			}
		} else if node.TLS {
			params = append(params, "security=tls")
		} else {
			params = append(params, "security=none")
		}
		if node.ServerName != "" {
			params = append(params, "sni="+node.ServerName)
		}
		if fp, ok := node.Extra["client-fingerprint"].(string); ok && fp != "" {
			params = append(params, "fp="+fp)
		}
		if alpn, ok := node.Extra["alpn"].([]string); ok && len(alpn) > 0 {
			params = append(params, "alpn="+strings.Join(alpn, ","))
		}
		// Transport sub-options
		if grpcOpts, ok := node.Extra["grpc-opts"].(map[string]interface{}); ok {
			if sn, ok := grpcOpts["grpc-service-name"].(string); ok && sn != "" {
				params = append(params, "serviceName="+sn)
			}
		}
		if wsOpts, ok := node.Extra["ws-opts"].(map[string]interface{}); ok {
			if path, ok := wsOpts["path"].(string); ok && path != "" {
				params = append(params, "path="+path)
			}
			if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
				if host, ok := headers["Host"].(string); ok && host != "" {
					params = append(params, "host="+host)
				}
			}
		}
		if len(params) > 0 {
			uri += "?" + strings.Join(params, "&")
		}
		uri += "#" + node.Name
		return uri

	case "hysteria2":
		uri := fmt.Sprintf("hysteria2://%s@%s:%d", node.Password, node.Host, node.Port)
		if node.TLS {
			uri += "?security=tls"
			if node.ServerName != "" {
				uri += "&sni=" + node.ServerName
			}
		}
		uri += "#" + node.Name
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
	if down := params["down"]; down != "" {
		extra["down"] = down
	}
	if ports := params["ports"]; ports != "" {
		extra["ports"] = ports
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

func (f *Base64AggregateFormatter) ContentType() string  { return "text/plain" }
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
