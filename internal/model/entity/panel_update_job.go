package entity

import "time"

// PanelUpdateJob 是面板自更新 / 自回滚任务的 GORM 模型。
//
// 一个任务对应一次 update 或 rollback：
//   - update：下载 → 校验 sha256 → 备份当前二进制 → 原子替换 → 重启面板；
//   - rollback：用备份二进制还原 → 重启面板。
//
// 任务在后台 goroutine 中执行，接口立即返回 202 + 任务 ID，前端轮询
// /api/v1/system/update-jobs 查看进度。注意：重启面板会中断当前进程，
// 因此成功任务的 finished_at 可能来不及落库 —— 前端应以「进程重启后
// 版本号是否变化」为准。
type PanelUpdateJob struct {
	ID           int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	Action       string     `json:"action" gorm:"size:32;index;not null"` // update | rollback
	Status       string     `json:"status" gorm:"size:32;index;not null"` // pending | running | success | failed
	Phase        string     `json:"phase" gorm:"size:64"`                 // resolving | downloading | verifying | applying | restarting
	FromVersion  string     `json:"from_version" gorm:"size:64"`          // 任务开始前的版本
	ToVersion    string     `json:"to_version" gorm:"size:64"`            // 目标版本（回滚时为备份版本）
	DownloadURL  string     `json:"download_url" gorm:"size:512"`         // 本次使用的产物地址
	Checksum     string     `json:"checksum" gorm:"size:128"`             // 期望的 sha256（下载后校验）
	BackupPath   string     `json:"backup_path" gorm:"size:512"`          // 本次更新产生的 .bak 路径
	ErrorMessage string     `json:"error_message" gorm:"type:longtext"`   // 失败原因
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"not null"`
}

func (PanelUpdateJob) TableName() string { return "panel_update_jobs" }
