package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	notificationdomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/notification"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/metrics"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/uuid"
)

// Dispatcher routes notifications to contact points via their channel,
// records delivery attempts, and emits metrics.
type Dispatcher struct {
	repo       notificationdomain.Repository
	deliveries notificationdomain.DeliveryRepository
	channels   map[notificationdomain.ChannelType]notificationdomain.Channel
	logger     *slog.Logger
}

// NewDispatcher creates a Dispatcher.
func NewDispatcher(repo notificationdomain.Repository, deliveries notificationdomain.DeliveryRepository, channels map[notificationdomain.ChannelType]notificationdomain.Channel, logger *slog.Logger) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{repo: repo, deliveries: deliveries, channels: channels, logger: logger}
}

// Send dispatches a message to every enabled contact point of an org.
// Returns the number of successful sends; individual failures are logged
// but do not stop other contact points from receiving the message.
func (d *Dispatcher) Send(ctx context.Context, orgID string, msg *notificationdomain.Message) (int, error) {
	if err := msg.Validate(); err != nil {
		return 0, fmt.Errorf("notification message invalid: %w", err)
	}
	points, err := d.repo.ListEnabledByOrg(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("list contact points: %w", err)
	}
	if len(points) == 0 {
		return 0, nil
	}

	succeeded := 0
	for _, cp := range points {
		if err := d.dispatchOne(ctx, cp, msg); err != nil {
			d.logger.Error("notification delivery failed",
				"contact_point_id", cp.ID, "channel", cp.Channel, "error", err)
			continue
		}
		succeeded++
	}
	return succeeded, nil
}

// SendToPoint dispatches a message to a specific contact point (test button).
func (d *Dispatcher) SendToPoint(ctx context.Context, cp *notificationdomain.ContactPoint, msg *notificationdomain.Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("notification message invalid: %w", err)
	}
	return d.dispatchOne(ctx, cp, msg)
}

// sanitizeMessage returns a copy of the message with its free-text fields redacted.
//
// HTML is scrubbed as text: a credential pasted into a body is usually inside a tag or
// an attribute, and the text patterns match either way. Data is redacted by key, so a
// value filed under "token" is masked regardless of its shape.
func sanitizeMessage(msg *notificationdomain.Message) *notificationdomain.Message {
	if msg == nil {
		return nil
	}

	r := redaction.DefaultRedactor

	out := *msg
	out.Subject = r.Redact(msg.Subject)
	out.Body = r.Redact(msg.Body)
	out.HTML = r.Redact(msg.HTML)
	out.Data = r.RedactMapKey(msg.Data)

	return &out
}

// dispatchOne delivers one message through one contact point.
//
// The message is redacted first: it leaves the process, so a credential that reached a
// subject or body — an alert whose text quotes a failing connection string, a test
// message echoing a user-supplied URL — would be handed to a third party. Sanitizing
// here covers Send and SendToPoint alike, since both route through this function.
func (d *Dispatcher) dispatchOne(ctx context.Context, cp *notificationdomain.ContactPoint, msg *notificationdomain.Message) error {
	channel, ok := d.channels[cp.Channel]
	if !ok {
		return fmt.Errorf("no channel registered for %q", cp.Channel)
	}

	msg = sanitizeMessage(msg)
	err := channel.Send(ctx, cp, msg)
	status := "sent"
	errStr := ""
	if err != nil {
		status = "failed"
		errStr = err.Error()
		metrics.Get().RecordNotificationDelivery(string(cp.Channel), "failed")
	} else {
		metrics.Get().RecordNotificationDelivery(string(cp.Channel), "sent")
	}

	delivery := &notificationdomain.Delivery{
		ID:             uuid.New(),
		ContactPointID: cp.ID,
		Channel:        cp.Channel,
		Status:         status,
		Error:          errStr,
		Message:        msg,
		CreatedAt:      time.Now(),
	}
	if d.deliveries != nil {
		if appendErr := d.deliveries.Append(ctx, delivery); appendErr != nil {
			d.logger.Error("delivery record append failed", "error", appendErr)
		}
	}
	return err
}
