package provisioninghandler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// BrowserHandler serves the repository browse routes: tree, file and history.
//
// It is read-only and separate from the edit routes on purpose. Reading a repository needs
// only `provisioning.read`, and it must work on a read-only connection: an operator who
// cannot push should still see what is in the repository. The write path stays where it was,
// on the resource's preview and push endpoints, which also carry the base-commit conflict
// check.
type BrowserHandler struct {
	browser      svc.RepoBrowser
	repositories *svc.RepositoryService
	connections  svc.ConnectionStore
	presenter    *response.Presenter
}

// NewBrowserHandler builds a BrowserHandler.
func NewBrowserHandler(
	browser svc.RepoBrowser,
	repositories *svc.RepositoryService,
	connections svc.ConnectionStore,
	presenter *response.Presenter,
) *BrowserHandler {
	return &BrowserHandler{
		browser:      browser,
		repositories: repositories,
		connections:  connections,
		presenter:    presenter,
	}
}

type treeEntryJSON struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Size int64  `json:"size"`
}

type treeJSON struct {
	// Dir is the directory these entries belong to; empty means the repository root.
	Dir     string          `json:"dir"`
	Entries []treeEntryJSON `json:"entries"`
	Total   int             `json:"total"`
}

type fileJSON struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Commit    string `json:"commit"`
	Message   string `json:"message,omitempty"`
	Author    string `json:"author,omitempty"`
	When      int64  `json:"when,omitempty"`
	Truncated bool   `json:"truncated"`
}

type commitJSON struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	When    int64  `json:"when"`
}

type historyJSON struct {
	Commits []commitJSON `json:"commits"`
	Total   int          `json:"total"`
	// Depth is how much history the read covered, so a client can say "the last N" rather
	// than implying the full log.
	Depth int `json:"depth"`
}

// Tree handles GET /v1/provisioning/repositories/:id/tree.
// @Summary      List a repository's files
// @Description  Lists one directory of the repository at its branch. An empty dir means the root. Reads only; works on a read-only connection.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true   "Organization ID"
// @Param        id    path   string  true   "repository ID"
// @Param        dir   query  string  false  "directory, relative to the repository root"
// @Success      200  {object}  provisioninghandler.treeJSON  "entries"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "repository or path not found"
// @Router       /provisioning/repositories/{id}/tree [get]
func (h *BrowserHandler) Tree(c *gin.Context) {
	repo, token, ok := h.resolve(c)
	if !ok {
		return
	}

	entries, err := h.browser.ListTree(c.Request.Context(), repo, c.Query("dir"), token)
	if err != nil {
		h.writeBrowseError(c, err)

		return
	}

	out := make([]treeEntryJSON, len(entries))
	for i, entry := range entries {
		out[i] = treeEntryJSON{Path: entry.Path, Name: entry.Name, Kind: entry.Kind, Size: entry.Size}
	}

	c.JSON(http.StatusOK, treeJSON{Dir: strings.Trim(c.Query("dir"), "/"), Entries: out, Total: len(out)})
}

// File handles GET /v1/provisioning/repositories/:id/file.
// @Summary      Read one repository file
// @Description  Returns a file's content and the commit it came from, so an edit can be based on it.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true   "Organization ID"
// @Param        id    path   string  true   "repository ID"
// @Param        path  query  string  true   "file path, relative to the repository root"
// @Success      200  {object}  provisioninghandler.fileJSON  "file"
// @Failure      400  {object}  openapi.ErrorResponse  "path required or invalid"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "repository or file not found"
// @Router       /provisioning/repositories/{id}/file [get]
func (h *BrowserHandler) File(c *gin.Context) {
	path := c.Query("path")
	if strings.TrimSpace(path) == "" {
		h.presenter.BadRequest(c, "path is required")

		return
	}

	repo, token, ok := h.resolve(c)
	if !ok {
		return
	}

	view, err := h.browser.ReadFileAt(c.Request.Context(), repo, path, token)
	if err != nil {
		h.writeBrowseError(c, err)

		return
	}

	out := fileJSON{
		Path:      view.Path,
		Content:   string(view.Content),
		Commit:    view.Commit,
		Message:   view.Message,
		Author:    view.Author,
		Truncated: view.Truncated,
	}
	if !view.When.IsZero() {
		out.When = view.When.UnixMilli()
	}

	c.JSON(http.StatusOK, out)
}

// History handles GET /v1/provisioning/repositories/:id/history.
// @Summary      List a repository's recent commits
// @Description  Returns recent commits on the repository's branch, newest first. The window is bounded, so a large repository is not cloned in full to answer it.
// @Tags         provisioning
// @Produce      json
// @Param        X-Organization-ID  header  string  true   "Organization ID"
// @Param        id     path   string  true   "repository ID"
// @Param        limit  query  int     false  "how many commits to return"
// @Success      200  {object}  provisioninghandler.historyJSON  "commits"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "repository not found"
// @Router       /provisioning/repositories/{id}/history [get]
func (h *BrowserHandler) History(c *gin.Context) {
	repo, token, ok := h.resolve(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	commits, err := h.browser.History(c.Request.Context(), repo, limit, token)
	if err != nil {
		h.writeBrowseError(c, err)

		return
	}

	out := make([]commitJSON, len(commits))
	for i, commit := range commits {
		out[i] = commitJSON{SHA: commit.SHA, Message: commit.Message, Author: commit.Author, When: commit.When.UnixMilli()}
	}

	c.JSON(http.StatusOK, historyJSON{Commits: out, Total: len(out), Depth: 50})
}

// resolve loads the repository and its read token.
//
// The token comes from the repository's connection, so a browse works with the same
// credential a sync uses — the caller never supplies one.
func (h *BrowserHandler) resolve(c *gin.Context) (*domain.Repository, redaction.Secret, bool) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return nil, "", false
	}

	repo, err := h.repositories.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeBrowseError(c, err)

		return nil, "", false
	}

	token, err := h.connections.ConnectionToken(c.Request.Context(), orgID, repo.ConnectionID)
	if err != nil {
		h.presenter.InternalError(c, "could not read the repository credential")

		return nil, "", false
	}

	return repo, token, true
}

func (h *BrowserHandler) writeBrowseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrRepositoryNotFound):
		h.presenter.NotFound(c, "repository not found")
	case errors.Is(err, svc.ErrPathNotFound):
		h.presenter.NotFound(c, "no such file or directory in the repository")
	case errors.Is(err, svc.ErrPathOutsideRepo):
		// The path is well-formed but names something outside the repository, which is a
		// refusal rather than a not-found: it is not "missing", it is "not allowed".
		h.presenter.BadRequest(c, "path must stay inside the repository")
	case errors.Is(err, svc.ErrFetchFailed):
		h.presenter.UnprocessableEntity(c, err.Error())
	default:
		h.presenter.InternalError(c, "could not read the repository")
	}
}

// compile-time guard: the browser interface is what this handler needs, so a change to it
// fails here rather than at wiring time.
var _ svc.RepoBrowser = (*svc.GitBrowser)(nil)
