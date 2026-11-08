package deploy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deploycredential"
)

// The deploy credential (§7.4): a long-lived token scoped to one device, so CI
// can deploy without an operator session. It is minted, listed and revoked by an
// operator with `device.deploy`, and it authenticates exactly one device.
//
// The plaintext is returned once, at mint time, and never again. Only its SHA-256
// is stored, so a copy of the database yields no usable credential. A fast hash is
// correct here and a slow one is not: the token is 32 random bytes, so there is
// nothing to brute-force, and this hash runs on every authenticated request.

// CredentialTokenPrefix marks a deploy credential in a log or a header, so a
// leaked one is recognisable as such.
const CredentialTokenPrefix = "vxd"

// CredentialError is a refusal naming the reason. The CLI prints it verbatim.
type CredentialError struct {
	Message string
}

// Error implements error.
func (e *CredentialError) Error() string { return e.Message }

// CredentialService mints, lists and revokes deploy credentials.
type CredentialService struct {
	credentials deploycredential.Repository
	now         func() time.Time
}

// NewCredentialService builds a CredentialService.
func NewCredentialService(credentials deploycredential.Repository) *CredentialService {
	return &CredentialService{credentials: credentials, now: time.Now}
}

// Mint creates a credential for one device and returns the plaintext exactly
// once. The caller is responsible for showing it; nothing here can return it
// again.
func (s *CredentialService) Mint(ctx context.Context, orgID, deviceID, name, createdBy string) (string, *deploycredential.Credential, error) {
	if strings.TrimSpace(deviceID) == "" {
		return "", nil, &CredentialError{Message: "a deploy credential must name a device"}
	}

	if strings.TrimSpace(name) == "" {
		return "", nil, &CredentialError{Message: "a deploy credential needs a name, so it can be revoked by intent"}
	}

	secret, err := randomToken()
	if err != nil {
		return "", nil, fmt.Errorf("mint deploy credential: %w", err)
	}

	now := s.now()
	credential := &deploycredential.Credential{
		ID:        newCredentialID(now),
		OrgID:     orgID,
		DeviceID:  deviceID,
		Name:      name,
		Prefix:    credentialPrefix(secret),
		TokenHash: HashCredential(secret),
		CreatedBy: createdBy,
		CreatedAt: now,
	}

	if err := s.credentials.Create(ctx, credential); err != nil {
		return "", nil, err
	}

	return secret, credential, nil
}

// List returns a device's credentials, without token material.
func (s *CredentialService) List(ctx context.Context, deviceID string) ([]*deploycredential.Credential, error) {
	return s.credentials.ListForDevice(ctx, deviceID)
}

// Revoke marks a credential revoked. A revoked credential is refused immediately,
// because authentication checks the revocation on every request.
func (s *CredentialService) Revoke(ctx context.Context, deviceID, id string) error {
	return s.credentials.Revoke(ctx, deviceID, id, s.now())
}

// Authenticate resolves a presented credential to the device it names. It returns
// ErrCredentialRefused for every failure, so a caller cannot distinguish "no such
// token" from "revoked" and use the difference to probe.
func (s *CredentialService) Authenticate(ctx context.Context, secret string) (*deploycredential.Credential, error) {
	credential, err := s.credentials.FindByHash(ctx, HashCredential(secret))
	if err != nil {
		return nil, ErrCredentialRefused
	}

	if !credential.IsActive() {
		return nil, ErrCredentialRefused
	}

	// Best-effort: recording the use must not fail the request it authenticated.
	_ = s.credentials.Touch(ctx, credential.ID, s.now())

	return credential, nil
}

// ErrCredentialRefused is the single refusal for a deploy credential, so the
// cause is not observable to the presenter.
var ErrCredentialRefused = &CredentialError{Message: "deploy credential is not valid"}

// HashCredential hashes a credential for storage and lookup. SHA-256 over the
// whole token, hex encoded, so the stored value is a fixed length and the lookup
// is an index hit.
func HashCredential(secret string) string {
	sum := sha256.Sum256([]byte(secret))

	return hex.EncodeToString(sum[:])
}

// randomToken returns `vxd_<64 hex>`: 32 bytes of randomness, which is what makes
// the fast hash safe.
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return CredentialTokenPrefix + "_" + hex.EncodeToString(buf), nil
}

// credentialPrefix is the non-secret display form: enough to identify a
// credential in a list, not enough to use it.
func credentialPrefix(secret string) string {
	if len(secret) <= len(CredentialTokenPrefix)+9 {
		return secret
	}

	return secret[:len(CredentialTokenPrefix)+9]
}

// newCredentialID derives a stable, unique id from the clock and randomness. The
// repository has no id generator, and a uuid dependency here would be a package
// for one call.
func newCredentialID(now time.Time) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// A failed read here is fatal to uniqueness, but the caller's Mint has
		// already read 32 bytes from the same source; fall back to the clock alone.
		return fmt.Sprintf("dc_%d", now.UnixNano())
	}

	return fmt.Sprintf("dc_%d_%s", now.UnixNano(), hex.EncodeToString(buf))
}
