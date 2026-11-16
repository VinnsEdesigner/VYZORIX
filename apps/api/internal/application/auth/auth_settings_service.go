package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/theme"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// ErrValidationError is returned when validation fails.
var ErrValidationError = &ValidationError{}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ClientSettingsInput represents client settings input from the API.
type ClientSettingsInput struct {
	ServerURL          *string `json:"serverUrl,omitempty"`
	DeviceID           *string `json:"deviceId,omitempty"`
	RequestTimeoutMs   *int    `json:"requestTimeoutMs,omitempty"`
	AutoReconnect      *bool   `json:"autoReconnect,omitempty"`
	StrictHmac         *bool   `json:"strictHmac,omitempty"`
	LogBufferLimit     *int    `json:"logBufferLimit,omitempty"`
	SignalHistoryLimit *int    `json:"signalHistoryLimit,omitempty"`
}

// OrgThemeSource supplies an organisation's default theme id. Declared here as
// an interface rather than importing application/organization, which already
// imports this package (an import would be a cycle). The organisation settings
// service satisfies it.
type OrgThemeSource interface {
	// OrgTheme returns the organisation's theme id, or "" when none is set.
	OrgTheme(ctx context.Context, orgID string) (string, error)
}

// ClientSettingsService handles client settings operations.
type ClientSettingsService struct {
	userRepo user.Repository
	orgTheme OrgThemeSource
}

// NewClientSettingsService creates a new ClientSettingsService.
func NewClientSettingsService(repo user.Repository, orgTheme OrgThemeSource) *ClientSettingsService {
	return &ClientSettingsService{userRepo: repo, orgTheme: orgTheme}
}

// GetSettings retrieves all settings for a user.
func (s *ClientSettingsService) GetSettings(ctx context.Context, userID string) (*SettingsResponse, error) {
	// Get full user settings in single query.
	settings, err := s.userRepo.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &SettingsResponse{
		Notifications: settings.Notifications,
		Client:        &settings.Client,
	}, nil
}

