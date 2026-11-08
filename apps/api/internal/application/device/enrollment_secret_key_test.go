package device

import (
	"context"
	"errors"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// secretPublicKeyPEM returns a valid RSA public key PEM, the shape a device's
// PURPOSE_DECRYPT key arrives in.
func secretPublicKeyPEM(t *testing.T) string {
	t.Helper()

	priv, err := crypto.GenerateSecretKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pemBytes, err := crypto.EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode key: %v", err)
	}

	return pemBytes
}

// TestEnrollCarriesTheSecretKey proves the device's second key is recorded, so a
// secret can later be wrapped to it.
func TestEnrollCarriesTheSecretKey(t *testing.T) {
	t.Parallel()

	svc, repo := newEnrollmentServiceWithRepo(t)
	ctx := context.Background()

	secretPEM := secretPublicKeyPEM(t)

	if _, err := svc.Enroll(ctx, EnrollRequest{
		DeviceUUID:   "dev-secret",
		PublicKeyPEM: testPublicKey(t),
		SecretKeyPEM: secretPEM,
	}); err != nil {
		t.Fatalf("enroll: %v", err)
	}

	got, err := repo.FindByID(ctx, "dev-secret")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if got.SecretPublicKey != secretPEM {
		t.Fatal("enroll must record the device's secret key")
	}
}

// TestEnrollWithoutASecretKeyIsAllowed proves the key is optional: a device that
// does not support secrets still enrols.
func TestEnrollWithoutASecretKeyIsAllowed(t *testing.T) {
	t.Parallel()

	svc, repo := newEnrollmentServiceWithRepo(t)
	ctx := context.Background()

	if _, err := svc.Enroll(ctx, EnrollRequest{
		DeviceUUID:   "dev-nosecret",
		PublicKeyPEM: testPublicKey(t),
	}); err != nil {
		t.Fatalf("enroll without a secret key must be allowed: %v", err)
	}

	got, err := repo.FindByID(ctx, "dev-nosecret")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if got.SecretPublicKey != "" {
		t.Fatalf("no secret key must be recorded as empty, got %q", got.SecretPublicKey)
	}
}

// TestEnrollRefusesAMalformedSecretKey proves a bad key is refused at the door,
// not stored and then failing when a secret is first set.
func TestEnrollRefusesAMalformedSecretKey(t *testing.T) {
	t.Parallel()

	svc, _ := newEnrollmentServiceWithRepo(t)

	// An EC key is valid PEM but the wrong key type for the envelope.
	_, err := svc.Enroll(context.Background(), EnrollRequest{
		DeviceUUID:   "dev-bad",
		PublicKeyPEM: testPublicKey(t),
		SecretKeyPEM: testPublicKey(t),
	})
	if err == nil {
		t.Fatal("a malformed secret key must be refused")
	}

	if !errors.Is(err, ErrSecretKeyInvalid) {
		t.Fatalf("the refusal must be ErrSecretKeyInvalid, got %v", err)
	}
}
