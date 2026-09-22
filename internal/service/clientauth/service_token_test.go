package clientauth

import (
	"testing"
	"time"

	"corefusion/internal/auth/jwt"
)

// 安全审计 v5 M10 真机踩到的坑：客户端与管理端**共用同一把 JWT 签名密钥** ✗，
// 所以管理端 access token 也能通过 ValidateAccessToken（其 user_id 被当成 client_id ✗）
// → 管理端身份可以调所有客户端受保护接口，还能从 `POST /client-auth/ticket` 签出客户端票据
// （真机实测 200 ✗，本应 401）。严格校验必须要求 role == "client"。
func TestValidateClientAccessTokenRejectsAdminToken(t *testing.T) {
	h := jwt.New("test-secret-for-client-token-check", time.Hour, time.Hour)
	svc := &Service{jwt: h}

	// 客户端 token → 通过，且拿到的就是 client id
	clientToken, err := h.GenerateAccessToken(42, "user@example.com", "client")
	if err != nil {
		t.Fatalf("生成客户端 token 失败：%v", err)
	}
	if id, err := svc.ValidateClientAccessToken(clientToken); err != nil || id != 42 {
		t.Errorf("客户端 token 应通过并返回 client id 42：id=%d err=%v", id, err)
	}

	// 管理端 token（同一把密钥、同样是 access 类型）→ 必须被拒
	for _, role := range []string{"admin", "super_admin", "operator", ""} {
		adminToken, err := h.GenerateAccessToken(1, "root", role)
		if err != nil {
			t.Fatalf("生成管理端 token 失败：%v", err)
		}
		if id, err := svc.ValidateClientAccessToken(adminToken); err == nil {
			t.Errorf("role=%q 的 token 不得作为客户端凭证（否则管理端身份可冒充客户端）：id=%d", role, id)
		}
	}

	// 角色大小写不敏感（role 一律由服务端写入，容忍大小写更稳）
	upperToken, _ := h.GenerateAccessToken(7, "c@example.com", "CLIENT")
	if _, err := svc.ValidateClientAccessToken(upperToken); err != nil {
		t.Errorf("role 应大小写不敏感：%v", err)
	}

	// 无签名/损坏的 token → 拒绝
	if _, err := svc.ValidateClientAccessToken("not-a-jwt"); err == nil {
		t.Error("非法 token 必须拒绝")
	}
}
