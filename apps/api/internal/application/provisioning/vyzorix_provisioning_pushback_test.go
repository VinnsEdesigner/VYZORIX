package provisioning

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// fakePusher records what a push-back would do, so a test can assert the preview wrote
// nothing and the apply wrote exactly once.
type fakePusher struct {
	current   []byte
	base      string
	readErr   error
	pushErr   error
	pushes    []PushRequest
	readCalls int
}

func (p *fakePusher) ReadFile(context.Context, *domain.Repository, string, redaction.Secret) ([]byte, string, error) {
	p.readCalls++
	if p.readErr != nil {
		return nil, "", p.readErr
	}

	return p.current, p.base, nil
}

func (p *fakePusher) Push(_ context.Context, req PushRequest, _ redaction.Secret) (*PushResult, error) {
	p.pushes = append(p.pushes, req)
	if p.pushErr != nil {
		return nil, p.pushErr
	}

	return &PushResult{Commit: "abc123", Branch: req.Branch, Changed: true}, nil
}

// fakePR records the pull request it was asked to open.
type fakePR struct {
	err      error
	url      string
	requests []PullRequest
}

func (f *fakePR) OpenPullRequest(_ context.Context, req PullRequest, _ redaction.Secret) (string, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return "", f.err
	}

	return f.url, nil
}

// pushBackFixture builds a service over a write-enabled connection.
func pushBackFixture(t *testing.T, writeEnabled bool) (*PushBackService, *fakePusher, *fakePR) {
	t.Helper()

	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
		WriteEnabled: writeEnabled,
	}
	conns.tokens["conn-1"] = "token"

	pusher := &fakePusher{current: []byte("old content\n"), base: "base-sha"}
	pr := &fakePR{url: "https://github.com/acme/fleet/pull/1"}

	return NewPushBackService(pusher, conns, pr, nil, nil), pusher, pr
}

func editFixture() EditRequest {
	return EditRequest{
		Actor:   domain.Actor{Kind: domain.ActorUser, ID: "u1", Label: "a@b.c"},
		Repo:    &domain.Repository{ID: "repo-1", OrgID: "org-1", ConnectionID: "conn-1", Name: "fleet", Branch: "main"},
		Path:    "devices/device-a/vyzorix.toml",
		Content: []byte("new content\n"),
	}
}

// TestPreviewWritesNothing is the guarantee the preview exists for: it must not touch the
// repository.
func TestPreviewWritesNothing(t *testing.T) {
	svc, pusher, _ := pushBackFixture(t, true)

	preview, err := svc.Preview(context.Background(), editFixture())
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if len(pusher.pushes) != 0 {
		t.Fatalf("the preview pushed %d times", len(pusher.pushes))
	}
	if !preview.Changed {
		t.Fatal("a changed file was reported unchanged")
	}
	if preview.BaseCommit != "base-sha" {
		t.Fatalf("base commit = %q, want the commit read from the repository", preview.BaseCommit)
	}
	if !strings.Contains(preview.Diff, "-old content") || !strings.Contains(preview.Diff, "+new content") {
		t.Fatalf("the diff does not show the change:\n%s", preview.Diff)
	}
}

