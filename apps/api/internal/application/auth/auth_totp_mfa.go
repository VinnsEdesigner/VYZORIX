package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/session"
	infraauth "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security"
)

// MFAEnrollResult holds MFA enrollment data.
type MFAEnrollResult struct {
	Secret      string
	URI         string
	BackupCodes []string
}

// VerifyMFACode verifies a TOTP code for a user.
// After successful verification, it creates a session and marks it as MFA-verified.
func (s *AuthService) VerifyMFACode(ctx context.Context, userID, code string) (*session.Session, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !op.HasMFA() {
		return nil, application.ErrForbidden
	}

	// Verify the stored MAC binding matches this userID.
	// This prevents attackers from using a stolen MFASecret with a different account.
	if op.MFASecret != "" {
		binding := infraauth.MFASecretBinding{
			Secret: op.MFASecret,
			MAC:    op.MFASecretMAC,
		}
		if !infraauth.VerifyMFASecretBinding(userID, binding) {
			return nil, application.ErrForbidden
		}
	}

	// Verify TOTP code.
	cfg := infraauth.DefaultTOTPConfig()
	cfg.AccountName = op.Email
	cfg.Issuer = fmt.Sprintf("Vyzorix-%s", userID[:8])

	totp := infraauth.NewTOTP(op.MFASecret, cfg)
	if !totp.Verify(code) {
		return nil, application.ErrInvalidCredentials
	}

	// Create session and mark it as MFA-verified.
	sess, err := s.CreateSession(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Set MFAVerifiedAt on the session.
	now := time.Now()
	if err := s.sessionRepo.SetMFAVerifiedAt(ctx, sess.ID, now); err != nil {
		return nil, err
	}

	// Update the session object with the MFAVerifiedAt timestamp.
	sess.MFAVerifiedAt = &now

	return sess, nil
}

// EnrollMFA generates a new MFA secret for enrollment.
func (s *AuthService) EnrollMFA(ctx context.Context, userID, email string) (*MFAEnrollResult, error) {
	secret, err := infraauth.GenerateSecret()
	if err != nil {
		return nil, err
	}

	binding := infraauth.CreateMFASecretBinding(userID, secret)
	_ = binding // Store via UpdateMFA call.

	cfg := infraauth.DefaultTOTPConfig()
	cfg.AccountName = email
	cfg.Issuer = fmt.Sprintf("Vyzorix-%s", userID[:8])
	totp := infraauth.NewTOTP(secret, cfg)

	backupCodes, err := infraauth.GenerateBackupCodes(8)
	if err != nil {
		return nil, err
	}

	return &MFAEnrollResult{
		Secret:      secret,
		URI:         totp.ProvisioningURI(),
		BackupCodes: backupCodes,
	}, nil
}

// EnableMFA enables MFA for a user after verifying a code.
// It invalidates all existing sessions to ensure MFA is enforced on next login.
func (s *AuthService) EnableMFA(ctx context.Context, userID, secret string, backupCodes []string) error {
	binding := infraauth.CreateMFASecretBinding(userID, secret)
	now := time.Now()
	if err := s.userRepo.UpdateMFA(ctx, userID, secret, binding.MAC, true, &now); err != nil {
		return err
	}

	// Invalidate all existing sessions - user must re-authenticate with MFA.
	if err := s.LogoutAll(ctx, userID); err != nil {
		slog.Warn("failed to revoke sessions during MFA enable", "user_id", userID, "error", err)
	}

	return nil
}

// DisableMFA disables MFA for a user.
// It revokes all sessions and refresh tokens to ensure security after MFA is disabled.
func (s *AuthService) DisableMFA(ctx context.Context, userID string) error {
	if err := s.userRepo.UpdateMFA(ctx, userID, "", "", false, nil); err != nil {
		return err
	}

	// Revoke all sessions for this user.
	if err := s.LogoutAll(ctx, userID); err != nil {
		slog.Warn("failed to revoke sessions during MFA disable", "user_id", userID, "error", err)
	}

	// Revoke all refresh tokens for this user.
	if s.refreshTokenRepo != nil {
		if err := s.refreshTokenRepo.RevokeAllForUser(ctx, userID); err != nil {
			slog.Warn("failed to revoke refresh tokens during MFA disable", "user_id", userID, "error", err)
		}
	}

	return nil
}

// GetMFAStatus returns MFA status for a user.
func (s *AuthService) GetMFAStatus(ctx context.Context, userID string) (bool, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	return op.MFAEnabled, nil
}

// RegenerateBackupCodes generates and saves new backup codes for a user.
func (s *AuthService) RegenerateBackupCodes(ctx context.Context, userID string) ([]string, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if op.MFASecret == "" {
		return nil, application.ErrInvalidInput
	}

	codes, err := infraauth.GenerateBackupCodes(8)
	if err != nil {
		return nil, err
	}

	err = s.userRepo.UpdateUserMFA(ctx, userID, op.MFASecret, op.MFASecretMAC, codes)
	if err != nil {
		return nil, err
	}

	return codes, nil
}

// VerifyBackupCode verifies a backup code and removes it if valid.
func (s *AuthService) VerifyBackupCode(ctx context.Context, userID, code string) (bool, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if len(op.BackupCodes) == 0 {
		return false, nil
	}

	idx := infraauth.ValidateBackupCode(op.BackupCodes, code)
	if idx < 0 {
		return false, nil
	}

	remaining := infraauth.RemoveBackupCode(op.BackupCodes, idx)
	err = s.userRepo.UpdateUserMFA(ctx, userID, op.MFASecret, op.MFASecretMAC, remaining)
	if err != nil {
		return false, err
	}

	return true, nil
}
