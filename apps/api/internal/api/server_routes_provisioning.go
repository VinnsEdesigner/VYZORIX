package api

import (
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/gin-gonic/gin"
)

// setupProvisioningWebhookRoute registers the provider push receiver.
//
// It is mounted on the PUBLIC group, not the tenant group: a provider has no session, no
// API key and no organisation header. Safety rests entirely on the HMAC signature, which
// the handler verifies against the repository's own secret before anything happens. The
// public group's rate limiter still applies.
func (s *Server) setupProvisioningWebhookRoute(public *gin.RouterGroup) {
	if s.provisioningWebhookHandler == nil {
		return
	}

	public.POST("/v1/provisioning/webhooks/:repoID", s.provisioningWebhookHandler.Receive)
}

// setupProvisioningRoutes registers the GitOps provisioning surface.
//
// Every route is org-scoped: the organisation is resolved from the credential by
// OrganizationContext, and the caller must hold provisioning.read or
// provisioning.write in it. The two actions are separate because reading is
// harmless and writing can rewrite any device's config.
func (s *Server) setupProvisioningRoutes(r *gin.RouterGroup) {
	if s.provisioningConnectionHandler == nil {
		return
	}

	group := r.Group("/provisioning")
	group.Use(middleware.NewOrganizationContext(nil).Middleware())
	if s.memberHandler != nil {
		group.Use(middleware.NewOrganizationMembership(s.memberHandler.MembershipChecker()).Middleware())
	}
	group.Use(middleware.NoCache())

	read := s.requireScope(permission.ActionProvisioningRead, permission.WildcardScope(permission.ScopeOrg))
	write := s.requireScope(permission.ActionProvisioningWrite, permission.WildcardScope(permission.ScopeOrg))

	if s.provisioningRepositoryHandler != nil {
		group.GET("/repositories", read, s.provisioningRepositoryHandler.List)
		group.POST("/repositories", write, s.provisioningRepositoryHandler.Create)
		group.GET("/repositories/:id", read, s.provisioningRepositoryHandler.Get)
		group.PATCH("/repositories/:id", write, s.provisioningRepositoryHandler.Update)
		group.DELETE("/repositories/:id", write, s.provisioningRepositoryHandler.Delete)
	}

	if s.provisioningPushBackHandler != nil {
		// Preview and push share the write scope: an edit writes to a repository, which is
		// fleet-wide deploy authority, so it sits with provisioning.write rather than read.
		group.POST("/repositories/:id/resources/:resourceId/preview", write,
			s.provisioningPushBackHandler.Preview)
		group.POST("/repositories/:id/resources/:resourceId/push", write,
			s.provisioningPushBackHandler.Apply)
	}

	if s.provisioningSyncHandler != nil {
		group.POST("/repositories/:id/sync", write, s.provisioningSyncHandler.Sync)
		// A single-resource sync takes the same write scope as a full sync: applying one
		// file still changes what a device runs.
		group.POST("/repositories/:id/resources/:resourceId/sync", write, s.provisioningSyncHandler.SyncResource)
		group.GET("/repositories/:id/resources", read, s.provisioningSyncHandler.ListResources)
		group.GET("/repositories/:id/resources/:resourceId/document", read, s.provisioningSyncHandler.GetResourceDocument)
		// Declared secrets and whether each is set. Read scope: it names keys, never values.
		group.GET("/repositories/:id/resources/:resourceId/secrets", read, s.provisioningSyncHandler.GetResourceSecrets)
		group.GET("/repositories/:id/jobs", read, s.provisioningSyncHandler.ListJobs)
		group.GET("/repositories/:id/jobs/:jobId", read, s.provisioningSyncHandler.GetJob)
		group.GET("/repositories/:id/health", read, s.provisioningSyncHandler.GetHealth)
		group.GET("/repositories/:id/discovery", read, s.provisioningSyncHandler.Discover)
		group.GET("/repositories/:id/events", read, s.provisioningSyncHandler.ListResourceEvents)
		group.GET("/events", read, s.provisioningSyncHandler.ListEvents)
		// The instance summary is org-scoped: it counts one organisation's repositories,
		// resources, and connections. It is therefore mounted inside this group, at
		// /v1/provisioning/instance rather than repeating the prefix.
		group.GET("/instance", read, s.provisioningSyncHandler.InstanceProvisioning)
	}

	group.GET("/connections", read, s.provisioningConnectionHandler.List)
	group.POST("/connections", write, s.provisioningConnectionHandler.Create)
	group.GET("/connections/:id", read, s.provisioningConnectionHandler.Get)
	group.PATCH("/connections/:id", write, s.provisioningConnectionHandler.Update)
	group.DELETE("/connections/:id", write, s.provisioningConnectionHandler.Delete)
	group.POST("/connections/:id/check", write, s.provisioningConnectionHandler.Check)
}
