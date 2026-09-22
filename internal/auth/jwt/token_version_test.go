package jwt

import (
	"testing"
	"time"
)

// 安全审计 v5 M8：token 需要携带"用户令牌版本"，服务端版本提升后旧 token 立刻失效。
func TestTokenVersionRoundTrip(t *testing.T) {
	j := New("secret", time.Hour, time.Hour)

	tok, err := j.GenerateAccessTokenWithVersion(9, "u", "admin", 42)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := j.ValidateTyped(tok, TokenTypeAccess)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TokenVersion != 42 {
		t.Errorf("版本未往返：得到 %d", claims.TokenVersion)
	}

	// 不带版本的签发 → 0（历史 token 也是 0，所以升级部署不会踢人）
	old, err := j.GenerateAccessToken(9, "u", "admin")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := j.ValidateTyped(old, TokenTypeAccess)
	if err != nil {
		t.Fatal(err)
	}
	if c2.TokenVersion != 0 {
		t.Errorf("默认应为 0，得到 %d", c2.TokenVersion)
	}

	// refresh 也要带版本（登出后不能被用来续签）
	rt, err := j.GenerateRefreshTokenWithVersion(9, "u", "admin", 7)
	if err != nil {
		t.Fatal(err)
	}
	c3, err := j.ValidateTyped(rt, TokenTypeRefresh)
	if err != nil {
		t.Fatal(err)
	}
	if c3.TokenVersion != 7 {
		t.Errorf("refresh 版本未往返：%d", c3.TokenVersion)
	}
}
