// Package serviceaccount provides non-human identities for automation:
// named, org-scoped service accounts with scoped tokens. They carry the same
// scope model as API keys (read/write/admin) but are not attached to an
// user, so key rotation and audit stay separate from human auth.
package serviceaccount

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// ErrNotFound is returned when a service account is not found.
var ErrNotFound = errors.New("service account not found")

// Scope defines the permission level for a service account token.
type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
	ScopeAdmin Scope = "admin"
)

// Valid reports whether the scope is known.
func (s Scope) Valid() bool {
	switch s {
	case ScopeRead, ScopeWrite, ScopeAdmin:
		return true
	}
	return false
}

// MaxDisabledReasonLength bounds the free-text reason recorded when an account
// is disabled.
//
// Counted in runes rather than bytes so a multi-byte script is not cut short,
// and deliberately small: the reason is a one-line annotation rendered inside a
// notice, not a paragraph. Rejecting an over-long value beats truncating it —
// silently shortening something a person typed and expects to see echoed back
// loses information without telling anyone.
const MaxDisabledReasonLength = 280

// ErrDisabledReasonTooLong is returned when a reason exceeds the cap.
var ErrDisabledReasonTooLong = errors.New("disabled reason is too long")

// ServiceAccount is a named, org-scoped identity for automation.
type ServiceAccount struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	// DisabledAt is when the account was last disabled; nil while enabled.
	DisabledAt *time.Time
	ID         string
	OrgID      string
	Name       string
	// DisabledBy and EnabledBy are the user IDs of the last actor to change the
	// state. Stored as IDs so the fact survives a rename or a deleted actor; the
	// display name is resolved on read.
	DisabledBy     string
	EnabledBy      string
	DisabledReason string
	// DisabledByName and EnabledByName are resolved on read from the ids above;
	// Save ignores them.
	DisabledByName string
	EnabledByName  string
	Enabled        bool
	// TokenCount is set by read queries only; Save ignores it.
	TokenCount int
}

// SetEnabled records a state change together with the actor who made it.
//
// The facts are stamped in the same step as the flag, so a disabled account
// always has a disabled_at and a disabled_by and the two can never disagree.
// Re-enabling clears the disable facts rather than keeping stale ones: "who
// disabled it" describes the current disabled state, not a historical event —
// the audit log holds the history.
func (sa *ServiceAccount) SetEnabled(enabled bool, actorID, reason string) error {
	if enabled {
		sa.Enabled = true
		sa.DisabledAt = nil
		sa.DisabledBy = ""
		sa.DisabledReason = ""
		sa.EnabledBy = actorID

		return nil
	}

	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > MaxDisabledReasonLength {
		return ErrDisabledReasonTooLong
	}

	now := time.Now()
	sa.Enabled = false
	sa.DisabledAt = &now
	sa.DisabledBy = actorID
	sa.DisabledReason = reason

	return nil
}

// Validate checks the service account is well-formed.
func (sa *ServiceAccount) Validate() error {
	if strings.TrimSpace(sa.OrgID) == "" {
		return errors.New("org_id is required")
	}
	if strings.TrimSpace(sa.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

// Rename sets a new name after validating it is non-empty.
func (sa *ServiceAccount) Rename(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("name is required")
	}
	sa.Name = trimmed
	return nil
}

// Token is one API token of a service account. The full key is only returned
// at creation; subsequent lookups see only the prefix + hash.
type Token struct {
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	ID         string
	ServiceID  string
	Name       string
	KeyHash    string
	KeyPrefix  string
	// Scopes are the coarse grants, the floor under the token's roles.
	Scopes []string
	// BasicRole is the org role the token acts as (viewer/operator/admin).
	BasicRole string
	// Roles are the fixed-role grants, each at a breadth.
	Roles []permission.FixedRoleGrant
	Valid bool
}

// IsExpired reports whether the token is past its expiry.
func (t *Token) IsExpired() bool {
	return t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt)
}

// IsUsable reports whether the token can authenticate.
func (t *Token) IsUsable() bool {
	return t.Valid && !t.IsExpired() && t.RevokedAt == nil
}

// Validate checks the token is well-formed.
func (t *Token) Validate() error {
	if strings.TrimSpace(t.ServiceID) == "" {
		return errors.New("service_id is required")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("name is required")
	}
	if len(t.Scopes) == 0 && len(t.Roles) == 0 && t.BasicRole == "" {
		return errors.New("a token needs a basic role or at least one fixed role")
	}
	for _, scope := range t.Scopes {
		if !Scope(scope).Valid() {
			return fmt.Errorf("invalid scope %q", scope)
		}
	}
	return nil
}

// Repository persists service accounts.
type Repository interface {
	Save(ctx context.Context, sa *ServiceAccount) error
	GetByID(ctx context.Context, id string) (*ServiceAccount, error)
	ListByOrg(ctx context.Context, orgID string) ([]*ServiceAccount, error)
	ListAllOrgs(ctx context.Context) ([]string, error)
	Delete(ctx context.Context, id string) (bool, error)
}

// TokenRepository persists service account tokens.
type TokenRepository interface {
	Save(ctx context.Context, token *Token) error
	GetByID(ctx context.Context, id string) (*Token, error)
	GetByKeyHash(ctx context.Context, keyHash string) (*Token, error)
	GetByPrefix(ctx context.Context, prefix string) (*Token, error)
	ListByService(ctx context.Context, serviceID string) ([]*Token, error)
	ListExpired(ctx context.Context, now time.Time) ([]*Token, error)
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
	TouchLastUsed(ctx context.Context, id string, usedAt time.Time) error
	Delete(ctx context.Context, id string) (bool, error)
}

// EffectivePermissions is the set of scoped permissions this token may exercise:
// its basic role's defaults unioned with its fixed-role grants.
//
// Roles are the only source. The token also carries coarse Scopes, but those are
// a legacy display value and are NOT consulted here — two independent permission
// sources meant a token created with roles alone still silently received every
// org-scoped read action from a defaulted "read" scope.
func (t *Token) EffectivePermissions() permission.ScopedPermissions {
	perms := permission.ScopedPermissions{}

	if t.BasicRole != "" {
		perms = append(perms, permission.DefaultScopesForRole(t.BasicRole)...)
	}

	for _, grant := range t.Roles {
		perms = append(perms, grant.ScopedPermissions()...)
	}

	return permission.ExpandActionSets(perms)
}
