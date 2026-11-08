package deploy

import (
	"context"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/appsecret"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// memSecrets is an in-memory appsecret.Repository that records exactly what it
// was given, so a test can assert the plaintext never reached it.
type memSecrets struct {
	records map[appsecret.Name]*appsecret.Record
}

func newMemSecrets() *memSecrets {
	return &memSecrets{records: map[appsecret.Name]*appsecret.Record{}}
}

func (m *memSecrets) Get(_ context.Context, name appsecret.Name) (*appsecret.Record, error) {
	rec, ok := m.records[name]
	if !ok {
		return nil, appsecret.ErrNotFound
	}

	return rec, nil
}

func (m *memSecrets) List(_ context.Context, deviceID, app string) ([]*appsecret.Record, error) {
	var out []*appsecret.Record

	for name, rec := range m.records {
		if name.DeviceID == deviceID && name.App == app {
			out = append(out, rec)
		}
	}

	return out, nil
}

func (m *memSecrets) Put(_ context.Context, record *appsecret.Record) (int64, error) {
	name := appsecret.Name{DeviceID: record.DeviceID, App: record.App, Key: record.Key}

	next := int64(1)
	if existing, ok := m.records[name]; ok {
		next = existing.Generation + 1
	}

	stored := *record
	stored.Generation = next
	m.records[name] = &stored

	return next, nil
}

func (m *memSecrets) Delete(_ context.Context, name appsecret.Name) error {
	delete(m.records, name)

	return nil
}

func (m *memSecrets) GenerationForApp(_ context.Context, deviceID, app string) (int64, error) {
	var highest int64

	for name, rec := range m.records {
		if name.DeviceID == deviceID && name.App == app && rec.Generation > highest {
			highest = rec.Generation
		}
	}

	return highest, nil
}

// memKeyLookup returns a fixed public key, whose private half the fixture keeps
// so a test can prove the stored ciphertext actually opens.
type memKeyLookup struct {
	publicPEM string
}

func (m *memKeyLookup) SecretPublicKey(_ context.Context, _ string) (string, error) {
	if m.publicPEM == "" {
		return "", errors.New("no secret key registered")
	}

	return m.publicPEM, nil
}

// secretFixture builds a service whose device key has a known private half.
func secretFixture(t *testing.T) (*SecretService, *memSecrets, *rsa.PrivateKey) {
	t.Helper()

	priv, err := crypto.GenerateSecretKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	publicPEM, err := crypto.EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}

	store := newMemSecrets()
	svc := NewSecretService(store, &memKeyLookup{publicPEM: publicPEM})
	svc.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }

	return svc, store, priv
}

// TestSetStoresOnlyCiphertext is the invariant: the plaintext never reaches the
// store, and what is stored opens with the device's private key.
func TestSetStoresOnlyCiphertext(t *testing.T) {
	t.Parallel()

	svc, store, key := secretFixture(t)
	ctx := context.Background()

	const value = "postgres://user:pw@db.internal:5432/app"

	if _, err := svc.Set(ctx, "d_1", "vfd", "DATABASE_URL", value); err != nil {
		t.Fatalf("set: %v", err)
	}

	rec, err := store.Get(ctx, appsecret.Name{DeviceID: "d_1", App: "vfd", Key: "DATABASE_URL"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if strings.Contains(rec.Ciphertext, value) {
		t.Fatal("the stored ciphertext must not contain the plaintext")
	}

	opened, err := crypto.UnwrapSecret(key, rec.Ciphertext)
	if err != nil {
		t.Fatalf("the stored ciphertext must open with the device key: %v", err)
	}

	if string(opened) != value {
		t.Fatalf("round trip changed the value: %q", opened)
	}
}

// TestSetReplacesAndBumpsGeneration covers rotation: a set for an existing key
// overwrites and bumps, so the device re-fetches.
func TestSetReplacesAndBumpsGeneration(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)
	ctx := context.Background()

	first, err := svc.Set(ctx, "d_1", "vfd", "K", "one")
	if err != nil {
		t.Fatalf("set one: %v", err)
	}

	second, err := svc.Set(ctx, "d_1", "vfd", "K", "two")
	if err != nil {
		t.Fatalf("set two: %v", err)
	}

	if second <= first {
		t.Fatalf("a replacement must bump the generation: %d -> %d", first, second)
	}
}

// TestListNeverReturnsAValue is the console guarantee: names and generations,
// never a value.
func TestListNeverReturnsAValue(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)
	ctx := context.Background()

	const value = "s3cr3t-value-marker"

	if _, err := svc.Set(ctx, "d_1", "vfd", "TOKEN", value); err != nil {
		t.Fatalf("set: %v", err)
	}

	records, err := svc.List(ctx, "d_1", "vfd")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}

	if records[0].Key != "TOKEN" {
		t.Fatalf("list must return the key name, got %q", records[0].Key)
	}

	if strings.Contains(records[0].Ciphertext, value) {
		t.Fatal("list must not return a value")
	}
}

