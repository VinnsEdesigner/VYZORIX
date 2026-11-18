package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/refresh_token"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/session"
)

// Ensure SessionRepository implements session.Repository.
var _ session.Repository = (*SessionRepository)(nil)

// SessionRepository implements session.Repository using SQLite.
type SessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a new SessionRepository.
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// FindByID retrieves a session by ID.
func (r *SessionRepository) FindByID(ctx context.Context, id string) (*session.Session, error) {
	query := `SELECT id, user_id, expires_at, created_at, ip_address, user_agent, organization_id, mfa_verified_at, signing_key FROM auth_sessions WHERE id = ?`

	var s session.Session

	var ipAddress, userAgent, orgID, signingKey sql.NullString
	var expiresAt, createdAt, mfaVerifiedAt sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.UserID, &expiresAt, &createdAt, &ipAddress, &userAgent, &orgID, &mfaVerifiedAt, &signingKey,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, session.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	s.IPAddress = ipAddress.String
	s.UserAgent = userAgent.String
	s.SelectedOrganizationID = orgID.String
	s.SigningKey = signingKey.String

	// Parse timestamps from Unix milliseconds.
	if expiresAt.Valid {
		s.ExpiresAt = time.UnixMilli(expiresAt.Int64)
	}
	if createdAt.Valid {
		s.CreatedAt = time.UnixMilli(createdAt.Int64)
	}
	if mfaVerifiedAt.Valid {
		t := time.UnixMilli(mfaVerifiedAt.Int64)
		s.MFAVerifiedAt = &t
	}

	return &s, nil
}

// FindByUserID retrieves all sessions for a user.
func (r *SessionRepository) FindByUserID(ctx context.Context, userID string) ([]*session.Session, error) {
	query := `SELECT id, user_id, expires_at, created_at, ip_address, user_agent, organization_id, mfa_verified_at FROM auth_sessions WHERE user_id = ?`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var sessions []*session.Session

	for rows.Next() {
		var s session.Session

		var ipAddress, userAgent, orgID sql.NullString
		var expiresAt, createdAt, mfaVerifiedAt sql.NullInt64

		if err := rows.Scan(&s.ID, &s.UserID, &expiresAt, &createdAt, &ipAddress, &userAgent, &orgID, &mfaVerifiedAt); err != nil {
			return nil, err
		}

		s.IPAddress = ipAddress.String
		s.UserAgent = userAgent.String
		s.SelectedOrganizationID = orgID.String

		// Parse timestamps from Unix milliseconds.
		if expiresAt.Valid {
			s.ExpiresAt = time.UnixMilli(expiresAt.Int64)
		}
		if createdAt.Valid {
			s.CreatedAt = time.UnixMilli(createdAt.Int64)
		}
		if mfaVerifiedAt.Valid {
			t := time.UnixMilli(mfaVerifiedAt.Int64)
			s.MFAVerifiedAt = &t
		}

		sessions = append(sessions, &s)
	}

	return sessions, rows.Err()
}

// ListActiveByUser retrieves active (non-expired) sessions for a user, ordered by creation time.
func (r *SessionRepository) ListActiveByUser(ctx context.Context, userID string) ([]*session.Session, error) {
	query := `SELECT id, user_id, expires_at, created_at, ip_address, user_agent, organization_id, mfa_verified_at
		FROM auth_sessions
		WHERE user_id = ? AND expires_at > ?
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, userID, time.Now())
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var sessions []*session.Session

	for rows.Next() {
		var s session.Session

		var ipAddress, userAgent, orgID sql.NullString
		var expiresAt, createdAt, mfaVerifiedAt sql.NullInt64

		if err := rows.Scan(&s.ID, &s.UserID, &expiresAt, &createdAt, &ipAddress, &userAgent, &orgID, &mfaVerifiedAt); err != nil {
			return nil, err
		}

		s.IPAddress = ipAddress.String
		s.UserAgent = userAgent.String
		s.SelectedOrganizationID = orgID.String

		// Parse timestamps from Unix milliseconds.
		if expiresAt.Valid {
			s.ExpiresAt = time.UnixMilli(expiresAt.Int64)
		}
		if createdAt.Valid {
			s.CreatedAt = time.UnixMilli(createdAt.Int64)
		}
		if mfaVerifiedAt.Valid {
			t := time.UnixMilli(mfaVerifiedAt.Int64)
			s.MFAVerifiedAt = &t
		}

		sessions = append(sessions, &s)
	}

	return sessions, rows.Err()
}

// Create creates a new session.
func (r *SessionRepository) Create(ctx context.Context, s *session.Session) error {
	query := `INSERT INTO auth_sessions (id, user_id, token_hash, expires_at, created_at, ip_address, user_agent, organization_id, mfa_verified_at, signing_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var mfaVerifiedAt interface{}
	if s.MFAVerifiedAt != nil {
		mfaVerifiedAt = s.MFAVerifiedAt.UnixMilli()
	}

	_, err := r.db.ExecContext(ctx, query,
		s.ID, s.UserID, s.ID, s.ExpiresAt.UnixMilli(), s.CreatedAt.UnixMilli(),
		nullString(s.IPAddress), nullString(s.UserAgent), nullString(s.SelectedOrganizationID), mfaVerifiedAt, nullString(s.SigningKey),
	)

	return err
}

// UpdateSessionMeta sets the client IP address and user agent on a session.
// It is used to backfill sessions created before those fields were captured.
func (r *SessionRepository) UpdateSessionMeta(ctx context.Context, id, ipAddress, userAgent string) error {
        result, err := r.db.ExecContext(ctx,
                "UPDATE auth_sessions SET ip_address = ?, user_agent = ? WHERE id = ?",
                nullString(ipAddress), nullString(userAgent), id,
        )
        if err != nil {
                return err
        }

        rows, err := result.RowsAffected()
        if err != nil {
                return err
        }

        if rows == 0 {
                return session.ErrNotFound
        }

        return nil
}

// Delete deletes a session.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM auth_sessions WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return session.ErrNotFound
	}

	return nil
}

