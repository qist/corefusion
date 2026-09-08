package singbox

import (
	"encoding/json"
	"fmt"
	"strings"

	"corefusion/internal/configgen/common"
	"corefusion/internal/domain"
)

// Generator generates sing-box configuration from the unified model.
type Generator struct{}

// NewGenerator creates a new sing-box config generator.
func NewGenerator() *Generator { return &Generator{} }

// CoreType returns the core type.
func (g *Generator) CoreType() domain.CoreType { return domain.CoreTypeSingBox }

// ---------------------------------------------------------------------------
// Top-level config
// ---------------------------------------------------------------------------

// SingBoxConfig is the top-level sing-box configuration.
// https://sing-box.sagernet.org/configuration/
type SingBoxConfig struct {
	Log          *LogConfig        `json:"log,omitempty"`
	DNS          *DNSConfig        `json:"dns,omitempty"`
	NTP          *NTPConfig        `json:"ntp,omitempty"`
	Inbounds     []InboundConfig   `json:"inbounds"`
	Outbounds    []OutboundConfig  `json:"outbounds"`
	Route        *RouteConfig      `json:"route,omitempty"`
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`
}

// ---------------------------------------------------------------------------
// Log
// ---------------------------------------------------------------------------

// LogConfig matches sing-box log structure.
// https://sing-box.sagernet.org/configuration/log/
type LogConfig struct {
	Disabled  bool   `json:"disabled,omitempty"`
	Level     string `json:"level"`
	Output    string `json:"output,omitempty"`
	Timestamp bool   `json:"timestamp,omitempty"`
}

// ---------------------------------------------------------------------------
// NTP
// ---------------------------------------------------------------------------

// NTPConfig — sing-box NTP configuration.
type NTPConfig struct {
	Enabled    bool   `json:"enabled"`
	Server     string `json:"server,omitempty"`
	ServerPort int    `json:"server_port,omitempty"`
	Interval   string `json:"interval,omitempty"`
	TimeSync   bool   `json:"time_sync,omitempty"`
}

// ---------------------------------------------------------------------------
// DNS
// ---------------------------------------------------------------------------

// DNSConfig matches sing-box DNS structure.
// https://sing-box.sagernet.org/configuration/dns/
type DNSConfig struct {
	Servers        []DNSServer   `json:"servers"`
	Rules          []DNSRule     `json:"rules,omitempty"`
	Final          string        `json:"final,omitempty"`
	Strategy       string        `json:"strategy,omitempty"`
	DisableCache   bool          `json:"disable_cache,omitempty"`
	DisableExpire  bool          `json:"disable_expire,omitempty"`
	ReverseMapping bool          `json:"reverse_mapping,omitempty"`
	FakeIP         *FakeIPConfig `json:"fakeip,omitempty"`
}

// DNSServer — sing-box 1.12+ DNS server structure.
// https://sing-box.sagernet.org/configuration/dns/server/
type DNSServer struct {
	Type    string `json:"type,omitempty"`
	Tag     string `json:"tag"`
	Address string `json:"address,omitempty"`
	Detour  string `json:"detour,omitempty"`
}

// DNSRule — sing-box DNS rule.
type DNSRule struct {
	Type     string   `json:"type,omitempty"`
	Domain   []string `json:"domain,omitempty"`
	Server   string   `json:"server"`
}

// FakeIPConfig — FakeIP DNS configuration.
type FakeIPConfig struct {
	Enabled    bool   `json:"enabled"`
	Inet4Range string `json:"inet4_range,omitempty"`
}

// ---------------------------------------------------------------------------
// Inbound
// ---------------------------------------------------------------------------

// InboundConfig — sing-box inbound configuration.
// Embeds Listen Fields + protocol-specific fields.
// https://sing-box.sagernet.org/configuration/inbound/
type InboundConfig struct {
	// Listen Fields (shared by all inbounds)
	Type                   string `json:"type"`
	Tag                    string `json:"tag"`
	Listen                 string `json:"listen,omitempty"`
	ListenPort             int    `json:"listen_port,omitempty"`
	TCPFastOpen            bool   `json:"tcp_fast_open,omitempty"`
	TCPMultiPath           bool   `json:"tcp_multi_path,omitempty"`
	UDPFragment            bool   `json:"udp_fragment,omitempty"`
	UDPTimeout             string `json:"udp_timeout,omitempty"`
	Detour                 string `json:"detour,omitempty"`

	// Deprecated Listen Fields (still supported in 1.14.x)
	Sniff                    bool   `json:"sniff,omitempty"`
	SniffOverrideDestination bool   `json:"sniff_override_destination,omitempty"`
	SniffTimeout             string `json:"sniff_timeout,omitempty"`
	DomainStrategy           string `json:"domain_strategy,omitempty"`

	// Protocol-specific: VLESS / VMess / Trojan / TUIC
	Users []InboundUser `json:"users,omitempty"`

	// Protocol-specific: Shadowsocks
	Method   string `json:"method,omitempty"`
	Password string `json:"password,omitempty"`
	Network  string `json:"network,omitempty"`
	Managed  bool   `json:"managed,omitempty"`

	// Protocol-specific: Hysteria2 (sing-box 1.14+ uses up_mbps/down_mbps)
	UpMbps                int         `json:"up_mbps,omitempty"`
	DownMbps              int         `json:"down_mbps,omitempty"`
	IgnoreClientBandwidth bool        `json:"ignore_client_bandwidth,omitempty"`
	Masquerade             interface{} `json:"masquerade,omitempty"`

	// Protocol-specific: TUIC
	CongestionControl string `json:"congestion_control,omitempty"`
	AuthTimeout       string `json:"auth_timeout,omitempty"`
	MaxIdleTime       string `json:"max_idle_time,omitempty"`

	// Protocol-specific: ShadowTLS / Snell / Hysteria
	Version  int    `json:"version,omitempty"`
	PSK      string `json:"psk,omitempty"`
	Obfs     string `json:"obfs,omitempty"`
	ObfsHost string `json:"obfs_host,omitempty"`

	// Protocol-specific: WireGuard
	PrivateKey  string             `json:"private_key,omitempty"`
	LocalAddress []string          `json:"local_address,omitempty"`
	Peers       []WireGuardPeer    `json:"peers,omitempty"`

	// Shared optional fields
	TLS       *TLSConfig            `json:"tls,omitempty"`
	Multiplex *MultiplexConfig      `json:"multiplex,omitempty"`
	Transport *V2RayTransportConfig `json:"transport,omitempty"`

	// Trojan fallback
	Fallback        *FallbackConfig           `json:"fallback,omitempty"`
	FallbackForALPN map[string]FallbackConfig `json:"fallback_for_alpn,omitempty"`
}

// WireGuardPeer — WireGuard peer without server fields (inbound uses system wireguard).
type WireGuardPeer struct {
	PublicKey string `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	PersistentKeepalive int `json:"persistent_keepalive,omitempty"`
}

