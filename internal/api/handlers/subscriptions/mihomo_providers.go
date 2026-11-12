package subscriptions

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/model/entity"
	"corefusion/internal/subscription/providercfg"
)

const (
	// subscriptionProvidersParam 控制主配置是否进入 Provider 模式：
	// 缺省 / 其它值 = 原始模式（节点内联在顶层 proxies）；
	// 1 / true = Provider 模式（proxy-providers + 组内 use）。
	subscriptionProvidersParam = "providers"
	// subscriptionProviderParam 只出现在内部生成的 provider URL 中，
	// 客户端不需要也不能手工指定。
	subscriptionProviderParam = "provider"
)

// internalProviderDef 是解析后的内部 provider（节点组来源）。
type internalProviderDef struct {
	Key           string
	NodeGroupID   int64
	Interval      int
	Path          string
	Filter        string
	ExcludeFilter string
	HealthCheck   map[string]interface{}
	Override      map[string]interface{}
	Prefix        string
	Suffix        string
}

// subscriptionProviderPlan 是套餐在当前订阅上下文里的 provider 解析结果。
type subscriptionProviderPlan struct {
	Internal map[string]*internalProviderDef
	External map[string]map[string]interface{}
	GroupUse map[string][]string
	// Defaults 是自动生成的内部 provider 的默认参数（interval/path/health-check 等）。
	Defaults internalProviderDef
	// AllNodes 是保留 provider `subscription-nodes` 的覆盖参数（nil = 全用 Defaults）。
	AllNodes *internalProviderDef
	// Keys 是具体的 provider key（内部 + 外部，按字典序），不含保留的 subscription-nodes。
	Keys []string
	// NodeNames：provider key → 该来源下的节点名（原始模式展开用）。
	// subscription-nodes 也放在这里，表示本套餐全部节点。
	NodeNames map[string][]string
	// MemberGroup：NodeGroupMember ID → 节点组 ID（数据源按 provider 过滤用）。
	MemberGroup map[int64]int64
}

// mihomoProviderDefinition 是准备输出到客户端的 proxy-provider。
type mihomoProviderDefinition struct {
	Key    string
	Fields map[string]interface{}
}

type mihomoGroupPlan struct {
	Use     []string // Provider 模式：组 use 引用的 provider key
	Proxies []string // 原始模式：服务端展开后的节点名
}

type mihomoRenderOptions struct {
	ProviderMode         bool
	ProviderDefinitions  []mihomoProviderDefinition
	ProviderNodeNames    map[string][]string
	GroupUse             map[string][]string
	SpecificProviderKeys []string
}

