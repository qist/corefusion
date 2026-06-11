package singbox

import (
	"encoding/json"
	"fmt"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// Generator generates sing-box configuration from the unified model.
type Generator struct{}

// NewGenerator creates a new sing-box config generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// CoreType returns the core type.
func (g *Generator) CoreType() domain.CoreType {
	return domain.CoreTypeSingBox
}

// SingBoxConfig is the top-level sing-box configuration.
type SingBoxConfig struct {
	Log        *LogConfig       `json:"log,omitempty"`
	DNS        *DNSConfig       `json:"dns,omitempty"`
	Inbounds   []InboundConfig  `json:"inbounds"`
	Outbounds  []OutboundConfig `json:"outbounds"`
	Route      *RouteConfig     `json:"route,omitempty"`
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`
}

type LogConfig struct {
	Level     string `json:"level"`
	Output    string `json:"output"`
	Timestamp bool   `json:"timestamp"`
}

type DNSConfig struct {
	Servers  []DNSServer  `json:"servers"`
	Rules    []DNSRule    `json:"rules,omitempty"`
	FakeIP   *FakeIPConfig `json:"fakeip,omitempty"`
}

type DNSServer struct {
	Tag:     string `json:"tag"`
	Address  string `json:"address"`
	Detour   string `json:"detour,omitempty"`
}

type DNSRule struct {
	Type        string   `json:"type"`
	Domain      []string `json:"domain,omitempty"`
	Server      string   `json:"server"`
}

type FakeIPConfig struct {
	Enabled    bool   `json:"enabled"`
	Inet4Range string `json:"inet4_range"`
}

type InboundConfig struct {
	Type           string          `json:"type"`
	Tag            string          `json:"tag"`
	Listen         string          `json:"listen"`
	ListenPort     int             `json:"listen_port"`
	Sniff          bool            `json:"sniff,omitempty"`
	SniffOverrideDestination bool `json:"sniff_override_destination,omitempty"`
	Users          []UserConfig    `json:"users,omitempty"`
	TLS            *TLSConfig      `json:"tls,omitempty"`
	Transport      *TransportConfig `json:"transport,omitempty"`
	Multiplex      *MultiplexConfig `json:"multiplex,omitempty"`
}

type UserConfig struct {
	Name     string `json:"name"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
}

type TLSConfig struct {
	Enabled     bool     `json:"enabled"`
	ServerName  string   `json:"server_name,omitempty"`
	ALPN        []string `json:"alpn,omitempty"`
	MinVersion  string   `json:"min_version,omitempty"`
	MaxVersion  string   `json:"max_version,omitempty"`
	CertificatePath string `json:"certificate_path,omitempty"`
	KeyPath     string   `json:"key_path,omitempty"`
	Reality     *RealityConfig `json:"reality,omitempty"`
}

type RealityConfig struct {
	Enabled    bool   `json:"enabled"`
	Handshake  *HandshakeConfig `json:"handshake,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	PublicKey  string `json:"public_key,omitempty"`
	ShortID    string `json:"short_id,omitempty"`
}

type HandshakeConfig struct {
	Server string `json:"server"`
	ServerPort int `json:"server_port"`
}

type TransportConfig struct {
	Type        string `json:"type"`
	Path        string `json:"path,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Host        string `json:"host,omitempty"`
}

type MultiplexConfig struct {
	Enabled bool   `json:"enabled"`
	Protocol string `json:"protocol,omitempty"`
}

type OutboundConfig struct {
	Type        string        `json:"type"`
	Tag         string        `json:"tag"`
	Server      string        `json:"server,omitempty"`
	ServerPort  int           `json:"server_port,omitempty"`
	UUID        string        `json:"uuid,omitempty"`
	Password    string        `json:"password,omitempty"`
	Method      string        `json:"method,omitempty"`
	Transport   *TransportConfig `json:"transport,omitempty"`
	TLS         *TLSConfig    `json:"tls,omitempty"`
}

type RouteConfig struct {
	Rules    []RouteRule `json:"rules"`
	Final    string      `json:"final,omitempty"`
	AutoDetectInterface bool `json:"auto_detect_interface,omitempty"`
}

type RouteRule struct {
	Type        string   `json:"type"`
	Mode        string   `json:"mode,omitempty"`
	Domain      []string `json:"domain,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	IPCIDR      []string `json:"ip_cidr,omitempty"`
	Outbound    string   `json:"outbound"`
}

type ExperimentalConfig struct {
	ClashAPI *ClashAPIConfig `json:"clash_api,omitempty"`
}

type ClashAPIConfig struct {
	ExternalController string `json:"external_controller,omitempty"`
	ExternalUI         string `json:"external_ui,omitempty"`
	Secret             string `json:"secret,omitempty"`
}

// GenerateServiceConfig generates a sing-box inbound service configuration.
func (g *Generator) GenerateServiceConfig(spec common.ServiceConfigSpec) ([]byte, error) {
	inbound := InboundConfig{
		Type:       string(spec.Protocol),
		Tag:        fmt.Sprintf("inbound-%d", spec.ServiceID),
		Listen:     spec.ListenHost,
		ListenPort: spec.ListenPort,
		Sniff:      true,
		SniffOverrideDestination: true,
	}

	// Build users
	var users []UserConfig
	for _, u := range spec.Users {
		user := UserConfig{Name: u.Email}
		if u.UUID != "" {
			user.UUID = u.UUID
		}
		if u.Password != "" {
			user.Password = u.Password
		}
		users = append(users, user)
	}
	inbound.Users = users

	// TLS
	if spec.TLSMode != domain.TLSModeNone {
		tlsConfig := &TLSConfig{Enabled: true}
		if len(spec.Endpoints) > 0 && spec.Endpoints[0].ServerName != "" {
			tlsConfig.ServerName = spec.Endpoints[0].ServerName
		}
		if spec.CertPath != "" {
			tlsConfig.CertificatePath = spec.CertPath
			tlsConfig.KeyPath = spec.KeyPath
		}
		if spec.TLSMode == domain.TLSModeReality {
			tlsConfig.Reality = &RealityConfig{Enabled: true}
			if privKey, ok := spec.Extra["reality_private_key"].(string); ok {
				tlsConfig.Reality.PrivateKey = privKey
			}
			if pubKey, ok := spec.Extra["reality_public_key"].(string); ok {
				tlsConfig.Reality.PublicKey = pubKey
			}
			if shortID, ok := spec.Extra["reality_short_id"].(string); ok {
				tlsConfig.Reality.ShortID = shortID
			}
		}
		inbound.TLS = tlsConfig
	}

	// Transport
	if spec.Transport != "" && spec.Transport != domain.TransportTCP {
		transportConfig := &TransportConfig{Type: string(spec.Transport)}
		if path, ok := spec.Extra["ws_path"].(string); ok {
			transportConfig.Path = path
		}
		if sn, ok := spec.Extra["grpc_service_name"].(string); ok {
			transportConfig.ServiceName = sn
		}
		inbound.Transport = transportConfig
	}

	config := SingBoxConfig{
		Log: &LogConfig{
			Level:     "warn",
			Output:    "stderr",
			Timestamp: true,
		},
		Inbounds: []InboundConfig{inbound},
		Outbounds: []OutboundConfig{
			{Type: "direct", Tag: "direct"},
			{Type: "block", Tag: "block"},
			{Type: "dns", Tag: "dns-out"},
		},
		Route: &RouteConfig{
			AutoDetectInterface: true,
			Final: "direct",
			Rules: []RouteRule{
				{Type: "logical", Mode: "or", Domain: []string{}, Outbound: "dns-out"},
			},
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

// GenerateClientConfig generates a sing-box client configuration.
func (g *Generator) GenerateClientConfig(spec common.ClientConfigSpec) ([]byte, error) {
	config := SingBoxConfig{
		Log: &LogConfig{
			Level:     "warn",
			Output:    "stderr",
			Timestamp: true,
		},
	}

	// Generate outbounds from nodes
	for i, node := range spec.Nodes {
		outbound := OutboundConfig{
			Type:       string(node.Protocol),
			Tag:        fmt.Sprintf("proxy-%d", i),
			Server:     node.Host,
			ServerPort: node.Port,
		}

		if node.Auth.UUID != "" {
			outbound.UUID = node.Auth.UUID
		}
		if node.Auth.Password != "" {
			outbound.Password = node.Auth.Password
		}

		// TLS
		if node.TLS.Enabled {
			outbound.TLS = &TLSConfig{
				Enabled:    true,
				ServerName: node.TLS.ServerName,
				ALPN:       node.TLS.ALPN,
			}
		}

		// Transport
		if node.Transport != "" && node.Transport != domain.TransportTCP {
			outbound.Transport = &TransportConfig{
				Type: string(node.Transport),
			}
		}

		config.Outbounds = append(config.Outbounds, outbound)
	}

	// Add direct, block, dns outbounds
	config.Outbounds = append(config.Outbounds,
		OutboundConfig{Type: "direct", Tag: "direct"},
		OutboundConfig{Type: "block", Tag: "block"},
		OutboundConfig{Type: "dns", Tag: "dns-out"},
	)

	// Build route rules
	route := &RouteConfig{
		AutoDetectInterface: true,
	}

	for _, rule := range spec.RouteRules {
		rr := RouteRule{
			Type:     "field",
			Outbound: rule.OutboundTag,
		}

		switch rule.MatchType {
		case "domain", "domain_suffix":
			rr.DomainSuffix = []string{rule.MatchValue}
		case "domain_keyword":
			rr.Domain = []string{rule.MatchValue}
		case "ip", "cidr":
			rr.IPCIDR = []string{rule.MatchValue}
		}

		route.Rules = append(route.Rules, rr)
	}

	config.Route = route

	// TUN
	if spec.TunEnabled {
		config.Inbounds = append(config.Inbounds, InboundConfig{
			Type: "tun",
			Tag:  "tun-in",
		})
	}

	return json.MarshalIndent(config, "", "  ")
}

// ValidateConfig validates a sing-box configuration.
func (g *Generator) ValidateConfig(content []byte) error {
	var config SingBoxConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(config.Inbounds) == 0 && len(config.Outbounds) == 0 {
		return fmt.Errorf("no inbounds or outbounds defined")
	}

	return nil
}
