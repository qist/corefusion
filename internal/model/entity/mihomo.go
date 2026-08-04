package entity

import "time"

// MihomoConfig is the main mihomo configuration.
type MihomoConfig struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	ClientType  string    `gorm:"size:50;default:'clash'" json:"client_type"`  // clash, singbox, v2ray
	Format      string    `gorm:"size:50;not null;default:'mihomo_yaml'" json:"format"` // mihomo_yaml, singbox_json, xray_json
	Description string    `gorm:"size:500" json:"description"`
	Platform    string    `gorm:"size:50;default:''" json:"platform"`   // win, mac, android, ios, universal (empty = universal)
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	ConfigJSON  string    `gorm:"type:longtext" json:"config_json"` // Complete JSON for import/export
	PlanIDs     Int64List `gorm:"type:longtext;serializer:json" json:"plan_ids"`  // Bound plan IDs; empty = universal (visible to all)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (MihomoConfig) TableName() string { return "mihomo_configs" }

// MihomoGeneral stores general settings for a mihomo config.
type MihomoGeneral struct {
	ID                 int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID           int64  `gorm:"index;not null" json:"config_id"`
	MixedPort          int    `gorm:"default:0" json:"mixed_port"`
	Port               int    `gorm:"default:0" json:"port"`
	SocksPort          int    `gorm:"default:0" json:"socks_port"`
	RedirPort          int    `gorm:"default:0" json:"redir_port"`
	TProxyPort         int    `gorm:"default:0" json:"tproxy_port"`
	AllowLan           bool   `gorm:"default:true" json:"allow_lan"`
	BindAddress        string `gorm:"size:255;default:'*'" json:"bind_address"`
	Mode               string `gorm:"size:20;default:'rule'" json:"mode"`
	LogLevel           string `gorm:"size:20;default:'info'" json:"log_level"`
	IPv6               bool   `gorm:"default:true" json:"ipv6"`
	FindProcessMode    string `gorm:"size:20;default:'strict'" json:"find_process_mode"`
	Authentication     string `gorm:"type:longtext" json:"authentication"`
	SkipAuthPrefixes   string `gorm:"type:longtext" json:"skip_auth_prefixes"`
	LanAllowedIPs      string `gorm:"type:longtext" json:"lan_allowed_ips"`
	LanDisallowedIPs   string `gorm:"type:longtext" json:"lan_disallowed_ips"`
	StoreSelected      bool   `gorm:"default:false" json:"store_selected"`
	StoreFakeIP        bool   `gorm:"default:true" json:"store_fake_ip"`
	InterfaceName      string `gorm:"size:100" json:"interface_name"`
	RoutingMark        int    `gorm:"default:0" json:"routing_mark"`
	TCPConcurrent      bool   `gorm:"default:true" json:"tcp_concurrent"`
	KeepAliveIdle      int    `gorm:"default:0" json:"keep_alive_idle"`
	GeoAutoUpdate      bool   `gorm:"default:false" json:"geo_auto_update"`
	GeoUpdateInterval  int    `gorm:"default:24" json:"geo_update_interval"`
	GeoIPUrl           string `gorm:"size:500" json:"geo_ip_url"`
	GeositeUrl         string `gorm:"size:500" json:"geosite_url"`
	MmdbUrl            string `gorm:"size:500" json:"mmdb_url"`
	QuicDisableGSO     bool   `gorm:"default:false" json:"quic_disable_gso"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (MihomoGeneral) TableName() string { return "mihomo_general" }

// MihomoDNS stores DNS configuration.
type MihomoDNS struct {
	ID                     int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID               int64  `gorm:"index;not null" json:"config_id"`
	Enable                 bool   `gorm:"default:false" json:"enable"`
	Listen                 string `gorm:"size:50;default:'0.0.0.0:53'" json:"listen"`
	EnhancedMode           string `gorm:"size:20;default:'redir-host'" json:"enhanced_mode"`
	CacheAlgorithm         string `gorm:"size:20;default:'arc'" json:"cache_algorithm"`
	PreferH3               bool   `gorm:"default:false" json:"prefer_h3"`
	IPv6                   bool   `gorm:"default:true" json:"ipv6"`
	UseHosts               bool   `gorm:"default:true" json:"use_hosts"`
	RespectRules           bool   `gorm:"default:false" json:"respect_rules"`
	DefaultNameserver      string `gorm:"type:longtext" json:"default_nameserver"`
	Nameserver             string `gorm:"type:longtext" json:"nameserver"`
	Fallback               string `gorm:"type:longtext" json:"fallback"`
	ProxyServerNameserver  string `gorm:"type:longtext" json:"proxy_server_nameserver"`
	DirectNameserver       string `gorm:"type:longtext" json:"direct_nameserver"`
	FakeIPRange            string `gorm:"size:50;default:'198.18.0.1/16'" json:"fake_ip_range"`
	FakeIPFilterMode       string `gorm:"size:20;default:'blacklist'" json:"fake_ip_filter_mode"`
	FakeIPFilter           string `gorm:"type:longtext" json:"fake_ip_filter"`
	FallbackGeoIP          bool   `gorm:"default:true" json:"fallback_geoip"`
	FallbackGeoIPCode      string `gorm:"size:10;default:'CN'" json:"fallback_geoip_code"`
	FallbackIPCIDR         string `gorm:"type:longtext" json:"fallback_ipcidr"`
	FallbackDomain         string `gorm:"type:longtext" json:"fallback_domain"`
	NameserverPolicy       string `gorm:"type:longtext" json:"nameserver_policy"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

func (MihomoDNS) TableName() string { return "mihomo_dns" }

// MihomoHost stores custom host mappings.
type MihomoHost struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID  int64     `gorm:"index;not null" json:"config_id"`
	Domain    string    `gorm:"size:255;not null" json:"domain"`
	IP        string    `gorm:"size:255;not null" json:"ip"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MihomoHost) TableName() string { return "mihomo_hosts" }

// MihomoTunnel stores tunnel configurations.
type MihomoTunnel struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID  int64     `gorm:"index;not null" json:"config_id"`
	Network   string    `gorm:"type:longtext" json:"network"`   // JSON array ["tcp","udp"]
	Address   string    `gorm:"size:255" json:"address"`
	Target    string    `gorm:"size:255" json:"target"`
	Proxy     string    `gorm:"size:100" json:"proxy"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MihomoTunnel) TableName() string { return "mihomo_tunnels" }