// UpdateClientSettings updates client settings for a user.
func (s *ClientSettingsService) UpdateClientSettings(ctx context.Context, userID string, input *ClientSettingsInput) (*SettingsResponse, error) {
	if err := s.validateClientSettings(input); err != nil {
		return nil, err
	}

	currentSettings, err := s.userRepo.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	s.applyClientSettingsUpdates(&currentSettings.Client, input)

	if err = s.userRepo.UpdateClientSettings(ctx, userID, currentSettings.Client); err != nil {
		return nil, err
	}

	notifications, err := s.userRepo.GetNotifications(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &SettingsResponse{
		Notifications: notifications,
		Client:        &currentSettings.Client,
	}, nil
}

// validateClientSettings validates the client settings input.
func (s *ClientSettingsService) validateClientSettings(input *ClientSettingsInput) error {
	if input.RequestTimeoutMs != nil {
		if *input.RequestTimeoutMs < 500 || *input.RequestTimeoutMs > 60000 {
			return &ValidationError{Message: "requestTimeoutMs must be between 500 and 60000"}
		}
	}
	if input.LogBufferLimit != nil {
		if *input.LogBufferLimit < 50 || *input.LogBufferLimit > 5000 {
			return &ValidationError{Message: "logBufferLimit must be between 50 and 5000"}
		}
	}
	if input.SignalHistoryLimit != nil {
		if *input.SignalHistoryLimit < 30 || *input.SignalHistoryLimit > 2000 {
			return &ValidationError{Message: "signalHistoryLimit must be between 30 and 2000"}
		}
	}
	return s.validateServerURL(input.ServerURL)
}

// validateServerURL validates the server URL format.
func (s *ClientSettingsService) validateServerURL(serverURL *string) error {
	if serverURL == nil || *serverURL == "" {
		return nil
	}
	parsedURL, urlErr := url.Parse(*serverURL)
	if urlErr != nil {
		return &ValidationError{Message: "serverUrl must be a valid HTTP/HTTPS URL"}
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ValidationError{Message: "serverUrl must be a valid HTTP/HTTPS URL"}
	}
	if parsedURL.Host == "" {
		return &ValidationError{Message: "serverUrl must have a valid host"}
	}
	return nil
}

// applyClientSettingsUpdates applies input updates to client settings.
func (s *ClientSettingsService) applyClientSettingsUpdates(client *user.ClientSettings, input *ClientSettingsInput) {
	if input.ServerURL != nil {
		client.ServerURL = *input.ServerURL
	}
	if input.DeviceID != nil {
		client.DeviceID = *input.DeviceID
	}
	if input.RequestTimeoutMs != nil {
		client.RequestTimeoutMs = *input.RequestTimeoutMs
	}
	if input.AutoReconnect != nil {
		client.AutoReconnect = *input.AutoReconnect
	}
	if input.StrictHmac != nil {
		client.StrictHmac = *input.StrictHmac
	}
	if input.LogBufferLimit != nil {
		client.LogBufferLimit = *input.LogBufferLimit
	}
	if input.SignalHistoryLimit != nil {
		client.SignalHistoryLimit = *input.SignalHistoryLimit
	}
}

// ResetSettings resets all settings to defaults.
func (s *ClientSettingsService) ResetSettings(ctx context.Context, userID string) (*SettingsResponse, error) {
	err := s.userRepo.ResetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.GetSettings(ctx, userID)
}

// SettingsResponse represents the complete settings response.
type SettingsResponse struct {
	Notifications *user.NotificationSettings `json:"notifications"`
	Client        *user.ClientSettings       `json:"client,omitempty"`
	Preferences   map[string]any                 `json:"preferences,omitempty"`
}

var defaultPreferences = map[string]any{
	"refresh_interval":    30,
	"notification_filter": "all",
	"language":            "en",
}

// DefaultTheme is the theme applied when neither the user nor the organisation
// has set one. It is kept OUT of defaultPreferences: the merge in
// ResolveTheme treats an absent user theme as "inherit", so defaulting it here
// would make every user look like they had explicitly chosen dark.
const DefaultTheme = "dark"

func (s *ClientSettingsService) GetPreferences(ctx context.Context, userID string) (map[string]any, error) {
	raw, err := s.userRepo.GetPreferencesRaw(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	if raw == "" {
		return copyDefaults(), nil
	}
	prefs := make(map[string]any)
	if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
		return copyDefaults(), fmt.Errorf("invalid preferences JSON: %w", err)
	}
	for k, v := range defaultPreferences {
		if _, exists := prefs[k]; !exists {
			prefs[k] = v
		}
	}
	return prefs, nil
}

// copyDefaults returns a fresh copy of defaultPreferences. Returning the shared
// map itself would let a caller's later mutation write into the package-level
// map and leak across requests.
func copyDefaults() map[string]any {
	out := make(map[string]any, len(defaultPreferences))
	for k, v := range defaultPreferences {
		out[k] = v
	}
	return out
}

// ResolveTheme resolves the effective theme id for a user in an organisation.
//
// This is the cascade, highest precedence first:
//
//	user preference  >  organisation default  >  "dark"
//
// It is resolved server-side because a member cannot read the organisation's
// settings (that endpoint requires admin), so the client cannot perform the
// merge itself. An absent user theme means "inherit", not "dark": the theme is
// deliberately kept out of defaultPreferences so an unset value stays unset.
//
// An empty orgID (no organisation selected) skips the org tier.
func (s *ClientSettingsService) ResolveTheme(ctx context.Context, userID, orgID string) (string, error) {
	prefs, err := s.GetPreferences(ctx, userID)
	if err != nil {
		return "", err
	}
	if v, ok := prefs["theme"]; ok {
		if id, isString := v.(string); isString && id != "" {
			return id, nil
		}
	}

	if orgID != "" && s.orgTheme != nil {
		orgTheme, err := s.orgTheme.OrgTheme(ctx, orgID)
		if err != nil {
			return "", err
		}
		if orgTheme != "" {
			return orgTheme, nil
		}
	}

	return DefaultTheme, nil
}

func (s *ClientSettingsService) UpdatePreferences(ctx context.Context, userID string, updates map[string]any) (map[string]any, error) {
	// Reject an unknown theme id before persisting. An absent key or empty
	// string is allowed: it clears the preference so the org default (or the
	// system default) applies.
	if raw, ok := updates["theme"]; ok && raw != nil {
		if id, isString := raw.(string); !isString || !theme.IsValid(id) {
			return nil, fmt.Errorf("%w: invalid theme", ErrValidationError)
		}
	}

	current, err := s.GetPreferences(ctx, userID)
	if err != nil {
		current = make(map[string]any)
	}
	for k, v := range updates {
		// A nil value, or an empty theme string, clears the key so the next tier
		// in the cascade applies. Storing "" would be indistinguishable from
		// "unset" but would persist a meaningless value.
		if v == nil || (k == "theme" && v == "") {
			delete(current, k)
		} else {
			current[k] = v
		}
	}
	data, err := json.Marshal(current)
	if err != nil {
		return nil, ErrValidationError
	}
	if err := s.userRepo.SavePreferencesRaw(ctx, userID, string(data)); err != nil {
		return nil, err
	}
	return current, nil
}
