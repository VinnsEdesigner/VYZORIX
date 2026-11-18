package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/devicestore"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/email_verification"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/password_reset"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/refresh_token"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/session"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
	infraSession "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security/session"
)

// PasswordPolicy delegates to domain user package.
type PasswordPolicy = user.PasswordPolicy

// DefaultPasswordPolicy delegates to domain user package.
var DefaultPasswordPolicy = user.DefaultPasswordPolicy

// ValidatePassword delegates to domain user package.
var ValidatePassword = user.ValidatePassword

// PasswordStrength delegates to domain user package.
var PasswordStrength = user.Strength

// PasswordError delegates to domain user package.
type PasswordError = user.PasswordError

// RefreshTokenRepository interface for refresh token operations.
type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *refresh_token.RefreshToken) error
	FindByID(ctx context.Context, id string) (*refresh_token.RefreshToken, error)
	FindByTokenHash(ctx context.Context, tokenHash string) (*refresh_token.RefreshToken, error)
	Revoke(ctx context.Context, id string) error
	RevokeByTokenHash(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	CleanupExpired(ctx context.Context, olderThan time.Duration) (int, error)
}

// SelectOrganizationResult contains the result of selecting an organization.
type SelectOrganizationResult struct {
	Session          *session.Session
	OrganizationID   string
	OrganizationName string
	Role             organization.OrganizationRole
}

// OrganizationInfo contains organization details for client response.
type OrganizationInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// AuthService handles authentication operations.
type AuthService struct {
	refreshTokenRepo   RefreshTokenRepository
	deviceRepo         device.Repository
	orgRepo            organization.OrganizationRepository
	invitationRepo     organization.InvitationRepository
	sessionRepo        session.Repository
	emailVerifyRepo    email_verification.Repository
	passwordResetRepo  password_reset.Repository
	passwordHasher     PasswordHasher
	memberRepo         organization.MemberRepository
	userRepo       user.Repository
	jwtManager         *infraauth.JWTManager
	ldapConfig         *LDAPConfig
	deviceStore        devicestore.Store
	sessionManager     *infraSession.Manager
	logger             *slog.Logger
	sessionTTL         time.Duration
	refreshTokenExpiry time.Duration
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	userRepo user.Repository,
	sessionRepo session.Repository,
	emailVerifyRepo email_verification.Repository,
	passwordResetRepo password_reset.Repository,
	passwordHasher PasswordHasher,
	sessionTTL time.Duration,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		sessionRepo:       sessionRepo,
		emailVerifyRepo:   emailVerifyRepo,
		passwordResetRepo: passwordResetRepo,
		passwordHasher:    passwordHasher,
		sessionTTL:        sessionTTL,
		deviceStore:       NewDeviceStore(),
	}
}

// NewAuthServiceWithRefresh creates a new AuthService with refresh token support.
func NewAuthServiceWithRefresh(
	userRepo user.Repository,
	sessionRepo session.Repository,
	emailVerifyRepo email_verification.Repository,
	passwordResetRepo password_reset.Repository,
	passwordHasher PasswordHasher,
	sessionTTL time.Duration,
	refreshTokenRepo RefreshTokenRepository,
	refreshTokenExpiry time.Duration,
	jwtManager *infraauth.JWTManager,
	ldapConfig *LDAPConfig,
) *AuthService {
	return &AuthService{
		userRepo:       userRepo,
		sessionRepo:        sessionRepo,
		emailVerifyRepo:    emailVerifyRepo,
		passwordResetRepo:  passwordResetRepo,
		passwordHasher:     passwordHasher,
		sessionTTL:         sessionTTL,
		refreshTokenRepo:   refreshTokenRepo,
		refreshTokenExpiry: refreshTokenExpiry,
		jwtManager:         jwtManager,
		ldapConfig:         ldapConfig,
		deviceStore:        NewDeviceStore(),
	}
}

// SetLDAPConfig sets the LDAP configuration for the auth service.
func (s *AuthService) SetLDAPConfig(cfg *LDAPConfig) {
	s.ldapConfig = cfg
}

// SetJWTManager sets the JWT manager for the auth service.
func (s *AuthService) SetJWTManager(jwtManager *infraauth.JWTManager) {
	s.jwtManager = jwtManager
}

// SetSessionManager sets the session manager for the auth service.
func (s *AuthService) SetSessionManager(sessionManager *infraSession.Manager) {
	s.sessionManager = sessionManager
}

// SetInvitationRepository sets the invitation repository for user deletion flow.
func (s *AuthService) SetInvitationRepository(invitationRepo organization.InvitationRepository) {
	s.invitationRepo = invitationRepo
}

// SetDeviceRepository sets the device repository for user deletion flow.
func (s *AuthService) SetDeviceRepository(deviceRepo device.Repository) {
	s.deviceRepo = deviceRepo
}

// SetMemberRepository sets the member repository for organization membership checks.
func (s *AuthService) SetMemberRepository(memberRepo organization.MemberRepository) {
	s.memberRepo = memberRepo
}

// SetOrganizationRepository sets the organization repository.
func (s *AuthService) SetOrganizationRepository(orgRepo organization.OrganizationRepository) {
	s.orgRepo = orgRepo
}