// InboundUser — user in an inbound.
type InboundUser struct {
	Name     string `json:"name"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"` // socks/http/mixed/naive
	UserKey  string `json:"userkey,omitempty"` // snell
	AlterID  int    `json:"alterId,omitempty"`
	Flow     string `json:"flow,omitempty"`
}

// FallbackConfig — Trojan fallback server.
type FallbackConfig struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

// ---------------------------------------------------------------------------
// TLS
// ---------------------------------------------------------------------------

// TLSConfig — sing-box TLS configuration.
// https://sing-box.sagernet.org/configuration/shared/tls/
type TLSConfig struct {
	Enabled          bool     `json:"enabled"`
	DisableSNI       bool     `json:"disable_sni,omitempty"`
	ServerName       string   `json:"server_name,omitempty"`
	Insecure         bool     `json:"insecure,omitempty"`
	ALPN             []string `json:"alpn,omitempty"`
	MinVersion       string   `json:"min_version,omitempty"`
	MaxVersion       string   `json:"max_version,omitempty"`
	CipherSuites     []string `json:"cipher_suites,omitempty"`
	CurvePreferences []string `json:"curve_preferences,omitempty"`

	// Server-side (inbound)
	Certificate     []string `json:"certificate,omitempty"`
	CertificatePath string   `json:"certificate_path,omitempty"`
	Key             []string `json:"key,omitempty"`
	KeyPath         string   `json:"key_path,omitempty"`

	// Client-side (outbound)
	ClientCertificate     []string `json:"client_certificate,omitempty"`
	ClientCertificatePath string   `json:"client_certificate_path,omitempty"`
	ClientKey             []string `json:"client_key,omitempty"`
	ClientKeyPath         string   `json:"client_key_path,omitempty"`

	// uTLS
	UTLS *UTLSConfig `json:"utls,omitempty"`

	// ECH (Encrypted Client Hello) — sing-box 1.14+
	ECHConfig string `json:"ech_config,omitempty"`

	// Reality
	Reality *RealityConfig `json:"reality,omitempty"`

	// Handshake timeout (1.14+)
	HandshakeTimeout string `json:"handshake_timeout,omitempty"`
}

// UTLSConfig — uTLS fingerprint configuration.
type UTLSConfig struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// RealityConfig — sing-box Reality TLS configuration.
type RealityConfig struct {
	Enabled          bool             `json:"enabled"`
	Handshake        *HandshakeConfig `json:"handshake,omitempty"`
	PrivateKey       string           `json:"private_key,omitempty"`
	PublicKey        string           `json:"public_key,omitempty"`
	ShortID          []string         `json:"short_id,omitempty"`
	MaxTimeDifference string         `json:"max_time_difference,omitempty"`
}

// HandshakeConfig — Reality handshake target.
type HandshakeConfig struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

// ---------------------------------------------------------------------------
// V2Ray Transport
// ---------------------------------------------------------------------------

// V2RayTransportConfig — sing-box V2Ray transport configuration.
// https://sing-box.sagernet.org/configuration/shared/v2ray-transport/
type V2RayTransportConfig struct {
	Type               string            `json:"type"`
	Host               string            `json:"host,omitempty"`
	Path               string            `json:"path,omitempty"`
	Headers            map[string]string `json:"headers,omitempty"`
	MaxEarlyData       int               `json:"max_early_data,omitempty"`
	EarlyDataHeaderName string           `json:"early_data_header_name,omitempty"`
	ServiceName        string            `json:"service_name,omitempty"`
	IdleTimeout        string            `json:"idle_timeout,omitempty"`
	PingTimeout        string            `json:"ping_timeout,omitempty"`
	PermitWithoutStream bool             `json:"permit_without_stream,omitempty"`
}

