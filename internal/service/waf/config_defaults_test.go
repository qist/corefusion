package waf

import (
	"encoding/json"
	"strings"
	"testing"
)

// 存量 caddy config.json 缺 cc_mode / match_cache：必须补齐，且已有值不能被改。
func TestMergeCaddyWAFConfigDefaults(t *testing.T) {
	content := `{
    "waf_enable": "off",
    "trust_proxy_headers": "on",
    "cc_check": "on",
    "cc_rate": "150/60"
}`
	out, changed := mergeWAFConfigDefaults("caddy", content)
	if !changed {
		t.Fatal("应识别出缺失键")
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, out)
	}
	for _, k := range []string{"cc_mode", "match_cache", "score_block_threshold", "url_check", "waf_output"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("缺少 %s: %v", k, m)
		}
	}
	if m["waf_enable"] != "off" || m["cc_rate"] != "150/60" {
		t.Fatalf("已有值被覆盖: %v", m)
	}
	// 幂等
	if _, changed2 := mergeWAFConfigDefaults("caddy", out); changed2 {
		t.Fatal("第二次不应再有变化")
	}
}

// 非法内容不能被改坏
func TestMergeCaddyWAFConfigDefaultsInvalidJSON(t *testing.T) {
	content := `{"waf_enable": "on",`
	out, changed := mergeWAFConfigDefaults("caddy", content)
	if changed || out != content {
		t.Fatalf("非法 JSON 应原样返回，got changed=%v out=%q", changed, out)
	}
}

// 存量 config.lua 缺 config_match_cache / config_cc_mode：补到 config_output_html 之前，
// 且不能重复补、不能覆盖已有值、不能补路径类键。
func TestMergeNginxWAFConfigDefaults(t *testing.T) {
	content := `--NginxGuard config file
config_waf_enable = "off"
config_cc_rate = "150/60"
config_cc_block_ttl = 600
config_output_html=[[
<html></html>
]]
`
	out, changed := mergeWAFConfigDefaults("nginx", content)
	if !changed {
		t.Fatal("应识别出缺失键")
	}
	for _, key := range []string{"config_match_cache", "config_cc_mode", "config_score_block_threshold", "config_bodyless"} {
		if !strings.Contains(out, key) {
			t.Fatalf("缺少 %s:\n%s", key, out)
		}
	}
	if strings.Contains(out, "config_rule_dir") || strings.Contains(out, "config_log_dir") {
		t.Fatalf("路径类键不应补: \n%s", out)
	}
	if !strings.Contains(out, `config_waf_enable = "off"`) || !strings.Contains(out, `config_cc_rate = "150/60"`) {
		t.Fatal("已有值被覆盖")
	}
	// 补的内容必须在 output_html 之前，否则会插进多行块的中间
	idxCache := strings.Index(out, "config_match_cache")
	idxOutput := strings.Index(out, "config_output_html")
	if idxCache < 0 || idxOutput < 0 || idxCache > idxOutput {
		t.Fatalf("补插位置不对: cache=%d output=%d\n%s", idxCache, idxOutput, out)
	}
	if _, changed2 := mergeWAFConfigDefaults("nginx", out); changed2 {
		t.Fatalf("第二次不应再有变化:\n%s", out)
	}
}

// 域名配置只有 rule_dir/waf_enable 时，按全局补齐其余开关；rule_dir 保留原值。
func TestMergeDomainConfigDefaults(t *testing.T) {
	domain := `{"rule_dir":"domains/p.tycng.com","waf_enable":"on"}`
	out := mergeDomainConfigDefaults(DefaultCaddyWAFConfig, domain)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if m["rule_dir"] != "domains/p.tycng.com" || m["waf_enable"] != "on" {
		t.Fatalf("关键字段被改: %v", m)
	}
	for _, k := range []string{"cc_mode", "score_block_threshold", "post_check"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("缺少 %s: %v", k, m)
		}
	}
	// 即便历史条目里已经写了 match_cache，也要在输出时清掉（上游仅全局）
	withMatch := `{"rule_dir":"domains/a.com","waf_enable":"on","match_cache":"off"}`
	out2 := mergeDomainConfigDefaults(DefaultCaddyWAFConfig, withMatch)
	var m2 map[string]interface{}
	if err := json.Unmarshal([]byte(out2), &m2); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if _, ok := m2["match_cache"]; ok {
		t.Fatalf("match_cache 应从站点级清掉: %v", m2)
	}
	// 路径类与 match_cache（上游仅全局）不补到站点级
	for _, k := range []string{"log_dir", "match_cache"} {
		if _, ok := m[k]; ok {
			t.Fatalf("%s 不应补到站点级: %v", k, m)
		}
	}
	// 幂等
	if again := mergeDomainConfigDefaults(DefaultCaddyWAFConfig, out); again != out {
		t.Fatalf("第二次不应变化:\n%s\n%s", out, again)
	}
}

// 域名配置非法 JSON 原样返回
func TestMergeDomainConfigDefaultsInvalid(t *testing.T) {
	bad := `{"waf_enable":`
	if out := mergeDomainConfigDefaults(DefaultCaddyWAFConfig, bad); out != bad {
		t.Fatalf("非法 JSON 应原样返回: %q", out)
	}
}
