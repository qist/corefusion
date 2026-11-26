package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"corefusion/internal/model/nodecheckup"
	agentpb "corefusion/internal/runtime/agent/proto"
)

func TestCollectSystemFactsReadOnly(t *testing.T) {
	resp := CollectSystemFacts(&agentpb.GetSystemFactsRequest{IncludeFirewallRules: true, IncludeSsh: true})
	if resp == nil || !resp.GetSuccess() {
		t.Fatalf("采集失败：%+v", resp)
	}
	if len(resp.GetFacts()) > 256*1024 {
		t.Fatalf("facts 超过 256KB：%d", len(resp.GetFacts()))
	}
	var facts nodecheckup.Facts
	if err := json.Unmarshal([]byte(resp.GetFacts()), &facts); err != nil {
		t.Fatalf("facts JSON 无法解析：%v", err)
	}
	if facts.Env.Kind == "" || facts.Env.OS == "" {
		t.Fatalf("环境信息缺失：%+v", facts.Env)
	}
	if len(facts.Sysctl) == 0 {
		t.Fatalf("未采集到 sysctl 白名单")
	}
	// 容器/宿主都必须能判断「可写性」，不能因为 /proc/sys 只读而整体失败。
	for key, f := range facts.Sysctl {
		if f.Scope == "" {
			t.Fatalf("sysctl %s 缺少 scope", key)
		}
	}
}

// firewalld 必须优先于 nft/iptables：它可能用任一后端，直接看底层会把
// firewalld 自己的表/链误判成 nftables/iptables。
func TestParseFirewalldTarget(t *testing.T) {
	cases := map[string]string{
		"public (active)\n  target: default\n  services: ssh": "drop",
		"public (active)\n  target: ACCEPT":                   "accept",
		"public (active)\n  target: %%REJECT%%":               "drop",
		"public (active)\n  target: DROP":                     "drop",
		"no target line":                                      "unknown",
	}
	for in, want := range cases {
		if got := parseFirewalldTarget(in); got != want {
			t.Fatalf("parseFirewalldTarget(%q)=%q, want %q", in, got, want)
		}
	}
}

// `iptables -S` 永远有 `-P INPUT`，不能据此判定"有防火墙"。
func TestHasIptablesInputRules(t *testing.T) {
	noRules := "-P INPUT ACCEPT\n-P FORWARD ACCEPT\n-P OUTPUT ACCEPT\n-N DOCKER\n"
	if hasIptablesInputRules(noRules) {
		t.Fatal("只有默认策略和一个空链时不应算 INPUT 规则")
	}
	withRules := noRules + "-A INPUT -p tcp --dport 22 -j ACCEPT\n"
	if !hasIptablesInputRules(withRules) {
		t.Fatal("存在 -A INPUT 规则时应判定为有入站规则")
	}
	// podman/netavark 的 INPUT 跳转不是主机防火墙。
	netavark := noRules + "-A INPUT -m comment --comment \"netavark firewall rules\" -j NETAVARK_INPUT\n"
	if hasIptablesInputRules(netavark) {
		t.Fatal("netavark 的 INPUT 跳转不应算主机防火墙")
	}
}

// 容器网络（podman/netavark）也会建 input 链，不能当成主机防火墙。
func TestNftInputHasEffectiveRules(t *testing.T) {
	netavark := `table inet netavark {
	chain INPUT {
		type filter hook input priority filter; policy accept;
		counter packets 145792154 bytes 93838037985 jump NETAVARK_INPUT
	}
}
`
	if nftInputHasEffectiveRules(netavark) {
		t.Fatal("只有 NETAVARK_INPUT 跳转时不应算主机防火墙")
	}

	realFilter := `table inet filter {
	chain input {
		type filter hook input priority filter; policy drop;
		tcp dport 22 accept
		ct state established,related accept
	}
}
`
	if !nftInputHasEffectiveRules(realFilter) {
		t.Fatal("存在 accept/drop 规则时应判定为有效防火墙")
	}

	if nftInputHasEffectiveRules("table inet corefusion_ratelimit {\n\tset x {\n\t}\n}\n") {
		t.Fatal("没有 input hook 时不应算防火墙")
	}
}

// sysctl 配置文件解析：只保留白名单 key，跳过注释/空行，记录行号。
func TestParseSysctlConfigFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "99-test.conf")
	content := "# comment\n\nnet.core.somaxconn = 32768\nvm.swappiness=10\nnet.ipv4.ip_forward=1 # not in whitelist\n- kernel.pid_max = 4194304\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{
		"net.core.somaxconn": true,
		"vm.swappiness":      true,
		"kernel.pid_max":     true,
	}
	hits := parseSysctlConfigFile(p, wanted, 7)
	if len(hits) != 3 {
		t.Fatalf("应命中 3 条，实际 %d：%+v", len(hits), hits)
	}
	byKey := map[string]nodecheckup.ConfigHit{}
	for _, h := range hits {
		byKey[h.Key] = h
	}
	if h := byKey["net.core.somaxconn"]; h.Value != "32768" || h.Line != 3 || h.Order != 7 {
		t.Fatalf("解析错误：%+v", h)
	}
	if h := byKey["kernel.pid_max"]; h.Value != "4194304" {
		t.Fatalf("-key 形式应去掉前导 -：%+v", h)
	}
	if _, ok := byKey["net.ipv4.ip_forward"]; ok {
		t.Fatal("不在白名单的 key 不应采集")
	}
}