func providersEnabled(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func renderedNodeName(item renderedSubscriptionNode) string {
	if item.Node != nil {
		return item.Node.Name
	}
	return ""
}

// pruneProviderNodeNames 只保留真正出现在顶层 proxies 里的节点名。
// 原始模式把 provider 来源展开成组内 proxies 引用，若引用了被格式转换丢弃的节点，
// mihomo 会因 "proxy not found" 拒载整份配置。
func pruneProviderNodeNames(names map[string][]string, proxyMaps []map[string]interface{}) {
	available := make(map[string]bool, len(proxyMaps))
	for _, proxyMap := range proxyMaps {
		if name, _ := proxyMap["name"].(string); name != "" {
			available[name] = true
		}
	}
	for key, list := range names {
		kept := make([]string, 0, len(list))
		for _, name := range list {
			if available[name] {
				kept = append(kept, name)
			}
		}
		names[key] = kept
	}
}

// resolveSubscriptionProviderPlan 把套餐配置、节点组、当前选中的节点收敛成生成计划。
func (h *PublicHandler) resolveSubscriptionProviderPlan(ctx context.Context, tmpl *entity.PlanTemplate, members []entity.NodeGroupMember, rendered []renderedSubscriptionNode) *subscriptionProviderPlan {
	plan := &subscriptionProviderPlan{
		Internal:    map[string]*internalProviderDef{},
		External:    map[string]map[string]interface{}{},
		GroupUse:    map[string][]string{},
		NodeNames:   map[string][]string{},
		MemberGroup: map[int64]int64{},
	}
	for _, m := range members {
		plan.MemberGroup[m.ID] = m.GroupID
	}

	var cfg providercfg.Config
	if tmpl != nil {
		parsed, err := providercfg.Parse(tmpl.SubscriptionProviderConfig)
		if err != nil {
			log.Printf("[subscription] parse provider config failed: tmpl=%d err=%v", tmpl.ID, err)
		} else {
			cfg = parsed
		}
	}
	plan.GroupUse = cfg.GroupUse
	plan.Defaults = *internalDefFromConfig("", 0, cfg.InternalDefaults)
	if internalProviderConfigured(cfg.AllNodesProvider) {
		d := internalDefFromConfig(providercfg.ReservedAllNodesKey, 0, cfg.AllNodesProvider)
		plan.AllNodes = d
	}

	groupNames := map[int64]string{}
	if tmpl != nil && h.planSvc != nil {
		if groups, err := h.planSvc.ListNodeGroups(ctx); err == nil {
			for _, g := range groups {
				groupNames[g.ID] = g.Name
			}
		}
	}

	validGroups := map[int64]bool{}
	if tmpl != nil {
		for _, gid := range tmpl.NodeGroupIDs {
			validGroups[gid] = true
		}
	}

	if tmpl != nil && len(cfg.InternalProviders) == 0 {
		// 默认：为套餐的每个节点组自动生成一个内部 provider，key 用组名。
		for _, gid := range tmpl.NodeGroupIDs {
			key := providercfg.SanitizeKey(groupNames[gid])
			if key == "" {
				key = fmt.Sprintf("group-%d", gid)
			}
			key = uniqueProviderKey(key, plan.Internal, nil)
			plan.Internal[key] = applyProviderKeyTemplate(mergeProviderDefs(&plan.Defaults, &internalProviderDef{Key: key, NodeGroupID: gid}), key)
		}
	} else {
		keys := make([]string, 0, len(cfg.InternalProviders))
		for key := range cfg.InternalProviders {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, rawKey := range keys {
			def := cfg.InternalProviders[rawKey]
			key := providercfg.SanitizeKey(rawKey)
			if key == "" || key == providercfg.ReservedAllNodesKey {
				log.Printf("[subscription] skip invalid internal provider key: %q", rawKey)
				continue
			}
			if !validGroups[def.NodeGroupID] {
				log.Printf("[subscription] internal provider %q: node_group_id=%d not in plan template, skip", key, def.NodeGroupID)
				continue
			}
			key = uniqueProviderKey(key, plan.Internal, nil)
			explicit := internalDefFromConfig(key, def.NodeGroupID, def)
			plan.Internal[key] = applyProviderKeyTemplate(mergeProviderDefs(&plan.Defaults, explicit), key)
		}
	}

	externalKeys := make([]string, 0, len(cfg.ExternalProviders))
	for key := range cfg.ExternalProviders {
		externalKeys = append(externalKeys, key)
	}
	sort.Strings(externalKeys)
	for _, rawKey := range externalKeys {
		key := providercfg.SanitizeKey(rawKey)
		if key == "" || key == providercfg.ReservedAllNodesKey {
			log.Printf("[subscription] skip invalid external provider key: %q", rawKey)
			continue
		}
		if _, exists := plan.Internal[key]; exists {
			log.Printf("[subscription] external provider %q shadowed by internal provider, skip", key)
			continue
		}
		if _, exists := plan.External[key]; exists {
			continue
		}
		fields := map[string]interface{}{}
		for k, v := range cfg.ExternalProviders[rawKey] {
			fields[k] = v
		}
		plan.External[key] = fields
	}

	for key := range plan.Internal {
		plan.Keys = append(plan.Keys, key)
	}
	for key := range plan.External {
		plan.Keys = append(plan.Keys, key)
	}
	sort.Strings(plan.Keys)

	// 每个内部 provider 的节点名（原始模式展开 / 数据源过滤）。
	for key, def := range plan.Internal {
		names := make([]string, 0)
		seen := map[string]bool{}
		for _, item := range rendered {
			if plan.MemberGroup[item.MemberID] != def.NodeGroupID {
				continue
			}
			name := renderedNodeName(item)
			if name == "" || seen[name] {
				continue
			}
			if !providerNameMatches(name, def.Filter, def.ExcludeFilter) {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
		plan.NodeNames[key] = names
	}
	allNames := make([]string, 0, len(rendered))
	seenAll := map[string]bool{}
	for _, item := range rendered {
		name := renderedNodeName(item)
		if name == "" || seenAll[name] {
			continue
		}
		seenAll[name] = true
		allNames = append(allNames, name)
	}
	plan.NodeNames[providercfg.ReservedAllNodesKey] = allNames

	return plan
}

// buildProviderDefinitions 生成 Provider 模式的 proxy-providers 列表。
// 内部 provider 的 URL 指向面板数据源（继承 config/platform，追加 provider=key）；
// 外部 provider 原样透传，不经过面板。
func (h *PublicHandler) buildProviderDefinitions(c *gin.Context, token string, plan *subscriptionProviderPlan) []mihomoProviderDefinition {
	defs := make([]mihomoProviderDefinition, 0, len(plan.Keys)+1)
	for _, key := range plan.Keys {
		if def, ok := plan.Internal[key]; ok {
			url := h.buildProviderURL(c, token, key)
			defs = append(defs, mihomoProviderDefinition{
				Key:    key,
				Fields: buildInternalProviderFields(key, def, url),
			})
			continue
		}
		if fields, ok := plan.External[key]; ok {
			defs = append(defs, mihomoProviderDefinition{
				Key:    key,
				Fields: buildExternalProviderFields(key, fields),
			})
		}
	}
	// 保留 provider：本套餐全部节点（无 provider 参数，回落全部节点）。
	// 参数优先 all_nodes_provider，其次 internal_defaults；都为空时用内置默认。
	allDef := applyProviderKeyTemplate(mergeProviderDefs(&plan.Defaults, plan.AllNodes), providercfg.ReservedAllNodesKey)
	defs = append(defs, mihomoProviderDefinition{
		Key:    providercfg.ReservedAllNodesKey,
		Fields: buildInternalProviderFields(providercfg.ReservedAllNodesKey, allDef, h.buildProviderURL(c, token, providercfg.ReservedAllNodesKey)),
	})
	sort.Slice(defs, func(i, j int) bool { return defs[i].Key < defs[j].Key })
	return defs
}

func uniqueProviderKey(base string, internal map[string]*internalProviderDef, external map[string]map[string]interface{}) string {
	key := base
	for i := 2; ; i++ {
		_, inInternal := internal[key]
		_, inExternal := external[key]
		if !inInternal && !inExternal && key != providercfg.ReservedAllNodesKey {
			break
		}
		key = fmt.Sprintf("%s-%d", base, i)
	}
	return key
}

// providerNameMatches 应用 provider 级 filter / exclude-filter（先 filter 再 exclude，与 mihomo 一致）。
func providerNameMatches(name, filter, excludeFilter string) bool {
	if filter != "" {
		re, err := regexp.Compile(filter)
		if err != nil {
			log.Printf("[subscription] invalid provider filter %q: %v", filter, err)
		} else if !re.MatchString(name) {
			return false
		}
	}
	if excludeFilter != "" {
		re, err := regexp.Compile(excludeFilter)
		if err != nil {
			log.Printf("[subscription] invalid provider exclude-filter %q: %v", excludeFilter, err)
		} else if re.MatchString(name) {
			return false
		}
	}
	return true
}

// filterProxyNames 应用组级 filter / exclude-filter（原始模式服务端过滤）。
func filterProxyNames(names []string, filter, excludeFilter string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if providerNameMatches(name, filter, excludeFilter) {
			out = append(out, name)
		}
	}
	return out
}

// buildInternalProviderFields 生成内部 provider 对客户端输出的字段。
// node_group_id 是面板内部字段，必须剥掉；url 由调用方按当前请求参数生成。
// internalDefFromConfig 把套餐配置里的 provider 参数转成内部定义。
// path 支持 {key} 占位（如 ./providers/{key}.yaml），多 provider 时不会互相覆盖。
func internalDefFromConfig(key string, nodeGroupID int64, p providercfg.InternalProvider) *internalProviderDef {
	return &internalProviderDef{
		Key:           key,
		NodeGroupID:   nodeGroupID,
		Interval:      p.Interval,
		Path:          strings.TrimSpace(p.Path),
		Filter:        p.Filter,
		ExcludeFilter: p.ExcludeFilter,
		HealthCheck:   p.HealthCheck,
		Override:      p.Override,
		Prefix:        p.AdditionalPrefix,
		Suffix:        p.AdditionalSuffix,
	}
}

// applyProviderKeyTemplate 把 path 里的 {key} 占位替换成实际 provider key
// （默认模板 ./providers/{key}.yaml 用于所有自动生成的 provider）。
func applyProviderKeyTemplate(def *internalProviderDef, key string) *internalProviderDef {
	if def != nil && key != "" && strings.Contains(def.Path, "{key}") {
		def.Path = strings.ReplaceAll(def.Path, "{key}", key)
	}
	return def
}

// internalProviderConfigured 判断用户是否给 provider 配过任何参数。
func internalProviderConfigured(p providercfg.InternalProvider) bool {
	return p.Interval > 0 ||
		strings.TrimSpace(p.Path) != "" ||
		strings.TrimSpace(p.Filter) != "" ||
		strings.TrimSpace(p.ExcludeFilter) != "" ||
		len(p.HealthCheck) > 0 ||
		len(p.Override) > 0 ||
		strings.TrimSpace(p.AdditionalPrefix) != "" ||
		strings.TrimSpace(p.AdditionalSuffix) != ""
}

// mergeProviderDefs 以 base 为默认、over 里的非零字段覆盖 base。
func mergeProviderDefs(base, over *internalProviderDef) *internalProviderDef {
	if base == nil && over == nil {
		return &internalProviderDef{}
	}
	out := internalProviderDef{}
	if base != nil {
		out = *base
	}
	if over == nil {
		return &out
	}
	if over.Key != "" {
		out.Key = over.Key
	}
	if over.NodeGroupID != 0 {
		out.NodeGroupID = over.NodeGroupID
	}
	if over.Interval > 0 {
		out.Interval = over.Interval
	}
	if over.Path != "" {
		out.Path = over.Path
	}
	if over.Filter != "" {
		out.Filter = over.Filter
	}
	if over.ExcludeFilter != "" {
		out.ExcludeFilter = over.ExcludeFilter
	}
	if len(over.HealthCheck) > 0 {
		out.HealthCheck = over.HealthCheck
	}
	if len(over.Override) > 0 {
		out.Override = over.Override
	}
	if over.Prefix != "" {
		out.Prefix = over.Prefix
	}
	if over.Suffix != "" {
		out.Suffix = over.Suffix
	}
	return &out
}

func buildInternalProviderFields(key string, def *internalProviderDef, url string) map[string]interface{} {
	fields := map[string]interface{}{
		"type": "http",
		"url":  url,
	}
	interval := def.Interval
	if interval <= 0 {
		interval = 3600
	}
	fields["interval"] = interval
	path := strings.TrimSpace(def.Path)
	if path == "" {
		path = "./providers/" + key + ".yaml"
	}
	fields["path"] = path
	if def.Filter != "" {
		fields["filter"] = def.Filter
	}
	if def.ExcludeFilter != "" {
		fields["exclude-filter"] = def.ExcludeFilter
	}
	if len(def.HealthCheck) > 0 {
		fields["health-check"] = def.HealthCheck
	} else {
		fields["health-check"] = map[string]interface{}{
			"enable":   true,
			"url":      "https://www.gstatic.com/generate_204",
			"interval": 3600,
		}
	}
	override := map[string]interface{}{}
	for k, v := range def.Override {
		override[k] = v
	}
	if def.Prefix != "" {
		override["additional-prefix"] = def.Prefix
	}
	if def.Suffix != "" {
		override["additional-suffix"] = def.Suffix
	}
	if len(override) > 0 {
		fields["override"] = override
	}
	return fields
}

// buildSubscriptionNodesProviderFields 生成保留 provider subscription-nodes 的字段，
// 其数据源不带 provider 参数，返回本套餐全部节点。
func buildSubscriptionNodesProviderFields(url string) map[string]interface{} {
	return map[string]interface{}{
		"type":     "http",
		"url":      url,
		"interval": 3600,
		"path":     "./providers/subscription-nodes.yaml",
		"health-check": map[string]interface{}{
			"enable":   true,
			"url":      "https://www.gstatic.com/generate_204",
			"interval": 3600,
		},
	}
}

func buildExternalProviderFields(key string, fields map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range fields {
		out[k] = v
	}
	if typ, _ := out["type"].(string); strings.TrimSpace(typ) == "" {
		out["type"] = "http"
	}
	if p, _ := out["path"].(string); strings.TrimSpace(p) == "" {
		out["path"] = "./providers/" + key + ".yaml"
	}
	return out
}

// resolveGroupSources 计算某个代理组的节点来源 key。
//
// 优先级：套餐 group_use → 共享 YAML 的历史 use → _hasExternalNodes（全部来源）。
func resolveGroupSources(name string, hasExternal bool, legacyUse []string, opts *mihomoRenderOptions) []string {
	if opts == nil {
		return nil
	}
	if keys, ok := opts.GroupUse[name]; ok {
		return keys
	}
	// 历史共享 YAML 里残留的 use 只在 Provider 模式按名字解析；原始模式不使用（见设计 §6.2）。
	if opts.ProviderMode && len(legacyUse) > 0 {
		return legacyUse
	}
	if hasExternal {
		if len(opts.SpecificProviderKeys) > 0 {
			return opts.SpecificProviderKeys
		}
		return []string{providercfg.ReservedAllNodesKey}
	}
	return nil
}

// buildMihomoGroupPlans 预先计算每个组要注入的 provider / 节点名。
func buildMihomoGroupPlans(cfg map[string]interface{}, opts *mihomoRenderOptions) map[string]mihomoGroupPlan {
	plans := map[string]mihomoGroupPlan{}
	groups, ok := cfg["proxy-groups"].([]interface{})
	if !ok {
		return plans
	}
	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		name := getString(group, "name", "")
		if name == "" {
			continue
		}
		hasExternal, _ := group["_hasExternalNodes"].(bool)
		legacyUse := getStringSlice(group, "use")
		sources := resolveGroupSources(name, hasExternal, legacyUse, opts)
		if opts == nil || len(sources) == 0 {
			continue
		}
		seen := map[string]bool{}
		if opts.ProviderMode {
			use := make([]string, 0, len(sources))
			for _, rawKey := range sources {
				key := providercfg.SanitizeKey(rawKey)
				if key == "" || seen[key] {
					continue
				}
				if !providerDefinitionExists(opts.ProviderDefinitions, key) {
					log.Printf("[subscription] group %q references unknown provider %q, skip", name, rawKey)
					continue
				}
				seen[key] = true
				use = append(use, key)
			}
			plans[name] = mihomoGroupPlan{Use: use}
			continue
		}
		names := make([]string, 0)
		for _, rawKey := range sources {
			key := providercfg.SanitizeKey(rawKey)
			if key == "" {
				continue
			}
			for _, n := range opts.ProviderNodeNames[key] {
				if n == "" || seen[n] {
					continue
				}
				seen[n] = true
				names = append(names, n)
			}
		}
		// 组级 filter / exclude-filter 在原始模式下由服务端执行（客户端此时只有显式节点名）。
		names = filterProxyNames(names, getString(group, "filter", ""), getString(group, "exclude-filter", ""))
		plans[name] = mihomoGroupPlan{Proxies: names}
	}
	return plans
}

func providerDefinitionExists(defs []mihomoProviderDefinition, key string) bool {
	for _, d := range defs {
		if d.Key == key {
			return true
		}
	}
	return false
}

// filterRenderedNodesForProvider 按 provider key 过滤数据源节点。
// 未知 / 外部 / 不属于套餐的定义一律回落到全部节点（不返回 4xx）。
func filterRenderedNodesForProvider(rendered []renderedSubscriptionNode, plan *subscriptionProviderPlan, key string) []renderedSubscriptionNode {
	key = strings.TrimSpace(key)
	if plan == nil || key == "" || key == providercfg.ReservedAllNodesKey {
		return rendered
	}
	def, ok := plan.Internal[key]
	if !ok {
		if _, external := plan.External[key]; external {
			log.Printf("[subscription] provider %q is external, panel does not proxy it, fallback to all nodes", key)
		} else {
			log.Printf("[subscription] unknown provider %q, fallback to all nodes", key)
		}
		return rendered
	}
	out := make([]renderedSubscriptionNode, 0, len(rendered))
	for _, item := range rendered {
		if plan.MemberGroup[item.MemberID] != def.NodeGroupID {
			continue
		}
		if !providerNameMatches(renderedNodeName(item), def.Filter, def.ExcludeFilter) {
			continue
		}
		out = append(out, item)
	}
	return out
}
