package push

import "context"

// Subscription is one browser's Web Push subscription, as produced by
// PushSubscription.toJSON() in the browser.
type Subscription struct {
	Endpoint  string
	UserID    string
	P256dh    string
	Auth      string
	UserAgent string
	CreatedAt int64
}

// Repository stores browser push subscriptions.
type Repository interface {
	// Upsert stores a subscription, replacing the keys for an endpoint that
	// already exists (a browser re-subscribing) and re-owning it to userID.
	Upsert(ctx context.Context, sub *Subscription) error

	// ListByUser returns every subscription a user has registered.
	ListByUser(ctx context.Context, userID string) ([]*Subscription, error)

	// DeleteByEndpoint removes a single subscription (the user disabling this
	// browser, or a push service reporting it gone).
	DeleteByEndpoint(ctx context.Context, endpoint string) error
}
