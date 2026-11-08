package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// TestSecretKeyRoundTrip proves the device's second key survives a write and a
// read. It is the key a secret is wrapped to, so a value written but never
// selected would silently make every secret unwrappable.
func TestSecretKeyRoundTrip(t *testing.T) {
	t.Parallel()

	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	priv, err := crypto.GenerateSecretKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	secretPEM, err := crypto.EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode key: %v", err)
	}

	identityKey := "-----BEGIN PUBLIC KEY-----\nidentity\n-----END PUBLIC KEY-----"

	d := device.NewDevice("d_1", "")
	d.PublicKey = identityKey
	d.SecretPublicKey = secretPEM

	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.FindByID(ctx, "d_1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if got.SecretPublicKey != secretPEM {
		t.Fatal("the secret key must survive the round trip")
	}

	// The two keys must not be confused for each other.
	if got.PublicKey != identityKey {
		t.Fatal("the identity key must be unchanged")
	}
}

// TestSecretKeyLookupServesTheDeviceKey proves the lookup the secret service
// uses returns the stored key.
func TestSecretKeyLookupServesTheDeviceKey(t *testing.T) {
	t.Parallel()

	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	lookup := NewDeviceSecretKeyRepository(db)
	ctx := context.Background()

	priv, err := crypto.GenerateSecretKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	secretPEM, err := crypto.EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode key: %v", err)
	}

	d := device.NewDevice("d_1", "")
	d.SecretPublicKey = secretPEM

	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := lookup.SecretPublicKey(ctx, "d_1")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	if got != secretPEM {
		t.Fatal("the lookup must return the stored secret key")
	}
}

// TestSecretKeyLookupRefusesADeviceWithoutOne proves an empty key is a refusal,
// not an empty string: a caller must not be able to wrap a secret to nothing.
func TestSecretKeyLookupRefusesADeviceWithoutOne(t *testing.T) {
	t.Parallel()

	db := newTestDeviceDB(t)
	repo := NewDeviceRepository(db)
	lookup := NewDeviceSecretKeyRepository(db)
	ctx := context.Background()

	d := device.NewDevice("d_1", "")

	if err := repo.Create(ctx, d); err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err := lookup.SecretPublicKey(ctx, "d_1")
	if err == nil {
		t.Fatal("a device with no secret key must be refused")
	}

	if !strings.Contains(err.Error(), "no secret key") {
		t.Fatalf("the refusal must say why, got %q", err)
	}
}

// TestSecretKeyLookupRefusesAnUnknownDevice covers the absent case.
func TestSecretKeyLookupRefusesAnUnknownDevice(t *testing.T) {
	t.Parallel()

	lookup := NewDeviceSecretKeyRepository(newTestDeviceDB(t))

	if _, err := lookup.SecretPublicKey(context.Background(), "absent"); err == nil {
		t.Fatal("an unknown device must be refused")
	}
}
