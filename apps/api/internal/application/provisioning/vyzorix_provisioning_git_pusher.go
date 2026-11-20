package provisioning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

var (
	// ErrPushFailed is returned when a commit or push cannot complete.
	ErrPushFailed = errors.New("could not push to the repository")
	// ErrConflict is returned when the remote moved while the edit was in progress.
	// The caller must re-read and re-apply rather than force-pushing over someone
	// else's change.
	ErrConflict = errors.New("the remote changed while this edit was in progress")
	// ErrWriteDisabled is returned when a push is attempted on a connection whose token
	// has no push scope. It is refused here rather than left to fail at the provider.
	ErrWriteDisabled = errors.New("this connection is read-only; enable write access to push")
)

// PushRequest is one file change to commit.
type PushRequest struct {
	Repo *domain.Repository
	// Path is the manifest path within the repository, e.g. devices/x/vyzorix.toml.
	Path string
	// BaseCommit is the commit the edit was based on. If the remote has moved past it,
	// the push is refused rather than clobbering a concurrent change.
	BaseCommit string
	// Branch is the branch to commit onto. In PR mode it is a generated branch; in
	// direct mode it is the repository's own branch.
	Branch string
	// Message is the commit message.
	Message string
	// Content is the full new file content.
	Content []byte
}

// PushResult reports what a push did.
type PushResult struct {
	// Commit is the new commit's SHA, when the push created one.
	Commit string
	// Branch is the branch that was pushed.
	Branch string
	// Changed is false when the file already held the requested content, so no commit
	// was made and nothing was pushed.
	Changed bool
}

// Pusher writes a manifest back to its repository. It is an interface so the service can
// be tested without a network or a git binary.
type Pusher interface {
	// Push commits one file and pushes the branch.
	Push(ctx context.Context, req PushRequest, token redaction.Secret) (*PushResult, error)
	// ReadFile returns a file's current content at the repository's branch, so an edit
	// can be based on what is actually there.
	ReadFile(ctx context.Context, repo *domain.Repository, path string, token redaction.Secret) ([]byte, string, error)
}

// GitPusher performs push-back with the same git binary the fetcher uses.
//
// It shares the fetcher's credential-helper environment, so the token reaches git through
// the helper and never through argv. Write access is a separate capability: the caller must
// have verified the connection is write-enabled before reaching here, and a network remote
// with no token is refused rather than pushed anonymously.
type GitPusher struct {
	now     func() time.Time
	bin     string
	timeout time.Duration
}

// binary returns the git executable to run.
func (g *GitPusher) binary() string {
	if g.bin == "" {
		return "git"
	}

	return g.bin
}

// NewGitPusher builds a pusher.
func NewGitPusher() *GitPusher {
	return &GitPusher{bin: "git", timeout: 2 * time.Minute, now: time.Now}
}

// ReadFile reads a file from the repository's branch.
//
// It does a shallow clone and reads the file, which is the same cost the sync pays. The
// alternative — caching the content at sync time — would let an edit be based on a stale
// copy, which is the conflict this feature exists to avoid.
func (g *GitPusher) ReadFile(
	ctx context.Context, repo *domain.Repository, path string, token redaction.Secret,
) ([]byte, string, error) {
	checkout, err := NewGitFetcher().Fetch(ctx, repo, token)
	if err != nil {
		return nil, "", err
	}
	defer checkout.Cleanup()

	commit, err := g.revParse(ctx, checkout.Dir, "HEAD")
	if err != nil {
		return nil, "", err
	}

	full, err := safeJoin(checkout.Dir, path)
	if err != nil {
		return nil, "", err
	}

	content, err := os.ReadFile(full) // #nosec G304 -- full is contained within the checkout by safeJoin.
	if err != nil {
		return nil, "", fmt.Errorf("%w: %s: %v", ErrPushFailed, path, err)
	}

	return content, commit, nil
}

