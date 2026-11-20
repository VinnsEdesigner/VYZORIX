package storage

import (
	"context"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// EventStore is the provisioning audit trail.
type EventStore interface {
	Record(ctx context.Context, event *provisioning.ProvisioningEvent) error
	ListForOrg(ctx context.Context, orgID string, limit int) ([]*provisioning.ProvisioningEvent, error)
	ListForResource(ctx context.Context, orgID, resourceType, resourceID string, limit int) ([]*provisioning.ProvisioningEvent, error)
}

var _ EventStore = (*ProvisioningStorage)(nil)

const provisioningEventColumns = `id, org_id, actor_kind, actor_id, actor_label, type,
        resource_type, resource_id, result, detail, created_at`

// Record appends one provisioning event.
//
// It refuses an event with no actor: an unattributed provisioning change is exactly
// what this table exists to prevent, so a missing actor is a caller bug worth
// surfacing rather than a row worth storing with an empty column.
func (r *ProvisioningStorage) Record(ctx context.Context, event *provisioning.ProvisioningEvent) error {
	if event.Actor.Kind == "" || !event.Actor.Kind.Valid() {
		return provisioning.ErrUnknownActor
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO provisioning_events
                        (id, org_id, actor_kind, actor_id, actor_label, type,
                         resource_type, resource_id, result, detail, created_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.OrgID, string(event.Actor.Kind), event.Actor.ID, event.Actor.Label,
		string(event.Type), event.ResourceType, event.ResourceID, event.Result,
		event.Detail, event.CreatedAt.UnixMilli(),
	)

	return err
}

// ListForOrg returns an organisation's events, newest first.
func (r *ProvisioningStorage) ListForOrg(ctx context.Context, orgID string, limit int) ([]*provisioning.ProvisioningEvent, error) {
	return r.listEvents(ctx,
		`SELECT `+provisioningEventColumns+`
                 FROM provisioning_events WHERE org_id = ?
                 ORDER BY created_at DESC LIMIT ?`, orgID, eventLimit(limit))
}

// ListForResource returns events about one resource, newest first.
func (r *ProvisioningStorage) ListForResource(ctx context.Context, orgID, resourceType, resourceID string, limit int) ([]*provisioning.ProvisioningEvent, error) {
	return r.listEvents(ctx,
		`SELECT `+provisioningEventColumns+`
                 FROM provisioning_events
                 WHERE org_id = ? AND resource_type = ? AND resource_id = ?
                 ORDER BY created_at DESC LIMIT ?`,
		orgID, resourceType, resourceID, eventLimit(limit))
}

// listEvents runs a query and scans the rows.
func (r *ProvisioningStorage) listEvents(ctx context.Context, query string, args ...any) ([]*provisioning.ProvisioningEvent, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.ProvisioningEvent

	for rows.Next() {
		var event provisioning.ProvisioningEvent
		var kind, typ string
		var createdAt int64

		if err := rows.Scan(&event.ID, &event.OrgID, &kind, &event.Actor.ID, &event.Actor.Label,
			&typ, &event.ResourceType, &event.ResourceID, &event.Result, &event.Detail,
			&createdAt); err != nil {
			return nil, err
		}

		event.Actor.Kind = provisioning.ActorKind(kind)
		event.Type = provisioning.EventType(typ)
		event.CreatedAt = time.UnixMilli(createdAt)
		out = append(out, &event)
	}

	return out, rows.Err()
}

// eventLimit bounds a listing so a caller cannot ask for the whole table.
func eventLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 50
	}

	return limit
}
