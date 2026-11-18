package coreconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// DropMissingSingboxLocalRuleSets 摘掉「指向不存在本地文件」的 sing-box rule_set
// 定义，以及引用它们的路由规则。
//
// 背景：sing-box 对 `type: local` 的 rule_set 在启动时会直接打开文件，缺失就
// FATAL（`open rules/geosite-category-ads-all.srs: no such file or directory`）。
// 面板的规则库文件是异步下载/推送的，节点首次连接时可能还没到位；用户也可能
// 在规则库页删掉文件。这里做一次存在性守卫，宁可暂时少一条分流规则，也不让
// 整个核心起不来。文件补齐后的下一次配置同步会自动恢复（规则仍在 DB 里）。
//
// 返回被摘掉的 rule_set 数量。
func DropMissingSingboxLocalRuleSets(fullConfig map[string]interface{}, rulesDir string) int {
	if fullConfig == nil || rulesDir == "" {
		return 0
	}
	route, ok := fullConfig["route"].(map[string]interface{})
	if !ok || route == nil {
		return 0
	}
	sets, _ := route["rule_set"].([]interface{})
	if len(sets) == 0 {
		return 0
	}

	missing := make(map[string]bool)
	kept := make([]interface{}, 0, len(sets))
	for _, item := range sets {
		set, ok := item.(map[string]interface{})
		if !ok {
			kept = append(kept, item)
			continue
		}
		typ, _ := set["type"].(string)
		path, _ := set["path"].(string)
		if !strings.EqualFold(typ, "local") || strings.TrimSpace(path) == "" {
			kept = append(kept, item)
			continue
		}
		if localRuleFileExists(rulesDir, path) {
			kept = append(kept, item)
			continue
		}
		for _, tag := range ruleSetTags(set["tag"]) {
			missing[tag] = true
		}
	}
	if len(missing) == 0 {
		return 0
	}
	route["rule_set"] = kept

	// 引用缺失 tag 的规则：先剔除 tag，列表变空则整条规则丢弃。
	rules, _ := route["rules"].([]interface{})
	if len(rules) > 0 {
		newRules := make([]interface{}, 0, len(rules))
		for _, item := range rules {
			rule, ok := item.(map[string]interface{})
			if !ok {
				newRules = append(newRules, item)
				continue
			}
			raw, has := rule["rule_set"]
			if !has {
				newRules = append(newRules, item)
				continue
			}
			tags := ruleSetTags(raw)
			if len(tags) == 0 {
				newRules = append(newRules, item)
				continue
			}
			remaining := make([]interface{}, 0, len(tags))
			for _, tag := range tags {
				if !missing[tag] {
					remaining = append(remaining, tag)
				}
			}
			if len(remaining) == 0 {
				continue // 整条规则只依赖缺失的 rule_set
			}
			rule["rule_set"] = remaining
			newRules = append(newRules, rule)
		}
		route["rules"] = newRules
	}
	return len(missing)
}

// localRuleFileExists 把 rule_set 的 path 映射到面板本地 rules 目录并检查文件。
// 节点侧 path 形如 `rules/xxx.srs`，面板本地是 `<dataDir>/rules/xxx.srs`；
// 只取 basename，避免 `../` 越界。
func localRuleFileExists(rulesDir, path string) bool {
	name := filepath.Base(strings.TrimSpace(path))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return false
	}
	fi, err := os.Stat(filepath.Join(rulesDir, name))
	if err != nil {
		return false
	}
	return !fi.IsDir() && fi.Size() > 0
}

func ruleSetTags(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return []string{s}
		}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}