// ---------------------------------------------------------------------------
// Multiplex
// ---------------------------------------------------------------------------

// MultiplexConfig — sing-box multiplex configuration.
// https://sing-box.sagernet.org/configuration/shared/multiplex/
type MultiplexConfig struct {
	Enabled        bool          `json:"enabled"`
	Protocol       string        `json:"protocol,omitempty"`
	MaxConnections int           `json:"max_connections,omitempty"`
	MinStreams     int           `json:"min_streams,omitempty"`
	MaxStreams     int           `json:"max_streams,omitempty"`
	Padding        bool          `json:"padding,omitempty"`
	Brutal         *BrutalConfig  `json:"brutal,omitempty"`
}

// BrutalConfig — TCP Brutal congestion control.
type BrutalConfig struct {
	Enabled  bool `json:"enabled"`
	UpMbps   int  `json:"up_mbps,omitempty"`
	DownMbps int  `json:"down_mbps,omitempty"`
}

// ---------------------------------------------------------------------------
// Outbound
// ---------------------------------------------------------------------------

// OutboundConfig — sing-box outbound configuration.
// https://sing-box.sagernet.org/configuration/outbound/
type OutboundConfig struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`

	// Proxy outbounds
	Server            string `json:"server,omitempty"`
	ServerPort        int    `json:"server_port,omitempty"`
	UUID              string `json:"uuid,omitempty"`
	Password          string `json:"password,omitempty"`
	Username          string `json:"username,omitempty"`
	Flow              string `json:"flow,omitempty"`
	Method            string `json:"method,omitempty"`
	AlterID           int    `json:"alterId,omitempty"`
	PacketEncoding   string `json:"packet_encoding,omitempty"`
	Network          string `json:"network,omitempty"`
	CongestionControl string `json:"congestion_control,omitempty"`
	UpMbps           int    `json:"up_mbps,omitempty"`
	DownMbps         int    `json:"down_mbps,omitempty"`

	// TLS & Transport
	TLS       *TLSConfig            `json:"tls,omitempty"`
	Transport *V2RayTransportConfig  `json:"transport,omitempty"`
	Multiplex *MultiplexConfig       `json:"multiplex,omitempty"`

	// Selector / URLTest
	Outbounds                []string `json:"outbounds,omitempty"`
	Default                  string   `json:"default,omitempty"`
	URL                      string   `json:"url,omitempty"`
	Interval                 string   `json:"interval,omitempty"`
	Tolerance                int      `json:"tolerance,omitempty"`
	InterruptExistConnections bool    `json:"interrupt_exist_connections,omitempty"`

	// Dial Fields
	BindInterface  string `json:"bind_interface,omitempty"`
	RoutingMark    int    `json:"routing_mark,omitempty"`
	DomainResolver string `json:"domain_resolver,omitempty"`
}

// ---------------------------------------------------------------------------
// Route
// ---------------------------------------------------------------------------

// RouteConfig — sing-box route configuration.
// https://sing-box.sagernet.org/configuration/route/
type RouteConfig struct {
	Rules               []RouteRule  `json:"rules,omitempty"`
	Final               string       `json:"final,omitempty"`
	AutoDetectInterface bool         `json:"auto_detect_interface,omitempty"`
	OverrideAndroidVPN  bool         `json:"override_android_vpn,omitempty"`
	DefaultInterface    string       `json:"default_interface,omitempty"`
	DefaultMark         int          `json:"default_mark,omitempty"`
	FindProcess         bool         `json:"find_process,omitempty"`
}

// RouteRule — sing-box route rule.
// https://sing-box.sagernet.org/configuration/route/rule/
type RouteRule struct {
	Inbound        []string `json:"inbound,omitempty"`
	IPVersion      int     `json:"ip_version,omitempty"`
	AuthUser       []string `json:"auth_user,omitempty"`
	Protocol       []string `json:"protocol,omitempty"`
	Network        []string `json:"network,omitempty"`
	Domain         []string `json:"domain,omitempty"`
	DomainSuffix   []string `json:"domain_suffix,omitempty"`
	DomainKeyword  []string `json:"domain_keyword,omitempty"`
	DomainRegex    []string `json:"domain_regex,omitempty"`
	Geosite        []string `json:"geosite,omitempty"`
	SourceGeoIP    []string `json:"source_geoip,omitempty"`
	GeoIP          []string `json:"geoip,omitempty"`
	SourceIPCIDR   []string `json:"source_ip_cidr,omitempty"`
	IPCIDR         []string `json:"ip_cidr,omitempty"`
	IPIsPrivate    bool     `json:"ip_is_private,omitempty"`
	SourcePort     []int    `json:"source_port,omitempty"`
	SourcePortRange []string `json:"source_port_range,omitempty"`
	Port           []int    `json:"port,omitempty"`
	PortRange      []string `json:"port_range,omitempty"`
	ProcessName    []string `json:"process_name,omitempty"`
	RuleSet        []string `json:"rule_set,omitempty"`
	Invert         bool     `json:"invert,omitempty"`

	Action    string `json:"action,omitempty"`
	Outbound  string `json:"outbound,omitempty"`

	// Logical rule
	Type string      `json:"type,omitempty"`
	Mode string      `json:"mode,omitempty"`
	Rules []RouteRule `json:"rules,omitempty"`
}

// ---------------------------------------------------------------------------
// Experimental
// ---------------------------------------------------------------------------

// ExperimentalConfig — sing-box experimental configuration.
// https://sing-box.sagernet.org/configuration/experimental/
type ExperimentalConfig struct {
	CacheFile *CacheFileConfig `json:"cache_file,omitempty"`
	ClashAPI  *ClashAPIConfig  `json:"clash_api,omitempty"`
	V2RayAPI  *V2RayAPIConfig  `json:"v2ray_api,omitempty"`
}

// CacheFileConfig — persistent cache.
type CacheFileConfig struct {
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path,omitempty"`
	StoreFakeIP bool   `json:"store_fakeip,omitempty"`
}

