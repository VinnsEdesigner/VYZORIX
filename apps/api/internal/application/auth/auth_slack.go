package auth

import (
	"context"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// GetNotificationSettings returns a user's notification settings.
func (s *ClientSettingsService) GetNotificationSettings(ctx context.Context, userID string) (*user.NotificationSettings, error) {
	return s.userRepo.GetNotifications(ctx, userID)
}

// LinkSlackWorkspace stores the result of the Slack link-up on the user's
// notification settings: the bot token, the workspace and, as a convenience,
// enables the slack channel so notifications start flowing once a channel is
// chosen. It never touches a webhook secret.
func (s *ClientSettingsService) LinkSlackWorkspace(ctx context.Context, userID, accessToken, teamID, teamName string) error {
	settings, err := s.userRepo.GetNotifications(ctx, userID)
	if err != nil {
		return err
	}

	settings.Slack.Connected = true
	settings.Slack.AccessToken = accessToken
	settings.Slack.TeamID = teamID
	settings.Slack.TeamName = teamName
	// A previous link-up may have targeted a channel in a different workspace, so
	// clear it; the user picks again from the linked workspace's channel list.
	settings.Slack.ChannelID = ""
	settings.Slack.ChannelName = ""

	if !containsChannel(settings.Channels, "slack") {
		settings.Channels = append(settings.Channels, "slack")
	}

	return s.userRepo.UpdateNotifications(ctx, userID, settings)
}

// UnlinkSlack clears the stored Slack workspace and token.
func (s *ClientSettingsService) UnlinkSlack(ctx context.Context, userID string) error {
	settings, err := s.userRepo.GetNotifications(ctx, userID)
	if err != nil {
		return err
	}
	settings.Slack = user.SlackNotifications{}
	settings.Channels = removeChannel(settings.Channels, "slack")
	return s.userRepo.UpdateNotifications(ctx, userID, settings)
}

func containsChannel(channels []string, channel string) bool {
	for _, c := range channels {
		if c == channel {
			return true
		}
	}
	return false
}

func removeChannel(channels []string, channel string) []string {
	out := make([]string, 0, len(channels))
	for _, c := range channels {
		if c != channel {
			out = append(out, c)
		}
	}
	return out
}
