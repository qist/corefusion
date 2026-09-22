package subscription

import "testing"

// 审计 v5 待决项：`security_mode` / `bind_mode` 是"设计文档写了、代码里没有任何执行点"的漂移 ✗ ——
// 存进库**不会带来任何保护**，却会让人以为订阅被签名/绑定校验保护了（虚假安全感 ✗）。
// 修法：写入时**拒绝未实现的取值**（失败关闭 ✓），要么真实现、要么别承诺。
func TestValidateSecurityFields(t *testing.T) {
	ok := []struct{ sec, bind string }{
		{"", ""}, // 调用方已填默认值，空串兜底
		{SecurityModeTokenOnly, ""},
		{"", BindModeNone},
		{SecurityModeTokenOnly, BindModeNone},
	}
	for _, c := range ok {
		if err := ValidateSecurityFields(c.sec, c.bind); err != nil {
			t.Errorf("已实现/默认值应通过：(%q,%q) → %v", c.sec, c.bind, err)
		}
	}

	bad := []struct{ sec, bind string }{
		{SecurityModeTokenWithExpiry, "none"},
		{SecurityModeTokenWithSignature, "none"},
		{SecurityModeTokenWithBinding, "none"},
		{"token_only", "ip"},
		{"whatever", "none"},
	}
	for _, c := range bad {
		if err := ValidateSecurityFields(c.sec, c.bind); err == nil {
			t.Errorf("未实现/未知取值必须被拒（否则落库后毫无保护却像受保护）：(%q,%q)", c.sec, c.bind)
		}
	}
}
