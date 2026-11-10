// Package deploycredential is the deploy credential: a long-lived token scoped
// to a single device, so CI can deploy without an operator session
//.
//
// It is deliberately not a general API key. A credential that could reach the
// fleet would turn one leaked CI secret into a fleet-wide write; scoping it to
// one device means a leak reaches one device's config.
package deploycredential

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when no credential matches.
var ErrNotFound = errors.New("deploy credential not found")

// Credential is one minted credential. Only the hash of the secret is stored, so
// the plaintext exists once, at mint time, and is never returned by any GET.
type Credential struct {
	CreatedAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	ID         string
	OrgID      string
	DeviceID   string
	Name       string
	Prefix     string
	TokenHash  string
	CreatedBy  string
}

// IsActive reports whether the credential may still authenticate.
func (c *Credential) IsActive() bool {
	return c.RevokedAt == nil
}

// Repository persists the credentials. Every method is device-scoped: a
// credential names exactly one device and no lookup can widen that.
type Repository interface {
	// Create stores a new credential.
	Create(ctx context.Context, credential *Credential) error
	// FindByHash returns the credential for a token hash, or ErrNotFound.
	FindByHash(ctx context.Context, tokenHash string) (*Credential, error)
	// ListForDevice returns a device's credentials, without token material.
	ListForDevice(ctx context.Context, deviceID string) ([]*Credential, error)
	// Revoke marks a credential revoked. Revoking an already-revoked credential is
	// not an error: the caller asked for a state, not a transition.
	Revoke(ctx context.Context, deviceID, id string, at time.Time) error
	// Touch records the last use, which is best-effort and never blocks auth.
	Touch(ctx context.Context, id string, at time.Time) error
}
