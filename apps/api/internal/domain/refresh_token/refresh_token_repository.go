package refresh_token

import (
	"context"
	"time"
)

// Repository defines the interface for refresh token data access.
type Repository interface {
	// Create creates a new refresh token.
	Create(ctx context.Context, rt *RefreshToken) error

	// FindByID retrieves a refresh token by ID.
	FindByID(ctx context.Context, id string) (*RefreshToken, error)

	// FindByTokenHash retrieves a refresh token by its hash.
	FindByTokenHash(ctx context.Context, tokenHash string) (*RefreshToken, error)

	// Revoke revokes a refresh token.
	Revoke(ctx context.Context, id string) error

	// RevokeByTokenHash revokes a refresh token by its hash.
	RevokeByTokenHash(ctx context.Context, tokenHash string) error

	// RevokeAllForUser revokes all refresh tokens for a user.
	RevokeAllForUser(ctx context.Context, userID string) error

	// CleanupExpired removes expired refresh tokens older than the specified duration.
	CleanupExpired(ctx context.Context, olderThan time.Duration) (int, error)
}
