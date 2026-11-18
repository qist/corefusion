package coreconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDropMissingSingboxLocalRuleSets(t *testing.T) {
	dir := t.TempDir()
	// 只放广告文件，RU 文件缺失
	if err := os.WriteFile(filepath.Join(dir, "geosite-category-ads-all.srs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{
		"route": map[string]interface{}{
			"rule_set": []interface{}{
				map[string]interface{}{"tag": "geosite-category-ads-all", "type": "local", "format": "binary", "path": "rules/geosite-category-ads-all.srs"},
				map[string]interface{}{"tag": "geosite-category-ru", "type": "local", "format": "binary", "path": "rules/geosite-category-ru.srs"},
				map[string]interface{}{"tag": "remote-set", "type": "remote", "format": "binary", "url": "https://example.com/x.srs"},
			},
			"rules": []interface{}{
				map[string]interface{}{"rule_set": []interface{}{"geosite-category-ads-all"}, "outbound": "block"},
				map[string]interface{}{"rule_set": []interface{}{"geosite-category-ru"}, "outbound": "warp-wg"},
				map[string]interface{}{"rule_set": []interface{}{"geosite-category-ru", "geosite-category-ads-all"}, "outbound": "warp-wg"},
				map[string]interface{}{"domain_suffix": []interface{}{"example.com"}, "outbound": "direct"},
			},
			"final": "direct",
		},
	}

	removed := DropMissingSingboxLocalRuleSets(cfg, dir)
	if removed != 1 {
		t.Fatalf("应摘掉 1 个缺失 rule_set，got %d", removed)
	}

	route := cfg["route"].(map[string]interface{})
	sets := route["rule_set"].([]interface{})
	if len(sets) != 2 {
		t.Fatalf("rule_set 剩余 %d, want 2（保留 ads + remote）", len(sets))
	}
	for _, item := range sets {
		if m, _ := item.(map[string]interface{}); m != nil && m["tag"] == "geosite-category-ru" {
			t.Errorf("缺失文件的 rule_set 未被摘掉")
		}
	}

	rules := route["rules"].([]interface{})
	// 纯 RU 规则应被删除；混合规则应保留 ads 引用；普通规则保留
	if len(rules) != 3 {
		t.Fatalf("rules 剩余 %d, want 3", len(rules))
	}
	for _, item := range rules {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		vals, _ := m["rule_set"].([]interface{})
		for _, v := range vals {
			if v == "geosite-category-ru" {
				t.Errorf("规则仍引用缺失 rule_set: %v", m)
			}
		}
	}
}

func TestDropMissingSingboxLocalRuleSetsKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "geosite-cn.srs"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]interface{}{
		"route": map[string]interface{}{
			"rule_set": []interface{}{
				map[string]interface{}{"tag": "geosite-cn", "type": "local", "path": "rules/geosite-cn.srs"},
			},
			"rules": []interface{}{
				map[string]interface{}{"rule_set": []interface{}{"geosite-cn"}, "outbound": "direct"},
			},
		},
	}
	if n := DropMissingSingboxLocalRuleSets(cfg, dir); n != 0 {
		t.Fatalf("文件存在时不应摘除，got %d", n)
	}
}

// 路径穿越/空路径必须按缺失处理，不能去读 rules 目录外的文件。
func TestLocalRuleFileExistsRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	if localRuleFileExists(dir, "../../etc/passwd") {
		t.Errorf("不应允许路径穿越")
	}
	if localRuleFileExists(dir, "") {
		t.Errorf("空路径应视为缺失")
	}
}
