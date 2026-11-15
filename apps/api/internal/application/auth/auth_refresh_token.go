package auth

import (
	"context"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/shared"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/refresh_token"
)

// RefreshTokenResult holds the result of a refresh token rotation.
type RefreshTokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	SessionID    string
}

// AccessTokenResult holds the result of generating an access token.
type AccessTokenResult struct {
	AccessToken string
	ExpiresAt   int64
}

// GenerateAccessToken generates a JWT access token for a user.
func (s *AuthService) GenerateAccessToken(ctx context.Context, userID, email, name, role string) (*AccessTokenResult, error) {
	if s.jwtManager == nil {
		return nil, errors.New("JWT manager not configured")
	}

	accessToken, expiresAt, err := s.jwtManager.Generate(userID, email, name, role)
	if err != nil {
		return nil, err
	}

	return &AccessTokenResult{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt.Unix(),
	}, nil
}

// RotateRefreshToken rotates a refresh token, revoking the old one and issuing a new one.
func (s *AuthService) RotateRefreshToken(ctx context.Context, oldRefreshToken string) (*RefreshTokenResult, error) {
	// Check if refresh token repository is configured.
	if s.refreshTokenRepo == nil {
		return nil, application.ErrUnauthorized
	}

	// Check if JWT manager is configured.
	if s.jwtManager == nil {
		return nil, application.ErrUnauthorized
	}

	// Hash the incoming token.
	tokenHash := hashTokenSha256(oldRefreshToken)

	// Find the existing refresh token.
	existing, err := s.refreshTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, application.ErrUnauthorized
	}

	// 10: Check if token was used from a different user (cross-account theft!).
	if existing.IsRevoked {
		// Token was already used after rotation - potential theft!.
		// Revoke ALL refresh tokens for the original user (force re-login).
		_ = s.refreshTokenRepo.RevokeAllForUser(ctx, existing.UserID)
		return nil, application.ErrUnauthorized
	}

	// Check if expired.
	if existing.IsExpired() {
		return nil, application.ErrTokenExpired
	}

	// Fetch user details for the JWT claims.
	op, err := s.userRepo.FindByID(ctx, existing.UserID)
	if err != nil {
		return nil, application.ErrUnauthorized
	}

	// Determine the role from organization membership.
	// Role is org-scoped, so we use the user's LastOrganizationID to determine context.
	role := "operator"
	if s.memberRepo != nil {
		if op.LastOrganizationID != "" {
			// Try the user's last accessed org first.
			member, memberErr := s.memberRepo.FindByUserAndOrg(ctx, op.ID, op.LastOrganizationID)
			if memberErr == nil && member.IsActive() {
				role = string(member.Role)
			}
		} else {
			// No LastOrganizationID — load all memberships to find the highest role.
			memberships, memberErr := s.memberRepo.ListByUser(ctx, op.ID)
			if memberErr == nil {
				for _, m := range memberships {
					if m.IsActive() && m.Role.Level() > organization.LevelOperator {
						role = string(m.Role)
						if m.Role.IsSuperAdmin() {
							break
						}
					}
				}
			}
		}
	}

	// Generate new JWT access token with full user details.
	accessToken, expiresAt, err := s.jwtManager.Generate(
		op.ID,
		op.Email,
		op.Name,
		role,
	)
	if err != nil {
		return nil, err
	}

	// Revoke old token.
	_ = s.refreshTokenRepo.Revoke(ctx, existing.ID)

	// Issue new refresh token.
	newRefreshToken, err := s.IssueRefreshToken(ctx, existing.UserID, existing.SessionID)
	if err != nil {
		return nil, err
	}

	return &RefreshTokenResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    expiresAt,
		SessionID:    existing.SessionID,
	}, nil
}

// IssueRefreshToken issues a new refresh token.
func (s *AuthService) IssueRefreshToken(ctx context.Context, userID, sessionID string) (string, error) {
	if s.refreshTokenRepo == nil {
		return "", errors.New("refresh token repository not configured")
	}

	token, err := shared.GenerateToken()
	if err != nil {
		return "", err
	}

	tokenHash := hashTokenSha256(token)
	id := shared.GenerateID()

	// Default to 7 days if refreshTokenExpiry not set.
	expiry := s.refreshTokenExpiry
	if expiry == 0 {
		expiry = 7 * 24 * time.Hour
	}

	rt := &refresh_token.RefreshToken{
		ID:         id,
		UserID: userID,
		SessionID:  sessionID,
		TokenHash:  tokenHash,
		ExpiresAt:  time.Now().Add(expiry),
		CreatedAt:  time.Now(),
	}

	if err := s.refreshTokenRepo.Create(ctx, rt); err != nil {
		return "", err
	}

	return token, nil
}

// RevokeAllRefreshTokens revokes all refresh tokens for a user.
func (s *AuthService) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	return s.refreshTokenRepo.RevokeAllForUser(ctx, userID)
}
