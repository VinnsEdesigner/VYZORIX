package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateConsolidateUserSchema brings a database created by an older release
// onto the current person schema: the person table is `users`, the settings
// table is `user_settings`, and every foreign key to the person is `user_id`.
//
// A fresh database already creates the current names (the CREATE TABLE
// statements in the earlier migrations use them), so this is a no-op there; it
// exists for a database created before the consolidation. SQLite has no
// "RENAME COLUMN IF EXISTS", so each rename is attempted and a missing
// table/column is tolerated — the ledger runs each migration once, and an older
// database may already be partly consolidated.
//
// The `role` column is untouched: its values name a role, not the person.
func migrateConsolidateUserSchema(tx *sql.Tx) error {
	ctx := context.Background()

	// 1. The person table and its settings table.
	if err := renameTableIfExists(ctx, tx, "operators", "users"); err != nil {
		return err
	}
	if err := renameTableIfExists(ctx, tx, "operator_settings", "user_settings"); err != nil {
		return err
	}

	// 2. Every table that carries a foreign key to the person. The list is the set
	// of tables that referenced it at the time of the consolidation; a table
	// absent from an older database is skipped.
	renames := []struct{ table, oldCol, newCol string }{
		{"auth_sessions", "operator_id", "user_id"},
		{"refresh_tokens", "operator_id", "user_id"},
		{"api_keys", "operator_id", "user_id"},
		{"api_clients", "operator_id", "user_id"},
		{"devices", "operator_id", "user_id"},
		{"commands", "operator_id", "user_id"},
		{"command_confirmations", "operator_id", "user_id"},
		{"audit_logs", "operator_id", "user_id"},
		{"events", "operator_id", "user_id"},
		{"device_logs", "operator_id", "user_id"},
		{"device_events", "operator_id", "user_id"},
		{"device_settings", "operator_id", "user_id"},
		{"organization_members", "operator_id", "user_id"},
		{"invitations", "operator_id", "user_id"},
		{"permission_grants", "operator_id", "user_id"},
		{"resource_permissions", "operator_id", "user_id"},
		{"known_devices", "operator_id", "user_id"},
		{"known_device_signals", "operator_id", "user_id"},
		{"account_lockouts", "operator_id", "user_id"},
		{"failed_login_attempts", "operator_id", "user_id"},
		{"email_verifications", "operator_id", "user_id"},
		{"password_reset_tokens", "operator_id", "user_id"},
		{"session_revocations", "operator_id", "user_id"},
		{"notification_audit_log", "operator_id", "user_id"},
		{"notification_deliveries", "operator_id", "user_id"},
		{"registration_logs", "operator_id", "user_id"},
		{"inbox_requests", "operator_id", "user_id"},
		{"config_versions", "operator_id", "user_id"},
		{"device_deploy_configs", "operator_id", "user_id"},
		{"device_deploy_credentials", "operator_id", "user_id"},
		{"annotations", "operator_id", "user_id"},
		{"update_pushes", "operator_id", "user_id"},
		{"alert_events", "operator_id", "user_id"},
		{"idempotency_records", "operator_id", "user_id"},
		{"oauth_states", "operator_id", "user_id"},
		{"message_queue", "operator_id", "user_id"},
	}
	for _, r := range renames {
		if err := renameColumnIfExists(ctx, tx, r.table, r.oldCol, r.newCol); err != nil {
			return err
		}
	}

	// 3. Non-`_id` person columns on the person-adjacent tables.
	for _, r := range []struct{ table, oldCol, newCol string }{
		{"users", "operator_name", "user_name"},
		{"users", "operator_email", "user_email"},
		{"organization_members", "operator_name", "user_name"},
		{"organization_members", "operator_email", "user_email"},
	} {
		if err := renameColumnIfExists(ctx, tx, r.table, r.oldCol, r.newCol); err != nil {
			return err
		}
	}

	return nil
}

func renameTableIfExists(ctx context.Context, tx *sql.Tx, oldName, newName string) error {
	var exists int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, oldName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check table %s: %w", oldName, err)
	}
	if exists == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldName, newName)); err != nil {
		return fmt.Errorf("rename table %s -> %s: %w", oldName, newName, err)
	}
	return nil
}

func renameColumnIfExists(ctx context.Context, tx *sql.Tx, table, oldCol, newCol string) error {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil // Table absent.
	}
	defer func() { _ = rows.Close() }()
	hasOld := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info(%s): %w", table, err)
		}
		if name == oldCol {
			hasOld = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !hasOld {
		return nil
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, oldCol, newCol)); err != nil {
		return fmt.Errorf("rename column %s.%s -> %s: %w", table, oldCol, newCol, err)
	}
	return nil
}
