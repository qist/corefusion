package dnsrecord

import (
	"strings"
	"testing"
)

// 能力表：已实现的标记 implemented，只有真机验证过的标 verified；
// 证书支持但未实现的服务商也要列出来（前端解释为什么不能选）。
func TestCapabilitiesMarksImplementedAndVerified(t *testing.T) {
	caps := Capabilities([]string{"cloudflare", "alidns", "tencentcloud"})
	byProvider := map[string]ProviderCapability{}
	for _, c := range caps {
		byProvider[c.Provider] = c
	}

	if c := byProvider["cloudflare"]; !c.Implemented || !c.Verified {
		t.Errorf("cloudflare 应为已实现+已验证：%+v", c)
	}
	if c := byProvider["alidns"]; !c.Implemented || c.Verified {
		t.Errorf("alidns 应为已实现+未验证：%+v", c)
	}
	for _, p := range []string{"tencentcloud"} {
		c := byProvider[p]
		if c.Implemented {
			t.Errorf("%s 不应标记为已实现：%+v", p, c)
		}
		if c.Note == "" {
			t.Errorf("%s 未实现时应给出说明文案", p)
		}
	}
	if len(caps) < 3 {
		t.Errorf("能力表至少应包含 3 条，实际 %d：%+v", len(caps), caps)
	}
}

// 未实现的服务商调用时必须明确报错，而不是静默失败。
func TestNewUpdaterRejectsUnimplementedProvider(t *testing.T) {
	_, err := newUpdater("tencentcloud", map[string]string{"TENCENTCLOUD_SECRET_ID": "x"}, ProviderConfig{})
	if err == nil {
		t.Fatal("未实现的服务商应报错")
	}
	if !strings.Contains(err.Error(), "尚未实现") {
		t.Errorf("错误信息应说明未实现，实际：%v", err)
	}
}

// 凭据缺失时构造适配器要失败（不能等到调用 API 才报）。
func TestNewUpdaterRequiresCredentials(t *testing.T) {
	if _, err := newUpdater("alidns", map[string]string{}, ProviderConfig{}); err == nil {
		t.Error("阿里云缺 AccessKey 时应报错")
	}
	if _, err := newUpdater("cloudflare", map[string]string{}, ProviderConfig{}); err == nil {
		t.Error("Cloudflare 缺凭据时应报错")
	}
}

// 阿里云 RR 计算：域名本身 → @，子域 → 相对主机名。
func TestAliDNSRecordRR(t *testing.T) {
	cases := []struct{ fqdn, zone, want string }{
		{"entry.tycng.com", "tycng.com", "entry"},
		{"a.b.tycng.com", "tycng.com", "a.b"},
		{"tycng.com", "tycng.com", "@"},
		{"entry.other.com", "tycng.com", ""},
	}
	for _, c := range cases {
		if got := recordRR(c.fqdn, c.zone); got != c.want {
			t.Errorf("recordRR(%q,%q)=%q, want %q", c.fqdn, c.zone, got, c.want)
		}
	}
}

// 通道状态非 active 时直接拒绝（避免拿停用的凭据改线上解析）。
func TestUpdaterRejectsInactiveChannel(t *testing.T) {
	s := NewService(nil)
	if _, err := s.updaterForChannel(nil); err == nil {
		t.Error("nil 通道应报错")
	}
}
