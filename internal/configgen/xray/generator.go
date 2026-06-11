package xray

import (
	"encoding/json"
	"fmt"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// Generator generates Xray-core configuration from the unified model.
type Generator struct{}

// NewGenerator creates a new Xray config generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// CoreType returns the core type.
func (g *Generator) CoreType() domain.CoreType {
	return domain.CoreTypeXray
}

// XrayConfig is the top-level Xray configuration.
type XrayConfig struct {
	Log       LogConfig        `json:"log"`
	API       *APIConfig       `json:"api,omitempty"`
	DNS       *DNSConfig       `json:"dns,omitempty"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
	Routing   *RoutingConfig   `json:"routing,omitempty"`
	Policy    *PolicyConfig    `json:"policy,omitempty"`
	Stats     *StatsConfig     `json:"stats,omitempty"`
}

type LogConfig struct {
	Access   string `json:"access"`
	Error    string `json:"error"`
	LogLevel string `json:"loglevel"`
}

type APIConfig struct {
	Tag      string   `json:"tag"`
	Services []string `json:"services"`
}

type StatsConfig struct{}

type PolicyConfig struct {
	Levels map[string]PolicyLevel `json:"levels"`
	System *SystemPolicy          `json:"system,omitempty"`
}

type PolicyLevel struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

type SystemPolicy struct {
	StatsInboundUplink   bool `json:"statsInboundUplink"`
	StatsInboundDownlink bool `json:"statsInboundDownlink"`
}

type InboundConfig struct {
	Listen         string           `json:"listen,omitempty"`
	Port           int              `json:"port"`
	Protocol       string           `json:"protocol"`
	Settings       *InboundSettings `json:"settings,omitempty"`
	StreamSettings *StreamSettings  `json:"streamSettings,omitempty"`
	Tag            string           `json:"tag"`
	Sniffing       *SniffingConfig  `json:"sniffing,omitempty"`
}

type SniffingConfig struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

type InboundSettings struct {
	Clients    []ClientConfig `json:"clients,omitempty"`
	UDP        bool           `json:"udp,omitempty"`
	Auth       string         `json:"auth,omitempty"`
	Password   string         `json:"password,omitempty"`
	Email      string         `json:"email,omitempty"`
}

type ClientConfig struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	AlterID int  `json:"alterId,omitempty"`
}

type StreamSettings struct {
	Network    string         `json:"network"`
	Security   string         `json:"security"`
	TLSSettings *TLSSettings  `json:"tlsSettings,omitempty"`
	WSSettings  *WSSettings   `json:"wsSettings,omitempty"`
	GRPCSettings *GRPCSettings `json:"grpcSettings,omitempty"`
	HTTPSettings *HTTPSettings `json:"httpSettings,omitempty"`
	RealitySettings *RealitySettings `json:"realitySettings,omitempty"`
}

type TLSSettings struct {
	ServerName string   `json:"serverName"`
	ALPN       []string `json:"alpn,omitempty"`
	Fingerprint string  `json:"fingerprint,omitempty"`
	AllowInsecure bool  `json:"allowInsecure,omitempty"`
}

type WSSettings struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

type GRPCSettings struct {
	ServiceName string `json:"serviceName"`
}

type HTTPSettings struct {
	Path string   `json:"path"`
	Host []string `json:"host,omitempty"`
}

type RealitySettings struct {
	Show   bool   `json:"show"`
	Xver   int    `json:"xver,omitempty"`
	Server string `json:"server"`
	Serv   string `json:"serv"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	ShortID    string `json:"shortId"`
	SpiderX    string `json:"spiderX,omitempty"`
}

type OutboundConfig struct {
	Protocol       string            `json:"protocol"`
	Settings       *OutboundSettings `json:"settings,omitempty"`
	StreamSettings *StreamSettings   `json:"streamSettings,omitempty"`
	Tag            string            `json:"tag"`
}

type OutboundSettings struct {
	Vnext []VnextServer `json:"vnext,omitempty"`
	Servers []ServerConfig `json:"servers,omitempty"`
}

type VnextServer struct {
	Address string       `json:"address"`
	Port    int          `json:"port"`
	Users   []UserConfig `json:"users"`
}

type UserConfig struct {
	ID       string `json:"id"`
	AlterID  int    `json:"alterId,omitempty"`
	Email    string `json:"email,omitempty"`
	Security string `json:"security,omitempty"`
	Password string `json:"password,omitempty"`
}

type ServerConfig struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Method   string `json:"method,omitempty"`
	Password string `json:"password,omitempty"`
}

