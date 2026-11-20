package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/appsecret"
)

// AppSecretRepository is the SQL persistence for per-device app secrets.
type AppSecretRepository struct {
	db *sql.DB
}

// NewAppSecretRepository creates an AppSecretRepository.
func NewAppSecretRepository(db *sql.DB) *AppSecretRepository {
	return &AppSecretRepository{db: db}
}

const appSecretColumns = `device_id, app, key, ciphertext, generation, updated_at`

func scanAppSecret(scanner interface{ Scan(...any) error }) (*appsecret.Record, error) {
	var (
		rec       appsecret.Record
		updatedAt int64
	)

	if err := scanner.Scan(&rec.DeviceID, &rec.App, &rec.Key, &rec.Ciphertext, &rec.Generation, &updatedAt); err != nil {
		return nil, err
	}

	rec.UpdatedAt = time.UnixMilli(updatedAt)

	return &rec, nil
}

// Get returns one secret, or ErrNotFound.
func (r *AppSecretRepository) Get(ctx context.Context, name appsecret.Name) (*appsecret.Record, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+appSecretColumns+` FROM device_app_secrets
		 WHERE device_id = ? AND app = ? AND key = ?`,
		name.DeviceID, name.App, name.Key)

	rec, err := scanAppSecret(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, appsecret.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read app secret: %w", err)
	}

	return rec, nil
}

// List returns names and generations for an app, without ciphertext. The
// user confirms what is set; the value is never returned by a list.
func (r *AppSecretRepository) List(ctx context.Context, deviceID, app string) ([]*appsecret.Record, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT device_id, app, key, '', generation, updated_at FROM device_app_secrets
		 WHERE device_id = ? AND app = ? ORDER BY key`,
		deviceID, app)
	if err != nil {
		return nil, fmt.Errorf("list app secrets: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var records []*appsecret.Record

	for rows.Next() {
		rec, err := scanAppSecret(rows)
		if err != nil {
			return nil, fmt.Errorf("scan app secret: %w", err)
		}

		records = append(records, rec)
	}

	return records, rows.Err()
}

// Put stores or replaces a secret and returns its new generation. A replacement
// bumps the generation so a device that holds the old value re-fetches.
func (r *AppSecretRepository) Put(ctx context.Context, record *appsecret.Record) (int64, error) {
	// The generation is computed in SQL rather than read-then-write, so two
	// concurrent sets cannot both read the same value and write the same next.
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO device_app_secrets (`+appSecretColumns+`)
		 VALUES (?, ?, ?, ?, 1, ?)
		 ON CONFLICT (device_id, app, key) DO UPDATE SET
		   ciphertext = excluded.ciphertext,
		   generation = device_app_secrets.generation + 1,
		   updated_at = excluded.updated_at
		 RETURNING generation`,
		record.DeviceID, record.App, record.Key, record.Ciphertext, record.UpdatedAt.UnixMilli())

	var generation int64
	if err := row.Scan(&generation); err != nil {
		return 0, fmt.Errorf("put app secret: %w", err)
	}

	return generation, nil
}

// Delete removes a secret.
//
// It reports ErrNotFound when no row matched, so a delete for a key that was never set
// is not answered with a 204 that tells the operator a stale secret was cleared when
// nothing changed. This is the same rule the deploy credential's Revoke follows.
func (r *AppSecretRepository) Delete(ctx context.Context, name appsecret.Name) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM device_app_secrets WHERE device_id = ? AND app = ? AND key = ?`,
		name.DeviceID, name.App, name.Key)
	if err != nil {
		return fmt.Errorf("delete app secret: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete app secret: %w", err)
	}
	if n == 0 {
		return appsecret.ErrNotFound
	}

	return nil
}

// GenerationForApp returns the highest generation across an app's secrets, or 0
// when the app has none.
func (r *AppSecretRepository) GenerationForApp(ctx context.Context, deviceID, app string) (int64, error) {
	var generation sql.NullInt64

	err := r.db.QueryRowContext(ctx,
		`SELECT MAX(generation) FROM device_app_secrets WHERE device_id = ? AND app = ?`,
		deviceID, app).Scan(&generation)
	if err != nil {
		return 0, fmt.Errorf("read app secret generation: %w", err)
	}

	return generation.Int64, nil
}
