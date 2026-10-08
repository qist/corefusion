package entity

import "time"

// InboundUserTraffic 是入站**用户级**流量计数（按 入站 × 用户 email）。
//
// 背景：流量原本寄存在 inbound_configs.config_json 的用户对象里（_usedUp/
// _usedDown），导致每次流量归户都要「读全表 → 解析 JSON → 整包回写」——配置行
// 与高频易变数据混在同一行，既放大读写、又有并发覆盖风险，也让配置无法整体加密。
// 现在拆到本表，config_json 只在配置变更时写。
//
// 核心（xray/sing-box）不消费本表：渲染配置时 _usedUp/_usedDown 本来就会被剔除。
type InboundUserTraffic struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID  int64     `json:"server_id" gorm:"not null;uniqueIndex:idx_inbound_user_traffic_uniq,priority:1"`
	InboundID int64     `json:"inbound_id" gorm:"not null;uniqueIndex:idx_inbound_user_traffic_uniq,priority:2"`
	Email     string    `json:"email" gorm:"size:191;not null;uniqueIndex:idx_inbound_user_traffic_uniq,priority:3"`
	UsedUp    int64     `json:"used_up" gorm:"not null;default:0"`
	UsedDown  int64     `json:"used_down" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (InboundUserTraffic) TableName() string { return "inbound_user_traffic" }
