package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/shared"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
)

// UpdateUserRequest represents a request to update a user.
type UpdateUserRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
	Role  *string `json:"role,omitempty"`
}

// ListAllUsers returns all users (admin use).
func (s *AuthService) ListAllUsers(ctx context.Context, limit, offset int) ([]dto.UserListResponse, int, error) {
	if limit <= 0 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	users, total, err := s.userRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	response := make([]dto.UserListResponse, len(users))
	for i, op := range users {
		// Note: Role is org-scoped, not per-user. Use first membership role or "user" as default.
		role := "operator"
		if len(op.Memberships) > 0 {
			role = string(op.Memberships[0].Role)
		}
		response[i] = dto.UserListResponse{
			ID:            op.ID,
			Email:         op.Email,
			Name:          op.Name,
			Role:          role,
			MFAEnabled:    op.MFASecret != "",
			EmailVerified: op.EmailVerified,
			CreatedAt:     op.CreatedAt.UnixMilli(),
		}
	}

	return response, total, nil
}

// CreateUser creates a new user (admin only).
func (s *AuthService) CreateUser(ctx context.Context, req *dto.RegisterRequest) (*user.User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))

	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil && err != user.ErrNotFound {
		return nil, err
	}

	if existing != nil {
		return nil, application.ErrEmailExists
	}

	passwordHash, err := s.passwordHasher.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	role := user.RoleOperator //nolint:wastedassign // Role assignment deferred to membership creation.
	if req.Role != "" {
		role = user.UserRole(req.Role)
		if !role.IsValid() {
			return nil, application.ErrInvalidInput
		}
	}

	now := time.Now()
	id := shared.GenerateID()

	op := &user.User{
		ID:           id,
		Email:        email,
		Name:         strings.TrimSpace(req.Name),
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.Create(ctx, op); err != nil {
		return nil, err
	}

	return op, nil
}

// GetUserByID retrieves a user by ID.
func (s *AuthService) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	return s.userRepo.FindByID(ctx, id)
}

// VerifyJWT verifies a JWT token and returns the claims.
func (s *AuthService) VerifyJWT(token string) (*infraauth.UserClaims, error) {
	if s.jwtManager == nil {
		return nil, infraauth.ErrInvalidToken
	}

	return s.jwtManager.Verify(token)
}

// UpdateUser updates an existing user (admin only).
func (s *AuthService) UpdateUser(ctx context.Context, userID string, req *UpdateUserRequest) (*user.User, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, application.ErrInvalidInput
		}
		op.Name = name
	}

	if req.Email != nil {
		existing, err := s.userRepo.FindByEmail(ctx, *req.Email)
		if err != nil && err != user.ErrNotFound {
			return nil, err
		}

		if existing != nil && existing.ID != userID {
			return nil, application.ErrEmailExists
		}

		op.Email = *req.Email
	}

	if req.Role != nil {
		// Role field is intentionally ignored here.
		// Role is org-scoped and updates should be done through organization membership.
		_ = req.Role // Acknowledge the field is set but not used.
	}

	op.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, op); err != nil {
		return nil, err
	}

	return op, nil
}

// DeleteUser deletes a user (admin only).
// Implements the user deletion flow:.
// 1. Check if user is last super_admin of any org - CANNOT delete if so.
// 2. Cancel pending invitations sent by user.
// 3. Remove from all org memberships.
// 4. Delete sessions.
// DeleteUser deletes a user (admin only).
// Implements the user deletion flow:.
// 1. Check if user is last super_admin of any org - CANNOT delete if so.
// 2. Cancel pending invitations sent by user.
// 3. Remove from all org memberships.
// 4. Delete sessions.
// 5. Delete user record.
func (s *AuthService) DeleteUser(ctx context.Context, userID string) error {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return application.ErrUserNotFound
		}
		return err
	}

	// Check if user is the last super_admin of any organization.
	memberships, err := s.memberRepo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, m := range memberships {
		if m.Role.IsSuperAdmin() && m.IsActive() {
			// Check if this is the last super_admin.
			count, err := s.memberRepo.CountSuperAdminsByOrganization(ctx, m.OrganizationID)
			if err != nil {
				return err
			}
			if count <= 1 {
				return application.ErrCannotDeleteLastSuperAdmin
			}
		}
	}

	// Cancel all pending invitations sent by this user.
	if err := s.invitationRepo.SoftDeleteByInvitedBy(ctx, userID); err != nil {
		return err
	}

	// Remove from all organization memberships.
	if err := s.memberRepo.SoftDeleteByUser(ctx, userID); err != nil {
		return err
	}

	// Delete all sessions for this user.
	if err := s.sessionRepo.DeleteByUserID(ctx, userID); err != nil {
		return err
	}

	// Delete the user record.
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return err
	}

	_ = op // op loaded for validation above.
	return nil
}

// DeleteOwnAccount deletes the user's own account.
// Requires password confirmation and follows the deletion flow.
func (s *AuthService) DeleteOwnAccount(ctx context.Context, userID, password string) error {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return application.ErrUserNotFound
		}
		return err
	}

	// Verify password.
	if op.PasswordHash != "" {
		if verifyErr := s.passwordHasher.Verify(password, op.PasswordHash); verifyErr != nil {
			return application.ErrInvalidCredentials
		}
	}

	// Check if user is the last super_admin of any organization.
	memberships, err := s.memberRepo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}

	for _, m := range memberships {
		if m.Role.IsSuperAdmin() && m.IsActive() {
			count, countErr := s.memberRepo.CountSuperAdminsByOrganization(ctx, m.OrganizationID)
			if countErr != nil {
				return countErr
			}
			if count <= 1 {
				return application.ErrCannotDeleteLastSuperAdmin
			}
		}
	}

	// Cancel all pending invitations sent by this user.
	if err := s.invitationRepo.SoftDeleteByInvitedBy(ctx, userID); err != nil {
		return err
	}

	// Remove from all organization memberships.
	if err := s.memberRepo.SoftDeleteByUser(ctx, userID); err != nil {
		return err
	}

	// Delete all sessions for this user.
	if err := s.sessionRepo.DeleteByUserID(ctx, userID); err != nil {
		return err
	}

	// Delete the user record.
	return s.userRepo.Delete(ctx, userID)
}

// GetDevicesByUser returns all devices owned by a user (for deletion warning).
func (s *AuthService) GetDevicesByUser(ctx context.Context, userID string) (int, error) {
	return s.deviceRepo.CountByUser(ctx, userID)
}