// ClashAPIConfig — Clash RESTful API.
type ClashAPIConfig struct {
	ExternalController          string   `json:"external_controller,omitempty"`
	ExternalUI                  string   `json:"external_ui,omitempty"`
	ExternalUIDownloadURL       string   `json:"external_ui_download_url,omitempty"`
	ExternalUIDownloadDetour    string   `json:"external_ui_download_detour,omitempty"`
	Secret                      string   `json:"secret,omitempty"`
	DefaultMode                 string   `json:"default_mode,omitempty"`
	AccessControlAllowOrigin    []string `json:"access_control_allow_origin,omitempty"`
	AccessControlAllowPrivateNetwork bool `json:"access_control_allow_private_network,omitempty"`
}

// V2RayAPIConfig — V2Ray gRPC API for traffic statistics.
// https://sing-box.sagernet.org/configuration/experimental/v2ray-api/
type V2RayAPIConfig struct {
	Listen string       `json:"listen,omitempty"`
	Stats  *V2RayStats  `json:"stats,omitempty"`
}

// V2RayStats — traffic statistics settings.
type V2RayStats struct {
	Enabled   bool     `json:"enabled"`
	Inbounds  []string `json:"inbounds,omitempty"`
	Outbounds []string `json:"outbounds,omitempty"`
	Users     []string `json:"users,omitempty"`
}

// ===========================================================================
// GenerateServiceConfig
// ===========================================================================

