package storage

import (
	"context"
	"database/sql"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/push"
)

// Ensure PushSubscriptionRepository implements push.Repository.
var _ push.Repository = (*PushSubscriptionRepository)(nil)

// PushSubscriptionRepository stores browser Web Push subscriptions in SQLite.
type PushSubscriptionRepository struct {
	db *sql.DB
}

// NewPushSubscriptionRepository creates a new PushSubscriptionRepository.
func NewPushSubscriptionRepository(db *sql.DB) *PushSubscriptionRepository {
	return &PushSubscriptionRepository{db: db}
}

// Upsert stores a subscription, keyed by endpoint so a browser re-subscribing
// replaces its keys instead of duplicating the row.
func (r *PushSubscriptionRepository) Upsert(ctx context.Context, sub *push.Subscription) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
			user_id = excluded.user_id,
			p256dh = excluded.p256dh,
			auth = excluded.auth,
			user_agent = excluded.user_agent
	`, sub.Endpoint, sub.UserID, sub.P256dh, sub.Auth, nullString(sub.UserAgent), sub.CreatedAt)
	return err
}

// ListByUser returns every subscription a user has registered.
func (r *PushSubscriptionRepository) ListByUser(ctx context.Context, userID string) ([]*push.Subscription, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT endpoint, user_id, p256dh, auth, user_agent, created_at
		FROM push_subscriptions WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var subs []*push.Subscription
	for rows.Next() {
		var s push.Subscription
		var userAgent sql.NullString
		if err := rows.Scan(&s.Endpoint, &s.UserID, &s.P256dh, &s.Auth, &userAgent, &s.CreatedAt); err != nil {
			return nil, err
		}
		s.UserAgent = userAgent.String
		subs = append(subs, &s)
	}

	return subs, rows.Err()
}

// DeleteByEndpoint removes a single subscription.
func (r *PushSubscriptionRepository) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}
