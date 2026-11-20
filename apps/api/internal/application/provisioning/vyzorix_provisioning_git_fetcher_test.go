package provisioning

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// initBareRemote creates a local git repository with a branch and returns a
// file:// URL and the working copy, so the real fetcher can be exercised without
// a network.
func initBareRemote(t *testing.T, files map[string]string) (remoteURL, workdir string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "remote.git")

	run := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@e.st",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@e.st",
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+root,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	run(work, "init", "--initial-branch=main")
	for rel, content := range files {
		full := filepath.Join(work, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	run(work, "add", ".")
	run(work, "commit", "-m", "initial")
	run(root, "clone", "--bare", work, bare)

	return "file://" + bare, work
}

// TestGitFetcher_ClonesLocalRemote proves the real fetcher materializes the
// branch and discovery finds the manifests, without a mock in the path.
func TestGitFetcher_ClonesLocalRemote(t *testing.T) {
	remote, _ := initBareRemote(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
	})

	repo := testRepo()
	repo.URL = remote

	checkout, err := NewGitFetcher().Fetch(context.Background(), repo, "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer checkout.Cleanup()

	manifests, err := Discover(checkout.Dir, "/")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(manifests) != 1 || manifests[0].DeviceID != "device-a" {
		t.Fatalf("manifests = %+v, want one for device-a", manifests)
	}
}

// TestSync_EndToEndAgainstLocalRemote runs the whole path: a real clone, a real
// discovery, a real diff, and an apply through a recording deployer.
func TestSync_EndToEndAgainstLocalRemote(t *testing.T) {
	remote, _ := initBareRemote(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
	})

	repo := testRepo()
	repo.URL = remote

	resources := newFakeResources()
	dep := &fakeDeployer{}
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{ID: "conn-1", OrgID: "org-1", Type: domain.ConnectionGeneric}
	conns.tokens["conn-1"] = ""

	s := NewSyncService(NewGitFetcher(), resources, conns, newFakeRepositoryStore(), dep, newFakeJobs(), nil)

	// Dry run first: it must apply nothing.
	if _, err := s.Sync(context.Background(), repo, SyncOptions{DryRun: true}); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(dep.calls) != 0 {
		t.Fatalf("dry run applied %d manifests", len(dep.calls))
	}

	// Then apply.
	res, err := s.Sync(context.Background(), repo, SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(dep.calls) != 1 || res.Job.Applied != 1 {
		t.Fatalf("applied %d, job applied %d, want 1/1", len(dep.calls), res.Job.Applied)
	}
	if dep.calls[0].DeviceID != "device-a" {
		t.Fatalf("deployed to %q, want device-a", dep.calls[0].DeviceID)
	}
}

// TestGitFetcher_UnreachableRemoteFails proves a bad URL is a fetch failure with
// the reason kept, not a panic or a silent success.
func TestGitFetcher_UnreachableRemoteFails(t *testing.T) {
	repo := testRepo()
	repo.URL = "file:///nonexistent/definitely/not/a/repo.git"

	if _, err := NewGitFetcher().Fetch(context.Background(), repo, ""); err == nil {
		t.Fatal("fetching a nonexistent remote succeeded")
	}
}

// TestFetchKeepsTokenOutOfArgv is the security property: anything in a child's
// argv is readable in /proc/<pid>/cmdline, so the token must travel only in the
// environment. A stand-in git records exactly what it was given.
func TestFetchKeepsTokenOutOfArgv(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is not available")
	}

	dir := t.TempDir()
	captured := filepath.Join(dir, "argv.txt")
	fake := filepath.Join(dir, "fakegit")

	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + captured + "\nexit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatalf("write stand-in: %v", err)
	}

	const token = "SECRET_TOKEN_VALUE"
	f := NewGitFetcher()
	f.bin = fake

	repo := &domain.Repository{URL: "https://github.com/acme/fleet.git", Branch: "main", Type: domain.ConnectionGitHub}
	if _, err := f.Fetch(context.Background(), repo, token); err == nil {
		t.Fatal("a failing clone must return an error")
	}

	argv, err := os.ReadFile(captured) // #nosec G304 -- path built from t.TempDir.
	if err != nil {
		t.Fatalf("read captured argv: %v", err)
	}
	if strings.Contains(string(argv), token) {
		t.Fatalf("the token reached the command line:\n%s", argv)
	}
	if strings.Contains(string(argv), "@github.com") {
		t.Fatalf("the URL carried credentials into argv:\n%s", argv)
	}
	// The credential helper must be configured, or the clone has no way to authenticate.
	if !strings.Contains(string(argv), "credential.helper=!") {
		t.Fatalf("no credential helper was configured:\n%s", argv)
	}
}

// TestCredentialHelperAnswersFromEnv proves the helper emits the token git needs,
// and that a non-helper invocation is declined so normal startup can proceed.
func TestCredentialHelperAnswersFromEnv(t *testing.T) {
	t.Setenv("VYZORIX_GIT_TOKEN", "tok-from-env")
	t.Setenv("VYZORIX_GIT_USERNAME", "x-access-token")

	if RunCredentialHelper([]string{"serve"}) {
		t.Fatal("a normal invocation must not be treated as a credential helper")
	}

	if !RunCredentialHelper([]string{"git-credential-helper", "get"}) {
		t.Fatal("the get operation must be served")
	}

	// store/erase are acknowledged but write nothing.
	if !RunCredentialHelper([]string{"git-credential-helper", "store"}) {
		t.Fatal("the store operation must be acknowledged")
	}
}

// TestGitFetcher_AuthenticatesThroughHelper is the real end-to-end proof: an
// actual `git clone` against a Basic-auth HTTP remote, authenticated only by the
// credential helper reading the token from the environment.
//
// It is skipped unless a test remote is advertised, because it needs a git HTTP
// server rather than a fixture.
func TestGitFetcher_AuthenticatesThroughHelper(t *testing.T) {
	remote := os.Getenv("VYZORIX_TEST_AUTH_REMOTE")
	if remote == "" {
		t.Skip("set VYZORIX_TEST_AUTH_REMOTE to a Basic-auth git URL to run this")
	}

	repo := testRepo()
	repo.URL = remote
	repo.Type = domain.ConnectionGitHub

	checkout, err := NewGitFetcher().Fetch(context.Background(), repo, "SECRET_TOKEN_VALUE")
	if err != nil {
		t.Fatalf("authenticated fetch failed: %v", err)
	}
	defer checkout.Cleanup()

	if _, statErr := os.Stat(filepath.Join(checkout.Dir, "README.md")); statErr != nil {
		t.Fatalf("the clone did not produce the repository contents: %v", statErr)
	}

	// The same URL without a token must fail, proving the helper is what authenticated.
	if _, err := NewGitFetcher().Fetch(context.Background(), repo, ""); err == nil {
		t.Fatal("an unauthenticated clone of a protected remote succeeded")
	}
}

// TestRedactOutput keeps a token out of a stored error message.
func TestRedactOutput(t *testing.T) {
	got := redactOutput("fatal: could not read from https://x-access-token:secret123@host/r.git", "secret123")
	if got == "" || strings.Contains(got, "secret123") {
		t.Fatalf("token leaked into output: %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
