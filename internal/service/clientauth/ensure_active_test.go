package clientauth

import (
	"strings"
	"testing"

	"corefusion/internal/model/entity"
)

// 安全审计 v5 M2：被管理员停用的客户端不允许再登录/刷新 token。
func TestEnsureClientActive(t *testing.T) {
	if err := ensureClientActive(&entity.Client{Status: "active"}); err != nil {
		t.Errorf("active 客户端应放行，得到 %v", err)
	}

	for _, st := range []string{"suspended", "expired", "", "pending", "unknown"} {
		err := ensureClientActive(&entity.Client{Status: st})
		if err == nil {
			t.Errorf("状态 %q 必须被拒（失败关闭）", st)
		}
	}

	if err := ensureClientActive(nil); err == nil {
		t.Error("nil 客户端应报错")
	}
	// 错误信息要能让用户知道该找客服（不要把内部状态当敏感信息，但要可诊断）
	err := ensureClientActive(&entity.Client{Status: "suspended"})
	if err == nil || !strings.Contains(err.Error(), "suspended") {
		t.Errorf("错误信息应包含状态，得到 %v", err)
	}
}
