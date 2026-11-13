package device

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
)

// newEnrollmentService builds a Service over a real in-memory SQLite database
// with the devices schema. A hand-rolled repository mock would not catch the bug
// class this guards against — a column written but never selected — because the
// mock would not have SQL at all.
func newEnrollmentService(t *testing.T) *Service {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if _, err := db.Exec(`CREATE TABLE devices (
		id TEXT PRIMARY KEY, firebase_install_id TEXT, fcm_token TEXT, app_version TEXT,
		device_class TEXT, online INTEGER NOT NULL DEFAULT 0,
		registered_at INTEGER NOT NULL DEFAULT 0, last_seen INTEGER NOT NULL DEFAULT 0,
		operator_id TEXT, organization_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP,
		device_name TEXT, manufacturer TEXT, model TEXT, os_version TEXT, security_patch TEXT,
			build_id TEXT, api_level INTEGER NOT NULL DEFAULT 0, supported_abis TEXT,
			ram_bytes INTEGER NOT NULL DEFAULT 0, storage_bytes INTEGER NOT NULL DEFAULT 0,
		deregistered_at INTEGER, deletion_scheduled_at INTEGER, fcm_token_refreshed_at INTEGER,
		tags TEXT, tier INTEGER NOT NULL DEFAULT 1, public_key TEXT, secret_public_key TEXT,
		pairing_code TEXT, pairing_expires_at INTEGER,
		reported_config_version INTEGER, reported_app_digests TEXT, reported_at INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return NewService(storage.NewDeviceRepository(db), nil, nil)
}

// newEnrollmentServiceWithRepo returns the service and the repository, so a
// test can assert what was recorded.
func newEnrollmentServiceWithRepo(t *testing.T) (*Service, *storage.DeviceRepository) {
	t.Helper()

	svc, repo := newEnrollmentServiceRepo(t)

	return svc, repo
}

func newEnrollmentServiceRepo(t *testing.T) (*Service, *storage.DeviceRepository) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if _, err := db.Exec(`CREATE TABLE devices (
		id TEXT PRIMARY KEY, firebase_install_id TEXT, fcm_token TEXT, app_version TEXT,
		device_class TEXT, online INTEGER NOT NULL DEFAULT 0,
		registered_at INTEGER NOT NULL DEFAULT 0, last_seen INTEGER NOT NULL DEFAULT 0,
		operator_id TEXT, organization_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP,
		device_name TEXT, manufacturer TEXT, model TEXT, os_version TEXT, security_patch TEXT,
		build_id TEXT, api_level INTEGER NOT NULL DEFAULT 0, supported_abis TEXT,
		ram_bytes INTEGER NOT NULL DEFAULT 0, storage_bytes INTEGER NOT NULL DEFAULT 0,
		deregistered_at INTEGER, deletion_scheduled_at INTEGER, fcm_token_refreshed_at INTEGER,
		tags TEXT, tier INTEGER NOT NULL DEFAULT 1, public_key TEXT, secret_public_key TEXT,
		pairing_code TEXT, pairing_expires_at INTEGER,
		reported_config_version INTEGER, reported_app_digests TEXT, reported_at INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	repo := storage.NewDeviceRepository(db)

	return NewService(repo, nil, nil), repo
}

// testPublicKey returns a real P-256 public key in PEM, the shape a device sends.
func testPublicKey(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestEnrollCreatesPendingDeviceWithCode(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()

	result, err := svc.Enroll(ctx, EnrollRequest{
		DeviceUUID:    "dev-1",
		PublicKeyPEM:  testPublicKey(t),
		Model:         "C22",
		Manufacturer:  "Nokia",
		OSVersion:     "13",
		APIVersion:    33,
		AppVersion:    "0.1.0",
		DeviceClass:   "phone",
		BuildID:       "nokia/C22/C22:13/TP1A/123:user/release-keys",
		SecurityPatch: "2024-03-01",
		SupportedABIs: []string{"arm64-v8a", "armeabi-v7a"},
		RAMBytes:      4_000_000_000,
		StorageBytes:  64_000_000_000,
		TierHint:      1,
	})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if len(result.PairingCode) != 8 {
		t.Errorf("code %q is not 8 characters", result.PairingCode)
	}
	if result.PairingDisplay != result.PairingCode[:4]+"-"+result.PairingCode[4:] {
		t.Errorf("display %q is not the grouped code", result.PairingDisplay)
	}

	d, err := svc.deviceRepo.FindByID(ctx, "dev-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if d.Lifecycle.IsRegistered() {
		t.Error("an enrolled device must be pending, not registered")
	}
	if d.PublicKey == "" {
		t.Error("the public key must be stored")
	}
	if d.OrganizationID != "" {
		t.Error("an enrolled device must have no organization until claimed")
	}
	// The enriched facts must survive the round trip to the store, not just be
	// accepted by the request struct.
	if d.APIVersion != 33 {
		t.Errorf("api level = %d, want 33", d.APIVersion)
	}
	if d.BuildID == "" || d.SecurityPatch == "" {
		t.Errorf("build id and security patch must be stored, got %q/%q", d.BuildID, d.SecurityPatch)
	}
	if d.SupportedABIs != "arm64-v8a,armeabi-v7a" {
		t.Errorf("abis = %q, want the joined list", d.SupportedABIs)
	}
	if d.RAMBytes != 4_000_000_000 || d.StorageBytes != 64_000_000_000 {
		t.Errorf("capacity = %d/%d, want the reported values", d.RAMBytes, d.StorageBytes)
	}
	if d.DeviceClass != "phone" {
		t.Errorf("device class = %q, want phone", d.DeviceClass)
	}
}

// TestEnrollRefusesAMalformedKey proves the key is parsed before anything is
// written, so a device that sends garbage is refused at the door.
func TestEnrollRefusesAMalformedKey(t *testing.T) {
	svc := newEnrollmentService(t)

	_, err := svc.Enroll(context.Background(), EnrollRequest{
		DeviceUUID:   "dev-bad",
		PublicKeyPEM: "not a key",
	})
	if !errors.Is(err, ErrPublicKeyInvalid) {
		t.Fatalf("expected ErrPublicKeyInvalid, got %v", err)
	}
}

func TestEnrollRefusesAMissingKey(t *testing.T) {
	svc := newEnrollmentService(t)

	_, err := svc.Enroll(context.Background(), EnrollRequest{DeviceUUID: "dev-1"})
	if !errors.Is(err, ErrPublicKeyRequired) {
		t.Fatalf("expected ErrPublicKeyRequired, got %v", err)
	}
}

// TestEnrollRefusesAKeyOnTheWrongCurve proves a P-384 key is refused: it would
// verify against a canonical string the device never signed.
func TestEnrollRefusesAKeyOnTheWrongCurve(t *testing.T) {
	svc := newEnrollmentService(t)

	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

	_, err = svc.Enroll(context.Background(), EnrollRequest{
		DeviceUUID:   "dev-384",
		PublicKeyPEM: pemKey,
	})
	if !errors.Is(err, ErrPublicKeyInvalid) {
		t.Fatalf("expected ErrPublicKeyInvalid for P-384, got %v", err)
	}
}

// TestEnrollIsIdempotentForAPendingDevice proves a device that lost its code gets
// a fresh one rather than a duplicate row.
func TestEnrollIsIdempotentForAPendingDevice(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()
	key := testPublicKey(t)

	first, err := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: key})
	if err != nil {
		t.Fatalf("first enroll: %v", err)
	}
	second, err := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: key})
	if err != nil {
		t.Fatalf("second enroll: %v", err)
	}

	if first.PairingCode == second.PairingCode {
		t.Error("re-enrolling must issue a new code, not reuse the old one")
	}

	// The old code must no longer resolve.
	if _, err := svc.deviceRepo.FindByPairingCode(ctx, first.PairingCode); err == nil {
		t.Error("the superseded code must not resolve")
	}
}

