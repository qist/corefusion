package domain

import "time"

// CoreType represents the proxy core type.
type CoreType string

const (
	CoreTypeXray      CoreType = "xray"
	CoreTypeSingBox   CoreType = "singbox"
	CoreTypeHysteria2 CoreType = "hysteria2"
	CoreTypeTUIC      CoreType = "tuic"
)

// SupportsHotApply 报告该核心的配置改动能否通过运行时 API 热下发（无需重启进程）。
//
// xray：gRPC HandlerService 支持 AlterInbound / AddInbound / RemoveInbound 与 routing
// 更新，面板的入站/出站/路由编辑器在写库后一定会调用 xray/control/* 把改动下发到
// 运行中的进程，因此这些改动不需要重启核心（重启会断开该核心的全部在线连接）。
// sing-box：v2ray API 只注册了 StatsService（没有 AlterInbound），入站/用户变更只能
// 「重新生成配置 + 重启核心」才能生效。
func (c CoreType) SupportsHotApply() bool {
	return c == CoreTypeXray
}

// ProtocolType represents the proxy protocol.
type ProtocolType string

const (
	ProtocolVLESS           ProtocolType = "vless"
	ProtocolVMess           ProtocolType = "vmess"
	ProtocolTrojan          ProtocolType = "trojan"
	ProtocolShadowsocks     ProtocolType = "shadowsocks"
	ProtocolShadowsocks2022 ProtocolType = "shadowsocks2022"
	ProtocolHysteria2       ProtocolType = "hysteria2"
	ProtocolTUIC            ProtocolType = "tuic"
	ProtocolDokodemo        ProtocolType = "dokodemo-door"
	ProtocolTUN             ProtocolType = "tun"
	// sing-box specific multi-purpose and auth inbounds
	ProtocolMixed ProtocolType = "mixed"
	ProtocolHTTP  ProtocolType = "http"
	ProtocolSocks ProtocolType = "socks"
	ProtocolNaive ProtocolType = "naive"
	// sing-box advanced inbounds
	ProtocolAnyTLS    ProtocolType = "anytls"
	ProtocolShadowTLS ProtocolType = "shadowtls"
	ProtocolHysteria  ProtocolType = "hysteria"
	ProtocolWireGuard ProtocolType = "wireguard"
	ProtocolSnell     ProtocolType = "snell"
)

// TransportType represents the transport layer.
type TransportType string

const (
	TransportTCP         TransportType = "tcp"
	TransportWS          TransportType = "ws"
	TransportGRPC        TransportType = "grpc"
	TransportHTTP2       TransportType = "http2"
	TransportQUIC        TransportType = "quic"
	TransportReality     TransportType = "reality"
	TransportHTTPUpgrade TransportType = "httpupgrade"
	// Additional transports supported by xray/sing-box
	TransportKCP   TransportType = "kcp"
	TransportMKCP  TransportType = "mkcp"
	TransportH2    TransportType = "h2"
	TransportXHTTP TransportType = "xhttp"
)

// TLSMode represents the TLS mode.
type TLSMode string

const (
	TLSModeNone    TLSMode = "none"
	TLSModeTLS     TLSMode = "tls"
	TLSModeReality TLSMode = "reality"
	TLSModeUTLS    TLSMode = "utls"
)

// EndpointType represents the service endpoint type.
type EndpointType string

const (
	EndpointTypeDomain EndpointType = "domain"
	EndpointTypeIPv4   EndpointType = "ipv4"
	EndpointTypeIPv6   EndpointType = "ipv6"
)

// RuntimeApplyMode represents how a config change is applied.
type RuntimeApplyMode string

const (
	RuntimeApplyModeHotReload RuntimeApplyMode = "hot_reload"
	RuntimeApplyModeReload    RuntimeApplyMode = "reload"
	RuntimeApplyModeRestart   RuntimeApplyMode = "restart"
)

// RuntimeApplyState represents the state of a runtime apply job.
type RuntimeApplyState string

