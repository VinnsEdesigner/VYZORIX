package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deploycredential"
)

// DeployCredentialRepository is the SQL persistence for device-scoped deploy
// credentials. Only the token hash is stored; the plaintext is never written.
type DeployCredentialRepository struct {
	db *sql.DB
}

// NewDeployCredentialRepository creates a DeployCredentialRepository.
func NewDeployCredentialRepository(db *sql.DB) *DeployCredentialRepository {
	return &DeployCredentialRepository{db: db}
}

const deployCredentialColumns = `id, org_id, device_id, name, prefix, token_hash, created_by, created_at, last_used_at, revoked_at`

func scanDeployCredential(scanner interface{ Scan(...any) error }) (*deploycredential.Credential, error) {
	var (
		cred       deploycredential.Credential
		createdAt  int64
		lastUsedAt sql.NullInt64
		revokedAt  sql.NullInt64
	)

	if err := scanner.Scan(
		&cred.ID, &cred.OrgID, &cred.DeviceID, &cred.Name, &cred.Prefix, &cred.TokenHash,
		&cred.CreatedBy, &createdAt, &lastUsedAt, &revokedAt,
	); err != nil {
		return nil, err
	}

	cred.CreatedAt = time.UnixMilli(createdAt)
	cred.LastUsedAt = nullTime(lastUsedAt)
	cred.RevokedAt = nullTime(revokedAt)

	return &cred, nil
}

// nullTime turns a nullable millisecond column into a *time.Time, nil when unset.
func nullTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}

	t := time.UnixMilli(v.Int64)

	return &t
}

// Create stores a new credential.
func (r *DeployCredentialRepository) Create(ctx context.Context, credential *deploycredential.Credential) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO device_deploy_credentials (`+deployCredentialColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL)`,
		credential.ID, credential.OrgID, credential.DeviceID, credential.Name,
		credential.Prefix, credential.TokenHash, credential.CreatedBy,
		credential.CreatedAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("create deploy credential: %w", err)
	}

	return nil
}

// FindByHash returns the credential for a token hash, or ErrNotFound.
func (r *DeployCredentialRepository) FindByHash(ctx context.Context, tokenHash string) (*deploycredential.Credential, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+deployCredentialColumns+` FROM device_deploy_credentials WHERE token_hash = ?`,
		tokenHash)

	cred, err := scanDeployCredential(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, deploycredential.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read deploy credential: %w", err)
	}

	return cred, nil
}

// ListForDevice returns a device's credentials. The token hash is not selected:
// a list must not be able to leak credential material even by accident.
func (r *DeployCredentialRepository) ListForDevice(ctx context.Context, deviceID string) ([]*deploycredential.Credential, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, device_id, name, prefix, '', created_by, created_at, last_used_at, revoked_at
		 FROM device_deploy_credentials WHERE device_id = ? ORDER BY created_at DESC`,
		deviceID)
	if err != nil {
		return nil, fmt.Errorf("list deploy credentials: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var credentials []*deploycredential.Credential

	for rows.Next() {
		cred, err := scanDeployCredential(rows)
		if err != nil {
			return nil, fmt.Errorf("scan deploy credential: %w", err)
		}

		credentials = append(credentials, cred)
	}

	return credentials, rows.Err()
}

// Revoke marks a credential revoked. It is scoped to the device, so a caller
// that knows a credential id from another device cannot revoke it.
func (r *DeployCredentialRepository) Revoke(ctx context.Context, deviceID, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE device_deploy_credentials SET revoked_at = ?
		 WHERE id = ? AND device_id = ? AND revoked_at IS NULL`,
		at.UnixMilli(), id, deviceID)
	if err != nil {
		return fmt.Errorf("revoke deploy credential: %w", err)
	}

	return nil
}

// Touch records the last use. A failure here must not fail the request it
// authenticated, so the caller logs and ignores the error.
func (r *DeployCredentialRepository) Touch(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE device_deploy_credentials SET last_used_at = ? WHERE id = ?`,
		at.UnixMilli(), id)
	if err != nil {
		return fmt.Errorf("touch deploy credential: %w", err)
	}

	return nil
}
