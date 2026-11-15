package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/transaction"
)

// Querier is an interface that both *sql.DB and *sql.Tx implement.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// getQuerier returns the transaction from context if available, otherwise the db.
func (r *DeviceRepository) getQuerier(ctx context.Context) Querier {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// queryRow is a helper that uses transaction-aware querier.
func (r *DeviceRepository) queryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return r.getQuerier(ctx).QueryRowContext(ctx, query, args...)
}

// queryRows is a helper that uses transaction-aware querier.
func (r *DeviceRepository) queryRows(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return r.getQuerier(ctx).QueryContext(ctx, query, args...)
}

// exec is a helper that uses transaction-aware querier.
func (r *DeviceRepository) exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return r.getQuerier(ctx).ExecContext(ctx, query, args...)
}

// Ensure DeviceRepository implements device.Repository.
var _ device.Repository = (*DeviceRepository)(nil)

// DeviceRepository implements device.Repository using SQLite.
type DeviceRepository struct {
	db *sql.DB
}

// NewDeviceRepository creates a new DeviceRepository.
func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

// deviceColumns returns the list of columns to select.
const deviceColumns = `
	id, firebase_install_id, fcm_token, app_version, device_class,
	online, registered_at, last_seen, user_id,
	organization_id,
	created_at, updated_at, device_name, manufacturer, model, os_version, security_patch,
	build_id, api_level, supported_abis, ram_bytes, storage_bytes,
	deregistered_at, deletion_scheduled_at, fcm_token_refreshed_at, tags, tier,
	public_key, secret_public_key, pairing_code, pairing_expires_at,
	reported_config_version, reported_app_digests, reported_at
`

// applyReportedState copies the reported-state columns onto a device.
//
// Shared by the single-row and list scans so the two cannot drift apart — the same
// reason the columns are declared once. A NULL version leaves the pointer nil, which
// is "does not report" and stays distinguishable from a reported 0.
func applyReportedState(
	d *device.Device,
	configVersion sql.NullInt64,
	appDigests sql.NullString,
	reportedAt sql.NullInt64,
) {
	if configVersion.Valid {
		v := int(configVersion.Int64)
		d.ReportedConfigVersion = &v
	}
	if appDigests.Valid && appDigests.String != "" {
		_ = json.Unmarshal([]byte(appDigests.String), &d.ReportedAppDigests)
	}
	d.ReportedAt = reportedAt.Int64
}

