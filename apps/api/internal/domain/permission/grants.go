package permission

import "context"

// SubjectType identifies whom a ResourcePermission is granted to.
type SubjectType string

const (
	SubjectOperator SubjectType = "operator"
	SubjectTeam     SubjectType = "team"
	SubjectBuiltin  SubjectType = "builtin"
)

// ResourcePermission is a first-class persisted permission grant: one or more
// actions on a scope, granted to a subject (operator, team, or built-in role)
// within an organization. This is the unit of grant in the scoped RBAC system —
// assignable to operators OR teams OR built-in roles, with managed/inherited
// lifecycle flags.
type ResourcePermission struct {
	OrgID       string
	ID          string
	SubjectType SubjectType
	SubjectID   string
	Scope       string
	Actions     []Action
	IsManaged   bool
	IsInherited bool
	CreatedAt   int64
	UpdatedAt   int64
}

// Grant is a compatibility alias for ResourcePermission restricted to a single
// action granted to an operator. Existing call sites that build a Grant continue
// to work; new code should construct ResourcePermission directly.
type Grant = ResourcePermission

// GrantRepository persists ResourcePermissions. The canonical loader is
// ListEffective, which assembles an operator's effective grants (operator
// grants UNION team grants) so the evaluator sees the full set in one query.
type GrantRepository interface {
	// Save upserts a ResourcePermission (idempotent on subject+scope+actions).
	Save(ctx context.Context, p *ResourcePermission) error
	// ListEffective returns all grants an operator can act with in an org:
	// operator-direct grants UNION grants on teams the operator belongs to.
	ListEffective(ctx context.Context, operatorID, orgID string) ([]*ResourcePermission, error)
	// ListByOrg returns all ResourcePermissions in an org (admin view).
	ListByOrg(ctx context.Context, orgID string) ([]*ResourcePermission, error)
	// Revoke deletes a grant by id, returning whether one was removed.
	Revoke(ctx context.Context, id string) (bool, error)
	// RevokeForSubject removes all grants for a subject (e.g. when a team is deleted).
	RevokeForSubject(ctx context.Context, subjectType SubjectType, subjectID string) error
}

// SessionCapability is the resolved authorization a session carries to the
// device. It is deliberately flat: identity, the resolved API allowlist, the
// device tier, and an expiry. It carries no role, no scope and no tags — the
// device must not re-derive authority, because a token that could be reasoned
// about could disagree with the server, and because the token is the only
// cryptographically-bound artefact.
//
// The API allowlist is a filter on top of the capability, not the capability
// itself: for anything with a file descriptor (a PTY, a USB handle) the fd is the
// real revocable grant, and closing it revokes access (architecture §9.1).
type SessionCapability struct {
	SessionID    string
	DeviceID     string
	OperatorID   string
	APIAllowlist []string
	Tier         Tier
	ExpiresAt    int64
}

// Permits reports whether the session may call api. Fail-closed: an empty
// allowlist permits nothing. This is the opposite of the device bridge's current
// behaviour, where an empty allowlist means "every API"; that inversion is the
// single most dangerous line in the bridge and this model forbids it.
func (c SessionCapability) Permits(api string) bool {
	if c.SessionID == "" || c.DeviceID == "" {
		return false
	}
	for _, allowed := range c.APIAllowlist {
		if allowed == api {
			return true
		}
	}
	return false
}

// ResolveSessionCapability is the whole server-side decision in one call: given a
// role, its custom grants, the target device, and the device's tier, it produces
// the capability the device will enforce. It composes both axes — the tag
// requirement (capability) and the scoped evaluator (resource) — and intersects
// on the device tier.
//
// The allowlist is deny-by-default: an API absent from the catalog, a role that
// lacks its tags, or a device below its minimum tier all leave it out. A role
// with no matching grant therefore receives an empty allowlist and can call
// nothing, which is the correct outcome rather than an error.
//
// ctx is threaded through rather than replaced with context.Background():
// GrantsResolved consults scope resolvers, which may read the database, so the
// call must honour the request's cancellation and deadline like any other.
func ResolveSessionCapability(
	ctx context.Context,
	sessionID string,
	role string,
	custom []*ResourcePermission,
	deviceID string,
	deviceTier Tier,
	expiresAt int64,
) SessionCapability {
	eval := NewEvaluator(role, custom)
	deviceScope := BuildScope(ScopeDevices, deviceID)

	return resolveWith(ctx, sessionID, role, eval, deviceID, deviceScope, deviceTier, expiresAt)
}

// resolveWith is the shared body. It applies the two axes in one pass so neither
// can be forgotten:
//
//	capability = catalog[api].Tags.SatisfiedBy(roleTags(role))
//	resource   = evaluator.Grants(api.Action, deviceScope)
//	allowed    = capability && resource && tier >= api.MinTier
func resolveWith(
	ctx context.Context,
	sessionID string,
	role string,
	eval *Evaluator,
	deviceID string,
	deviceScope string,
	deviceTier Tier,
	expiresAt int64,
) SessionCapability {
	grants := DefaultTagsForRole(role)

	allowlist := make([]string, 0, len(DeviceAPICatalog))
	for api, req := range DeviceAPICatalog {
		if deviceTier < req.MinTier {
			continue
		}
		if !req.Tags.SatisfiedBy(grants) {
			continue
		}
		if !eval.GrantsResolved(ctx, req.Action, deviceScope) {
			continue
		}
		allowlist = append(allowlist, api)
	}

	return SessionCapability{
		SessionID:    sessionID,
		DeviceID:     deviceID,
		APIAllowlist: allowlist,
		Tier:         deviceTier,
		ExpiresAt:    expiresAt,
	}
}
