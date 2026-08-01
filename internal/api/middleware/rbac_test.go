package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	domainuser "corefusion/internal/domain/user"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// runWithRole exercises RequireRole with the given role already in context,
// mimicking what AuthRequired does, and reports the resulting status code.
func runWithRole(t *testing.T, role, minRole string) int {
	t.Helper()

	w := httptest.NewRecorder()
	c, engine := gin.CreateTestContext(w)

	engine.GET("/t", func(ctx *gin.Context) {
		if role != "" {
			ctx.Set("role", role)
		}
	}, RequireRole(minRole), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	c.Request = httptest.NewRequest(http.MethodGet, "/t", nil)
	engine.ServeHTTP(w, c.Request)
	return w.Code
}

func TestRequireRoleAllowsEqualAndHigher(t *testing.T) {
	cases := []struct{ role, min string }{
		{domainuser.RoleSuperAdmin, domainuser.RoleSuperAdmin},
		{domainuser.RoleSuperAdmin, domainuser.RoleAdmin},
		{domainuser.RoleAdmin, domainuser.RoleAdmin},
		{domainuser.RoleAdmin, domainuser.RoleViewer},
		{domainuser.RoleOperator, domainuser.RoleViewer},
	}
	for _, tc := range cases {
		if got := runWithRole(t, tc.role, tc.min); got != http.StatusOK {
			t.Errorf("role=%s min=%s: got %d, want 200", tc.role, tc.min, got)
		}
	}
}

// The core regression: a low-privilege principal must not reach super_admin
// endpoints such as administrator account creation.
func TestRequireRoleRejectsLower(t *testing.T) {
	cases := []struct{ role, min string }{
		{domainuser.RoleViewer, domainuser.RoleSuperAdmin},
		{domainuser.RoleOperator, domainuser.RoleSuperAdmin},
		{domainuser.RoleAdmin, domainuser.RoleSuperAdmin},
		{domainuser.RoleViewer, domainuser.RoleAdmin},
	}
	for _, tc := range cases {
		if got := runWithRole(t, tc.role, tc.min); got != http.StatusForbidden {
			t.Errorf("role=%s min=%s: got %d, want 403", tc.role, tc.min, got)
		}
	}
}

// An unrecognised role ranks below everything, so it must fail closed rather
// than accidentally being treated as privileged.
func TestRequireRoleUnknownRoleFailsClosed(t *testing.T) {
	if got := runWithRole(t, "wizard", domainuser.RoleViewer); got != http.StatusForbidden {
		t.Errorf("unknown role: got %d, want 403", got)
	}
}

// Missing role means AuthRequired never ran; that is a wiring bug and must not
// be silently allowed.
func TestRequireRoleMissingRoleUnauthorized(t *testing.T) {
	if got := runWithRole(t, "", domainuser.RoleViewer); got != http.StatusUnauthorized {
		t.Errorf("missing role: got %d, want 401", got)
	}
}
