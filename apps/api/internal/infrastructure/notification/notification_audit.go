package notification

import (
	"context"
	"database/sql"
	"time"
)

// AuditEntry represents a notification audit log entry.
type AuditEntry struct {
	SentAt    time.Time `json:"sentAt"`
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	EventType string    `json:"eventType"`
	Channel   string    `json:"channel"`
	DeviceID  string    `json:"deviceId,omitempty"`
	Payload   string    `json:"payload,omitempty"`
	ErrorMsg  string    `json:"errorMsg,omitempty"`
	Success   bool      `json:"success"`
}

// Repository handles notification audit log operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new notification audit repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// LogEntry logs a notification audit entry.
func (r *Repository) LogEntry(ctx context.Context, entry *AuditEntry) error {
	query := `
		INSERT INTO notification_audit_log (
			id, user_id, event_type, channel, device_id, payload, success, error_msg, sent_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		entry.ID,
		entry.UserID,
		entry.EventType,
		entry.Channel,
		entry.DeviceID,
		entry.Payload,
		entry.Success,
		entry.ErrorMsg,
		entry.SentAt.UnixMilli(),
	)

	return err
}

// GetByUser retrieves audit logs for a user.
func (r *Repository) GetByUser(ctx context.Context, userID string, limit, offset int) ([]*AuditEntry, error) {
	query := `
		SELECT id, user_id, event_type, channel, device_id, payload, success, error_msg, sent_at
		FROM notification_audit_log
		WHERE user_id = ?
		ORDER BY sent_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var entries []*AuditEntry
	for rows.Next() {
		var entry AuditEntry
		var sentAt int64
		var deviceID, payload, errorMsg sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.EventType,
			&entry.Channel,
			&deviceID,
			&payload,
			&entry.Success,
			&errorMsg,
			&sentAt,
		)
		if err != nil {
			return nil, err
		}

		entry.DeviceID = deviceID.String
		entry.Payload = payload.String
		entry.ErrorMsg = errorMsg.String
		entry.SentAt = time.UnixMilli(sentAt)

		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}

// CountByUser returns the number of audit entries for a user, optionally
// narrowed to one channel and/or one outcome (success / failure).
func (r *Repository) CountByUser(ctx context.Context, userID, channel string, success *bool) (int, error) {
	query := `SELECT COUNT(*) FROM notification_audit_log WHERE user_id = ?`
	args := []any{userID}
	if channel != "" {
		query += ` AND channel = ?`
		args = append(args, channel)
	}
	if success != nil {
		query += ` AND success = ?`
		args = append(args, *success)
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountByUserSince returns how many of a user's entries are newer than a
// watermark (Unix milliseconds). It is the unread count for the history view.
func (r *Repository) CountByUserSince(ctx context.Context, userID string, since int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notification_audit_log WHERE user_id = ? AND sent_at > ?`,
		userID, since,
	).Scan(&count)
	return count, err
}

// ListByUser returns a user's audit entries newest first, optionally narrowed to
// one channel and/or one outcome.
func (r *Repository) ListByUser(ctx context.Context, userID, channel string, success *bool, limit, offset int) ([]*AuditEntry, error) {
	query := `
		SELECT id, user_id, event_type, channel, device_id, payload, success, error_msg, sent_at
		FROM notification_audit_log
		WHERE user_id = ?`
	args := []any{userID}
	if channel != "" {
		query += ` AND channel = ?`
		args = append(args, channel)
	}
	if success != nil {
		query += ` AND success = ?`
		args = append(args, *success)
	}
	query += ` ORDER BY sent_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	entries := make([]*AuditEntry, 0, limit)
	for rows.Next() {
		var entry AuditEntry
		var sentAt int64
		var deviceID, payload, errorMsg sql.NullString

		if err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.EventType, &entry.Channel,
			&deviceID, &payload, &entry.Success, &errorMsg, &sentAt,
		); err != nil {
			return nil, err
		}

		entry.DeviceID = deviceID.String
		entry.Payload = payload.String
		entry.ErrorMsg = errorMsg.String
		entry.SentAt = time.UnixMilli(sentAt)
		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}
func (r *Repository) GetByEventType(ctx context.Context, eventType string, limit, offset int) ([]*AuditEntry, error) {
	query := `
		SELECT id, user_id, event_type, channel, device_id, payload, success, error_msg, sent_at
		FROM notification_audit_log
		WHERE event_type = ?
		ORDER BY sent_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, eventType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var entries []*AuditEntry
	for rows.Next() {
		var entry AuditEntry
		var sentAt int64
		var deviceID, payload, errorMsg sql.NullString

		err := rows.Scan(
			&entry.ID,
			&entry.UserID,
			&entry.EventType,
			&entry.Channel,
			&deviceID,
			&payload,
			&entry.Success,
			&errorMsg,
			&sentAt,
		)
		if err != nil {
			return nil, err
		}

		entry.DeviceID = deviceID.String
		entry.Payload = payload.String
		entry.ErrorMsg = errorMsg.String
		entry.SentAt = time.UnixMilli(sentAt)

		entries = append(entries, &entry)
	}

	return entries, rows.Err()
}
