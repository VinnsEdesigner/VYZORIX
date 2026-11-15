package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// ValidatePasswordPolicy validates a new password against user's security policy.
func (s *AuthService) ValidatePasswordPolicy(ctx context.Context, op *user.User, newPassword string) error {
	settings := op.SecuritySettings

	if settings.PasswordHistoryCount > 0 {
		if err := s.checkPasswordHistory(ctx, op.ID, newPassword, settings.PasswordHistoryCount); err != nil {
			return err
		}
	}

	if settings.PasswordMinAgeDays > 0 {
		elapsed := time.Since(op.UpdatedAt)
		minAge := time.Duration(settings.PasswordMinAgeDays) * 24 * time.Hour
		if elapsed < minAge {
			return fmt.Errorf("%w: password cannot be changed for %d days", application.ErrPasswordPolicy, settings.PasswordMinAgeDays)
		}
	}

	return nil
}

func (s *AuthService) checkPasswordHistory(ctx context.Context, userID, newPassword string, _ int) error {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	if err := s.passwordHasher.Verify(newPassword, op.PasswordHash); err == nil {
		return fmt.Errorf("%w: cannot reuse current password", application.ErrPasswordPolicy)
	}

	return nil
}

// IsPasswordExpired checks if user's password has expired.
func (s *AuthService) IsPasswordExpired(ctx context.Context, op *user.User) (bool, error) {
	settings := op.SecuritySettings

	if settings.PasswordMaxAgeDays <= 0 {
		return false, nil
	}

	maxAge := time.Duration(settings.PasswordMaxAgeDays) * 24 * time.Hour
	elapsed := time.Since(op.UpdatedAt)

	return elapsed > maxAge, nil
}
