package user

import (
	"context"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// Repository defines the interface for user data access.
type Repository interface {
	// FindByID retrieves a user by ID.
	FindByID(ctx context.Context, id string) (*User, error)

	// FindByEmail retrieves a user by email.
	FindByEmail(ctx context.Context, email string) (*User, error)

	// FindByGoogleID retrieves a user by Google ID.
	FindByGoogleID(ctx context.Context, googleID string) (*User, error)

	// FindByGitHubID retrieves a user by GitHub ID.
	FindByGitHubID(ctx context.Context, githubID string) (*User, error)

	// Create creates a new user.
	Create(ctx context.Context, op *User) error

	// Update updates an existing user.
	Update(ctx context.Context, op *User) error

	// Delete deletes a user.
	Delete(ctx context.Context, id string) error

	// Count returns the total number of users.
	Count(ctx context.Context) (int, error)

	// List returns a paginated list of users.
	List(ctx context.Context, limit, offset int) ([]*User, int, error)

	// UpdatePassword updates the password hash for a user.
	UpdatePassword(ctx context.Context, id, passwordHash string) error

	// UpdateMFA updates MFA settings for a user.
	// When enabled is true, mfaEnabledAt should be set to the current time.
	// When enabled is false, mfaEnabledAt is ignored (will be set to nil).
	UpdateMFA(ctx context.Context, id, secret, secretMAC string, enabled bool, mfaEnabledAt *time.Time) error

	// UpdateUserMFA updates the MFA secret, MAC, and backup codes for a user.
	UpdateUserMFA(ctx context.Context, userID, mfaSecret, mfaSecretMAC string, backupCodes []string) error

	// VerifyEmail marks a user's email as verified.
	VerifyEmail(ctx context.Context, id string) error

	// UpdateEmailVerified updates the email verified status for a user.
	UpdateEmailVerified(ctx context.Context, id string, verified bool) error

	// UpdateGoogleID updates the Google ID for a user.
	UpdateGoogleID(ctx context.Context, id, googleID string) error

	// UpdateGitHubID updates the GitHub ID for a user.
	UpdateGitHubID(ctx context.Context, id, githubID string) error

	// UpdateName updates the display name for a user.
	UpdateName(ctx context.Context, id, name string) error

	// UpdateAvatarURL updates the avatar image URL for a user. An empty string
	// clears it, so the client falls back to its default glyph.
	UpdateAvatarURL(ctx context.Context, id, avatarURL string) error

	// RecordLogin stamps the user's last login time, called on every successful
	// authentication so the profile can show "last login".
	RecordLogin(ctx context.Context, id string, at time.Time) error

	// GetNotificationsReadAt returns the notification read watermark (Unix
	// milliseconds); 0 when the user has never marked anything read.
	GetNotificationsReadAt(ctx context.Context, userID string) (int64, error)

	// SetNotificationsReadAt stamps the notification read watermark.
	SetNotificationsReadAt(ctx context.Context, userID string, at int64) error

	// LastLoginAt returns the user's most recent login time. The bool is false
	// when none has been recorded (an account created before the column existed),
	// so the caller can render a dash rather than a zero time.
	LastLoginAt(ctx context.Context, id string) (time.Time, bool, error)

	// UpdateThresholds updates the alert thresholds for a user.
	UpdateThresholds(ctx context.Context, id string, th Thresholds) error

	// GetThresholds retrieves the alert thresholds for a user.
	GetThresholds(ctx context.Context, id string) (Thresholds, error)

	// UpdateClientSettings updates the client preferences for a user.
	UpdateClientSettings(ctx context.Context, id string, cs ClientSettings) error

	// GetPreferencesRaw retrieves the raw JSON preferences string for a user.
	GetPreferencesRaw(ctx context.Context, userID string) (string, error)

	// SavePreferencesRaw saves the raw JSON preferences string for a user.
	SavePreferencesRaw(ctx context.Context, userID string, json string) error

	// ResetSettings resets all settings to defaults for a user.
	ResetSettings(ctx context.Context, id string) error

	// GetEmailVerified returns whether a user has verified their email.
	GetEmailVerified(ctx context.Context, id string) (bool, error)

	// DisableMFA disables MFA for a user by clearing the MFA secret and backup codes.
	DisableMFA(ctx context.Context, id string) error

	// GetSetting retrieves a setting value by key.
	GetSetting(ctx context.Context, key string) (string, error)

	// SetSetting updates or inserts a setting value.
	SetSetting(ctx context.Context, key, value string) error

	// GetEnforceHMAC returns whether HMAC enforcement is enabled.
	GetEnforceHMAC(ctx context.Context) (bool, error)

	// SetEnforceHMAC updates the HMAC enforcement setting.
	SetEnforceHMAC(ctx context.Context, enforce bool) error

	// GetHMACWindowSeconds returns the HMAC timestamp window in seconds.
	GetHMACWindowSeconds(ctx context.Context) (int, error)

	// SetHMACWindowSeconds updates the HMAC timestamp window.
	SetHMACWindowSeconds(ctx context.Context, seconds int) error

	// GetUserSettings retrieves all settings for a user.
	GetUserSettings(ctx context.Context, userID string) (*UserSettings, error)

	// GetNotifications retrieves notification settings for a user.
	GetNotifications(ctx context.Context, userID string) (*NotificationSettings, error)

	// UpdateNotifications updates notification settings for a user.
	UpdateNotifications(ctx context.Context, userID string, settings *NotificationSettings) error

	// RotateWebhookSecret generates a new webhook secret for a user.
	RotateWebhookSecret(ctx context.Context, userID string) (string, error)

	// UpdateFCMToken updates the FCM token for a user.
	UpdateFCMToken(ctx context.Context, userID, fcmToken string) error

	// UpdateInstanceRole sets a user's authority over the deployment itself
	// (permission.InstanceAdmin or permission.InstanceNone). It is separate from
	// Update because that method does not carry the instance role and would
	// otherwise reset it on every profile write.
	UpdateInstanceRole(ctx context.Context, userID string, role permission.InstanceRole) error
}
