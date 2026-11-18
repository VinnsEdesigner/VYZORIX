package session

import (
	"context"
	"time"
)

// Repository defines the interface for session data access.
type Repository interface {
	// FindByID retrieves a session by ID.
	FindByID(ctx context.Context, id string) (*Session, error)

	// FindByUserID retrieves all sessions for a user.
	FindByUserID(ctx context.Context, userID string) ([]*Session, error)

	// ListActiveByUser retrieves active (non-expired) sessions for a user, ordered by creation time.
	ListActiveByUser(ctx context.Context, userID string) ([]*Session, error)

	// Create creates a new session.
	Create(ctx context.Context, s *Session) error

	// UpdateSessionMeta sets the client IP address and user agent on a session.
	// Used to backfill a session created before those fields were captured, so the
	// profile page has something to show without a fresh login.
	UpdateSessionMeta(ctx context.Context, id, ipAddress, userAgent string) error

	// Delete deletes a session.
	Delete(ctx context.Context, id string) error

	// DeleteByUserID deletes all sessions for a user.
	DeleteByUserID(ctx context.Context, userID string) error

	// DeleteExpired deletes all expired sessions.
	DeleteExpired(ctx context.Context) (int, error)

	// Extend extends a session's expiration time.
	Extend(ctx context.Context, id string, newExpiry time.Time) error

	// UpdateOrganizationID updates the selected organization ID for a session.
	UpdateOrganizationID(ctx context.Context, sessionID, organizationID string) error

	// UpdateUserSessionsOrgID sets the selected org on all active sessions
	// for a user (used when an invite is accepted).
	UpdateUserSessionsOrgID(ctx context.Context, userID, organizationID string) error

	// SetMFAVerifiedAt sets the MFAVerifiedAt timestamp for a session.
	// This is called after successful MFA verification during login.
	SetMFAVerifiedAt(ctx context.Context, sessionID string, verifiedAt time.Time) error

	// AddSessionRevocation adds a session token hash to the revocation list.
	AddSessionRevocation(ctx context.Context, tokenHash, reason string) error

	// IsSessionRevoked checks if a session token hash is in the revocation list.
	IsSessionRevoked(ctx context.Context, tokenHash string) (bool, error)

	// RemoveSessionRevocation removes a session from the revocation list.
	RemoveSessionRevocation(ctx context.Context, tokenHash string) error

	// ListSessionRevocations retrieves all revoked sessions, optionally filtered by reason.
	ListSessionRevocations(ctx context.Context, reason string, limit int) ([]*SessionRevocation, error)

	// CleanupSessionRevocations removes revocation entries older than the specified duration.
	CleanupSessionRevocations(ctx context.Context, olderThan time.Duration) (int, error)

	// RevokeAllUserSessions revokes all sessions for a specific user.
	RevokeAllUserSessions(ctx context.Context, userID string) error
}
