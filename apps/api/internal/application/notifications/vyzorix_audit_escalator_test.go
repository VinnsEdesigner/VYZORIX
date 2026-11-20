package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"

	notificationdomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/notification"
)

// stubPointRepo is the contact-point repository the dispatcher reads.
type stubPointRepo struct {
	points []*notificationdomain.ContactPoint
}

func (r *stubPointRepo) Save(context.Context, *notificationdomain.ContactPoint) error { return nil }

func (r *stubPointRepo) GetByID(context.Context, string) (*notificationdomain.ContactPoint, error) {
	return nil, nil
}

func (r *stubPointRepo) ListByOrg(context.Context, string) ([]*notificationdomain.ContactPoint, error) {
	return r.points, nil
}

func (r *stubPointRepo) ListEnabledByOrg(context.Context, string) ([]*notificationdomain.ContactPoint, error) {
	return r.points, nil
}

func (r *stubPointRepo) Delete(context.Context, string) (bool, error) { return true, nil }

// capturingChannel records what was delivered.
type capturingChannel struct {
	messages []*notificationdomain.Message
	err      error
}

func (c *capturingChannel) Send(_ context.Context, _ *notificationdomain.ContactPoint, msg *notificationdomain.Message) error {
	if c.err != nil {
		return c.err
	}
	c.messages = append(c.messages, msg)

	return nil
}

// TestEscalatorReachingNobodyIsAnError is the guard against a silent escalation. The
// dispatcher reports success with zero deliveries when an organisation has no contact
// points, and an escalation that reaches nobody must not look handled.
func TestEscalatorReachingNobodyIsAnError(t *testing.T) {
	dispatcher := NewDispatcher(&stubPointRepo{points: nil}, nil,
		map[notificationdomain.ChannelType]notificationdomain.Channel{}, nil)

	escalator := NewAuditEscalator(dispatcher)

	err := escalator.EscalateAuditFailure(context.Background(), "org-1", errors.New("boom"))
	if err == nil {
		t.Fatal("an escalation that reached no contact point reported success")
	}
	if !strings.Contains(err.Error(), "no contact point") {
		t.Fatalf("error = %v, want it to name the delivery gap", err)
	}
}

// TestEscalatorRedactsTheCause proves a driver error carrying a connection string does
// not put its password into a message that leaves the process.
func TestEscalatorRedactsTheCause(t *testing.T) {
	channel := &capturingChannel{}
	dispatcher := NewDispatcher(
		&stubPointRepo{points: []*notificationdomain.ContactPoint{
			{ID: "cp-1", OrgID: "org-1", Channel: notificationdomain.ChannelTypeWebhook, Enabled: true},
		}},
		nil,
		map[notificationdomain.ChannelType]notificationdomain.Channel{notificationdomain.ChannelTypeWebhook: channel},
		nil,
	)

	escalator := NewAuditEscalator(dispatcher)

	// A libsql connection string with its token, the shape a driver error can carry.
	cause := errors.New(`failed to open: libsql://vyzorix.turso.io?authToken=supersecrettokenvalue`)
	if err := escalator.EscalateAuditFailure(context.Background(), "org-1", cause); err != nil {
		t.Fatalf("EscalateAuditFailure: %v", err)
	}

	if len(channel.messages) != 1 {
		t.Fatalf("delivered %d messages, want 1", len(channel.messages))
	}

	body := channel.messages[0].Body + channel.messages[0].Data["cause"]
	if strings.Contains(body, "supersecrettokenvalue") {
		t.Fatalf("the credential survived redaction: %s", body)
	}
	if !strings.Contains(body, "libsql") {
		t.Fatalf("redaction destroyed the whole cause, leaving nothing diagnosable: %s", body)
	}
}
