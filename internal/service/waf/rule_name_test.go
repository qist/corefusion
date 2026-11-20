package waf

import "testing"

// 安全审计 v7 H-2：规则名会作为 waf-rules/ 下的文件名，必须拒绝路径穿越。
func TestValidateRuleNameRejectsTraversal(t *testing.T) {
	bad := []string{
		"", " ", ".", "..", "../x", "../../etc/cron.d/x",
		"a/b", `a\b`, "a\x00b", "rules/../../x",
		"-leading-dash", ".hidden", "a b", "a;b",
	}
	for _, name := range bad {
		if err := ValidateRuleName(name); err == nil {
			t.Errorf("ValidateRuleName(%q) = nil, want error", name)
		}
	}
}

func TestValidateRuleNameAcceptsNormalNames(t *testing.T) {
	good := []string{
		"args.rule", "custom_rule.json", "sql-injection.rule",
		"geoip2-cn.rule", "rule.v2", "A1",
	}
	for _, name := range good {
		if err := ValidateRuleName(name); err != nil {
			t.Errorf("ValidateRuleName(%q) = %v, want nil", name, err)
		}
	}
}
