package entity

import "time"

// NodeServerType defines the role of a node server in the multi-region topology.
const (
	NodeTypeStandalone     = "standalone"     // 独立服务节点
	NodeTypeMirror         = "mirror"         // 镜像节点（流量合并到主节点）
	NodeTypePrimaryBackup  = "primary_backup" // 主备节点（自动故障切换）
	NodeTypeChainEntry     = "chain_entry"    // 链式代理入口（国内中转）
	NodeTypeChainExit      = "chain_exit"      // 链式代理出口（国际出口，不计费）
)

// AgentStatus constants.
const (
	AgentStatusOnline   = "online"
	AgentStatusOffline  = "offline"
	AgentStatusDegraded = "degraded"
)

// FailoverRole constants for primary_backup nodes.
const (
	FailoverRolePrimary = "primary"
	FailoverRoleBackup  = "backup"
)

// NodeServer represents a remote server node managed by the panel.
// Each server runs an Agent that communicates with the panel via gRPC mTLS.
type NodeServer struct {
	ID          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"size:128;not null"`
	Hostname    string `json:"hostname" gorm:"size:255;not null"`
	Region      string `json:"region" gorm:"size:64;index;not null;default:''"`
	RegionLabel string `json:"region_label" gorm:"size:128;not null;default:''"`

	// 节点类型: standalone / mirror / primary_backup / chain_entry / chain_exit
	NodeType string `json:"node_type" gorm:"size:32;not null;default:standalone;index"`

	// 主备关系
	PartnerServerID  int64  `json:"partner_server_id" gorm:"not null;default:0;index"`
	FailoverRole     string `json:"failover_role" gorm:"size:16;not null;default:''"`
	FailoverPriority int    `json:"failover_priority" gorm:"not null;default:0"`

	// 链式代理配置（JSON）
	ChainConfig string `json:"chain_config" gorm:"type:text"`

	// Agent 连接
	AgentPort       int        `json:"agent_port" gorm:"not null;default:18443"`
	AgentStatus     string     `json:"agent_status" gorm:"size:32;not null;default:offline;index"`
	AgentVersion    string     `json:"agent_version" gorm:"size:64"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`

	// Bootstrap Token（仅首次注册使用，存储哈希）
	BootstrapToken          string     `json:"-" gorm:"size:256"`
	BootstrapTokenExpiresAt *time.Time `json:"bootstrap_token_expires_at"`
	BootstrapUsedAt         *time.Time `json:"bootstrap_used_at"`

	// Agent mTLS 证书
	AgentCertSerial        string     `json:"-" gorm:"size:128"`
	AgentCertExpiresAt     *time.Time `json:"agent_cert_expires_at"`
	AgentCertLastRenewedAt *time.Time `json:"agent_cert_last_renewed_at"`

	// 外部服务证书关联（TLS 入站用）
	CertificateID int64 `json:"certificate_id" gorm:"not null;default:0;index"`

	// 本地标记：1=面板本机（走原有路径）
	IsLocal bool `json:"is_local" gorm:"not null;default:false"`

	// 服务器能力
	SupportedCores string `json:"supported_cores" gorm:"type:varchar(2048);not null;default:'[]'"`
	MaxInbounds    int    `json:"max_inbounds" gorm:"not null;default:0"`

	// 网络信息
	PublicIP      string `json:"public_ip" gorm:"size:64"`
	InternalIP    string `json:"internal_ip" gorm:"size:64"`
	BandwidthMbps int    `json:"bandwidth_mbps" gorm:"not null;default:0"`

	// 计费标记：chain_exit/mirror 默认 0
	BillingEnabled bool `json:"billing_enabled" gorm:"not null;default:true"`

	// 元数据
	Metadata  string    `json:"metadata" gorm:"type:text"`
	SortOrder int       `json:"sort_order" gorm:"not null;default:0"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`

	// ── 出口网卡流量配额 ──
	// 选择出口网卡名称（如 eth0、ens3），用于监控该节点的月度总流量
	EgressInterface string `json:"egress_interface" gorm:"size:64;not null;default:''"`
	// 月度流量配额（字节），0=不限
	TrafficQuotaBytes int64 `json:"traffic_quota_bytes" gorm:"not null;default:0"`
	// 每月结算日（1-28），默认 1 号重置
	TrafficQuotaResetDay int `json:"traffic_quota_reset_day" gorm:"not null;default:1"`
	// 当前是否因流量超限被暂停
	TrafficSuspended bool `json:"traffic_suspended" gorm:"not null;default:false"`
	// 流量暂停时间
	TrafficSuspendedAt *time.Time `json:"traffic_suspended_at"`
	// 当前周期累计已用流量（缓存字段，由定时任务更新）
	TrafficUsedBytes int64 `json:"traffic_used_bytes" gorm:"not null;default:0"`
	// 当前结算周期起始时间
	TrafficPeriodStart *time.Time `json:"traffic_period_start"`
}

func (NodeServer) TableName() string { return "node_servers" }

// IsChainExit returns true if this node is a chain proxy exit point.
func (n *NodeServer) IsChainExit() bool { return n.NodeType == NodeTypeChainExit }

// IsMirror returns true if this node mirrors another node's traffic.
func (n *NodeServer) IsMirror() bool { return n.NodeType == NodeTypeMirror }

// ShouldBill returns true if this node participates in billing (user creation + traffic stats).
func (n *NodeServer) ShouldBill() bool { return n.Enabled && n.BillingEnabled && !n.IsChainExit() }

// HasTrafficQuota returns true if this node has a traffic quota configured.
func (n *NodeServer) HasTrafficQuota() bool {
	return n.EgressInterface != "" && n.TrafficQuotaBytes > 0
}

// IsTrafficSuspended returns true if this node is suspended due to traffic quota exceeded.
func (n *NodeServer) IsTrafficSuspended() bool {
	return n.TrafficSuspended
}
