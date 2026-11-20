package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// recordingAudit captures deploy audit events so a test can assert which actions a
// request produced.
type recordingAudit struct {
	mu     sync.Mutex
	events []audit.DeployEvent
}

func (r *recordingAudit) Deploy(_ context.Context, e audit.DeployEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, e)
}

// DeployFromService records a service-side event. The service emits through this method,
// so a sink installed on the service must implement it.
func (r *recordingAudit) DeployFromService(_ context.Context, e deploy.DeployEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, audit.DeployEvent{
		UserID:   e.UserID,
		OrgID:    e.OrgID,
		DeviceID: e.DeviceID,
		Version:  e.Version,
		Action:   audit.Action(e.Action),
		Result:   audit.Result(e.Result),
		Detail:   e.Detail,
	})
}

func (r *recordingAudit) snapshot() []audit.DeployEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]audit.DeployEvent, len(r.events))
	copy(out, r.events)

	return out
}

// TestDeployRecordsAnAuditEvent proves a successful deploy writes an audit row naming the
// device, the organisation and the version, attributed to the acting user.
//
// A deploy decides what code a device runs, so it is audited with the same weight as a
// command. Without this a fleet could be reconfigured with no record of who did it.
//
// The event is emitted by the deploy service, not the handler, so it is recorded whichever
// door the deploy came through — including a provisioned repository sync, which never
// touches the HTTP handler. The sink is therefore installed on the service.
func TestDeployRecordsAnAuditEvent(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	sink := &recordingAudit{}
	f.server.deployService.SetAudit(sink)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("deploy status = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	events := sink.snapshot()
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}

	e := events[0]
	if e.Action != audit.ActionDeploySucceeded {
		t.Fatalf("action = %q, want %q", e.Action, audit.ActionDeploySucceeded)
	}
	if e.Result != audit.ResultSuccess {
		t.Fatalf("result = %q, want success", e.Result)
	}
	if e.DeviceID != deployDevice {
		t.Fatalf("audit device = %q, want %q", e.DeviceID, deployDevice)
	}
	if e.OrgID != deployOrg {
		t.Fatalf("audit org = %q, want %q", e.OrgID, deployOrg)
	}
	if e.Version != 1 {
		t.Fatalf("audit version = %d, want 1", e.Version)
	}
	if e.UserID == "" {
		t.Fatal("the audit row names no actor")
	}
}

// TestDryRunIsAuditedAsAPreview proves a dry run is recorded with a detail that says it
// stored nothing, so an auditor can tell a preview from an applied change.
func TestDryRunIsAuditedAsAPreview(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	sink := &recordingAudit{}
	f.server.deployHandler.SetAudit(sink)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	w := do(t, r, http.MethodPost, "/v1/deploy?dry_run=true", []byte(deployManifest), nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("dry run status = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	events := sink.snapshot()
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if !strings.Contains(events[0].Detail, "dry run") {
		t.Fatalf("detail = %q, want it to say dry run", events[0].Detail)
	}
}

// TestFailedDeployIsAudited proves a refused deploy is recorded. An attempt to change what
// a device runs that was told no is exactly what an audit trail is for.
func TestFailedDeployIsAudited(t *testing.T) {
	t.Parallel()

	f := newDeployFixture(t)
	f.grantDeploy(t, "devices:"+deployDevice)

	sink := &recordingAudit{}
	f.server.deployHandler.SetAudit(sink)

	r := f.engineFor(memberUser(organization.RoleOperator), deployOrg)

	// An unparseable manifest is refused before the service runs; the handler still
	// records the attempt.
	w := do(t, r, http.MethodPost, "/v1/deploy", []byte("not a manifest"), nil)
	if w.Code == http.StatusAccepted {
		t.Fatalf("an invalid manifest was accepted: %s", w.Body.String())
	}

	events := sink.snapshot()
	if len(events) != 1 || events[0].Action != audit.ActionDeployFailed {
		t.Fatalf("audit events = %+v, want one deploy_failed", events)
	}
	if events[0].Result != audit.ResultFailure {
		t.Fatalf("result = %q, want failure", events[0].Result)
	}
}

// TestDeployAuditAdapterNilLogger proves a nil logger yields a nil adapter and the nil
// adapter's methods are safe to call, so the wiring can skip rather than install a sink
// that silently drops events.
func TestDeployAuditAdapterNilLogger(t *testing.T) {
	t.Parallel()

	if newDeployAuditAdapter(nil) != nil {
		t.Fatal("a nil logger must yield a nil adapter")
	}

	var a *deployAuditAdapter

	a.DeployFromService(context.Background(), deploy.DeployEvent{Action: "deploy_rolled_back"})
	a.Deploy(context.Background(), audit.DeployEvent{Action: audit.ActionDeployApplied})
}
