package coreconfig

// 背景：xray 在 2026-07（docs commit "Transport: Rename network to method"）
// 把传输层字段从 `network` 改名为 `method`，旧版本只认 `network`。
//
// 兼容策略：两者**同值双写**。
//   - 新核心：使用 `method`（`network` 仍是它的兼容别名）
//   - 旧核心：忽略未知的 `method`，继续用 `network`
//
// 实测（xray 26.9.30）：network+method 同值/不同值、以及存在未知字段，配置都能通过
// `xray -test`；因此双写不会让旧核心因未知字段报错。
// 存储层（inbound/outbound config_json、前端表单）继续用 `network`，只在最终生成的
// 运行配置里补别名，避免大范围改数据。

// ApplyTransportMethodAlias 为 xray 配置里所有带 streamSettings 的入站/出站
// 补上 `method`（与 `network` 同值）。已存在 `method` 的不覆盖。
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
			network, _ := ss["network"].(string)
			if network == "" {
				continue
			}
			if _, exists := ss["method"]; exists {
				continue
			}
			ss["method"] = network
		}
	}
}
