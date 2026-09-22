package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/auth/jwt"
)

type fakeVersionProvider struct{ versions map[int64]int }

func (f *fakeVersionProvider) GetTokenVersion(_ context.Context, userID int64) (int, error) {
	return f.versions[userID], nil
}

// 安全审计 v5 M8：JWT 无状态，登出原本只是前端丢 token ✗ —— 被窃取的 token 在有效期内
// （access 24h / refresh 7d）仍然可用。这里断言"版本对不上即拒绝"。
func TestAuthRequiredRejectsRevokedTokenVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test-secret-m8", time.Hour, time.Hour)
	SetJWT(j)
	provider := &fakeVersionProvider{versions: map[int64]int{7: 3}}
	SetTokenVersionProvider(provider)
	defer SetTokenVersionProvider(nil)

	call := func(token string) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)
		AuthRequired()(c)
		return w.Code
	}

	// ① 版本一致 → 放行
	okTok, err := j.GenerateAccessTokenWithVersion(7, "u@example.com", "admin", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := call(okTok); got != http.StatusOK {
		t.Fatalf("版本一致应放行，得到 %d", got)
	}

	// ② 旧版本（登出前签发）→ 必须 401
	staleTok, err := j.GenerateAccessTokenWithVersion(7, "u@example.com", "admin", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := call(staleTok); got != http.StatusUnauthorized {
		t.Fatalf("已被吊销的 token 必须 401，得到 %d（登出形同虚设）", got)
	}

	// ③ 历史 token（无 tv 声明 → 0），而当前版本是 3 → 也必须拒绝
	legacyTok, err := j.GenerateAccessToken(7, "u@example.com", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got := call(legacyTok); got != http.StatusUnauthorized {
		t.Fatalf("缺少 tv 声明的 token 在版本被提升后必须 401，得到 %d", got)
	}

	// ④ 版本提升后，新签发的 token 立即可用
	provider.versions[7] = 4
	newTok, err := j.GenerateAccessTokenWithVersion(7, "u@example.com", "admin", 4)
	if err != nil {
		t.Fatal(err)
	}
	if got := call(newTok); got != http.StatusOK {
		t.Fatalf("重新登录后的 token 应可用，得到 %d", got)
	}
}

// 未注入 provider 时（例如某些单测）不校验版本，保持兼容。
func TestAuthRequiredSkipsVersionCheckWithoutProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	SetJWT(jwt.New("test-secret-m8", time.Hour, time.Hour))
	SetTokenVersionProvider(nil)

	tok, err := jwt.New("test-secret-m8", time.Hour, time.Hour).GenerateAccessToken(1, "a", "admin")
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tok)
	AuthRequired()(c)
	if w.Code != http.StatusOK {
		t.Errorf("未注入 provider 时应跳过版本校验，得到 %d", w.Code)
	}
}
