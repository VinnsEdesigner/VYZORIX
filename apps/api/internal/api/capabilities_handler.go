package api

import (
	"net/http"
	"strings"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
	"github.com/gin-gonic/gin"
)

// capabilitiesHandler handles GET /v1/me/capabilities.
// @Summary      Get effective capabilities
// @Description  Returns the caller's effective permissions across the personal, organization and instance grant stores, as "action:scope" strings.
// @Tags         me
// @Accept       json
// @Produce      json
// @Success      200  {object}  openapi.CapabilitiesResult  "effective capabilities"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Router       /me/capabilities [get]
//
// It reports the caller's effective permissions as three namespaces, one per
// grant store:
//
//	personal  ← constant (+ account policy)
//	org       ← UnionScopes(role, customGrants)   // selected org
//	instance  ← DefaultScopesForInstanceRole(user.InstanceRole)
//
// The namespaces are returned separately, not merged, because they are separate
// stores: a client renders personal capabilities on the account page, instance
// capabilities on the deployment page, and org capabilities inside an org. The
// server is the one derivation — the client never recomputes a capability from
// a role name, which is what keeps "button shown ⇒ server allows" true.
func (s *Server) capabilitiesHandler(c *gin.Context) {
	op := middleware.GetUserFromContext(c)
	if op == nil {
		responses.RespondStructured(c, http.StatusUnauthorized, "authentication required")
		return
	}

	// One derivation: effective = personal ∪ instance ∪ org. The three
	// namespaces are then split by scope prefix, because a client renders them in
	// different contexts (account page, deployment page, inside an org).
	orgID := middleware.GetOrganizationID(c)

	var (
		orgRole string
		grants  []*permission.ResourcePermission
	)
	if orgID != "" {
		grants = s.effectiveOrgGrants(c, op.ID, orgID)
		if m := s.activeMembership(c, op, orgID); m != nil {
			orgRole = string(m.Role)
		}
	}

	// A minimal in-memory user (API-key auth builds one from an id alone) has no
	// role set; normalise so the wire value is a known role rather than "".
	instanceRole := permission.ParseInstanceRole(string(op.InstanceRole))
	effective := permission.EffectiveScopes(instanceRole, orgRole, grants)

	personal, instance, orgScopes := partitionScopes(effective)

	c.JSON(http.StatusOK, openapi.CapabilitiesResult{
		InstanceRole: string(instanceRole),
		Personal:     wireScopes(personal),
		Instance:     wireScopes(instance),
		Org:          wireScopes(orgScopes),
	})
}

// partitionScopes splits an effective permission set by scope prefix into the
// personal, instance and org namespaces. The three stores are disjoint by
// prefix, so the split is exact.
func partitionScopes(effective permission.ScopedPermissions) (personal, instance, org permission.ScopedPermissions) {
	for _, p := range effective {
		switch {
		case strings.HasPrefix(p.Scope, permission.ScopePersonal):
			personal = append(personal, p)
		case strings.HasPrefix(p.Scope, permission.ScopeInstance):
			instance = append(instance, p)
		default:
			org = append(org, p)
		}
	}

	return personal, instance, org
}

// effectiveOrgGrants returns the caller's custom scoped grants in the org, or nil
// when no grant repository is configured.
func (s *Server) effectiveOrgGrants(c *gin.Context, userID, orgID string) []*permission.ResourcePermission {
	if s.grantRepo == nil {
		return nil
	}
	grants, _ := s.grantRepo.ListEffective(c.Request.Context(), userID, orgID)

	return grants
}

// activeMembership resolves the caller's active membership, preferring the
// in-memory memberships and falling back to the authoritative DB check, the
// same way requireScope does.
func (s *Server) activeMembership(c *gin.Context, op *user.User, orgID string) *organization.OrganizationMember {
	if m := op.GetMembership(orgID); m != nil && m.IsActive() {
		return m
	}
	if s.memberHandler == nil {
		return nil
	}
	loaded, err := s.memberHandler.MembershipChecker().GetMembership(c.Request.Context(), op.ID, orgID)
	if err != nil {
		return nil
	}
	m, ok := loaded.(*organization.OrganizationMember)
	if !ok || !m.IsActive() {
		return nil
	}

	return m
}

// wireScopes renders a permission set as the "action:scope" strings the API
// contract uses. A nil set renders as an empty list, never null.
func wireScopes(perms permission.ScopedPermissions) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		out = append(out, string(p.Action)+":"+p.Scope)
	}

	return out
}
