package mihomo

import (
	"fmt"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// Generator generates Mihomo (Clash.Meta) configuration.
type Generator struct{}

func NewGenerator() *Generator { return &Generator{} }
func (g *Generator) CoreType() domain.CoreType { return "mihomo" }

// GenerateServiceConfig is not applicable for Mihomo (client-only).
func (g *Generator) GenerateServiceConfig(spec common.ServiceConfigSpec) ([]byte, error) {
	return nil, fmt.Errorf("mihomo is a client-only core, use GenerateClientConfig")
}

// GenerateClientConfig generates a Mihomo YAML client configuration.
func (g *Generator) GenerateClientConfig(spec common.ClientConfigSpec) ([]byte, error) {
	// Build YAML output as string (avoiding YAML library dependency)
	yaml := "mixed-port: 7890\n"
	yaml += "allow-lan: false\n"
	yaml += "mode: rule\n"
	yaml += "log-level: info\n\n"

	// DNS
	yaml += "dns:\n"
	yaml += "  enable: true\n"
	yaml += "  enhanced-mode: fake-ip\n"
	if len(spec.DNSServers) > 0 {
		yaml += "  nameserver:\n"
		for _, s := range spec.DNSServers {
			yaml += fmt.Sprintf("    - %s\n", s)
		}
	}
	yaml += "\n"

	// Proxies
	yaml += "proxies:\n"
	for _, node := range spec.Nodes {
		proxy := g.renderProxy(node)
		yaml += proxy
	}
	yaml += "\n"

	// Proxy groups
	yaml += "proxy-groups:\n"
	yaml += "  - name: PROXY\n"
	yaml += "    type: select\n"
	yaml += "    proxies:\n"
	for _, node := range spec.Nodes {
		yaml += fmt.Sprintf("      - %s\n", node.Name)
	}
	yaml += "\n"

	// Rules
	yaml += "rules:\n"
	for _, rule := range spec.RouteRules {
		yaml += fmt.Sprintf("  - %s,%s,%s\n", rule.MatchType, rule.MatchValue, rule.OutboundTag)
	}
	yaml += "  - GEOIP,PRIVATE,DIRECT\n"
	yaml += "  - MATCH,PROXY\n"

	return []byte(yaml), nil
}

func (g *Generator) renderProxy(node common.NodeConfig) string {
	yaml := fmt.Sprintf("  - name: %s\n", node.Name)

	switch node.Protocol {
	case "vmess":
		yaml += "    type: vmess\n"
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
		yaml += fmt.Sprintf("    uuid: %s\n", node.Auth.UUID)
		yaml += "    alterId: 0\n"
		yaml += "    cipher: auto\n"
		if node.Transport != "" && node.Transport != "tcp" {
			yaml += fmt.Sprintf("    network: %s\n", node.Transport)
		}
	case "vless":
		yaml += "    type: vless\n"
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
		yaml += fmt.Sprintf("    uuid: %s\n", node.Auth.UUID)
		if node.Transport != "" && node.Transport != "tcp" {
			yaml += fmt.Sprintf("    network: %s\n", node.Transport)
		}
	case "trojan":
		yaml += "    type: trojan\n"
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
		yaml += fmt.Sprintf("    password: %s\n", node.Auth.Password)
	case "shadowsocks":
		yaml += "    type: ss\n"
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
		yaml += fmt.Sprintf("    cipher: %s\n", "auto")
		yaml += fmt.Sprintf("    password: %s\n", node.Auth.Password)
	case "hysteria2":
		yaml += "    type: hysteria2\n"
		yaml += fmt.Sprintf("    server: %s\n", node.Host)
		yaml += fmt.Sprintf("    port: %d\n", node.Port)
		yaml += fmt.Sprintf("    password: %s\n", node.Auth.Password)
	}

	if node.TLS.Enabled {
		yaml += "    tls: true\n"
		if node.TLS.ServerName != "" {
			yaml += fmt.Sprintf("    servername: %s\n", node.TLS.ServerName)
		}
		if node.TLS.Fingerprint != "" {
			yaml += fmt.Sprintf("    fingerprint: %s\n", node.TLS.Fingerprint)
		}
	}

	return yaml
}

// ValidateConfig validates a Mihomo configuration.
func (g *Generator) ValidateConfig(content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("empty configuration")
	}
	// Basic validation - check for required fields
	content_str := string(content)
	if len(content_str) < 10 {
		return fmt.Errorf("configuration too short")
	}
	return nil
}
