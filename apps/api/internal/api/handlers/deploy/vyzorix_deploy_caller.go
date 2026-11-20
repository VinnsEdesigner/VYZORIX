// Package deployhandlers provides the HTTP surface for the deploy contract
// . It is a thin adapter: every decision —
// who may deploy, what compiles, what is refused, what is signed — belongs to
// internal/deploy, and this package only turns a request into a call and a
// result into a response.
//
// The handler is the authentication boundary. The service takes a resolved
// *deploy.Caller, because a resolver inside the service cannot see the request
// context the user, the membership and the grants are carried in.
package deployhandlers

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// MembershipChecker resolves a user's membership in an organization. It is
// the middleware's own interface, so the checker the org-scope middleware uses is
// the one the caller resolver uses.
type MembershipChecker interface {
	GetMembership(ctx context.Context, userID, orgID string) (interface{}, error)
}

// GrantLister returns a user's effective scoped grants in an organization.
type GrantLister interface {
	ListEffective(ctx context.Context, userID, orgID string) ([]*permission.ResourcePermission, error)
}

// resolveCaller builds the deploy.Caller for the request's organization and the
// member id the deploy is attributed to.
//
// Two credentials reach here. A deploy credential is a grant scoped to the
// one device it names, so it is represented as exactly that — not as a super
// admin, which would be a lie the predicate could later act on. A user
// session or API key is resolved through the same membership check requireScope
// performs: the in-memory memberships are populated on the session path, but
// API-key auth builds a minimal user, so the database is the fallback.
func (h *Handler) resolveCaller(c *gin.Context, orgID string) (*deploy.Caller, string, bool) {
	if identity, ok := middleware.DeployCredentialFromContext(c); ok {
		// The credential's org is the authority; a caller cannot move it with a
		// header. A mismatch with the request's org is a refusal.
		if orgID != "" && identity.OrgID != orgID {
			return nil, "", false
		}

		return &deploy.Caller{
			OrgID:         identity.OrgID,
			Authenticated: true,
			Permissions: permission.ScopedPermissions{
				{
					Action: permission.ActionDeviceDeploy,
					Scope:  permission.BuildScope(permission.ScopeDevices, identity.DeviceID),
				},
			},
		}, identity.CreatedBy, true
	}

	op := middleware.GetUserFromContext(c)
	if op == nil {
		return nil, "", false
	}

	member := h.membership(c, op, orgID)
	if member == nil {
		return nil, "", false
	}

	var grants []*permission.ResourcePermission
	if h.grants != nil {
		grants, _ = h.grants.ListEffective(c.Request.Context(), op.ID, orgID)
	}

	return &deploy.Caller{
		OrgID:         orgID,
		Permissions:   permission.UnionScopes(string(member.Role), grants),
		Authenticated: true,
		IsSuperAdmin:  member.Role.IsSuperAdmin(),
	}, op.ID, true
}

// membership returns the active membership for the user in the org, from the
// in-memory set first and the database second.
func (h *Handler) membership(c *gin.Context, op *user.User, orgID string) *organization.OrganizationMember {
	if m := op.GetMembership(orgID); m != nil && m.IsActive() {
		return m
	}

	if h.membershipChecker == nil {
		return nil
	}

	loaded, err := h.membershipChecker.GetMembership(c.Request.Context(), op.ID, orgID)
	if err != nil {
		return nil
	}

	member, ok := loaded.(*organization.OrganizationMember)
	if !ok || !member.IsActive() {
		return nil
	}

	return member
}
