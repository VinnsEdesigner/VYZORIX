package serviceaccount

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	sadomain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/serviceaccount"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
)

func setupServiceAccountTestDB(t *testing.T) *storage.SQLite {
	t.Helper()
	cfg := storage.DefaultConfig(filepath.Join(t.TempDir(), "sa-test.db"))
	s, err := storage.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestService_CRUD(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sa.ID == "" {
		t.Fatal("expected generated ID")
	}

	accounts, err := service.List(ctx, "org-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("List returned %d, want 1", len(accounts))
	}

	// Cross-org hidden.
	if _, err := service.Get(ctx, "org-2", sa.ID); err != ErrServiceAccountNotFound {
		t.Errorf("cross-org Get: %v", err)
	}

	if err := service.Delete(ctx, "org-1", sa.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := service.Delete(ctx, "org-1", sa.ID); err != ErrServiceAccountNotFound {
		t.Errorf("second Delete: %v", err)
	}
}

func TestService_TokenLifecycle(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	token, fullKey, err := service.CreateToken(ctx, &TokenInput{
		ServiceID: sa.ID,
		Name:      "default",
		Scopes:    []string{"read"},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if fullKey == "" || token.KeyPrefix == "" {
		t.Error("expected full key and prefix")
	}
	if !token.IsUsable() {
		t.Error("expected usable token")
	}

	// Auth validates the key.
	validated, err := service.Authenticate(ctx, fullKey)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if validated.Token.ID != token.ID {
		t.Errorf("expected token %s, got %s", token.ID, validated.Token.ID)
	}
	if validated.OrgID() != "org-1" {
		t.Errorf("authenticated identity org = %q, want org-1", validated.OrgID())
	}

	// Invalid key rejected.
	if _, err := service.Authenticate(ctx, "bogus"); err == nil {
		t.Error("expected auth failure for bogus key")
	}

	// Rotate: old revoked, new usable.
	newToken, newKey, err := service.RotateToken(ctx, token.ID, &TokenInput{
		ServiceID: sa.ID,
		Scopes:    []string{"write"},
	})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if newToken.ID == token.ID {
		t.Error("rotate should create a new token")
	}
	if !newToken.IsUsable() {
		t.Error("expected usable rotated token")
	}
	if _, err := service.Authenticate(ctx, newKey); err != nil {
		t.Errorf("new key auth: %v", err)
	}

	// Old key no longer authenticates.
	if _, err := service.Authenticate(ctx, fullKey); err == nil {
		t.Error("old key should be revoked")
	}

	// Full key never returned again.
	tokens, _ := service.ListTokens(ctx, sa.ID)
	if tokens[0].KeyHash == fullKey {
		t.Error("hash must not be the full key")
	}
}

func TestSecretscan(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	token, _, err := service.CreateToken(ctx, &TokenInput{
		ServiceID: sa.ID,
		Name:      "default",
		Scopes:    []string{"read"},
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	leaks, err := service.Secretscan(ctx, "org-1", []byte("key is "+token.KeyPrefix))
	if err != nil {
		t.Fatalf("Secretscan: %v", err)
	}
	if leaks != 1 {
		t.Errorf("expected 1 leak, got %d", leaks)
	}

	// Clean payload has no leak.
	leaks, _ = service.Secretscan(ctx, "org-1", []byte("nothing to see"))
	if leaks != 0 {
		t.Errorf("expected 0 leaks, got %d", leaks)
	}
}

func TestService_Update(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Rename only.
	name := "  ci-prod  "
	updated, err := service.Update(ctx, "org-1", sa.ID, &name, nil, "", "")
	if err != nil {
		t.Fatalf("Update rename: %v", err)
	}
	if updated.Name != "ci-prod" {
		t.Errorf("expected trimmed name, got %q", updated.Name)
	}
	if !updated.Enabled {
		t.Error("rename must not change enabled")
	}

	// Disable only.
	disabled := false
	updated, err = service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", "")
	if err != nil {
		t.Fatalf("Update disable: %v", err)
	}
	if updated.Enabled {
		t.Error("expected disabled account")
	}
	if updated.Name != "ci-prod" {
		t.Errorf("disable must not change name, got %q", updated.Name)
	}

	// Re-read confirms persistence.
	reloaded, err := service.Get(ctx, "org-1", sa.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.Enabled || reloaded.Name != "ci-prod" {
		t.Errorf("persisted state wrong: enabled=%v name=%q", reloaded.Enabled, reloaded.Name)
	}

	// Empty name is rejected.
	blank := "   "
	if _, err := service.Update(ctx, "org-1", sa.ID, &blank, nil, "", ""); err == nil {
		t.Error("expected error for blank name")
	}

	// Cross-org update is invisible.
	other := "x"
	if _, err := service.Update(ctx, "org-2", sa.ID, &other, nil, "", ""); err != ErrServiceAccountNotFound {
		t.Errorf("cross-org update: %v", err)
	}
}

func TestService_TokenCount(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sa.TokenCount != 0 {
		t.Errorf("fresh account token count = %d, want 0", sa.TokenCount)
	}

	token, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: sa.ID, Name: "one", Scopes: []string{"read"}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	_, _, err = service.CreateToken(ctx, &TokenInput{ServiceID: sa.ID, Name: "two", Scopes: []string{"read"}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	reloaded, err := service.Get(ctx, "org-1", sa.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.TokenCount != 2 {
		t.Errorf("token count = %d, want 2", reloaded.TokenCount)
	}

	// Revoking drops the live count.
	if err := service.RevokeToken(ctx, token.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	reloaded, _ = service.Get(ctx, "org-1", sa.ID)
	if reloaded.TokenCount != 1 {
		t.Errorf("token count after revoke = %d, want 1", reloaded.TokenCount)
	}
}

func TestService_TokenFieldsPersisted(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	expiry := time.Now().Add(24 * time.Hour)
	created, _, err := service.CreateToken(ctx, &TokenInput{
		ServiceID: sa.ID,
		Name:      "expiring",
		Scopes:    []string{"read"},
		ExpiresAt: &expiry,
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	tokens, err := service.ListTokens(ctx, sa.ID)
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].ExpiresAt == nil {
		t.Fatal("expected expiry to round-trip through storage")
	}
	if tokens[0].ExpiresAt.Unix() != expiry.Unix() {
		t.Errorf("expiry = %v, want %v", tokens[0].ExpiresAt, expiry)
	}
	if tokens[0].RevokedAt != nil {
		t.Error("a fresh token must not be revoked")
	}

	// Revoke stamps revoked_at so the client can render the badge.
	if err := service.RevokeToken(ctx, created.ID); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	tokens, _ = service.ListTokens(ctx, sa.ID)
	if tokens[0].RevokedAt == nil {
		t.Error("expected revoked_at to be set")
	}
}

// TestService_DisabledAccountRejectsTokens is the guard for the disable switch
// actually meaning something: a token that is valid and unexpired must stop
// authenticating the moment its owning account is disabled, and start working
// again when the account is re-enabled.
func TestService_DisabledAccountRejectsTokens(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, fullKey, err := service.CreateToken(ctx, &TokenInput{ServiceID: sa.ID, Name: "default", Scopes: []string{"read"}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Baseline: the key authenticates while the account is enabled.
	if _, err := service.Authenticate(ctx, fullKey); err != nil {
		t.Fatalf("enabled account key must authenticate: %v", err)
	}

	disabled := false
	if _, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", ""); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := service.Authenticate(ctx, fullKey); err == nil {
		t.Error("a disabled account's token must not authenticate")
	}

	enabled := true
	if _, err := service.Update(ctx, "org-1", sa.ID, nil, &enabled, "u-1", ""); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	if _, err := service.Authenticate(ctx, fullKey); err != nil {
		t.Errorf("re-enabled account key must authenticate again: %v", err)
	}
}

// TestService_DisableRecordsStateFacts pins that disabling records who, when and
// why, and that re-enabling clears them. A disabled account has to be able to
// explain itself; "enabled: false" alone tells an operator nothing actionable.
func TestService_DisableRecordsStateFacts(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)

	ctx := context.Background()
	sa, err := service.Create(ctx, "org-1", "deploy-bot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	disabled := false
	updated, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", "rotating credentials")
	if err != nil {
		t.Fatalf("disable: %v", err)
	}

	if updated.Enabled {
		t.Error("account should be disabled")
	}
	if updated.DisabledAt == nil {
		t.Error("disabled_at must be stamped")
	}
	if updated.DisabledBy != "u-1" {
		t.Errorf("disabled_by = %q, want u-1", updated.DisabledBy)
	}
	if updated.DisabledReason != "rotating credentials" {
		t.Errorf("disabled_reason = %q, want the supplied reason", updated.DisabledReason)
	}

	// The facts survive a re-read, not just the mutation's return value.
	reread, err := service.Get(ctx, "org-1", sa.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reread.DisabledBy != "u-1" || reread.DisabledReason != "rotating credentials" {
		t.Error("state facts must persist across a read")
	}

	// Re-enabling clears the disable facts: they describe the current disabled
	// state, not a historical event.
	enabled := true
	reenabled, err := service.Update(ctx, "org-1", sa.ID, nil, &enabled, "u-2", "")
	if err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	if reenabled.DisabledAt != nil {
		t.Error("disabled_at must be cleared on re-enable")
	}
	if reenabled.DisabledBy != "" {
		t.Errorf("disabled_by must be cleared, got %q", reenabled.DisabledBy)
	}
	if reenabled.DisabledReason != "" {
		t.Errorf("disabled_reason must be cleared, got %q", reenabled.DisabledReason)
	}
	if reenabled.EnabledBy != "u-2" {
		t.Errorf("enabled_by = %q, want u-2", reenabled.EnabledBy)
	}
}

// TestService_DisabledReasonIsCapped pins the server-side cap: a reason over the
// limit is rejected, the account is left enabled, and nothing is half-written.
//
// Rejecting rather than truncating is deliberate — a person typed the reason and
// expects to see it echoed back; silently shortening it loses information.
func TestService_DisabledReasonIsCapped(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)

	ctx := context.Background()
	sa, err := service.Create(ctx, "org-1", "deploy-bot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	tooLong := strings.Repeat("a", sadomain.MaxDisabledReasonLength+1)
	disabled := false
	if _, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", tooLong); !errors.Is(err, sadomain.ErrDisabledReasonTooLong) {
		t.Fatalf("got %v, want sadomain.ErrDisabledReasonTooLong", err)
	}

	// The rejected call must not have disabled the account as a side effect.
	after, err := service.Get(ctx, "org-1", sa.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !after.Enabled {
		t.Error("a rejected reason must leave the account enabled")
	}
	if after.DisabledReason != "" {
		t.Errorf("a rejected reason must not be stored, got %q", after.DisabledReason)
	}

	// Exactly at the limit is accepted.
	atLimit := strings.Repeat("b", sadomain.MaxDisabledReasonLength)
	updated, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", atLimit)
	if err != nil {
		t.Fatalf("reason at the limit must be accepted: %v", err)
	}
	if len(updated.DisabledReason) != sadomain.MaxDisabledReasonLength {
		t.Errorf("stored reason length = %d, want %d", len(updated.DisabledReason), sadomain.MaxDisabledReasonLength)
	}
}

// TestService_DisabledReasonCountsRunes pins that the cap is measured in
// characters, not bytes: a multi-byte script must not be cut off early.
func TestService_DisabledReasonCountsRunes(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)

	ctx := context.Background()
	sa, err := service.Create(ctx, "org-1", "deploy-bot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Three bytes per rune: byte length far exceeds the cap, rune count does not.
	reason := strings.Repeat("日", sadomain.MaxDisabledReasonLength)
	if len(reason) <= sadomain.MaxDisabledReasonLength {
		t.Fatal("precondition: the fixture should exceed the cap in bytes")
	}

	disabled := false
	updated, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", reason)
	if err != nil {
		t.Fatalf("a reason at the rune limit must be accepted: %v", err)
	}
	if utf8.RuneCountInString(updated.DisabledReason) != sadomain.MaxDisabledReasonLength {
		t.Errorf("stored rune count = %d, want %d", utf8.RuneCountInString(updated.DisabledReason), sadomain.MaxDisabledReasonLength)
	}
}

// TestService_RenameLeavesStateFactsAlone guards the other direction: a rename
// must not stamp or clear the state facts.
func TestService_RenameLeavesStateFactsAlone(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)

	ctx := context.Background()
	sa, err := service.Create(ctx, "org-1", "deploy-bot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	disabled := false
	if _, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", "audit"); err != nil {
		t.Fatalf("disable: %v", err)
	}

	renamed := "deploy-bot-2"
	updated, err := service.Update(ctx, "org-1", sa.ID, &renamed, nil, "u-9", "should be ignored")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if updated.Name != renamed {
		t.Errorf("name = %q, want %q", updated.Name, renamed)
	}
	if updated.DisabledBy != "u-1" || updated.DisabledReason != "audit" {
		t.Error("a rename must not overwrite the state facts")
	}
}

// TestService_DisableIsIdempotentForFacts pins that repeating the same state
// does not move the timestamp, so the recorded time is the moment it happened.
func TestService_DisableIsIdempotentForFacts(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)

	ctx := context.Background()
	sa, err := service.Create(ctx, "org-1", "deploy-bot")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	disabled := false
	first, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-1", "first")
	if err != nil {
		t.Fatalf("disable: %v", err)
	}

	second, err := service.Update(ctx, "org-1", sa.ID, nil, &disabled, "u-2", "second")
	if err != nil {
		t.Fatalf("disable again: %v", err)
	}

	if first.DisabledAt.UnixMilli() != second.DisabledAt.UnixMilli() {
		t.Error("re-disabling an already disabled account must not move disabled_at")
	}
	if second.DisabledBy != "u-1" || second.DisabledReason != "first" {
		t.Error("re-disabling an already disabled account must not overwrite the facts")
	}
}

// TestService_TokenNameUniquePerAccount enforces one token name per service
// account: a duplicate is rejected (case-insensitively) and the rejected attempt
// mints no key, while two different accounts may each own the same name.
func TestService_TokenNameUniquePerAccount(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	first, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	second, err := service.Create(ctx, "org-1", "other-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: first.ID, Name: "nightly", Scopes: []string{"read"}}); err != nil {
		t.Fatalf("first token: %v", err)
	}

	// Same account, same name (different case) is rejected.
	if _, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: first.ID, Name: "Nightly", Scopes: []string{"read"}}); !errors.Is(err, ErrDuplicateTokenName) {
		t.Errorf("duplicate name: got %v, want ErrDuplicateTokenName", err)
	}

	// The rejected attempt must not have created a token.
	tokens, _ := service.ListTokens(ctx, first.ID)
	if len(tokens) != 1 {
		t.Errorf("expected 1 token after a rejected duplicate, got %d", len(tokens))
	}

	// Another account in the same org may use the same name.
	if _, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: second.ID, Name: "nightly", Scopes: []string{"read"}}); err != nil {
		t.Errorf("same name on a different account should be allowed: %v", err)
	}

	// An empty name is rejected.
	if _, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: first.ID, Name: "   ", Scopes: []string{"read"}}); err == nil {
		t.Error("expected an error for a blank token name")
	}
}

// TestService_RotateKeepsOwnName proves rotation can reuse the name of the token
// it replaces, but still refuses to steal a *different* token's name.
func TestService_RotateKeepsOwnName(t *testing.T) {
	s := setupServiceAccountTestDB(t)
	service := NewService(
		storage.NewServiceAccountRepository(s.DB()),
		storage.NewServiceAccountTokenRepository(s.DB()),
	)
	ctx := context.Background()

	sa, err := service.Create(ctx, "org-1", "ci-deployer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	original, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: sa.ID, Name: "nightly", Scopes: []string{"read"}})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if _, _, err := service.CreateToken(ctx, &TokenInput{ServiceID: sa.ID, Name: "other", Scopes: []string{"read"}}); err != nil {
		t.Fatalf("CreateToken other: %v", err)
	}

	// Rotating with no name keeps the original name (not "-rotated").
	rotated, _, err := service.RotateToken(ctx, original.ID, &TokenInput{ServiceID: sa.ID})
	if err != nil {
		t.Fatalf("RotateToken: %v", err)
	}
	if rotated.Name != "nightly" {
		t.Errorf("rotated name = %q, want nightly", rotated.Name)
	}

	// Rotating onto another token's name is refused.
	if _, _, err := service.RotateToken(ctx, rotated.ID, &TokenInput{ServiceID: sa.ID, Name: "other"}); !errors.Is(err, ErrDuplicateTokenName) {
		t.Errorf("rotate onto taken name: got %v, want ErrDuplicateTokenName", err)
	}
}
