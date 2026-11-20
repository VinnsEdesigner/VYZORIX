package provisioning

import (
	"context"
	"log/slog"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/google/uuid"
)

// Auditor writes provisioning events and escalates when it cannot.
//
// Every audit write goes through here rather than calling the store directly, so the
// escalation is not something each call site has to remember. A write that fails is
// logged and escalated; the caller still gets the error, because a caller that
// considers the audit part of its contract may want to act on it.
type Auditor struct {
	store     domain.EventRepository
	escalator domain.AuditEscalator
	log       *slog.Logger
}

// NewAuditor builds an Auditor. A nil store disables recording: the caller gets a
// no-op rather than a nil check at every site.
func NewAuditor(store domain.EventRepository, escalator domain.AuditEscalator, log *slog.Logger) *Auditor {
	if log == nil {
		log = slog.Default()
	}

	return &Auditor{store: store, escalator: escalator, log: log}
}

// Audit records an event for a resource, defaulting the actor when none was supplied.
//
// It is the convenience the CRUD services call: they have the actor from the request
// and the resource they just touched, and nothing else worth threading. The event id
// and timestamp are filled here so no call site can forget them.
func (a *Auditor) Audit(ctx context.Context, actor domain.Actor, typ domain.EventType,
	orgID, resourceType, resourceID, result, detail string) {
	if a == nil || a.store == nil {
		return
	}

	event := &domain.ProvisioningEvent{
		ID:           uuid.New().String(),
		OrgID:        orgID,
		Actor:        resolveActor(actor),
		Type:         typ,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       result,
		Detail:       detail,
		CreatedAt:    time.Now(),
	}

	// The error is already escalated inside Record; the caller here is a mutation that
	// has completed, so it has nothing to do with it.
	_ = a.Record(ctx, event)
}

// SetEscalator attaches the alert path. It is a setter because the notification
// dispatcher is built after the handler set; until it is attached a failed audit write
// is logged with an explicit "visible only in logs" warning.
func (a *Auditor) SetEscalator(e domain.AuditEscalator) {
	if a == nil {
		return
	}
	a.escalator = e
}

// Record writes one event and escalates if the write fails.
//
// A failure is escalated exactly once per write: the escalation is the alert, so
// retrying it in a loop would turn one broken audit row into an alert storm.
func (a *Auditor) Record(ctx context.Context, event *domain.ProvisioningEvent) error {
	if a == nil || a.store == nil {
		return nil
	}

	err := a.store.Record(ctx, event)
	if err == nil {
		return nil
	}

	a.log.Error("provisioning: audit write failed",
		"org_id", event.OrgID, "type", event.Type, "resource_id", event.ResourceID, "err", err)

	a.escalate(ctx, event.OrgID, err)

	return err
}

// escalationTimeout bounds the out-of-band alert. It matches the audit logger's own
// bound: long enough for a webhook or Slack call, short enough not to pin a worker.
const escalationTimeout = 5 * time.Second

// escalate raises the out-of-band alert. A failure to escalate is logged but not
// returned: the caller is already being told the audit write failed, and stacking a
// second error would only obscure the first.
//
// The alert does NOT inherit the caller's context. The commonest reason an audit write
// fails mid-request is that the request was cancelled — the client disconnected, or the
// server is shutting down — and a cancelled context would abort the HTTP/Slack send
// too, losing the alert at exactly the moment it is needed. A detached, bounded context
// gives the alert its own chance to land.
func (a *Auditor) escalate(ctx context.Context, orgID string, cause error) {
	if a.escalator == nil {
		a.log.Error("provisioning: audit write failed and no escalator is configured; "+
			"this failure is visible only in logs", "org_id", orgID)

		return
	}

	alertCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), escalationTimeout)
	defer cancel()

	if err := a.escalator.EscalateAuditFailure(alertCtx, orgID, cause); err != nil {
		a.log.Error("provisioning: could not escalate audit failure",
			"org_id", orgID, "err", err)
	}
}
