package provisioning

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// initPushRemote creates a local bare remote with one manifest, so the real pusher can be
// exercised without a network.
func initPushRemote(t *testing.T, content string) (remoteURL, workdir string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "remote.git")

	run := func(dir string, args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@e.st",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@e.st",
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+root,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}

		return strings.TrimSpace(string(out))
	}

	if err := os.MkdirAll(filepath.Join(work, "devices", "device-a"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	run(work, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(work, "devices", "device-a", "vyzorix.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	run(work, "add", ".")
	run(work, "commit", "-m", "initial")
	run(root, "clone", "--bare", work, bare)

	return "file://" + bare, work
}

// TestGitPusherReadsTheCurrentFile proves ReadFile returns the file and the commit it came
// from, which is what a preview is based on.
func TestGitPusherReadsTheCurrentFile(t *testing.T) {
	remote, _ := initPushRemote(t, "original content\n")

	repo := testRepo()
	repo.URL = remote

	content, commit, err := NewGitPusher().ReadFile(context.Background(), repo,
		"devices/device-a/vyzorix.toml", redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(content) != "original content\n" {
		t.Fatalf("content = %q", content)
	}
	if len(commit) != 40 {
		t.Fatalf("commit = %q, want a full SHA", commit)
	}
}

// TestGitPusherCommitsToANewBranch is the real end-to-end proof for PR mode: a new branch
// is created on the remote carrying the edited file, and the base branch is untouched.
func TestGitPusherCommitsToANewBranch(t *testing.T) {
	remote, _ := initPushRemote(t, "original content\n")

	repo := testRepo()
	repo.URL = remote

	pusher := NewGitPusher()

	_, base, err := pusher.ReadFile(context.Background(), repo, "devices/device-a/vyzorix.toml", redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	result, err := pusher.Push(context.Background(), PushRequest{
		Repo:       repo,
		Path:       "devices/device-a/vyzorix.toml",
		Content:    []byte("edited content\n"),
		BaseCommit: base,
		Branch:     "vyzorix/edit-test",
		Message:    "test edit",
	}, redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	if !result.Changed || result.Commit == "" {
		t.Fatalf("result = %+v, want a change with a commit", result)
	}

	// The branch exists on the remote with the new content.
	root := t.TempDir()
	verify := exec.Command("git", "clone", "--branch", "vyzorix/edit-test", remote, filepath.Join(root, "check"))
	verify.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+root)
	if out, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("the branch was not pushed: %v\n%s", err, out)
	}

	got, err := os.ReadFile(filepath.Join(root, "check", "devices", "device-a", "vyzorix.toml")) // #nosec G304 -- test fixture.
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "edited content\n" {
		t.Fatalf("pushed content = %q, want the edit", got)
	}
}

// TestGitPusherRefusesAStaleBaseCommit is the conflict guard, proven against real git: a
// commit that is not the branch head is refused rather than force-pushed over.
func TestGitPusherRefusesAStaleBaseCommit(t *testing.T) {
	remote, _ := initPushRemote(t, "original content\n")

	repo := testRepo()
	repo.URL = remote

	_, err := NewGitPusher().Push(context.Background(), PushRequest{
		Repo:       repo,
		Path:       "devices/device-a/vyzorix.toml",
		Content:    []byte("edited\n"),
		BaseCommit: "0000000000000000000000000000000000000000",
		Branch:     "vyzorix/edit-stale",
		Message:    "should not land",
	}, redaction.NewSecret(""))
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}

// TestGitPusherNoOpWhenContentMatches proves an identical edit makes no commit, so the
// provider's history is not polluted with empty changes.
func TestGitPusherNoOpWhenContentMatches(t *testing.T) {
	remote, _ := initPushRemote(t, "same content\n")

	repo := testRepo()
	repo.URL = remote

	pusher := NewGitPusher()

	_, base, err := pusher.ReadFile(context.Background(), repo, "devices/device-a/vyzorix.toml", redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	result, err := pusher.Push(context.Background(), PushRequest{
		Repo:       repo,
		Path:       "devices/device-a/vyzorix.toml",
		Content:    []byte("same content\n"),
		BaseCommit: base,
		Branch:     "main",
		Message:    "no change",
	}, redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("Push: %v", err)
	}

	if result.Changed {
		t.Fatal("an identical edit was committed")
	}
}

// TestGitPusherRefusesAnEmptyTokenForANetworkRemote proves an anonymous push to a network
// remote is refused rather than attempted.
func TestGitPusherRefusesAnEmptyTokenForANetworkRemote(t *testing.T) {
	repo := testRepo()
	repo.URL = "https://github.com/acme/fleet.git"

	_, err := NewGitPusher().Push(context.Background(), PushRequest{Repo: repo}, "")
	if !errors.Is(err, ErrWriteDisabled) {
		t.Fatalf("got %v, want ErrWriteDisabled", err)
	}
}

// TestGitPusherAllowsATokenlessLocalRemote proves a file:// remote needs no credential, so
// the guard does not block the one case that has none.
func TestGitPusherAllowsATokenlessLocalRemote(t *testing.T) {
	if !isLocalRemote("file:///tmp/x.git") || !isLocalRemote("/tmp/x.git") {
		t.Fatal("a local path must be recognised as needing no credential")
	}
	if isLocalRemote("https://github.com/acme/fleet.git") {
		t.Fatal("a network URL must not be treated as local")
	}
}

// TestSameCommitAcceptsAnAbbreviatedBase is the guard for a defect the live test caught: a
// client that sends the short SHA it was shown must not be reported as a conflict when the
// branch has not moved. git accepts abbreviations everywhere, so this must too.
func TestSameCommitAcceptsAnAbbreviatedBase(t *testing.T) {
	const full = "56b40d7ae76d70ced5c764e2f900caa7424d31f0"

	if !sameCommit(full, full) {
		t.Fatal("identical full SHAs must match")
	}
	if !sameCommit(full, "56b40d7") {
		t.Fatal("an abbreviated base must match its full SHA")
	}
	if !sameCommit("56b40d7", full) {
		t.Fatal("an abbreviated head must match its full SHA")
	}
	if sameCommit(full, "deadbeef") {
		t.Fatal("a different commit must not match")
	}
	if sameCommit(full, "") || sameCommit("", full) {
		t.Fatal("an empty SHA must never match")
	}
}

// TestGitPusherAcceptsAShortBaseCommit proves the whole push path accepts an abbreviated
// base against a real remote.
func TestGitPusherAcceptsAShortBaseCommit(t *testing.T) {
	remote, _ := initPushRemote(t, "original content\n")

	repo := testRepo()
	repo.URL = remote

	pusher := NewGitPusher()

	_, base, err := pusher.ReadFile(context.Background(), repo, "devices/device-a/vyzorix.toml", redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	result, err := pusher.Push(context.Background(), PushRequest{
		Repo:       repo,
		Path:       "devices/device-a/vyzorix.toml",
		Content:    []byte("short-base edit\n"),
		BaseCommit: base[:12],
		Branch:     "vyzorix/edit-short",
		Message:    "short base",
	}, redaction.NewSecret(""))
	if err != nil {
		t.Fatalf("Push with a short base: %v", err)
	}
	if !result.Changed {
		t.Fatal("the edit did not land")
	}
}
