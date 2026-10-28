package subscriptions

import "strings"

// normalizeSubscriptionFormat 把常见的客户端/历史写法归一成 formatter 的合法取值。
// 非法值原样返回（最终会走 base64 兜底），但常见别名不应再静默回落：
//   - Clash / Mihomo → mihomo_yaml（注意：**没有** "clash" 这个格式名）
//   - sing-box → singbox_json
//   - xray → xray_json
func normalizeSubscriptionFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "":
		return "base64_uri"
	case "base64", "base64_uri", "v2ray", "v2rayn", "uri":
		return "base64_uri"
	case "raw", "raw_uri":
		return "raw_uri"
	case "clash", "clash_yaml", "mihomo", "mihomo_yaml", "yaml":
		return "mihomo_yaml"
	case "mihomo_proxies", "mihomo_proxies_yaml", "proxies":
		return "mihomo_proxies_yaml"
	case "singbox", "sing-box", "singbox_json":
		return "singbox_json"
	case "xray", "xray_json":
		return "xray_json"
	}
	return strings.TrimSpace(format)
}
