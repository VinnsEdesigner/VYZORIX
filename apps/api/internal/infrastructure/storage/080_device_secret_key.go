package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateAddDeviceSecretKey adds the device's second public key: the
// PURPOSE_DECRYPT RSA key a secret is wrapped to.
//
// It is separate from `public_key`, which is the identity key — an EC P-256
// signing key whose public half is handed to the container. A secret must not
// rest on a key the whole system already knows, so the envelope uses its own key
// .
//
// Nullable: a device that enrolled before this migration, or one that does not
// support secrets, has no value, and setting a secret for it fails rather than
// wrapping to nothing.
func migrateAddDeviceSecretKey(tx *sql.Tx) error {
	statements := []string{
		`ALTER TABLE devices ADD COLUMN secret_public_key TEXT`,
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(context.Background(), stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}

			return err
		}
	}

	return nil
}
