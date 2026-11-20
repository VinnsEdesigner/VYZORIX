package provisioninghandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
)

var _ openapi.SuccessResult

// DocumentReader reads the signed config a device currently holds, so a provisioned
// resource can show the exact bytes this repository caused to be applied.
//
// The org is passed through to the read so a resource row pointing at a device outside
// the organisation cannot be used to fetch another tenant's signed config.
type DocumentReader interface {
	CurrentDocument(ctx context.Context, orgID, deviceID string) (document string, version int, ok bool, err error)
	// SecretReadinessFor reports which secrets the device's current config declares, and
	// whether each is set. It reads names only, never a value.
	SecretReadinessFor(ctx context.Context, orgID, deviceID string) ([]deploy.SecretRequirement, bool, error)
}

// SyncHandler serves the sync, resource, and job routes.
type SyncHandler struct {
	repositories *svc.RepositoryService
	sync         *svc.SyncService
	resources    domain.ResourceRepository
	jobs         domain.JobRepository
	documents    DocumentReader
	counters     Counters
	events       domain.EventRepository
	presenter    *response.Presenter
	log          *slog.Logger
}

// SetDocumentReader injects the reader behind the resource document route.
func (h *SyncHandler) SetDocumentReader(r DocumentReader) { h.documents = r }

// NewSyncHandler builds a SyncHandler.
func NewSyncHandler(
	repositories *svc.RepositoryService,
	syncSvc *svc.SyncService,
	resources domain.ResourceRepository,
	jobs domain.JobRepository,
	presenter *response.Presenter,
) *SyncHandler {
	return &SyncHandler{
		repositories: repositories,
		sync:         syncSvc,
		resources:    resources,
		jobs:         jobs,
		presenter:    presenter,
	}
}

type resourceJSON struct {
	LastAppliedAt *int64 `json:"last_applied_at,omitempty"`
	ID            string `json:"id"`
	// Kind is what the resource declares, so a client can filter and group by it. An org
	// resource has no device_id.
	Kind          string `json:"kind"`
	DeviceID      string `json:"device_id"`
	Path          string `json:"path"`
	ContentHash   string `json:"content_hash"`
	State         string `json:"state"`
	LastError     string `json:"last_error,omitempty"`
	ConfigVersion int    `json:"config_version"`
}

type resourceListJSON struct {
	Resources []resourceJSON `json:"resources"`
	Total     int            `json:"total"`
}

type jobResultJSON struct {
	ResourceID string `json:"resource_id,omitempty"`
	Path       string `json:"path"`
	Outcome    string `json:"outcome"`
	Message    string `json:"message,omitempty"`
}

type jobJSON struct {
	FinishedAt *int64          `json:"finished_at,omitempty"`
	ID         string          `json:"id"`
	RepoID     string          `json:"repo_id"`
	State      string          `json:"state"`
	Error      string          `json:"error,omitempty"`
	Results    []jobResultJSON `json:"results,omitempty"`
	StartedAt  int64           `json:"started_at"`
	Applied    int             `json:"applied"`
	Skipped    int             `json:"skipped"`
	Failed     int             `json:"failed"`
	DryRun     bool            `json:"dry_run"`
}

func toResourceJSON(r *domain.Resource) resourceJSON {
	out := resourceJSON{
		ID:            r.ID,
		Kind:          string(r.Kind),
		DeviceID:      r.DeviceID,
		Path:          r.Path,
		ContentHash:   r.ContentHash,
		State:         string(r.State),
		LastError:     r.LastError,
		ConfigVersion: r.ConfigVersion,
	}
	if r.LastAppliedAt != nil {
		ms := r.LastAppliedAt.UnixMilli()
		out.LastAppliedAt = &ms
	}

	return out
}

func toJobJSON(j *domain.Job, results []*domain.JobResult) jobJSON {
	out := jobJSON{
		ID:        j.ID,
		RepoID:    j.RepoID,
		State:     string(j.State),
		Error:     j.Error,
		Applied:   j.Applied,
		Skipped:   j.Skipped,
		Failed:    j.Failed,
		DryRun:    j.DryRun,
		StartedAt: j.StartedAt.UnixMilli(),
	}
	if j.FinishedAt != nil {
		ms := j.FinishedAt.UnixMilli()
		out.FinishedAt = &ms
	}
	for _, r := range results {
		out.Results = append(out.Results, jobResultJSON{
			ResourceID: r.ResourceID,
			Path:       r.Path,
			Outcome:    string(r.Outcome),
			Message:    r.Message,
		})
	}

	return out
}

