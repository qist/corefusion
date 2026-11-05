package certificate

import "testing"

// lego v5 用 tencentcloud 取代了被上游移除的 dnspod ✗：
// 新建通道必须能选到它，且字段就是腾讯云的 SecretId / SecretKey ✓。
func TestSupportedDNSProvidersIncludesTencentCloud(t *testing.T) {
	providers := SupportedDNSProviders()

	fields, ok := providers["tencentcloud"]
	if !ok {
		t.Fatal("SupportedDNSProviders 必须包含 tencentcloud（lego v5 的 DNSPod 替代者）")
	}
	want := map[string]bool{"TENCENTCLOUD_SECRET_ID": false, "TENCENTCLOUD_SECRET_KEY": false}
	for _, f := range fields {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Fatalf("tencentcloud 缺少必填字段 %s（实际: %v）", k, fields)
		}
	}

	// 老通道仍要能被列出/编辑（迁移前不能直接"消失"）✓
	if _, ok := providers["dnspod"]; !ok {
		t.Fatal("dnspod 需保留在列表里，供既有通道编辑与迁移")
	}
}