// Push commits one file and pushes the branch.
func (g *GitPusher) Push(ctx context.Context, req PushRequest, token redaction.Secret) (*PushResult, error) {
	// A network remote needs a credential to push. A file:// remote does not, which is why
	// the check is on the scheme rather than on the token alone: refusing a tokenless local
	// remote would make the pusher unusable for the one case that needs no credential, and
	// the connection's write scope is enforced by the service before reaching here.
	if token.IsZero() && !isLocalRemote(req.Repo.URL) {
		return nil, fmt.Errorf("%w: no credential", ErrWriteDisabled)
	}

	timeout := g.timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	checkout, fetchErr := NewGitFetcher().Fetch(ctx, req.Repo, token)
	if fetchErr != nil {
		return nil, fetchErr
	}
	defer checkout.Cleanup()

	// Conflict check: the edit was based on a commit, and the remote must still be at it.
	// A mismatch means someone pushed while the user was editing, and overwriting their
	// change is the one outcome a push-back must never produce silently.
	if req.BaseCommit != "" {
		head, err := g.revParse(ctx, checkout.Dir, "HEAD")
		if err != nil {
			return nil, err
		}
		if !sameCommit(head, req.BaseCommit) {
			return nil, fmt.Errorf("%w: the branch is at %s, the edit was based on %s",
				ErrConflict, shortSHA(head), shortSHA(req.BaseCommit))
		}
	}

	full, err := safeJoin(checkout.Dir, req.Path)
	if err != nil {
		return nil, err
	}

	if err = os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPushFailed, err)
	}
	if err = os.WriteFile(full, req.Content, 0o600); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPushFailed, err)
	}

	// Nothing to do when the content is already what the file holds. A no-op commit
	// would show up in the provider's history as a change that changed nothing.
	if !g.stagedChange(ctx, checkout.Dir, req.Path) {
		return &PushResult{Branch: req.Branch, Changed: false}, nil
	}

	if err = g.run(ctx, checkout.Dir, req.Repo.Type, token, "checkout", "-B", req.Branch); err != nil {
		return nil, err
	}
	if err = g.run(ctx, checkout.Dir, req.Repo.Type, token, "add", "--", req.Path); err != nil {
		return nil, err
	}
	if err = g.run(ctx, checkout.Dir, req.Repo.Type, token,
		"-c", "user.name="+commitAuthorName, "-c", "user.email="+commitAuthorEmail,
		"commit", "-m", req.Message); err != nil {
		return nil, err
	}
	if err = g.run(ctx, checkout.Dir, req.Repo.Type, token, "push", "origin", req.Branch); err != nil {
		return nil, err
	}

	commit, err := g.revParse(ctx, checkout.Dir, "HEAD")
	if err != nil {
		return nil, err
	}

	return &PushResult{Commit: commit, Branch: req.Branch, Changed: true}, nil
}

// safeJoin resolves a repository-relative path inside the checkout, refusing anything
// that escapes it.
//
// The path comes from a resource row, but it is written back to a git working tree: a
// "../" segment would let an edit read or overwrite a file outside the clone, which on
// this server means the env file holding the session secret. filepath.Join cleans ".."
// silently, so the containment has to be checked on the result.
func safeJoin(root, rel string) (string, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))

	relPath, err := filepath.Rel(root, full)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path %q escapes the checkout", ErrPushFailed, rel)
	}

	return full, nil
}

// stagedChange reports whether the working tree differs from HEAD at path.
func (g *GitPusher) stagedChange(ctx context.Context, dir, path string) bool {
	cmd := exec.CommandContext(ctx, g.binary(), "status", "--porcelain", "--", path)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return true
	}

	return strings.TrimSpace(string(out)) != ""
}

// run executes one git command in the checkout, with the credential available to the
// helper and nowhere else.
//
// The connection type decides the username the helper reports, so a GitLab token is not
// offered as a GitHub one.
func (g *GitPusher) run(
	ctx context.Context, dir string, kind domain.ConnectionType, token redaction.Secret, args ...string,
) error {
	full := append([]string{}, credentialHelperConfig()...)
	full = append(full, "-c", "core.askPass=")
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, g.binary(), full...)
	cmd.Dir = dir
	cmd.Env = gitEnv(kind, token.Reveal())

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", ErrPushFailed, redactOutput(string(out), token.Reveal()))
	}

	return nil
}

// revParse reads a revision's SHA in the checkout.
func (g *GitPusher) revParse(ctx context.Context, dir, rev string) (string, error) {
	cmd := exec.CommandContext(ctx, g.binary(), "rev-parse", rev)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: could not read %s", ErrPushFailed, rev)
	}

	return strings.TrimSpace(string(out)), nil
}

// isLocalRemote reports whether a URL is a local path or file:// remote, which needs no
// credential.
func isLocalRemote(raw string) bool {
	return strings.HasPrefix(raw, "file://") || strings.HasPrefix(raw, "/")
}

// sameCommit reports whether two SHAs name the same commit, allowing an abbreviated
// form.
//
// A client may send the short SHA it was shown rather than the full one, and git accepts
// abbreviations everywhere. Requiring an exact 40-character match would report a conflict
// when nothing had changed, which is a false alarm the user cannot diagnose.
func sameCommit(head, base string) bool {
	head = strings.ToLower(strings.TrimSpace(head))
	base = strings.ToLower(strings.TrimSpace(base))

	if head == "" || base == "" {
		return false
	}

	return strings.HasPrefix(head, base) || strings.HasPrefix(base, head)
}

// shortSHA abbreviates a SHA for a message, so a conflict reads as a sentence rather than
// a wall of hex.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}

	return sha
}

// The commit identity a provisioned edit is attributed to. It is a fixed, honest name:
// the change came from VYZORIX on behalf of the user, not from the user's own git client.
const (
	commitAuthorName  = "VYZORIX Provisioning"
	commitAuthorEmail = "provisioning@vyzorix.local"
)
