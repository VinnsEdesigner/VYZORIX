package organization

import (
	"context"
)

// OrganizationRepository defines the interface for organization data access.
type OrganizationRepository interface {
	// Create creates a new organization.
	Create(ctx context.Context, org *Organization) error

	// FindByID retrieves an organization by ID.
	FindByID(ctx context.Context, id string) (*Organization, error)

	// FindByOrganizationID retrieves an organization by OrganizationID value object.
	FindByOrganizationID(ctx context.Context, id OrganizationID) (*Organization, error)

	// FindByName finds an organization by name for a specific user.
	FindByName(ctx context.Context, userID, name string) (*Organization, error)

	// FindByNameAndUserID finds an organization by name for a specific user.
	FindByNameAndUserID(ctx context.Context, userID UserID, name string) (*Organization, error)

	// Update updates an existing organization.
	Update(ctx context.Context, org *Organization) error

	// SoftDelete soft-deletes an organization.
	SoftDelete(ctx context.Context, id string) error

	// SoftDeleteByID soft-deletes an organization by OrganizationID.
	SoftDeleteByID(ctx context.Context, id OrganizationID) error

	// ListByUser lists all organizations for a user (via memberships).
	ListByUser(ctx context.Context, userID string) ([]*Organization, error)

	// ListByUserID lists all organizations for an UserID.
	ListByUserID(ctx context.Context, userID UserID) ([]*Organization, error)

	// ListByUserPaginated lists organizations for a user with pagination.
	ListByUserPaginated(ctx context.Context, userID string, limit, offset int) ([]*Organization, int, error)

	// ListActive lists all active (non-archived) organizations.
	ListActive(ctx context.Context) ([]*Organization, error)

	// CountActiveMembers counts the number of active members in an organization.
	CountActiveMembers(ctx context.Context, orgID string) (int, error)

	// CountActiveMembersByID counts the number of active members by OrganizationID.
	CountActiveMembersByID(ctx context.Context, orgID OrganizationID) (int, error)
}
