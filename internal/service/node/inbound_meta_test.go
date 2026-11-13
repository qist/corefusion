package node

import "testing"

// 账号名后缀必须与 client.generateInboundEmail 完全一致，否则在线列表的
// 「入口」永远匹配不到。
func TestShortInboundTag(t *testing.T) {
	cases := map[string]string{
		"inbound-vless-20296":     "vless20296",
		"inbound-hysteria2-51071": "hysteria251071",
		"inbound-anytls-49146":    "anytls49146",
		"sb-traffic-test":         "traffictest", // 与 generateInboundEmail 一致：parts[1]+最后一段
		"":                        "",
	}
	for in, want := range cases {
		if got := shortInboundTag(in); got != want {
			t.Errorf("shortInboundTag(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAccountSuffix(t *testing.T) {
	cases := map[string]string{
		"user@example.com_vless20296": "vless20296",
		"user_name@example.com":       "", // 下划线在 @ 之前，不是 tag 后缀
		"user@example.com":            "",
		"":                            "",
	}
	for in, want := range cases {
		if got := accountSuffix(in); got != want {
			t.Errorf("accountSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

// clash_api 的 metadata.type 形如 "anytls/inbound-anytls-49146"。
func TestSingboxInboundTag(t *testing.T) {
	cases := map[string]string{
		"anytls/inbound-anytls-49146": "inbound-anytls-49146",
		"vless/inbound-vless-51697":   "inbound-vless-51697",
		"inbound-vless-51697":         "inbound-vless-51697",
		"":                            "",
	}
	for in, want := range cases {
		if got := singboxInboundTag(in); got != want {
			t.Errorf("singboxInboundTag(%q) = %q, want %q", in, got, want)
		}
	}
}
