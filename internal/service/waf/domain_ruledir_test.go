package waf

import "testing"

// TestValidateWAFDomainRuleDir 锁定安全审计 v6 H-4：
// rule_dir 来自域配置 JSON，`..` 必须被拒（合法值是 domains/<domain>）。
func TestValidateWAFDomainRuleDir(t *testing.T) {
	ok := `{"waf_enable":"on","rule_dir":"domains/example.com"}`
	if err := validateWAFDomainRuleDir("example.com", ok); err != nil {
		t.Fatalf("legitimate rule_dir rejected: %v", err)
	}
	if err := validateWAFDomainRuleDir("example.com", `{"waf_enable":"off"}`); err != nil {
		t.Fatalf("empty rule_dir rejected: %v", err)
	}
	bad := `{"waf_enable":"on","rule_dir":"../../../../root/.ssh"}`
	if err := validateWAFDomainRuleDir("example.com", bad); err == nil {
		t.Fatal("traversal rule_dir must be rejected")
	}
	bad2 := `{"waf_enable":"on","rule_dir":"domains/../../../etc/cron.d"}`
	if err := validateWAFDomainRuleDir("example.com", bad2); err == nil {
		t.Fatal("nested traversal rule_dir must be rejected")
	}
}
