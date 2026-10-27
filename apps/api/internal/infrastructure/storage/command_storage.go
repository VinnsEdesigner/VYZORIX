package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/transaction"
)

// Ensure CommandRepository implements command.Repository.
var _ command.Repository = (*CommandRepository)(nil)

// CommandRepository implements command.Repository using SQLite.
type CommandRepository struct {
	db *sql.DB
}

// NewCommandRepository creates a new CommandRepository.
func NewCommandRepository(db *sql.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

// getQuerier returns the transaction from context if available, otherwise the db.
func (r *CommandRepository) getQuerier(ctx context.Context) Querier {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// queryRow is a helper that uses transaction-aware querier.
func (r *CommandRepository) queryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return r.getQuerier(ctx).QueryRowContext(ctx, query, args...)
}

// queryRows is a helper that uses transaction-aware querier.
func (r *CommandRepository) queryRows(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return r.getQuerier(ctx).QueryContext(ctx, query, args...)
}

// exec is a helper that uses transaction-aware querier.
func (r *CommandRepository) exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return r.getQuerier(ctx).ExecContext(ctx, query, args...)
}

// FindByID retrieves a command by ID.
func (r *CommandRepository) FindByID(ctx context.Context, id string) (*command.Command, error) {
	query := `
		SELECT id, device_id, dispatch_id, command, args, status, 
		       delivered_at, completed_at, created_at, updated_at, result
		FROM commands WHERE id = ?`

	var cmd command.Command

	var argsJSON []byte

	var deliveredAt, completedAt sql.NullInt64

	var result sql.NullString

	err := r.queryRow(ctx, query, id).Scan(
		&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
		&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &result,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, command.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	if argsJSON != nil {
		_ = json.Unmarshal(argsJSON, &cmd.Args)
	}

	if deliveredAt.Valid {
		cmd.DeliveredAt = &deliveredAt.Int64
	}

	if completedAt.Valid {
		cmd.CompletedAt = &completedAt.Int64
	}
	if result.Valid {
		cmd.Result = result.String
	}

	return &cmd, nil
}

// FindByDispatchID retrieves a command by dispatch ID (for idempotency).
func (r *CommandRepository) FindByDispatchID(ctx context.Context, deviceID, dispatchID string) (*command.Command, error) {
	query := `
		SELECT id, device_id, dispatch_id, command, args, status, 
		       delivered_at, completed_at, created_at, updated_at, result
		FROM commands WHERE dispatch_id = ? AND device_id = ?`

	var cmd command.Command

	var argsJSON []byte

	var deliveredAt, completedAt sql.NullInt64

	var result sql.NullString

	err := r.queryRow(ctx, query, dispatchID, deviceID).Scan(
		&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
		&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &result,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, command.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	if argsJSON != nil {
		_ = json.Unmarshal(argsJSON, &cmd.Args)
	}

	if deliveredAt.Valid {
		cmd.DeliveredAt = &deliveredAt.Int64
	}

	if completedAt.Valid {
		cmd.CompletedAt = &completedAt.Int64
	}
	if result.Valid {
		cmd.Result = result.String
	}

	return &cmd, nil
}

// FindByDispatchIDOnly retrieves a command by dispatch ID only (dispatch ID should be globally unique).
func (r *CommandRepository) FindByDispatchIDOnly(ctx context.Context, dispatchID string) (*command.Command, error) {
	query := `
		SELECT id, device_id, dispatch_id, command, args, status, 
		       delivered_at, completed_at, created_at, updated_at, result
		FROM commands WHERE dispatch_id = ?`

	var cmd command.Command

	var argsJSON []byte

	var deliveredAt, completedAt sql.NullInt64

	var result sql.NullString

	err := r.queryRow(ctx, query, dispatchID).Scan(
		&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
		&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &result,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, command.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	if argsJSON != nil {
		_ = json.Unmarshal(argsJSON, &cmd.Args)
	}

	if deliveredAt.Valid {
		cmd.DeliveredAt = &deliveredAt.Int64
	}

	if completedAt.Valid {
		cmd.CompletedAt = &completedAt.Int64
	}
	if result.Valid {
		cmd.Result = result.String
	}

	return &cmd, nil
}

// FindByDeviceID retrieves commands for a device.
func (r *CommandRepository) FindByDeviceID(ctx context.Context, deviceID string, limit int) ([]*command.Command, error) {
	query := `
		SELECT id, device_id, dispatch_id, command, args, status, 
		       delivered_at, completed_at, created_at, updated_at, result
		FROM commands WHERE device_id = ? ORDER BY created_at DESC LIMIT ?`

	rows, err := r.queryRows(ctx, query, deviceID, limit)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var commands []*command.Command

	for rows.Next() {
		var cmd command.Command

		var argsJSON []byte

		var deliveredAt, completedAt sql.NullInt64

		var result sql.NullString

		if err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
			&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &result,
		); err != nil {
			return nil, err
		}

		if argsJSON != nil {
			_ = json.Unmarshal(argsJSON, &cmd.Args)
		}

		if deliveredAt.Valid {
			cmd.DeliveredAt = &deliveredAt.Int64
		}

		if completedAt.Valid {
			cmd.CompletedAt = &completedAt.Int64
		}
		if result.Valid {
			cmd.Result = result.String
		}

		commands = append(commands, &cmd)
	}

	return commands, rows.Err()
}

// Create creates a new command.
func (r *CommandRepository) Create(ctx context.Context, cmd *command.Command) error {
	argsJSON, err := json.Marshal(cmd.Args)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO commands (id, device_id, dispatch_id, command, args, status, created_at, updated_at, retry_count, max_retries)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.exec(ctx, query,
		cmd.ID, cmd.DeviceID, cmd.DispatchID, cmd.Command, argsJSON,
		cmd.Status, cmd.CreatedAt, cmd.UpdatedAt, cmd.RetryCount, cmd.MaxRetries,
	)

	return err
}

// UpdateStatus updates the status of a command.
func (r *CommandRepository) UpdateStatus(ctx context.Context, id string, status command.Status) error {
	now := time.Now()

	var query string

	var args []interface{}

	switch status {
	case command.StatusDelivered:
		query = "UPDATE commands SET status = ?, delivered_at = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now.UnixMilli(), now, id}
	case command.StatusCompleted:
		query = "UPDATE commands SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now.UnixMilli(), now, id}
	case command.StatusFailed:
		query = "UPDATE commands SET status = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now, id}
	case command.StatusPending, command.StatusCancelled:
		// StatusPending and StatusCancelled - no additional timestamp fields.
		query = "UPDATE commands SET status = ?, updated_at = ? WHERE id = ?"
		args = []interface{}{status, now, id}
	}

	result, err := r.exec(ctx, query, args...)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return command.ErrNotFound
	}

	return nil
}

// Delete deletes a command.
func (r *CommandRepository) Delete(ctx context.Context, id string) error {
	result, err := r.exec(ctx, "DELETE FROM commands WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return command.ErrNotFound
	}

	return nil
}

// DeleteByDeviceID deletes all commands for a device.
func (r *CommandRepository) DeleteByDeviceID(ctx context.Context, deviceID string) error {
	_, err := r.exec(ctx, "DELETE FROM commands WHERE device_id = ?", deviceID)
	return err
}

// Update updates a command.
func (r *CommandRepository) Update(ctx context.Context, cmd *command.Command) error {
	query := `
		UPDATE commands SET 
			device_id = ?, dispatch_id = ?, command = ?, args = ?, 
			status = ?, delivered_at = ?, completed_at = ?, updated_at = ?,
			result = ?, failure_reason = ?
		WHERE id = ?`

	argsJSON, err := json.Marshal(cmd.Args)
	if err != nil {
		return err
	}

	_, err = r.exec(ctx, query,
		cmd.DeviceID, cmd.DispatchID, cmd.Command, argsJSON,
		cmd.Status, cmd.DeliveredAt, cmd.CompletedAt, cmd.UpdatedAt,
		cmd.Result, cmd.FailureReason, cmd.ID)

	return err
}

// FindPendingByDeviceID retrieves pending commands for a device.
func (r *CommandRepository) FindPendingByDeviceID(ctx context.Context, deviceID string) ([]*command.Command, error) {
	query := `
		SELECT id, device_id, dispatch_id, command, args, status,
		       delivered_at, completed_at, created_at, updated_at, result
		FROM commands WHERE device_id = ? AND status = 'pending'
		ORDER BY created_at ASC`

	rows, err := r.queryRows(ctx, query, deviceID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var commands []*command.Command

	for rows.Next() {
		var cmd command.Command

		var argsJSON []byte

		var deliveredAt, completedAt sql.NullInt64

		var result sql.NullString

		err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
			&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &result,
		)
		if err != nil {
			return nil, err
		}

		if argsJSON != nil {
			_ = json.Unmarshal(argsJSON, &cmd.Args)
		}

		if deliveredAt.Valid {
			cmd.DeliveredAt = &deliveredAt.Int64
		}

		if completedAt.Valid {
			cmd.CompletedAt = &completedAt.Int64
		}
		if result.Valid {
			cmd.Result = result.String
		}

		commands = append(commands, &cmd)
	}

	return commands, rows.Err()
}

// FindHistoryByDeviceID retrieves paginated command history for a device with time range filtering.
func (r *CommandRepository) FindHistoryByDeviceID(ctx context.Context, deviceID string, status string, startTime, endTime time.Time, limit, offset int) ([]*command.Command, int, error) {
	// Build query with optional status filter.
	baseQuery := `FROM commands WHERE device_id = ? AND created_at >= ? AND created_at <= ?`
	args := []interface{}{deviceID, startTime.UnixMilli(), endTime.UnixMilli()}

	if status != "" && status != "all" {
		baseQuery += ` AND status = ?`
		args = append(args, status)
	}

	// Get total count.
	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	err := r.queryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results - include failure_reason.
	query := `SELECT id, device_id, dispatch_id, command, args, status, delivered_at, completed_at, created_at, updated_at, failure_reason, result ` + baseQuery + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.queryRows(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}

	defer func() { _ = rows.Close() }()

	var commands []*command.Command

	for rows.Next() {
		var cmd command.Command

		var argsJSON []byte

		var deliveredAt, completedAt sql.NullInt64

		var result sql.NullString

		var failureReason sql.NullString

		err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
			&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &failureReason, &result,
		)
		if err != nil {
			return nil, 0, err
		}

		if argsJSON != nil {
			_ = json.Unmarshal(argsJSON, &cmd.Args)
		}

		if deliveredAt.Valid {
			cmd.DeliveredAt = &deliveredAt.Int64
		}

		if completedAt.Valid {
			cmd.CompletedAt = &completedAt.Int64
		}
		if result.Valid {
			cmd.Result = result.String
		}

		if failureReason.Valid {
			cmd.FailureReason = failureReason.String
		}

		commands = append(commands, &cmd)
	}

	return commands, total, rows.Err()
}

