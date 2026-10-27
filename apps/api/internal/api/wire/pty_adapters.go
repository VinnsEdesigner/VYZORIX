// Package wire provides dependency injection utilities for the API server.
package wire

import (
	"context"

	websockethandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/websocket"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
)

// ptyDeviceResolver answers "which organization owns this device" for the PTY
// endpoint. It reuses the device repository rather than a bespoke query, so the
// answer matches the command path exactly.
type ptyDeviceResolver struct {
	repo *storage.DeviceRepository
}

// NewPTYDeviceResolver creates the resolver.
func NewPTYDeviceResolver(repo *storage.DeviceRepository) websockethandlers.PTYDeviceResolver {
	if repo == nil {
		return nil
	}

	return &ptyDeviceResolver{repo: repo}
}

// DeviceOrganization returns the org ID for a device, or "" when the device is
// unknown. The endpoint compares it against the operator's org, so an empty
// result fails the check closed.
func (r *ptyDeviceResolver) DeviceOrganization(ctx context.Context, deviceID string) string {
	dev, err := r.repo.FindByID(ctx, deviceID)
	if err != nil || dev == nil {
		return ""
	}

	return dev.OrganizationID
}

// Compile-time assertion that the resolver satisfies the handler contract.
var _ websockethandlers.PTYDeviceResolver = (*ptyDeviceResolver)(nil)
