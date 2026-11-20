package provisioninghandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
)

var _ openapi.SuccessResult

// StatsSource reports a repository's per-state resource counts.
type StatsSource interface {
	StatsByRepo(ctx context.Context, repoID string) (*domain.ResourceStats, error)
}

// RepositoryHandler serves the repository routes.
type RepositoryHandler struct {
	service   *svc.RepositoryService
	stats     StatsSource
	presenter *response.Presenter
}

// NewRepositoryHandler builds a RepositoryHandler.
func NewRepositoryHandler(service *svc.RepositoryService, presenter *response.Presenter) *RepositoryHandler {
	return &RepositoryHandler{service: service, presenter: presenter}
}

// SetStatsSource injects the per-state counter behind the stats[] field. It is a
// setter because the store is wired alongside the handler, not before it.
func (h *RepositoryHandler) SetStatsSource(s StatsSource) { h.stats = s }

// statsFor returns a repository's counts, or an empty set when no source is wired.
// A nil source must not fail the list: the repositories themselves are still useful.
func (h *RepositoryHandler) statsFor(ctx context.Context, repoID string) domain.ResourceStats {
	if h.stats == nil {
		return domain.ResourceStats{}
	}

	stats, err := h.stats.StatsByRepo(ctx, repoID)
	if err != nil {
		return domain.ResourceStats{}
	}

	return *stats
}

type repositoryJSON struct {
	LastSyncAt      *int64               `json:"last_sync_at,omitempty"`
	Path            string               `json:"path"`
	SyncState       string               `json:"sync_state"`
	ConnectionID    string               `json:"connection_id"`
	LastError       string               `json:"last_error,omitempty"`
	Title           string               `json:"title,omitempty"`
	Description     string               `json:"description,omitempty"`
	Type            string               `json:"type"`
	URL             string               `json:"url"`
	ID              string               `json:"id"`
	Branch          string               `json:"branch"`
	Name            string               `json:"name"`
	SyncTarget      string               `json:"sync_target"`
	Exclude         []string             `json:"exclude"`
	Include         []string             `json:"include"`
	Stats           domain.ResourceStats `json:"stats"`
	IntervalSeconds int                  `json:"sync_interval_seconds"`
	CreatedAt       int64                `json:"created_at"`
	UpdatedAt       int64                `json:"updated_at"`
	Enabled         bool                 `json:"enabled"`
}

type repositoryListJSON struct {
	Repositories []repositoryJSON `json:"repositories"`
	Total        int              `json:"total"`
}

