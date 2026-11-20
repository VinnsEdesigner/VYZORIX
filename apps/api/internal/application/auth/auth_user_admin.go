package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/shared"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
	infraSession "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security/session"
)

// ListUsersQuery narrows an admin user list request.
type ListUsersQuery struct {
	Search string
	Sort   string
	// OrgID is the organisation the administrator is acting in. It is the
	// authorisation boundary of the listing: only members of this org are returned.
	OrgID string
	// Status filters the membership lifecycle. Empty means every state except
	// removed, so a caller sees suspended members too unless it asks otherwise.
	Status string
	Desc   bool
	Page   int
	Limit  int
}

// maxAdminUserPageSize caps the page size for the admin user list.
const maxAdminUserPageSize = 100

// ListOrgUsers returns a page of the organisation's members. The listing is
// joined on membership, so an administrator of one organisation can never
// enumerate another's accounts: there is no deployment-wide user list reachable
// with an org role.
func (s *AuthService) ListOrgUsers(ctx context.Context, q ListUsersQuery) ([]dto.OrgUserResponse, int, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > maxAdminUserPageSize {
		limit = maxAdminUserPageSize
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, total, err := s.userRepo.ListByOrganization(ctx, q.OrgID, limit, offset, q.Search, q.Sort, q.Desc, q.Status)
	if err != nil {
		return nil, 0, err
	}

	out := make([]dto.OrgUserResponse, len(rows))
	for i, row := range rows {
		out[i] = orgUserResponse(row)
	}

	return out, total, nil
}

// orgUserResponse maps a membership-joined row to the wire shape.
func orgUserResponse(row *user.OrganizationUser) dto.OrgUserResponse {
	resp := dto.OrgUserResponse{
		ID:            row.ID,
		Email:         row.Email,
		Name:          row.Name,
		AvatarURL:     row.AvatarURL,
		Role:          row.Role,
		Status:        row.Status,
		JoinedAt:      row.JoinedAt.UnixMilli(),
		CreatedAt:     row.CreatedAt.UnixMilli(),
		MFAEnabled:    row.MFAEnabled,
		EmailVerified: row.EmailVerified,
		Disabled:      row.Disabled,
		MemberID:      row.MemberID,
	}
	if row.LastSeenAt != nil {
		ms := row.LastSeenAt.UnixMilli()
		resp.LastSeenAt = &ms
	}

	return resp
}

// OrgUserStats is the per-role member count plus the pending invitation count for
// one organisation.
type OrgUserStats struct {
	ByRole         map[string]int
	Total          int
	PendingInvites int
}

// UserStats counts the organisation's members by role and its pending
// invitations. Counting here rather than in the client keeps the tab header one
// query instead of a page-by-page tally.
func (s *AuthService) UserStats(ctx context.Context, orgID string) (*OrgUserStats, error) {
	rows, err := s.memberRepo.FindByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}

	stats := &OrgUserStats{ByRole: map[string]int{}}
	for _, m := range rows {
		if m.Lifecycle == organization.MemberLifecycleRemoved {
			continue
		}
		stats.ByRole[string(m.Role)]++
		stats.Total++
	}

	pending, err := s.invitationRepo.CountPendingByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	stats.PendingInvites = pending

	return stats, nil
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

// GetUserByID retrieves a user with memberships loaded, so the role resolves.
func (s *AuthService) GetUserByID(ctx context.Context, id string) (*user.User, error) {
	op, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.memberRepo != nil {
		if memberships, err := s.memberRepo.ListByUser(ctx, id); err == nil {
			op.Memberships = memberships
		}
	}
	return op, nil
}

// GetOrgUser returns a user only if they hold a membership in orgID. The
// membership is the authorisation boundary: a caller acting in one organisation
// must not be able to read an account that belongs to another, so a missing
// membership is reported as "not found" rather than "forbidden" — the caller
// learns nothing about accounts outside its org.
func (s *AuthService) GetOrgUser(ctx context.Context, orgID, userID string) (*user.OrganizationUser, error) {
	if s.memberRepo == nil {
		return nil, application.ErrInternal
	}

	member, err := s.memberRepo.FindByUserAndOrg(ctx, userID, orgID)
	if err != nil {
		if errors.Is(err, organization.ErrMemberNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}

	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}

	out := &user.OrganizationUser{
		User:     *op,
		MemberID: member.ID,
		Role:     string(member.Role),
		Status:   string(member.Lifecycle),
		JoinedAt: member.JoinedAt,
	}
	if last, ok, lastErr := s.userRepo.LastLoginAt(ctx, userID); lastErr == nil && ok {
		out.LastSeenAt = &last
	}

	return out, nil
}

// VerifyJWT verifies a JWT token and returns the claims.
func (s *AuthService) VerifyJWT(token string) (*infraauth.UserClaims, error) {
	if s.jwtManager == nil {
		return nil, infraauth.ErrInvalidToken
	}

	return s.jwtManager.Verify(token)
}

// SetUserDisabled stamps or clears disabled_at. Disabling also revokes sessions.
func (s *AuthService) SetUserDisabled(ctx context.Context, userID string, disabled bool) (*user.User, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}

	var at *time.Time
	if disabled {
		now := time.Now()
		at = &now
	}
	if err := s.userRepo.SetDisabled(ctx, userID, at); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}

	if disabled && s.sessionRepo != nil {
		_ = s.sessionRepo.RevokeAllUserSessions(ctx, userID)
	}

	op.Disabled = disabled
	if disabled {
		op.DisabledAt = at
	} else {
		op.DisabledAt = nil
	}
	return op, nil
}

// ListUserSessions returns a user's active sessions (admin view).
func (s *AuthService) ListUserSessions(ctx context.Context, userID string) ([]*infraSession.Session, error) {
	if s.sessionManager == nil {
		return nil, application.ErrInternal
	}
	if _, err := s.userRepo.FindByID(ctx, userID); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUserNotFound
		}
		return nil, err
	}
	return s.sessionManager.ListActiveSessions(ctx, userID)
}

// RevokeUserSession revokes one of the user's sessions. Ownership is checked, so
// a guessed ID cannot revoke an unrelated session.
func (s *AuthService) RevokeUserSession(ctx context.Context, userID, sessionID string) error {
	if s.sessionManager == nil {
		return application.ErrInternal
	}
	sessions, err := s.sessionManager.ListActiveSessions(ctx, userID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		if sess.ID == sessionID {
			return s.sessionManager.RevokeSession(ctx, sessionID)
		}
	}
	return application.ErrUserNotFound
}

// RevokeAllUserSessions revokes every active session of a user.
func (s *AuthService) RevokeAllUserSessions(ctx context.Context, userID string) (int, error) {
	if s.sessionManager == nil {
		return 0, application.ErrInternal
	}
	sessions, err := s.sessionManager.ListActiveSessions(ctx, userID)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, sess := range sessions {
		if err := s.sessionManager.RevokeSession(ctx, sess.ID); err == nil {
			count++
		}
	}
	return count, nil
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
