package subscriptions

import "testing"

func TestNormalizeSubscriptionFormat(t *testing.T) {
	cases := map[string]string{
		"":                    "base64_uri",
		"base64":              "base64_uri",
		"clash":               "mihomo_yaml",
		"Clash":               "mihomo_yaml",
		"mihomo":              "mihomo_yaml",
		"mihomo_yaml":         "mihomo_yaml",
		"mihomo_proxies_yaml": "mihomo_proxies_yaml",
		"sing-box":            "singbox_json",
		"singbox":             "singbox_json",
		"xray":                "xray_json",
		"xray_json":           "xray_json",
		"raw_uri":             "raw_uri",
		"unknown_fmt":         "unknown_fmt",
	}
	for in, want := range cases {
		if got := normalizeSubscriptionFormat(in); got != want {
			t.Fatalf("normalizeSubscriptionFormat(%q) = %q, want %q", in, got, want)
		}
	}
}
