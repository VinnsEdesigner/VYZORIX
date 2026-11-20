package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
)

// DeployConfigRepository is the SQL persistence for per-device signed configs.
type DeployConfigRepository struct {
	db *sql.DB
}

// NewDeployConfigRepository creates a DeployConfigRepository.
func NewDeployConfigRepository(db *sql.DB) *DeployConfigRepository {
	return &DeployConfigRepository{db: db}
}

const deployConfigColumns = `device_id, kind, version, document, created_at, applied_at`

func scanDeployConfig(scanner interface{ Scan(...any) error }) (*deployconfig.Record, error) {
	var (
		rec       deployconfig.Record
		kind      string
		createdAt int64
		appliedAt int64
	)

	if err := scanner.Scan(&rec.DeviceID, &kind, &rec.Version, &rec.Document, &createdAt, &appliedAt); err != nil {
		return nil, err
	}

	rec.Kind = deployconfig.Kind(kind)
	rec.CreatedAt = time.UnixMilli(createdAt)

	if appliedAt > 0 {
		rec.AppliedAt = time.UnixMilli(appliedAt)
	}

	return &rec, nil
}

// Current returns the config the device should be running, scoped to an organisation.
//
// The org is enforced by the query, not by the caller remembering to check: the join
// against devices means a device in another tenant reads as ErrNotFound, exactly like one
// that does not exist. The config table has no org column of its own — the org is a
// property of the device — so the join is what makes tenancy a property of the read.
func (r *DeployConfigRepository) Current(ctx context.Context, orgID, deviceID string) (*deployconfig.Record, error) {
	return r.get(ctx, orgID, deviceID, deployconfig.KindCurrent)
}

// LastGood returns the config that last applied, scoped to an organisation.
func (r *DeployConfigRepository) LastGood(ctx context.Context, orgID, deviceID string) (*deployconfig.Record, error) {
	return r.get(ctx, orgID, deviceID, deployconfig.KindLastGood)
}

// CurrentForDevice returns the config the device should be running, keyed by device id
// alone.
//
// This is the device's own read: the device signed the request with its own key, so the
// device is the authenticated principal and the id in the path is the id that was proven.
// There is no org to check against and no cross-tenant risk — a device can only read its
// own row. It is a separate method from Current so a user-facing caller cannot reach it by
// accident: every user path goes through Current and its org join.
func (r *DeployConfigRepository) CurrentForDevice(ctx context.Context, deviceID string) (*deployconfig.Record, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+deployConfigColumns+` FROM device_deploy_configs WHERE device_id = ? AND kind = ?`,
		deviceID, string(deployconfig.KindCurrent))

	rec, err := scanDeployConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, deployconfig.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read current config for %s: %w", deviceID, err)
	}

	return rec, nil
}

// get reads one retained config by kind, refusing a device outside the organisation.
func (r *DeployConfigRepository) get(ctx context.Context, orgID, deviceID string, kind deployconfig.Kind) (*deployconfig.Record, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+prefixedColumns(deployConfigColumns, "c")+`
		 FROM device_deploy_configs c
		 JOIN devices d ON d.id = c.device_id
		 WHERE c.device_id = ? AND c.kind = ? AND d.organization_id = ?`,
		deviceID, string(kind), orgID)

	rec, err := scanDeployConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, deployconfig.ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("read %s config for %s: %w", kind, deviceID, err)
	}

	return rec, nil
}

// prefixedColumns qualifies a bare column list with a table alias, so the join's
// columns cannot be ambiguous.
func prefixedColumns(columns, alias string) string {
	parts := strings.Split(columns, ",")

	for i, part := range parts {
		parts[i] = alias + "." + strings.TrimSpace(part)
	}

	return strings.Join(parts, ", ")
}

// SaveCurrent stores a new current config. In one transaction it demotes the
// previous current to last-good, drops any older last-good, and writes the new
// current, so retention stays at two and never leaves two last-goods behind.
func (r *DeployConfigRepository) SaveCurrent(ctx context.Context, record *deployconfig.Record) error {
	if record.Kind != deployconfig.KindCurrent {
		return fmt.Errorf("SaveCurrent requires kind %q, got %q", deployconfig.KindCurrent, record.Kind)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin deploy config transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if err := demoteCurrentToLastGood(ctx, tx, record.DeviceID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO device_deploy_configs (`+deployConfigColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (device_id, kind) DO UPDATE SET
		   version = excluded.version,
		   document = excluded.document,
		   created_at = excluded.created_at,
		   applied_at = excluded.applied_at`,
		record.DeviceID, string(record.Kind), record.Version, record.Document,
		record.CreatedAt.UnixMilli(), appliedAtMillis(record.AppliedAt),
	); err != nil {
		return fmt.Errorf("write current config for %s: %w", record.DeviceID, err)
	}

	return tx.Commit()
}

// demoteCurrentToLastGood replaces the last-good row with the current one, so
// the previous current becomes the recovery point and any older one is gone.
func demoteCurrentToLastGood(ctx context.Context, tx *sql.Tx, deviceID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO device_deploy_configs (`+deployConfigColumns+`)
		 SELECT device_id, ?, version, document, created_at, applied_at
		 FROM device_deploy_configs
		 WHERE device_id = ? AND kind = ?
		 ON CONFLICT (device_id, kind) DO UPDATE SET
		   version = excluded.version,
		   document = excluded.document,
		   created_at = excluded.created_at,
		   applied_at = excluded.applied_at`,
		string(deployconfig.KindLastGood), deviceID, string(deployconfig.KindCurrent))
	if err != nil {
		return fmt.Errorf("demote current to last-good for %s: %w", deviceID, err)
	}

	return nil
}

// MarkApplied records the version the device reported as applied.
func (r *DeployConfigRepository) MarkApplied(ctx context.Context, deviceID string, version int, at time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE device_deploy_configs SET applied_at = ?
		 WHERE device_id = ? AND kind = ? AND version = ?`,
		at.UnixMilli(), deviceID, string(deployconfig.KindCurrent), version)
	if err != nil {
		return fmt.Errorf("mark config applied for %s: %w", deviceID, err)
	}

	return nil
}

// appliedAtMillis is 0 for the zero time, which the column reads as "not yet".
func appliedAtMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}

	return t.UnixMilli()
}
