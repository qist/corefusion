package proxy

import (
	"context"
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// nginx 带 stream_pp_var 补丁时，PROXY Protocol 要按 SNI 分别发送：
//   - 版本 v2 的后端 → v2
//   - 版本 v1 的后端 → on（v1 文本头）
//   - 关闭 / 未配置的后端 → off
//
// 这样同一个 443 才能同时承载 xray、sing-box 与不需要 PP 的后端。
func TestNginxStreamProxyProtocolPerSNIWhenSupported(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "x.example.com", Template: "sni", Path: "127.0.0.1:24443",
			SNITarget: "127.0.0.1:28443", ProxyProtocol: true, ProxyProtocolVersion: "v2", Enabled: true},
		{ServerID: 2, Domain: "legacy.example.com", Template: "sni", Path: "127.0.0.1:24443",
			SNITarget: "127.0.0.1:28445", ProxyProtocol: true, ProxyProtocolVersion: "v1", Enabled: true},
		{ServerID: 2, Domain: "sb.example.com", Template: "sni", Path: "127.0.0.1:24443",
			SNITarget: "127.0.0.1:28444", ProxyProtocolVersion: "off", Enabled: true},
	}
	m := newNginxTestManager(t, sites, `{"stream_sni_enabled":true}`)
	m.SetNginxCapabilityFunc(func(context.Context, int64) ([]string, error) {
		return []string{NginxCapStreamPPVar}, nil
	})

	cfg := generateFor(t, m)
	for _, want := range []string{
		"map $ssl_preread_server_name $sni_pp {",
		"x.example.com v2;",
		"legacy.example.com on;",
		"sb.example.com off;",
		"proxy_protocol $sni_pp;",
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("缺少 %q：\n%s", want, cfg)
		}
	}
	if strings.Contains(cfg, "proxy_protocol on;") {
		t.Fatalf("支持变量时不应再整组发 v1：\n%s", cfg)
	}
}

// 全局开了 PP 时，未显式勾选的手建站点要保持旧行为（v1），
// 但 sing-box 派生规则（NoProxyProtocol）必须 off —— 这是升级兼容的关键。
func TestNginxStreamProxyProtocolValueCompat(t *testing.T) {
	cases := []struct {
		name             string
		noPP             bool
		version          string
		sitePP, globalPP bool
		want             string
	}{
		{"singbox 派生永远 off", true, "v2", true, true, "off"},
		{"版本 v2", false, "v2", false, false, "v2"},
		{"版本 v1", false, "v1", false, false, "on"},
		{"版本 off 覆盖全局", false, "off", true, true, "off"},
		{"未指定 + 站点勾选：旧行为 v1", false, "", true, false, "on"},
		{"未指定 + 全局开：保持 v1", false, "", false, true, "on"},
		{"全关", false, "", false, false, "off"},
		{"非法版本按未指定处理", false, "bogus", false, false, "off"},
	}
	for _, c := range cases {
		if got := nginxSiteProxyProtocolValue(c.noPP, c.version, c.sitePP, c.globalPP); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}

	// 派生合并必须给 sing-box 规则打上 NoProxyProtocol。
	merged := mergeDerivedNginxSNISites(2, nil, []derivedSNIRule{
		{Domain: "sb.example.com", Target: "127.0.0.1:28444", FromSingbox: true},
		{Domain: "x.example.com", Target: "127.0.0.1:28443", ProxyProtocol: true},
	})
	if len(merged) != 2 || !merged[0].NoProxyProtocol || merged[1].NoProxyProtocol {
		t.Fatalf("派生标记不正确: %+v", merged)
	}
}

// 老 nginx（无补丁）必须回退整组 on，行为与升级前一致。
func TestNginxStreamProxyProtocolFallsBackWhenUnsupported(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "x.example.com", Template: "sni", Path: "127.0.0.1:24443",
			SNITarget: "127.0.0.1:28443", ProxyProtocol: true, Enabled: true},
	}
	m := newNginxTestManager(t, sites, `{"stream_sni_enabled":true,"stream_proxy_protocol":true}`)
	m.SetNginxCapabilityFunc(func(context.Context, int64) ([]string, error) {
		return nil, nil // 老版本 / 探测失败
	})

	cfg := generateFor(t, m)
	if !strings.Contains(cfg, "proxy_protocol on;") {
		t.Fatalf("不支持变量时应回退整组 proxy_protocol on：\n%s", cfg)
	}
	if strings.Contains(cfg, "$sni_pp") {
		t.Fatalf("不支持变量时不应生成 $sni_pp：\n%s", cfg)
	}
}

// 探测函数未注入时（例如单元测试/特殊部署）同样按不支持处理，绝不能让
// 生成器写出可能无法启动的配置。
func TestNginxStreamProxyProtocolNoProvider(t *testing.T) {
	sites := []entity.NginxSite{
		{ServerID: 2, Domain: "x.example.com", Template: "sni", Path: "127.0.0.1:24443",
			SNITarget: "127.0.0.1:28443", ProxyProtocol: true, Enabled: true},
	}
	cfg := generateFor(t, newNginxTestManager(t, sites, `{"stream_sni_enabled":true}`))
	if strings.Contains(cfg, "$sni_pp") {
		t.Fatalf("没有能力探测时不应生成变量模式：\n%s", cfg)
	}
}
