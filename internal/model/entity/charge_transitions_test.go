package entity

import "testing"

// TestCanChargeOrderTransition 钉住充值订单的状态机 ✓ ——
// 这里踩过一个 P0 真 bug ✓：网关回调是"一次性终态" ✓，会**直接**把 pending 结算成 paid ✓，
// 而状态机当初只允许 `pending → processing` ✗ → `pending → paid` 被判非法 ✗ →
// 回调恒 400 "order status is pending, cannot process callback" ✓、订单永远到不了账 ✗。
func TestCanChargeOrderTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
		why      string
	}{
		// —— 合法路径 ——
		{ChargeStatusCreated, ChargeStatusPending, true, "创建后进入待支付"},
		{ChargeStatusPending, ChargeStatusPaid, true, "⚠ 网关回调直接结算（本次修复的核心）"},
		{ChargeStatusPending, ChargeStatusFailed, true, "网关回调直接失败"},
		{ChargeStatusPending, ChargeStatusProcessing, true, "内部中间态"},
		{ChargeStatusPending, ChargeStatusExpired, true, "超时"},
		{ChargeStatusProcessing, ChargeStatusPaid, true, "事务内结算"},
		{ChargeStatusProcessing, ChargeStatusFailed, true, "事务内失败"},
		{ChargeStatusPaid, ChargeStatusRefunded, true, "退款"},
		{ChargeStatusFailed, ChargeStatusPending, true, "允许重新支付"},
		// —— 幂等 ——
		{ChargeStatusPaid, ChargeStatusPaid, true, "同状态幂等"},
		// —— 必须拒绝（防止越级/回退/终态复活）——
		{ChargeStatusCreated, ChargeStatusPaid, false, "不能跳过 pending"},
		{ChargeStatusPaid, ChargeStatusPending, false, "已支付不能回退"},
		{ChargeStatusPaid, ChargeStatusFailed, false, "已支付不能变失败"},
		{ChargeStatusExpired, ChargeStatusPaid, true, "⚠ 已过期但已收到款 → 必须履行（正常交易逻辑）"},
		{ChargeStatusExpired, ChargeStatusFailed, true, "过期的失败通知照常受理（无事可做）"},
		{ChargeStatusRefunded, ChargeStatusPaid, false, "已退款是终态"},
		{ChargeStatusRefunded, ChargeStatusPending, false, "已退款是终态"},
		{"unknown", ChargeStatusPaid, false, "未知状态一律拒绝"},
	}
	for _, c := range cases {
		if got := CanChargeOrderTransition(c.from, c.to); got != c.want {
			t.Errorf("%s → %s = %v，期望 %v（%s）", c.from, c.to, got, c.want, c.why)
		}
	}
}