// Count returns the total number of commands.
func (r *CommandRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.queryRow(ctx, "SELECT COUNT(*) FROM commands").Scan(&count)

	return count, err
}

// CountPending returns the number of pending commands.
func (r *CommandRepository) CountPending(ctx context.Context) (int, error) {
	var count int
	err := r.queryRow(ctx, "SELECT COUNT(*) FROM commands WHERE status = 'pending'").Scan(&count)

	return count, err
}
func (r *CommandRepository) CountPendingByDevice(ctx context.Context, deviceID string) (int, error) {
	var count int
	err := r.queryRow(ctx, "SELECT COUNT(*) FROM commands WHERE device_id = ? AND status = 'pending'", deviceID).Scan(&count)

	return count, err
}

// CountPendingByDeviceIDs returns the number of pending commands for the given device IDs.
// Used for organization-scoped dashboard stats (org scoping is device-anchored).
func (r *CommandRepository) CountPendingByDeviceIDs(ctx context.Context, deviceIDs []string) (int, error) {
	if len(deviceIDs) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(deviceIDs))
	args := make([]interface{}, len(deviceIDs))
	for i, id := range deviceIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM commands WHERE device_id IN (%s) AND status = 'pending'`, strings.Join(placeholders, ","))

	var count int
	err := r.queryRow(ctx, query, args...).Scan(&count)

	return count, err
}

// MarkWake marks whether a wake command was sent successfully for a command dispatch.
func (r *CommandRepository) MarkWake(ctx context.Context, dispatchID string, errText string) error {
	wakeSent := 1
	if errText != "" {
		wakeSent = 0
	}

	_, err := r.exec(ctx,
		`UPDATE commands SET wake_sent = ?, failure_reason = ? WHERE dispatch_id = ?`,
		wakeSent, errText, dispatchID,
	)

	return err
}

// MarkDelivered marks a command as delivered by dispatch ID.
func (r *CommandRepository) MarkDelivered(ctx context.Context, dispatchID string) error {
	now := time.Now()
	_, err := r.exec(ctx,
		`UPDATE commands SET status = ?, delivered_at = ?, updated_at = ? WHERE dispatch_id = ?`,
		command.StatusDelivered, now.UnixMilli(), now, dispatchID,
	)

	return err
}

// MarkCompleted marks a command as completed by dispatch ID with result.
// The result is the device-reported output (e.g. stdout); failure_reason is
// cleared because a completed command did not fail.
func (r *CommandRepository) MarkCompleted(ctx context.Context, dispatchID, result string) error {
	now := time.Now()
	res, err := r.exec(ctx,
		`UPDATE commands SET status = ?, completed_at = ?, updated_at = ?, result = ?, failure_reason = NULL WHERE dispatch_id = ?`,
		command.StatusCompleted, now.UnixMilli(), now, result, dispatchID,
	)
	if err != nil {
		return err
	}

	return requireRowsAffected(res, command.ErrNotFound)
}

// MarkFailed marks a command as failed by dispatch ID with error message.
func (r *CommandRepository) MarkFailed(ctx context.Context, dispatchID, errMsg string) error {
	now := time.Now()
	res, err := r.exec(ctx,
		`UPDATE commands SET status = ?, completed_at = ?, updated_at = ?, failure_reason = ? WHERE dispatch_id = ?`,
		command.StatusFailed, now.UnixMilli(), now, errMsg, dispatchID,
	)
	if err != nil {
		return err
	}

	return requireRowsAffected(res, command.ErrNotFound)
}

// requireRowsAffected turns a zero-row update into err, so callers cannot
// mistake a no-op (unknown dispatch ID) for a successful state transition.
func requireRowsAffected(res sql.Result, err error) error {
	rows, rerr := res.RowsAffected()
	if rerr != nil {
		return rerr
	}
	if rows == 0 {
		return err
	}
	return nil
}

// DeleteOldCommands removes commands older than the given timestamp.
func (r *CommandRepository) DeleteOldCommands(ctx context.Context, olderThan int64) (int64, error) {
	result, err := r.exec(ctx,
		`DELETE FROM commands WHERE created_at < ?`,
		olderThan,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// FindByDispatchPrefix retrieves all commands whose dispatch_id starts with the given prefix.
func (r *CommandRepository) FindByDispatchPrefix(ctx context.Context, prefix string) ([]*command.Command, error) {
	query := `
		SELECT id, device_id, dispatch_id, command, args, status,
		       delivered_at, completed_at, created_at, updated_at, result
		FROM commands WHERE dispatch_id LIKE ? || '%'
		ORDER BY created_at ASC`

	rows, err := r.queryRows(ctx, query, prefix)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var commands []*command.Command

	for rows.Next() {
		var cmd command.Command
		var argsJSON []byte
		var deliveredAt, completedAt sql.NullInt64

		var result sql.NullString

		err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
			&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &result,
		)
		if err != nil {
			return nil, err
		}

		if argsJSON != nil {
			_ = json.Unmarshal(argsJSON, &cmd.Args)
		}

		if deliveredAt.Valid {
			cmd.DeliveredAt = &deliveredAt.Int64
		}

		if completedAt.Valid {
			cmd.CompletedAt = &completedAt.Int64
		}
		if result.Valid {
			cmd.Result = result.String
		}

		commands = append(commands, &cmd)
	}

	return commands, rows.Err()
}

// CancelByDispatchPrefix marks all pending commands whose dispatch_id starts with the given prefix as cancelled.
func (r *CommandRepository) CancelByDispatchPrefix(ctx context.Context, prefix string) (int64, error) {
	now := time.Now()
	result, err := r.exec(ctx,
		`UPDATE commands SET status = ?, updated_at = ? WHERE dispatch_id LIKE ? || '%' AND status = 'pending'`,
		command.StatusCancelled, now, prefix,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// FindPending retrieves pending commands for the outbox worker.
// Returns commands with status=pending where next_retry_at is null or in the past,.
// ordered by creation time (oldest first).
func (r *CommandRepository) FindPending(ctx context.Context, limit int) ([]*command.Command, error) {
	now := time.Now().UnixMilli()
	query := `
		SELECT id, device_id, dispatch_id, command, args, status,
		       delivered_at, completed_at, created_at, updated_at, failure_reason, result,
		       retry_count, max_retries, next_retry_at
		FROM commands
		WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?`

	rows, err := r.queryRows(ctx, query, now, limit)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var commands []*command.Command

	for rows.Next() {
		var cmd command.Command
		var argsJSON []byte
		var deliveredAt, completedAt sql.NullInt64

		var result sql.NullString
		var failureReason sql.NullString
		var nextRetryAt sql.NullInt64

		err := rows.Scan(
			&cmd.ID, &cmd.DeviceID, &cmd.DispatchID, &cmd.Command, &argsJSON,
			&cmd.Status, &deliveredAt, &completedAt, &cmd.CreatedAt, &cmd.UpdatedAt, &failureReason, &result, &cmd.RetryCount, &cmd.MaxRetries, &nextRetryAt,
		)
		if err != nil {
			return nil, err
		}

		if argsJSON != nil {
			_ = json.Unmarshal(argsJSON, &cmd.Args)
		}

		if deliveredAt.Valid {
			cmd.DeliveredAt = &deliveredAt.Int64
		}

		if completedAt.Valid {
			cmd.CompletedAt = &completedAt.Int64
		}
		if result.Valid {
			cmd.Result = result.String
		}

		if failureReason.Valid {
			cmd.FailureReason = failureReason.String
		}

		if nextRetryAt.Valid {
			t := time.UnixMilli(nextRetryAt.Int64)
			cmd.NextRetryAt = &t
		}

		commands = append(commands, &cmd)
	}

	return commands, rows.Err()
}

// UpdateRetryInfo updates the retry tracking fields for a command.
func (r *CommandRepository) UpdateRetryInfo(ctx context.Context, id string, retryCount int, maxRetries int, nextRetryAt *time.Time) error {
	now := time.Now()

	var query string
	var args []interface{}

	if nextRetryAt != nil {
		query = `UPDATE commands SET retry_count = ?, max_retries = ?, next_retry_at = ?, updated_at = ? WHERE id = ?`
		args = []interface{}{retryCount, maxRetries, nextRetryAt.UnixMilli(), now, id}
	} else {
		query = `UPDATE commands SET retry_count = ?, max_retries = ?, next_retry_at = NULL, updated_at = ? WHERE id = ?`
		args = []interface{}{retryCount, maxRetries, now, id}
	}

	result, err := r.exec(ctx, query, args...)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return command.ErrNotFound
	}

	return nil
}

// DeleteByDeviceIDs deletes all commands for the given device IDs.
// This is used during organization deletion.
func (r *CommandRepository) DeleteByDeviceIDs(ctx context.Context, deviceIDs []string) (int64, error) {
	if len(deviceIDs) == 0 {
		return 0, nil
	}

	// Build query with placeholders for each device ID.
	placeholders := make([]string, len(deviceIDs))
	args := make([]interface{}, len(deviceIDs))
	for i, id := range deviceIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`DELETE FROM commands WHERE device_id IN (%s)`, strings.Join(placeholders, ","))

	result, err := r.exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
