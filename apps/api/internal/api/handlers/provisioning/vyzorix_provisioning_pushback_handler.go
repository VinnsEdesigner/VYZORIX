package provisioninghandler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// maxDocumentBytes bounds an edited manifest. A manifest is a small TOML document, so a
// large body is a mistake or an attack rather than a legitimate edit.
const maxDocumentBytes = 256 << 10 // 256 KiB

// PushBackHandler serves the manifest edit routes.
type PushBackHandler struct {
	service      *svc.PushBackService
	repositories *svc.RepositoryService
	resources    domain.ResourceRepository
	presenter    *response.Presenter
}

// NewPushBackHandler builds a PushBackHandler.
func NewPushBackHandler(
	service *svc.PushBackService,
	repositories *svc.RepositoryService,
	resources domain.ResourceRepository,
	presenter *response.Presenter,
) *PushBackHandler {
	return &PushBackHandler{
		service:      service,
		repositories: repositories,
		resources:    resources,
		presenter:    presenter,
	}
}

// editRequest is the wire shape for an edit. The content is the full new file.
type editRequest struct {
	Content       string `json:"content"`
	BaseCommit    string `json:"base_commit"`
	Mode          string `json:"mode"`
	CommitMessage string `json:"commit_message"`
}

// previewJSON is the diff a push would make.
type previewJSON struct {
	Path       string `json:"path"`
	Diff       string `json:"diff"`
	BaseCommit string `json:"base_commit"`
	Changed    bool   `json:"changed"`
}

// editResultJSON is what a completed push reports.
type editResultJSON struct {
	Branch           string `json:"branch"`
	Commit           string `json:"commit,omitempty"`
	PullRequestURL   string `json:"pull_request_url,omitempty"`
	PullRequestError string `json:"pull_request_error,omitempty"`
	Changed          bool   `json:"changed"`
}

// Preview handles POST /v1/provisioning/repositories/:id/resources/:resourceId/preview.
// @Summary      Preview an edit to a provisioned manifest
// @Description  Returns the unified diff an edit would produce. Nothing is written.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id          path  string  true  "repository ID"
// @Param        resourceId  path  string  true  "resource ID"
// @Param        body        body  provisioninghandler.editRequest  true  "new content"
// @Success      200  {object}  provisioninghandler.previewJSON  "diff"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid body"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "connection is read-only"
// @Router       /provisioning/repositories/{id}/resources/{resourceId}/preview [post]
func (h *PushBackHandler) Preview(c *gin.Context) {
	req, repo, resource, ok := h.resolve(c)
	if !ok {
		return
	}

	preview, err := h.service.Preview(c.Request.Context(), svc.EditRequest{
		Actor:      actorFrom(c),
		Repo:       repo,
		Path:       resource.Path,
		Content:    []byte(req.Content),
		BaseCommit: req.BaseCommit,
	})
	if err != nil {
		h.writeEditError(c, err)

		return
	}

	c.JSON(http.StatusOK, previewJSON{
		Path:       preview.Path,
		Diff:       preview.Diff,
		BaseCommit: preview.BaseCommit,
		Changed:    preview.Changed,
	})
}

// Apply handles POST /v1/provisioning/repositories/:id/resources/:resourceId/push.
// @Summary      Push an edit to a provisioned manifest
// @Description  Commits the new content to a generated branch and opens a pull request (mode pr), or commits straight to the repository's branch (mode direct). Refused when the remote moved since the preview.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id          path  string  true  "repository ID"
// @Param        resourceId  path  string  true  "resource ID"
// @Param        body        body  provisioninghandler.editRequest  true  "new content"
// @Success      202  {object}  provisioninghandler.editResultJSON  "pushed"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid body"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "read-only connection, or the remote moved"
// @Router       /provisioning/repositories/{id}/resources/{resourceId}/push [post]
func (h *PushBackHandler) Apply(c *gin.Context) {
	req, repo, resource, ok := h.resolve(c)
	if !ok {
		return
	}

	mode := svc.PushMode(req.Mode)
	if mode == "" {
		// PR is the default: a provisioned manifest can rewrite any device's config, so a
		// change deserves review unless the caller explicitly asks otherwise.
		mode = svc.PushModePR
	}
	if !mode.Valid() {
		h.presenter.BadRequest(c, "mode must be pr or direct")

		return
	}

	result, err := h.service.Apply(c.Request.Context(), svc.EditRequest{
		Actor:         actorFrom(c),
		Repo:          repo,
		Path:          resource.Path,
		Content:       []byte(req.Content),
		BaseCommit:    req.BaseCommit,
		Mode:          mode,
		CommitMessage: req.CommitMessage,
	})
	if err != nil {
		h.writeEditError(c, err)

		return
	}

	c.JSON(http.StatusAccepted, editResultJSON{
		Branch:           result.Branch,
		Commit:           result.Commit,
		PullRequestURL:   result.PullRequestURL,
		PullRequestError: result.PullRequestError,
		Changed:          result.Changed,
	})
}

// resolve reads the request, the repository, and the resource the edit targets.
func (h *PushBackHandler) resolve(c *gin.Context) (editRequest, *domain.Repository, *domain.Resource, bool) {
	var req editRequest

	// The body is read before any lookup: a malformed request should not cost two database
	// round trips, and the cheapest rejection is the one that happens first.
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid request body")

		return req, nil, nil, false
	}
	if len(req.Content) > maxDocumentBytes {
		h.presenter.BadRequest(c, "content is too large for a manifest")

		return req, nil, nil, false
	}

	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return req, nil, nil, false
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeEditError(c, err)

		return req, nil, nil, false
	}

	resource, err := h.resources.GetByID(c.Request.Context(), repo.ID, c.Param("resourceId"))
	if err != nil {
		h.writeEditError(c, err)

		return req, nil, nil, false
	}

	return req, repo, resource, true
}

// writeEditError maps an edit failure to its response.
func (h *PushBackHandler) writeEditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound), errors.Is(err, domain.ErrResourceNotFound):
		h.presenter.NotFound(c, "not found")
	case errors.Is(err, svc.ErrBaseCommitRequired):
		// The caller skipped the preview. A 400 says what to do, rather than a 409 that
		// implies a conflict that was never checked.
		h.presenter.BadRequest(c, err.Error())
	case errors.Is(err, svc.ErrEditDisabled):
		h.presenter.Conflict(c, err.Error())
	case errors.Is(err, svc.ErrConflict):
		// The remote moved. The user must re-read and re-apply, so the diff they approved
		// is not the one that would land.
		h.presenter.Conflict(c, err.Error())
	case errors.Is(err, svc.ErrPushFailed):
		h.presenter.BadGateway(c, err.Error())
	default:
		h.presenter.InternalError(c, "provisioning edit failed")
	}
}
