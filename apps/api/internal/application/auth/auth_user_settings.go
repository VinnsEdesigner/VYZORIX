package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// UpdateSettingsRequest represents a request to update user settings.
type UpdateSettingsRequest struct {
	Client *user.ClientSettings `json:"client,omitempty"`
	Name   *string                  `json:"name,omitempty"`
	Reset  bool                     `json:"reset,omitempty"`
}

// UpdateUserName updates a user's name.
func (s *AuthService) UpdateUserName(ctx context.Context, userID, name string) (*user.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, application.ErrInvalidInput
	}

	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	op.Name = name
	op.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, op); err != nil {
		return nil, err
	}

	return op, nil
}

// UpdateSettings updates a user's settings.
func (s *AuthService) UpdateSettings(ctx context.Context, userID string, req *UpdateSettingsRequest) (*user.User, error) {
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUnauthorized
		}
		return nil, err
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, application.ErrInvalidInput
		}

		op.Name = name
		if err := s.userRepo.Update(ctx, op); err != nil {
			return nil, err
		}
	}

	// Update client settings in user_settings table with validation.
	if req.Client != nil {
		// Validate client settings.
		if req.Client.RequestTimeoutMs < 500 || req.Client.RequestTimeoutMs > 60000 {
			return nil, fmt.Errorf("requestTimeoutMs must be between 500 and 60000")
		}
		if req.Client.LogBufferLimit < 50 || req.Client.LogBufferLimit > 5000 {
			return nil, fmt.Errorf("logBufferLimit must be between 50 and 5000")
		}
		if req.Client.SignalHistoryLimit < 30 || req.Client.SignalHistoryLimit > 2000 {
			return nil, fmt.Errorf("signalHistoryLimit must be between 30 and 2000")
		}

		if err := s.userRepo.UpdateClientSettings(ctx, userID, *req.Client); err != nil {
			return nil, err
		}
		op.ClientSettings = *req.Client
	}

	op.UpdatedAt = time.Now()

	return op, nil
}

// ResetSettings resets user settings to defaults.
func (s *AuthService) ResetSettings(ctx context.Context, userID string) (*user.User, error) {
	_, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, application.ErrUnauthorized
		}
		return nil, err
	}

	if resetErr := s.userRepo.ResetSettings(ctx, userID); resetErr != nil {
		return nil, resetErr
	}

	// Reload user to get fresh data.
	op, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return op, nil
}
