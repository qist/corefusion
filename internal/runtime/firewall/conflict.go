package firewall

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// 端口跳跃（端口段 → 单端口 DNAT）的规则是**节点级全局资源**：
// 同一端口段只能指向一个入站端口。xray 与 sing-box 可能同时开启，也可能有多个
// 入站同时开启，因此必须做冲突检测，否则：
//   - 同段不同目标：后加的规则不会生效（nft 先匹配者优先），入站静默不可达
//   - 区间重叠：同样只有一条生效，另一个入站被"吃掉"
//
// 本文件提供与核心无关的规则模型、解析与冲突判定。

// RuleSpec 一条端口跳跃规则。
type RuleSpec struct {
	PortRange  string
	TargetPort int
	Interface  string
}

// Key 用于判重（同段 + 同目标 + 同网卡 = 同一条规则）。
func (r RuleSpec) Key() string {
	return fmt.Sprintf("%s|%d|%s", r.PortRange, r.TargetPort, r.Interface)
}

// ParsePortRange 解析 "20000-50000" 或单端口 "443" → (lo, hi, ok)。
func ParsePortRange(s string) (int, int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, false
	}
	if i := strings.IndexByte(s, '-'); i > 0 {
		lo, err1 := strconv.Atoi(strings.TrimSpace(s[:i]))
		hi, err2 := strconv.Atoi(strings.TrimSpace(s[i+1:]))
		if err1 != nil || err2 != nil || lo <= 0 || hi < lo || hi > 65535 {
			return 0, 0, false
		}
		return lo, hi, true
	}
	p, err := strconv.Atoi(s)
	if err != nil || p <= 0 || p > 65535 {
		return 0, 0, false
	}
	return p, p, true
}

// RangesOverlap 判断两个端口段/端口是否重叠。
func RangesOverlap(a, b string) bool {
	aLo, aHi, okA := ParsePortRange(a)
	bLo, bHi, okB := ParsePortRange(b)
	if !okA || !okB {
		return false
	}
	return aLo <= bHi && bLo <= aHi
}

// FindConflicts 检出与新规则冲突的既有规则：
//   - 完全相同的规则（同段同目标同网卡）→ 视为重复，调用方按幂等处理，不算冲突
//   - 同段/区间重叠但目标端口不同 → 冲突（会互相覆盖）
//   - 同段同目标但网卡不同 → 允许（不同网卡各自生效）
func FindConflicts(existing []RuleSpec, want RuleSpec) []RuleSpec {
	var conflicts []RuleSpec
	for _, e := range existing {
		if e.Key() == want.Key() {
			continue // 重复，交给幂等逻辑
		}
		if !RangesOverlap(e.PortRange, want.PortRange) {
			continue
		}
		if e.TargetPort == want.TargetPort {
			// 同目标：不同网卡可共存；同网卡则视作重复（上面 Key 已排除同网卡）
			continue
		}
		conflicts = append(conflicts, e)
	}
	return conflicts
}

// FormatConflicts 把冲突规则格式化成给用户看的提示。
func FormatConflicts(conflicts []RuleSpec, want RuleSpec) string {
	parts := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		iface := c.Interface
		if iface == "" {
			iface = "所有网卡"
		}
		parts = append(parts, fmt.Sprintf("%s → 端口 %d (%s)", c.PortRange, c.TargetPort, iface))
	}
	return fmt.Sprintf("端口段 %s 与已有规则冲突：%s。同一端口段只能指向一个入站端口，"+
		"请改用不重叠的端口段（例如另一个入站改成 40000-65000）", want.PortRange, strings.Join(parts, "；"))
}

// CheckConflict 读取节点当前规则并检查冲突，返回可直接展示给用户的错误。
// 找不到任何规则的读取方式时返回 nil（不阻塞，交由核心/系统报错）。
func CheckConflict(want RuleSpec) error {
	existing, ok := ListRules()
	if !ok {
		return nil
	}
	conflicts := FindConflicts(existing, want)
	if len(conflicts) == 0 {
		return nil
	}
	return fmt.Errorf("%s", FormatConflicts(conflicts, want))
}

// ListRules 读取当前节点的端口跳跃规则（nft 优先，其次 iptables）。
// 返回 ok=false 表示两种工具都不可用/读取失败。
func ListRules() ([]RuleSpec, bool) {
	if _, err := exec.LookPath("nft"); err == nil {
		if rules, err := listNftRules(); err == nil {
			return rules, true
		}
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		if rules, err := listIptablesRules(); err == nil {
			return rules, true
		}
	}
	return nil, false
}

// listNftRules 解析 `nft -a list chain inet hysteria_porthopping prerouting`。
func listNftRules() ([]RuleSpec, error) {
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", "hysteria_porthopping", "prerouting").Output()
	if err != nil {
		return nil, err
	}
	var rules []RuleSpec
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "redirect to") || !strings.Contains(line, "dport") {
			continue
		}
		spec := RuleSpec{}
		fields := strings.Fields(line)
		for i, f := range fields {
			switch {
			case f == "dport" && i+1 < len(fields):
				spec.PortRange = normalizeNftRange(fields[i+1])
			case f == "to" && i+1 < len(fields):
				spec.TargetPort = parseNftPort(fields[i+1])
			case f == "iifname" && i+1 < len(fields):
				spec.Interface = strings.Trim(fields[i+1], "\"")
			}
		}
		if spec.PortRange != "" && spec.TargetPort > 0 {
			rules = append(rules, spec)
		}
	}
	return rules, nil
}

// normalizeNftRange nft 打印会把 "20000-50000" 原样输出；兼容 "-" 前后的空格。
func normalizeNftRange(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), " - ", "-")
}

// parseNftPort 解析 "redirect to :23456" 里的端口（含 ":" 前缀）。
func parseNftPort(s string) int {
	s = strings.TrimPrefix(strings.TrimSpace(s), ":")
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return p
}

// listIptablesRules 解析 `iptables -t nat -S PREROUTING` 里的 REDIRECT 规则。
func listIptablesRules() ([]RuleSpec, error) {
	out, err := exec.Command("iptables", "-t", "nat", "-S", "PREROUTING").Output()
	if err != nil {
		return nil, err
	}
	var rules []RuleSpec
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "REDIRECT") || !strings.Contains(line, "--dport") {
			continue
		}
		spec := RuleSpec{}
		fields := strings.Fields(line)
		for i, f := range fields {
			if i+1 >= len(fields) {
				break
			}
			switch f {
			case "--dport":
				spec.PortRange = strings.TrimSpace(fields[i+1])
			case "--to-ports":
				if p, err := strconv.Atoi(strings.TrimSpace(fields[i+1])); err == nil {
					spec.TargetPort = p
				}
			case "-i":
				spec.Interface = strings.TrimSpace(fields[i+1])
			}
		}
		if spec.PortRange != "" && spec.TargetPort > 0 {
			rules = append(rules, spec)
		}
	}
	return rules, nil
}
