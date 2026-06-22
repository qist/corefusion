package formatter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	Encryption string `json:"encryption,omitempty"` // VLESS encryption (e.g. "mlkem768x25519plus.native.0rtt.<key>")
	Flow       string `json:"flow,omitempty"`       // XTLS Vision flow
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
			streamSettings["tlsSettings"] = map[string]interface{}{
				"serverName": node.ServerName,
			}
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
		yaml += fmt.Sprintf("    type: %s\n", node.Protocol)
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
		if node.UUID != "" {
			yaml += fmt.Sprintf("    uuid: %s\n", node.UUID)
		}
		if node.Password != "" {
			yaml += fmt.Sprintf("    password: %s\n", node.Password)
		}
		// Add flow for VLESS with XTLS Vision
		if node.Protocol == "vless" && node.Flow != "" {
			yaml += fmt.Sprintf("    flow: %s\n", node.Flow)
		}
		// Add VLESS encryption if configured
		if node.Protocol == "vless" && node.Encryption != "" && node.Encryption != "none" {
			yaml += fmt.Sprintf("    encryption: %s\n", node.Encryption)
		}
		if node.TLS {
			yaml += "    tls: true\n"
			if node.ServerName != "" {
				yaml += fmt.Sprintf("    servername: %s\n", node.ServerName)
			}
		}
	}
	return []byte(yaml), nil
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
			"tls":  "tls",
		}
		if !node.TLS {
			config["tls"] = ""
		}
		data, _ := json.Marshal(config)
		return "vmess://" + base64.StdEncoding.EncodeToString(data)

	case "vless":
		uri := fmt.Sprintf("vless://%s@%s:%d", node.UUID, node.Host, node.Port)
		params := []string{}
		if node.Transport != "" && node.Transport != "tcp" {
			params = append(params, "type="+string(node.Transport))
		}
		if node.TLS {
			params = append(params, "security=tls")
			if node.ServerName != "" {
				params = append(params, "sni="+node.ServerName)
			}
		}
		// Add VLESS encryption if configured
		if node.Encryption != "" && node.Encryption != "none" {
			params = append(params, "encryption="+node.Encryption)
		}
		// Add XTLS Vision flow if configured
		if node.Flow != "" {
			params = append(params, "flow="+node.Flow)
		}
		if len(params) > 0 {
			uri += "?" + strings.Join(params, "&")
		}
		uri += "#" + node.Name
		return uri

	case "trojan":
		uri := fmt.Sprintf("trojan://%s@%s:%d", node.Password, node.Host, node.Port)
		if node.TLS {
			uri += "?security=tls"
			if node.ServerName != "" {
				uri += "&sni=" + node.ServerName
			}
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
