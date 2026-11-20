// Package deployconfig is the per-device signed config store: the current
// version and the last known-good one, retained so a failed apply can be undone
// without a network. It is deliberately not the org-scoped configversion
// store; a device config is per-device and signed.
package deployconfig

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a device has no stored config.
var ErrNotFound = errors.New("deploy config not found")

// Kind distinguishes the two retained configs.
type Kind string

const (
	// KindCurrent is the config the device should be running.
	KindCurrent Kind = "current"
	// KindLastGood is the config that last applied successfully, kept for the
	// one rollback that matters: put back what was working.
	KindLastGood Kind = "last_good"
)

// Retention is the number of configs kept per device. Not N — the recovery path
// that matters is one step back, and a device's disk is the scarce resource.
const Retention = 2

// Record is one retained config. Document is the signed vyzorix.v text stored
// verbatim, so a re-check sees the same bytes that were signed.
type Record struct {
	CreatedAt time.Time
	AppliedAt time.Time
	DeviceID  string
	Document  string
	Kind      Kind
	Version   int
}

// Repository persists the retained configs. Every method is scoped to one
// device: deploy is one device per call, never a fleet.
type Repository interface {
	// Current returns the config the device should be running, or ErrNotFound.
	//
	// It takes the organisation as well as the device, so a device in another tenant
	// reads as not-found rather than as a config the caller should not see. The org is a
	// property of the device, so the store enforces it with a join; a caller cannot
	// forget the check.
	Current(ctx context.Context, orgID, deviceID string) (*Record, error)
	// LastGood returns the config that last applied, scoped to an organisation.
	LastGood(ctx context.Context, orgID, deviceID string) (*Record, error)
	// CurrentForDevice returns the config the device should be running, keyed by device
	// id alone. It is for the device's own signed read, where the device is the
	// authenticated principal; user-facing reads use Current and its org check.
	CurrentForDevice(ctx context.Context, deviceID string) (*Record, error)
	// SaveCurrent stores a new current config, demoting the previous current to
	// last-good and dropping any older last-good. The demotion and the drop are
	// one transaction: a half-applied retention would leave two last-goods.
	SaveCurrent(ctx context.Context, record *Record) error
	// MarkApplied records that the device reported a version as applied.
	MarkApplied(ctx context.Context, deviceID string, version int, at time.Time) error
}