// MihomoSniffer stores sniffer configuration.
type MihomoSniffer struct {
	ID                   int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID             int64  `gorm:"index;not null" json:"config_id"`
	Enable               bool   `gorm:"default:false" json:"enable"`
	ForceDNSMapping      bool   `gorm:"default:false" json:"force_dns_mapping"`
	ParsePureIP          bool   `gorm:"default:false" json:"parse_pure_ip"`
	OverrideDestination  bool   `gorm:"default:false" json:"override_destination"`
	HTTPEnabled          bool   `gorm:"default:true" json:"http_enabled"`
	HTTPPorts            string `gorm:"size:200;default:'80, 8080-8880'" json:"http_ports"`
	HTTPOverride         bool   `gorm:"default:false" json:"http_override"`
	TLSEnabled           bool   `gorm:"default:true" json:"tls_enabled"`
	TLSPorts             string `gorm:"size:200;default:'443, 8443'" json:"tls_ports"`
	QUICEnabled          bool   `gorm:"default:true" json:"quic_enabled"`
	QUICPorts            string `gorm:"size:200;default:'443, 8443'" json:"quic_ports"`
	ForceDomain          string `gorm:"type:longtext" json:"force_domain"`
	SkipDomain           string `gorm:"type:longtext" json:"skip_domain"`
	SkipSrcAddress       string `gorm:"type:longtext" json:"skip_src_address"`
	SkipDstAddress       string `gorm:"type:longtext" json:"skip_dst_address"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (MihomoSniffer) TableName() string { return "mihomo_sniffer" }

// MihomoExternalController stores external controller settings.
type MihomoExternalController struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID  int64     `gorm:"index;not null" json:"config_id"`
	Enable    bool      `gorm:"default:true" json:"enable"`
	Listen    string    `gorm:"size:50;default:'0.0.0.0:9093'" json:"listen"`
	Secret    string    `gorm:"size:255" json:"secret"`
	UIName    string    `gorm:"size:100" json:"ui_name"`
	UIUrl     string    `gorm:"size:500" json:"ui_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MihomoExternalController) TableName() string { return "mihomo_external_controller" }

// MihomoTLS stores TLS configuration.
type MihomoTLS struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID    int64     `gorm:"index;not null" json:"config_id"`
	Enable      bool      `gorm:"default:false" json:"enable"`
	Certificate string    `gorm:"type:longtext" json:"certificate"`
	PrivateKey  string    `gorm:"type:longtext" json:"private_key"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (MihomoTLS) TableName() string { return "mihomo_tls" }

// MihomoTun stores TUN configuration.
type MihomoTun struct {
	ID                      int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID                int64  `gorm:"index;not null" json:"config_id"`
	Enable                  bool   `gorm:"default:false" json:"enable"`
	Stack                   string `gorm:"size:20;default:'system'" json:"stack"`
	Device                  string `gorm:"size:100" json:"device"`
	DNSHijack               string `gorm:"type:longtext" json:"dns_hijack"`
	AutoRoute               bool   `gorm:"default:true" json:"auto_route"`
	AutoDetectInterface     bool   `gorm:"default:true" json:"auto_detect_interface"`
	AutoRedirect            bool   `gorm:"default:false" json:"auto_redirect"`
	MTU                     int    `gorm:"default:0" json:"mtu"`
	StrictRoute             bool   `gorm:"default:false" json:"strict_route"`
	DisableICMPForwarding   bool   `gorm:"default:false" json:"disable_icmp_forwarding"`
	GSO                     bool   `gorm:"default:false" json:"gso"`
	GSOMaxSize              int    `gorm:"default:65536" json:"gso_max_size"`
	Inet6Address            string `gorm:"size:100" json:"inet6_address"`
	UDPTimeout              int    `gorm:"default:300" json:"udp_timeout"`
	EndpointIndependentNAT  bool   `gorm:"default:false" json:"endpoint_independent_nat"`
	RouteAddress            string `gorm:"type:longtext" json:"route_address"`
	RouteExcludeAddress     string `gorm:"type:longtext" json:"route_exclude_address"`
	RouteAddressSet         string `gorm:"type:longtext" json:"route_address_set"`
	RouteExcludeAddressSet  string `gorm:"type:longtext" json:"route_exclude_address_set"`
	IncludeInterface        string `gorm:"type:longtext" json:"include_interface"`
	ExcludeInterface        string `gorm:"type:longtext" json:"exclude_interface"`
	IncludeUID              string `gorm:"type:longtext" json:"include_uid"`
	ExcludeUID              string `gorm:"type:longtext" json:"exclude_uid"`
	IncludeUIDRange         string `gorm:"type:longtext" json:"include_uid_range"`
	ExcludeUIDRange         string `gorm:"type:longtext" json:"exclude_uid_range"`
	IncludeMACAddress       string `gorm:"type:longtext" json:"include_mac_address"`
	ExcludeMACAddress       string `gorm:"type:longtext" json:"exclude_mac_address"`
	IncludeAndroidUser      string `gorm:"type:longtext" json:"include_android_user"`
	IncludePackage          string `gorm:"type:longtext" json:"include_package"`
	ExcludePackage          string `gorm:"type:longtext" json:"exclude_package"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func (MihomoTun) TableName() string { return "mihomo_tun" }

// MihomoProxyGroup stores proxy group configuration.
type MihomoProxyGroup struct {
	ID              int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID        int64  `gorm:"index;not null" json:"config_id"`
	Name            string `gorm:"size:100;not null" json:"name"`
	Type            string `gorm:"size:20;default:'select'" json:"type"`
	Proxies         string `gorm:"type:longtext" json:"proxies"` // JSON array
	Hidden          bool   `gorm:"default:false" json:"hidden"`
	URL             string `gorm:"size:500" json:"url"`
	Interval        int    `gorm:"default:300" json:"interval"`
	Lazy            bool   `gorm:"default:true" json:"lazy"`
	Timeout         int    `gorm:"default:5000" json:"timeout"`
	MaxFailedTimes  int    `gorm:"default:5" json:"max_failed_times"`
	EmptyFallback   string `gorm:"size:20" json:"empty_fallback"`
	DisableUDP      bool   `gorm:"default:false" json:"disable_udp"`
	Filter          string `gorm:"size:500" json:"filter"`
	ExcludeFilter   string `gorm:"size:500" json:"exclude_filter"`
	ExcludeType     string `gorm:"size:200" json:"exclude_type"`
	IncludeAll      bool   `gorm:"default:false" json:"include_all"`
	InterfaceName   string `gorm:"size:100" json:"interface_name"`
	RoutingMark     int    `gorm:"default:0" json:"routing_mark"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (MihomoProxyGroup) TableName() string { return "mihomo_proxy_groups" }

// MihomoRuleProvider stores rule provider configuration.
type MihomoRuleProvider struct {
	ID           int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID     int64  `gorm:"index;not null" json:"config_id"`
	Name         string `gorm:"size:100;not null" json:"name"`
	Type         string `gorm:"size:20;default:'http'" json:"type"`
	Behavior     string `gorm:"size:20;default:'domain'" json:"behavior"`
	Format       string `gorm:"size:20;default:'yaml'" json:"format"`
	URL          string `gorm:"size:500" json:"url"`
	Path         string `gorm:"size:500" json:"path"`
	Proxy        string `gorm:"size:100;default:'DIRECT'" json:"proxy"`
	PathInBundle string `gorm:"size:200" json:"path_in_bundle"`
	Interval     int    `gorm:"default:86400" json:"interval"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (MihomoRuleProvider) TableName() string { return "mihomo_rule_providers" }

// MihomoRule stores routing rules as complete strings.
type MihomoRule struct {
	ID         int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID   int64  `gorm:"index;not null" json:"config_id"`
	RuleString string `gorm:"size:1000;not null" json:"rule_string"` // Complete rule string
	Priority   int    `gorm:"default:0" json:"priority"`
	Enabled    bool   `gorm:"default:true" json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (MihomoRule) TableName() string { return "mihomo_rules" }

// MihomoSpecialOutbound stores special outbound configurations.
type MihomoSpecialOutbound struct {
	ID              int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	ConfigID        int64  `gorm:"index;not null" json:"config_id"`
	Name            string `gorm:"size:100;not null" json:"name"`
	Type            string `gorm:"size:20;not null" json:"type"` // dns, rematch, direct
	InterfaceName   string `gorm:"size:100" json:"interface_name"`
	RoutingMark     int    `gorm:"default:0" json:"routing_mark"`
	TargetRematch   string `gorm:"size:100" json:"target_rematch"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (MihomoSpecialOutbound) TableName() string { return "mihomo_special_outbounds" }
