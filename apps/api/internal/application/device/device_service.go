package device

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/inbox"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/transaction"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

var (
	ErrDeviceHijack               = errors.New("device registration hijack detected")
	ErrDeviceNotFound             = errors.New("device not found")
	ErrDeviceQuotaExceeded        = errors.New("device quota exceeded for organization")
	ErrDeviceAlreadyApproved      = errors.New("device already approved and registered")
	ErrInvalidLifecycleTransition = errors.New("invalid lifecycle state transition")
)

// Service handles device operations.
type Service struct {
	deviceRepo    device.Repository
	userRepo      user.Repository
	txManager     transaction.TxManager
	logger        *slog.Logger
	statusUpdater DeviceStatusUpdater
	deployConfigs DeployConfigSource
}

// DeviceStatusUpdater defines the interface for updating device online status.

type DeviceStatusUpdater interface {
	SetDeviceOnline(deviceID string, online bool)
}

// NewService creates a new DeviceService.
func NewService(
	deviceRepo device.Repository,
	userRepo user.Repository,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
		logger:     logger,
	}
}

// WithStatusUpdater sets the status updater (typically the hub) for serialized status updates.

func (s *Service) WithStatusUpdater(updater DeviceStatusUpdater) *Service {
	s.statusUpdater = updater
	return s
}

// WithTxManager sets the transaction manager for ACID transactions.
func (s *Service) WithTxManager(txManager transaction.TxManager) *Service {
	s.txManager = txManager
	return s
}

// StartDeletionWorker starts a background worker that periodically cleans up.
// deregistered devices whose deletion_scheduled_at time has passed.
// This implements the 30-day retention policy for deregistered devices.
func (s *Service) StartDeletionWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour // Default to 1 hour.
	}

	s.logger.Info("starting device deletion worker",
		"interval", interval,
	)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("stopping device deletion worker")
				return
			case <-ticker.C:
				s.runScheduledDeletion()
			}
		}
	}()
}

// runScheduledDeletion performs the actual deletion of scheduled devices.
func (s *Service) runScheduledDeletion() {
	ctx := context.Background()

	deleted, err := s.deviceRepo.DeleteScheduled(ctx)
	if err != nil {
		s.logger.Error("failed to run scheduled device deletion",
			"error", err,
		)
		return
	}

	if deleted > 0 {
		s.logger.Info("completed scheduled device deletion",
			"deleted_count", deleted,
		)
	}
}

// ApproveDevice transitions a pending device to registered state.
// Returns error if device is not in pending state.
func (s *Service) ApproveDevice(ctx context.Context, deviceID string) error {
	d, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	// Use domain method to enforce valid transitions.
	if err := d.Approve(); err != nil {
		return ErrInvalidLifecycleTransition
	}

	// Set RegisteredAt when transitioning to registered.
	d.RegisteredAt = time.Now().UnixMilli()
	d.UpdatedAt = time.Now()

	if err := s.deviceRepo.Update(ctx, d); err != nil {
		return err
	}

	return nil
}

// GetStatus retrieves device status.
func (s *Service) GetStatus(ctx context.Context, deviceID string) (*dto.DeviceStatusResponse, error) {
	d, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, application.ErrDeviceNotFound
		}

		return nil, err
	}

	return &dto.DeviceStatusResponse{
		DeviceID:    d.ID,
		Online:      d.Online,
		LastSeen:    d.LastSeen,
		AppVersion:  d.AppVersion,
		DeviceClass: d.DeviceClass,
	}, nil
}

// GetStatusByUser retrieves device status with DOA verification.
// Only returns device if it belongs to the specified user.
func (s *Service) GetStatusByUser(ctx context.Context, deviceID, userID string) (*dto.DeviceStatusResponse, error) {
	d, err := s.deviceRepo.FindByIDAndUser(ctx, deviceID, userID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, application.ErrDeviceNotFound
		}

		return nil, err
	}

	return &dto.DeviceStatusResponse{
		DeviceID:    d.ID,
		Online:      d.Online,
		LastSeen:    d.LastSeen,
		AppVersion:  d.AppVersion,
		DeviceClass: d.DeviceClass,
	}, nil
}

