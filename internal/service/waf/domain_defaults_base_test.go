package waf

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPickDomainDefaultsBasePrefersRunningProxy(t *testing.T) {
	nginxText := "config_cc_rate = \"150/60\"\n"
	caddyText := `{"cc_rate":"200/30"}`

	cases := []struct {
		name          string
		nginx, caddy  string
		localProxy    string
		wantType      string
		wantSubstring string
	}{
		{"local caddy wins over nginx", nginxText, caddyText, "caddy", "caddy", "200/30"},
		{"local nginx wins over caddy", nginxText, caddyText, "nginx", "nginx", "150/60"},
		{"remote both configured falls back to nginx", nginxText, caddyText, "", "nginx", "150/60"},
		{"remote caddy only", "", caddyText, "", "caddy", "200/30"},
		{"remote nginx only", nginxText, "", "", "nginx", "150/60"},
		{"nothing configured uses nginx default", "", "", "", "nginx", "config_waf_enable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base, typ := pickDomainDefaultsBase(c.nginx, c.caddy, c.localProxy)
			if typ != c.wantType {
				t.Fatalf("base type = %q, want %q", typ, c.wantType)
			}
			if !strings.Contains(base, c.wantSubstring) {
				t.Fatalf("base does not contain %q: %q", c.wantSubstring, base)
			}
		})
	}
}

func TestNginxConfigMapParsesAssignments(t *testing.T) {
	m := nginxConfigMap(DefaultNginxWAFConfig)
	if m == nil {
		t.Fatal("nginxConfigMap returned nil")
	}
	if m["cc_rate"] != "150/60" {
		t.Fatalf("cc_rate = %#v, want \"150/60\"", m["cc_rate"])
	}
	if n, ok := m["cc_block_ttl"].(float64); !ok || n != 600 {
		t.Fatalf("cc_block_ttl = %#v, want float64(600)", m["cc_block_ttl"])
	}
	if _, ok := m["output_html"]; ok {
		t.Fatal("multi-line output_html must not be parsed as a site-level key")
	}
	if _, ok := m["config_waf_enable"]; ok {
		t.Fatal("config_ prefix must be stripped")
	}
	if m["rule_dir"] == "" || m["log_dir"] == "" || m["match_cache"] == "" {
		t.Fatal("path/global keys should still be present in the raw map (filtered by merge)")
	}
}

// 站点级补缺以 NginxGuard 全局为基线时，值与键名都必须来自 nginx 侧，
// 且全局专属键（rule_dir/log_dir/match_cache）不落进域名条目。
func TestMergeDomainDefaultsFromNginxBase(t *testing.T) {
	base := nginxConfigMap(DefaultNginxWAFConfig)
	domain := `{"waf_enable":"off","rule_dir":"domains/example.com"}`

	out := mergeDomainConfigDefaultsFromMap(base, domain)

	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal result: %v (%s)", err, out)
	}
	if got["waf_enable"] != "off" {
		t.Fatalf("existing key overwritten: %#v", got["waf_enable"])
	}
	if got["rule_dir"] != "domains/example.com" {
		t.Fatalf("rule_dir overwritten: %#v", got["rule_dir"])
	}
	if got["cc_rate"] != "150/60" {
		t.Fatalf("cc_rate = %#v, want nginx global 150/60", got["cc_rate"])
	}
	if got["waf_redirect_url"] != "https://www.waf.com" {
		t.Fatalf("waf_redirect_url = %#v, want nginx global value", got["waf_redirect_url"])
	}
	for _, k := range []string{"log_dir", "match_cache"} {
		if _, ok := got[k]; ok {
			t.Fatalf("global-only key %q must not be copied into domain config", k)
		}
	}
	if len(got) != 26 {
		t.Fatalf("domain key count = %d, want 26 (25 site keys + rule_dir): %v", len(got), got)
	}
	if again := mergeDomainConfigDefaultsFromMap(base, out); again != out {
		t.Fatalf("merge not idempotent:\n%s\n%s", out, again)
	}
}
