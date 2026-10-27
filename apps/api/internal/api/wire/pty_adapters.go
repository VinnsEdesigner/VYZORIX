// Package wire provides dependency injection utilities for the API server.
package wire

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	websockethandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/websocket"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	domaincommand "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
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

// ptyAuditLogger adapts the audit logger to the handler's narrow interface. It
// returns a no-op when no audit store is configured, so the handler never holds
// a nil dependency and an unaudited deployment is explicit rather than silent.
func ptyAuditLogger(logger *audit.Logger) websockethandlers.PTYAudit {
	if logger == nil {
		return audit.NewNoOpLogger()
	}

	return logger
}

// newPTYRecorderFactory builds the registry's recorder constructor. It creates
// the recording directory on first use, so an operator can enable recording
// without a provisioning step; a failure is reported per session rather than at
// startup, since recording is optional.
func newPTYRecorderFactory(dir string, log *slog.Logger) func(sessionID string) (*hub.PTYRecorder, error) {
	return func(sessionID string) (*hub.PTYRecorder, error) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create pty recording dir: %w", err)
		}

		rec, err := hub.NewPTYRecorder(dir, sessionID, 80, 24, 0)
		if err != nil && log != nil {
			log.Warn("failed to open pty recording", "sessionId", sessionID, "err", err)
		}

		return rec, err
	}
}

// ptyAuthorizer adapts the shared command Authorizer to the terminal handler's
// gate. Delegating (rather than reimplementing) is the point: the critical tier
// registered for PTY_SESSION_OPEN in risk_catalog.go is consulted by the same
// evaluator the command path uses, so a shell gets the same MFA and
// single-use-confirmation requirements as any other critical operation.
type ptyAuthorizer struct {
	authorizer *command.Authorizer
}

// NewPTYAuthorizer creates the terminal-open authorizer.
func NewPTYAuthorizer(authorizer *command.Authorizer) websockethandlers.PTYAuthorizer {
	if authorizer == nil {
		return nil
	}

	return &ptyAuthorizer{authorizer: authorizer}
}

// AuthorizePTYOpen evaluates the request against the PTY_SESSION_OPEN profile.
func (a *ptyAuthorizer) AuthorizePTYOpen(ctx context.Context, req websockethandlers.PTYAuthRequest) websockethandlers.PTYAuthDecision {
	outcome := a.authorizer.Authorize(ctx, domaincommand.ActorContext{
		OperatorID:   req.OperatorID,
		OrgID:        req.OrgID,
		IsSuperAdmin: req.IsSuperAdmin,
		MFAVerified:  req.MFAVerified,
	}, domaincommand.TypePTYSessionOpen, req.DeviceID, req.ConfirmationToken)

	return websockethandlers.PTYAuthDecision{
		Allowed:           outcome.Allowed,
		Reason:            outcome.Reason,
		Message:           outcome.Message,
		NeedsConfirmation: outcome.NeedsConfirmation,
	}
}

// Compile-time assertion.
var _ websockethandlers.PTYAuthorizer = (*ptyAuthorizer)(nil)

// Compile-time assertions that the adapters satisfy the handler contracts.
var (
	_ websockethandlers.PTYDeviceResolver = (*ptyDeviceResolver)(nil)
	_ websockethandlers.PTYAudit          = (*audit.Logger)(nil)
)
