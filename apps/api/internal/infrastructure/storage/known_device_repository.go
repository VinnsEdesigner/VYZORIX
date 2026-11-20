package storage

import (
	"context"
	"database/sql"
	"time"

	devicestore "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/devicestore"
)

// KnownDeviceRepository is the SQLite persistence for known-device tracking used by
// new-device login notifications. It implements auth.DeviceStoreBackend.
//
// Both tables are bounded here, in SQL, so the storage cannot grow without limit
// regardless of how often a client changes its IP or User-Agent:
//   - at most maxKnownDevicesPerUser rows per user, evicting the
//     least-recently-seen device, and
//   - at most maxSignalsPerDevice signals per (device, kind), evicting the oldest.
type KnownDeviceRepository struct {
	db *sql.DB
}

// Bounds mirror the in-memory store's caps.
const (
	maxKnownDevicesPerUser = 50
	maxSignalsPerDevice    = 20
)

// NewKnownDeviceRepository creates a new KnownDeviceRepository.
func NewKnownDeviceRepository(db *sql.DB) *KnownDeviceRepository {
	return &KnownDeviceRepository{db: db}
}

// IsKnownDevice reports whether the user has a device with this fingerprint.
func (r *KnownDeviceRepository) IsKnownDevice(ctx context.Context, userID, fingerprint string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM known_devices WHERE user_id = ? AND fingerprint = ? LIMIT 1`,
		userID, fingerprint).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// HasSignal reports whether the device has been seen from this value of kind.
func (r *KnownDeviceRepository) HasSignal(ctx context.Context, userID, fingerprint, kind, value string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM known_device_signals
		  WHERE user_id = ? AND fingerprint = ? AND kind = ? AND value = ? LIMIT 1`,
		userID, fingerprint, kind, value).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// TouchDevice updates the device's last-seen timestamp.
func (r *KnownDeviceRepository) TouchDevice(ctx context.Context, userID, fingerprint string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE known_devices SET last_seen_at = ? WHERE user_id = ? AND fingerprint = ?`,
		time.Now().UnixMilli(), userID, fingerprint)
	return err
}

// RegisterDevice records the device and the observed IP / User-Agent, creating the
// device row if needed and applying the per-user and per-device bounds.
func (r *KnownDeviceRepository) RegisterDevice(ctx context.Context, userID, fingerprint, ipAddress, userAgent string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UnixMilli()

	var exists int
	err = tx.QueryRowContext(ctx,
		`SELECT 1 FROM known_devices WHERE user_id = ? AND fingerprint = ? LIMIT 1`,
		userID, fingerprint).Scan(&exists)
	switch {
	case err == sql.ErrNoRows:
		// Bound the per-user device count before inserting a new one.
		err = r.evictOldestDeviceIfFull(ctx, tx, userID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO known_devices (user_id, fingerprint, first_seen_at, last_seen_at, is_trusted)
			 VALUES (?, ?, ?, ?, 1)`,
			userID, fingerprint, now, now)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		_, err = tx.ExecContext(ctx,
			`UPDATE known_devices SET last_seen_at = ? WHERE user_id = ? AND fingerprint = ?`,
			now, userID, fingerprint)
		if err != nil {
			return err
		}
	}

	if err := r.recordSignal(ctx, tx, userID, fingerprint, devicestore.KindIP, ipAddress, now); err != nil {
		return err
	}
	if err := r.recordSignal(ctx, tx, userID, fingerprint, devicestore.KindUserAgent, userAgent, now); err != nil {
		return err
	}

	return tx.Commit()
}

// recordSignal inserts a signal, evicting the oldest for that (device, kind) when
// the per-device cap is reached.
func (r *KnownDeviceRepository) recordSignal(ctx context.Context, tx *sql.Tx, userID, fingerprint, kind, value string, now int64) error {
	if value == "" {
		return nil
	}

	var exists int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM known_device_signals
		  WHERE user_id = ? AND fingerprint = ? AND kind = ? AND value = ? LIMIT 1`,
		userID, fingerprint, kind, value).Scan(&exists)
	if err == nil {
		// Already recorded.
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}

	var count int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM known_device_signals
		  WHERE user_id = ? AND fingerprint = ? AND kind = ?`,
		userID, fingerprint, kind).Scan(&count)
	if err != nil {
		return err
	}

	if count >= maxSignalsPerDevice {
		// Evict the oldest signal of this kind.
		_, err = tx.ExecContext(ctx,
			`DELETE FROM known_device_signals
			  WHERE rowid IN (
				SELECT rowid FROM known_device_signals
				 WHERE user_id = ? AND fingerprint = ? AND kind = ?
				 ORDER BY seen_at ASC LIMIT 1
			  )`,
			userID, fingerprint, kind)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO known_device_signals (user_id, fingerprint, kind, value, seen_at)
		 VALUES (?, ?, ?, ?, ?)`,
		userID, fingerprint, kind, value, now)
	return err
}

// evictOldestDeviceIfFull drops the least-recently-seen device for the user when
// the per-user cap has been reached.
func (r *KnownDeviceRepository) evictOldestDeviceIfFull(ctx context.Context, tx *sql.Tx, userID string) error {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM known_devices WHERE user_id = ?`, userID).Scan(&count)
	if err != nil {
		return err
	}
	if count < maxKnownDevicesPerUser {
		return nil
	}

	var oldest string
	err = tx.QueryRowContext(ctx,
		`SELECT fingerprint FROM known_devices WHERE user_id = ?
		  ORDER BY last_seen_at ASC LIMIT 1`, userID).Scan(&oldest)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM known_devices WHERE user_id = ? AND fingerprint = ?`,
		userID, oldest)
	if err != nil {
		return err
	}
	// SQLite does not enforce ON DELETE CASCADE unless foreign_keys is on, so drop
	// the signals explicitly.
	_, err = tx.ExecContext(ctx,
		`DELETE FROM known_device_signals WHERE user_id = ? AND fingerprint = ?`,
		userID, oldest)
	return err
}

// Compile-time check that the repository satisfies the auth contract.
var _ devicestore.Store = (*KnownDeviceRepository)(nil)
