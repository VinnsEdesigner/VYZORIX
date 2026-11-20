package provisioning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

var (
	// ErrEditDisabled is returned when a repository's connection has no push scope. Editing
	// a manifest needs write access, so a read-only connection is refused with a clear
	// reason rather than a provider 403.
	ErrEditDisabled = errors.New("editing a provisioned manifest needs write access")
)

// PushMode is how an edit reaches the repository.
type PushMode string

const (
	// PushModePR commits to a generated branch and opens a pull request, so a reviewer
	// sees the change before it lands. It is the default: a provisioned manifest can
	// rewrite any device's config, so a change deserves a second pair of eyes.
	PushModePR PushMode = "pr"
	// PushModeDirect commits straight to the repository's branch. It exists for a
	// single-maintainer repository where a pull request is ceremony.
	PushModeDirect PushMode = "direct"
)

// Valid reports whether m is a known mode.
func (m PushMode) Valid() bool {
	switch m {
	case PushModePR, PushModeDirect:
		return true
	}

	return false
}

// EditPreview is the diff a push would make, computed without writing anything.
type EditPreview struct {
	// Diff is the unified diff, so the user sees exactly what would change.
	Diff string
	// BaseCommit is the commit the edit is based on; the push refuses if the remote has
	// moved past it.
	BaseCommit string
	// Path is the manifest path.
	Path string
	// Current is the file's content now.
	Current []byte
	// Proposed is the content that would be committed.
	Proposed []byte
	// Changed is false when the proposed content already matches.
	Changed bool
}

// EditResult reports a completed push.
type EditResult struct {
	// Branch is the branch that was pushed.
	Branch string
	// Commit is the new commit, when one was made.
	Commit string
	// PullRequestURL is set in PR mode, when the provider accepted the request.
	PullRequestURL string
	// PullRequestError is set in PR mode when the branch was pushed but the request could
	// not be opened. The push succeeded, so this is reported rather than returned.
	PullRequestError string
	// Changed is false when the content already matched and nothing was pushed.
	Changed bool
}

// EditRequest is one manifest edit.
type EditRequest struct {
	Repo  *domain.Repository
	Actor domain.Actor
	// Path is the manifest path, e.g. devices/x/vyzorix.toml.
	Path string
	// BaseCommit is what the preview was based on. It is required for both modes: even a
	// direct push must detect that the remote moved since the diff was approved, because
	// the approved diff is then not the change that would land.
	BaseCommit string
	// Mode is how the change reaches the repository.
	Mode PushMode
	// CommitMessage overrides the default template.
	CommitMessage string
	// Content is the full new file content.
	Content []byte
}

// PullRequest is one pull request to open.
type PullRequest struct {
	Repo  *domain.Repository
	Head  string
	Base  string
	Title string
	Body  string
}

// PullRequester opens a pull request on a provider.
//
// It is an interface because a provider that cannot be reached must not lose the push: the
// branch is already on the remote, and the caller is told the request could not be opened
// rather than being told the commit failed.
type PullRequester interface {
	OpenPullRequest(ctx context.Context, req PullRequest, token redaction.Secret) (string, error)
}

// PushBackService edits a manifest and commits it back.
//
// Preview and Apply share the same read-and-verify path: Preview stops before the write,
// Apply continues. That is what makes the preview trustworthy — it is not a separate
// computation that can disagree with what the push then does.
type PushBackService struct {
	pusher      Pusher
	connections ConnectionStore
	prs         PullRequester
	auditor     *Auditor
	log         *slog.Logger
	now         func() time.Time
}

// NewPushBackService builds the service.
func NewPushBackService(
	pusher Pusher,
	connections ConnectionStore,
	prs PullRequester,
	auditor *Auditor,
	log *slog.Logger,
) *PushBackService {
	if log == nil {
		log = slog.Default()
	}

	return &PushBackService{
		pusher: pusher, connections: connections, prs: prs,
		auditor: auditor, log: log, now: time.Now,
	}
}

// Preview computes the diff an edit would produce, without writing anything.
func (s *PushBackService) Preview(ctx context.Context, req EditRequest) (*EditPreview, error) {
	token, err := s.writeToken(ctx, req.Repo)
	if err != nil {
		return nil, err
	}

	current, base, err := s.pusher.ReadFile(ctx, req.Repo, req.Path, token)
	if err != nil {
		return nil, err
	}

	return &EditPreview{
		Path:       req.Path,
		Current:    current,
		Proposed:   req.Content,
		Diff:       unifiedDiff(req.Path, current, req.Content),
		BaseCommit: base,
		Changed:    string(current) != string(req.Content),
	}, nil
}

// ErrBaseCommitRequired is returned when a push omits the commit the edit was based on.
//
// The base is what makes a push safe: without it the conflict check cannot run, and a
// concurrent change would be silently overwritten. A caller must preview first, which
// returns the base to send back.
var ErrBaseCommitRequired = errors.New("base_commit is required; preview the edit first")

