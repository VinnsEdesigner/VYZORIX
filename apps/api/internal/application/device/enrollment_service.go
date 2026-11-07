package device

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// Errors the enroll/claim flow distinguishes. A caller that cannot tell "code
// expired" from "code already used" cannot give the operator a useful message.
var (
	ErrPairingCodeExpired = errors.New("pairing code has expired")
	ErrPairingCodeUnknown = errors.New("pairing code is not recognised")
	ErrPublicKeyRequired  = errors.New("a device public key is required")
	ErrPublicKeyInvalid   = errors.New("device public key is not a valid P-256 key")
	ErrDeviceAlreadyOwned = errors.New("device is already claimed by an organization")
)

// PairingCodeTTL is how long a code stays claimable. Short enough that a leaked
// code is stale by the time it could be used, long enough to walk to a browser.
// See doc/REGISTRATION_CONTRACT.md §4.
const PairingCodeTTL = 10 * time.Minute

// EnrollRequest is what a freshly installed device sends. It carries no secret:
// the public key is the device's identity, and the private half never leaves it.
type EnrollRequest struct {
	DeviceUUID   string
	PublicKeyPEM string
	Model        string
	Manufacturer string
	OSVersion    string
	// APIVersion is the Android API level. Recorded so the server can show what a
	// device runs without parsing osVersion.
	APIVersion  int
	AppVersion  string
	DeviceClass string
	// BuildID is the build fingerprint.
	BuildID string
	// SecurityPatch is the patch level, so staleness is visible to an operator.
	SecurityPatch string
	// SupportedABIs is the device's ABI list, joined for storage.
	SupportedABIs []string
	// RAMBytes and StorageBytes are the device's capacity as reported at enroll.
	RAMBytes     int64
	StorageBytes int64
	// TierHint is the device's self-reported state. It is used only to cap what
	// is granted, never to grant more than the policy allows: the server cannot
	// verify root.
	TierHint int
}

// EnrollResult is what the device receives: a code to display and how long it is
// good for. Nothing secret, so nothing to intercept.
type EnrollResult struct {
	DeviceID       string
	PairingCode    string
	PairingDisplay string
	ExpiresAt      int64
}

// Enroll registers an unknown device as pending and returns its pairing code.
//
// This is the one unauthenticated step, by necessity: the device has no
// credential yet. Its safety rests on the code's entropy, its TTL, its
// single-use rule and the caller's rate limiting — stated plainly rather than
// implied. See doc/REGISTRATION_CONTRACT.md §4 and §9.
func (s *Service) Enroll(ctx context.Context, req EnrollRequest) (*EnrollResult, error) {
	if req.DeviceUUID == "" {
		return nil, ErrPublicKeyRequired
	}

	// Parse the key before writing anything: a device that sends a malformed key
	// must be refused at the door, not stored and then fail on first use.
	if _, err := crypto.DecodePublicKey(req.PublicKeyPEM); err != nil {
		if errors.Is(err, crypto.ErrNoPublicKey) {
			return nil, ErrPublicKeyRequired
		}

		return nil, fmt.Errorf("%w: %v", ErrPublicKeyInvalid, err)
	}

	// Re-enrolling the same uuid is idempotent: a device that lost its code (or
	// was reinstalled) gets a fresh one rather than a duplicate row. A device that
	// is already claimed is not re-enrolled — that would be a way to hijack one.
	existing, err := s.deviceRepo.FindByID(ctx, req.DeviceUUID)
	if err == nil && existing != nil {
		if existing.Lifecycle.IsRegistered() {
			return nil, ErrDeviceAlreadyOwned
		}
	} else if err != nil && !errors.Is(err, device.ErrNotFound) {
		return nil, err
	}

	code, display, err := crypto.GeneratePairingCode()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(PairingCodeTTL).UnixMilli()

	d := device.NewDevice(req.DeviceUUID, "")
	d.PublicKey = req.PublicKeyPEM
	d.PairingCode = code
	d.PairingExpiresAt = &expiresAt
	d.Model = req.Model
	d.Manufacturer = req.Manufacturer
	d.OSVersion = req.OSVersion
	d.APIVersion = req.APIVersion
	d.AppVersion = req.AppVersion
	d.DeviceClass = req.DeviceClass
	d.BuildID = req.BuildID
	d.SecurityPatch = req.SecurityPatch
	d.SupportedABIs = strings.Join(req.SupportedABIs, ",")
	d.RAMBytes = req.RAMBytes
	d.StorageBytes = req.StorageBytes
	d.Tier = device.TierFromInt(req.TierHint)

	if existing != nil {
		if err := s.deviceRepo.Update(ctx, d); err != nil {
			return nil, err
		}
	} else if err := s.deviceRepo.Create(ctx, d); err != nil {
		return nil, err
	}

	s.logger.Info("device enrolled", "deviceId", d.ID, "tier", d.Tier)

	return &EnrollResult{
		DeviceID:       d.ID,
		PairingCode:    code,
		PairingDisplay: display,
		ExpiresAt:      expiresAt,
	}, nil
}

// ClaimRequest binds a pending device to an owner and organization.
type ClaimRequest struct {
	PairingCode    string
	OperatorID     string
	OrganizationID string
	Name           string
	TierCap        int
}

// Claim consumes a pairing code and registers the device.
//
// This is the operator's action: they type the code the device is showing, see
// the device's facts, and confirm it is theirs. Consuming the code is what makes
// it single-use, so a second claim with the same code fails.
func (s *Service) Claim(ctx context.Context, req ClaimRequest) (*device.Device, error) {
	code := crypto.NormalizePairingCode(req.PairingCode)
	if code == "" {
		return nil, ErrPairingCodeUnknown
	}
	if req.OrganizationID == "" {
		return nil, ErrDeviceNotFound
	}

	d, err := s.deviceRepo.FindByPairingCode(ctx, code)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			// The query already excludes expired codes, so "not found" covers both
			// unknown and expired. Distinguishing them would let a caller probe
			// which codes exist.
			return nil, ErrPairingCodeUnknown
		}

		return nil, err
	}

	if d.Lifecycle.IsRegistered() {
		return nil, ErrDeviceAlreadyOwned
	}
	if d.PairingExpiresAt != nil && *d.PairingExpiresAt < time.Now().UnixMilli() {
		return nil, ErrPairingCodeExpired
	}

	// Bind ownership before the transition, so a failure leaves the device
	// pending with its code intact rather than half-claimed.
	d.OperatorID = req.OperatorID
	d.OrganizationID = req.OrganizationID
	if req.Name != "" {
		d.DeviceName = req.Name
	}
	if req.TierCap > 0 {
		d.Tier = device.TierFromInt(req.TierCap)
	}

	if err := d.Lifecycle.TransitionTo(device.LifecycleRegistered); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidLifecycleTransition, err)
	}
	d.RegisteredAt = time.Now().UnixMilli()

	if err := s.deviceRepo.Update(ctx, d); err != nil {
		return nil, err
	}

	// Consume the code only after the device is bound, so a crash between the two
	// leaves a claimable device rather than a registered one with a live code.
	if err := s.deviceRepo.ClearPairingCode(ctx, d.ID); err != nil {
		return nil, err
	}

	s.logger.Info("device claimed",
		"deviceId", d.ID, "operatorId", req.OperatorID, "organizationId", req.OrganizationID)

	if s.statusUpdater != nil {
		s.statusUpdater.SetDeviceOnline(d.ID, true)
	}

	return d, nil
}
