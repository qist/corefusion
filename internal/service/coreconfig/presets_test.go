package coreconfig

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qist/sqlite"
	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

func newPresetTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.CoreConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := NewService(repository.NewCoreConfigRepository(db))
	if err := svc.SeedDefaults(context.Background(), 1); err != nil {
		t.Fatalf("seed defaults: %v", err)
	}
	return svc
}

func sectionMap(t *testing.T, svc *Service, coreType, section string) map[string]interface{} {
	t.Helper()
	raw, err := svc.GetSection(context.Background(), 1, coreType, section)
	if err != nil {
		t.Fatalf("get %s/%s: %v", coreType, section, err)
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		t.Fatalf("%s/%s 不是对象: %T", coreType, section, raw)
	}
	return m
}

// 默认路由只应包含：内网/Google 直连 + 广告拦截；CN/RU/IR 必须靠导入按钮加。
func TestDefaultRoutingOnlyHasBaselineRules(t *testing.T) {
	svc := newPresetTestService(t)

	xray := sectionMap(t, svc, "xray", "routing")
	rules, _ := xray["rules"].([]interface{})
	if len(rules) != 3 {
		t.Fatalf("xray 默认规则数 = %d, want 3: %v", len(rules), rules)
	}
	if !ruleListContains(rules, "ip", "geoip:private") {
		t.Errorf("xray 缺少 geoip:private 直连规则")
	}
	if !ruleListContains(rules, "domain", "geosite:category-ads-all") {
		t.Errorf("xray 缺少广告拦截规则")
	}
	if ruleListContains(rules, "domain", "geosite:cn") {
		t.Errorf("xray 不应默认包含 CN 规则")
	}

	sb := sectionMap(t, svc, "singbox", "route")
	sbRules, _ := sb["rules"].([]interface{})
	if len(sbRules) != 3 {
		t.Fatalf("sing-box 默认规则数 = %d, want 3: %v", len(sbRules), sbRules)
	}
	if !ruleListContains(sbRules, "rule_set", "geosite-category-ads-all") {
		t.Errorf("sing-box 缺少广告 rule_set 规则")
	}
	if ruleListContains(sbRules, "rule_set", "geosite-cn") {
		t.Errorf("sing-box 不应默认包含 CN rule_set")
	}
}

func TestImportRegionRouteXrayIdempotent(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	added, err := svc.ImportRegionRoute(ctx, 1, "xray", "cn", "direct")
	if err != nil {
		t.Fatalf("import cn: %v", err)
	}
	if added != 2 {
		t.Fatalf("首次导入应加 2 条（域名+IP），got %d", added)
	}

	again, err := svc.ImportRegionRoute(ctx, 1, "xray", "cn", "direct")
	if err != nil {
		t.Fatalf("import cn again: %v", err)
	}
	if again != 0 {
		t.Errorf("重复导入应为 0，got %d", again)
	}

	routing := sectionMap(t, svc, "xray", "routing")
	rules, _ := routing["rules"].([]interface{})
	if !ruleListContains(rules, "domain", "geosite:cn") {
		t.Errorf("缺少 CN 域名规则")
	}
	if !ruleListContains(rules, "ip", "geoip:cn") {
		t.Errorf("缺少 CN IP 规则")
	}
	// 默认三条必须原样保留
	if !ruleListContains(rules, "ip", "geoip:private") || !ruleListContains(rules, "domain", "geosite:category-ads-all") {
		t.Errorf("导入后默认规则被破坏: %v", rules)
	}
}

func TestImportRegionRouteXrayExtFiles(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	if _, err := svc.ImportRegionRoute(ctx, 1, "xray", "ir", "warp-wg"); err != nil {
		t.Fatalf("import ir: %v", err)
	}
	if _, err := svc.ImportRegionRoute(ctx, 1, "xray", "ru", "warp-masque"); err != nil {
		t.Fatalf("import ru: %v", err)
	}

	routing := sectionMap(t, svc, "xray", "routing")
	rules, _ := routing["rules"].([]interface{})
	if !ruleListContains(rules, "domain", "ext:geosite_IR.dat:ir") {
		t.Errorf("缺少 IR geosite ext 引用")
	}
	if !ruleListContains(rules, "ip", "ext:geoip_IR.dat:ir") {
		t.Errorf("缺少 IR geoip ext 引用")
	}
	if !ruleListContains(rules, "domain", "ext:geosite_RU.dat:ru-available-only-inside") {
		t.Errorf("缺少 RU geosite ext 引用")
	}

	var irRule, ruRule map[string]interface{}
	for _, item := range rules {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		switch m["outboundTag"] {
		case "warp-wg":
			irRule = m
		case "warp-masque":
			ruRule = m
		}
	}
	if irRule == nil || ruRule == nil {
		t.Fatalf("未按出口 tag 写入规则: %v", rules)
	}
}

