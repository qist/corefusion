package cert

import (
	"strings"
	"testing"
)

// lego v5 删除了 DNSPod 自己的 API 实现 ✗ —— 老通道必须给出**明确的迁移提示** ✓，
// 不能静默换成别的 provider ✗（用户会以为"填的凭据坏了"）。
func TestDNSPodProviderReturnsMigrationHint(t *testing.T) {
	c := NewACMEClient(t.TempDir(), "ops@example.com", CAProviderLetsEncrypt)

	_, err := c.createDNSProvider(&DNSProviderConfig{Provider: DNSProviderDNSPod})
	if err == nil {
		t.Fatal("dnspod 通道应当返回迁移错误，而不是静默成功")
	}
	if !strings.Contains(err.Error(), string(DNSProviderTencentCloud)) {
		t.Fatalf("错误信息必须指向 tencentcloud，实际: %v", err)
	}
}

// tencentcloud 必须是独立 provider（不能与 dnspod 混用）✓
func TestTencentCloudProviderIsDistinct(t *testing.T) {
	if DNSProviderTencentCloud == DNSProviderDNSPod {
		t.Fatal("tencentcloud 与 dnspod 必须是不同的 provider id")
	}
}