// GenerateServiceConfig generates a sing-box inbound service configuration.
func (g *Generator) GenerateServiceConfig(spec common.ServiceConfigSpec) ([]byte, error) {
	inbound := InboundConfig{
		Type:                   singboxInboundType(spec.Protocol),
		Tag:                    fmt.Sprintf("inbound-%d", spec.ServiceID),
		Listen:                 spec.ListenHost,
		ListenPort:             spec.ListenPort,
		Sniff:                  true,
		SniffOverrideDestination: true,
	}

	// Build protocol-specific inbound fields
	switch spec.Protocol {
	case domain.ProtocolVLESS:
		for _, u := range spec.Users {
			user := InboundUser{Name: u.Email, UUID: u.UUID}
			if flow, ok := spec.Extra["vless_flow"].(string); ok && flow != "" {
				user.Flow = flow
			}
			inbound.Users = append(inbound.Users, user)
		}

	case domain.ProtocolVMess:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:    u.Email,
				UUID:    u.UUID,
				AlterID: 0,
			})
		}

	case domain.ProtocolTrojan:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:     u.Email,
				Password: u.Password,
			})
		}

	case domain.ProtocolShadowsocks, domain.ProtocolShadowsocks2022:
		method := extraString(spec.Extra, "ss_method")
		inbound.Method = method
		if pw, ok := spec.Extra["ss_password"].(string); ok {
			inbound.Password = pw
		}
		if n, ok := spec.Extra["ss_network"].(string); ok {
			inbound.Network = n
		}
		// Multi-user (2022-blake3-*): use users[] with individual passwords
		if strings.HasPrefix(method, "2022-blake3-") && len(spec.Users) > 0 {
			for _, u := range spec.Users {
				inbound.Users = append(inbound.Users, InboundUser{
					Name:     u.Email,
					Password: u.Password,
				})
			}
		}

	case domain.ProtocolHysteria2:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:     u.Email,
				Password: u.Password,
			})
		}
		if up, ok := spec.Extra["hysteria2_up"].(string); ok {
			inbound.UpMbps = parseMbps(up)
		}
		if down, ok := spec.Extra["hysteria2_down"].(string); ok {
			inbound.DownMbps = parseMbps(down)
		}
		if masq, ok := spec.Extra["hysteria2_masquerade"]; ok {
			inbound.Masquerade = masq
		}

	case domain.ProtocolTUIC:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:     u.Email,
				UUID:     u.UUID,
				Password: u.Password,
			})
		}
		inbound.CongestionControl = "bbr"
		if cc, ok := spec.Extra["tuic_congestion_control"].(string); ok && cc != "" {
			inbound.CongestionControl = cc
		}
		if at, ok := spec.Extra["tuic_auth_timeout"].(string); ok && at != "" {
			inbound.AuthTimeout = at
		}
		if mit, ok := spec.Extra["tuic_max_idle_time"].(string); ok && mit != "" {
			inbound.MaxIdleTime = mit
		}

	case domain.ProtocolMixed, domain.ProtocolHTTP, domain.ProtocolSocks, domain.ProtocolNaive:
		// multifunctional & auth-based inbounds store username/password per user
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:     u.Email,
				Username: u.AuthKey,
				Password: u.Password,
			})
		}
		// naive requires network=tcp
		if spec.Protocol == domain.ProtocolNaive {
			inbound.Network = "tcp"
		}
		if n, ok := spec.Extra["net_network"].(string); ok && n != "" {
			inbound.Network = n
		}

	case domain.ProtocolAnyTLS, domain.ProtocolShadowTLS:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:     u.Email,
				Password: u.Password,
			})
		}
		if v, ok := spec.Extra["protocol_version"].(float64); ok && int(v) > 0 {
			inbound.Version = int(v)
		} else if v, ok := spec.Extra["shadowtls_version"].(float64); ok && int(v) > 0 {
			inbound.Version = int(v)
		}

	case domain.ProtocolHysteria:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:     u.Email,
				Password: u.Password,
			})
		}
		if up, ok := spec.Extra["hysteria_up"].(string); ok {
			inbound.UpMbps = parseMbps(up)
		}
		if down, ok := spec.Extra["hysteria_down"].(string); ok {
			inbound.DownMbps = parseMbps(down)
		}
		if obfs, ok := spec.Extra["hysteria_obfs"].(string); ok && obfs != "" {
			inbound.Obfs = obfs
		}

	case domain.ProtocolSnell:
		for _, u := range spec.Users {
			inbound.Users = append(inbound.Users, InboundUser{
				Name:    u.Email,
				UserKey: u.AuthKey,
			})
		}
		if v, ok := spec.Extra["snell_version"].(float64); ok && int(v) > 0 {
			inbound.Version = int(v)
		}
		if psk, ok := spec.Extra["snell_psk"].(string); ok && psk != "" {
			inbound.PSK = psk
		}
		if obfs, ok := spec.Extra["snell_obfs"].(string); ok && obfs != "" {
			inbound.Obfs = obfs
		}
		if host, ok := spec.Extra["snell_obfs_host"].(string); ok && host != "" {
			inbound.ObfsHost = host
		}

	case domain.ProtocolWireGuard:
		if key, ok := spec.Extra["wireguard_private_key"].(string); ok && key != "" {
			inbound.PrivateKey = key
		}
		if addr, ok := spec.Extra["wireguard_address"].(string); ok && addr != "" {
			inbound.LocalAddress = strings.Split(addr, ",")
		}
	}

	// TLS
	inbound.TLS = buildInboundTLS(spec)

	// Transport
	inbound.Transport = buildTransport(spec)

	// Multiplex
	if mp, ok := spec.Extra["multiplex_enabled"].(bool); ok && mp {
		mc := &MultiplexConfig{Enabled: true}
		if proto, ok := spec.Extra["multiplex_protocol"].(string); ok {
			mc.Protocol = proto
		}
		inbound.Multiplex = mc
	}

	// NTP (optional, enabled via Extra)
	var ntpCfg *NTPConfig
	if ntpEnabled, ok := spec.Extra["ntp_enabled"].(bool); ok && ntpEnabled {
		ntpCfg = &NTPConfig{
			Enabled:  true,
			Server:   "time.google.com",
			Interval: "30m",
			TimeSync: true,
		}
		if s, ok := spec.Extra["ntp_server"].(string); ok && s != "" {
			ntpCfg.Server = s
		}
		if i, ok := spec.Extra["ntp_interval"].(string); ok && i != "" {
			ntpCfg.Interval = i
		}
	}

	config := SingBoxConfig{
		Log: &LogConfig{
			Level:     "info",
			Timestamp: true,
		},
		Inbounds: []InboundConfig{inbound},
		Outbounds: []OutboundConfig{
			{Type: "direct", Tag: "direct"},
			{Type: "block", Tag: "block"},
		},
		Route: &RouteConfig{
			AutoDetectInterface: true,
			Final:               "direct",
		},
		Experimental: &ExperimentalConfig{
			ClashAPI: &ClashAPIConfig{
				ExternalController: "127.0.0.1:10086",
			},
		},
		NTP: ntpCfg,
	}

	return json.MarshalIndent(config, "", "  ")
}

