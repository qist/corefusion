package entity

import "time"

// 配额重置周期取值（subscriptions.quota_reset_cycle）。
const (
	QuotaResetCycleNone    = "none"
	QuotaResetCycleMonthly = "monthly"
)

// NextQuotaResetAt 计算下一次配额重置时间；cycle 为空或 none 时返回 nil（不重置）。
//
// monthly 语义：以 from 为基准取下一个自然月；若配置了 resetDay（1-28），
// 则固定到该日，方便「每月 1 号统一重置」。resetDay 超出 1-28 会被忽略
// （29-31 在小月不存在，避免时间被 Go 规范化到下个月）。
func NextQuotaResetAt(cycle string, resetDay *int, from time.Time) *time.Time {
	if cycle != QuotaResetCycleMonthly {
		return nil
	}
	next := from.AddDate(0, 1, 0)
	if resetDay != nil && *resetDay >= 1 && *resetDay <= 28 {
		next = time.Date(next.Year(), next.Month(), *resetDay, 0, 0, 0, 0, next.Location())
	}
	return &next
}
