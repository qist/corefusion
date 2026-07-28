package entity

import "time"

// NodeTrafficUsage records monthly traffic usage per node for quota tracking.
// Each row represents one billing period for a specific server.
type NodeTrafficUsage struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID  int64    `json:"server_id" gorm:"not null;uniqueIndex:idx_node_traffic_period"`
	Interface string   `json:"interface" gorm:"size:64;not null;default:''"`

	// 结算周期
	PeriodStart time.Time `json:"period_start" gorm:"not null;uniqueIndex:idx_node_traffic_period"`
	PeriodEnd   time.Time `json:"period_end" gorm:"not null"`

	// 累计流量（字节）
	RxBytes int64 `json:"rx_bytes" gorm:"not null;default:0"` // 出口网卡接收
	TxBytes int64 `json:"tx_bytes" gorm:"not null;default:0"` // 出口网卡发送
	// 总流量 = RxBytes + TxBytes（对于代理节点，TxBytes 是主要出口流量）
	TotalBytes int64 `json:"total_bytes" gorm:"not null;default:0"`

	// 配额信息快照（记录该周期生效的配额）
	QuotaBytes int64 `json:"quota_bytes" gorm:"not null;default:0"`

	// 是否已超限暂停
	Suspended   bool       `json:"suspended" gorm:"not null;default:false"`
	SuspendedAt *time.Time `json:"suspended_at"`

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (NodeTrafficUsage) TableName() string { return "node_traffic_usage" }