// ===========================================================================
// GenerateClientConfig
// ===========================================================================

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
		ob := OutboundConfig{
			Type:       singboxOutboundType(node.Protocol),
			Tag:        fmt.Sprintf("proxy-%d", i),
			Server:     node.Host,
			ServerPort: node.Port,
		}

		// Auth
		switch node.Protocol {
		case domain.ProtocolVLESS:
			ob.UUID = node.Auth.UUID
			if flow, ok := getExtra(node, "vless_flow"); ok {
				ob.Flow = flow
			}
		case domain.ProtocolVMess:
			ob.UUID = node.Auth.UUID
			ob.AlterID = 0
			if pe, ok := node.Extra["packet_encoding"].(string); ok && pe != "" {
				ob.PacketEncoding = pe
			}
		case domain.ProtocolTrojan:
			ob.Password = node.Auth.Password
		case domain.ProtocolShadowsocks, domain.ProtocolShadowsocks2022:
			ob.Password = node.Auth.Password
			if m, ok := getExtra(node, "ss_method"); ok {
				ob.Method = m
			}
			if pe, ok := node.Extra["packet_encoding"].(string); ok && pe != "" {
				ob.PacketEncoding = pe
			}
		case domain.ProtocolHysteria2:
			ob.Password = node.Auth.Password
		if up, ok := getExtra(node, "hysteria2_up"); ok {
			ob.UpMbps = parseMbps(up)
		}
		if down, ok := getExtra(node, "hysteria2_down"); ok {
			ob.DownMbps = parseMbps(down)
		}
		case domain.ProtocolTUIC:
			ob.UUID = node.Auth.UUID
			ob.Password = node.Auth.Password
			ob.CongestionControl = "bbr"
			if cc, ok := getExtra(node, "tuic_congestion_control"); ok && cc != "" {
				ob.CongestionControl = cc
			}
		case domain.ProtocolHTTP, domain.ProtocolSocks, domain.ProtocolMixed, domain.ProtocolNaive:
			// these are mostly inbound protocols, but sing-box can also dial
			// http/socks outbounds; username/password carried via AuthKey/Password
			ob.Username = node.Auth.AuthKey
			ob.Password = node.Auth.Password
		}

		// TLS
		if node.TLS.Enabled {
			ob.TLS = &TLSConfig{
				Enabled:    true,
				ServerName: node.TLS.ServerName,
				ALPN:       node.TLS.ALPN,
			}
			if node.TLS.Fingerprint != "" {
				ob.TLS.UTLS = &UTLSConfig{
					Enabled:     true,
					Fingerprint: node.TLS.Fingerprint,
				}
			}
			// ECH (Encrypted Client Hello)
			if node.TLS.ECH && node.TLS.ECHConfig != "" {
				ob.TLS.ECHConfig = node.TLS.ECHConfig
			}
		}

		// Multiplex
		if mp, ok := node.Extra["multiplex_enabled"].(bool); ok && mp {
			mc := &MultiplexConfig{Enabled: true}
			if proto, ok := node.Extra["multiplex_protocol"].(string); ok && proto != "" {
				mc.Protocol = proto
			}
			if mc, ok := node.Extra["multiplex_max_connections"].(float64); ok && mc > 0 {
				ob.Multiplex.MaxConnections = int(mc)
			}
			ob.Multiplex = mc
		}

		// Transport
		if node.Transport != "" && node.Transport != domain.TransportTCP {
			ob.Transport = buildClientTransport(node)
		}

		config.Outbounds = append(config.Outbounds, ob)
	}

	// Default outbounds
	config.Outbounds = append(config.Outbounds,
		OutboundConfig{Type: "direct", Tag: "direct"},
		OutboundConfig{Type: "block", Tag: "block"},
	)

	// Route
	route := &RouteConfig{
		AutoDetectInterface: true,
		Final:               "direct",
	}

	for _, rule := range spec.RouteRules {
		rr := RouteRule{Outbound: rule.OutboundTag}
		switch rule.MatchType {
		case "domain":
			rr.Domain = []string{rule.MatchValue}
		case "domain_suffix":
			rr.DomainSuffix = []string{rule.MatchValue}
		case "domain_keyword":
			rr.DomainKeyword = []string{rule.MatchValue}
		case "ip", "cidr":
			rr.IPCIDR = []string{rule.MatchValue}
		}
		route.Rules = append(route.Rules, rr)
	}
	config.Route = route

	// DNS
	if len(spec.DNSServers) > 0 {
		config.DNS = buildClientDNS(spec.DNSServers)
	}

	// TUN
	if spec.TunEnabled {
		config.Inbounds = append(config.Inbounds, InboundConfig{
			Type: "tun",
			Tag:  "tun-in",
		})
	}

	return json.MarshalIndent(config, "", "  ")
}

// ===========================================================================
// ValidateConfig
// ===========================================================================

