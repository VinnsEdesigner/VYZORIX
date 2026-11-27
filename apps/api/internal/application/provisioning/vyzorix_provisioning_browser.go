package provisioning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

var (
	// ErrPathNotFound is returned when a browse request names a file or directory the
	// repository does not contain at its branch.
	ErrPathNotFound = errors.New("no such file or directory in the repository")
	// ErrPathOutsideRepo is returned when a requested path would escape the checkout. A path
	// arrives from the network, so ".." must be refused rather than resolved.
	ErrPathOutsideRepo = errors.New("path escapes the repository")
)

// TreeEntry is one entry in the repository's file listing.
type TreeEntry struct {
	// Path is relative to the repository root, so a client can link to it directly.
	Path string
	Name string
	// Kind is "file" or "dir".
	Kind string
	Size int64
}

// FileView is one file's content at a commit.
//
// Fields are ordered so the time.Time (a wide value) comes first and the pointer-bearing
// slice sits before the lone bool: fieldalignment flags any other arrangement, and the
// comment on each field says what it is for.
type FileView struct {
	When time.Time
	Path string
	// Commit is the commit the content was read at, so an edit can be based on it and the
	// push can detect that the remote moved.
	Commit  string
	Message string
	Author  string
	Content []byte
	// Truncated is set when the file exceeded the read limit, so a client does not present a
	// partial file as complete.
	Truncated bool
}

// CommitInfo is one entry in the repository's history.
type CommitInfo struct {
	When    time.Time
	SHA     string
	Message string
	Author  string
	// Files is how many files the commit touched, when it could be counted cheaply.
	Files int
}

// RepoBrowser reads a repository's tree, files and history.
//
// It is a separate interface from Pusher because browsing must work on a read-only
// connection: an operator who cannot push should still be able to see what is in the
// repository.
type RepoBrowser interface {
	// ListTree returns the entries under dir (empty for the root). One clone serves the whole
	// listing rather than one per entry.
	ListTree(ctx context.Context, repo *domain.Repository, dir string, token redaction.Secret) ([]TreeEntry, error)
	// ReadFileAt returns one file's content and the commit it came from.
	ReadFileAt(ctx context.Context, repo *domain.Repository, path string, token redaction.Secret) (*FileView, error)
	// History returns the repository's recent commits, newest first.
	History(ctx context.Context, repo *domain.Repository, limit int, token redaction.Secret) ([]CommitInfo, error)
}

// maxBrowsableFileBytes bounds what a browse read will return. A manifest is a small TOML
// document; a multi-megabyte blob is a mistake, and streaming it through JSON is worse than
// refusing it.
const maxBrowsableFileBytes = 512 << 10

// historyDepth is how much history a log read clones. A full clone of a large repository is
// minutes of work to answer "show me the last 20 commits", so the clone is bounded and the
// response says what window it covers.
const historyDepth = 50

// GitBrowser implements RepoBrowser with the same git binary the fetcher uses, so it shares
// the credential helper, CA bundle and ssh agent.
type GitBrowser struct {
	timeout time.Duration
}

// NewGitBrowser builds a browser. A zero timeout falls back to two minutes.
func NewGitBrowser() *GitBrowser {
	return &GitBrowser{timeout: 2 * time.Minute}
}

// ListTree lists the entries under dir.
func (g *GitBrowser) ListTree(
	ctx context.Context, repo *domain.Repository, dir string, token redaction.Secret,
) ([]TreeEntry, error) {
	checkout, err := NewGitFetcher().Fetch(ctx, repo, token)
	if err != nil {
		return nil, err
	}
	defer checkout.Cleanup()

	// An empty dir means the root. A non-empty one must stay inside the checkout.
	root := checkout.Dir
	if strings.TrimSpace(dir) != "" && dir != "/" {
		root, err = browseJoin(checkout.Dir, dir)
		if err != nil {
			return nil, err
		}
	}

	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrPathNotFound
		}

		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrPathNotFound
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	base := strings.Trim(strings.TrimSpace(dir), "/")
	out := make([]TreeEntry, 0, len(entries))

	for _, entry := range entries {
		// .git is the checkout's own metadata, not repository content.
		if entry.Name() == ".git" {
			continue
		}

		relative := entry.Name()
		if base != "" {
			relative = base + "/" + entry.Name()
		}

		kind := "file"
		var size int64
		if entry.IsDir() {
			kind = "dir"
		} else if stat, statErr := entry.Info(); statErr == nil {
			size = stat.Size()
		}

		out = append(out, TreeEntry{Path: relative, Name: entry.Name(), Kind: kind, Size: size})
	}

	// Directories first, then files, each alphabetical: the order a person reads a tree in.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == "dir"
		}

		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})

	return out, nil
}

