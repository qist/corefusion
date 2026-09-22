package entity

import "time"

// ChainLink defines a single entry→exit link in the chain proxy topology.
// An entry node uses chain_links to specify where its outbound traffic goes.
// The exit target can be a cluster server (exit_server_id) or an external
// node (exit_external_id).
type ChainLink struct {
	ID             int64 `json:"id" gorm:"primaryKey;autoIncrement"`
	EntryServerID  int64 `json:"entry_server_id" gorm:"not null;default:0;index:idx_chain_link_entry"`
	EntryInboundID int64 `json:"entry_inbound_id" gorm:"not null;default:0;index:idx_chain_link_entry"`
	ExitServerID   int64 `json:"exit_server_id" gorm:"not null;default:0;index"`
	// ExitInboundID：链式出口使用**出口节点上已有的入站**（>0 时用它，不再自动创建 relay-in 入站）。
	// 与 ExitServerID 搭配：ExitInboundID 必须属于 ExitServerID 那台机器。
	ExitInboundID  int64 `json:"exit_inbound_id" gorm:"not null;default:0;index"`
	ExitExternalID int64 `json:"exit_external_id" gorm:"not null;default:0;index"`
	ExitTag        string `json:"exit_tag" gorm:"size:128;not null;default:''"`
	Weight         int    `json:"weight" gorm:"not null;default:100"`
	Priority       int    `json:"priority" gorm:"not null;default:1"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true"`
	SortOrder      int    `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null"`
}

func (ChainLink) TableName() string { return "chain_links" }

// IsExitExternal returns true if this link points to an external node.
func (c *ChainLink) IsExitExternal() bool { return c.ExitExternalID > 0 }

// IsExitServer returns true if this link points to a cluster server.
func (c *ChainLink) IsExitServer() bool { return c.ExitServerID > 0 }

// UseExitInbound 表示该链路使用"出口节点上已有的入站"作为链式出口
// （此时链式**不创建** relay-in 入站，也不需要 chain_exit_configs）。
func (c *ChainLink) UseExitInbound() bool { return c.ExitInboundID > 0 }
