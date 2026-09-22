package cert

import "testing"

// 用户脚本里的两个参数必须能被认出来 ✓ —— 这正是"填了却说缺凭据"的根因 ✗。
func TestNormalizeDNSEnvVarsCloudflareScriptNames(t *testing.T) {
	got := normalizeDNSEnvVars(DNSProviderCloudflare, map[string]string{
		"CF_Token":      "tok-123",
		"CF_Account_ID": "acc-456",
	})
	if got["CF_DNS_API_TOKEN"] != "tok-123" {
		t.Fatalf("CF_Token 应归一为 CF_DNS_API_TOKEN ✓，实际 %v", got)
	}
	if got["CF_ACCOUNT_ID"] != "acc-456" {
		t.Fatalf("CF_Account_ID 应保留为 CF_ACCOUNT_ID ✓，实际 %v", got)
	}
}

// 大小写/连字符等写法都要能命中 ✓
func TestNormalizeDNSEnvVarsCaseAndSeparators(t *testing.T) {
	for _, key := range []string{"CF_Token", "CF_TOKEN", "cf-token", "cf token", "CFToken"} {
		got := normalizeDNSEnvVars(DNSProviderCloudflare, map[string]string{key: "v"})
		if got["CF_DNS_API_TOKEN"] != "v" {
			t.Fatalf("%q 应命中 CF_DNS_API_TOKEN ✓，实际 %v", key, got)
		}
	}
}

// **标准名优先** ✓：同时给了标准和别名时，不能被别名覆盖 ✗
func TestNormalizeDNSEnvVarsCanonicalWins(t *testing.T) {
	got := normalizeDNSEnvVars(DNSProviderCloudflare, map[string]string{
		"CF_DNS_API_TOKEN": "canonical",
		"CF_Token":         "alias",
	})
	if got["CF_DNS_API_TOKEN"] != "canonical" {
		t.Fatalf("标准名应优先 ✓，实际 %v", got)
	}
}

// 老式全局 Key ✓ 与不认识的键（原样保留 ✓）
func TestNormalizeDNSEnvVarsLegacyAndUnknownKeys(t *testing.T) {
	got := normalizeDNSEnvVars(DNSProviderCloudflare, map[string]string{
		"CLOUDFLARE_EMAIL":   "me@example.com",
		"CLOUDFLARE_API_KEY": "globalkey",
		"CF_TTL":             "120",
	})
	if got["CF_API_EMAIL"] != "me@example.com" || got["CF_API_KEY"] != "globalkey" {
		t.Fatalf("老式全局 Key 应归一 ✓，实际 %v", got)
	}
	if got["CF_TTL"] != "120" {
		t.Fatalf("不认识的键应原样保留 ✓，实际 %v", got)
	}
}

// 其它提供者不受影响 ✓（键原样透传 ✓）
func TestNormalizeDNSEnvVarsOtherProvidersUntouched(t *testing.T) {
	in := map[string]string{"ALICLOUD_ACCESS_KEY": "ak", "ALICLOUD_SECRET_KEY": "sk"}
	got := normalizeDNSEnvVars(DNSProviderAliDNS, in)
	if got["ALICLOUD_ACCESS_KEY"] != "ak" || got["ALICLOUD_SECRET_KEY"] != "sk" {
		t.Fatalf("阿里云应原样透传 ✓，实际 %v", got)
	}
}
