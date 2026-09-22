package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/auth/jwt"
)

// 安全审计 v5 S2：Portal 客户端 token 与管理端 token 共用同一把密钥、结构一致
// （只差 role="client"），而很多管理端路由只挂 AuthRequired、没有 RequireRole ——
// 于是**任意登录的 Portal 用户**都能调管理端接口。这里断言管理端中间件直接拒绝这类 token。
func TestAuthRequiredRejectsNonAdminRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test-secret-for-role-test", time.Hour, time.Hour)
	SetJWT(j)

	cases := []struct {
		role       string
		wantStatus int
		note       string
	}{
		{"client", http.StatusForbidden, "Portal 客户端 token 不得访问管理端（越权）"},
		{"bogus-role", http.StatusForbidden, "未知角色必须失败关闭，不能放行"},
		{"", http.StatusForbidden, "空角色必须失败关闭"},
		{"viewer", http.StatusOK, "管理端最低角色应放行"},
		{"operator", http.StatusOK, "运营角色应放行"},
		{"admin", http.StatusOK, "管理员应放行"},
		{"super_admin", http.StatusOK, "超级管理员应放行"},
	}
	for _, tc := range cases {
		token, err := j.GenerateAccessToken(1, "u@example.com", tc.role)
		if err != nil {
			t.Fatalf("签发 token(role=%q): %v", tc.role, err)
		}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
		c.Request.Header.Set("Authorization", "Bearer "+token)

		AuthRequired()(c)

		if w.Code != tc.wantStatus {
			t.Errorf("role=%q：期望 %d，得到 %d（%s）", tc.role, tc.wantStatus, w.Code, tc.note)
		}
	}
}

// 回归：原有行为不受影响 —— 缺 token 401、refresh token 不能当 access 用。
func TestAuthRequiredExistingBehaviour(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test-secret-for-role-test", time.Hour, time.Hour)
	SetJWT(j)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	AuthRequired()(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("缺 token 应 401，得到 %d", w.Code)
	}

	refresh, err := j.GenerateRefreshToken(1, "u@example.com", "admin")
	if err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	c2.Request.Header.Set("Authorization", "Bearer "+refresh)
	AuthRequired()(c2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("refresh token 不得当 access 用，得到 %d", w2.Code)
	}
}
