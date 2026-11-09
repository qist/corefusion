package router

import "testing"

// 面板转发站点判断依赖域名归一：WAF 域名可能带端口或是逗号分隔的域名组，
// 不归一会出现「站点是 panel_forward 但启用站点级 WAF 时补不上白名单」。
func TestNormalizeWafMatchDomain(t *testing.T) {
	cases := map[string]string{
		"panel.example.com":                "panel.example.com",
		"Panel.Example.com:8443":           "panel.example.com",
		"panel.example.com.":               "panel.example.com",
		"panel.example.com, a.example.com": "panel.example.com",
		"  a.example.com:443 , b.com ":     "a.example.com",
		"":                                 "",
	}
	for in, want := range cases {
		if got := normalizeWafMatchDomain(in); got != want {
			t.Errorf("normalizeWafMatchDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPanelForwardDomainExistsWithoutRepos(t *testing.T) {
	if panelForwardDomainExists(nil, nil, nil, 1, "panel.example.com") {
		t.Fatal("expected false when no site repositories are configured")
	}
}