// Apply commits the edit and pushes it.
func (s *PushBackService) Apply(ctx context.Context, req EditRequest) (*EditResult, error) {
	// The base is mandatory. A push without it cannot detect that the remote moved, which
	// is the one failure a push-back must never produce silently.
	if strings.TrimSpace(req.BaseCommit) == "" {
		return nil, ErrBaseCommitRequired
	}

	token, err := s.writeToken(ctx, req.Repo)
	if err != nil {
		return nil, err
	}

	branch := req.Repo.Branch
	if req.Mode == PushModePR {
		branch = generatedBranch(req.Repo.Name, s.now())
	}

	pushed, err := s.pusher.Push(ctx, PushRequest{
		Repo:       req.Repo,
		Path:       req.Path,
		Content:    req.Content,
		BaseCommit: req.BaseCommit,
		Branch:     branch,
		Message:    commitMessage(req),
	}, token)
	if err != nil {
		return nil, err
	}

	result := &EditResult{Branch: pushed.Branch, Commit: pushed.Commit, Changed: pushed.Changed}

	if req.Mode == PushModePR && pushed.Changed && s.prs != nil {
		url, prErr := s.prs.OpenPullRequest(ctx, PullRequest{
			Repo:  req.Repo,
			Head:  branch,
			Base:  req.Repo.Branch,
			Title: pullTitle(req),
			Body:  pullBody(req),
		}, token)
		if prErr != nil {
			// The branch is on the remote, so the push succeeded. Reporting the request
			// failure beside it is honest; failing the whole call would tell the user
			// nothing was pushed when something was.
			result.PullRequestError = prErr.Error()
			s.log.Error("provisioning: could not open a pull request",
				"repo", req.Repo.ID, "branch", branch, "err", prErr)
		} else {
			result.PullRequestURL = url
		}
	}

	s.audit(ctx, req, result)

	return result, nil
}

// writeToken returns the connection's token, refusing when it has no push scope.
func (s *PushBackService) writeToken(ctx context.Context, repo *domain.Repository) (redaction.Secret, error) {
	conn, err := s.connections.GetConnection(ctx, repo.OrgID, repo.ConnectionID)
	if err != nil {
		return "", err
	}
	if !conn.WriteEnabled {
		return "", ErrEditDisabled
	}

	return s.connections.ConnectionToken(ctx, repo.OrgID, conn.ID)
}

// audit records the push against the acting user.
func (s *PushBackService) audit(ctx context.Context, req EditRequest, result *EditResult) {
	if s.auditor == nil {
		return
	}

	detail := "edited " + req.Path + " on " + result.Branch
	if !result.Changed {
		detail = "no change to " + req.Path
	}
	// The commit and the pull request are what make the row followable: an audit entry that
	// names only the branch cannot be traced to the change it caused.
	if result.Commit != "" {
		detail += " (commit " + shortSHA(result.Commit) + ")"
	}
	if result.PullRequestURL != "" {
		detail += " (pull request opened)"
	}

	// A pushed branch with no pull request is a partial success, not a success: the change
	// is on the remote but no review exists, which is the opposite of what PR mode promises.
	outcome := "success"
	if result.PullRequestError != "" {
		outcome = "partial"
		detail += ": " + result.PullRequestError
	}

	s.auditor.Audit(ctx, req.Actor, domain.EventDocumentPushed, req.Repo.OrgID,
		"repository", req.Repo.ID, outcome, detail)
}

// generatedBranch names a PR branch uniquely enough that two edits cannot collide, while
// still reading as what it is.
//
// The random suffix is what guarantees uniqueness. A timestamp alone is only accurate to
// the second, so two edits in the same second produced the same branch — and the second
// "checkout -B" reset the branch under the first commit, silently losing it. The slug is
// sanitised because a repository name is free text and git refs reject "~^:?*[\" and
// spaces.
func generatedBranch(repoName string, at time.Time) string {
	return fmt.Sprintf("vyzorix/edit-%s-%s-%s",
		refSlug(repoName), at.UTC().Format("20060102-150405"), uuid.New().String()[:8])
}

// refSlug reduces free text to a git-ref-safe segment.
func refSlug(raw string) string {
	var b strings.Builder

	for _, r := range strings.ToLower(raw) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '.':
			b.WriteByte('-')
		}
	}

	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "repository"
	}

	return slug
}

// commitMessage renders the commit message, defaulting to a template that names what
// changed.
func commitMessage(req EditRequest) string {
	if strings.TrimSpace(req.CommitMessage) != "" {
		return req.CommitMessage
	}

	return fmt.Sprintf("chore(provisioning): edit %s\n\nCommitted by VYZORIX on behalf of the operator.",
		req.Path)
}

// pullTitle is the pull request title.
func pullTitle(req EditRequest) string {
	return "provisioning: edit " + req.Path
}

// pullBody is the pull request body.
func pullBody(req EditRequest) string {
	return fmt.Sprintf(
		"Edits %s through VYZORIX provisioning.\n\n"+
			"Applying this manifest changes the device's config on the next sync.",
		req.Path)
}

// unifiedDiff renders a unified diff between two file versions.
//
// The diff is computed, not asserted: it is the same content the push will write, so a
// user who approves the preview has approved the change.
func unifiedDiff(path string, current, proposed []byte) string {
	edits := myers.ComputeEdits(span.URIFromPath(path), string(current), string(proposed))
	unified := gotextdiff.ToUnified("a/"+path, "b/"+path, string(current), edits)

	return fmt.Sprint(unified)
}
