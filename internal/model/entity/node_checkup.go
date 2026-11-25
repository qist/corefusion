package entity

// NodeCheckupResult 保存每个节点最新一次安全体检的判定结果（不保存原始 facts）。
type NodeCheckupResult struct {
	ServerID    int64  `json:"server_id" gorm:"primaryKey;autoIncrement:false"`
	GeneratedAt int64  `json:"generated_at" gorm:"not null"`
	Env         string `json:"env" gorm:"type:text"`
	Summary     string `json:"summary" gorm:"type:text"`
	Items       string `json:"items" gorm:"type:longtext"`
}

func (NodeCheckupResult) TableName() string { return "node_checkup_results" }
