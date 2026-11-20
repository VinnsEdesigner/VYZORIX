package provisioning

import (
	"context"
	"errors"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// failingEvents is an EventRepository whose write always fails.
type failingEvents struct {
	err    error
	writes int
}

func (f *failingEvents) Record(context.Context, *domain.ProvisioningEvent) error {
	f.writes++

	return f.err
}

func (f *failingEvents) ListForOrg(context.Context, string, int) ([]*domain.ProvisioningEvent, error) {
	return nil, nil
}

func (f *failingEvents) ListForResource(context.Context, string, string, string, int) ([]*domain.ProvisioningEvent, error) {
	return nil, nil
}

// recordingEscalator counts escalations and remembers their arguments.
type recordingEscalator struct {
	err    error
	orgs   []string
	causes []error
}

func (r *recordingEscalator) EscalateAuditFailure(_ context.Context, orgID string, cause error) error {
	r.orgs = append(r.orgs, orgID)
	r.causes = append(r.causes, cause)

	return r.err
}

// TestAuditFailureEscalates is the requirement: a failed audit write must raise an
// alert, not merely log. Logging alone means nobody learns until after the incident.
func TestAuditFailureEscalates(t *testing.T) {
	cause := errors.New("disk full")
	store := &failingEvents{err: cause}
	escalator := &recordingEscalator{}

	auditor := NewAuditor(store, escalator, nil)

	err := auditor.Record(context.Background(), &domain.ProvisioningEvent{
		ID: "ev-1", OrgID: "org-1", Actor: domain.SchedulerActor(),
		Type: domain.EventSyncApplied, ResourceType: "repository", Result: "success",
	})
	if err == nil {
		t.Fatal("Record must return the write error")
	}

	if len(escalator.orgs) != 1 {
		t.Fatalf("escalations = %d, want 1", len(escalator.orgs))
	}
	if escalator.orgs[0] != "org-1" {
		t.Fatalf("escalated for %q, want org-1", escalator.orgs[0])
	}
	if !errors.Is(escalator.causes[0], cause) {
		t.Fatalf("escalated cause = %v, want the write error", escalator.causes[0])
	}
}

// TestAuditSuccessDoesNotEscalate pins the inverse: a healthy write must never alert,
// or the alert becomes noise and stops being read.
func TestAuditSuccessDoesNotEscalate(t *testing.T) {
	escalator := &recordingEscalator{}
	auditor := NewAuditor(&fakeEvents{}, escalator, nil)

	if err := auditor.Record(context.Background(), &domain.ProvisioningEvent{
		ID: "ev-1", OrgID: "org-1", Actor: domain.SchedulerActor(),
		Type: domain.EventSyncApplied, ResourceType: "repository", Result: "success",
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if len(escalator.orgs) != 0 {
		t.Fatalf("a successful write escalated %d times", len(escalator.orgs))
	}
}

// TestAuditEscalatesOncePerWrite proves one broken write is one alert, not a storm.
func TestAuditEscalatesOncePerWrite(t *testing.T) {
	store := &failingEvents{err: errors.New("locked")}
	escalator := &recordingEscalator{}
	auditor := NewAuditor(store, escalator, nil)

	_ = auditor.Record(context.Background(), &domain.ProvisioningEvent{
		ID: "ev-1", OrgID: "org-1", Actor: domain.SchedulerActor(),
		Type: domain.EventSyncApplied, ResourceType: "repository", Result: "success",
	})

	if store.writes != 1 {
		t.Fatalf("writes = %d, want 1 (no retry loop)", store.writes)
	}
	if len(escalator.orgs) != 1 {
		t.Fatalf("escalations = %d, want exactly 1", len(escalator.orgs))
	}
}

// TestAuditEscalatesEvenWhenTheRequestWasCancelled is the guard for the case that
// matters most. The commonest reason an audit write fails mid-request is a cancelled
// request, and an alert that inherits that cancellation would be lost exactly when it
// is needed.
func TestAuditEscalatesEvenWhenTheRequestWasCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	escalator := &ctxRecordingEscalator{}
	auditor := NewAuditor(&failingEvents{err: errors.New("write failed")}, escalator, nil)

	_ = auditor.Record(ctx, &domain.ProvisioningEvent{
		ID: "ev-1", OrgID: "org-1", Actor: domain.SchedulerActor(),
		Type: domain.EventSyncApplied, ResourceType: "repository", Result: "success",
	})

	if len(escalator.ctxErrors) != 1 {
		t.Fatalf("escalations = %d, want 1", len(escalator.ctxErrors))
	}
	if escalator.ctxErrors[0] != nil {
		t.Fatalf("the alert inherited the cancelled context: %v", escalator.ctxErrors[0])
	}
}

// ctxRecordingEscalator records whether the context it received was already dead.
type ctxRecordingEscalator struct {
	ctxErrors []error
}

func (r *ctxRecordingEscalator) EscalateAuditFailure(ctx context.Context, _ string, _ error) error {
	r.ctxErrors = append(r.ctxErrors, ctx.Err())

	return nil
}

// TestEscalationFailureIsContained proves a broken escalator does not mask the
// original error: the caller still learns the audit write failed.
func TestEscalationFailureIsContained(t *testing.T) {
	cause := errors.New("disk full")
	auditor := NewAuditor(&failingEvents{err: cause}, &recordingEscalator{err: errors.New("no channel")}, nil)

	err := auditor.Record(context.Background(), &domain.ProvisioningEvent{
		ID: "ev-1", OrgID: "org-1", Actor: domain.SchedulerActor(),
		Type: domain.EventSyncApplied, ResourceType: "repository", Result: "success",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want the original write error, not the escalation's", err)
	}
}

// TestAuditorWithoutStoreIsNoOp proves an unwired auditor does not panic or escalate.
func TestAuditorWithoutStoreIsNoOp(t *testing.T) {
	escalator := &recordingEscalator{}
	auditor := NewAuditor(nil, escalator, nil)

	if err := auditor.Record(context.Background(), &domain.ProvisioningEvent{ID: "x"}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if len(escalator.orgs) != 0 {
		t.Fatal("an auditor with no store escalated")
	}
}

// TestSyncAuditFailureEscalatesThroughTheService proves the wiring: a broken audit
// trail during a sync reaches the escalator, and the sync itself still succeeds.
func TestSyncAuditFailureEscalatesThroughTheService(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	escalator := &recordingEscalator{}
	auditor := NewAuditor(&failingEvents{err: errors.New("audit down")}, escalator, nil)

	s := newSyncServiceForTest(t, root, newFakeResources(), newFakeJobs(), &fakeDeployer{})
	s.SetAuditor(auditor)

	res, err := s.Sync(context.Background(), testRepo(), SyncOptions{})
	if err != nil {
		t.Fatalf("Sync must succeed even when the audit write fails: %v", err)
	}
	if res.Job.State != domain.SyncSuccess {
		t.Fatalf("state = %s, want success", res.Job.State)
	}
	if len(escalator.orgs) != 1 || escalator.orgs[0] != "org-1" {
		t.Fatalf("escalations = %v, want one for org-1", escalator.orgs)
	}
}
