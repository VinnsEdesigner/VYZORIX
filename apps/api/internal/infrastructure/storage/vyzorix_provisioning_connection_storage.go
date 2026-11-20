package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// ConnectionStore is the connection half of the provisioning store. It is a
// separate interface so a caller that only manages connections does not depend
// on the repository half existing yet.
type ConnectionStore interface {
	SaveConnection(ctx context.Context, c *provisioning.Connection) error
	SaveConnectionWithToken(ctx context.Context, c *provisioning.Connection, token redaction.Secret) error
	GetConnection(ctx context.Context, orgID, id string) (*provisioning.Connection, error)
	ListConnections(ctx context.Context, orgID string) ([]*provisioning.Connection, error)
	DeleteConnection(ctx context.Context, orgID, id string) error
	CountRepositoriesUsing(ctx context.Context, orgID, connectionID string) (int, error)
	ConnectionToken(ctx context.Context, orgID, id string) (redaction.Secret, error)
}

var _ ConnectionStore = (*ProvisioningStorage)(nil)

// ProvisioningStorage persists provisioning connections, repositories, owned
// resources, and sync jobs.
type ProvisioningStorage struct {
	db     *sql.DB
	cipher *secretCipher
}

// NewProvisioningStorage creates a new ProvisioningStorage.
func NewProvisioningStorage(db *sql.DB) *ProvisioningStorage {
	return &ProvisioningStorage{db: db}
}

// SetSecretCipher enables encryption at rest for connection tokens. Without it a
// token is stored as-is, which keeps tests and tools that build the repository
// directly working.
func (r *ProvisioningStorage) SetSecretCipher(secret string) {
	if secret != "" {
		r.cipher = newSecretCipher(secret)
	}
}

// ErrNoCipher is returned when a token would be stored without encryption. A
// provisioning token grants access to a repository, so storing it in the clear
// because the server started without a session secret is a silent downgrade.
var ErrNoCipher = errors.New("provisioning: no cipher configured, refusing to store a token")

// encrypt returns the stored form of a token, or ErrNoCipher when no cipher is
// configured.
func (r *ProvisioningStorage) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if r.cipher == nil {
		return "", ErrNoCipher
	}

	return r.cipher.encrypt(plaintext)
}

// decrypt reverses encrypt. It is only called by the sync path, never by a read
// route: no GET returns token material.
func (r *ProvisioningStorage) decrypt(stored string) string {
	if r.cipher == nil {
		return stored
	}

	return r.cipher.decrypt(stored)
}

// ConnectionToken returns the decrypted token for a connection. It exists for the
// sync engine, which must authenticate to the remote; it is deliberately not
// reachable from any HTTP handler.
//
// It is scoped to an organisation even though every current caller already resolved the
// id org-scoped: an unscoped decrypt-by-id is a tenancy hole waiting for the first caller
// that takes an id from a request body. A connection in another org reads as not-found,
// exactly like a connection that does not exist.
//
// The result is a Secret so it cannot be printed: a caller that needs the plaintext
// calls Reveal, which makes every such place visible to a reader and to a grep.
func (r *ProvisioningStorage) ConnectionToken(ctx context.Context, orgID, id string) (redaction.Secret, error) {
	var stored string

	err := r.db.QueryRowContext(ctx,
		`SELECT token_ciphertext FROM provisioning_connections WHERE org_id = ? AND id = ?`,
		orgID, id).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		return "", provisioning.ErrConnectionNotFound
	}
	if err != nil {
		return "", err
	}

	return redaction.NewSecret(r.decrypt(stored)), nil
}

// SaveConnection upserts a connection. The token is passed separately so a caller
// that only renames a connection cannot accidentally blank its secret.
func (r *ProvisioningStorage) SaveConnection(ctx context.Context, c *provisioning.Connection) error {
	return r.SaveConnectionWithToken(ctx, c, "")
}

// SaveConnectionWithToken upserts a connection and, when token is non-empty,
// replaces the stored secret.
func (r *ProvisioningStorage) SaveConnectionWithToken(ctx context.Context, c *provisioning.Connection, token redaction.Secret) error {
	query := `
		INSERT INTO provisioning_connections
			(id, org_id, name, type, host, token_ciphertext, status, last_error,
			checked_at, created_at, updated_at, write_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			host = excluded.host,
			token_ciphertext = CASE
				WHEN excluded.token_ciphertext = '' THEN provisioning_connections.token_ciphertext
				ELSE excluded.token_ciphertext
			END,
			status = excluded.status,
			last_error = excluded.last_error,
			checked_at = excluded.checked_at,
			updated_at = excluded.updated_at,
			write_enabled = excluded.write_enabled
	`
	ciphertext, err := r.encrypt(token.Reveal())
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query,
		c.ID, c.OrgID, c.Name, string(c.Type), c.Host, ciphertext,
		string(c.Status), c.LastError,
		nullTimeMillis(c.CheckedAt), c.CreatedAt.UnixMilli(), c.UpdatedAt.UnixMilli(),
		boolToInt(c.WriteEnabled),
	)

	return err
}

// GetConnection returns one connection. Token material is never included.
func (r *ProvisioningStorage) GetConnection(ctx context.Context, orgID, id string) (*provisioning.Connection, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, type, host, status, last_error, checked_at,
		        created_at, updated_at, write_enabled
		 FROM provisioning_connections WHERE org_id = ? AND id = ?`, orgID, id)

	return scanProvisioningConnection(row)
}

// ListConnections returns an organisation's connections.
func (r *ProvisioningStorage) ListConnections(ctx context.Context, orgID string) ([]*provisioning.Connection, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, name, type, host, status, last_error, checked_at,
		        created_at, updated_at, write_enabled
		 FROM provisioning_connections WHERE org_id = ? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.Connection
	for rows.Next() {
		c, err := scanProvisioningConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}

	return out, rows.Err()
}

// DeleteConnection removes a connection. The foreign key refuses the delete while
// a repository references it; the service maps that to a 409.
func (r *ProvisioningStorage) DeleteConnection(ctx context.Context, orgID, id string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM provisioning_connections WHERE org_id = ? AND id = ?`, orgID, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return provisioning.ErrConnectionNotFound
	}

	return nil
}

// CountRepositoriesUsing reports how many repositories reference a connection, so
// the service can refuse with a message rather than surfacing a raw FK error.
func (r *ProvisioningStorage) CountRepositoriesUsing(ctx context.Context, orgID, connectionID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM provisioning_repositories WHERE org_id = ? AND connection_id = ?`,
		orgID, connectionID).Scan(&n)

	return n, err
}

func scanProvisioningConnection(scanner interface{ Scan(...any) error }) (*provisioning.Connection, error) {
	var c provisioning.Connection
	var typ, status string
	var checkedAt sql.NullInt64
	var createdAt, updatedAt int64
	var writeEnabled int

	err := scanner.Scan(&c.ID, &c.OrgID, &c.Name, &typ, &c.Host, &status,
		&c.LastError, &checkedAt, &createdAt, &updatedAt, &writeEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provisioning.ErrConnectionNotFound
	}
	if err != nil {
		return nil, err
	}

	c.Type = provisioning.ConnectionType(typ)
	c.Status = provisioning.ConnectionStatus(status)
	c.CreatedAt = time.UnixMilli(createdAt)
	c.UpdatedAt = time.UnixMilli(updatedAt)
	c.WriteEnabled = writeEnabled == 1
	if checkedAt.Valid {
		t := time.UnixMilli(checkedAt.Int64)
		c.CheckedAt = &t
	}

	return &c, nil
}