// GetDeviceByUser retrieves a device with DOA verification.
func (s *Service) GetDeviceByUser(ctx context.Context, deviceID, userID string) (*dto.DeviceResponse, error) {
	d, err := s.deviceRepo.FindByIDAndUser(ctx, deviceID, userID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, application.ErrDeviceNotFound
		}

		return nil, err
	}

	return &dto.DeviceResponse{
		ID:           d.ID,
		IMEI:         d.ID,
		DeviceName:   d.DeviceName,
		Model:        d.Model,
		Manufacturer: d.Manufacturer,
		OSVersion:    d.OSVersion,
		AppVersion:   d.AppVersion,
		Status:       d.GetStatus(),
		Online:       d.Online,
		RegisteredAt: d.RegisteredAt,
		LastSeen:     d.LastSeen,
	}, nil
}

// UpdateFCMToken updates the FCM token for a device.
func (s *Service) UpdateFCMToken(ctx context.Context, deviceID, fcmToken string) error {
	return s.deviceRepo.UpdateFCMToken(ctx, deviceID, fcmToken)
}

// UpdateFCMTokenAndReturn updates FCM token and returns the device.
func (s *Service) UpdateFCMTokenAndReturn(ctx context.Context, deviceID, fcmToken string) (*device.Device, error) {
	if err := s.deviceRepo.UpdateFCMToken(ctx, deviceID, fcmToken); err != nil {
		return nil, err
	}

	return s.deviceRepo.FindByID(ctx, deviceID)
}

// SetOnline sets the online status of a device.
func (s *Service) SetOnline(ctx context.Context, deviceID string, online bool) error {
	return s.deviceRepo.SetOnline(ctx, deviceID, online)
}

// List returns a paginated list of devices filtered by organization.
func (s *Service) List(ctx context.Context, orgID string, limit, offset int) (*dto.DeviceListResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	devices, total, err := s.deviceRepo.List(ctx, orgID, limit, offset)
	if err != nil {
		return nil, err
	}

	response := &dto.DeviceListResponse{
		Devices: make([]dto.DeviceResponse, len(devices)),
		Pagination: dto.PaginationInfo{
			Total: total,
			Limit: limit,
		},
	}

	for i, d := range devices {
		response.Devices[i] = dto.DeviceResponse{
			ID:           d.ID,
			IMEI:         d.ID,
			DeviceName:   d.DeviceName,
			Model:        d.Model,
			Manufacturer: d.Manufacturer,
			OSVersion:    d.OSVersion,
			AppVersion:   d.AppVersion,
			Status:       d.GetStatus(),
			Online:       d.Online,
			RegisteredAt: d.RegisteredAt,
			LastSeen:     d.LastSeen,
		}
	}

	// Calculate pagination info.
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	response.Pagination.Page = (offset / limit) + 1
	response.Pagination.TotalPages = totalPages

	return response, nil
}

// ListByOrganization returns a paginated list of devices for an organization.
func (s *Service) ListByOrganization(ctx context.Context, orgID string, limit, offset int) (*dto.DeviceListResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	devices, total, err := s.deviceRepo.ListByOrganizationPaginated(ctx, orgID, limit, offset)
	if err != nil {
		return nil, err
	}

	response := &dto.DeviceListResponse{
		Devices: make([]dto.DeviceResponse, len(devices)),
		Pagination: dto.PaginationInfo{
			Total: total,
			Limit: limit,
		},
	}

	for i, d := range devices {
		response.Devices[i] = dto.DeviceResponse{
			ID:             d.ID,
			IMEI:           d.ID,
			DeviceName:     d.DeviceName,
			Model:          d.Model,
			Manufacturer:   d.Manufacturer,
			OSVersion:      d.OSVersion,
			AppVersion:     d.AppVersion,
			Status:         d.GetStatus(),
			Online:         d.Online,
			RegisteredAt:   d.RegisteredAt,
			LastSeen:       d.LastSeen,
			DeregisteredAt: d.DeregisteredAt,
			OrganizationID: d.OrganizationID,
		}
	}

	// Calculate pagination info.
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	response.Pagination.Page = (offset / limit) + 1
	response.Pagination.TotalPages = totalPages

	return response, nil
}

