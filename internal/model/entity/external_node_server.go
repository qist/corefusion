package entity

import "time"

// ExternalNodeServer registers an external node as a virtual entry node
// in the cluster topology. This allows external nodes to participate in
// chain links and appear in cluster management views.
//
// Without this registration, external nodes only appear in subscription
// output (via node_group_members). With registration, they become
// first-class cluster entries that can be managed, assigned regions,
// and selected as chain exit targets by other entries.
type ExternalNodeServer struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ExternalNodeID int64     `json:"external_node_id" gorm:"not null;uniqueIndex"`
	Name           string    `json:"name" gorm:"size:128;not null"`
	Hostname       string    `json:"hostname" gorm:"size:255;not null"`
	Region         string    `json:"region" gorm:"size:64;not null;default:''"`
	RegionLabel    string    `json:"region_label" gorm:"size:128;not null;default:''"`
	Enabled        bool      `json:"enabled" gorm:"not null;default:true"`
	SortOrder      int       `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null"`
}

func (ExternalNodeServer) TableName() string { return "external_node_servers" }