// actorFrom resolves who is making this request.
//
// It reads the identity the auth middleware already established rather than trusting
// anything in the body: a caller cannot claim to be someone else. A service account is
// checked first because that middleware sets auth_type alongside a minimal user, so
// testing for a user first would misattribute a token's action to a person.
func actorFrom(c *gin.Context) domain.Actor {
	if c.GetString("auth_type") == "service_account" {
		if token := middleware.GetServiceAccountToken(c); token != nil {
			return domain.Actor{
				Kind:  domain.ActorServiceAccount,
				ID:    token.ID,
				Label: token.Name,
			}
		}
	}

	if u := middleware.GetUserFromContext(c); u != nil {
		return domain.Actor{Kind: domain.ActorUser, ID: u.ID, Label: u.Email}
	}

	// An authenticated request that carries neither is a service account whose token
	// the middleware did not expose; attribute it to the token type rather than
	// leaving it blank.
	return domain.Actor{Kind: domain.ActorServiceAccount, ID: "unknown", Label: "service account"}
}

// Sync handles POST /v1/provisioning/repositories/:id/sync.
// @Summary      Sync a provisioning repository
// @Description  Fetches the branch, discovers manifests, diffs them against the resources this repository owns, and applies the new or changed ones through the deploy service. With dry_run=true nothing is applied; the response is the preview.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id       path   string  true  "repository ID"
// @Param        dry_run  query  bool    false  "preview only, apply nothing"
// @Param        revert_missing  query  bool  false  "revert devices whose manifest was removed"
// @Success      200  {object}  provisioninghandler.jobJSON  "sync result"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "a sync is already running"
// @Router       /provisioning/repositories/{id}/sync [post]
func (h *SyncHandler) Sync(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	// A dry run is parsed strictly and fails closed: only an explicitly true value previews,
	// and an unrecognised one is a 400 rather than a silent real sync. "?dry_run=1" reading
	// as false would apply to the fleet the thing the caller asked to preview.
	dryRun, ok := boolQuery(c, "dry_run")
	if !ok {
		h.presenter.BadRequest(c, "dry_run must be true or false")

		return
	}

	revertMissing, ok := boolQuery(c, "revert_missing")
	if !ok {
		h.presenter.BadRequest(c, "revert_missing must be true or false")

		return
	}

	opts := svc.SyncOptions{
		DryRun:        dryRun,
		RevertMissing: revertMissing,
		Actor:         actorFrom(c),
	}

	result, err := h.sync.Sync(c.Request.Context(), repo, opts)
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	// The sync already ran and its job row exists. If the per-manifest results cannot be
	// read, answering 500 would throw away the job id the caller needs to find the run —
	// so the job is returned with empty results rather than lost.
	results, err := h.jobs.ListResults(c.Request.Context(), result.Job.ID)
	if err != nil {
		h.logSync("could not read sync results", result.Job.ID, err)
		results = nil
	}

	c.JSON(http.StatusOK, toJobJSON(result.Job, results))
}

// SyncResource handles POST /v1/provisioning/repositories/:id/resources/:resourceId/sync.
// @Summary      Sync a single provisioned resource
// @Description  Fetches the branch and plans as a full sync does, then applies only this one resource. The job records the rest as skipped. The repository's sync state is not stamped, because the repository was not synced.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id          path  string  true  "repository ID"
// @Param        resourceId  path  string  true  "resource ID"
// @Param        dry_run     query  bool    false  "preview only, apply nothing"
// @Success      200  {object}  provisioninghandler.jobJSON  "sync result"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "repository or resource not found"
// @Failure      409  {object}  openapi.ErrorResponse  "a sync is already running"
// @Router       /provisioning/repositories/{id}/resources/{resourceId}/sync [post]
func (h *SyncHandler) SyncResource(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	dryRun, ok := boolQuery(c, "dry_run")
	if !ok {
		h.presenter.BadRequest(c, "dry_run must be true or false")

		return
	}

	result, err := h.sync.SyncResource(c.Request.Context(), repo, c.Param("resourceId"), svc.SyncOptions{
		DryRun: dryRun,
		Actor:  actorFrom(c),
	})
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	results, err := h.jobs.ListResults(c.Request.Context(), result.Job.ID)
	if err != nil {
		h.logSync("could not read sync results", result.Job.ID, err)
		results = nil
	}

	c.JSON(http.StatusOK, toJobJSON(result.Job, results))
}

