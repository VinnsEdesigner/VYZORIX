package api

import (
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/gin-gonic/gin"
)

// setupInstanceRoutes registers the deployment-scoped surface under /v1/instance.
//
// The group is the declaration of context: every route in it is guarded by
// requireInstanceScope, so authority comes from the user's InstanceRole and never
// from an org membership. The reverse holds too, which keeps the two axes
// orthogonal rather than hierarchical.
func (s *Server) setupInstanceRoutes(r *gin.RouterGroup) {
	instance := r.Group("/instance")
	instance.Use(middleware.NoCache())
	{
		// admin.lockout names a user and locks them out of the deployment, so its
		// blast radius is not contained by any org. The org-local sibling is
		// members.write (suspend/reinstate a member of this org), which stays on the
		// org routes: a different action, not a duplicate.
		instance.POST("/lockout/unlock/:user_id",
			s.requireInstanceScope(permission.ActionAdminLockout),
			s.authHandlers.Lockout.UnlockAccount)
	}
}
