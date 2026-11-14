package coreconfig

import (
	"strings"

	"corefusion/internal/domain"
)

// 背景：xray 在 2026-07（docs commit "Transport: Rename network to method"）
// 把传输层字段从 `network` 改名为 `method`，新枚举使用全称（websocket 等），
// 旧版本只认 `network`（ws 等）。
//
// 兼容策略：两者**双写**。
//   - 新核心：使用 `method`（`network` 仍是兼容别名）
//   - 旧核心：忽略未知的 `method`，继续用 `network`
//
// 实测（xray 26.6）：network ws + method websocket 同存时配置通过 `-test`，
// 因此双写不会让旧核心因未知字段/异名报错。
// 存储层（inbound/outbound config_json、前端表单）继续用短名，只在最终生成的
// 运行配置里归一：network 用短名（ws），method 用新名（websocket）。
func ApplyTransportMethodAlias(fullConfig map[string]interface{}) {
	if fullConfig == nil {
		return
	}
	for _, section := range []string{"inbounds", "outbounds"} {
		items, ok := fullConfig[section].([]interface{})
		if !ok {
			continue
		}
		for _, raw := range items {
			item, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			ss, ok := item["streamSettings"].(map[string]interface{})
			if !ok {
				continue
			}
			canonical := domain.StreamTransport(ss)
			if canonical == "" {
				continue
			}
			// 旧核心读 network（保持原值/短名，不被改写）；新核心读 method（新枚举名）。
			// 只有新格式只写了 method 时才反补 network，保证旧核心也能启动。
			if _, exists := ss["method"]; !exists {
				ss["method"] = domain.TransportMethod(canonical)
			}
			if v, _ := ss["network"].(string); strings.TrimSpace(v) == "" {
				ss["network"] = canonical
			}
		}
	}
}
