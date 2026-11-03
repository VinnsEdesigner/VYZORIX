// Package wire provides dependency injection utilities for the API server.
package wire

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	websockethandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/websocket"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
	orgapplication "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	domaincommand "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
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

// NewDevicePublicKeyProvider resolves a device's registered signing key for the
// WebSocket connect handshake.
//
// The device signs the connect with the private key it generated at first boot and
// the server verifies with the public half it received at enroll, so no shared
// secret is involved. A device that has not enrolled resolves to "", which the
// handshake treats as a refusal.
func NewDevicePublicKeyProvider(repo *storage.DeviceRepository) websockethandlers.DevicePublicKeyProvider {
	if repo == nil {
		return nil
	}

	return func(deviceID string) string {
		dev, err := repo.FindByID(context.Background(), deviceID)
		if err != nil || dev == nil {
			return ""
		}

		return dev.PublicKey
	}
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

// ptyCapabilityResolver resolves a session's capability at open: the operator's
// role and grants intersected with the device's tier, per the boundary contract's
// §7. It is the server-side decision, made once, so the device never re-derives
// authority.
type ptyCapabilityResolver struct {
	members *orgapplication.MemberService
	grants  *storage.GrantRepository
	devices *storage.DeviceRepository
}

// NewPTYCapabilityResolver builds the resolver. It returns nil when the pieces it
// needs are absent, so the handler's nil check disables the step rather than
// panicking — a deployment without a member service still serves terminals, it
// just does not record a resolved capability.
func NewPTYCapabilityResolver(
	members *orgapplication.MemberService,
	grants *storage.GrantRepository,
	devices *storage.DeviceRepository,
) websockethandlers.PTYCapabilityResolver {
	if members == nil || devices == nil {
		return nil
	}

	return &ptyCapabilityResolver{members: members, grants: grants, devices: devices}
}

// newPTYCapabilityResolver builds the resolver from the handler dependencies. The
// grant repository is constructed here from the same database the rest of the
// server uses, so no new dependency has to be threaded through the DI graph for
// one caller. It returns nil when the database is absent, which disables the
// capability step rather than failing startup.
func newPTYCapabilityResolver(deps HandlerDependencies) websockethandlers.PTYCapabilityResolver {
	if deps.DB == nil {
		return nil
	}

	var grants *storage.GrantRepository
	if db := deps.DB.DB(); db != nil {
		grants = storage.NewGrantRepository(db)
	}

	return NewPTYCapabilityResolver(deps.MemberService, grants, deps.DeviceRepo)
}

// ResolveForSession evaluates the two axes and returns the allowlist the device
// will enforce. ok is false when the operator is not an active member, which the
// handler treats as a refusal rather than an empty capability.
func (r *ptyCapabilityResolver) ResolveForSession(
	ctx context.Context,
	operatorID, orgID, deviceID string,
) (websockethandlers.SessionCapabilityInfo, bool) {
	member, err := r.members.GetMembership(ctx, operatorID, orgID)
	if err != nil || member == nil || !member.IsActive() {
		return websockethandlers.SessionCapabilityInfo{}, false
	}

	dev, err := r.devices.FindByID(ctx, deviceID)
	if err != nil || dev == nil {
		return websockethandlers.SessionCapabilityInfo{}, false
	}

	var custom []*permission.ResourcePermission
	if r.grants != nil {
		custom, _ = r.grants.ListEffective(ctx, operatorID, orgID)
	}

	capability := permission.ResolveSessionCapability(
		ctx,
		// The session id is assigned by the handler and is not part of the decision.
		"",
		string(member.Role),
		custom,
		deviceID,
		permission.Tier(dev.Tier),
		0,
	)

	return websockethandlers.SessionCapabilityInfo{
		APIAllowlist: capability.APIAllowlist,
		Tier:         int(capability.Tier),
		ExpiresAt:    capability.ExpiresAt,
	}, true
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
