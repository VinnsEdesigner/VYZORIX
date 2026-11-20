package api

import (
	"net/http"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/serviceaccount"
	"github.com/gin-gonic/gin"
)

// requireScope returns gin middleware that enforces a scoped permission: the
// caller must hold `action` on `scope` in the current organization, resolved
// through the scoped permission engine (role defaults unioned with custom
// grants). This mirrors the standard scoped-RBAC pattern `authorize(EvalPermission(action, scope))`
// at the route level. The scope is normally a resource-type wildcard
// ("devices:*"); per-resource narrowing happens in the handler (e.g. the
// command path's owner-or-group check).
func (s *Server) requireScope(action permission.Action, scope string) gin.HandlerFunc {
	s.recordGuard(guardStoreOrg, action, scope)

	return func(c *gin.Context) {
		// Service accounts have no user; evaluate the token's own permissions.
		if c.GetString("auth_type") == "service_account" {
			saToken := middleware.GetServiceAccountToken(c)
			if !serviceAccountGrants(saToken, middleware.GetOrganizationID(c), action) {
				responses.RespondStructured(c, http.StatusForbidden, "insufficient permissions")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		op := middleware.GetUserFromContext(c)
		if op == nil {
			responses.RespondStructured(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}

		orgID := middleware.GetOrganizationID(c)
		if orgID == "" {
			responses.RespondStructured(c, http.StatusBadRequest, "organization context required")
			c.Abort()
			return
		}

		m := op.GetMembership(orgID)
		if m == nil || !m.IsActive() {
			// The in-memory memberships are only populated on the session/login
			// path. API-key auth builds a minimal user (ID only), so fall
			// back to the authoritative DB membership check before denying.
			if s.memberHandler != nil {
				if loaded, err := s.memberHandler.MembershipChecker().GetMembership(c.Request.Context(), op.ID, orgID); err == nil {
					if member, ok := loaded.(*organization.OrganizationMember); ok && member.IsActive() {
						m = member
					}
				}
			}
		}
		if m == nil || !m.IsActive() {
			responses.RespondStructured(c, http.StatusForbidden, "not a member of this organization")
			c.Abort()
			return
		}

		var grants []*permission.Grant
		if s.grantRepo != nil {
			grants, _ = s.grantRepo.ListEffective(c.Request.Context(), op.ID, orgID)
		}
		eval := permission.NewEvaluator(string(m.Role), grants)
		if !eval.Grants(action, scope) {
			responses.RespondStructured(c, http.StatusForbidden, "insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

// requireInstanceScope returns gin middleware that enforces an instance-scoped
// permission: the caller must hold `action` on the deployment (instance:*),
// resolved from the user's InstanceRole. It consults exactly one grant store —
// the instance one — so a route in the instance group can never be reached by an
// org role, and an org route can never be reached by instance authority. This is
// what keeps the two axes orthogonal rather than hierarchical.
//
// The personal store is not consulted here: a personal grant is identity-derived
// and belongs to the caller's own account, not to the deployment.
func (s *Server) requireInstanceScope(action permission.Action) gin.HandlerFunc {
	scope := permission.WildcardScope(permission.ScopeInstance)
	s.recordGuard(guardStoreInstance, action, scope)

	return func(c *gin.Context) {
		op := middleware.GetUserFromContext(c)
		if op == nil {
			responses.RespondStructured(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}

		if !permission.DefaultScopesForInstanceRole(op.InstanceRole).Grants(action, scope) {
			responses.RespondStructured(c, http.StatusForbidden, "insufficient instance authority")
			c.Abort()
			return
		}

		c.Next()
	}
}

// serviceAccountGrants maps token scopes (read/write/admin) to the required
// action class: read satisfies *.read, write adds *.write, admin adds every
// remaining scope prefix.
// serviceAccountGrants evaluates a token's effective permissions: coarse scopes,
// its basic role's defaults, and its fixed-role grants.
func serviceAccountGrants(token *serviceaccount.Token, orgID string, action permission.Action) bool {
	if token == nil {
		return false
	}
	scope := permission.BuildScope(permission.RequiredScope(action), orgID)
	if scope == "" {
		return false
	}
	return token.EffectivePermissions().Grants(action, scope)
}
