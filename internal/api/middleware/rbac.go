package middleware

import (
	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
	domainuser "corefusion/internal/domain/user"
)

// roleRank maps a role to its privilege level. A higher rank implies all the
// capabilities of the lower ranks.
//
// Ranks are intentionally coarse-grained: CoreFusion's admin surface is small
// enough that a full permission matrix (casbin et al.) would add more moving
// parts than it removes. If per-resource grants are ever needed, replace this
// map with a policy table rather than growing more special cases here.
var roleRank = map[string]int{
	domainuser.RoleViewer:     10,
	domainuser.RoleOperator:   20,
	domainuser.RoleAdmin:      30,
	domainuser.RoleSuperAdmin: 40,
}

// RankOf returns the privilege rank of a role. Unknown or empty roles rank 0,
// i.e. below every named role, so an unrecognised value fails closed.
func RankOf(role string) int {
	return roleRank[role]
}

// RequireRole returns a middleware allowing only principals whose role ranks at
// or above minRole. It must be mounted after AuthRequired, which is what puts
// the "role" key into the context.
func RequireRole(minRole string) gin.HandlerFunc {
	required := roleRank[minRole]
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role == "" {
			// No role in context means AuthRequired did not run. Treat this as
			// a wiring bug rather than silently allowing the request through.
			response.Unauthorized(c, "MISSING_ROLE", "authentication required")
			c.Abort()
			return
		}

		if roleRank[role] < required {
			response.Forbidden(c, "INSUFFICIENT_ROLE", "insufficient privileges for this operation")
			c.Abort()
			return
		}

		c.Next()
	}
}