// TestUnsetRemoves proves a removed key is gone.
func TestUnsetRemoves(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)
	ctx := context.Background()

	if _, err := svc.Set(ctx, "d_1", "vfd", "K", "v"); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := svc.Unset(ctx, "d_1", "vfd", "K"); err != nil {
		t.Fatalf("unset: %v", err)
	}

	records, err := svc.List(ctx, "d_1", "vfd")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(records) != 0 {
		t.Fatalf("the key must be gone, got %d", len(records))
	}
}

// TestFetchRefusesAKeyTheConfigDoesNotName is the FR-27 gate: the config, not
// the device's claim, decides what may be fetched.
func TestFetchRefusesAKeyTheConfigDoesNotName(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)
	ctx := context.Background()

	if _, err := svc.Set(ctx, "d_1", "vfd", "DATABASE_URL", "v"); err != nil {
		t.Fatalf("set: %v", err)
	}

	config := "' c\nv device d_1\nvv apps\nvvv app vfd\nvvvv secret OTHER_KEY\n"

	_, err := svc.FetchForDevice(ctx, "d_1", "vfd", "DATABASE_URL", config)
	if err == nil {
		t.Fatal("fetching a key the config does not name must be refused")
	}

	var secretErr *SecretError
	if !errors.As(err, &secretErr) {
		t.Fatalf("expected *SecretError, got %T", err)
	}
}

// TestFetchReturnsCiphertextForANamedKey covers the allowed path.
func TestFetchReturnsCiphertextForANamedKey(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)
	ctx := context.Background()

	if _, err := svc.Set(ctx, "d_1", "vfd", "DATABASE_URL", "v"); err != nil {
		t.Fatalf("set: %v", err)
	}

	config := "' c\nv device d_1\nvv apps\nvvv app vfd\nvvvv secret DATABASE_URL\n"

	rec, err := svc.FetchForDevice(ctx, "d_1", "vfd", "DATABASE_URL", config)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if rec.Ciphertext == "" {
		t.Fatal("fetch must return the ciphertext")
	}
}

// TestFetchDoesNotMatchAnotherAppsNode proves the check reads the app's own
// nodes, not the document text.
func TestFetchDoesNotMatchAnotherAppsNode(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)
	ctx := context.Background()

	if _, err := svc.Set(ctx, "d_1", "vfd", "DATABASE_URL", "v"); err != nil {
		t.Fatalf("set: %v", err)
	}

	config := "' c\nv device d_1\nvv apps\nvvv app wiki\nvvvv secret DATABASE_URL\n"

	if _, err := svc.FetchForDevice(ctx, "d_1", "vfd", "DATABASE_URL", config); err == nil {
		t.Fatal("a key named by another app must not be fetchable")
	}
}

// TestSetRefusesAnEmptyValue proves an empty secret is not silently stored.
func TestSetRefusesAnEmptyValue(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)

	if _, err := svc.Set(context.Background(), "d_1", "vfd", "K", ""); err == nil {
		t.Fatal("an empty value must be refused")
	}
}

// TestSetRefusesAKeyWithWhitespace proves a name cannot smuggle a value.
func TestSetRefusesAKeyWithWhitespace(t *testing.T) {
	t.Parallel()

	svc, _, _ := secretFixture(t)

	if _, err := svc.Set(context.Background(), "d_1", "vfd", "K=V", "v"); err == nil {
		t.Fatal("a key containing '=' must be refused")
	}
}

// TestSetRequiresARegisteredKey proves a device with no secret key is refused
// rather than having its secret wrapped to nothing.
func TestSetRequiresARegisteredKey(t *testing.T) {
	t.Parallel()

	svc := NewSecretService(newMemSecrets(), &memKeyLookup{})

	if _, err := svc.Set(context.Background(), "d_1", "vfd", "K", "v"); err == nil {
		t.Fatal("a device with no secret key must be refused")
	}
}
