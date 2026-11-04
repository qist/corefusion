package entity

import (
	"encoding/json"
	"time"
)

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
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceID   int64  `json:"source_id" gorm:"not null"` // FK to external_node_sources
	Name       string `json:"name" gorm:"not null"`
	Protocol   string `json:"protocol"` // vless, vmess, trojan, etc.
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Transport  string `json:"transport"`
	TLS        bool   `json:"tls"`
	ServerName string `json:"server_name"`
	ProxyJSON  string `json:"proxy_json" gorm:"type:longtext;not null"` // full mihomo proxy map JSON
	Enabled    bool   `json:"enabled" gorm:"not null;default:true"`
	SortOrder  int    `json:"sort_order" gorm:"not null;default:0"`
	// AccessDomainJSON 保存「访问域名」配置（与入站 _accessDomain 同构的 JSON 对象）。
	// 用于 CDN 加速 / 优选 IP：订阅渲染时把 server/port 换成这里选择的域名/IP，
	// SNI/Host 保持导入时的值。为空 = 未启用，直接使用导入的地址。
	AccessDomainJSON string `json:"-" gorm:"type:longtext;not null;default:''"`
	// AccessDomain 仅用于 API 序列化（不落库），由 AccessDomainJSON 解析而来。
	AccessDomain map[string]interface{} `json:"access_domain,omitempty" gorm:"-"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

func (ExternalNode) TableName() string { return "external_nodes" }

// LoadAccessDomain 从 AccessDomainJSON 解析出访问域名对象；无配置或解析失败返回 nil。
func (n *ExternalNode) LoadAccessDomain() map[string]interface{} {
	if n == nil || n.AccessDomainJSON == "" {
		return nil
	}
	var ad map[string]interface{}
	if json.Unmarshal([]byte(n.AccessDomainJSON), &ad) != nil {
		return nil
	}
	return ad
}

// SetAccessDomain 把访问域名对象序列化进 AccessDomainJSON；nil/空对象表示不启用。
func (n *ExternalNode) SetAccessDomain(ad map[string]interface{}) {
	if n == nil {
		return
	}
	if len(ad) == 0 {
		n.AccessDomainJSON = ""
		n.AccessDomain = nil
		return
	}
	if b, err := json.Marshal(ad); err == nil {
		n.AccessDomainJSON = string(b)
	}
	n.AccessDomain = ad
}
