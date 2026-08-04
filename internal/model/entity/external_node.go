package entity

import "time"

// ExternalNodeSource represents an imported external node source.
type ExternalNodeSource struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string    `json:"name" gorm:"not null"`
	SourceType string    `json:"source_type" gorm:"not null"`
	SourceData string    `json:"source_data" gorm:"type:longtext"`
	SourceURL  string    `json:"source_url"`
	Enabled    bool      `json:"enabled" gorm:"not null;default:true"`
	SortOrder  int       `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ExternalNodeSource) TableName() string { return "external_node_sources" }

// ExternalNode represents a single external proxy node.
type ExternalNode struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceID   int64     `json:"source_id" gorm:"not null"`   // FK to external_node_sources
	Name       string    `json:"name" gorm:"not null"`
	Protocol   string    `json:"protocol"`                     // vless, vmess, trojan, etc.
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Transport  string    `json:"transport"`
	TLS        bool      `json:"tls"`
	ServerName string    `json:"server_name"`
	ProxyJSON  string    `json:"proxy_json" gorm:"type:longtext;not null"` // full mihomo proxy map JSON
	Enabled    bool      `json:"enabled" gorm:"not null;default:true"`
	SortOrder  int       `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ExternalNode) TableName() string { return "external_nodes" }