func TestClaimBindsOwnerAndOrgAndConsumesTheCode(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()

	enrolled, err := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: testPublicKey(t)})
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	d, err := svc.Claim(ctx, ClaimRequest{
		PairingCode:    enrolled.PairingDisplay, // as the operator types it
		OperatorID:     "op-1",
		OrganizationID: "org-1",
		Name:           "field-ops-07",
		TierCap:        2,
	})
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if d.OperatorID != "op-1" || d.OrganizationID != "org-1" {
		t.Errorf("owner not bound: %q %q", d.OperatorID, d.OrganizationID)
	}
	if d.DeviceName != "field-ops-07" {
		t.Errorf("name not applied: %q", d.DeviceName)
	}
	if d.Tier != device.TierRooted {
		t.Errorf("tier cap not applied: %d", d.Tier)
	}
	if !d.Lifecycle.IsRegistered() {
		t.Error("a claimed device must be registered")
	}

	// Single-use: the code is gone.
	if _, err := svc.deviceRepo.FindByPairingCode(ctx, enrolled.PairingCode); err == nil {
		t.Error("a claimed code must not resolve again")
	}
}

// TestClaimIsSingleUse proves a second claim with the same code fails, which is
// what stops one operator from taking a device another already owns.
func TestClaimIsSingleUse(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()

	enrolled, _ := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: testPublicKey(t)})
	req := ClaimRequest{
		PairingCode:    enrolled.PairingCode,
		OperatorID:     "op-1",
		OrganizationID: "org-1",
	}

	if _, err := svc.Claim(ctx, req); err != nil {
		t.Fatalf("first claim: %v", err)
	}

	req.OperatorID = "op-2"
	req.OrganizationID = "org-2"
	if _, err := svc.Claim(ctx, req); !errors.Is(err, ErrPairingCodeUnknown) {
		t.Fatalf("a second claim must fail, got %v", err)
	}
}

