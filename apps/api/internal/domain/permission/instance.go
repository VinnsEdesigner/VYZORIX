package permission

// PersonalScopes returns the grants that derive from being authenticated at all,
// with no membership and no instance role. The resource is the caller's own
// account, so every grant is on ScopePersonal and none satisfies an org-scoped
// check. This is the third store in effective(user, org) = personal ∪ instance ∪ org.
func PersonalScopes() ScopedPermissions {
	personalAll := WildcardScope(ScopePersonal)

	return ExpandActionSets(ScopedPermissions{
		{ActionAccountRead, personalAll},
		{ActionAccountWrite, personalAll},
		{ActionInvitationRead, personalAll},
	})
}

// InstanceRole is a user's authority over the deployment itself, as opposed to
// a single organisation. It is orthogonal to the org role ladder: an instance
// admin is not an admin of every org, and an org super_admin holds no instance
// authority. Authority is additive — see DefaultScopesForInstanceRole.
type InstanceRole string

const (
	InstanceNone  InstanceRole = "none"
	InstanceAdmin InstanceRole = "instance_admin"
	// InstanceViewer, InstanceBilling later. An enum rather than a boolean, so a
	// third value costs one line and no call-site rename.
)

// IsValid reports whether r is a known instance role.
func (r InstanceRole) IsValid() bool {
	switch r {
	case InstanceNone, InstanceAdmin:
		return true
	default:
		return false
	}
}

// ParseInstanceRole maps a stored string to an InstanceRole, defaulting to
// InstanceNone for anything unrecognised so an unknown value fails closed.
func ParseInstanceRole(s string) InstanceRole {
	r := InstanceRole(s)
	if !r.IsValid() {
		return InstanceNone
	}
	return r
}

// DefaultScopesForInstanceRole returns the scoped permissions an instance role
// grants. The scope is the deployment, not an organisation, so the grants never
// satisfy an org-scoped check: an instance admin with no membership in an org
// can see the org exists but cannot read its resources.
//
// org.list and org.create are held by every authenticated user: they are the
// bootstrap (a new user must be able to create their first organisation), not a
// privilege. A signup policy can narrow this by not issuing the grant.
func DefaultScopesForInstanceRole(role InstanceRole) ScopedPermissions {
	instanceAll := WildcardScope(ScopeInstance)

	perms := ScopedPermissions{
		{ActionOrgList, instanceAll},
		{ActionOrgCreate, instanceAll},
	}

	if role == InstanceAdmin {
		perms = append(perms,
			ScopedPermission{ActionAdminLockout, instanceAll},
			ScopedPermission{ActionAdminImpersonate, instanceAll},
		)
	}

	return ExpandActionSets(perms)
}

// EffectiveScopes assembles the three grant stores into one set:
//
//	effective(user, org) = personal(user) ∪ instance(user) ∪ org(user, org)
//
// The stores are disjoint by scope prefix, so the union is additive and never a
// superset across axes: instance authority cannot satisfy an org-scoped check,
// and an org role cannot satisfy an instance-scoped one. Passing an empty role
// omits the org store (a caller with no membership in the selected org).
func EffectiveScopes(instanceRole InstanceRole, orgRole string, custom []*ResourcePermission) ScopedPermissions {
	scopes := append(ScopedPermissions{}, PersonalScopes()...)
	scopes = append(scopes, DefaultScopesForInstanceRole(instanceRole)...)
	if orgRole != "" {
		scopes = append(scopes, DefaultScopesForRole(orgRole)...)
	}
	for _, g := range custom {
		for _, a := range g.Actions {
			scopes = append(scopes, ScopedPermission{Action: a, Scope: g.Scope})
		}
	}
	return ExpandActionSets(scopes)
}
