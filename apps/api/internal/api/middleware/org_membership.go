package middleware

import (
	"context"
	"net/http"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/gin-gonic/gin"
)

// ContextKeyMembership is the key used to store organization membership in gin context.
const ContextKeyMembership = "org_membership"

// OrganizationMembershipChecker defines the interface for checking organization membership.
type OrganizationMembershipChecker interface {
	GetMembership(ctx context.Context, userID, orgID string) (interface{}, error)
}

// OrganizationMembership middleware validates user's membership in the organization.
type OrganizationMembership struct {
	membershipChecker OrganizationMembershipChecker
}

// NewOrganizationMembership creates a new OrganizationMembership middleware.
func NewOrganizationMembership(checker OrganizationMembershipChecker) *OrganizationMembership {
	return &OrganizationMembership{
		membershipChecker: checker,
	}
}

// Middleware returns the gin middleware handler.
func (m *OrganizationMembership) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := GetOrganizationID(c)
		if orgID == "" {
			responses.RespondStructuredAbort(c, http.StatusBadRequest,

				"organization context required",
			)
			return
		}

		// A service account has no user and no membership row; its credential is the
		// membership, and auth already pinned the org to the owning account.
		if c.GetString("auth_type") == "service_account" {
			if acctOrg := c.GetString("service_account_org_id"); acctOrg != "" && acctOrg != orgID {
				responses.RespondStructuredAbort(c, http.StatusForbidden, "service account does not belong to this organization")
				return
			}
			c.Next()
			return
		}

		// Get user from context (set by auth middleware).
		user := GetUserFromContext(c)
		if user == nil {
			responses.RespondStructuredAbort(c, http.StatusUnauthorized,

				"authentication required",
			)
			return
		}

		// If no membership checker is configured, deny access (fail secure).
		if m.membershipChecker == nil {
			responses.RespondStructuredAbort(c, http.StatusInternalServerError,

				"membership validation not configured",
			)
			return
		}

		// Check if user is a member of the organization.
		membership, err := m.membershipChecker.GetMembership(c.Request.Context(), user.ID, orgID)
		if err != nil {
			responses.RespondStructuredAbort(c, http.StatusForbidden,

				"not a member of this organization",
			)
			return
		}

		// Store actual membership object in context for downstream use.
		// GetMembership returns interface{}, so we need to assert the type.
		if member, ok := membership.(*organization.OrganizationMember); ok {
			c.Set(ContextKeyMembership, member)
		} else {
			// Fallback: store the raw value if it's not the expected type.
			c.Set(ContextKeyMembership, membership)
		}

		c.Next()
	}
}

// RequireOrganizationMembership is a convenience function for requiring organization membership.
// Note: This should only be used when the membership checker has been set via NewOrganizationMembership.
func RequireOrganizationMembership() gin.HandlerFunc {
	return NewOrganizationMembership(nil).Middleware()
}