const (
	RuntimeApplyStateCreated             RuntimeApplyState = "created"
	RuntimeApplyStateSnapshotting        RuntimeApplyState = "snapshotting"
	RuntimeApplyStateBackingUpActive     RuntimeApplyState = "backing_up_active"
	RuntimeApplyStateRestoringOriginal   RuntimeApplyState = "restoring_original"
	RuntimeApplyStateRendering           RuntimeApplyState = "rendering"
	RuntimeApplyStateValidating          RuntimeApplyState = "validating"
	RuntimeApplyStateStaging             RuntimeApplyState = "staging"
	RuntimeApplyStateApplyingCertificate RuntimeApplyState = "applying_certificate"
	RuntimeApplyStateApplyingProxy       RuntimeApplyState = "applying_proxy"
	RuntimeApplyStateApplyingCore        RuntimeApplyState = "applying_core"
	RuntimeApplyStateHealthChecking      RuntimeApplyState = "health_checking"
	RuntimeApplyStateSucceeded           RuntimeApplyState = "succeeded"
	RuntimeApplyStateFailed              RuntimeApplyState = "failed"
	RuntimeApplyStateRollingBack         RuntimeApplyState = "rolling_back"
	RuntimeApplyStateRolledBack          RuntimeApplyState = "rolled_back"
	RuntimeApplyStateNeedsIntervention   RuntimeApplyState = "needs_intervention"
)

// QuotaState represents the unified quota state for both generic and subscription quotas.
type QuotaState string

const (
	QuotaStateNormal        QuotaState = "normal"
	QuotaStateNearLimit     QuotaState = "near_limit"
	QuotaStateOverLimitSoft QuotaState = "over_limit_soft"
	QuotaStateOverLimitHard QuotaState = "over_limit_hard"
	QuotaStateSuspended     QuotaState = "suspended"
	QuotaStateRestored      QuotaState = "restored"
	QuotaStateExpiredDays   QuotaState = "expired_days"
)

// AccessControlState represents the state of an IP's access control.
type AccessControlState string

const (
	AccessControlStateAllowed     AccessControlState = "allowed"
	AccessControlStateObserved    AccessControlState = "observed"
	AccessControlStateRateLimited AccessControlState = "rate_limited"
	AccessControlStateTempBanned  AccessControlState = "temp_banned"
	AccessControlStatePermBanned  AccessControlState = "perm_banned"
	AccessControlStateCooling     AccessControlState = "cooling"
)

// ReleaseChannel represents the binary release channel.
type ReleaseChannel string

const (
	ReleaseChannelStable ReleaseChannel = "stable"
	ReleaseChannelBeta   ReleaseChannel = "beta"
	ReleaseChannelLTS    ReleaseChannel = "lts"
)

// BackupKind represents the kind of backup.
type BackupKind string

const (
	BackupKindManual           BackupKind = "manual"
	BackupKindScheduled        BackupKind = "scheduled"
	BackupKindPreApplyAuto     BackupKind = "pre_apply_auto"
	BackupKindPreDisableProxy  BackupKind = "pre_disable_proxy_auto"
	BackupKindOriginalBaseline BackupKind = "original_baseline"
)

// RuntimeConfigObjectType represents the type of runtime config object.
type RuntimeConfigObjectType string

const (
	RuntimeConfigObjectTypeCoreRuntime  RuntimeConfigObjectType = "core_runtime_config"
	RuntimeConfigObjectTypeReverseProxy RuntimeConfigObjectType = "reverse_proxy_config"
	RuntimeConfigObjectTypeDNSMasq      RuntimeConfigObjectType = "dnsmasq_config"
	RuntimeConfigObjectTypeFW4Runtime   RuntimeConfigObjectType = "fw4_runtime_config"
	RuntimeConfigObjectTypeSystemProxy  RuntimeConfigObjectType = "system_proxy_config"
)

// LogSourceType represents the source of a log file.
type LogSourceType string

const (
	LogSourceNginx  LogSourceType = "nginx"
	LogSourceCaddy  LogSourceType = "caddy"
	LogSourceCustom LogSourceType = "custom"
)

// TimeRange represents a time range.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