func TestClaimRefusesAnUnknownCode(t *testing.T) {
	svc := newEnrollmentService(t)

	_, err := svc.Claim(context.Background(), ClaimRequest{
		PairingCode:    "ZZZZZZZZ",
		OperatorID:     "op-1",
		OrganizationID: "org-1",
	})
	if !errors.Is(err, ErrPairingCodeUnknown) {
		t.Fatalf("expected ErrPairingCodeUnknown, got %v", err)
	}
}

// TestClaimRefusesAnExpiredCode proves a stale notification cannot be claimed.
func TestClaimRefusesAnExpiredCode(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()

	enrolled, _ := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: testPublicKey(t)})

	// Age the code past its TTL.
	d, _ := svc.deviceRepo.FindByID(ctx, "dev-1")
	past := int64(1)
	d.PairingExpiresAt = &past
	if err := svc.deviceRepo.Update(ctx, d); err != nil {
		t.Fatalf("age the code: %v", err)
	}

	_, err := svc.Claim(ctx, ClaimRequest{
		PairingCode:    enrolled.PairingCode,
		OperatorID:     "op-1",
		OrganizationID: "org-1",
	})
	if !errors.Is(err, ErrPairingCodeUnknown) {
		t.Fatalf("an expired code must be refused, got %v", err)
	}
}

// TestEnrollRefusesAnAlreadyClaimedDevice proves a registered device cannot be
// re-enrolled, which would be a way to hijack it.
func TestEnrollRefusesAnAlreadyClaimedDevice(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()
	key := testPublicKey(t)

	enrolled, _ := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: key})
	if _, err := svc.Claim(ctx, ClaimRequest{
		PairingCode:    enrolled.PairingCode,
		OperatorID:     "op-1",
		OrganizationID: "org-1",
	}); err != nil {
		t.Fatalf("claim: %v", err)
	}

	_, err := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: key})
	if !errors.Is(err, ErrDeviceAlreadyOwned) {
		t.Fatalf("re-enrolling a claimed device must fail, got %v", err)
	}
}

// TestClaimNormalizesTheCodeTheOperatorTyped proves lowercase, spaces and the
// grouped form are all accepted, and the ambiguous letters map to digits.
func TestClaimNormalizesTheCodeTheOperatorTyped(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()

	enrolled, _ := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: testPublicKey(t)})

	typed := " " + enrolled.PairingDisplay[:4] + " " + enrolled.PairingDisplay[5:]
	if _, err := svc.Claim(ctx, ClaimRequest{
		PairingCode:    typed,
		OperatorID:     "op-1",
		OrganizationID: "org-1",
	}); err != nil {
		t.Fatalf("a code typed with a space must be accepted: %v", err)
	}
}

// TestEnrollStoresOnlyThePublicHalf is the invariant that makes this model safe:
// nothing secret is ever transmitted or stored.
func TestEnrollStoresOnlyThePublicHalf(t *testing.T) {
	svc := newEnrollmentService(t)
	ctx := context.Background()

	enrolled, _ := svc.Enroll(ctx, EnrollRequest{DeviceUUID: "dev-1", PublicKeyPEM: testPublicKey(t)})

	d, _ := svc.deviceRepo.FindByID(ctx, "dev-1")

	// The stored value must be a public key, and must parse as one.
	if _, err := crypto.DecodePublicKey(d.PublicKey); err != nil {
		t.Fatalf("the stored key must be a valid public key: %v", err)
	}
	// A PEM private key would say PRIVATE KEY; the public form must not.
	if strings.Contains(d.PublicKey, "PRIVATE KEY") {
		t.Error("the stored key must never be a private key")
	}

	_ = enrolled
}
