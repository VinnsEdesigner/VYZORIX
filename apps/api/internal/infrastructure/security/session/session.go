// Package session provides session cookie management.
package session

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	CookieName       = "vyz_session"
	CookieMaxAge     = 86400
	CookiePath       = "/"
	EncryptionKeyLen = 32
)

var (
	ErrInvalidCookie    = errors.New("invalid session cookie")
	ErrExpiredCookie    = errors.New("session cookie expired")
	ErrDecryptionFailed = errors.New("cookie decryption failed")
)

// Repository defines the interface for session persistence.
type Repository interface {
	FindByID(ctx context.Context, id string) (*Session, error)
	FindByUserID(ctx context.Context, userID string) ([]*Session, error)
	Create(ctx context.Context, s *Session) error
	// UpdateSessionMeta backfills the client IP and user agent on a session
	// created before those fields were captured.
	UpdateSessionMeta(ctx context.Context, id, ipAddress, userAgent string) error
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) (int, error)
	Extend(ctx context.Context, id string, newExpiry time.Time) error
	RevokeAllUserSessions(ctx context.Context, userID string) error
}

// Session represents an authentication session.
type Session struct {
	ID                     string
	UserID                 string
	SelectedOrganizationID string
	ExpiresAt              time.Time
	CreatedAt              time.Time
	IPAddress              string
	UserAgent              string
}

// Manager handles encrypted session cookies for HttpOnly cookie auth.
type Manager struct {
	sessionRepo   Repository
	encryptionKey []byte
	// cookieSecure is the Secure attribute on the session cookie. It defaults to
	// true and only a development build turns it off: a Secure cookie is never sent
	// over plain HTTP, so a local run against http://localhost cannot authenticate at
	// all. Production must keep it on, or the session cookie travels in the clear.
	cookieSecure bool
}

// NewManager creates a new session manager with the given secret.
func NewManager(secret string) *Manager {
	h := sha512.New()
	h.Write([]byte(secret))
	fullHash := h.Sum(nil)
	key := make([]byte, EncryptionKeyLen)
	copy(key, fullHash)

	return &Manager{
		encryptionKey: key,
		// Secure by default. SetInsecureCookie flips it for a development build that
		// runs over plain HTTP.
		cookieSecure: true,
	}
}

// SetInsecureCookie disables the Secure attribute on the session cookie.
//
// It exists for local development over http://, where a Secure cookie is dropped
// by every client and authentication is impossible. It must never be called in
// production: without Secure, the session cookie is sent over plain HTTP.
func (sm *Manager) SetInsecureCookie(insecure bool) {
	sm.cookieSecure = !insecure
}

// SetRepository sets the session repository for session management.
func (sm *Manager) SetRepository(repo Repository) {
	sm.sessionRepo = repo
}

// EncryptSessionID encrypts a session ID for storage in a cookie value.
func (sm *Manager) EncryptSessionID(sessionID string) (string, error) {
	block, err := aes.NewCipher(sm.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(sessionID), nil)

	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// DecryptSessionID decrypts a session ID from a cookie value.
func (sm *Manager) DecryptSessionID(cookieValue string) (string, error) {
	ciphertext, err := base64.RawURLEncoding.DecodeString(cookieValue)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base64 encoding", ErrDecryptionFailed)
	}

	block, err := aes.NewCipher(sm.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("%w: ciphertext too short", ErrDecryptionFailed)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(plaintext), nil
}

// CreateCookie creates an HttpOnly session cookie for the given session.
// The sessionID is encrypted and stored in the cookie value.
func (sm *Manager) CreateCookie(sessionID string) (*http.Cookie, error) {
	encryptedID, err := sm.EncryptSessionID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt session: %w", err)
	}

	return &http.Cookie{
		Name:     CookieName,
		Value:    encryptedID,
		Path:     CookiePath,
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		Secure:   sm.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	}, nil
}

// CreateCookieWithExpiry creates an HttpOnly session cookie with custom expiry.
// The sessionID is encrypted and stored in the cookie value.
func (sm *Manager) CreateCookieWithExpiry(sessionID string, maxAge int) (*http.Cookie, error) {
	encryptedID, err := sm.EncryptSessionID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt session: %w", err)
	}

	return &http.Cookie{
		Name:     CookieName,
		Value:    encryptedID,
		Path:     CookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   sm.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	}, nil
}

// ClearCookie creates an expired cookie to clear the session.
func (sm *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     CookiePath,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   sm.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
}

// ExtractFromCookie extracts the session ID from a session cookie value.
func (sm *Manager) ExtractFromCookie(cookieValue string) (string, error) {
	if cookieValue == "" {
		return "", ErrInvalidCookie
	}

	return sm.DecryptSessionID(cookieValue)
}

// HashUserID creates a SHA-512 hash of a user ID for database lookups.
// Uses SHA-512 for stronger security.
func HashUserID(userID string) string {
	h := sha512.Sum512([]byte(userID))
	return hex.EncodeToString(h[:])
}

// ListActiveSessions returns all active (non-expired) sessions for a user.
func (sm *Manager) ListActiveSessions(ctx context.Context, userID string) ([]*Session, error) {
	if sm.sessionRepo == nil {
		return nil, errors.New("session repository not configured")
	}

	sessions, err := sm.sessionRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter out expired sessions.
	active := make([]*Session, 0, len(sessions))
	now := time.Now()
	for _, s := range sessions {
		if s.ExpiresAt.After(now) {
			active = append(active, s)
		}
	}

	return active, nil
}

// RevokeSession revokes a specific session by ID.
func (sm *Manager) RevokeSession(ctx context.Context, sessionID string) error {
	if sm.sessionRepo == nil {
		return errors.New("session repository not configured")
	}

	return sm.sessionRepo.Delete(ctx, sessionID)
}
