package user

import (
	"context"
	"errors"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// NotificationService handles notification settings operations.
type NotificationService struct {
	userRepo user.Repository
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(repo user.Repository) *NotificationService {
	return &NotificationService{userRepo: repo}
}

// GetNotifications retrieves notification settings for a user.
func (s *NotificationService) GetNotifications(ctx context.Context, userID string) (*user.NotificationSettings, error) {
	return s.userRepo.GetNotifications(ctx, userID)
}

// UpdateNotifications updates notification settings for a user.
func (s *NotificationService) UpdateNotifications(ctx context.Context, userID string, input *user.NotificationInput) (*user.NotificationSettings, error) {
	notifications, err := s.userRepo.GetNotifications(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.applyNotificationUpdates(input, notifications)

	err = s.validateNotificationSettings(notifications)
	if err != nil {
		return nil, err
	}

	err = s.userRepo.UpdateNotifications(ctx, userID, notifications)
	if err != nil {
		return nil, err
	}

	return notifications, nil
}

// applyNotificationUpdates applies input updates to notification settings.
func (s *NotificationService) applyNotificationUpdates(input *user.NotificationInput, notifications *user.NotificationSettings) {
	if input.Enabled != nil {
		notifications.Enabled = *input.Enabled
	}
	if input.Channels != nil {
		notifications.Channels = *input.Channels
	}

	s.applyEmailUpdates(input.Email, &notifications.Email)
	s.applyPushUpdates(input.Push, &notifications.Push)
	s.applyWebhookUpdates(input.Webhook, &notifications.Webhook)
}

// applyEmailUpdates applies email notification updates.
func (s *NotificationService) applyEmailUpdates(input *user.EmailNotificationInput, email *user.EmailNotifications) {
	if input == nil {
		return
	}
	if input.ThresholdBreach != nil {
		email.ThresholdBreach = *input.ThresholdBreach
	}
	if input.DeviceOffline != nil {
		email.DeviceOffline = *input.DeviceOffline
	}
	if input.DeviceOnline != nil {
		email.DeviceOnline = *input.DeviceOnline
	}
	if input.UpdateAvailable != nil {
		email.UpdateAvailable = *input.UpdateAvailable
	}
	if input.CommandFailed != nil {
		email.CommandFailed = *input.CommandFailed
	}
	if input.RegistrationRequest != nil {
		email.RegistrationRequest = *input.RegistrationRequest
	}
}

// applyPushUpdates applies push notification updates.
func (s *NotificationService) applyPushUpdates(input *user.PushNotificationInput, push *user.PushNotifications) {
	if input == nil {
		return
	}
	if input.ThresholdBreach != nil {
		push.ThresholdBreach = *input.ThresholdBreach
	}
	if input.DeviceOffline != nil {
		push.DeviceOffline = *input.DeviceOffline
	}
	if input.DeviceOnline != nil {
		push.DeviceOnline = *input.DeviceOnline
	}
	if input.UpdateAvailable != nil {
		push.UpdateAvailable = *input.UpdateAvailable
	}
	if input.CommandFailed != nil {
		push.CommandFailed = *input.CommandFailed
	}
	if input.RegistrationRequest != nil {
		push.RegistrationRequest = *input.RegistrationRequest
	}
}

// applyWebhookUpdates applies webhook notification updates.
func (s *NotificationService) applyWebhookUpdates(input *user.WebhookNotificationInput, webhook *user.WebhookNotifications) {
	if input == nil {
		return
	}
	if input.Enabled != nil {
		webhook.Enabled = *input.Enabled
	}
	if input.URL != nil {
		webhook.URL = *input.URL
	}
	if input.Types != nil {
		webhook.Types = input.Types
	}
}

// validateNotificationSettings validates the notification settings.
func (s *NotificationService) validateNotificationSettings(notifications *user.NotificationSettings) error {
	if notifications.Webhook.Enabled && notifications.Webhook.URL == "" {
		return errors.New("webhook URL is required when webhook is enabled")
	}
	return nil
}

// RotateWebhookSecret rotates the webhook secret for a user.
func (s *NotificationService) RotateWebhookSecret(ctx context.Context, userID string) (string, error) {
	return s.userRepo.RotateWebhookSecret(ctx, userID)
}