// TestPreviewReportsNoChangeWhenIdentical proves an identical edit is reported as such, so
// the UI can disable the push.
func TestPreviewReportsNoChangeWhenIdentical(t *testing.T) {
	svc, pusher, _ := pushBackFixture(t, true)
	pusher.current = []byte("same content\n")

	req := editFixture()
	req.Content = []byte("same content\n")

	preview, err := svc.Preview(context.Background(), req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.Changed {
		t.Fatal("an identical edit was reported as changed")
	}
}

// TestReadOnlyConnectionRefusesAnEdit proves a read-only credential cannot push, and that
// it is refused before anything is read or written.
func TestReadOnlyConnectionRefusesAnEdit(t *testing.T) {
	svc, pusher, _ := pushBackFixture(t, false)

	if _, err := svc.Preview(context.Background(), editFixture()); !errors.Is(err, ErrEditDisabled) {
		t.Fatalf("preview on a read-only connection: %v, want ErrEditDisabled", err)
	}
	applyReq := editFixture()
	applyReq.BaseCommit = "base-sha"
	if _, err := svc.Apply(context.Background(), applyReq); !errors.Is(err, ErrEditDisabled) {
		t.Fatalf("apply on a read-only connection: %v, want ErrEditDisabled", err)
	}
	if pusher.readCalls != 0 || len(pusher.pushes) != 0 {
		t.Fatal("a read-only connection still touched the repository")
	}
}

// TestPRModePushesAGeneratedBranchAndOpensARequest is the v1 requirement: PR mode is
// supported, not just direct-to-branch.
func TestPRModePushesAGeneratedBranchAndOpensARequest(t *testing.T) {
	svc, pusher, pr := pushBackFixture(t, true)

	req := editFixture()
	req.Mode = PushModePR
	req.BaseCommit = "base-sha"

	result, err := svc.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(pusher.pushes) != 1 {
		t.Fatalf("pushed %d times, want 1", len(pusher.pushes))
	}
	pushed := pusher.pushes[0]
	if !strings.HasPrefix(pushed.Branch, "vyzorix/edit-fleet-") {
		t.Fatalf("branch = %q, want a generated vyzorix/edit- branch", pushed.Branch)
	}
	if pushed.Branch == "main" {
		t.Fatal("PR mode committed to the repository's own branch")
	}

	if len(pr.requests) != 1 {
		t.Fatalf("opened %d pull requests, want 1", len(pr.requests))
	}
	if pr.requests[0].Base != "main" {
		t.Fatalf("pull request base = %q, want main", pr.requests[0].Base)
	}
	if result.PullRequestURL == "" {
		t.Fatal("the result does not carry the pull request URL")
	}
}

// TestDirectModePushesTheRepositoryBranch proves the second mode.
func TestDirectModePushesTheRepositoryBranch(t *testing.T) {
	svc, pusher, pr := pushBackFixture(t, true)

	req := editFixture()
	req.Mode = PushModeDirect
	req.BaseCommit = "base-sha"

	if _, err := svc.Apply(context.Background(), req); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(pusher.pushes) != 1 || pusher.pushes[0].Branch != "main" {
		t.Fatalf("pushed %+v, want one push to main", pusher.pushes)
	}
	if len(pr.requests) != 0 {
		t.Fatal("direct mode opened a pull request")
	}
}

// TestConflictIsRefused proves a remote that moved is reported as a conflict, never
// force-pushed over.
func TestConflictIsRefused(t *testing.T) {
	svc, pusher, _ := pushBackFixture(t, true)
	pusher.pushErr = ErrConflict

	req := editFixture()
	req.BaseCommit = "stale-sha"

	if _, err := svc.Apply(context.Background(), req); !errors.Is(err, ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}

// TestPullRequestFailureDoesNotLoseThePush proves a provider outage after the push is
// reported beside the result, not returned as a failure: the branch is already on the
// remote, and telling the user nothing was pushed would be false.
func TestPullRequestFailureDoesNotLoseThePush(t *testing.T) {
	svc, _, pr := pushBackFixture(t, true)
	pr.err = errors.New("provider unavailable")

	req := editFixture()
	req.Mode = PushModePR
	req.BaseCommit = "base-sha"

	result, err := svc.Apply(context.Background(), req)
	if err != nil {
		t.Fatalf("a pull request failure lost the push: %v", err)
	}
	if result.PullRequestError == "" {
		t.Fatal("the pull request failure was not reported")
	}
	if result.Commit != "abc123" {
		t.Fatalf("commit = %q, want the pushed commit", result.Commit)
	}
}

// TestCommitMessageTemplating pins the default message and the override.
func TestCommitMessageTemplating(t *testing.T) {
	req := editFixture()

	if got := commitMessage(req); !strings.Contains(got, req.Path) {
		t.Fatalf("default message = %q, want it to name the path", got)
	}

	req.CommitMessage = "custom message"
	if got := commitMessage(req); got != "custom message" {
		t.Fatalf("override = %q, want the custom message", got)
	}
}

// TestGeneratedBranchIsUnique pins that two edits at different times cannot collide.
func TestGeneratedBranchIsUnique(t *testing.T) {
	a := generatedBranch("my fleet", time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC))
	b := generatedBranch("my fleet", time.Date(2026, 1, 1, 10, 0, 1, 0, time.UTC))

	if a == b {
		t.Fatalf("two edits produced the same branch: %s", a)
	}
	if !strings.HasPrefix(a, "vyzorix/edit-my-fleet-") {
		t.Fatalf("branch = %q, want a slugged generated name", a)
	}
}

// TestPushModeValid pins the accepted modes.
func TestPushModeValid(t *testing.T) {
	for _, m := range []PushMode{PushModePR, PushModeDirect} {
		if !m.Valid() {
			t.Errorf("%q should be valid", m)
		}
	}
	for _, m := range []PushMode{"", "push", "force"} {
		if m.Valid() {
			t.Errorf("%q should not be valid", m)
		}
	}
}

// TestApplyRequiresThePreviewBase proves a push cannot skip the conflict check. Without a
// base the check cannot run, and a concurrent change would be silently overwritten.
func TestApplyRequiresThePreviewBase(t *testing.T) {
	svc, pusher, _ := pushBackFixture(t, true)

	req := editFixture()
	req.Mode = PushModeDirect
	req.BaseCommit = ""

	if _, err := svc.Apply(context.Background(), req); !errors.Is(err, ErrBaseCommitRequired) {
		t.Fatalf("got %v, want ErrBaseCommitRequired", err)
	}
	if len(pusher.pushes) != 0 {
		t.Fatal("a push without a base still reached the remote")
	}
}

// TestAuditRecordsTheCommitAndOutcome proves the audit row is followable and honest about a
// partial success.
func TestAuditRecordsTheCommitAndOutcome(t *testing.T) {
	svc, _, pr := pushBackFixture(t, true)
	events := &fakeEvents{}
	svc.auditor = NewAuditor(events, nil, nil)

	req := editFixture()
	req.Mode = PushModePR
	req.BaseCommit = "base-sha"

	// A successful PR.
	if _, err := svc.Apply(context.Background(), req); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(events.recorded) != 1 {
		t.Fatalf("events = %d, want 1", len(events.recorded))
	}
	if events.recorded[0].Result != "success" {
		t.Fatalf("result = %q, want success", events.recorded[0].Result)
	}
	if !strings.Contains(events.recorded[0].Detail, "commit ") {
		t.Fatalf("the audit row does not name the commit: %q", events.recorded[0].Detail)
	}

	// A push whose pull request failed is partial, not success.
	pr.err = errors.New("provider unavailable")
	events.recorded = nil

	if _, err := svc.Apply(context.Background(), req); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if events.recorded[0].Result != "partial" {
		t.Fatalf("result = %q, want partial", events.recorded[0].Result)
	}
}
