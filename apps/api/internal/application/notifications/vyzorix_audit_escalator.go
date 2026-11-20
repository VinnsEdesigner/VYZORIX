package notifications

import (
	"context"
	"fmt"

	notificationdomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/notification"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// AuditEscalator raises an alert when the provisioning audit trail fails to record.
//
// A failed audit write means a change happened and nothing recorded who made it. That
// is a security-relevant gap, so it must reach a person through the same contact points
// an alert rule uses, not sit in a log nobody reads until after the incident.
//
// It is not an alert.Rule: a rule is polled against a metric on a schedule, and this
// must fire on the event, for the organisation that just failed, with no rule to
// configure first.
type AuditEscalator struct {
	dispatcher *Dispatcher
}

// NewAuditEscalator builds an AuditEscalator over the contact-point dispatcher.
func NewAuditEscalator(dispatcher *Dispatcher) *AuditEscalator {
	return &AuditEscalator{dispatcher: dispatcher}
}

// EscalateAuditFailure dispatches the alert to the organisation's enabled contact
// points.
//
// The cause is included because the operator's next question is always "why", and a
// disk-full or locked-database error points at a different fix than a schema fault. It
// is redacted first: a driver error can embed a connection string with its password,
// and this message leaves the process.
//
// Zero deliveries is an error, not a success. The dispatcher reports (0, nil) when the
// organisation has no contact points, and returning nil there would make the escalation
// look handled while reaching nobody — the silent failure this whole path exists to
// prevent.
func (a *AuditEscalator) EscalateAuditFailure(ctx context.Context, orgID string, cause error) error {
	if a == nil || a.dispatcher == nil {
		return fmt.Errorf("no dispatcher configured for audit escalation")
	}

	safeCause := redaction.DefaultRedactor.Redact(cause.Error())

	msg := &notificationdomain.Message{
		Subject: "[critical] Provisioning audit trail is failing",
		Body: fmt.Sprintf(
			"A provisioning change could not be written to the audit trail. "+
				"Changes are still being applied, but who made them is no longer being recorded.\n\n"+
				"Organisation: %s\nCause: %s", orgID, safeCause),
		Event: "audit_failure",
		Data: map[string]string{
			"orgId": orgID,
			"cause": safeCause,
		},
	}

	sent, err := a.dispatcher.Send(ctx, orgID, msg)
	if err != nil {
		return fmt.Errorf("dispatch audit escalation: %w", err)
	}
	if sent == 0 {
		return fmt.Errorf("audit escalation reached no contact point for org %s", orgID)
	}

	return nil
}