type DNSConfig struct {
	Servers []interface{} `json:"servers"`
}

type RoutingConfig struct {
	DomainStrategy string        `json:"domainStrategy"`
	Rules          []RoutingRule `json:"rules"`
}

type RoutingRule struct {
	Type        string   `json:"type"`
	Domain      []string `json:"domain,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Port        string   `json:"port,omitempty"`
	Network     string   `json:"network,omitempty"`
	Source      []string `json:"source,omitempty"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

// GenerateServiceConfig generates an Xray inbound service configuration.
func (g *Generator) GenerateServiceConfig(spec common.ServiceConfigSpec) ([]byte, error) {
	inbound := InboundConfig{
		Listen: spec.ListenHost,
		Port:   spec.ListenPort,
		Protocol: string(spec.Protocol),
		Tag:    fmt.Sprintf("inbound-%d", spec.ServiceID),
		Sniffing: &SniffingConfig{
			Enabled:      true,
			DestOverride: []string{"http", "tls", "quic"},
		},
	}

	// Build stream settings
	streamSettings := &StreamSettings{
		Network: "tcp",
		Security: "none",
	}

	if spec.Transport != "" {
		streamSettings.Network = string(spec.Transport)
	}

	switch spec.TLSMode {
	case domain.TLSModeTLS:
		streamSettings.Security = "tls"
		streamSettings.TLSSettings = &TLSSettings{
			ServerName: spec.Endpoints[0].ServerName,
		}
	case domain.TLSModeReality:
		streamSettings.Security = "reality"
		// Reality settings would be populated from spec.Extra
	}

	// Build network-specific settings
	switch spec.Transport {
	case domain.TransportWS:
		path := "/"
		if p, ok := spec.Extra["ws_path"].(string); ok {
			path = p
		}
		streamSettings.WSSettings = &WSSettings{Path: path}
	case domain.TransportGRPC:
		serviceName := ""
		if sn, ok := spec.Extra["grpc_service_name"].(string); ok {
			serviceName = sn
		}
		streamSettings.GRPCSettings = &GRPCSettings{ServiceName: serviceName}
	case domain.TransportHTTP2:
		streamSettings.HTTPSettings = &HTTPSettings{Path: "/"}
	}

	inbound.StreamSettings = streamSettings

	// Build client settings based on protocol
	settings := &InboundSettings{}
	switch spec.Protocol {
	case domain.ProtocolVLESS:
		var clients []ClientConfig
		for _, u := range spec.Users {
			clients = append(clients, ClientConfig{
				ID:    u.UUID,
				Email: u.Email,
			})
		}
		settings.Clients = clients
	case domain.ProtocolVMess:
		var clients []ClientConfig
		for _, u := range spec.Users {
			clients = append(clients, ClientConfig{
				ID:      u.UUID,
				Email:   u.Email,
				AlterID: 0,
			})
		}
		settings.Clients = clients
	case domain.ProtocolTrojan:
		var clients []ClientConfig
		for _, u := range spec.Users {
			clients = append(clients, ClientConfig{
				Password: u.Password,
				Email:    u.Email,
			})
		}
		settings.Clients = clients
	}

	inbound.Settings = settings

	// Build full config
	config := XrayConfig{
		Log: LogConfig{
			Access:   "/dev/null",
			Error:    "/dev/stderr",
			LogLevel: "warning",
		},
		Inbounds: []InboundConfig{inbound},
		Outbounds: []OutboundConfig{
			{
				Protocol: "freedom",
				Tag:      "direct",
			},
			{
				Protocol: "blackhole",
				Tag:      "blocked",
			},
		},
		Routing: &RoutingConfig{
			DomainStrategy: "AsIs",
			Rules: []RoutingRule{
				{
					Type:        "field",
					InboundTag:  []string{"api"},
					OutboundTag: "api",
				},
			},
		},
		Stats: &StatsConfig{},
		Policy: &PolicyConfig{
			Levels: map[string]PolicyLevel{
				"0": {
					StatsUserUplink:   true,
					StatsUserDownlink: true,
				},
			},
			System: &SystemPolicy{
				StatsInboundUplink:   true,
				StatsInboundDownlink: true,
			},
		},
	}

	return json.MarshalIndent(config, "", "  ")
}

// GenerateClientConfig generates an Xray client configuration.
func (g *Generator) GenerateClientConfig(spec common.ClientConfigSpec) ([]byte, error) {
	config := XrayConfig{
		Log: LogConfig{
			Access:   "/dev/null",
			Error:    "/dev/stderr",
			LogLevel: "warning",
		},
	}

	// Generate outbounds from nodes
	for i, node := range spec.Nodes {
		outbound := OutboundConfig{
			Protocol: string(node.Protocol),
			Tag:      fmt.Sprintf("proxy-%d", i),
		}

		// Build server settings
		outSettings := &OutboundSettings{
			Vnext: []VnextServer{
				{
					Address: node.Host,
					Port:    node.Port,
					Users: []UserConfig{
						{
							ID:       node.Auth.UUID,
							Email:    node.Name,
							Security: "auto",
							Password: node.Auth.Password,
						},
					},
				},
			},
		}
		outbound.Settings = outSettings

		// Build stream settings
		streamSettings := &StreamSettings{
			Network:  "tcp",
			Security: "none",
		}

		if node.Transport != "" {
			streamSettings.Network = string(node.Transport)
		}

		if node.TLS.Enabled {
			streamSettings.Security = "tls"
			streamSettings.TLSSettings = &TLSSettings{
				ServerName:   node.TLS.ServerName,
				Fingerprint:  node.TLS.Fingerprint,
				ALPN:         node.TLS.ALPN,
			}
		}

		outbound.StreamSettings = streamSettings
		config.Outbounds = append(config.Outbounds, outbound)
	}

	// Add direct and block outbounds
	config.Outbounds = append(config.Outbounds,
		OutboundConfig{Protocol: "freedom", Tag: "direct"},
		OutboundConfig{Protocol: "blackhole", Tag: "block"},
	)

	// Build routing rules
	routing := &RoutingConfig{
		DomainStrategy: "IPIfNonMatch",
	}

	for _, rule := range spec.RouteRules {
		rr := RoutingRule{
			Type:        "field",
			OutboundTag: rule.OutboundTag,
		}

		switch rule.MatchType {
		case "domain", "domain_suffix", "domain_keyword":
			rr.Domain = []string{rule.MatchValue}
		case "ip", "cidr":
			rr.IP = []string{rule.MatchValue}
		}

		routing.Rules = append(routing.Rules, rr)
	}

	// Default rules
	routing.Rules = append(routing.Rules,
		RoutingRule{Type: "field", IP: []string{"geoip:private"}, OutboundTag: "direct"},
		RoutingRule{Type: "field", Domain: []string{"geosite:category-ads-all"}, OutboundTag: "block"},
	)

	config.Routing = routing

	// DNS
	if len(spec.DNSServers) > 0 {
		servers := make([]interface{}, len(spec.DNSServers))
		for i, s := range spec.DNSServers {
			servers[i] = s
		}
		config.DNS = &DNSConfig{Servers: servers}
	}

	return json.MarshalIndent(config, "", "  ")
}

// ValidateConfig validates an Xray configuration.
func (g *Generator) ValidateConfig(content []byte) error {
	var config XrayConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(config.Inbounds) == 0 {
		return fmt.Errorf("no inbounds defined")
	}

	return nil
}