// ValidateConfig validates a sing-box configuration.
func (g *Generator) ValidateConfig(content []byte) error {
	var config SingBoxConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(config.Inbounds) == 0 && len(config.Outbounds) == 0 {
		return fmt.Errorf("no inbounds or outbounds defined")
	}

	// Validate each inbound has required fields
	for i, ib := range config.Inbounds {
		if ib.Type == "" {
			return fmt.Errorf("inbound[%d]: type is required", i)
		}
		if ib.Tag == "" {
			return fmt.Errorf("inbound[%d]: tag is required", i)
		}
		switch ib.Type {
		case "vless", "vmess":
			if len(ib.Users) == 0 {
				return fmt.Errorf("inbound[%s]: users is required for %s", ib.Tag, ib.Type)
			}
			for _, u := range ib.Users {
				if u.UUID == "" {
					return fmt.Errorf("inbound[%s]: user %s missing uuid", ib.Tag, u.Name)
				}
			}
		case "trojan":
			if len(ib.Users) == 0 {
				return fmt.Errorf("inbound[%s]: users is required for trojan", ib.Tag)
			}
		case "shadowsocks":
			if ib.Method == "" {
				return fmt.Errorf("inbound[%s]: method is required for shadowsocks", ib.Tag)
			}
		case "hysteria2":
			if len(ib.Users) == 0 {
				return fmt.Errorf("inbound[%s]: users is required for hysteria2", ib.Tag)
			}
		case "tuic":
			if len(ib.Users) == 0 {
				return fmt.Errorf("inbound[%s]: users is required for tuic", ib.Tag)
			}
		}
	}

	// Validate route rules reference existing outbounds
	outboundTags := make(map[string]bool)
	for _, ob := range config.Outbounds {
		outboundTags[ob.Tag] = true
	}
	if config.Route != nil {
		for i, rule := range config.Route.Rules {
			if rule.Outbound != "" && !outboundTags[rule.Outbound] {
				return fmt.Errorf("route rule[%d]: outbound %q not found", i, rule.Outbound)
			}
		}
		if config.Route.Final != "" && !outboundTags[config.Route.Final] {
			return fmt.Errorf("route.final: outbound %q not found", config.Route.Final)
		}
	}

	return nil
}

// ===========================================================================
// Helpers
// ===========================================================================

// singboxInboundType maps domain.ProtocolType to sing-box inbound type.
func singboxInboundType(p domain.ProtocolType) string {
	switch p {
	case domain.ProtocolShadowsocks2022:
		return "shadowsocks"
	default:
		return string(p)
	}
}

// singboxOutboundType maps domain.ProtocolType to sing-box outbound type.
func singboxOutboundType(p domain.ProtocolType) string {
	switch p {
	case domain.ProtocolShadowsocks2022:
		return "shadowsocks"
	default:
		return string(p)
	}
}

// buildInboundTLS builds TLS config for a service spec.
func buildInboundTLS(spec common.ServiceConfigSpec) *TLSConfig {
	if spec.TLSMode == domain.TLSModeNone {
		return nil
	}

	tls := &TLSConfig{Enabled: true}

	if len(spec.Endpoints) > 0 && spec.Endpoints[0].ServerName != "" {
		tls.ServerName = spec.Endpoints[0].ServerName
	}
	if spec.CertPath != "" {
		tls.CertificatePath = spec.CertPath
		tls.KeyPath = spec.KeyPath
	}
	if alpn, ok := spec.Extra["tls_alpn"].([]string); ok {
		tls.ALPN = alpn
	}
	if v, ok := spec.Extra["tls_min_version"].(string); ok {
		tls.MinVersion = v
	}
	if v, ok := spec.Extra["tls_max_version"].(string); ok {
		tls.MaxVersion = v
	}

	// Reality
	if spec.TLSMode == domain.TLSModeReality {
		tls.Reality = &RealityConfig{Enabled: true}
		if hk, ok := spec.Extra["reality_handshake_server"].(string); ok {
			tls.Reality.Handshake = &HandshakeConfig{
				Server:     hk,
				ServerPort: extraInt(spec.Extra, "reality_handshake_port"),
			}
		}
		tls.Reality.PrivateKey = extraString(spec.Extra, "reality_private_key")
		tls.Reality.PublicKey = extraString(spec.Extra, "reality_public_key")
		// sing-box uses string array for short_id
		if sid, ok := spec.Extra["reality_short_id"].(string); ok && sid != "" {
			tls.Reality.ShortID = []string{sid}
		}
		if mtd, ok := spec.Extra["reality_max_time_difference"].(string); ok {
			tls.Reality.MaxTimeDifference = mtd
		}
	}

	// uTLS
	if spec.TLSMode == domain.TLSModeUTLS {
		tls.UTLS = &UTLSConfig{
			Enabled:     true,
			Fingerprint: extraString(spec.Extra, "utls_fingerprint", "tls_fingerprint"),
		}
	}

	return tls
}

// buildTransport builds V2Ray transport config from a service spec.
func buildTransport(spec common.ServiceConfigSpec) *V2RayTransportConfig {
	if spec.Transport == "" || spec.Transport == domain.TransportTCP {
		return nil
	}

	t := &V2RayTransportConfig{Type: string(spec.Transport)}
	switch spec.Transport {
	case domain.TransportWS:
		t.Path = extraString(spec.Extra, "ws_path")
		if t.Path == "" {
			t.Path = "/"
		}
		if headers, ok := spec.Extra["ws_headers"].(map[string]any); ok {
			t.Headers = toStringMap(headers)
		}
		t.MaxEarlyData = extraInt(spec.Extra, "ws_max_early_data")
		t.EarlyDataHeaderName = extraString(spec.Extra, "ws_early_data_header_name")
	case domain.TransportGRPC:
		t.ServiceName = extraString(spec.Extra, "grpc_service_name")
		t.IdleTimeout = extraString(spec.Extra, "grpc_idle_timeout")
		t.PingTimeout = extraString(spec.Extra, "grpc_ping_timeout")
	case domain.TransportHTTP2:
		// sing-box uses "http" type for HTTP/2 transport
		t.Type = "http"
		t.Host = extraString(spec.Extra, "http_host")
		t.Path = extraString(spec.Extra, "http_path")
		if t.Path == "" {
			t.Path = "/"
		}
		if headers, ok := spec.Extra["http_headers"].(map[string]any); ok {
			t.Headers = toStringMap(headers)
		}
	case domain.TransportHTTPUpgrade:
		t.Type = "httpupgrade"
		t.Host = extraString(spec.Extra, "httpupgrade_host")
		t.Path = extraString(spec.Extra, "httpupgrade_path")
		if t.Path == "" {
			t.Path = "/"
		}
	case domain.TransportKCP, domain.TransportMKCP:
		// sing-box uses "kcp" type for KCP transport
		t.Type = "kcp"
		t.ServiceName = extraString(spec.Extra, "kcp_seed")
	case domain.TransportQUIC:
		// QUIC transport has no extra fields
	}
	return t
}

