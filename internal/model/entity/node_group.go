package entity

import "time"

// Node tier constants.
const (
	NodeTierStandard  = "standard"
	NodeTierPremium   = "premium"
	NodeTierExclusive = "exclusive" // 独享/专属节点
)

// NodeGroup is a logical collection of inbound configs and external nodes.
// Used to control which nodes are accessible by which plan templates.
type NodeGroup struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	Region      string    `gorm:"size:32" json:"region"`
	Tier        string    `gorm:"size:32;not null;default:standard" json:"tier"` // standard/premium/exclusive
	SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (NodeGroup) TableName() string { return "node_groups" }

// NodeGroupMember maps an inbound config or external node to a node group.
type NodeGroupMember struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupID         int64     `gorm:"not null;index" json:"group_id"`
	InboundConfigID int64     `gorm:"default:0" json:"inbound_config_id"`
	ExternalNodeID  int64     `gorm:"default:0" json:"external_node_id"`
	Enabled         bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

func (NodeGroupMember) TableName() string { return "node_group_members" }