// secretRequirementJSON is one declared secret and whether it is set.
type secretRequirementJSON struct {
	App string `json:"app"`
	Key string `json:"key"`
	Set bool   `json:"set"`
}

// secretReadinessJSON is a resource's declared secrets.
type secretReadinessJSON struct {
	Secrets []secretRequirementJSON `json:"secrets"`
	Total   int                     `json:"total"`
	// Missing counts the declared keys the device does not have, so a client can badge the
	// resource without scanning the list.
	Missing int `json:"missing"`
}

// GetResourceSecrets handles
// GET /v1/provisioning/repositories/:id/resources/:resourceId/secrets.
// @Summary      List a provisioned resource's declared secrets and whether each is set
// @Description  Reads the names the device's current config declares and reports which are set. Never returns a value.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id          path  string  true  "repository ID"
// @Param        resourceId  path  string  true  "resource ID"
// @Success      200  {object}  provisioninghandler.secretReadinessJSON  "declared secrets"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "resource not found, or the device has no config"
// @Router       /provisioning/repositories/{id}/resources/{resourceId}/secrets [get]
func (h *SyncHandler) GetResourceSecrets(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	resource, err := h.resources.GetByID(c.Request.Context(), repo.ID, c.Param("resourceId"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	if h.documents == nil {
		h.presenter.NotImplemented(c, "the document reader is not configured")

		return
	}

	// An org resource has no device, so it declares no device secrets.
	if resource.DeviceID == "" {
		c.JSON(http.StatusOK, secretReadinessJSON{Secrets: []secretRequirementJSON{}})

		return
	}

	requirements, ok, err := h.documents.SecretReadinessFor(c.Request.Context(), repo.OrgID, resource.DeviceID)
	if err != nil {
		h.presenter.InternalError(c, "failed to read secret readiness")

		return
	}
	if !ok {
		h.presenter.NotFound(c, "the device has no config")

		return
	}

	out := make([]secretRequirementJSON, 0, len(requirements))
	missing := 0

	for _, r := range requirements {
		out = append(out, secretRequirementJSON{App: r.App, Key: r.Key, Set: r.Set})
		if !r.Set {
			missing++
		}
	}

	c.JSON(http.StatusOK, secretReadinessJSON{Secrets: out, Total: len(out), Missing: missing})
}

// GetResourceDocument handles GET /v1/provisioning/repositories/:id/resources/:resourceId/document.
// @Summary      Get a provisioned resource's signed document
// @Description  Returns the signed config the device currently holds, as text. This is the document this repository's manifest produced, not a rebuild.
// @Tags         provisioning
// @Produce      text/plain
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id          path  string  true  "repository ID"
// @Param        resourceId  path  string  true  "resource ID"
// @Success      200  {string}  string  "signed config"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id}/resources/{resourceId}/document [get]
func (h *SyncHandler) GetResourceDocument(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	resource, err := h.resources.GetByID(c.Request.Context(), repo.ID, c.Param("resourceId"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	if h.documents == nil {
		h.presenter.NotImplemented(c, "the document reader is not configured")

		return
	}

	// An org resource has no device and no signed document. Answering "the device has no
	// config" for one would be a misleading 404; the request names a resource that cannot
	// have a document at all.
	if resource.Kind != "" && resource.Kind != domain.KindDevice {
		h.presenter.BadRequest(c, "this resource is not a device manifest, so it has no signed document")

		return
	}

	document, _, ok, err := h.documents.CurrentDocument(c.Request.Context(), repo.OrgID, resource.DeviceID)
	if err != nil {
		h.presenter.InternalError(c, "failed to read the signed document")

		return
	}
	if !ok {
		h.presenter.NotFound(c, "the device has no config")

		return
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(document))
}

// InstanceSummary reports what the boot seed is configured to do and what the
// GitOps side currently holds.
//
// It deliberately reports only facts the server can prove: whether PROVISIONING_FILE
// is set and where, plus live counts. The boot seed applies once at startup and keeps
// no per-kind counters, so inventing "last applied" counts here would be a guess.
type InstanceSummary struct {
	// BootSeedSet reports whether a boot seed file is configured. The path itself is not
	// returned: it is server filesystem layout, and a provisioning.read holder has no use
	// for it — knowing where the file lives only helps an attacker who already has a write
	// primitive.
	BootSeedSet  bool `json:"boot_seed_configured"`
	Repositories int  `json:"repositories"`
	Resources    int  `json:"resources"`
	Connections  int  `json:"connections"`
}

// InstanceProvisioning handles GET /v1/provisioning/instance.
// @Summary      Instance provisioning summary
// @Description  Reports whether a boot provisioning file is configured and how many GitOps repositories, resources, and connections exist.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  provisioninghandler.InstanceSummary  "summary"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Router       /provisioning/instance [get]
func (h *SyncHandler) InstanceProvisioning(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	ctx := c.Request.Context()

	repos, err := h.repositories.List(ctx, orgID)
	if err != nil {
		h.presenter.InternalError(c, "failed to list repositories")

		return
	}

	summary := InstanceSummary{Repositories: len(repos)}
	summary.BootSeedSet = os.Getenv("PROVISIONING_FILE") != ""

	if h.counters != nil {
		resources, connections, err := h.counters.Counts(ctx, orgID)
		if err != nil {
			h.presenter.InternalError(c, "failed to count provisioning rows")

			return
		}
		summary.Resources = resources
		summary.Connections = connections
	}

	c.JSON(http.StatusOK, summary)
}

// Counters reports the per-org totals the instance summary needs.
type Counters interface {
	Counts(ctx context.Context, orgID string) (resources, connections int, err error)
}

// SetCounters injects the totals source for the instance summary.
func (h *SyncHandler) SetCounters(c Counters) { h.counters = c }

// eventJSON is one audited provisioning action.
type eventJSON struct {
	ID           string `json:"id"`
	ActorKind    string `json:"actor_kind"`
	ActorID      string `json:"actor_id"`
	ActorLabel   string `json:"actor_label,omitempty"`
	Type         string `json:"type"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id,omitempty"`
	Result       string `json:"result"`
	Detail       string `json:"detail,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

// eventListJSON is a page of events.
type eventListJSON struct {
	Events []eventJSON `json:"events"`
	Total  int         `json:"total"`
}

// SetEventRepository injects the audit trail behind the events routes.
func (h *SyncHandler) SetEventRepository(events domain.EventRepository) { h.events = events }

// ListEvents handles GET /v1/provisioning/events.
// @Summary      List provisioning events
// @Description  Lists who caused each provisioning action, distinguishing a person, a service-account token, the scheduler, and the server itself.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        limit  query  int  false  "how many events to return"
// @Success      200  {object}  provisioninghandler.eventListJSON  "events"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Router       /provisioning/events [get]
func (h *SyncHandler) ListEvents(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	if h.events == nil {
		h.presenter.OK(c, eventListJSON{Events: []eventJSON{}})

		return
	}

	events, err := h.events.ListForOrg(c.Request.Context(), orgID, jobLimit(c))
	if err != nil {
		h.presenter.InternalError(c, "failed to list provisioning events")

		return
	}

	c.JSON(http.StatusOK, eventListJSON{Events: toEventJSON(events), Total: len(events)})
}

// ListResourceEvents handles GET /v1/provisioning/repositories/:id/events.
// @Summary      List a repository's provisioning events
// @Description  Lists who caused each action against this repository, newest first.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id     path   string  true  "repository ID"
// @Param        limit  query  int     false  "how many events to return"
// @Success      200  {object}  provisioninghandler.eventListJSON  "events"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id}/events [get]
func (h *SyncHandler) ListResourceEvents(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	if h.events == nil {
		h.presenter.OK(c, eventListJSON{Events: []eventJSON{}})

		return
	}

	events, err := h.events.ListForResource(c.Request.Context(), orgID, "repository", repo.ID, jobLimit(c))
	if err != nil {
		h.presenter.InternalError(c, "failed to list repository events")

		return
	}

	c.JSON(http.StatusOK, eventListJSON{Events: toEventJSON(events), Total: len(events)})
}

// toEventJSON renders events for the wire.
func toEventJSON(events []*domain.ProvisioningEvent) []eventJSON {
	out := make([]eventJSON, 0, len(events))

	for _, e := range events {
		out = append(out, eventJSON{
			ID:           e.ID,
			ActorKind:    string(e.Actor.Kind),
			ActorID:      e.Actor.ID,
			ActorLabel:   e.Actor.Label,
			Type:         string(e.Type),
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			Result:       e.Result,
			Detail:       e.Detail,
			CreatedAt:    e.CreatedAt.UnixMilli(),
		})
	}

	return out
}

// GetHealth handles GET /v1/provisioning/repositories/:id/health.
// @Summary      Repository sync health over time
// @Description  Returns the average and last sync duration, the current failure streak, and a timeline of recent completed runs.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id     path   string  true  "repository ID"
// @Param        limit  query  int     false  "how many recent runs to include"
// @Success      200  {object}  domain.RepositoryHealth  "health"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id}/health [get]
func (h *SyncHandler) GetHealth(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	health, err := h.jobs.Health(c.Request.Context(), repo.ID, jobLimit(c))
	if err != nil {
		h.presenter.InternalError(c, "failed to read sync health")

		return
	}

	c.JSON(http.StatusOK, health)
}

// discoveryItemJSON is one manifest in the discovery view.
type discoveryItemJSON struct {
	Path string `json:"path"`
	// Kind lets the wizard group step 3 by kind rather than assuming every item is a device.
	Kind     string `json:"kind"`
	DeviceID string `json:"device_id"`
	Action   string `json:"action"`
	Message  string `json:"message,omitempty"`
	Managed  bool   `json:"managed"`
}

// discoveryJSON is what a repository contains, classified against what it owns.
type discoveryJSON struct {
	Items     []discoveryItemJSON      `json:"items"`
	Unmanaged []svc.DeviceDir          `json:"unmanaged"`
	OrgFiles  []svc.OrgResourceSummary `json:"org_files"`
	Total     int                      `json:"total"`
}

// Discover handles GET /v1/provisioning/repositories/:id/discovery.
// @Summary      Discover a repository's contents
// @Description  Lists every manifest the repository declares with the action a sync would take, plus the device directories it does not manage. Nothing is applied.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "repository ID"
// @Success      200  {object}  provisioninghandler.discoveryJSON  "discovery"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      422  {object}  openapi.ErrorResponse  "remote not reachable"
// @Router       /provisioning/repositories/{id}/discovery [get]
func (h *SyncHandler) Discover(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	discovery, err := h.sync.Discover(c.Request.Context(), repo)
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	items := make([]discoveryItemJSON, 0, len(discovery.Items))
	for _, item := range discovery.Items {
		items = append(items, toDiscoveryItemJSON(item))
	}

	c.JSON(http.StatusOK, discoveryJSON{
		Items:     items,
		Unmanaged: discovery.Unmanaged,
		OrgFiles:  discovery.OrgFiles,
		Total:     len(items) + len(discovery.OrgFiles),
	})
}

// toDiscoveryItemJSON renders one plan item. Managed reports whether the device is
// already owned by this repository, which is what step 3 of the wizard asks.
func toDiscoveryItemJSON(item *svc.PlanItem) discoveryItemJSON {
	out := discoveryItemJSON{
		// Every item on the device plan is a device manifest; org files are reported in
		// org_files, which carry their own kind.
		Kind:    string(svc.KindDevice),
		Action:  string(item.Action),
		Message: item.Message,
	}

	if item.Manifest != nil {
		out.Path = item.Manifest.Path
		out.DeviceID = item.Manifest.DeviceID
	}
	if item.Resource != nil {
		out.Path = item.Resource.Path
		out.DeviceID = item.Resource.DeviceID
		out.Managed = true
	}

	return out
}

// ListResources handles GET /v1/provisioning/repositories/:id/resources.
// @Summary      List provisioned resources
// @Description  Lists the manifests a repository owns, with each one's drift state.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "repository ID"
// @Success      200  {object}  provisioninghandler.resourceListJSON  "resources"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id}/resources [get]
func (h *SyncHandler) ListResources(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	resources, err := h.resources.ListByRepo(c.Request.Context(), repo.ID)
	if err != nil {
		h.presenter.InternalError(c, "failed to list resources")

		return
	}

	out := make([]resourceJSON, len(resources))
	for i, r := range resources {
		out[i] = toResourceJSON(r)
	}

	c.JSON(http.StatusOK, resourceListJSON{Resources: out, Total: len(out)})
}

// GetJob handles GET /v1/provisioning/repositories/:id/jobs/:jobId.
// @Summary      Get a sync job
// @Description  Returns one sync run with its per-manifest results.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id     path  string  true  "repository ID"
// @Param        jobId  path  string  true  "job ID"
// @Success      200  {object}  provisioninghandler.jobJSON  "job"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id}/jobs/{jobId} [get]
func (h *SyncHandler) GetJob(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	job, err := h.jobs.GetJob(c.Request.Context(), repo.ID, c.Param("jobId"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	results, err := h.jobs.ListResults(c.Request.Context(), job.ID)
	if err != nil {
		h.presenter.InternalError(c, "failed to read job results")

		return
	}

	c.JSON(http.StatusOK, toJobJSON(job, results))
}

// logSync records a non-fatal sync-handler problem. The sync itself succeeded, so this
// is logged rather than turned into an error the caller would read as a failed sync.
func (h *SyncHandler) logSync(msg, jobID string, err error) {
	logger := h.log
	if logger == nil {
		logger = slog.Default()
	}

	logger.Warn("provisioning: "+msg, "job", jobID, "err", err)
}

// writeSyncError maps a sync failure to its response.
func (h *SyncHandler) writeSyncError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound):
		h.presenter.NotFound(c, "repository not found")
	case errors.Is(err, domain.ErrJobNotFound):
		h.presenter.NotFound(c, "job not found")
	case errors.Is(err, domain.ErrSyncInProgress):
		h.presenter.Conflict(c, err.Error())
	case errors.Is(err, svc.ErrSyncDisabled):
		h.presenter.Conflict(c, err.Error())
	case errors.Is(err, svc.ErrNotADeviceResource):
		// The request is well-formed but names a resource this endpoint cannot act on.
		h.presenter.BadRequest(c, err.Error())
	case errors.Is(err, domain.ErrDeviceOwned):
		// A device claimed by another repository, or the same manifest declared twice.
		// The message names the owner, so a conflict is the honest status.
		h.presenter.Conflict(c, err.Error())
	case errors.Is(err, svc.ErrFetchFailed):
		// The remote could not be reached with the stored credential. The request was
		// well-formed, so it is unprocessable rather than a syntax error.
		h.presenter.UnprocessableEntity(c, err.Error())
	default:
		h.presenter.InternalError(c, "provisioning sync failed")
	}
}

// boolQuery parses a boolean query parameter. An absent parameter is false; a present one
// must be a recognisable boolean, so a typo cannot silently mean the opposite of what was
// asked. strconv.ParseBool accepts 1/0/t/f/true/false/TRUE/FALSE.
func boolQuery(c *gin.Context, name string) (value, ok bool) {
	raw := c.Query(name)
	if raw == "" {
		return false, true
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}

	return parsed, true
}

// jobLimit reads a bounded limit for a job listing.
func jobLimit(c *gin.Context) int {
	n, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || n <= 0 || n > 100 {
		return 20
	}

	return n
}

// ListJobs handles GET /v1/provisioning/repositories/:id/jobs.
// @Summary      List sync jobs
// @Description  Returns a repository's recent sync runs, newest first.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id     path   string  true  "repository ID"
// @Param        limit  query  int     false  "maximum jobs to return"
// @Success      200  {object}  object  "jobs"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Router       /provisioning/repositories/{id}/jobs [get]
func (h *SyncHandler) ListJobs(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeSyncError(c, err)

		return
	}

	jobs, err := h.jobs.ListJobs(c.Request.Context(), repo.ID, jobLimit(c))
	if err != nil {
		h.presenter.InternalError(c, "failed to list jobs")

		return
	}

	out := make([]jobJSON, len(jobs))
	for i, j := range jobs {
		out[i] = toJobJSON(j, nil)
	}

	c.JSON(http.StatusOK, gin.H{"jobs": out, "total": len(out)})
}
