package cert

import "strings"

// dnsEnvAlias 把"用户/脚本里常见的写法"归一到 **lego 认的标准环境变量名** ✓。
//
// 为什么需要 ✗：不同工具对同一份凭据的命名完全不一样 ——
//   - Cloudflare 官方文档/脚本用 `CF_Token` + `CF_Account_ID` ✓；
//   - 老一些的脚本用 `CLOUDFLARE_EMAIL` + `CLOUDFLARE_API_KEY` ✓；
//   - 而 lego 的 cloudflare 提供者只认 **`CF_DNS_API_TOKEN`**（或全局 `CF_API_EMAIL`+`CF_API_KEY`）✗。
//
// 用户是照着**自己的脚本**填的 ✓ → 如果不在这一步归一，就会"填了、看着没错、却报
// `cloudflare: some credentials information are missing`" ✗（很难自查 ✗）。
//
// 匹配方式：先做 slug（小写 ✓、去掉 `_`/`-`/空格 ✓）再查表 ✓ —— 于是
// `CF_Token` / `CF_TOKEN` / `cf-token` / `CFToken` 都能命中 ✓。
var dnsEnvAliases = map[DNSProvider]map[string]string{
	DNSProviderCloudflare: {
		// —— API Token（推荐 ✓）：脚本里常叫 CF_Token / CF_API_TOKEN ✓
		"cftoken":               "CF_DNS_API_TOKEN",
		"cfapitoken":            "CF_DNS_API_TOKEN",
		"cfdnsapitoken":         "CF_DNS_API_TOKEN",
		"cloudflareapitoken":    "CF_DNS_API_TOKEN",
		"cloudflarednsapitoken": "CF_DNS_API_TOKEN",
		// —— 账号 ID：DNS-01 用不到 ✗，但脚本里常一起给 ✓ → 保留下来（无害 ✓）
		"cfaccountid":         "CF_ACCOUNT_ID",
		"cloudflareaccountid": "CF_ACCOUNT_ID",
		// —— 全局 Key（老式 ✓）：CF_Email / CF_Key / CLOUDFLARE_API_KEY ✓
		"cfemail":          "CF_API_EMAIL",
		"cloudflareemail":  "CF_API_EMAIL",
		"cfkey":            "CF_API_KEY",
		"cfapikey":         "CF_API_KEY",
		"cloudflareapikey": "CF_API_KEY",
		// —— zone 级 token（可选 ✓）
		"cfzonetoken":         "CF_ZONE_API_TOKEN",
		"cfzoneapitoken":      "CF_ZONE_API_TOKEN",
		"cloudflarezonetoken": "CF_ZONE_API_TOKEN",
	},
}

// envSlug 归一化环境变量名：小写 + 去掉 `_` `-` 空格 ✓。
func envSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch r {
		case '_', '-', ' ', '.', '/':
			// 丢弃 ✓
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeDNSEnvVars 返回"喂给 lego 用"的环境变量集合 ✓。
//
// 规则：
//  1. **标准名优先** ✓：用户已经填了标准名（例如 `CF_DNS_API_TOKEN` ✓）就以它为准 ✓，
//     不会被别名覆盖 ✗；
//  2. 别名归一 ✓：`CF_Token` → `CF_DNS_API_TOKEN` ✓ 等；
//  3. 不认识的键**原样保留** ✓（各家提供者都有自己的变量 ✓，不能乱丢 ✗）。
func normalizeDNSEnvVars(provider DNSProvider, in map[string]string) map[string]string {
	aliases := dnsEnvAliases[provider]
	out := make(map[string]string, len(in))

	// Pass 1：先放**标准名** ✓（键的 slug 归一后正好等于标准名的 slug ✓ → 说明它本身就是标准名 ✓）。
	// 必须先做这一遍 ✗：map 遍历顺序随机 ✓，若先跑别名就可能把标准名挤掉 ✗。
	for k, v := range in {
		if v == "" {
			continue
		}
		slug := envSlug(k)
		if canonical, ok := aliases[slug]; ok && envSlug(canonical) == slug {
			out[canonical] = v
		}
	}

	// Pass 2：别名的值只在标准名**缺失**时补上 ✓；不认识的键原样保留 ✓（各家提供者变量各不相同 ✓）。
	for k, v := range in {
		if v == "" {
			continue
		}
		slug := envSlug(k)
		canonical, ok := aliases[slug]
		if !ok {
			if _, dup := out[k]; !dup {
				out[k] = v
			}
			continue
		}
		if _, exists := out[canonical]; exists {
			continue // 标准名优先 ✓
		}
		out[canonical] = v
	}
	return out
}
