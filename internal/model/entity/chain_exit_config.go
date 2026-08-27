package entity

import "time"

// ChainExitConfig defines how a server acts as a chain exit point.
// A server with this record can receive relay traffic from entry nodes.
// If RelayOnly is true, the server is a pure exit (no user inbounds).
// If RelayOnly is false, the server is both an entry (has its own users)
// and an exit (accepts relay traffic from other entries).
type ChainExitConfig struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID       int64     `json:"server_id" gorm:"not null;uniqueIndex"`
	RelayPort      int       `json:"relay_port" gorm:"not null;default:0"`
	RelayProtocol  string    `json:"relay_protocol" gorm:"size:32;not null;default:vless"`
	RelayTransport string    `json:"relay_transport" gorm:"size:32;not null;default:tcp"`
	RelaySecurity  string    `json:"relay_security" gorm:"size:32;not null;default:none"`
	RelayUUID      string    `json:"relay_uuid" gorm:"size:128;not null;default:''"`
	RelayOnly      bool      `json:"relay_only" gorm:"not null;default:false"`
	Enabled        bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null"`
}

func (ChainExitConfig) TableName() string { return "chain_exit_configs" }
