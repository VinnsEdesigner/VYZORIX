package storage

import "database/sql"

// migrateAddAPIKeySigningSecret adds a signing_secret column to api_keys.
// API keys need their own signing secret so API-key-authenticated requests.
// can be HMAC-signed (Domain A: client↔server request signing) without a.
// session. The secret is a deterministic SHA-512 derivation of the full API.
// key value, generated at creation time and stored alongside the key hash.
// Existing keys get an empty signing_secret (signing is opt-in per key until.
// rotated). Idempotent.
func migrateAddAPIKeySigningSecret(tx *sql.Tx) error {
	_, err := tx.Exec(`
		ALTER TABLE api_keys ADD COLUMN signing_secret TEXT NOT NULL DEFAULT ''
	`)
	// The column may already exist on an idempotent re-run. isDuplicateColumnErr
	// matches on a substring, because the error text is driver-dependent: the
	// libSQL/Turso driver prefixes it with "SQLite error: ", so an exact compare
	// against the bare SQLite message misses and fails the migration.
	if err != nil && !isDuplicateColumnErr(err) {
		return err
	}
	return nil
}
