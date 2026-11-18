package coreconfig

import (
	"context"
	"encoding/json"
	"fmt"
)

// 默认路由基线（2026-11-18）：
//   - 内网/Google 连通性探测 → direct
//   - 广告域名 → blocked/block
//
// 三条规则全部用核心**内建**能力（xray 的 geoip:private / geosite:category-ads-all
// 来自随核心分发的 geoip.dat / geosite.dat；sing-box 的 ip_is_private 内建），
// 不引用 geoip-private.srs 这类需要额外下载的本地文件，避免规则库被删/没下载时
// 核心起不来。
//
// 这些规则只在「新建节点播种」和「存量节点一次性补全」时写入 core_configs，
// 之后完全由用户在路由页编辑/删除，同步时不会再注入最终配置。

const (
	baselineTagPrivate = "preset-private-direct"
	baselineTagGoogle  = "preset-google-direct"
	baselineTagAds     = "preset-ads-block"

	// sing-box 广告规则引用的本地 rule_set tag / 文件。
	singboxAdsRuleSetTag  = "geosite-category-ads-all"
	singboxAdsRuleSetFile = "rules/geosite-category-ads-all.srs"
)

func baselineXrayRules() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"type":        "field",
			"ruleTag":     baselineTagPrivate,
			"ip":          []interface{}{"geoip:private"},
			"outboundTag": "direct",
		},
		map[string]interface{}{
			"type":        "field",
			"ruleTag":     baselineTagGoogle,
			"domain":      []interface{}{"domain:gstatic.com", "domain:googleapis.com"},
			"outboundTag": "direct",
		},
		map[string]interface{}{
			"type":        "field",
			"ruleTag":     baselineTagAds,
			"domain":      []interface{}{"geosite:category-ads-all"},
			"outboundTag": "blocked",
		},
	}
}

func baselineSingboxRules() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"ip_is_private": true,
			"outbound":      "direct",
		},
		map[string]interface{}{
			"domain_suffix": []interface{}{"gstatic.com", "googleapis.com"},
			"outbound":      "direct",
		},
		map[string]interface{}{
			"rule_set": []interface{}{singboxAdsRuleSetTag},
			"outbound": "block",
		},
	}
}

// EnsureBaselineRouting 把缺失的默认规则补进节点配置段（写 core_configs）。
// 幂等且只补缺：用户已经改过/删掉的规则不会被恢复，已有等价规则不会重复加。
func (s *Service) EnsureBaselineRouting(ctx context.Context, serverID int64, coreType string) (int, error) {
	switch coreType {
	case "xray":
		return s.ensureBaselineXray(ctx, serverID)
	case "singbox":
		return s.ensureBaselineSingbox(ctx, serverID)
	default:
		return 0, fmt.Errorf("unsupported core type: %s", coreType)
	}
}

func (s *Service) ensureBaselineXray(ctx context.Context, serverID int64) (int, error) {
	raw, err := s.GetSection(ctx, serverID, "xray", "routing")
	if err != nil {
		return 0, err
	}
	routing, ok := raw.(map[string]interface{})
	if !ok || routing == nil {
		routing = map[string]interface{}{}
	}
	rules, _ := routing["rules"].([]interface{})

	added := 0
	for _, item := range baselineXrayRules() {
		rule := item.(map[string]interface{})
		tag, _ := rule["ruleTag"].(string)
		if ruleListContainsTag(rules, tag) {
			continue
		}
		// 老数据可能没有 ruleTag，用首个匹配值判重。
		if field, value, ok := firstRuleValue(rule); ok && ruleListContains(rules, field, value) {
			continue
		}
		rules = append(rules, rule)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	if _, ok := routing["domainStrategy"]; !ok {
		routing["domainStrategy"] = "IPIfNonMatch"
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

func (s *Service) ensureBaselineSingbox(ctx context.Context, serverID int64) (int, error) {
	raw, err := s.GetSection(ctx, serverID, "singbox", "route")
	if err != nil {
		return 0, err
	}
	route, ok := raw.(map[string]interface{})
	if !ok || route == nil {
		route = map[string]interface{}{}
	}
	rules, _ := route["rules"].([]interface{})
	ruleSets, _ := route["rule_set"].([]interface{})

	added := 0
	for _, item := range baselineSingboxRules() {
		rule := item.(map[string]interface{})
		if singboxBaselineRuleExists(rules, rule) {
			continue
		}
		rules = append(rules, rule)
		added++
	}

	// 广告规则引用本地 rule_set，定义必须存在（面板启动时会下载并推送该 srs）。
	if ruleListContains(rules, "rule_set", singboxAdsRuleSetTag) && !ruleSetTagExists(ruleSets, singboxAdsRuleSetTag) {
		ruleSets = append(ruleSets, map[string]interface{}{
			"tag":    singboxAdsRuleSetTag,
			"type":   "local",
			"format": "binary",
			"path":   singboxAdsRuleSetFile,
		})
		added++
	}

	if added == 0 {
		return 0, nil
	}
	route["rules"] = rules
	if len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
	}
	if _, ok := route["final"]; !ok {
		route["final"] = "direct"
	}
	data, err := json.Marshal(route)
	if err != nil {
		return 0, err
	}
	if err := s.SaveSection(ctx, serverID, "singbox", "route", data); err != nil {
		return 0, err
	}
	return added, nil
}

func ruleListContainsTag(rules []interface{}, tag string) bool {
	if tag == "" {
		return false
	}
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if v, _ := rule["ruleTag"].(string); v == tag {
			return true
		}
	}
	return false
}

func firstRuleValue(rule map[string]interface{}) (field, value string, ok bool) {
	for _, key := range []string{"ip", "domain", "rule_set", "domain_suffix"} {
		vals, isList := rule[key].([]interface{})
		if !isList || len(vals) == 0 {
			continue
		}
		if s, isStr := vals[0].(string); isStr && s != "" {
			return key, s, true
		}
	}
	return "", "", false
}

func singboxBaselineRuleExists(rules []interface{}, target map[string]interface{}) bool {
	field, value, ok := firstRuleValue(target)
	if !ok {
		// ip_is_private 没有列表字段，单独判断。
		if target["ip_is_private"] == true {
			for _, item := range rules {
				rule, _ := item.(map[string]interface{})
				if rule != nil && rule["ip_is_private"] == true {
					return true
				}
			}
		}
		return false
	}
	return ruleListContains(rules, field, value)
}

func ruleSetTagExists(sets []interface{}, tag string) bool {
	for _, item := range sets {
		set, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if v, _ := set["tag"].(string); v == tag {
			return true
		}
	}
	return false
}