// DeleteByUserID deletes all sessions for a user.
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM auth_sessions WHERE user_id = ?", userID)
	return err
}

// DeleteExpired deletes all expired sessions.
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM auth_sessions WHERE expires_at < ?", time.Now())
	if err != nil {
		return 0, err
	}

	deleted, err := result.RowsAffected()

	return int(deleted), err
}

// Extend extends a session's expiration time.
func (r *SessionRepository) Extend(ctx context.Context, id string, newExpiry time.Time) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE auth_sessions SET expires_at = ? WHERE id = ?",
		newExpiry, id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return session.ErrNotFound
	}

	return nil
}

// UpdateOrganizationID updates the selected organization ID for a session.
func (r *SessionRepository) UpdateOrganizationID(ctx context.Context, sessionID, organizationID string) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE auth_sessions SET organization_id = ? WHERE id = ?",
		nullString(organizationID), sessionID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return session.ErrNotFound
	}

	return nil
}

// UpdateUserSessionsOrgID sets the selected org on all active sessions
// for a user (used when an invite is accepted).
func (r *SessionRepository) UpdateUserSessionsOrgID(ctx context.Context, userID, organizationID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE auth_sessions SET organization_id = ? WHERE user_id = ? AND expires_at > ?",
		nullString(organizationID), userID, time.Now().Unix(),
	)
	return err
}

// SetMFAVerifiedAt sets the MFAVerifiedAt timestamp for a session.
// This is called after successful MFA verification during login.
func (r *SessionRepository) SetMFAVerifiedAt(ctx context.Context, sessionID string, verifiedAt time.Time) error {
	result, err := r.db.ExecContext(ctx,
		"UPDATE auth_sessions SET mfa_verified_at = ? WHERE id = ?",
		verifiedAt.UnixMilli(), sessionID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return session.ErrNotFound
	}

	return nil
}

// AddSessionRevocation adds a session token hash to the revocation list.
func (r *SessionRepository) AddSessionRevocation(ctx context.Context, tokenHash, reason string) error {
	query := `INSERT INTO session_revocations (session_id, revoked_at, reason) VALUES (?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, tokenHash, time.Now().UnixMilli(), reason)

	return err
}

// IsSessionRevoked checks if a session token hash is in the revocation list.
func (r *SessionRepository) IsSessionRevoked(ctx context.Context, tokenHash string) (bool, error) {
	query := `SELECT COUNT(*) FROM session_revocations WHERE session_id = ?`

	var count int

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// RemoveSessionRevocation removes a session from the revocation list.
func (r *SessionRepository) RemoveSessionRevocation(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM session_revocations WHERE session_id = ?`
	_, err := r.db.ExecContext(ctx, query, tokenHash)

	return err
}

