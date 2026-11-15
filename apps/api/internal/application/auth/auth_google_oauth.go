package auth

import (
	"context"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/shared"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// OAuthResult holds the result of OAuth operations.
type OAuthResult struct {
	User *user.User
	IsNew    bool
}

// DefaultOAuthRole is the role assigned to new OAuth users.
// This can be made configurable via environment variables in the future.
const DefaultOAuthRole = user.RoleOperator

// FindOrCreateGoogleUser finds or creates a user by Google ID.
func (s *AuthService) FindOrCreateGoogleUser(ctx context.Context, googleID, email, name string) (*OAuthResult, error) {
	// Try to find by Google ID.
	op, err := s.userRepo.FindByGoogleID(ctx, googleID)
	if err != nil && err != user.ErrNotFound {
		return nil, err
	}

	if op != nil {
		return &OAuthResult{User: op, IsNew: false}, nil
	}

	// Try to find by email and link Google account.
	op, err = s.userRepo.FindByEmail(ctx, email)
	if err != nil && err != user.ErrNotFound {
		return nil, err
	}

	if op != nil {
		// Link existing account to Google.
		if err = s.userRepo.UpdateGoogleID(ctx, op.ID, googleID); err != nil {
			return nil, err
		}

		op.GoogleID = googleID

		return &OAuthResult{User: op, IsNew: false}, nil
	}

	// Create new user.
	id := shared.GenerateID()

	now := time.Now()
	newOp := &user.User{
		ID:            id,
		Email:         email,
		Name:          name,
		GoogleID:      googleID,
		EmailVerified: true, // Google verifies email.
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.userRepo.Create(ctx, newOp); err != nil {
		return nil, err
	}

	return &OAuthResult{User: newOp, IsNew: true}, nil
}

// OAuthLinkError represents an error when linking OAuth accounts.
type OAuthLinkError struct {
	Message string
}

func (e *OAuthLinkError) Error() string {
	return e.Message
}
