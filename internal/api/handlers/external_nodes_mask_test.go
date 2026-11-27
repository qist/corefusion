package handlers

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	domainuser "corefusion/internal/domain/user"
)

// 只读角色不能拿到外部源的上游 URL（内嵌 token）与节点凭据。
func TestExternalCredentialVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	roleCtx := func(role string) *gin.Context {
		c, _ := gin.CreateTestContext(nil)
		c.Set("role", role)
		return c
	}

	if canSeeExternalCredential(roleCtx(domainuser.RoleViewer)) {
		t.Fatal("viewer should not see external credentials")
	}
	if !canSeeExternalCredential(roleCtx(domainuser.RoleOperator)) {
		t.Fatal("operator should see external credentials")
	}

	masked := maskExternalURL("https://upstream.example.com/sub/0123456789abcdef?token=secret")
	if strings.Contains(masked, "0123456789abcdef") || strings.Contains(masked, "secret") {
		t.Fatalf("masked url leaks credential: %q", masked)
	}
	if !strings.HasPrefix(masked, "https://upstream.example.com/") {
		t.Fatalf("masked url lost host: %q", masked)
	}
}
