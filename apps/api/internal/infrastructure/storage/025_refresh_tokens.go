package storage

import (
	"context"
	"database/sql"
)

// migrateCreateRefreshTokens creates the refresh_tokens table.
func migrateCreateRefreshTokens(tx *sql.Tx) error {
	_, err := tx.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id              TEXT PRIMARY KEY,
			token_hash      TEXT NOT NULL UNIQUE,
			user_id     TEXT NOT NULL,
			session_id      TEXT NOT NULL,
			expires_at      INTEGER NOT NULL,
			created_at      INTEGER NOT NULL,
			replaced_by_id  TEXT,
			is_revoked      INTEGER NOT NULL DEFAULT 0,
			revoked_at      INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(session_id) REFERENCES auth_sessions(id) ON DELETE CASCADE,
			FOREIGN KEY(replaced_by_id) REFERENCES refresh_tokens(id)
		)
	`)
	if err != nil {
		return err
	}

	// Create index for token hash lookups.
	_, err = tx.ExecContext(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash 
		ON refresh_tokens(token_hash)
	`)
	if err != nil {
		return err
	}

	// Create index for querying tokens by user.
	_, err = tx.ExecContext(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id 
		ON refresh_tokens(user_id)
	`)
	if err != nil {
		return err
	}

	// Create index for querying tokens by session.
	_, err = tx.ExecContext(context.Background(), `
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_session_id 
		ON refresh_tokens(session_id)
	`)

	return err
}