// ListSessionRevocations retrieves all revoked sessions, optionally filtered by reason.
func (r *SessionRepository) ListSessionRevocations(ctx context.Context, reason string, limit int) ([]*session.SessionRevocation, error) {
	if limit <= 0 {
		limit = 100
	}

	if limit > 1000 {
		limit = 1000
	}

	var query string

	var args []interface{}

	if reason != "" {
		query = `SELECT token_hash, revoked_at, reason FROM session_revocation_list WHERE reason = ? ORDER BY revoked_at DESC LIMIT ?`
		args = []interface{}{reason, limit}
	} else {
		query = `SELECT token_hash, revoked_at, reason FROM session_revocation_list ORDER BY revoked_at DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var revocations []*session.SessionRevocation

	for rows.Next() {
		var rev session.SessionRevocation

		var revokedAt int64

		var reasonVal sql.NullString
		if err := rows.Scan(&rev.TokenHash, &revokedAt, &reasonVal); err != nil {
			return nil, err
		}

		rev.RevokedAt = time.UnixMilli(revokedAt)
		if reasonVal.Valid {
			rev.Reason = reasonVal.String
		}

		revocations = append(revocations, &rev)
	}

	return revocations, rows.Err()
}

// CleanupSessionRevocations removes revocation entries older than the specified duration.
func (r *SessionRepository) CleanupSessionRevocations(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan).UnixMilli()
	query := `DELETE FROM session_revocation_list WHERE revoked_at < ?`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	n, _ := result.RowsAffected()

	return int(n), nil
}

// RevokeAllUserSessions revokes all sessions for a specific user.
func (r *SessionRepository) RevokeAllUserSessions(ctx context.Context, userID string) error {
	query := `SELECT id FROM auth_sessions WHERE user_id = ?`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			continue
		}

		if err := r.AddSessionRevocation(ctx, sessionID, "operator_logout"); err != nil {
			return err
		}
	}

	return rows.Err()
}

// =============================================================================.
// Refresh Token Repository (Implements refresh_token.Repository).
// =============================================================================.

// Ensure RefreshTokenRepository implements refresh_token.Repository.
var _ refresh_token.Repository = (*RefreshTokenRepository)(nil)

// RefreshTokenRepository implements refresh_token.Repository using SQLite.
type RefreshTokenRepository struct {
	db *sql.DB
}

// NewRefreshTokenRepository creates a new RefreshTokenRepository.
func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create creates a new refresh token.
func (r *RefreshTokenRepository) Create(ctx context.Context, rt *refresh_token.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, token_hash, user_id, session_id, expires_at, created_at, revoked_at, replaced_by_id, is_revoked)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// replaced_by_id is a self-reference (refresh-token rotation). An unset value
	// must be SQL NULL, not the empty string: the column is a FOREIGN KEY, and an
	// empty string is not a row, so it fails the constraint. This made every login
	// 500 with "FOREIGN KEY constraint failed".
	_, err := r.db.ExecContext(ctx, query,
		rt.ID, rt.TokenHash, rt.UserID, rt.SessionID, rt.ExpiresAt, rt.CreatedAt, rt.RevokedAt, nullString(rt.ReplacedByID), rt.IsRevoked,
	)

	return err
}

// FindByID retrieves a refresh token by ID.
func (r *RefreshTokenRepository) FindByID(ctx context.Context, id string) (*refresh_token.RefreshToken, error) {
	query := `
		SELECT id, token_hash, user_id, session_id, expires_at, created_at, revoked_at, replaced_by_id, is_revoked
		FROM refresh_tokens WHERE id = ?`

	var rt refresh_token.RefreshToken

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&rt.ID, &rt.TokenHash, &rt.UserID, &rt.SessionID, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt, &rt.ReplacedByID, &rt.IsRevoked,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, refresh_token.ErrNotFound
	}

	return &rt, err
}

// FindByTokenHash retrieves a refresh token by its hash.
func (r *RefreshTokenRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*refresh_token.RefreshToken, error) {
	query := `
		SELECT id, token_hash, user_id, session_id, expires_at, created_at, revoked_at, replaced_by_id, is_revoked
		FROM refresh_tokens WHERE token_hash = ?`

	var rt refresh_token.RefreshToken

	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&rt.ID, &rt.TokenHash, &rt.UserID, &rt.SessionID, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt, &rt.ReplacedByID, &rt.IsRevoked,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, refresh_token.ErrNotFound
	}

	return &rt, err
}

// Revoke revokes a refresh token by ID.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	query := `UPDATE refresh_tokens SET is_revoked = true, revoked_at = ? WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, time.Now(), id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return refresh_token.ErrNotFound
	}

	return nil
}

// RevokeByTokenHash revokes a refresh token by its hash.
func (r *RefreshTokenRepository) RevokeByTokenHash(ctx context.Context, tokenHash string) error {
	query := `UPDATE refresh_tokens SET is_revoked = true, revoked_at = ? WHERE token_hash = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), tokenHash)

	return err
}

// RevokeAllForUser revokes all refresh tokens for a user.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = true, revoked_at = ? WHERE user_id = ? AND is_revoked = false`
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)

	return err
}

// CleanupExpired removes expired refresh tokens older than the specified duration.
func (r *RefreshTokenRepository) CleanupExpired(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	query := `DELETE FROM refresh_tokens WHERE expires_at < ? AND is_revoked = true AND revoked_at < ?`

	result, err := r.db.ExecContext(ctx, query, cutoff, cutoff)
	if err != nil {
		return 0, err
	}

	rows, err := result.RowsAffected()

	return int(rows), err
}
