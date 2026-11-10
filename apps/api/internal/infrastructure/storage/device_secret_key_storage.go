package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

// DeviceSecretKeyRepository resolves the public key a secret is wrapped to: the
// device's second key, `secret_public_key`.
//
// It is separate from the identity key, whose public half is handed to the
// container. Wrapping a secret to a key the whole system knows would make the
// container able to read it.
type DeviceSecretKeyRepository struct {
	db *sql.DB
}

// NewDeviceSecretKeyRepository creates a DeviceSecretKeyRepository.
func NewDeviceSecretKeyRepository(db *sql.DB) *DeviceSecretKeyRepository {
	return &DeviceSecretKeyRepository{db: db}
}

// SecretPublicKey returns the device's secret public key, or an error naming the
// device when it has none. An empty key is a refusal, not an empty string: a
// caller must not be able to wrap a secret to nothing.
func (r *DeviceSecretKeyRepository) SecretPublicKey(ctx context.Context, deviceID string) (string, error) {
	var key sql.NullString

	err := r.db.QueryRowContext(ctx,
		`SELECT secret_public_key FROM devices WHERE id = ?`, deviceID).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: %s", device.ErrNotFound, deviceID)
	}

	if err != nil {
		return "", fmt.Errorf("read secret key for %s: %w", deviceID, err)
	}

	if key.String == "" {
		return "", fmt.Errorf("device %s has no secret key registered", deviceID)
	}

	return key.String, nil
}
