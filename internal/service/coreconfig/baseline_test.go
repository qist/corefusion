package coreconfig

import (
	"context"
	"encoding/json"
	"testing"
)

// 存量节点补基线：必须写进 core_configs（可编辑/删除），且只补缺、不覆盖用户规则。
func TestEnsureBaselineRoutingXrayFillsMissing(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	// 模拟老节点：只有用户自己的规则，没有任何默认基线。
	userRule := map[string]interface{}{
		"type": "field", "domain": []interface{}{"domain:example.com"}, "outboundTag": "direct",
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"domainStrategy": "IPIfNonMatch",
		"rules":          []interface{}{userRule},
	})
	if err := svc.SaveSection(ctx, 1, "xray", "routing", raw); err != nil {
		t.Fatalf("save routing: %v", err)
	}

	added, err := svc.EnsureBaselineRouting(ctx, 1, "xray")
	if err != nil {
		t.Fatalf("ensure baseline: %v", err)
	}
	if added != 3 {
		t.Fatalf("应补 3 条基线，got %d", added)
	}

	routing := sectionMap(t, svc, "xray", "routing")
	rules, _ := routing["rules"].([]interface{})
	if len(rules) != 4 {
		t.Fatalf("规则数 = %d, want 4（1 用户 + 3 基线）", len(rules))
	}
	if !ruleListContains(rules, "ip", "geoip:private") {
		t.Errorf("缺少内网直连规则")
	}
	if !ruleListContains(rules, "domain", "domain:gstatic.com") {
		t.Errorf("缺少 Google 直连规则")
	}
	if !ruleListContains(rules, "domain", "geosite:category-ads-all") {
		t.Errorf("缺少广告拦截规则")
	}
	// 用户规则必须原样保留
	if !ruleListContains(rules, "domain", "domain:example.com") {
		t.Errorf("用户规则被覆盖: %v", rules)
	}
	// 基线规则必须带 ruleTag，前端才能稳定识别/删除
	foundTag := false
	for _, item := range rules {
		m, _ := item.(map[string]interface{})
		if m != nil && m["ruleTag"] == baselineTagAds {
			foundTag = true
		}
	}
	if !foundTag {
		t.Errorf("基线规则缺少 ruleTag: %v", rules)
	}

	// 幂等：再跑一次不应重复添加
	again, err := svc.EnsureBaselineRouting(ctx, 1, "xray")
	if err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	if again != 0 {
		t.Errorf("重复补基线应为 0，got %d", again)
	}
}

func TestEnsureBaselineRoutingSingboxAddsRuleSet(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	// 模拟老节点：route 段只有用户规则，没有广告 rule_set。
	raw, _ := json.Marshal(map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{"domain_suffix": []interface{}{"example.com"}, "outbound": "direct"},
		},
		"final": "direct",
	})
	if err := svc.SaveSection(ctx, 1, "singbox", "route", raw); err != nil {
		t.Fatalf("save route: %v", err)
	}

	added, err := svc.EnsureBaselineRouting(ctx, 1, "singbox")
	if err != nil {
		t.Fatalf("ensure baseline: %v", err)
	}
	if added < 3 {
		t.Fatalf("应补 3 条规则（+1 rule_set 定义），got %d", added)
	}

	route := sectionMap(t, svc, "singbox", "route")
	rules, _ := route["rules"].([]interface{})
	if !singboxBaselineRuleExists(rules, map[string]interface{}{"ip_is_private": true}) {
		t.Errorf("缺少 ip_is_private 直连规则")
	}
	if !ruleListContains(rules, "domain_suffix", "gstatic.com") {
		t.Errorf("缺少 Google 直连规则")
	}
	if !ruleListContains(rules, "rule_set", singboxAdsRuleSetTag) {
		t.Errorf("缺少广告 rule_set 规则")
	}

	sets, _ := route["rule_set"].([]interface{})
	if !ruleSetTagExists(sets, singboxAdsRuleSetTag) {
		t.Fatalf("缺少广告 rule_set 定义: %v", sets)
	}
	for _, item := range sets {
		m, _ := item.(map[string]interface{})
		if m != nil && m["tag"] == singboxAdsRuleSetTag {
			if m["type"] != "local" || m["format"] != "binary" || m["path"] != singboxAdsRuleSetFile {
				t.Errorf("rule_set 定义不符合预期: %v", m)
			}
		}
	}

	again, err := svc.EnsureBaselineRouting(ctx, 1, "singbox")
	if err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	if again != 0 {
		t.Errorf("重复补基线应为 0，got %d", again)
	}
}

// 用户删掉基线后，只要标记已写过，就不该由 service 自动恢复；
// 这里只验证「单次调用不会重复加」，一次性语义由 router 的 marker 保证。
func TestEnsureBaselineRoutingDoesNotTouchExistingEquivalent(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	// 已有等价规则但没有 ruleTag（历史数据）
	raw, _ := json.Marshal(map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{"type": "field", "ip": []interface{}{"geoip:private"}, "outboundTag": "direct"},
		},
	})
	if err := svc.SaveSection(ctx, 1, "xray", "routing", raw); err != nil {
		t.Fatalf("save routing: %v", err)
	}
	added, err := svc.EnsureBaselineRouting(ctx, 1, "xray")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if added != 2 {
		t.Fatalf("已有 geoip:private 时只应补 2 条，got %d", added)
	}
	routing := sectionMap(t, svc, "xray", "routing")
	rules, _ := routing["rules"].([]interface{})
	count := 0
	for _, item := range rules {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		if vals, ok := m["ip"].([]interface{}); ok && len(vals) == 1 && vals[0] == "geoip:private" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("geoip:private 规则重复: %d 条", count)
	}
}
