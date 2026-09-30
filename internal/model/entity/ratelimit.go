package entity

import "time"

// RateLimitConfig stores per-node rate limiting configuration.
// Each node server can have its own rate limit settings.
// When enabled, the Agent on that node will apply nft + tc rules
// to enforce speed limits per user/subscription.
type RateLimitConfig struct {
	ID       int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID int64 `json:"server_id" gorm:"not null;uniqueIndex:idx_ratelimit_server"`

	// 启用状态
	Enabled bool `json:"enabled" gorm:"not null;default:false"`

	// 网络配置
	Iface     string `json:"iface" gorm:"size:64;not null;default:''"`        // 主网卡（空=自动检测）
	IfbDevice string `json:"ifb_device" gorm:"size:64;not null;default:ifb0"` // IFB 设备名

	// 默认限速（当订阅/客户没有单独配置时的回退值，0=不限）
	DefaultMaxDownMbps int32 `json:"default_max_down_mbps" gorm:"not null;default:0"` // 默认下载限速 Mbps
	DefaultMaxUpMbps   int32 `json:"default_max_up_mbps" gorm:"not null;default:0"`   // 默认上传限速 Mbps

	// 清理配置
	IdleTTLSeconds  int64 `json:"idle_ttl_seconds" gorm:"not null;default:3600"` // IP 归属空闲超时（秒），0=不清理
	CleanupInterval int64 `json:"cleanup_interval" gorm:"not null;default:300"`  // 清理检查间隔（秒）

	// 后端类型: "nft_tc" (当前) 或 "ebpf" (未来)
	BackendType string `json:"backend_type" gorm:"size:32;not null;default:nft_tc"`

	// 运行时状态（由 Agent 上报填充）
	NftAvailable  bool `json:"nft_available" gorm:"not null;default:false"`
	TcAvailable   bool `json:"tc_available" gorm:"not null;default:false"`
	IfbAvailable  bool `json:"ifb_available" gorm:"not null;default:false"`
	ActiveEntries int  `json:"active_entries" gorm:"not null;default:0"` // 当前活跃限速条目数

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (RateLimitConfig) TableName() string { return "rate_limit_configs" }

// RateLimitIPEntry persists an IP → subscription association in the Agent's local DB.
// On Agent restart, these entries are loaded to rebuild the MarkRegistry and re-apply
// nft + tc rules, so that active users don't lose their rate limit enforcement
// until the next subscription access or Portal login pushes a fresh association.
type RateLimitIPEntry struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID       int64  `json:"server_id" gorm:"not null;index:idx_rlip_server"`
	IP             string `json:"ip" gorm:"size:64;not null;uniqueIndex:idx_rlip_ip"`
	SubscriptionID int64  `json:"subscription_id" gorm:"not null;index:idx_rlip_sub"`
	MaxDownMbps    int32  `json:"max_down_mbps" gorm:"not null;default:0"`
	MaxUpMbps      int32  `json:"max_up_mbps" gorm:"not null;default:0"`
	// 账号（订阅）维度的并发连接限制与设备数限制：Agent 重启恢复时重建计数规则用
	ConnLimit   int32     `json:"conn_limit" gorm:"not null;default:0"`
	DeviceLimit int32     `json:"device_limit" gorm:"not null;default:0"`
	LastSeen    time.Time `json:"last_seen" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null"`
}

func (RateLimitIPEntry) TableName() string { return "rate_limit_ip_entries" }
