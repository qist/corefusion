package xraycontrol

import (
	"strings"
	"testing"
)

// iptables -D 要求整条规则逐字匹配，实际规则可能是 `-m udp` 等写法；
// 这里验证能从 `-S` 输出里找到并改写成 -D（幂等删除的兜底路径）。
func TestMatchIptablesDeleteArgs(t *testing.T) {
	out := strings.Join([]string{
		"-P PREROUTING ACCEPT",
		"-A PREROUTING -p udp -m udp --dport 10000:19999 -j REDIRECT --to-ports 4443",
		"-A PREROUTING -p udp -m udp --dport 20000:50000 -j REDIRECT --to-ports 4444",
	}, "\n")

	args, ok := matchIptablesDeleteArgs(out, "10000:19999", 4443)
	if !ok {
		t.Fatal("应匹配到 10000:19999 -> 4443")
	}
	joined := strings.Join(args, " ")
	if !strings.HasPrefix(joined, "-t nat -D PREROUTING ") {
		t.Fatalf("删除参数形式不对：%s", joined)
	}
	if strings.Contains(joined, "-A ") {
		t.Fatalf("应把 -A 换成 -D：%s", joined)
	}

	// 目标端口不同 → 不应误删
	if _, ok := matchIptablesDeleteArgs(out, "10000:19999", 4444); ok {
		t.Fatal("目标端口不同时不应匹配")
	}
	// 规则不存在
	if _, ok := matchIptablesDeleteArgs(out, "30000:39999", 4443); ok {
		t.Fatal("不存在的规则不应匹配")
	}
}
