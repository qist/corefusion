// Role-based access control helpers for the admin UI.
//
// Role rank (must stay in sync with backend internal/domain/user/role.go):
//   viewer(10) < operator(20) < admin(30) < super_admin(40)
// A role can perform an action if its rank is >= the action's required rank.

export const ROLE_RANK = {
  viewer: 10,
  operator: 20,
  admin: 30,
  super_admin: 40,
}

// Returns true if the current user's role meets or exceeds `requiredRole`.
// `requiredRole` defaults to 'admin' (most management surfaces).
// Falls back to false when the role is unknown/empty (fail-closed).
export function hasRole(requiredRole = 'admin', currentRole = '') {
  const req = ROLE_RANK[requiredRole]
  const cur = ROLE_RANK[currentRole]
  if (req == null || cur == null) return false
  return cur >= req
}
