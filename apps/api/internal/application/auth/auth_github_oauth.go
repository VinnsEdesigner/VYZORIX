package auth

import (
	"context"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/shared"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// FindOrCreateGitHubUser finds or creates a user by GitHub ID.
func (s *AuthService) FindOrCreateGitHubUser(ctx context.Context, githubID, email, name string) (*OAuthResult, error) {
	// Try to find by GitHub ID.
	op, err := s.userRepo.FindByGitHubID(ctx, githubID)
	if err != nil && err != user.ErrNotFound {
		return nil, err
	}

	if op != nil {
		return &OAuthResult{User: op, IsNew: false}, nil
	}

	// Try to find by email and link GitHub account.
	op, err = s.userRepo.FindByEmail(ctx, email)
	if err != nil && err != user.ErrNotFound {
		return nil, err
	}

	if op != nil {
		// Link existing account to GitHub.
		if err = s.userRepo.UpdateGitHubID(ctx, op.ID, githubID); err != nil {
			return nil, err
		}

		op.GitHubID = githubID

		return &OAuthResult{User: op, IsNew: false}, nil
	}

	// Create new user.
	id := shared.GenerateID()

	now := time.Now()
	newOp := &user.User{
		ID:            id,
		Email:         email,
		Name:          name,
		GitHubID:      githubID,
		EmailVerified: true, // GitHub verifies email.
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.userRepo.Create(ctx, newOp); err != nil {
		return nil, err
	}

	return &OAuthResult{User: newOp, IsNew: true}, nil
}
