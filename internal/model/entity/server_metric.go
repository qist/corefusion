package entity

import "time"

// ServerMetric stores historical resource metrics for a node server.
// One row per data point, collected periodically from Agent heartbeats or GetStatus.
type ServerMetric struct {
	ID          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID    int64  `json:"server_id" gorm:"index;not null"`

	// CPU & Memory
	CPUUsage    float64 `json:"cpu_usage" gorm:"not null;default:0"`     // 0-100
	MemUsageBytes int64 `json:"mem_usage_bytes" gorm:"not null;default:0"`

	// Disk
	DiskUsageBytes int64 `json:"disk_usage_bytes" gorm:"not null;default:0"`

	// Network (rate since last sample)
	NetRxBytes int64 `json:"net_rx_bytes" gorm:"not null;default:0"`
	NetTxBytes int64 `json:"net_tx_bytes" gorm:"not null;default:0"`

	// System load
	Load1m           float64 `json:"load_1m" gorm:"not null;default:0"`
	TCPConnections   int     `json:"tcp_connections" gorm:"not null;default:0"`

	// Traffic (user traffic rate)
	TrafficUplink   int64 `json:"traffic_uplink" gorm:"not null;default:0"`     // bytes/sec
	TrafficDownlink int64 `json:"traffic_downlink" gorm:"not null;default:0"`   // bytes/sec

	// Agent info
	AgentVersion string `json:"agent_version" gorm:"size:64"`

	CreatedAt time.Time `json:"created_at" gorm:"index;not null"`
}

func (ServerMetric) TableName() string { return "server_metrics" }

// SpeedTestRecord stores historical speed test results.
type SpeedTestRecord struct {
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID   int64  `json:"server_id" gorm:"index;not null"`
	LatencyMs  int32  `json:"latency_ms" gorm:"not null;default:0"`
	JitterMs   int32  `json:"jitter_ms" gorm:"not null;default:0"`
	PacketLoss float64 `json:"packet_loss" gorm:"not null;default:0"`
	Success    bool   `json:"success" gorm:"not null;default:true"`
	Error      string `json:"error" gorm:"size:512"`
	CreatedAt  time.Time `json:"created_at" gorm:"index;not null"`
}

func (SpeedTestRecord) TableName() string { return "speed_test_records" }
