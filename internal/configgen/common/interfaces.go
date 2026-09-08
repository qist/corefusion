package common

import (
	"corefusion/internal/domain"
)

// ConfigGenerator generates core-specific configuration from the unified model.
type ConfigGenerator interface {
	CoreType() domain.CoreType
	GenerateServiceConfig(spec ServiceConfigSpec) ([]byte, error)
	GenerateClientConfig(spec ClientConfigSpec) ([]byte, error)
	ValidateConfig(content []byte) error
}

// ServiceConfigSpec is the input for generating a service config.
type ServiceConfigSpec struct {
	ServiceID   int64
	Name        string
	CoreType    domain.CoreType
	Protocol    domain.ProtocolType
	Transport   domain.TransportType
	TLSMode     domain.TLSMode
	ListenHost  string
	ListenPort  int
	Endpoints   []EndpointConfig
	Users       []UserConfig
	CertPath    string
	KeyPath     string
	Extra       map[string]any
}

// ClientConfigSpec is the input for generating a client config.
type ClientConfigSpec struct {
	CoreType      domain.CoreType
	Nodes         []NodeConfig
	TunEnabled    bool
	ProxyMode     string
	DNSServers    []string
	RouteRules    []RouteRule
	Platform      string
}

// EndpointConfig is an endpoint in a service config.
type EndpointConfig struct {
	EndpointType domain.EndpointType
	Address      string
	ServerName   string
	ListenHost   string
	ListenPort   int
	TLSEnabled   bool
}

// UserConfig is a user in a service config.
type UserConfig struct {
	Email    string
	Password string
	UUID     string
	AuthKey  string
}

// NodeConfig is a node in a client config.
type NodeConfig struct {
	Name      string
	CoreType  domain.CoreType
	Protocol  domain.ProtocolType
	Host      string
	Port      int
	Transport domain.TransportType
	TLS       TLSConfig
	Auth      AuthConfig
	// Extra holds protocol/transport-specific parameters that don't fit
	// into the structured fields above. Examples:
	//   - ws_path, ws_headers, ws_max_early_data (WebSocket transport)
	//   - grpc_service_name (gRPC transport)
	//   - http_host, http_path (HTTP/2 transport)
	//   - httpupgrade_host, httpupgrade_path (HTTPUpgrade transport)
	//   - vless_flow (VLESS XTLS Vision flow control)
	//   - ss_method (Shadowsocks encryption method)
	//   - hysteria2_up, hysteria2_down (Hysteria2 bandwidth)
	//   - multiplex_enabled, multiplex_protocol (Multiplex)
	// These are populated by the subscription/sharelink parser.
	Extra     map[string]any
}

// TLSConfig is TLS configuration for a node.
type TLSConfig struct {
	Enabled     bool
	ServerName string
	ALPN       []string
	Fingerprint string
	// ECH enabled (Encrypted Client Hello)
	ECH        bool
	// ECHConfig (base64-encoded ECH config list)
	ECHConfig  string
}

// AuthConfig is authentication configuration for a node.
type AuthConfig struct {
	UUID     string
	Password string
	AuthKey  string
}

// RouteRule is a routing rule in a client config.
type RouteRule struct {
	MatchType   string
	MatchValue  string
	OutboundTag string
	Priority    int
}

// CapabilityValidator validates protocol/transport/TLS combinations.
type CapabilityValidator interface {
	Validate(spec ServiceConfigSpec) error
	SupportsProtocol(core domain.CoreType, protocol domain.ProtocolType) bool
	SupportsTransport(core domain.CoreType, protocol domain.ProtocolType, transport domain.TransportType) bool
	SupportsTLS(core domain.CoreType, protocol domain.ProtocolType, transport domain.TransportType, tls domain.TLSMode) bool
}