// buildClientTransport builds V2Ray transport config from a client node config.
// Uses node.Extra for protocol/transport-specific parameters.
func buildClientTransport(node common.NodeConfig) *V2RayTransportConfig {
	t := &V2RayTransportConfig{Type: string(node.Transport)}
	switch node.Transport {
	case domain.TransportWS:
		path := nodeExtraString(node, "ws_path")
		if path == "" {
			path = "/"
		}
		t.Path = path
		if headers, ok := node.Extra["ws_headers"].(map[string]any); ok {
			t.Headers = toStringMap(headers)
		}
		t.MaxEarlyData = nodeExtraInt(node, "ws_max_early_data")
		if edn := nodeExtraString(node, "ws_early_data_header_name"); edn != "" {
			t.EarlyDataHeaderName = edn
		}
	case domain.TransportGRPC:
		t.ServiceName = nodeExtraString(node, "grpc_service_name")
		t.IdleTimeout = nodeExtraString(node, "grpc_idle_timeout")
		t.PingTimeout = nodeExtraString(node, "grpc_ping_timeout")
	case domain.TransportHTTP2:
		// sing-box uses "http" type for HTTP/2 transport
		t.Type = "http"
		t.Host = nodeExtraString(node, "http_host")
		path := nodeExtraString(node, "http_path")
		if path == "" {
			path = "/"
		}
		t.Path = path
		if headers, ok := node.Extra["http_headers"].(map[string]any); ok {
			t.Headers = toStringMap(headers)
		}
	case domain.TransportHTTPUpgrade:
		t.Type = "httpupgrade"
		t.Host = nodeExtraString(node, "httpupgrade_host")
		path := nodeExtraString(node, "httpupgrade_path")
		if path == "" {
			path = "/"
		}
		t.Path = path
	case domain.TransportKCP, domain.TransportMKCP:
		// sing-box uses "kcp" type for KCP transport
		t.Type = "kcp"
		t.ServiceName = nodeExtraString(node, "kcp_seed")
	case domain.TransportQUIC:
		// QUIC transport has no extra fields
	}
	return t
}

// buildClientDNS builds DNS config from a list of server addresses.
func buildClientDNS(servers []string) *DNSConfig {
	dns := &DNSConfig{}
	for i, s := range servers {
		serverType := "udp"
		addr := s
		if strings.HasPrefix(s, "https://") {
			serverType = "https"
		} else if strings.HasPrefix(s, "tls://") {
			serverType = "tls"
			addr = strings.TrimPrefix(s, "tls://")
		} else if strings.HasPrefix(s, "quic://") {
			serverType = "quic"
			addr = strings.TrimPrefix(s, "quic://")
		} else if s == "local" {
			serverType = "local"
			addr = ""
		}
		dns.Servers = append(dns.Servers, DNSServer{
			Type:    serverType,
			Tag:     fmt.Sprintf("dns-%d", i),
			Address: addr,
		})
	}
	return dns
}

// getExtra retrieves a string value from node's Extra map.
func getExtra(node common.NodeConfig, key string) (string, bool) {
	if node.Extra == nil {
		return "", false
	}
	if v, ok := node.Extra[key].(string); ok && v != "" {
		return v, true
	}
	return "", false
}

// nodeExtraString returns the first non-empty string value among the given keys
// from the node's Extra map.
func nodeExtraString(node common.NodeConfig, keys ...string) string {
	if node.Extra == nil {
		return ""
	}
	return extraString(node.Extra, keys...)
}

// nodeExtraInt returns the first numeric value among the given keys
// from the node's Extra map.
func nodeExtraInt(node common.NodeConfig, keys ...string) int {
	if node.Extra == nil {
		return 0
	}
	return extraInt(node.Extra, keys...)
}

// extraString returns the first non-empty string value among the given keys.
func extraString(extra map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := extra[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extraInt returns the first numeric value among the given keys.
func extraInt(extra map[string]any, keys ...string) int {
	for _, k := range keys {
		switch v := extra[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
	}
	return 0
}

// parseMbps parses a bandwidth string like "100 Mbps" or "50" into an int (Mbps).
func parseMbps(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "mbps")
	s = strings.TrimSuffix(s, "mb")
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}

// toStringMap converts map[string]any to map[string]string.
func toStringMap(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
