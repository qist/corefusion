package entity

import (
	"encoding/json"
	"time"
)

// ShareDomainConfig stores share domain configuration.
type ShareDomainConfig struct {
	ID        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Type      string `json:"type" gorm:"size:32;not null;index"` // direct, cf_domain, cf_ip
	Value     string `json:"value" gorm:"size:255;not null"`     // domain or IP
	Port      int    `json:"port" gorm:"not null;default:443"`
	Ports     string `json:"ports" gorm:"type:longtext"` // JSON array of ports
	Enabled   bool   `json:"enabled" gorm:"not null;default:true"`
	SortOrder int    `json:"sort_order" gorm:"not null;default:0"`

	// 多区域：0=全局共享域名（所有服务器通用），>0=仅该服务器使用
	ServerID int64 `json:"server_id" gorm:"index;not null;default:0"`
	// 多节点绑定：JSON 数组文本（如 "[2,11]"）——非空时对这些节点生效；
	// 空 + ServerID=0 = 全局；ServerID>0 = 单节点（旧语义，向后兼容）
	ServerIDs string `json:"server_ids" gorm:"type:longtext"`
	// 区域过滤：为空=所有区域，指定区域=仅该区域节点使用
	Region string `json:"region" gorm:"size:64;index;not null;default:''"`

	CreatedAt time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (ShareDomainConfig) TableName() string { return "share_domain_configs" }

// MatchServer 判断该条目是否对指定节点生效：
// 全局（ServerID=0 且 ServerIDs 为空）、单节点命中（ServerID）、多节点命中（ServerIDs JSON 包含）。
func (c ShareDomainConfig) MatchServer(serverID int64) bool {
	if c.ServerID == 0 && c.ServerIDs == "" {
		return true // 全局
	}
	if c.ServerID == serverID {
		return true
	}
	if c.ServerIDs != "" {
		var ids []int64
		if json.Unmarshal([]byte(c.ServerIDs), &ids) == nil {
			for _, id := range ids {
				if id == serverID {
					return true
				}
			}
		}
	}
	return false
}