func TestImportRegionRouteSingboxRuleSets(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	added, err := svc.ImportRegionRoute(ctx, 1, "singbox", "ru", "warp-wg")
	if err != nil {
		t.Fatalf("import ru: %v", err)
	}
	if added != 1 {
		t.Fatalf("sing-box 地区规则应合并为 1 条，got %d", added)
	}

	route := sectionMap(t, svc, "singbox", "route")
	sets, _ := route["rule_set"].([]interface{})
	tags := map[string]string{}
	for _, item := range sets {
		m, _ := item.(map[string]interface{})
		if m == nil {
			continue
		}
		tag, _ := m["tag"].(string)
		path, _ := m["path"].(string)
		tags[tag] = path
	}
	if tags["geosite-category-ru"] != "rules/geosite-category-ru.srs" {
		t.Errorf("缺少 RU 域名 rule_set: %v", tags)
	}
	if tags["geoip-ru"] != "rules/geoip-ru.srs" {
		t.Errorf("缺少 RU IP rule_set: %v", tags)
	}
	if tags["geosite-category-ads-all"] != "rules/geosite-category-ads-all.srs" {
		t.Errorf("默认广告 rule_set 丢失: %v", tags)
	}

	rules, _ := route["rules"].([]interface{})
	var ruRule map[string]interface{}
	for _, item := range rules {
		m, _ := item.(map[string]interface{})
		if m != nil && m["outbound"] == "warp-wg" {
			ruRule = m
		}
	}
	if ruRule == nil {
		t.Fatalf("缺少指向 warp-wg 的 sing-box 规则: %v", rules)
	}
	tagsInRule, _ := ruRule["rule_set"].([]interface{})
	if len(tagsInRule) != 2 {
		t.Errorf("RU 规则应引用两个 rule_set，got %v", tagsInRule)
	}

	// 幂等
	again, err := svc.ImportRegionRoute(ctx, 1, "singbox", "ru", "warp-wg")
	if err != nil {
		t.Fatalf("import ru again: %v", err)
	}
	if again != 0 {
		t.Errorf("重复导入应为 0，got %d", again)
	}
}

// 确保写入的 JSON 可被再次解析（SaveSection 只做 JSONC 校验，这里做一次真实回读）。
func TestImportedRoutingIsValidJSON(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()
	if _, err := svc.ImportRegionRoute(ctx, 1, "xray", "ru", "direct"); err != nil {
		t.Fatalf("import: %v", err)
	}
	raw, err := svc.GetSection(ctx, 1, "xray", "routing")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if _, err := json.Marshal(raw); err != nil {
		t.Fatalf("marshal back: %v", err)
	}
}

// 地区一键导入写入的规则必须自带规则名称（ruleTag），否则路由页名称列是空的。
func TestImportRegionRouteXrayAssignsRuleTags(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	if _, err := svc.ImportRegionRoute(ctx, 1, "xray", "ir", "warp-wg"); err != nil {
		t.Fatalf("import ir: %v", err)
	}

	routing := sectionMap(t, svc, "xray", "routing")
	rules, _ := routing["rules"].([]interface{})
	seen := map[string]bool{}
	checked := 0
	for _, item := range rules {
		rule, _ := item.(map[string]interface{})
		if rule == nil {
			continue
		}
		if rule["outboundTag"] != "warp-wg" {
			continue
		}
		checked++
		tag, _ := rule["ruleTag"].(string)
		if !isGeneratedRuleTag(tag) {
			t.Errorf("导入规则缺少合法 ruleTag: %q (%v)", tag, rule)
		}
		if seen[tag] {
			t.Errorf("ruleTag 重复: %q", tag)
		}
		seen[tag] = true
	}
	if checked != 2 {
		t.Fatalf("应有 2 条导入规则，实际 %d", checked)
	}
}

func isGeneratedRuleTag(tag string) bool {
	if len(tag) != len("rule-")+6 || !strings.HasPrefix(tag, "rule-") {
		return false
	}
	for _, r := range tag[len("rule-"):] {
		if !strings.ContainsRune(ruleTagAlphabet, r) {
			return false
		}
	}
	return true
}

// 存量路由里没有 ruleTag 的规则要一次性补齐；已有名字的保持原样，且幂等。
func TestBackfillXrayRuleTags(t *testing.T) {
	svc := newPresetTestService(t)
	ctx := context.Background()

	raw, _ := json.Marshal(map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{"type": "field", "domain": []interface{}{"domain:a.com"}, "outboundTag": "warp-wg"},
			map[string]interface{}{"type": "field", "ip": []interface{}{"geoip:cn"}, "outboundTag": "direct"},
			map[string]interface{}{"type": "field", "ruleTag": "keep-me", "ip": []interface{}{"geoip:private"}, "outboundTag": "direct"},
		},
	})
	if err := svc.SaveSection(ctx, 1, "xray", "routing", raw); err != nil {
		t.Fatalf("save routing: %v", err)
	}

	n, err := svc.BackfillXrayRuleTags(ctx, 1)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 2 {
		t.Fatalf("应补 2 条名字，got %d", n)
	}

	routing := sectionMap(t, svc, "xray", "routing")
	rules, _ := routing["rules"].([]interface{})
	if len(rules) != 3 {
		t.Fatalf("规则数被改变: %d", len(rules))
	}
	seen := map[string]bool{}
	for _, item := range rules {
		rule, _ := item.(map[string]interface{})
		tag, _ := rule["ruleTag"].(string)
		if tag == "" {
			t.Errorf("仍有规则缺 ruleTag: %v", rule)
			continue
		}
		if tag == "keep-me" {
			continue
		}
		if !isGeneratedRuleTag(tag) {
			t.Errorf("生成的名字格式不对: %q", tag)
		}
		if seen[tag] {
			t.Errorf("名字重复: %q", tag)
		}
		seen[tag] = true
	}

	again, err := svc.BackfillXrayRuleTags(ctx, 1)
	if err != nil {
		t.Fatalf("backfill again: %v", err)
	}
	if again != 0 {
		t.Errorf("幂等：第二次应为 0，got %d", again)
	}
}
