package user

// UserRole represents the role of a user in the system.
// Role hierarchy (higher level = more permissions):.
//   - Viewer (10): Read-only access.
//   - User (50): Standard operations, manage own resources.
//   - Admin (80): Tenant management, manage other users.
//   - SuperAdmin (100): System-wide ownership, can do anything.
type UserRole string

const (
	RoleViewer     UserRole = "viewer"
	RoleOperator   UserRole = "operator"
	RoleAdmin      UserRole = "admin"
	RoleSuperAdmin UserRole = "super_admin"
)

// Role levels for hierarchy comparisons.
const (
	LevelViewer     = 10
	LevelOperator   = 50
	LevelAdmin      = 80
	LevelSuperAdmin = 100
)

// roleLevels maps roles to their permission levels.
var roleLevels = map[UserRole]int{
	RoleViewer:     LevelViewer,
	RoleOperator:   LevelOperator,
	RoleAdmin:      LevelAdmin,
	RoleSuperAdmin: LevelSuperAdmin,
}

// IsValid checks if the role is a valid user role.
func (r UserRole) IsValid() bool {
	switch r {
	case RoleViewer, RoleOperator, RoleAdmin, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

// Level returns the permission level for this role.
func (r UserRole) Level() int {
	if level, ok := roleLevels[r]; ok {
		return level
	}
	return 0
}

// IsAtLeast checks if this role is at least the given role.
func (r UserRole) IsAtLeast(other UserRole) bool {
	return r.Level() >= other.Level()
}

// CanPromoteTo checks if this role can promote to the target role.
func (r UserRole) CanPromoteTo(target UserRole) bool {
	// Can only promote to roles at or below own level, but not equal or higher.
	return r.Level() > target.Level() && r.Level() >= LevelAdmin
}

// CanDemote checks if this role can demote the target role.
func (r UserRole) CanDemote(target UserRole) bool {
	// Can only demote roles strictly below own level.
	return r.Level() > target.Level() && r.Level() >= LevelAdmin
}

// IsAdmin returns true if this role has admin privileges.
func (r UserRole) IsAdmin() bool {
	return r.Level() >= LevelAdmin
}

// IsSuperAdmin returns true if this role has super admin privileges.
func (r UserRole) IsSuperAdmin() bool {
	return r == RoleSuperAdmin
}