// ListByUser returns all devices for a user.
func (s *Service) ListByUser(ctx context.Context, userID string) ([]dto.DeviceResponse, error) {
	devices, err := s.deviceRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := make([]dto.DeviceResponse, len(devices))
	for i, d := range devices {
		response[i] = dto.DeviceResponse{
			ID:             d.ID,
			IMEI:           d.ID,
			DeviceName:     d.DeviceName,
			Model:          d.Model,
			Manufacturer:   d.Manufacturer,
			OSVersion:      d.OSVersion,
			AppVersion:     d.AppVersion,
			Status:         d.GetStatus(),
			Online:         d.Online,
			RegisteredAt:   d.RegisteredAt,
			LastSeen:       d.LastSeen,
			DeregisteredAt: d.DeregisteredAt,
			OrganizationID: d.OrganizationID,
		}
	}

	return response, nil
}

// ListByUserEntity returns all devices for a user as domain entities.
func (s *Service) ListByUserEntity(ctx context.Context, userID string) ([]*device.Device, error) {
	return s.deviceRepo.ListByUser(ctx, userID)
}

// Delete deletes a device.
func (s *Service) Delete(ctx context.Context, deviceID string) error {
	return s.deviceRepo.Delete(ctx, deviceID)
}

// DeleteDevice deletes a device (alias for Delete).
func (s *Service) DeleteDevice(ctx context.Context, deviceID string) error {
	return s.deviceRepo.Delete(ctx, deviceID)
}

// GetDevice retrieves a device by ID.
func (s *Service) GetDevice(ctx context.Context, deviceID string) (*device.Device, error) {
	return s.deviceRepo.FindByID(ctx, deviceID)
}

// Count returns the total number of devices filtered by organization.
func (s *Service) Count(ctx context.Context, orgID string) (int, error) {
	return s.deviceRepo.Count(ctx, orgID)
}

// CountByUser returns the total number of devices for a user.
func (s *Service) CountByUser(ctx context.Context, userID string) (int, error) {
	devices, err := s.deviceRepo.ListByUser(ctx, userID)
	if err != nil {
		return 0, err
	}

	return len(devices), nil
}

// CountByOrganization returns the total number of devices for an organization.
func (s *Service) CountByOrganization(ctx context.Context, orgID string) (int, error) {
	return s.deviceRepo.CountByOrganization(ctx, orgID)
}

// ListByUserPaginated returns paginated devices for a user.
func (s *Service) ListByUserPaginated(ctx context.Context, userID string, limit, offset int) ([]*device.Device, error) {
	if limit <= 0 {
		limit = 50
	}

	if limit > 100 {
		limit = 100
	}

	allDevices, err := s.deviceRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if offset >= len(allDevices) {
		return []*device.Device{}, nil
	}

	end := offset + limit
	if end > len(allDevices) {
		end = len(allDevices)
	}

	return allDevices[offset:end], nil
}

// GetRepo returns the device repository for use by other services.
func (s *Service) GetRepo() device.Repository {
	return s.deviceRepo
}

// DeviceRepo returns the device repository (alias for GetRepo).
func (s *Service) DeviceRepo() device.Repository {
	return s.deviceRepo
}

// ListQuery represents query parameters for listing devices.
type ListQuery struct {
	OrganizationID string
	Status         string
	Search         string
	Page           int
	Limit          int
}

