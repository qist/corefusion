package firewall

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeProtos(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{"tcp", "udp"}},
		{"tcp", []string{"tcp"}},
		{"udp", []string{"udp"}},
		{"tcp,udp", []string{"tcp", "udp"}},
		{"udp,tcp", []string{"tcp", "udp"}},
		{"TCP", []string{"tcp"}},
	}
	for _, tc := range cases {
		got, err := normalizeProtos(tc.in)
		if err != nil {
			t.Fatalf("normalizeProtos(%q) error: %v", tc.in, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("normalizeProtos(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if _, err := normalizeProtos("sctp"); err == nil {
		t.Fatal("unsupported protocol should error")
	}
}

// 只有显式 _fwAllow=true 才恢复放行；协议缺省 = TCP+UDP。
func TestParseAllowFromConfig(t *testing.T) {
	if on, net := ParseAllowFromConfig(`{"_fwAllow":true}`); !on || net != "" {
		t.Fatalf("expected enabled with empty network, got %v %q", on, net)
	}
	if on, _ := ParseAllowFromConfig(`{"_fwAllow":false}`); on {
		t.Fatal("_fwAllow=false should be disabled")
	}
	if on, _ := ParseAllowFromConfig(`{}`); on {
		t.Fatal("missing _fwAllow should be disabled")
	}
	if on, net := ParseAllowFromConfig(`{"_fwAllow":true,"_fwNetwork":"udp"}`); !on || net != "udp" {
		t.Fatalf("expected udp network, got %v %q", on, net)
	}
	if on, _ := ParseAllowFromConfig(`not-json`); on {
		t.Fatal("invalid json should be disabled")
	}
	// Unix Socket 入站不监听 TCP 端口，即使标记为 true 也不放行
	if on, _ := ParseAllowFromConfig(`{"_fwAllow":true,"_listenType":"sock"}`); on {
		t.Fatal("unix socket inbound should not allow port")
	}
}

func TestIsUnixSocketListen(t *testing.T) {
	cases := []struct {
		listen string
		want   bool
	}{
		{"/dev/shm/vless.sock", true},
		{"/run/xray.sock", true},
		{"unix:/dev/shm/x.sock", true},
		{"unix:///dev/shm/x.sock", true},
		{"@xray-abstract", true},
		{"./xray.sock", true},
		{"0.0.0.0", false},
		{"::", false},
		{"127.0.0.1", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsUnixSocketListen(tc.listen); got != tc.want {
			t.Fatalf("IsUnixSocketListen(%q) = %v, want %v", tc.listen, got, tc.want)
		}
	}
}

func TestPortSpecCovers(t *testing.T) {
	cases := []struct {
		spec string
		port int
		sep  string
		want bool
	}{
		{"443", 443, ":", true},
		{"443", 444, ":", false},
		{"20000:30000", 25000, ":", true},
		{"20000:30000", 30001, ":", false},
		{"50001-50020", 50020, "-", true},
		{"50001-50020", 50021, "-", false},
		{"50001-50020", 50015, ":", false}, // 用了错误的分隔符
		{"", 443, ":", false},
		{"abc", 443, ":", false},
	}
	for _, tc := range cases {
		if got := portSpecCovers(tc.spec, tc.port, tc.sep); got != tc.want {
			t.Fatalf("portSpecCovers(%q, %d, %q) = %v, want %v", tc.spec, tc.port, tc.sep, got, tc.want)
		}
	}
}

// 启动恢复时不能重复插入：已有覆盖该端口的 ACCEPT 规则就跳过。
func TestParsePortSpecs(t *testing.T) {
	cases := []struct {
		in      string
		want    [][2]int
		wantErr bool
	}{
		{"20000-50000", [][2]int{{20000, 50000}}, false},
		{"20000:50000", [][2]int{{20000, 50000}}, false},
		{"443", [][2]int{{443, 443}}, false},
		{"20000-29999,30000-39999", [][2]int{{20000, 29999}, {30000, 39999}}, false},
		{"443,443", [][2]int{{443, 443}}, false},
		{"0-100", nil, true},
		{"100-99", nil, true},
		{"70000-80000", nil, true},
		{"abc", nil, true},
		{"", nil, true},
	}
	for _, tc := range cases {
		got, err := ParsePortSpecs(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParsePortSpecs(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParsePortSpecs(%q) error: %v", tc.in, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("ParsePortSpecs(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestPortSpecRangeCovers(t *testing.T) {
	cases := []struct {
		spec   string
		lo, hi int
		sep    string
		want   bool
	}{
		{"20000:50000", 20000, 50000, ":", true},
		{"10000:60000", 20000, 50000, ":", true},
		{"20001:50000", 20000, 50000, ":", false},
		{"20000:49999", 20000, 50000, ":", false},
		{"20000-50000", 20000, 50000, "-", true},
		{"443", 443, 443, ":", true},
		{"443", 444, 444, ":", false},
	}
	for _, tc := range cases {
		if got := portSpecRangeCovers(tc.spec, tc.lo, tc.hi, tc.sep); got != tc.want {
			t.Fatalf("portSpecRangeCovers(%q, %d-%d, %q) = %v, want %v", tc.spec, tc.lo, tc.hi, tc.sep, got, tc.want)
		}
	}
}

func TestIptablesOutputCoversRange(t *testing.T) {
	output := `-P INPUT DROP
-A INPUT -p udp -m udp --dport 20000:30000 -j ACCEPT
-A INPUT -p udp -m multiport --dports 1000:2000,40000:45000 -j ACCEPT`
	if !iptablesOutputCoversRange(output, "udp", 20000, 30000) {
		t.Fatal("exact range should be covered")
	}
	if !iptablesOutputCoversRange(output, "udp", 21000, 29000) {
		t.Fatal("inner range should be covered")
	}
	if iptablesOutputCoversRange(output, "udp", 19999, 30000) {
		t.Fatal("partially covered range should not be covered")
	}
	if iptablesOutputCoversRange(output, "tcp", 20000, 30000) {
		t.Fatal("protocol mismatch should not be covered")
	}
	if !iptablesOutputCoversRange(output, "udp", 40000, 45000) {
		t.Fatal("multiport range should be covered")
	}
}

func TestUFWStatusRangeCovers(t *testing.T) {
	output := `Status: active
To                         Action      From
--                         ------      ----
20000:50000/udp            ALLOW       Anywhere
443/tcp                    ALLOW       Anywhere`
	if !ufwStatusRangeCovers(output, "udp", 20000, 50000) {
		t.Fatal("exact ufw range should be covered")
	}
	if !ufwStatusRangeCovers(output, "udp", 30000, 40000) {
		t.Fatal("inner ufw range should be covered")
	}
	if ufwStatusRangeCovers(output, "udp", 19999, 50000) {
		t.Fatal("partially covered ufw range should not be covered")
	}
	if ufwStatusRangeCovers(output, "tcp", 20000, 50000) {
		t.Fatal("protocol mismatch should not be covered")
	}
}

func TestNftOutputCoversRange(t *testing.T) {
	output := `chain input {
		type filter hook input priority filter; policy drop;
		udp dport 20000-50000 counter packets 0 bytes 0 accept
		udp dport { 10000-15000, 30000-40000 } accept
		tcp dport 443 accept
}`
	if !nftOutputCoversRange(output, "udp", 20000, 50000) {
		t.Fatal("exact nft range should be covered")
	}
	if !nftOutputCoversRange(output, "udp", 25000, 45000) {
		t.Fatal("inner nft range should be covered")
	}
	if nftOutputCoversRange(output, "udp", 19999, 50000) {
		t.Fatal("partially covered nft range should not be covered")
	}
	if !nftOutputCoversRange(output, "udp", 30000, 40000) {
		t.Fatal("nft set range should be covered")
	}
	if nftOutputCoversRange(output, "tcp", 20000, 50000) {
		t.Fatal("protocol mismatch should not be covered")
	}
}

func TestIptablesOutputCoversPort(t *testing.T) {
	output := `-P INPUT DROP
-A INPUT -i lo -j ACCEPT
-A INPUT -p tcp -m tcp --dport 443 -j ACCEPT
-A INPUT -p tcp -m multiport --dports 80,8080 -j ACCEPT
-A INPUT -p udp -m udp --dport 20000:30000 -j ACCEPT
-A INPUT -p tcp -m tcp --dport 9000 -j DROP
-A INPUT -j REJECT --reject-with icmp-port-unreachable`
	cases := []struct {
		proto string
		port  int
		want  bool
	}{
		{"tcp", 443, true},
		{"udp", 443, false},
		{"tcp", 8080, true},
		{"tcp", 8081, false},
		{"udp", 25000, true},
		{"udp", 30000, true},
		{"tcp", 25000, false},
		{"tcp", 9000, false}, // DROP 不算放行
		{"tcp", 22, false},
	}
	for _, tc := range cases {
		if got := iptablesOutputCoversPort(output, tc.proto, tc.port); got != tc.want {
			t.Fatalf("iptablesOutputCoversPort(%s/%d) = %v, want %v", tc.proto, tc.port, got, tc.want)
		}
	}

	// 没有 -p 的 ACCEPT 覆盖所有协议
	allProto := "-A INPUT -m comment --comment all --dport 12345 -j ACCEPT"
	if !iptablesOutputCoversPort(allProto, "udp", 12345) {
		t.Fatal("rule without -p should cover every protocol")
	}
}

func TestUFWStatusCoversPort(t *testing.T) {
	output := `Status: active

To                         Action      From
--                         ------      ----
443/tcp                    ALLOW       Anywhere
50001:50020/udp            ALLOW       Anywhere
9000/tcp                   DENY        Anywhere
22/tcp                     ALLOW       192.168.1.0/24`
	cases := []struct {
		proto string
		port  int
		want  bool
	}{
		{"tcp", 443, true},
		{"udp", 443, false},
		{"udp", 50010, true},
		{"udp", 50021, false},
		{"tcp", 9000, false},
		{"tcp", 22, true},
		{"tcp", 80, false},
	}
	for _, tc := range cases {
		if got := ufwStatusCoversPort(output, tc.proto, tc.port); got != tc.want {
			t.Fatalf("ufwStatusCoversPort(%s/%d) = %v, want %v", tc.proto, tc.port, got, tc.want)
		}
	}
}

func TestNftOutputCoversPort(t *testing.T) {
	output := `chain input {
		type filter hook input priority filter; policy drop;
		iif "lo" accept
		tcp dport 443 counter packets 0 bytes 0 accept
		tcp dport { 80, 8443 } accept
		udp dport 20000-30000 accept
		tcp dport 9000 drop
		meta l4proto udp th dport 5353 accept
		counter packets 0 bytes 0 drop
}`
	cases := []struct {
		proto string
		port  int
		want  bool
	}{
		{"tcp", 443, true},
		{"udp", 443, false},
		{"tcp", 8443, true},
		{"tcp", 8444, false},
		{"udp", 25000, true},
		{"udp", 30001, false},
		{"tcp", 9000, false}, // drop 不算放行
		{"udp", 5353, true},
		{"tcp", 5353, false},
		{"tcp", 22, false},
	}
	for _, tc := range cases {
		if got := nftOutputCoversPort(output, tc.proto, tc.port); got != tc.want {
			t.Fatalf("nftOutputCoversPort(%s/%d) = %v, want %v", tc.proto, tc.port, got, tc.want)
		}
	}
}

// 删除时只清理我们自己添加的精确规则，管理员的范围/集合规则要保留。
func TestNftLineExactPort(t *testing.T) {
	cases := []struct {
		line  string
		proto string
		port  int
		want  bool
	}{
		{"tcp dport 443 counter packets 0 bytes 0 accept", "tcp", 443, true},
		{"tcp dport 443 accept", "tcp", 443, true},
		{"tcp dport 8443 accept", "tcp", 443, false},
		{"tcp dport { 443, 8443 } accept", "tcp", 443, false},
		{"tcp dport 20000-30000 accept", "tcp", 25000, false},
		{"udp dport 443 accept", "tcp", 443, false},
	}
	for _, tc := range cases {
		if got := nftLineExactPort(strings.Fields(tc.line), tc.proto, tc.port); got != tc.want {
			t.Fatalf("nftLineExactPort(%q, %s/%d) = %v, want %v", tc.line, tc.proto, tc.port, got, tc.want)
		}
	}
}

// `nft -a list chain` 的 drop 行尾是 "# handle N"：
// 判定 drop 必须看注释前的规则体，否则新 accept 会被 append 到兜底 drop 之后（永远不生效）。
func TestNftDenyRule(t *testing.T) {
	cases := []struct {
		line   string
		handle string
		deny   bool
	}{
		{"\t\tcounter packets 292588 bytes 15547311 drop # handle 15", "15", true},
		{"\t\tip saddr 142.171.0.0/16 drop # handle 12", "12", true},
		{"\t\tudp dport 443 reject # handle 7", "7", true},
		{"\t\ttcp dport 39999 accept # handle 4", "", false},
		{"\t\ttype filter hook input priority filter; policy drop;", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		handle, deny := nftDenyRule(tc.line)
		if deny != tc.deny || handle != tc.handle {
			t.Fatalf("nftDenyRule(%q) = (%q,%v), want (%q,%v)", tc.line, handle, deny, tc.handle, tc.deny)
		}
	}
}

// firewalld 的 --list-ports 里已有范围规则时不能再加单端口条目。
func TestFirewalldListCovers(t *testing.T) {
	list := "443/tcp 20000-50000/udp 60000-60010/tcp"
	cases := []struct {
		spec  string
		proto string
		want  bool
	}{
		{"443", "tcp", true},
		{"443", "udp", false},
		{"30000", "udp", true},
		{"60005", "tcp", true},
		{"50001", "udp", false},
		{"20000-29999", "udp", true},
		{"40000-50001", "udp", false},
	}
	for _, tc := range cases {
		if got := firewalldListCovers(list, tc.spec, tc.proto); got != tc.want {
			t.Fatalf("firewalldListCovers(%q,%q) = %v, want %v", tc.spec, tc.proto, got, tc.want)
		}
	}
}