// scanDevice scans a device from a row.
func scanDevice(row *sql.Row) (*device.Device, error) {
	var d device.Device
	var fcmToken, appVersion, deviceClass, userID, organizationID, deviceName, manufacturer, model, osVersion, securityPatch, tagsJSON sql.NullString
	var buildID, supportedABIs sql.NullString
	var publicKey, secretPublicKey, pairingCode sql.NullString
	var deregisteredAt, deletionScheduledAt, fcmTokenRefreshedAt, tier, pairingExpiresAt sql.NullInt64
	var apiLevel, ramBytes, storageBytes sql.NullInt64
	// What the device reports it is running. reported_config_version is nullable so
	// "does not report" (NULL) stays distinct from "reports 0".
	var reportedConfigVersion, reportedAt sql.NullInt64
	var reportedAppDigests sql.NullString
	// created_at/updated_at are declared TIMESTAMP but the driver hands back whatever
	// was written, which is an ISO8601 string. Scanning them straight into time.Time
	// fails at runtime with "unsupported Scan, storing driver.Value type string into
	// type *time.Time", so they are read as a raw value and converted, the way
	// user_storage already does.
	var createdAt, updatedAt any

	err := row.Scan(
		&d.ID, &d.FirebaseInstallID, &fcmToken, &appVersion, &deviceClass,
		&d.Online, &d.RegisteredAt, &d.LastSeen, &userID,
		&organizationID,
		&createdAt, &updatedAt, &deviceName, &manufacturer, &model, &osVersion, &securityPatch,
		&buildID, &apiLevel, &supportedABIs, &ramBytes, &storageBytes,
		&deregisteredAt, &deletionScheduledAt, &fcmTokenRefreshedAt, &tagsJSON, &tier,
		&publicKey, &secretPublicKey, &pairingCode, &pairingExpiresAt,
		&reportedConfigVersion, &reportedAppDigests, &reportedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, device.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	d.AppVersion = appVersion.String
	d.DeviceClass = deviceClass.String
	d.CreatedAt = parseTimestamp(createdAt)
	d.UpdatedAt = parseTimestamp(updatedAt)
	d.Tier = device.TierFromInt(int(tier.Int64))
	d.PublicKey = publicKey.String
	d.SecretPublicKey = secretPublicKey.String
	d.PairingCode = pairingCode.String
	if pairingExpiresAt.Valid {
		d.PairingExpiresAt = &pairingExpiresAt.Int64
	}
	d.FCMToken = fcmToken.String
	d.UserID = userID.String
	d.OrganizationID = organizationID.String
	d.DeviceName = deviceName.String
	d.Manufacturer = manufacturer.String
	d.Model = model.String
	d.OSVersion = osVersion.String
	d.SecurityPatch = securityPatch.String
	d.BuildID = buildID.String
	d.SupportedABIs = supportedABIs.String
	d.APIVersion = int(apiLevel.Int64)
	d.RAMBytes = ramBytes.Int64
	d.StorageBytes = storageBytes.Int64
	if tagsJSON.Valid && tagsJSON.String != "" {
		_ = json.Unmarshal([]byte(tagsJSON.String), &d.Tags)
	}
	applyReportedState(&d, reportedConfigVersion, reportedAppDigests, reportedAt)

	if deregisteredAt.Valid {
		d.DeregisteredAt = &deregisteredAt.Int64
		d.Lifecycle = device.LifecycleDeregistered
	} else if d.RegisteredAt > 0 {
		d.Lifecycle = device.LifecycleRegistered
	} else {
		d.Lifecycle = device.LifecyclePending
	}

	if deletionScheduledAt.Valid {
		d.DeletionScheduledAt = &deletionScheduledAt.Int64
	}
	if fcmTokenRefreshedAt.Valid {
		d.FCMTokenRefreshedAt = &fcmTokenRefreshedAt.Int64
	}

	return &d, nil
}

// scanDevices scans multiple devices from rows.
func scanDevices(rows *sql.Rows) ([]*device.Device, error) {
	var devices []*device.Device

	for rows.Next() {
		var d device.Device
		var fcmToken, appVersion, deviceClass, userID, organizationID, deviceName, manufacturer, model, osVersion, securityPatch, tagsJSON sql.NullString
		var buildID, supportedABIs sql.NullString
		var publicKey, secretPublicKey, pairingCode sql.NullString
		var deregisteredAt, deletionScheduledAt, fcmTokenRefreshedAt, tier, pairingExpiresAt sql.NullInt64
		var apiLevel, ramBytes, storageBytes sql.NullInt64
		var reportedConfigVersion, reportedAt sql.NullInt64
		var reportedAppDigests sql.NullString
		var createdAt, updatedAt any

		if err := rows.Scan(
			&d.ID, &d.FirebaseInstallID, &fcmToken, &appVersion, &deviceClass,
			&d.Online, &d.RegisteredAt, &d.LastSeen, &userID,
			&organizationID,
			&createdAt, &updatedAt, &deviceName, &manufacturer, &model, &osVersion, &securityPatch,
			&buildID, &apiLevel, &supportedABIs, &ramBytes, &storageBytes,
			&deregisteredAt, &deletionScheduledAt, &fcmTokenRefreshedAt, &tagsJSON, &tier,
			&publicKey, &secretPublicKey, &pairingCode, &pairingExpiresAt,
			&reportedConfigVersion, &reportedAppDigests, &reportedAt,
		); err != nil {
			return nil, err
		}

		d.AppVersion = appVersion.String
		d.DeviceClass = deviceClass.String
		d.BuildID = buildID.String
		d.SupportedABIs = supportedABIs.String
		d.APIVersion = int(apiLevel.Int64)
		d.RAMBytes = ramBytes.Int64
		d.StorageBytes = storageBytes.Int64
		d.CreatedAt = parseTimestamp(createdAt)
		d.UpdatedAt = parseTimestamp(updatedAt)
		d.Tier = device.TierFromInt(int(tier.Int64))
		d.PublicKey = publicKey.String
		d.SecretPublicKey = secretPublicKey.String
		d.PairingCode = pairingCode.String
		if pairingExpiresAt.Valid {
			d.PairingExpiresAt = &pairingExpiresAt.Int64
		}
		d.FCMToken = fcmToken.String
		d.UserID = userID.String
		d.OrganizationID = organizationID.String
		d.DeviceName = deviceName.String
		d.Manufacturer = manufacturer.String
		d.Model = model.String
		d.OSVersion = osVersion.String
		d.SecurityPatch = securityPatch.String
		if tagsJSON.Valid && tagsJSON.String != "" {
			_ = json.Unmarshal([]byte(tagsJSON.String), &d.Tags)
		}
		applyReportedState(&d, reportedConfigVersion, reportedAppDigests, reportedAt)

		if deregisteredAt.Valid {
			d.DeregisteredAt = &deregisteredAt.Int64
			d.Lifecycle = device.LifecycleDeregistered
		} else if d.RegisteredAt > 0 {
			d.Lifecycle = device.LifecycleRegistered
		} else {
			d.Lifecycle = device.LifecyclePending
		}

		if deletionScheduledAt.Valid {
			d.DeletionScheduledAt = &deletionScheduledAt.Int64
		}
		if fcmTokenRefreshedAt.Valid {
			d.FCMTokenRefreshedAt = &fcmTokenRefreshedAt.Int64
		}

		devices = append(devices, &d)
	}

	return devices, rows.Err()
}

// FindByID retrieves a device by ID.
func (r *DeviceRepository) FindByID(ctx context.Context, id string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE id = ?`
	return scanDevice(r.queryRow(ctx, query, id))
}

// FindByIMEI retrieves a device by IMEI.
func (r *DeviceRepository) FindByIMEI(ctx context.Context, imei string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE id = ?`
	return scanDevice(r.queryRow(ctx, query, imei))
}

// FindByFirebaseInstallID retrieves a device by Firebase install ID.
func (r *DeviceRepository) FindByFirebaseInstallID(ctx context.Context, fid string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE firebase_install_id = ?`
	return scanDevice(r.queryRow(ctx, query, fid))
}

// FindByIDAndUser retrieves a device by ID and verifies it belongs to the user.
// This implements DOA (Data Ownership Attribution) checks for infraauth.
// Returns ErrNotFound if device doesn't exist OR doesn't belong to the user.
func (r *DeviceRepository) FindByIDAndUser(ctx context.Context, id, userID string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE id = ? AND user_id = ?`
	return scanDevice(r.queryRow(ctx, query, id, userID))
}

// FindByIMEIAndUser retrieves a device by IMEI and verifies it belongs to the user.
// This implements DOA (Data Ownership Attribution) checks for deregistration.
// Returns ErrNotFound if device doesn't exist OR doesn't belong to the user.
func (r *DeviceRepository) FindByIMEIAndUser(ctx context.Context, imei, userID string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE id = ? AND user_id = ?`
	return scanDevice(r.queryRow(ctx, query, imei, userID))
}

// FindByIMEIAndOrganization retrieves a device by IMEI within an organization.
// Returns ErrNotFound if device doesn't exist OR doesn't belong to the organization.
func (r *DeviceRepository) FindByIMEIAndOrganization(ctx context.Context, imei, orgID string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE id = ? AND organization_id = ?`
	return scanDevice(r.queryRow(ctx, query, imei, orgID))
}

// FindByIDAndOrganization retrieves a device by ID within an organization.
// Returns ErrNotFound if device doesn't exist OR doesn't belong to the organization.
func (r *DeviceRepository) FindByIDAndOrganization(ctx context.Context, id, orgID string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE id = ? AND organization_id = ?`

	return scanDevice(r.queryRow(ctx, query, id, orgID))
}

// FindForOrg is FindByIDAndOrganization under the name the deploy predicate's
// DeviceLookup uses. It is the same org-scoped query, so a device in another
// tenant is ErrNotFound and the predicate reads that as not-in-org.
func (r *DeviceRepository) FindForOrg(ctx context.Context, id, orgID string) (*device.Device, error) {
	return r.FindByIDAndOrganization(ctx, id, orgID)
}

// FindByPairingCode looks a device up by the code a user types to claim it.
//
// It matches only a live code: an expired one is not returned, so a user
// cannot claim a device whose notification went stale. The caller still checks
// expiry, but refusing here means the common case fails before any state change.
func (r *DeviceRepository) FindByPairingCode(ctx context.Context, code string) (*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices
	          WHERE pairing_code = ? AND pairing_code != ''
	            AND (pairing_expires_at IS NULL OR pairing_expires_at > ?)`

	return scanDevice(r.queryRow(ctx, query, code, time.Now().UnixMilli()))
}

// ClearPairingCode consumes a device's code, so it cannot be claimed twice.
func (r *DeviceRepository) ClearPairingCode(ctx context.Context, id string) error {
	result, err := r.exec(ctx,
		`UPDATE devices SET pairing_code = '', pairing_expires_at = NULL, updated_at = ? WHERE id = ?`,
		time.Now(), id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// Create creates a new device.
func (r *DeviceRepository) Create(ctx context.Context, d *device.Device) error {
	query := `
		INSERT INTO devices (id, firebase_install_id, fcm_token, app_version, device_class,
		                    online, registered_at, last_seen, user_id,
		                    organization_id,
		                    created_at, updated_at, device_name, manufacturer, model, os_version,
		                    security_patch, build_id, api_level, supported_abis, ram_bytes, storage_bytes,
		                    tags, tier,
		                    public_key, secret_public_key, pairing_code, pairing_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.exec(ctx, query,
		d.ID, d.FirebaseInstallID, nullString(d.FCMToken), d.AppVersion, d.DeviceClass,
		d.Online, d.RegisteredAt, d.LastSeen, nullString(d.UserID),
		nullString(d.OrganizationID),
		d.CreatedAt, d.UpdatedAt, nullString(d.DeviceName), nullString(d.Manufacturer),
		nullString(d.Model), nullString(d.OSVersion),
		nullString(d.SecurityPatch), nullString(d.BuildID), d.APIVersion,
		nullString(d.SupportedABIs), d.RAMBytes, d.StorageBytes,
		tagsToJSON(d.Tags), int(d.Tier),
		nullString(d.PublicKey), nullString(d.SecretPublicKey), nullString(d.PairingCode), nullInt(d.PairingExpiresAt),
	)

	return err
}

// Update updates an existing device.
//
// `tags` and `tier` are in the SET list. tags was missing before: the argument
// list already passed tagsToJSON(d.Tags), so the statement had one more argument
// than placeholders and every Update failed. tier is added with the same fix.
func (r *DeviceRepository) Update(ctx context.Context, d *device.Device) error {
	query := `
		UPDATE devices 
		SET firebase_install_id = ?, fcm_token = ?, app_version = ?, device_class = ?,
		    online = ?, registered_at = ?, last_seen = ?,
		    user_id = ?, organization_id = ?, updated_at = ?, device_name = ?, manufacturer = ?,
		    model = ?, os_version = ?, security_patch = ?, build_id = ?, api_level = ?,
		    supported_abis = ?, ram_bytes = ?, storage_bytes = ?,
		    tags = ?, tier = ?,
		    public_key = ?, secret_public_key = ?, pairing_code = ?, pairing_expires_at = ?
		WHERE id = ?`

	result, err := r.exec(ctx, query,
		d.FirebaseInstallID, nullString(d.FCMToken), d.AppVersion, d.DeviceClass,
		d.Online, d.RegisteredAt, d.LastSeen, nullString(d.UserID),
		nullString(d.OrganizationID), time.Now(), nullString(d.DeviceName), nullString(d.Manufacturer),
		nullString(d.Model), nullString(d.OSVersion),
		nullString(d.SecurityPatch), nullString(d.BuildID), d.APIVersion,
		nullString(d.SupportedABIs), d.RAMBytes, d.StorageBytes,
		tagsToJSON(d.Tags), int(d.Tier),
		nullString(d.PublicKey), nullString(d.SecretPublicKey), nullString(d.PairingCode), nullInt(d.PairingExpiresAt), d.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// Delete deletes a device.
func (r *DeviceRepository) Delete(ctx context.Context, id string) error {
	result, err := r.exec(ctx, "DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// UpdateFCMToken updates the FCM token for a device.
// UpdateAppVersion records the app version a device reports it is running.
//
// The version is otherwise only written at enroll, so an OTA update leaves the row
// stale. Telemetry carries the running version, and this keeps the record current
// without a re-enroll.
func (r *DeviceRepository) UpdateAppVersion(ctx context.Context, id, appVersion string) error {
	if appVersion == "" {
		return nil
	}

	_, err := r.exec(ctx,
		"UPDATE devices SET app_version = ?, updated_at = ? WHERE id = ?",
		appVersion, time.Now(), id,
	)

	return err
}

// UpdateReportedState records what the device says it is actually running.
//
// The device reports this on every telemetry heartbeat, so the comparison against
// the stored config can happen on read rather than only at apply time. A nil
// configVersion is stored as NULL, which is how "this device does not report state"
// stays distinguishable from "it reports holding no config".
func (r *DeviceRepository) UpdateReportedState(
	ctx context.Context,
	id string,
	configVersion *int,
	appDigestsJSON string,
	reportedAt int64,
) error {
	_, err := r.exec(ctx,
		`UPDATE devices
		    SET reported_config_version = ?, reported_app_digests = ?, reported_at = ?, updated_at = ?
		  WHERE id = ?`,
		configVersion, appDigestsJSON, reportedAt, time.Now(), id,
	)

	return err
}

func (r *DeviceRepository) UpdateFCMToken(ctx context.Context, id, fcmToken string) error {
	result, err := r.exec(ctx,
		"UPDATE devices SET fcm_token = ?, updated_at = ? WHERE id = ?",
		fcmToken, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// SetOnline sets the online status of a device.
// When coming online, clears deletion_scheduled_at to cancel scheduled deletion.
func (r *DeviceRepository) SetOnline(ctx context.Context, id string, online bool) error {
	now := time.Now().UnixMilli()

	var result sql.Result

	var err error

	if online {
		// Coming online cancels scheduled deletion.
		result, err = r.exec(ctx,
			"UPDATE devices SET online = ?, last_seen = ?, updated_at = ?, deletion_scheduled_at = 0 WHERE id = ?",
			online, now, now, id,
		)
	} else {
		result, err = r.exec(ctx,
			"UPDATE devices SET online = ?, updated_at = ? WHERE id = ?",
			online, now, id,
		)
	}

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// UpdateLastSeen updates the last seen timestamp.
func (r *DeviceRepository) UpdateLastSeen(ctx context.Context, id string) error {
	now := time.Now().UnixMilli()

	result, err := r.exec(ctx,
		"UPDATE devices SET last_seen = ?, updated_at = ? WHERE id = ?",
		now, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// List returns a paginated list of devices.
func (r *DeviceRepository) List(ctx context.Context, orgID string, limit, offset int) ([]*device.Device, int, error) {
	var query string
	var countQuery string
	var args []interface{}

	if orgID != "" {
		query = `SELECT ` + deviceColumns + ` FROM devices WHERE organization_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
		countQuery = `SELECT COUNT(*) FROM devices WHERE organization_id = ?`
		args = []interface{}{orgID, limit, offset}
	} else {
		query = `SELECT ` + deviceColumns + ` FROM devices ORDER BY created_at DESC LIMIT ? OFFSET ?`
		countQuery = `SELECT COUNT(*) FROM devices`
		args = []interface{}{limit, offset}
	}

	rows, err := r.queryRows(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}

	defer func() { _ = rows.Close() }()

	devices, err := scanDevices(rows)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if orgID != "" {
		err = r.queryRow(ctx, countQuery, orgID).Scan(&total)
	} else {
		err = r.queryRow(ctx, countQuery).Scan(&total)
	}
	if err != nil {
		return nil, 0, err
	}

	return devices, total, rows.Err()
}

// ListByUser returns all devices for a user.
func (r *DeviceRepository) ListByUser(ctx context.Context, userID string) ([]*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE user_id = ? ORDER BY created_at DESC`

	rows, err := r.queryRows(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	return scanDevices(rows)
}

// Count returns the total number of devices filtered by organization.
func (r *DeviceRepository) Count(ctx context.Context, orgID string) (int, error) {
	var count int
	var err error

	if orgID != "" {
		err = r.queryRow(ctx, "SELECT COUNT(*) FROM devices WHERE organization_id = ?", orgID).Scan(&count)
	} else {
		err = r.queryRow(ctx, "SELECT COUNT(*) FROM devices").Scan(&count)
	}

	return count, err
}

// CountByUser returns the number of devices for a user.
func (r *DeviceRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.queryRow(ctx,
		"SELECT COUNT(*) FROM devices WHERE user_id = ?", userID,
	).Scan(&count)

	return count, err
}

// CountByOrganization returns the number of devices for an organization.
func (r *DeviceRepository) CountByOrganization(ctx context.Context, orgID string) (int, error) {
	var count int
	err := r.queryRow(ctx,
		"SELECT COUNT(*) FROM devices WHERE organization_id = ?", orgID,
	).Scan(&count)

	return count, err
}

// Touch updates the last seen timestamp for a device.
func (r *DeviceRepository) Touch(ctx context.Context, deviceID string) error {
	return r.UpdateLastSeen(ctx, deviceID)
}

// SoftDelete marks a device as deregistered (soft delete).
// Sets deregistered_at and deletion_scheduled_at for 30-day retention.
func (r *DeviceRepository) SoftDelete(ctx context.Context, id string, deregisteredAt, deletionScheduledAt int64) error {
	result, err := r.exec(ctx,
		"UPDATE devices SET deregistered_at = ?, deletion_scheduled_at = ?, online = 0, updated_at = ? WHERE id = ?",
		deregisteredAt, deletionScheduledAt, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return device.ErrNotFound
	}

	return nil
}

// SoftDeleteByIMEI marks a device as deregistered by IMEI.
func (r *DeviceRepository) SoftDeleteByIMEI(ctx context.Context, imei string, deregisteredAt, deletionScheduledAt int64) error {
	return r.SoftDelete(ctx, imei, deregisteredAt, deletionScheduledAt)
}

// ListActive returns all non-deregistered devices.
func (r *DeviceRepository) ListActive(ctx context.Context, limit, offset int) ([]*device.Device, int, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE deregistered_at IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`

	rows, err := r.queryRows(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	defer func() { _ = rows.Close() }()

	devices, err := scanDevices(rows)
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM devices WHERE deregistered_at IS NULL").Scan(&total); err != nil {
		return nil, 0, err
	}

	return devices, total, nil
}

// ListActiveByUser returns all non-deregistered devices for a user.
func (r *DeviceRepository) ListActiveByUser(ctx context.Context, userID string) ([]*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE user_id = ? AND deregistered_at IS NULL ORDER BY created_at DESC`

	rows, err := r.queryRows(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	return scanDevices(rows)
}

// FindByDeviceID retrieves a device by DeviceID value object.
func (r *DeviceRepository) FindByDeviceID(ctx context.Context, id device.DeviceID) (*device.Device, error) {
	return r.FindByID(ctx, id.String())
}

// FindByIDAndUserID retrieves a device by DeviceID and UserID value objects.
func (r *DeviceRepository) FindByIDAndUserID(ctx context.Context, id device.DeviceID, userID device.UserID) (*device.Device, error) {
	return r.FindByIDAndUser(ctx, id.String(), userID.String())
}

// DeleteByDeviceID deletes a device by DeviceID.
func (r *DeviceRepository) DeleteByDeviceID(ctx context.Context, id device.DeviceID) error {
	return r.Delete(ctx, id.String())
}

// SetOnlineByDeviceID sets the online status by DeviceID.
func (r *DeviceRepository) SetOnlineByDeviceID(ctx context.Context, id device.DeviceID, online bool) error {
	return r.SetOnline(ctx, id.String(), online)
}

// ListByUserID returns all devices for an UserID.
func (r *DeviceRepository) ListByUserID(ctx context.Context, userID device.UserID) ([]*device.Device, error) {
	return r.ListByUser(ctx, userID.String())
}

// ListByOrganization returns all devices for an organization.
func (r *DeviceRepository) ListByOrganization(ctx context.Context, orgID string) ([]*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE organization_id = ? ORDER BY created_at DESC`

	rows, err := r.queryRows(ctx, query, orgID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	return scanDevices(rows)
}

// ListByOrganizationPaginated returns paginated devices for an organization.
func (r *DeviceRepository) ListByOrganizationPaginated(ctx context.Context, orgID string, limit, offset int) ([]*device.Device, int, error) {
	countQuery := `SELECT COUNT(*) FROM devices WHERE organization_id = ?`
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, orgID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + deviceColumns + ` FROM devices WHERE organization_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`
	rows, err := r.queryRows(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	defer func() { _ = rows.Close() }()

	devices, err := scanDevices(rows)
	if err != nil {
		return nil, 0, err
	}

	return devices, total, nil
}

// ListPending returns all devices in pending lifecycle state.
func (r *DeviceRepository) ListPending(ctx context.Context) ([]*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE registered_at = 0 AND deregistered_at IS NULL ORDER BY created_at DESC`

	rows, err := r.queryRows(ctx, query)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	return scanDevices(rows)
}

// ListPendingByUser returns all pending devices for a user.
func (r *DeviceRepository) ListPendingByUser(ctx context.Context, userID device.UserID) ([]*device.Device, error) {
	query := `SELECT ` + deviceColumns + ` FROM devices WHERE user_id = ? AND registered_at = 0 AND deregistered_at IS NULL ORDER BY created_at DESC`

	rows, err := r.queryRows(ctx, query, userID.String())
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	return scanDevices(rows)
}

// DeleteScheduled deletes all devices where deletion_scheduled_at <= now AND deregistered_at IS NOT NULL.
// Returns the number of devices deleted.
func (r *DeviceRepository) DeleteScheduled(ctx context.Context) (int, error) {
	now := time.Now().UnixMilli()
	query := `DELETE FROM devices WHERE deletion_scheduled_at > 0 AND deletion_scheduled_at <= ? AND deregistered_at IS NOT NULL`

	result, err := r.exec(ctx, query, now)
	if err != nil {
		return 0, err
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(deleted), nil
}

// SoftDeleteByOrganization soft-deletes all devices for an organization.
// This is used during organization deletion.
func (r *DeviceRepository) SoftDeleteByOrganization(ctx context.Context, orgID string, deregisteredAt, deletionScheduledAt int64) (int, error) {
	query := `UPDATE devices SET deregistered_at = ?, deletion_scheduled_at = ? WHERE organization_id = ? AND deregistered_at IS NULL`

	result, err := r.exec(ctx, query, deregisteredAt, deletionScheduledAt, orgID)
	if err != nil {
		return 0, err
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(deleted), nil
}

func tagsToJSON(tags []string) interface{} {
	if len(tags) == 0 {
		return nil
	}
	data, err := json.Marshal(tags)
	if err != nil {
		return nil
	}
	return string(data)
}
