package main

import (
	"testing"
	"time"
)

func TestIssueACMECertValidation(t *testing.T) {
	// 没有标识符 → 直接拒绝 ✓（不去打 CA ✗）
	if _, err := issueACMECert(acmeIssueParams{Email: "a@b.com"}); err == nil {
		t.Fatal("没有标识符必须报错")
	}
	// 没有邮箱 → 拒绝 ✓（LE 要求账号邮箱 ✓）
	if _, err := issueACMECert(acmeIssueParams{Identifiers: []string{"test.example.com"}}); err == nil {
		t.Fatal("没有邮箱必须报错")
	}
	// 私网地址 → 拒绝 ✓（不是公网就不该去签 ✓）
	if _, err := issueACMECert(acmeIssueParams{Identifiers: []string{"192.168.1.8"}, Email: "a@b.com"}); err == nil {
		t.Fatal("私网 IP 必须被拒绝（先于打 CA）")
	}
}

func TestAgentHelpers(t *testing.T) {
	if got := firstNonEmpty("", " ", "x", "y"); got != "x" {
		t.Fatalf("firstNonEmpty=%q", got)
	}
	if got := parseTimeout("90s"); got != 90*time.Second {
		t.Fatalf("parseTimeout(90s)=%v", got)
	}
	if got := parseTimeout("bogus"); got != 5*time.Minute {
		t.Fatalf("parseTimeout 默认应为 5m，实际 %v", got)
	}
	m := dnsEnvMap([]string{"A=1", "B=2", "bad", "=3"})
	if len(m) != 2 || m["A"] != "1" || m["B"] != "2" {
		t.Fatalf("dnsEnvMap=%v", m)
	}
}
