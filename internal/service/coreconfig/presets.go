package coreconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// regionPreset 描述某个地区要导入的路由规则内容。
type regionPreset struct {
	// xray：域名/IP 分成两条规则（同一规则里的 domain 与 ip 是 AND 关系）。
	xrayDomains []string
	xrayIPs     []string
	// sing-box：rule_set tag → 本地 srs 文件。
	ruleSets []presetRuleSet
}

type presetRuleSet struct {
	Tag  string
	File string
}

var regionPresets = map[string]regionPreset{
	"cn": {
		xrayDomains: []string{"geosite:cn", "regexp:.*\\.cn$"},
		xrayIPs:     []string{"geoip:cn"},
		ruleSets: []presetRuleSet{
			{Tag: "geosite-cn", File: "geosite-cn.srs"},
			{Tag: "geoip-cn", File: "geoip-cn.srs"},
		},
	},
	"ir": {
		xrayDomains: []string{"ext:geosite_IR.dat:ir", "regexp:.*\\.ir$", "regexp:.*\\.xn--mgba3a4f16a$"},
		xrayIPs:     []string{"ext:geoip_IR.dat:ir"},
		ruleSets: []presetRuleSet{
			{Tag: "geosite-ir", File: "geosite-ir.srs"},
			{Tag: "geoip-ir", File: "geoip-ir.srs"},
		},
	},
	"ru": {
		xrayDomains: []string{
			"ext:geosite_RU.dat:ru-available-only-inside",
			"regexp:.*\\.ru$", "regexp:.*\\.su$", "regexp:.*\\.xn--p1ai$",
		},
		xrayIPs: []string{"geoip:ru"},
		ruleSets: []presetRuleSet{
			{Tag: "geosite-category-ru", File: "geosite-category-ru.srs"},
			{Tag: "geoip-ru", File: "geoip-ru.srs"},
		},
	},
}

// ImportRegionRoute 把某个地区的预设规则写入节点的路由段（幂等：已存在则不重复加）。
// outboundTag 可以是 direct、blocked，或刚创建的 WARP/MASQUE 出站 tag。
func (s *Service) ImportRegionRoute(ctx context.Context, serverID int64, coreType, region, outboundTag string) (int, error) {
	region = strings.ToLower(strings.TrimSpace(region))
	preset, ok := regionPresets[region]
	if !ok {
		return 0, fmt.Errorf("unsupported region: %s", region)
	}
	outboundTag = strings.TrimSpace(outboundTag)
	if outboundTag == "" {
		outboundTag = "direct"
	}

	switch coreType {
	case "xray":
		return s.importXrayRegionRoute(ctx, serverID, preset, outboundTag)
	case "singbox":
		return s.importSingboxRegionRoute(ctx, serverID, preset, outboundTag)
	default:
		return 0, fmt.Errorf("unsupported core type: %s", coreType)
	}
}

func (s *Service) importXrayRegionRoute(ctx context.Context, serverID int64, preset regionPreset, outboundTag string) (int, error) {
	raw, err := s.GetSection(ctx, serverID, "xray", "routing")
	if err != nil {
		return 0, err
	}
	routing, ok := raw.(map[string]interface{})
	if !ok {
		routing = map[string]interface{}{}
	}
	rules, _ := routing["rules"].([]interface{})

	added := 0
	// 域名规则
	if !ruleListContains(rules, "domain", preset.xrayDomains[0]) {
		rule := map[string]interface{}{
			"type":        "field",
			"ruleTag":     NewRuleTag(),
			"domain":      preset.xrayDomains,
			"outboundTag": outboundTag,
		}
		rules = append(rules, rule)
		added++
	}
	// IP 规则
	if len(preset.xrayIPs) > 0 && !ruleListContains(rules, "ip", preset.xrayIPs[0]) {
		rule := map[string]interface{}{
			"type":        "field",
			"ruleTag":     NewRuleTag(),
			"ip":          preset.xrayIPs,
			"outboundTag": outboundTag,
		}
		rules = append(rules, rule)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	routing["rules"] = rules
	data, err := json.Marshal(routing)
	if err != nil {
		return 0, err
	}
	if err := s.SaveSection(ctx, serverID, "xray", "routing", data); err != nil {
		return 0, err
	}
	return added, nil
}

func (s *Service) importSingboxRegionRoute(ctx context.Context, serverID int64, preset regionPreset, outboundTag string) (int, error) {
	raw, err := s.GetSection(ctx, serverID, "singbox", "route")
	if err != nil {
		return 0, err
	}
	route, ok := raw.(map[string]interface{})
	if !ok {
		route = map[string]interface{}{}
	}
	rules, _ := route["rules"].([]interface{})
	ruleSets, _ := route["rule_set"].([]interface{})

	added := 0

	// 先补齐 rule_set 定义（local + binary + rules/<file>.srs）
	haveTag := make(map[string]bool)
	for _, item := range ruleSets {
		if m, ok := item.(map[string]interface{}); ok {
			if tag, _ := m["tag"].(string); tag != "" {
				haveTag[tag] = true
			}
		}
	}
	for _, rs := range preset.ruleSets {
		if haveTag[rs.Tag] {
			continue
		}
		ruleSets = append(ruleSets, map[string]interface{}{
			"tag":    rs.Tag,
			"type":   "local",
			"format": "binary",
			"path":   "rules/" + rs.File,
		})
		haveTag[rs.Tag] = true
	}
	if len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
	}

	// 再补规则（按第一个 rule_set tag 判重）
	marker := preset.ruleSets[0].Tag
	if !ruleListContains(rules, "rule_set", marker) {
		tags := make([]string, 0, len(preset.ruleSets))
		for _, rs := range preset.ruleSets {
			tags = append(tags, rs.Tag)
		}
		rules = append(rules, map[string]interface{}{
			"rule_set": tags,
			"outbound": outboundTag,
		})
		added++
	}
	route["rules"] = rules

	data, err := json.Marshal(route)
	if err != nil {
		return 0, err
	}
	if err := s.SaveSection(ctx, serverID, "singbox", "route", data); err != nil {
		return 0, err
	}
	return added, nil
}

// ruleListContains 检查已有规则里是否已包含某个列表字段的指定取值。
func ruleListContains(rules []interface{}, field, value string) bool {
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		vals, ok := rule[field].([]interface{})
		if !ok {
			continue
		}
		for _, v := range vals {
			if s, ok := v.(string); ok && s == value {
				return true
			}
		}
	}
	return false
}