// GetDevices returns a paginated list of devices filtered by organization.
func (s *Service) GetDevices(ctx context.Context, query *ListQuery) (*dto.DeviceListResponse, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}

	offset := (query.Page - 1) * query.Limit

	// Get devices filtered by organization using the paginated method.
	allDevices, _, err := s.deviceRepo.ListByOrganizationPaginated(ctx, query.OrganizationID, 10000, 0)
	if err != nil {
		return nil, err
	}

	// Apply filters.
	var filtered []*device.Device
	for _, d := range allDevices {
		// Apply status filter.
		if query.Status != "" && query.Status != "all" {
			isOnline := d.Online
			if query.Status == "online" && !isOnline {
				continue
			}
			if query.Status == "offline" && isOnline {
				continue
			}
		}

		// Apply search filter.
		if query.Search != "" {
			// Search by ID (IMEI) or other fields.
			searchLower := toLower(query.Search)
			idMatch := contains(toLower(d.ID), searchLower)
			classMatch := contains(toLower(d.DeviceClass), searchLower)
			if !idMatch && !classMatch {
				continue
			}
		}

		filtered = append(filtered, d)
	}

	// Calculate pagination.
	total := len(filtered)
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.Limit - 1) / query.Limit
	}

	// Apply pagination.
	start := offset
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + query.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	paged := filtered[start:end]

	// Build response.
	devices := make([]dto.DeviceResponse, 0, len(paged))
	for _, d := range paged {
		devices = append(devices, dto.DeviceResponse{
			ID:           d.ID,
			IMEI:         d.ID,
			DeviceName:   d.DeviceName,
			Model:        d.Model,
			Manufacturer: d.Manufacturer,
			OSVersion:    d.OSVersion,
			AppVersion:   d.AppVersion,
			Status:       d.GetStatus(),
			Online:       d.Online,
			RegisteredAt: d.RegisteredAt,
			LastSeen:     d.LastSeen,
		})
	}

	return &dto.DeviceListResponse{
		Devices: devices,
		Pagination: dto.PaginationInfo{
			Page:       query.Page,
			Limit:      query.Limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetDeviceDetail returns detailed device information for /v1/devices/:imei endpoint.
func (s *Service) GetDeviceDetail(ctx context.Context, imei string) (*dto.DeviceDetailResponse, error) {
	d, err := s.deviceRepo.FindByIMEI(ctx, imei)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	return s.deviceDetailResponse(d), nil
}

// GetDeviceDetailByUser returns detailed device information with DOA verification.
func (s *Service) GetDeviceDetailByUser(ctx context.Context, imei, userID string) (*dto.DeviceDetailResponse, error) {
	d, err := s.deviceRepo.FindByIMEIAndUser(ctx, imei, userID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	return s.deviceDetailResponse(d), nil
}

// SetDeviceTags replaces all tags on a device.
func (s *Service) SetDeviceTags(ctx context.Context, deviceID string, tags []string) error {
	d, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		return device.ErrNotFound
	}
	d.Tags = tags
	return s.deviceRepo.Update(ctx, d)
}

// AddDeviceTag adds a single tag to a device (idempotent).
func (s *Service) AddDeviceTag(ctx context.Context, deviceID, tag string) error {
	d, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		return device.ErrNotFound
	}
	d.AddTag(tag)
	return s.deviceRepo.Update(ctx, d)
}

// RemoveDeviceTag removes a single tag from a device.
func (s *Service) RemoveDeviceTag(ctx context.Context, deviceID, tag string) error {
	d, err := s.deviceRepo.FindByID(ctx, deviceID)
	if err != nil {
		return device.ErrNotFound
	}
	d.RemoveTag(tag)
	return s.deviceRepo.Update(ctx, d)
}

// CheckDeviceQuota verifies the org hasn't exceeded its device quota.
func (s *Service) CheckDeviceQuota(ctx context.Context, orgID string, maxDevices int) error {
	if maxDevices <= 0 {
		return nil
	}
	_, count, err := s.deviceRepo.ListByOrganizationPaginated(ctx, orgID, 1, 0)
	if err != nil {
		return fmt.Errorf("check device quota: %w", err)
	}
	if count >= maxDevices {
		return ErrDeviceQuotaExceeded
	}
	return nil
}

// GetDeviceDetailByOrganization returns detailed device information for an organization.
func (s *Service) GetDeviceDetailByOrganization(ctx context.Context, imei, orgID string) (*dto.DeviceDetailResponse, error) {
	d, err := s.deviceRepo.FindByIMEIAndOrganization(ctx, imei, orgID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	return s.deviceDetailResponse(d), nil
}

// DeregisterDeviceByOrganization deregisters a device within an organization.
func (s *Service) DeregisterDeviceByOrganization(ctx context.Context, imei, orgID string, hard bool) (*dto.DeregisterResponse, error) {
	// First verify device exists and belongs to this organization.
	_, err := s.deviceRepo.FindByIMEIAndOrganization(ctx, imei, orgID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	now := time.Now()
	deregisteredAt := now.UnixMilli()
	retentionUntil := now.Add(30 * 24 * time.Hour).UnixMilli() // 30 days retention.

	if hard {
		// Hard delete - actually remove the device.
		if err := s.deviceRepo.Delete(ctx, imei); err != nil {
			if errors.Is(err, device.ErrNotFound) {
				return nil, device.ErrNotFound
			}
			return nil, err
		}
		return &dto.DeregisterResponse{
			IMEI:           imei,
			Status:         "deleted",
			DeregisteredAt: deregisteredAt,
		}, nil
	}

	// Soft delete - mark as deregistered.
	if err := s.deviceRepo.SoftDelete(ctx, imei, deregisteredAt, retentionUntil); err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	return &dto.DeregisterResponse{
		IMEI:           imei,
		Status:         "deregistered",
		DeregisteredAt: deregisteredAt,
		RetentionUntil: retentionUntil,
	}, nil
}

// deviceDetailResponse creates a DeviceDetailResponse from a Device entity.
func (s *Service) deviceDetailResponse(d *device.Device) *dto.DeviceDetailResponse {
	// Check FCM token validity using domain method.
	fcmValid := d.IsFCMTokenValid()

	// Determine status using domain method.
	status := d.GetStatus()

	resp := &dto.DeviceDetailResponse{
		ID:            d.ID,
		IMEI:          d.ID,
		DeviceName:    d.DeviceName,
		Model:         d.Model,
		Manufacturer:  d.Manufacturer,
		OSVersion:     d.OSVersion,
		AppVersion:    d.AppVersion,
		SecurityPatch: d.SecurityPatch,
		Status:        status,
		RegisteredAt:  d.RegisteredAt,
		LastSeen:      d.LastSeen,
		FCMTokenValid: fcmValid,
	}

	return resp
}

// DeregisterDevice soft-deletes a device (marks as deregistered with 30-day retention).
func (s *Service) DeregisterDevice(ctx context.Context, imei string, hard bool) (*dto.DeregisterResponse, error) {
	now := time.Now()
	deregisteredAt := now.UnixMilli()
	deletionScheduledAt := now.Add(30 * 24 * time.Hour).UnixMilli() // 30 days retention.

	if hard {
		// Hard delete - actually remove the device.
		if err := s.deviceRepo.Delete(ctx, imei); err != nil {
			if errors.Is(err, device.ErrNotFound) {
				return nil, device.ErrNotFound
			}
			return nil, err
		}
		return &dto.DeregisterResponse{
			IMEI:           imei,
			Status:         "deleted",
			DeregisteredAt: deregisteredAt,
		}, nil
	}

	// Soft delete - mark as deregistered.
	if err := s.deviceRepo.SoftDeleteByIMEI(ctx, imei, deregisteredAt, deletionScheduledAt); err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	return &dto.DeregisterResponse{
		IMEI:           imei,
		Status:         "deregistered",
		DeregisteredAt: deregisteredAt,
		RetentionUntil: deletionScheduledAt,
	}, nil
}

// DeregisterDeviceByUser soft-deletes a device with DOA and org verification.
// Only the user who owns the device within an organization can deregister it.
func (s *Service) DeregisterDeviceByUser(ctx context.Context, imei, userID, orgID string, hard bool) (*dto.DeregisterResponse, error) {
	// First verify device exists and belongs to this user and organization.
	d, err := s.deviceRepo.FindByIDAndOrganization(ctx, imei, orgID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, device.ErrNotFound
		}
		return nil, err
	}

	// Verify user owns this device.
	if d.UserID != userID {
		return nil, device.ErrNotFound
	}

	now := time.Now()
	deregisteredAt := now.UnixMilli()
	deletionScheduledAt := now.Add(30 * 24 * time.Hour).UnixMilli() // 30 days retention.

	if hard {
		// Hard delete - actually remove the device.
		if err := s.deviceRepo.Delete(ctx, imei); err != nil {
			return nil, err
		}
		return &dto.DeregisterResponse{
			IMEI:           imei,
			Status:         "deleted",
			DeregisteredAt: deregisteredAt,
		}, nil
	}

	// Soft delete - mark as deregistered.
	if err := s.deviceRepo.SoftDeleteByIMEI(ctx, imei, deregisteredAt, deletionScheduledAt); err != nil {
		return nil, err
	}

	// Log the deregistration for audit.
	s.logDeviceAction(ctx, d.ID, imei, "deregistered", userID, fmt.Sprintf("Device deregistered, hard=%v", hard))

	return &dto.DeregisterResponse{
		IMEI:           imei,
		Status:         "deregistered",
		DeregisteredAt: deregisteredAt,
		RetentionUntil: deletionScheduledAt,
	}, nil
}

// logDeviceAction logs device-related actions for audit trail.
func (s *Service) logDeviceAction(ctx context.Context, deviceID, imei, action, userID, details string) {
	// This would typically write to an audit log.
	// For now, we rely on the user repository for ownership verification.
}

// CreateFromInbox creates a device from an approved inbox entry.
func (s *Service) CreateFromInbox(ctx context.Context, entry *inbox.InboxEntry) (*device.Device, error) {
	now := time.Now()
	d := &device.Device{
		ID:                entry.IMEI,
		OrganizationID:    entry.OrganizationID,
		FirebaseInstallID: entry.FirebaseInstallID,
		FCMToken:          entry.FCMToken,
		AppVersion:        entry.AppVersion,
		DeviceClass:       entry.DeviceClass,
		DeviceName:        entry.DeviceName,
		Model:             entry.Model,
		Manufacturer:      entry.Manufacturer,
		OSVersion:         entry.OSVersion,
		UserID:            entry.UserID,
		Online:            false,
		RegisteredAt:      now.UnixMilli(),
		LastSeen:          now.UnixMilli(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	createFn := func(ctx context.Context) error {
		existing, findErr := s.deviceRepo.FindByIMEI(ctx, entry.IMEI)
		if findErr != nil && findErr != device.ErrNotFound {
			return fmt.Errorf("failed to check existing device: %w", findErr)
		}
		if findErr := s.handleDeregisteredDevice(ctx, existing); findErr != nil {
			return findErr
		}
		return s.deviceRepo.Create(ctx, d)
	}

	if s.txManager != nil {
		return d, s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			return createFn(txCtx)
		})
	}
	return d, createFn(ctx)
}

// handleDeregisteredDevice handles cleanup when re-registering a previously deregistered device.
func (s *Service) handleDeregisteredDevice(ctx context.Context, existing *device.Device) error {
	if existing == nil {
		return nil
	}
	if existing.IsDeregistered() {
		if err := s.deviceRepo.Delete(ctx, existing.ID); err != nil {
			s.logger.Error("failed to delete old deregistered device",
				"deviceID", existing.ID, "error", err)
			return fmt.Errorf("failed to cleanup old device: %w", err)
		}
		s.logger.Info("deleted old deregistered device", "deviceID", existing.ID)
		return nil
	}
	return ErrDeviceAlreadyApproved
}

// GetDeviceByIMEI retrieves a device by IMEI.
func (s *Service) GetDeviceByIMEI(ctx context.Context, imei string) (*device.Device, error) {
	d, err := s.deviceRepo.FindByIMEI(ctx, imei)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	return d, nil
}

func toLower(s string) string {
	return strings.ToLower(s)
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TransferDevice transfers a device from one organization to another.
// Prerequisites:.
// - Device must be OFFLINE.
// - Actor must have permission in source AND target orgs.
func (s *Service) TransferDevice(ctx context.Context, imei, sourceOrgID, targetOrgID, actorUserID string) error {
	// Get the device.
	d, err := s.deviceRepo.FindByIMEI(ctx, imei)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	// Verify device belongs to source org.
	if d.OrganizationID != sourceOrgID {
		return ErrDeviceNotFound
	}

	// Device must be offline for transfer.
	if d.Online {
		return application.ErrDeviceOnline
	}

	// Update device organization.
	d.OrganizationID = targetOrgID
	d.UpdatedAt = time.Now()

	if err := s.deviceRepo.Update(ctx, d); err != nil {
		return err
	}

	s.logger.Info("device transferred between organizations",
		"imei", imei,
		"from_org", sourceOrgID,
		"to_org", targetOrgID,
		"actor_id", actorUserID,
	)

	return nil
}
