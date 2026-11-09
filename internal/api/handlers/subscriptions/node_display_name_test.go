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
