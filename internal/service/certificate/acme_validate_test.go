package certificate

import (
	"strings"
	"testing"
)

// 用户指正 + 核实结论：**公信 CA 可以给公网 IP 签证书** ✓（Let's Encrypt 2025-07 签出首张 IP 证书，
// 2026-01-15 IP 证书 GA，策略上必须是短时证书 ~6 天）；但**私网/保留地址永远不行** ✗
// （CA 必须能从公网连到该 IP 做验证）。这里把两类都钉住。
func TestValidateACMEDomains(t *testing.T) {
	// ① 公网 IP：必须放行 ✓
	ok := []string{"8.8.8.8", "23.254.233.185", "2606:4700:4700::1111"}
	for _, d := range ok {
		if err := ValidateACMEDomains([]string{d}, "http-01"); err != nil {
			t.Errorf("公网 IP %s 应允许（LE 自 2026-01-15 支持 IP 证书）：%v", d, err)
		}
	}

	// ② 私网/保留地址：必须拒绝，且错误里说明原因与可行方案 ✓
	bad := []string{
		"192.168.1.8",  // 用户那台节点的内网 IP
		"10.0.0.1",     // 私网
		"172.16.5.5",   // 私网
		"127.0.0.1",    // 回环
		"169.254.1.1",  // 链路本地
		"100.64.0.1",   // CGNAT
		"203.0.113.10", // 文档保留段（测试常用，但 CA 验证不了）
		"198.51.100.7", // 文档保留段
		"::1",          // IPv6 回环
	}
	for _, d := range bad {
		err := ValidateACMEDomains([]string{d}, "http-01")
		if err == nil {
			t.Errorf("%s 是私网/保留地址，公信 CA 不可能验证 → 必须提前拒绝", d)
			continue
		}
		if !strings.Contains(err.Error(), "公信 CA") {
			t.Errorf("%s 的错误信息应解释原因：%v", d, err)
		}
	}

	// ③ 域名照常通过（含通配符 + dns-01）
	for _, c := range []struct{ d, ch string }{
		{"example.com", "http-01"},
		{"sub.example.com", "http-01"},
		{"*.example.com", "dns-01"},
	} {
		if err := ValidateACMEDomains([]string{c.d}, c.ch); err != nil {
			t.Errorf("%s (%s) 应通过：%v", c.d, c.ch, err)
		}
	}

	// ④ 单标签名字 / 通配符配 HTTP-01 / 空输入 → 拒绝
	if err := ValidateACMEDomains([]string{"localhost"}, "http-01"); err == nil {
		t.Error("单标签名字公信 CA 不签，应拒绝")
	}
	if err := ValidateACMEDomains([]string{"*.example.com"}, "http-01"); err == nil {
		t.Error("通配符必须走 dns-01，应拒绝")
	}
	if err := ValidateACMEDomains(nil, "http-01"); err == nil {
		t.Error("空输入应拒绝")
	}
	if err := ValidateACMEDomains([]string{"  "}, "http-01"); err == nil {
		t.Error("空白名字应拒绝")
	}
}
