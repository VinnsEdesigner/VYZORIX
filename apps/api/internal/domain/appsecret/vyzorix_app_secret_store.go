// Package appsecret is the per-device secret store: a value encrypted to one
// device's key, named by the app that expects it. The value never enters the
// signed config, which names the keys only.
package appsecret

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when no secret is stored for the app and key.
var ErrNotFound = errors.New("app secret not found")

// Record is one stored secret. Ciphertext is the value wrapped to the device's
// key; the plaintext is never held here.
type Record struct {
	UpdatedAt  time.Time
	DeviceID   string
	App        string
	Key        string
	Ciphertext string
	// Generation is bumped on every set, so a device re-fetches only when the
	// value it holds is stale.
	Generation int64
}

// Name is the identity of one secret: a device, an app, and a key.
type Name struct {
	DeviceID string
	App      string
	Key      string
}

// Repository persists the encrypted secrets.
type Repository interface {
	// Get returns one secret, or ErrNotFound.
	Get(ctx context.Context, name Name) (*Record, error)
	// List returns the names and generations for an app on a device, without
	// ciphertext: the operator needs to confirm what is set, never to read it.
	List(ctx context.Context, deviceID, app string) ([]*Record, error)
	// Put stores or replaces a secret and returns the new generation.
	Put(ctx context.Context, record *Record) (int64, error)
	// Delete removes a secret. Deleting an absent one is not an error.
	Delete(ctx context.Context, name Name) error
	// GenerationForApp returns the highest generation across an app's secrets,
	// which is what the device compares to decide whether to re-fetch.
	GenerationForApp(ctx context.Context, deviceID, app string) (int64, error)
}
