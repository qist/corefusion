package entity

import (
	"testing"
	"time"
)

func TestResolveQuotaPolicy(t *testing.T) {
	tmplAcc := &PlanTemplate{DefaultQuotaPolicy: QuotaPolicyAccumulate}
	tmplExpire := &PlanTemplate{DefaultQuotaPolicy: QuotaPolicyResetOnExpire}

	cases := []struct {
		name    string
		variant *PlanVariant
		tmpl    *PlanTemplate
		want    string
	}{
		{"规格未设置继承套餐", &PlanVariant{}, tmplExpire, QuotaPolicyResetOnExpire},
		{"规格覆盖套餐", &PlanVariant{QuotaPolicy: QuotaPolicyResetOnRenew}, tmplAcc, QuotaPolicyResetOnRenew},
		{"套餐未设置回落累加", &PlanVariant{}, &PlanTemplate{}, QuotaPolicyAccumulate},
		{"全部为空回落累加", nil, nil, QuotaPolicyAccumulate},
		{"非法值视为未设置", &PlanVariant{QuotaPolicy: "bogus"}, tmplAcc, QuotaPolicyAccumulate},
	}
	for _, tc := range cases {
		if got := ResolveQuotaPolicy(tc.variant, tc.tmpl); got != tc.want {
			t.Errorf("%s: ResolveQuotaPolicy = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNextQuotaResetAt(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 30, 0, 0, time.Local)

	if got := NextQuotaResetAt(QuotaResetCycleNone, nil, base); got != nil {
		t.Errorf("none 不应重置，得到 %v", got)
	}
	if got := NextQuotaResetAt("", nil, base); got != nil {
		t.Errorf("空周期不应重置，得到 %v", got)
	}

	next := NextQuotaResetAt(QuotaResetCycleMonthly, nil, base)
	if next == nil || next.Month() != time.December || next.Year() != 2026 {
		t.Fatalf("monthly 应落在下个月，得到 %v", next)
	}

	day := 1
	next = NextQuotaResetAt(QuotaResetCycleMonthly, &day, base)
	if next == nil || next.Day() != 1 || next.Month() != time.December {
		t.Fatalf("配置 resetDay=1 应固定到 12 月 1 日，得到 %v", next)
	}

	// 29-31 在小月不存在 → 忽略，避免时间被规范化到下下个月
	badDay := 31
	next = NextQuotaResetAt(QuotaResetCycleMonthly, &badDay, base)
	if next == nil || next.Month() != time.December || next.Day() != 20 {
		t.Fatalf("resetDay=31 应回落到同日，得到 %v", next)
	}
}
