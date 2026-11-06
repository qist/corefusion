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

// 多域名站点的 key 允许 "a.com, b.com"，但 rule_dir 是文件路径：必须归一成单个合法目录（取首个域名）。
func TestDomainRuleDirForMultiDomainGroup(t *testing.T) {
	group := "a.example.com, b.example.org, c.example.net, d.example.io"
	if got, want := DomainRuleDirFor(group), "domains/a.example.com"; got != want {
		t.Fatalf("DomainRuleDirFor(%q) = %q, want %q", group, got, want)
	}
	if got, want := DomainRuleDirFor("Example.COM:443"), "domains/example.com"; got != want {
		t.Fatalf("带端口/大写应归一：got %q, want %q", got, want)
	}
	if got, want := DomainRuleDirFor("*.example.com"), "domains/_wildcard.example.com"; got != want {
		t.Fatalf("通配符应归一：got %q, want %q", got, want)
	}
	if got, want := DomainRuleDirFor(""), "domains/default"; got != want {
		t.Fatalf("空域名应回落：got %q, want %q", got, want)
	}
}

// 历史/手工写入的非法 rule_dir 在读写两侧都要归一；合法的目录与自定义绝对路径保持不变。
func TestNormalizeRuleDir(t *testing.T) {
	group := "www.example.com, www.example.org"
	cases := []struct{ domain, in, want string }{
		{group, "domains/www.example.com, www.example.org", "domains/www.example.com"},
		{group, "domains/www.example.com", "domains/www.example.com"},
		{group, "", ""},
		{group, "/custom/waf/rules", "/custom/waf/rules"},
		{group, "domains/../escape", "domains/www.example.com"},
		{group, "domains/", "domains/www.example.com"},
		{group, "domains/www.example.com:443", "domains/www.example.com"},
	}
	for _, c := range cases {
		if got := NormalizeRuleDir(c.domain, c.in); got != c.want {
			t.Errorf("NormalizeRuleDir(%q, %q) = %q, want %q", c.domain, c.in, got, c.want)
		}
	}

	fixed := NormalizeRuleDir(group, "domains/www.example.com, www.example.org")
	if again := NormalizeRuleDir(group, fixed); again != fixed {
		t.Errorf("归一必须幂等：%q → %q", fixed, again)
	}
}
