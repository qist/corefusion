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

// ProtocolType represents the proxy protocol.
type ProtocolType string

const (
	ProtocolVLESS          ProtocolType = "vless"
	ProtocolVMess          ProtocolType = "vmess"
	ProtocolTrojan         ProtocolType = "trojan"
	ProtocolShadowsocks    ProtocolType = "shadowsocks"
	ProtocolShadowsocks2022 ProtocolType = "shadowsocks2022"
	ProtocolHysteria2      ProtocolType = "hysteria2"
	ProtocolTUIC           ProtocolType = "tuic"
)

// TransportType represents the transport layer.
type TransportType string

const (
	TransportTCP     TransportType = "tcp"
	TransportWS      TransportType = "ws"
	TransportGRPC    TransportType = "grpc"
	TransportHTTP2   TransportType = "http2"
	TransportQUIC    TransportType = "quic"
	TransportReality TransportType = "reality"
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

// TimeRange represents a time range.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
