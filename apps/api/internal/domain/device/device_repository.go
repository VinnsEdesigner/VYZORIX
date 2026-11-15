package device

import (
	"context"
)

// Repository defines the interface for device data access.
type Repository interface {
	// FindByID retrieves a device by ID.
	FindByID(ctx context.Context, id string) (*Device, error)

	// FindByDeviceID retrieves a device by DeviceID value object.
	FindByDeviceID(ctx context.Context, id DeviceID) (*Device, error)

	// FindByIMEI retrieves a device by IMEI (device ID).
	FindByIMEI(ctx context.Context, imei string) (*Device, error)

	// FindByFirebaseInstallID retrieves a device by Firebase install ID.
	FindByFirebaseInstallID(ctx context.Context, fid string) (*Device, error)

	// FindByIDAndUser retrieves a device by ID and verifies ownership.
	// Returns ErrNotFound if device doesn't exist or doesn't belong to user.
	// This is used for DOA (Data Ownership Attribution) checks.
	FindByIDAndUser(ctx context.Context, id, userID string) (*Device, error)

	// FindByIDAndUserID retrieves a device by DeviceID and UserID value objects.
	FindByIDAndUserID(ctx context.Context, id DeviceID, userID UserID) (*Device, error)

	// FindByIMEIAndUser retrieves a device by IMEI and verifies ownership.
	// Returns ErrNotFound if device doesn't exist or doesn't belong to user.
	// This is used for DOA (Data Ownership Attribution) checks on deregistration.
	FindByIMEIAndUser(ctx context.Context, imei, userID string) (*Device, error)

	// FindByIMEIAndOrganization retrieves a device by IMEI within an organization.
	// Returns ErrNotFound if device doesn't exist or doesn't belong to the organization.
	FindByIMEIAndOrganization(ctx context.Context, imei, orgID string) (*Device, error)

	// FindByIDAndOrganization retrieves a device by ID within an organization.
	// Returns ErrNotFound if device doesn't exist or doesn't belong to the organization.
	FindByIDAndOrganization(ctx context.Context, id, orgID string) (*Device, error)

	// FindByPairingCode retrieves a pending device by the code a user types
	// to claim it. Only a live code matches, so an expired notification cannot
	// be claimed.
	FindByPairingCode(ctx context.Context, code string) (*Device, error)

	// ClearPairingCode consumes a device's pairing code so it cannot be claimed
	// twice.
	ClearPairingCode(ctx context.Context, id string) error

	// Create creates a new device.
	Create(ctx context.Context, d *Device) error

	// Update updates an existing device.
	Update(ctx context.Context, d *Device) error

	// Delete deletes a device.
	Delete(ctx context.Context, id string) error

	// DeleteByDeviceID deletes a device by DeviceID.
	DeleteByDeviceID(ctx context.Context, id DeviceID) error

	// UpdateFCMToken updates the FCM token for a device.
	UpdateFCMToken(ctx context.Context, id, fcmToken string) error

	// UpdateAppVersion records the app version the device reports it is running.
	UpdateAppVersion(ctx context.Context, id, appVersion string) error

	// UpdateReportedState records what the device says it is actually running: the
	// config version it holds and, per app, a digest of the artifacts installed.
	//
	// configVersion is a pointer so "the device did not report" (nil) is stored as
	// NULL and stays distinguishable from 0 ("reports holding no config"). The two
	// must not be conflated: the first is unknown and must never render as healthy.
	UpdateReportedState(
		ctx context.Context,
		id string,
		configVersion *int,
		appDigestsJSON string,
		reportedAt int64,
	) error

	// SetOnline sets the online status of a device.
	SetOnline(ctx context.Context, id string, online bool) error

	// SetOnlineByDeviceID sets the online status by DeviceID.
	SetOnlineByDeviceID(ctx context.Context, id DeviceID, online bool) error

	// UpdateLastSeen updates the last seen timestamp.
	UpdateLastSeen(ctx context.Context, id string) error

	// Touch updates the last seen timestamp (alias for UpdateLastSeen).
	Touch(ctx context.Context, deviceID string) error

	// List returns a paginated list of devices filtered by organization.
	List(ctx context.Context, orgID string, limit, offset int) ([]*Device, int, error)

	// ListByUser returns all devices for a user.
	ListByUser(ctx context.Context, userID string) ([]*Device, error)

	// ListByUserID returns all devices for an UserID.
	ListByUserID(ctx context.Context, userID UserID) ([]*Device, error)

	// ListByOrganization returns all devices for an organization.
	ListByOrganization(ctx context.Context, orgID string) ([]*Device, error)

	// ListByOrganizationPaginated returns paginated devices for an organization.
	ListByOrganizationPaginated(ctx context.Context, orgID string, limit, offset int) ([]*Device, int, error)

	// Count returns the total number of devices filtered by organization.
	Count(ctx context.Context, orgID string) (int, error)

	// CountByUser returns the number of devices for a user.
	CountByUser(ctx context.Context, userID string) (int, error)

	// CountByOrganization returns the number of devices for an organization.
	CountByOrganization(ctx context.Context, orgID string) (int, error)

	// SoftDelete marks a device as deregistered (soft delete).
	// Sets deregistered_at and deletion_scheduled_at for 30-day retention.
	SoftDelete(ctx context.Context, id string, deregisteredAt, deletionScheduledAt int64) error

	// SoftDeleteByIMEI marks a device as deregistered by IMEI.
	SoftDeleteByIMEI(ctx context.Context, imei string, deregisteredAt, deletionScheduledAt int64) error

	// ListActive returns all non-deregistered devices.
	ListActive(ctx context.Context, limit, offset int) ([]*Device, int, error)

	// ListActiveByUser returns all non-deregistered devices for a user.
	ListActiveByUser(ctx context.Context, userID string) ([]*Device, error)

	// ListPending returns all devices in pending lifecycle state.
	ListPending(ctx context.Context) ([]*Device, error)

	// ListPendingByUser returns all pending devices for a user.
	ListPendingByUser(ctx context.Context, userID UserID) ([]*Device, error)

	// DeleteScheduled deletes all devices where deletion_scheduled_at <= now AND deregistered_at IS NOT NULL.
	// Returns the number of devices deleted.
	DeleteScheduled(ctx context.Context) (int, error)

	// SoftDeleteByOrganization soft-deletes all devices for an organization.
	// This is used during organization deletion.
	SoftDeleteByOrganization(ctx context.Context, orgID string, deregisteredAt, deletionScheduledAt int64) (int, error)
}