// SetLogger sets the logger for the auth service.
func (s *AuthService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// GetSessionManager returns the session manager.
func (s *AuthService) GetSessionManager() *infraSession.Manager {
	return s.sessionManager
}

// ResolveOrganizationForUser resolves the appropriate organization for a user.
// Priority: LastOrganizationID (if valid) > Single org > Multiple orgs (require selection).
//
// Returns:.
// - orgID, orgName, nil: Single org or auto-selected org.
// - "", "", ErrNoOrganization: No memberships.
// - "", "", ErrOrgSelectionRequired: Multiple orgs require selection.
func (s *AuthService) ResolveOrganizationForUser(ctx context.Context, op *user.User) (string, string, error) {
	// Load memberships if not already loaded.
	if len(op.Memberships) == 0 && s.memberRepo != nil {
		members, err := s.memberRepo.ListByUser(ctx, op.ID)
		if err != nil {
			return "", "", err
		}
		op.Memberships = members
	}

	// Filter to active memberships only.
	var activeMemberships []*organization.OrganizationMember
	for _, m := range op.Memberships {
		if m.IsActive() {
			activeMemberships = append(activeMemberships, m)
		}
	}

	if len(activeMemberships) == 0 {
		return "", "", application.ErrNoOrganization
	}

	// Single active org - auto-select.
	if len(activeMemberships) == 1 {
		m := activeMemberships[0]
		orgName := m.OrganizationID
		if s.orgRepo != nil {
			if org, err := s.orgRepo.FindByID(ctx, m.OrganizationID); err == nil {
				orgName = org.Name
			}
		}
		return m.OrganizationID, orgName, nil
	}

	// Multiple active orgs - check LastOrganizationID first.
	if op.LastOrganizationID != "" {
		// Check if LastOrganizationID is still a valid active membership.
		for _, m := range activeMemberships {
			if m.OrganizationID == op.LastOrganizationID {
				orgName := m.OrganizationID
				if s.orgRepo != nil {
					if org, err := s.orgRepo.FindByID(ctx, m.OrganizationID); err == nil {
						orgName = org.Name
					}
				}
				return m.OrganizationID, orgName, nil
			}
		}
	}

	// Multiple orgs, no valid last org - require selection.
	return "", "", application.ErrOrgSelectionRequired
}

// SelectOrganization switches the session's organization context to the specified organization.
// This allows users with multiple organization memberships to switch between them.
// Also updates the user's LastOrganizationID for auto-selection on next login.
func (s *AuthService) SelectOrganization(ctx context.Context, userID, sessionID, organizationID string) (*SelectOrganizationResult, error) {
	// Validate user has membership in the target organization.
	member, err := s.ValidateOrganizationMembership(ctx, userID, organizationID)
	if err != nil {
		return nil, err
	}

	// Verify the session belongs to this user.
	sess, err := s.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, application.ErrUnauthorized
	}
	if sess.UserID != userID {
		return nil, application.ErrUnauthorized
	}

	// Update session with new organization.
	if err := s.sessionRepo.UpdateOrganizationID(ctx, sessionID, organizationID); err != nil {
		return nil, err
	}

	// Update user's LastOrganizationID for auto-selection on next login.

	if s.userRepo != nil {
		op, err := s.userRepo.FindByID(ctx, userID)
		if err == nil && op != nil {
			op.LastOrganizationID = organizationID
			if err := s.userRepo.Update(ctx, op); err != nil {
				s.logger.Warn("failed to update operator LastOrganizationID",
					"userID", userID,
					"organizationID", organizationID,
					"error", err)
			}
		}
	}

	// Get organization name.
	orgName := organizationID
	if s.orgRepo != nil {
		if org, err := s.orgRepo.FindByID(ctx, organizationID); err == nil {
			orgName = org.Name
		}
	}

	// Update session object for return.
	sess.SelectedOrganizationID = organizationID

	return &SelectOrganizationResult{
		Session:          sess,
		OrganizationID:   organizationID,
		OrganizationName: orgName,
		Role:             member.Role,
	}, nil
}

// ValidateOrganizationMembership checks if a user is a member of an organization.
func (s *AuthService) ValidateOrganizationMembership(ctx context.Context, userID, orgID string) (*organization.OrganizationMember, error) {
	if s.memberRepo == nil {
		return nil, application.ErrForbidden
	}

	member, err := s.memberRepo.FindByUserAndOrg(ctx, userID, orgID)
	if err != nil {
		if err == organization.ErrMemberNotFound {
			return nil, application.ErrForbidden
		}
		return nil, err
	}

	if !member.IsActive() {
		return nil, application.ErrForbidden
	}

	return member, nil
}

// GetUserOrganizations returns all organizations for a user with their roles.
func (s *AuthService) GetUserOrganizations(ctx context.Context, op *user.User) ([]OrganizationInfo, error) {
	// Load memberships if not already loaded.
	if len(op.Memberships) == 0 && s.memberRepo != nil {
		members, err := s.memberRepo.ListByUser(ctx, op.ID)
		if err != nil {
			return nil, err
		}
		op.Memberships = members
	}

	var result []OrganizationInfo
	for _, m := range op.Memberships {
		if !m.IsActive() {
			continue
		}

		orgInfo := OrganizationInfo{
			ID:   m.OrganizationID,
			Role: string(m.Role),
		}

		// Get org name if repo available.
		if s.orgRepo != nil {
			if org, err := s.orgRepo.FindByID(ctx, m.OrganizationID); err == nil {
				orgInfo.Name = org.Name
			}
		}

		result = append(result, orgInfo)
	}

	return result, nil
}
