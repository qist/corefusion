package firewall

import (
	"strings"
	"testing"
)

func TestParsePortRange(t *testing.T) {
	cases := []struct {
		in       string
		lo, hi   int
		ok       bool
	}{
		{"20000-50000", 20000, 50000, true},
		{"443", 443, 443, true},
		{" 1000-2000 ", 1000, 2000, true},
		{"", 0, 0, false},
		{"20000-", 0, 0, false},
		{"50000-20000", 0, 0, false}, // 反向区间非法
		{"0-100", 0, 0, false},
		{"1-70000", 0, 0, false},
		{"abc", 0, 0, false},
	}
	for _, c := range cases {
		lo, hi, ok := ParsePortRange(c.in)
		if ok != c.ok || (ok && (lo != c.lo || hi != c.hi)) {
			t.Errorf("ParsePortRange(%q) = (%d,%d,%v), want (%d,%d,%v)", c.in, lo, hi, ok, c.lo, c.hi, c.ok)
		}
	}
}

// 端口段是节点级全局资源：同段/重叠且目标端口不同 = 冲突（否则后加的不生效）。
func TestFindConflicts(t *testing.T) {
	existing := []RuleSpec{
		{PortRange: "20000-50000", TargetPort: 10195, Interface: ""},
		{PortRange: "10000-15000", TargetPort: 20001, Interface: "eth0"},
	}
	cases := []struct {
		name      string
		want      RuleSpec
		conflicts int
	}{
		{"完全不同段", RuleSpec{PortRange: "60000-65000", TargetPort: 12345}, 0},
		{"同段同目标同网卡（重复，按幂等处理）", RuleSpec{PortRange: "20000-50000", TargetPort: 10195}, 0},
		{"同段不同目标（冲突）", RuleSpec{PortRange: "20000-50000", TargetPort: 23456}, 1},
		{"区间重叠不同目标（冲突）", RuleSpec{PortRange: "40000-65000", TargetPort: 23456}, 1},
		{"单端口落在既有段内（冲突）", RuleSpec{PortRange: "30000", TargetPort: 23456}, 1},
		{"同段同目标但网卡不同（可共存）", RuleSpec{PortRange: "10000-15000", TargetPort: 20001, Interface: "wg0"}, 0},
		{"相邻但不重叠", RuleSpec{PortRange: "15001-16000", TargetPort: 23456, Interface: "wg0"}, 0},
	}
	for _, c := range cases {
		got := FindConflicts(existing, c.want)
		if len(got) != c.conflicts {
			t.Errorf("%s: 冲突数 = %d, want %d (%#v)", c.name, len(got), c.conflicts, got)
		}
	}
}

func TestFormatConflictsMessage(t *testing.T) {
	msg := FormatConflicts([]RuleSpec{{PortRange: "40000-65000", TargetPort: 10195}}, RuleSpec{PortRange: "20000-50000"})
	for _, want := range []string{"20000-50000", "40000-65000", "10195"} {
		if !strings.Contains(msg, want) {
			t.Errorf("提示缺少 %q: %s", want, msg)
		}
	}
}

func TestRangesOverlap(t *testing.T) {
	if !RangesOverlap("20000-50000", "40000-65000") {
		t.Error("应判定为重叠")
	}
	if RangesOverlap("10000-15000", "15001-20000") {
		t.Error("相邻不应判定为重叠")
	}
	if !RangesOverlap("443", "443") {
		t.Error("同端口应判定为重叠")
	}
}
