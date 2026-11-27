package subscriptions

import (
	"strings"
	"testing"
)

func TestNodeDisplayName(t *testing.T) {
	cases := []struct {
		remark string
		tag    string
		want   string
	}{
		// 管理员起的名字原样使用（含端口也是显式选择）
		{"香港 CN2", "inbound-vless-20001", "香港 CN2"},
		{"HK-443", "inbound-vless-20001", "HK-443"},
		// remark 为空：入站 tag 只留协议，端口不能出现在节点名里
		{"", "inbound-vless-20001", "vless"},
		{"", "inbound-trojan-443", "trojan"},
		{"", "inbound-hysteria2-51071", "hysteria2"},
		{"", "inbound-vmess-8880", "vmess"},
		// 非标准 tag（xray-ui 迁移）：去掉纯数字段
		{"", "in-443-tcp", "in-tcp"},
		{"", "vless_20001_ws", "vless-ws"},
		{"", "node:20001", "node"},
		{"", "12345", "node"},
		// remark 是自动名（inbound-<协议>-<端口>）：按 tag 脱敏，端口不外泄
		{"inbound-vless-20001", "inbound-vless-20001", "vless"},
		{"inbound-hysteria2-51071", "inbound-hysteria2-51071", "hysteria2"},
		// 任意语言的 remark 原样保留
		{"香港 01", "inbound-vless-20001", "香港 01"},
		{"東京・プレミアム", "inbound-vless-1", "東京・プレミアム"},
		{"Москва", "inbound-vless-1", "Москва"},
		{"تهران", "inbound-vless-1", "تهران"},
		{"Frankfurt-01", "inbound-vless-1", "Frankfurt-01"},
		// 空白与空值
		{"   ", "", "node"},
		{"", "", "node"},
	}
	for _, c := range cases {
		if got := nodeDisplayName(c.remark, c.tag); got != c.want {
			t.Errorf("nodeDisplayName(%q, %q) = %q, want %q", c.remark, c.tag, got, c.want)
		}
	}
}

// 回归：入站 remark 为空时，订阅/分享里的节点名不能出现入站端口。
func TestNodeDisplayNameNeverLeaksPortFromTag(t *testing.T) {
	for _, tag := range []string{
		"inbound-vless-20001", "inbound-trojan-51878", "in-443-tcp", "inbound-hysteria2-51071",
	} {
		name := nodeDisplayName("", tag)
		for _, port := range []string{"20001", "51878", "443", "51071"} {
			if strings.Contains(name, port) {
				t.Fatalf("tag %q 生成的节点名 %q 泄露端口 %s", tag, name, port)
			}
		}
	}
}

// 同名节点必须自动加后缀：sing-box/xray 的 outbound tag 重名会整份拒载。
func TestUniqueNodeName(t *testing.T) {
	used := map[string]int{}
	cases := []struct{ in, want string }{
		{"香港 01", "香港 01"},
		{"香港 01", "香港 01-2"},
		{"香港 01", "香港 01-3"},
		{"東京", "東京"},
		{"東京", "東京-2"},
		{"vless", "vless"},
		{"vless", "vless-2"},
		{"", "node"},
		{"", "node-2"},
	}
	for _, c := range cases {
		if got := uniqueNodeName(used, c.in); got != c.want {
			t.Errorf("uniqueNodeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 节点名不能占用订阅生成器保留的 tag，否则注入的 proxy 组会缺失。
func TestNodeNameRegistryReservesOutboundTags(t *testing.T) {
	used := newNodeNameRegistry()
	for _, reserved := range []string{"proxy", "direct", "block"} {
		if got := uniqueNodeName(used, reserved); got == reserved {
			t.Fatalf("保留名 %q 未被占用", reserved)
		}
	}
	// 普通名字不受影响
	if got := uniqueNodeName(used, "香港 01"); got != "香港 01" {
		t.Fatalf("普通名字被改写: %q", got)
	}
}