// ReadFileAt reads one file's content and the commit it came from.
func (g *GitBrowser) ReadFileAt(
	ctx context.Context, repo *domain.Repository, path string, token redaction.Secret,
) (*FileView, error) {
	checkout, err := NewGitFetcher().Fetch(ctx, repo, token)
	if err != nil {
		return nil, err
	}
	defer checkout.Cleanup()

	full, err := browseJoin(checkout.Dir, path)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrPathNotFound
		}

		return nil, err
	}
	if stat.IsDir() {
		return nil, ErrPathNotFound
	}

	view := &FileView{Path: strings.TrimPrefix(path, "/"), Truncated: stat.Size() > maxBrowsableFileBytes}

	content, err := os.ReadFile(full) // #nosec G304 -- full is contained within the checkout by browseJoin.
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrFetchFailed, path, err)
	}
	if view.Truncated {
		content = content[:maxBrowsableFileBytes]
	}
	view.Content = content

	// The commit and its subject give the reader "what am I looking at, and when did it
	// change" without a second round trip.
	commit, message, author, when := g.headCommit(ctx, checkout.Dir)
	view.Commit, view.Message, view.Author, view.When = commit, message, author, when

	return view, nil
}

// History returns recent commits, newest first.
func (g *GitBrowser) History(
	ctx context.Context, repo *domain.Repository, limit int, token redaction.Secret,
) ([]CommitInfo, error) {
	if limit <= 0 || limit > historyDepth {
		limit = 20
	}

	// A shallow clone has no history, so this one is deeper. It is still bounded: the point is
	// a recent-activity view, not an archaeology tool.
	dir, cleanup, err := g.cloneWithHistory(ctx, repo, token)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	format := "%H%x1f%s%x1f%an%x1f%aI"
	out, err := g.git(ctx, dir, "log", fmt.Sprintf("-%d", limit), "--date=iso-strict", "--pretty=format:"+format)
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "\x1f")
		if len(parts) < 4 {
			continue
		}

		when, _ := time.Parse(time.RFC3339, parts[3])
		commits = append(commits, CommitInfo{SHA: parts[0], Message: parts[1], Author: parts[2], When: when})
	}

	return commits, nil
}

// cloneWithHistory clones enough history for a log read.
func (g *GitBrowser) cloneWithHistory(
	ctx context.Context, repo *domain.Repository, token redaction.Secret,
) (string, func(), error) {
	timeout := g.timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "vyzorix-browse-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	helper := credentialHelperConfig()
	if len(helper) == 0 && !token.IsZero() && isHTTPS(repo.URL) {
		cleanup()

		return "", nil, fmt.Errorf("%w: no credential helper available", ErrFetchFailed)
	}

	args := append([]string{}, helper...)
	args = append(args,
		"-c", "core.askPass=",
		"clone", "--depth", fmt.Sprint(historyDepth), "--single-branch", "--no-tags",
		"--branch", repo.Branch, "--", repo.URL, dir,
	)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = gitEnv(repo.Type, token.Reveal())

	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()

		return "", nil, fmt.Errorf("%w: %s", ErrFetchFailed, redactOutput(string(out), token.Reveal()))
	}

	return dir, cleanup, nil
}

// headCommit reads HEAD's sha, subject, author and time.
func (g *GitBrowser) headCommit(ctx context.Context, dir string) (string, string, string, time.Time) {
	out, err := g.git(ctx, dir, "log", "-1", "--pretty=format:%H%x1f%s%x1f%an%x1f%aI")
	if err != nil {
		return "", "", "", time.Time{}
	}

	parts := strings.Split(strings.TrimSpace(out), "\x1f")
	if len(parts) < 4 {
		return "", "", "", time.Time{}
	}

	when, _ := time.Parse(time.RFC3339, parts[3])

	return parts[0], parts[1], parts[2], when
}

func (g *GitBrowser) git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrFetchFailed, strings.TrimSpace(string(out)))
	}

	return string(out), nil
}

// browseJoin resolves a browse path inside the checkout, refusing one that escapes it.
//
// It exists rather than reusing safeJoin because the two callers report a refusal
// differently: a push refuses with ErrPushFailed, while a browse must answer 400 — the path
// is not "missing", it is "not allowed", and a 500 would tell a caller the server broke when
// it did exactly what it should.
func browseJoin(root, rel string) (string, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))

	relPath, err := filepath.Rel(root, full)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path %q escapes the checkout", ErrPathOutsideRepo, rel)
	}

	return full, nil
}