type repositoryRequest struct {
	Enabled      *bool  `json:"enabled"`
	Branch       string `json:"branch"`
	Title        string `json:"title"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	Path         string `json:"path"`
	ConnectionID string `json:"connection_id"`
	SyncTarget   string `json:"sync_target"`
	// WebhookSecret configures the HMAC secret a provider signs pushes with. It is
	// write-only: no response echoes it back, and omitting it leaves the stored one alone.
	WebhookSecret string `json:"webhook_secret"`
	// Include and Exclude are the repository's scope patterns. A nil means "not
	// supplied", so a PATCH that only renames cannot silently clear them.
	Include         []string `json:"include"`
	Exclude         []string `json:"exclude"`
	IntervalSeconds int      `json:"sync_interval_seconds"`
	// ClearWebhook removes the stored webhook secret. Without it a user could replace a
	// secret but never disable one, because an empty webhook_secret means "unchanged".
	ClearWebhook bool `json:"clear_webhook"`
}

func (r repositoryRequest) input() svc.RepositoryInput {
	return svc.RepositoryInput{
		Name:            r.Name,
		Title:           r.Title,
		Description:     r.Description,
		URL:             r.URL,
		Branch:          r.Branch,
		Path:            r.Path,
		Include:         r.Include,
		Exclude:         r.Exclude,
		ConnectionID:    r.ConnectionID,
		SyncTarget:      domain.SyncTarget(r.SyncTarget),
		IntervalSeconds: r.IntervalSeconds,
		Enabled:         r.Enabled,
		WebhookSecret:   redaction.NewSecret(r.WebhookSecret),
		ClearWebhook:    r.ClearWebhook,
	}
}

func toRepositoryJSON(r *domain.Repository, stats domain.ResourceStats) repositoryJSON {
	out := repositoryJSON{
		ID:              r.ID,
		ConnectionID:    r.ConnectionID,
		Name:            r.Name,
		Title:           r.Title,
		Description:     r.Description,
		Type:            string(r.Type),
		URL:             r.URL,
		Branch:          r.Branch,
		Path:            r.Path,
		Include:         r.Include,
		Exclude:         r.Exclude,
		SyncTarget:      string(r.SyncTarget),
		SyncState:       string(r.SyncState),
		LastError:       r.LastError,
		Stats:           stats,
		IntervalSeconds: r.IntervalSeconds,
		Enabled:         r.Enabled,
		CreatedAt:       r.CreatedAt.UnixMilli(),
		UpdatedAt:       r.UpdatedAt.UnixMilli(),
	}
	if r.LastSyncAt != nil {
		ms := r.LastSyncAt.UnixMilli()
		out.LastSyncAt = &ms
	}

	return out
}

// List handles GET /v1/provisioning/repositories.
// @Summary      List provisioning repositories
// @Description  Lists an organisation's repositories with their sync state.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  provisioninghandler.repositoryListJSON  "repositories"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Router       /provisioning/repositories [get]
func (h *RepositoryHandler) List(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repos, err := h.service.List(c.Request.Context(), orgID)
	if err != nil {
		h.presenter.InternalError(c, "failed to list repositories")

		return
	}

	out := make([]repositoryJSON, len(repos))
	for i, r := range repos {
		out[i] = toRepositoryJSON(r, h.statsFor(c.Request.Context(), r.ID))
	}

	c.JSON(http.StatusOK, repositoryListJSON{Repositories: out, Total: len(out)})
}

// Get handles GET /v1/provisioning/repositories/:id.
// @Summary      Get provisioning repository
// @Description  Returns one repository with its sync state.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "repository ID"
// @Success      200  {object}  provisioninghandler.repositoryJSON  "repository"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id} [get]
func (h *RepositoryHandler) Get(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	repo, err := h.service.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeRepositoryError(c, err)

		return
	}

	c.JSON(http.StatusOK, toRepositoryJSON(repo, h.statsFor(c.Request.Context(), repo.ID)))
}

// Create handles POST /v1/provisioning/repositories.
// @Summary      Create provisioning repository
// @Description  Validates the remote and branch against the provider before storing. An unreachable remote is rejected rather than stored as a permanently failing row.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        body  body  provisioninghandler.repositoryRequest  true  "repository"
// @Success      201  {object}  provisioninghandler.repositoryJSON  "created"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      422  {object}  openapi.ErrorResponse  "remote not reachable"
// @Router       /provisioning/repositories [post]
func (h *RepositoryHandler) Create(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	var req repositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid request body")

		return
	}

	repo, err := h.service.Create(c.Request.Context(), orgID, req.input())
	if err != nil {
		h.writeRepositoryError(c, err)

		return
	}

	c.JSON(http.StatusCreated, toRepositoryJSON(repo, h.statsFor(c.Request.Context(), repo.ID)))
}

// Update handles PATCH /v1/provisioning/repositories/:id.
// @Summary      Update provisioning repository
// @Description  Changes the mutable fields. A URL, branch, or connection change is re-verified against the provider.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id    path  string  true  "repository ID"
// @Param        body  body  provisioninghandler.repositoryRequest  true  "fields to change"
// @Success      200  {object}  provisioninghandler.repositoryJSON  "updated"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      422  {object}  openapi.ErrorResponse  "remote not reachable"
// @Router       /provisioning/repositories/{id} [patch]
func (h *RepositoryHandler) Update(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	var req repositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid request body")

		return
	}

	repo, err := h.service.Update(c.Request.Context(), orgID, c.Param("id"), req.input())
	if err != nil {
		h.writeRepositoryError(c, err)

		return
	}

	c.JSON(http.StatusOK, toRepositoryJSON(repo, h.statsFor(c.Request.Context(), repo.ID)))
}

// Delete handles DELETE /v1/provisioning/repositories/:id.
// @Summary      Delete provisioning repository
// @Description  Removes a repository. Its resources and job history are removed with it; devices keep the config they already run.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "repository ID"
// @Param        keep_resources  query  bool  false  "release devices instead of deleting their ownership rows"
// @Success      200  {object}  openapi.SuccessResult  "deleted"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/repositories/{id} [delete]
func (h *RepositoryHandler) Delete(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	keepResources, ok := boolQuery(c, "keep_resources")
	if !ok {
		h.presenter.BadRequest(c, "keep_resources must be true or false")

		return
	}

	if err := h.service.Delete(c.Request.Context(), orgID, c.Param("id"), keepResources, actorFrom(c)); err != nil {
		h.writeRepositoryError(c, err)

		return
	}

	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: "repository deleted"})
}

func (h *RepositoryHandler) writeRepositoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound):
		h.presenter.NotFound(c, "repository not found")
	case errors.Is(err, svc.ErrRemoteUnreachable):
		h.presenter.UnprocessableEntity(c, err.Error())
	case errors.Is(err, svc.ErrInvalidRepository):
		h.presenter.BadRequest(c, err.Error())
	default:
		h.presenter.InternalError(c, "provisioning repository operation failed")
	}
}
