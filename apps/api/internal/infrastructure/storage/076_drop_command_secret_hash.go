package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateDropCommandSecretHash removes the command secret columns.
//
// They held the HMAC key the old registration flow needed: the server minted a
// shared secret per device and had to deliver it, which it did over FCM — an
// unauthenticated channel. The keypair model replaced it (see
// doc/REGISTRATION_CONTRACT.md), so nothing reads or writes these columns and
// they are dropped rather than left to rot. A dropped column is also a stronger
// statement than an unused one: nothing can quietly start using it again.
//
// SQLite supports DROP COLUMN from 3.35. An older engine reports an error, which
// is caught and ignored: the column is harmless if it remains, and failing
// startup over an unused column would be worse than leaving it.
func migrateDropCommandSecretHash(tx *sql.Tx) error {
	statements := []string{
		// command_secret was the plaintext column the very first flow used;
		// command_secret_hash superseded it, and the keypair model superseded both.
		`ALTER TABLE devices DROP COLUMN command_secret`,
		`ALTER TABLE devices DROP COLUMN command_secret_hash`,
		`ALTER TABLE inbox_requests DROP COLUMN command_secret_hash`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			// No such column, or an engine too old to drop one. Neither is fatal.
			if strings.Contains(err.Error(), "no such column") ||
				strings.Contains(err.Error(), "syntax error") {
				continue
			}

			return err
		}
	}

	return nil
}
