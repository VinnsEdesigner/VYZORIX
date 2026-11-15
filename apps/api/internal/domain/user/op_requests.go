package user

// LoginRequest is the payload for user login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest is the payload for user self-registration.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// UserRegisterRequest is the payload for user self-registration.
// Alias for RegisterRequest for backwards compatibility.
type UserRegisterRequest = RegisterRequest

// UpdateNameRequest is the payload for updating the user's display name.
type UpdateNameRequest struct {
	Name *string `json:"name,omitempty"`
}

// UpdateSettingsRequest is the payload for updating user settings.
type UpdateSettingsRequest struct {
	Name       *string         `json:"name,omitempty"`
	Thresholds *Thresholds     `json:"thresholds,omitempty"`
	Client     *ClientSettings `json:"client,omitempty"`
	Reset      bool            `json:"reset,omitempty"` // Reset all settings to defaults.
}
