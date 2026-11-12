// Package providercfg 定义套餐级订阅来源（proxy-providers）配置。
//
// 设计见 /opt/corefusion-private/design/SUBSCRIPTION_PROVIDERS_DESIGN.md §5.1。
// 该配置只属于套餐（plan_templates.subscription_provider_config），
// 绝不能写入共享 mihomo_configs.config_json —— 一份共享 YAML 会被多个套餐复用。
package providercfg

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// ReservedAllNodesKey 是保留 provider key：表示“本套餐全部节点”。
// 允许被 proxy-groups 的 use 引用，但不允许在配置里定义。
const ReservedAllNodesKey = "subscription-nodes"

// Config 是 subscription_provider_config 的 JSON 结构。
//
// internal_providers 为空时，订阅生成阶段会按套餐的每个节点组自动生成；
// external_providers 原样透传给 mihomo（未知字段保留）；
// group_use 是共享 YAML 组名 → provider key 列表的映射。
type Config struct {
	InternalProviders map[string]InternalProvider       `json:"internal_providers"`
	ExternalProviders map[string]map[string]interface{} `json:"external_providers"`
	GroupUse          map[string][]string               `json:"group_use"`
	// InternalDefaults 是自动生成的内部 provider（按节点组）的默认参数：
	// interval / path（支持 {key} 占位）/ health-check / filter / override 等。
	// 各节点组仍可在 internal_providers 里单独覆盖。
	InternalDefaults InternalProvider `json:"internal_defaults"`
}

// InternalProvider 是由面板节点组生成的 provider。
// node_group_id 是面板内部字段，不会输出给客户端。
type InternalProvider struct {
	NodeGroupID      int64                  `json:"node_group_id"`
	Interval         int                    `json:"interval"`
	Path             string                 `json:"path"`
	Filter           string                 `json:"filter"`
	ExcludeFilter    string                 `json:"exclude-filter"`
	HealthCheck      map[string]interface{} `json:"health-check"`
	AdditionalPrefix string                 `json:"additional-prefix"`
	AdditionalSuffix string                 `json:"additional-suffix"`
	Override         map[string]interface{} `json:"override"`
}

// Parse 解析套餐配置；空字符串返回零值配置。
func Parse(raw string) (Config, error) {
	var cfg Config
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse subscription_provider_config: %w", err)
	}
	if cfg.InternalProviders == nil {
		cfg.InternalProviders = map[string]InternalProvider{}
	}
	if cfg.ExternalProviders == nil {
		cfg.ExternalProviders = map[string]map[string]interface{}{}
	}
	if cfg.GroupUse == nil {
		cfg.GroupUse = map[string][]string{}
	}
	return cfg, nil
}

// Validate 校验套餐配置的静态合法性（不查库，节点组归属由保存接口另行校验）。
func Validate(raw string) error {
	cfg, err := Parse(raw)
	if err != nil {
		return err
	}
	for key, p := range cfg.InternalProviders {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("internal provider key is empty")
		}
		if key == ReservedAllNodesKey {
			return fmt.Errorf("internal provider key %q is reserved", key)
		}
		if p.NodeGroupID <= 0 {
			return fmt.Errorf("internal provider %q: node_group_id is required", key)
		}
	}
	for key, p := range cfg.ExternalProviders {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("external provider key is empty")
		}
		if key == ReservedAllNodesKey {
			return fmt.Errorf("external provider key %q is reserved", key)
		}
		typ, _ := p["type"].(string)
		typ = strings.TrimSpace(strings.ToLower(typ))
		if typ == "" {
			typ = "http"
		}
		if typ == "http" || typ == "file" {
			url, _ := p["url"].(string)
			if typ == "http" && strings.TrimSpace(url) == "" {
				return fmt.Errorf("external provider %q: url is required for http type", key)
			}
		}
	}
	for group, keys := range cfg.GroupUse {
		if strings.TrimSpace(group) == "" {
			return fmt.Errorf("group_use contains an empty group name")
		}
		for _, key := range keys {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("group_use[%s] contains an empty provider key", group)
			}
		}
	}
	return nil
}

// SanitizeKey 把节点组名清洗成可用的 provider key：
// 去掉首尾空白，空白/斜杠/冒号等 YAML/URL 敏感字符换成 '-'，保留中文与常规符号。
func SanitizeKey(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || strings.ContainsRune(`/\:*?"<>|,;`, r):
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		default:
			// emoji 等符号保留：mihomo 的 provider key / use 都是字符串
			if unicode.IsPrint(r) {
				b.WriteRune(r)
				lastDash = false
			}
		}
	}
	return strings.Trim(b.String(), "-.")
}
