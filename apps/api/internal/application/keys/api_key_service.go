package keys

import (
	"context"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain"
	infraStorage "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

// Config holds the API key service configuration.
type Config struct {
	Prefix            string // e.g., "vxyz".
	MaxPerMonth       int    // Max keys per user per month.
	MaxNameLength     int    // Max key name length.
	DefaultExpiryDays int    // Default expiry (0 = never).
	MaxExpiryDays     int    // Max expiry days.
	PrefixLength      int    // Prefix length for display.
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Prefix:            "vxyz",
		MaxPerMonth:       20,
		MaxNameLength:     64,
		DefaultExpiryDays: 0,
		MaxExpiryDays:     365,
		PrefixLength:      8,
	}
}

// APIKeyService handles API key business logic.
type APIKeyService struct {
	repo   infraStorage.APIKeyRepository
	config Config
}

// NewAPIKeyService creates a new API key service.
func NewAPIKeyService(repo infraStorage.APIKeyRepository, config Config) *APIKeyService {
	return &APIKeyService{
		repo:   repo,
		config: config,
	}
}

// GenerateKey generates a new API key and returns the full key (only time it's available).
func (s *APIKeyService) GenerateKey(ctx context.Context, userID, orgID string, req *domain.CreateAPIKeyRequest) (*domain.APIKeyWithFullKey, error) {
	// Validate name.
	if len(req.Name) > s.config.MaxNameLength {
		return nil, domain.ErrKeyNameTooLong
	}

	// Validate scope.
	if !req.Scope.IsValid() {
		return nil, domain.ErrInvalidScope
	}

	// Check for duplicate name per user.
	exists, err := s.repo.ExistsByUserAndName(ctx, userID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check key name: %w", err)
	}
	if exists {
		return nil, domain.ErrKeyNameConflict
	}

	// Check monthly limit.
	count, err := s.repo.CountByUserThisMonth(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count keys: %w", err)
	}
	if count >= s.config.MaxPerMonth {
		return nil, domain.ErrMonthlyLimitExceeded
	}

	// Generate the full key.
	fullKey, err := generateRandomKey(s.config.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Hash the key.
	keyHash := hashKey(fullKey)

	// Derive the signing secret for HMAC request signing (Domain A).
	signingSecret := deriveAPIKeySigningSecret(fullKey)

	// Calculate prefix.
	prefix := fullKey[:s.config.PrefixLength]

	// Calculate expiry.
	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		if *req.ExpiresInDays > s.config.MaxExpiryDays {
			return nil, domain.ErrInvalidExpiryDays
		}
		exp := time.Now().AddDate(0, 0, *req.ExpiresInDays)
		expiresAt = &exp
	}

	now := time.Now()
	key := &infraStorage.APIKey{
		ID:             uuid.Must(uuid.NewV7()).String(),
		UserID:         userID,
		OrganizationID: orgID,
		Name:           req.Name,
		KeyPrefix:      prefix,
		KeyHash:        keyHash,
		SigningSecret:  signingSecret,
		Scope:          string(req.Scope),
		ExpiresAt:      toMillis(expiresAt),
		IsActive:       true,
		CreatedAt:      now.UnixMilli(),
		UpdatedAt:      now.UnixMilli(),
	}

	if err := s.repo.Create(ctx, key); err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	return &domain.APIKeyWithFullKey{
		APIKey: domain.APIKey{
			ID:        key.ID,
			UserID:    key.UserID,
			Name:      key.Name,
			KeyPrefix: key.KeyPrefix,
			Scope:     domain.Scope(key.Scope),
			ExpiresAt: fromMillis(key.ExpiresAt),
			IsActive:  key.IsActive,
			CreatedAt: fromMillisVal(key.CreatedAt),
		},
		FullKey:    fullKey,
		SigningKey: signingSecret,
	}, nil
}

// ValidateKey validates an API key and returns the key if valid.
func (s *APIKeyService) ValidateKey(ctx context.Context, fullKey string) (*domain.APIKey, error) {
	keyHash := hashKeyValue(fullKey)

	key, err := s.repo.GetByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, domain.ErrInvalidAPIKey
	}

	apiKey := toDomainAPIKey(key)

	if !apiKey.IsValid() {
		if apiKey.IsExpired() {
			return nil, domain.ErrAPIKeyExpired
		}
		if !apiKey.IsActive {
			return nil, domain.ErrAPIKeyRevoked
		}
		return nil, domain.ErrAPIKeyInactive
	}

	return apiKey, nil
}

// IncrementUsage increments the request counter for an API key.
// VerifyKey verifies a key against a stored hash using constant-time comparison.
func (s *APIKeyService) VerifyKey(fullKey, keyHash string) bool {
	hashedKey := hashKeyValue(fullKey)
	// Use constant-time comparison to prevent timing attacks.
	return subtle.ConstantTimeCompare([]byte(hashedKey), []byte(keyHash)) == 1
}

func (s *APIKeyService) IncrementUsage(ctx context.Context, keyID string) error {
	return s.repo.IncrementRequestCount(ctx, keyID)
}

// ListKeys lists all API keys for a user.
func (s *APIKeyService) ListKeys(ctx context.Context, userID string, page, limit int) (*domain.ListAPIKeysResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	keys, total, err := s.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	// Count keys created this month.
	monthlyCount, err := s.repo.CountByUserThisMonth(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count monthly keys: %w", err)
	}

	responses := make([]domain.APIKeyResponse, len(keys))
	for i, key := range keys {
		responses[i] = toDomainAPIKey(key).ToResponse()
	}

	totalPages := (total + limit - 1) / limit

	return &domain.ListAPIKeysResponse{
		Keys: responses,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
		MonthlyLimit:         s.config.MaxPerMonth,
		KeysCreatedThisMonth: monthlyCount,
	}, nil
}

// GetKey gets a single API key by ID.
func (s *APIKeyService) GetKey(ctx context.Context, userID, keyID string) (*domain.APIKey, error) {
	key, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		return nil, domain.ErrAPIKeyNotFound
	}

	// Ensure the key belongs to the user.
	if key.UserID != userID {
		return nil, domain.ErrAPIKeyNotFound
	}

	return toDomainAPIKey(key), nil
}

// UpdateKey updates an API key (name and/or scope).
func (s *APIKeyService) UpdateKey(ctx context.Context, userID, keyID string, req *domain.UpdateAPIKeyRequest) (*domain.APIKey, error) {
	key, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		return nil, domain.ErrAPIKeyNotFound
	}

	// Ensure the key belongs to the user.
	if key.UserID != userID {
		return nil, domain.ErrAPIKeyNotFound
	}

	// Validate name if provided.
	if req.Name != nil {
		if len(*req.Name) > s.config.MaxNameLength {
			return nil, domain.ErrKeyNameTooLong
		}
		// Check for duplicate name (excluding current key).
		exists, err := s.repo.ExistsByUserAndNameExcluding(ctx, userID, *req.Name, keyID)
		if err != nil {
			return nil, fmt.Errorf("failed to check key name: %w", err)
		}
		if exists {
			return nil, domain.ErrKeyNameConflict
		}
		key.Name = *req.Name
	}

	// Validate scope if provided.
	if req.Scope != nil {
		if !req.Scope.IsValid() {
			return nil, domain.ErrInvalidScope
		}
		key.Scope = string(*req.Scope)
	}

	key.UpdatedAt = time.Now().UnixMilli()

	if err := s.repo.Update(ctx, key); err != nil {
		return nil, fmt.Errorf("failed to update key: %w", err)
	}

	return toDomainAPIKey(key), nil
}

// RevokeKey revokes an API key.
func (s *APIKeyService) RevokeKey(ctx context.Context, userID, keyID string) error {
	key, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		return domain.ErrAPIKeyNotFound
	}

	// Ensure the key belongs to the user.
	if key.UserID != userID {
		return domain.ErrAPIKeyNotFound
	}

	return s.repo.Revoke(ctx, keyID)
}

// ForceRevokeKey force revokes an API key (super admin only).
func (s *APIKeyService) ForceRevokeKey(ctx context.Context, keyID string) error {
	return s.repo.Revoke(ctx, keyID)
}

// RotateKey rotates an API key, generating a new key and invalidating the old one.
func (s *APIKeyService) RotateKey(ctx context.Context, userID, keyID string) (*domain.APIKeyWithFullKey, error) {
	key, err := s.repo.GetByID(ctx, keyID)
	if err != nil {
		return nil, domain.ErrAPIKeyNotFound
	}

	// Ensure the key belongs to the user.
	if key.UserID != userID {
		return nil, domain.ErrAPIKeyNotFound
	}

	// Preserve expiry time before revoking (copy value, not pointer).
	var expiresAt *int64
	if key.ExpiresAt != nil {
		expiresAtVal := *key.ExpiresAt
		expiresAt = &expiresAtVal
	}

	// Revoke the old key.
	if revokeErr := s.repo.Revoke(ctx, keyID); revokeErr != nil {
		return nil, fmt.Errorf("failed to revoke old key: %w", revokeErr)
	}

	// Re-check monthly limit (we're creating a new key).
	count, err := s.repo.CountByUserThisMonth(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count keys: %w", err)
	}
	if count >= s.config.MaxPerMonth {
		return nil, domain.ErrMonthlyLimitExceeded
	}

	// Generate the new key.
	fullKey, err := generateRandomKey(s.config.Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	keyHash := hashKey(fullKey)
	signingSecret := deriveAPIKeySigningSecret(fullKey)

	prefix := fullKey[:s.config.PrefixLength]
	now := time.Now()

	newKey := &infraStorage.APIKey{
		ID:             uuid.Must(uuid.NewV7()).String(),
		UserID:         userID,
		OrganizationID: key.OrganizationID,
		Name:           key.Name,
		KeyPrefix:      prefix,
		KeyHash:        keyHash,
		SigningSecret:  signingSecret,
		Scope:          key.Scope,
		ExpiresAt:      expiresAt, // Keep the same expiry (copied value, not pointer).
		IsActive:       true,
		CreatedAt:      now.UnixMilli(),
		UpdatedAt:      now.UnixMilli(),
	}

	if err := s.repo.Create(ctx, newKey); err != nil {
		return nil, fmt.Errorf("failed to create new key: %w", err)
	}

	return &domain.APIKeyWithFullKey{
		APIKey: domain.APIKey{
			ID:        newKey.ID,
			UserID:    newKey.UserID,
			Name:      newKey.Name,
			KeyPrefix: newKey.KeyPrefix,
			Scope:     domain.Scope(newKey.Scope),
			ExpiresAt: fromMillis(newKey.ExpiresAt),
			IsActive:  newKey.IsActive,
			CreatedAt: fromMillisVal(newKey.CreatedAt),
		},
		FullKey:    fullKey,
		SigningKey: signingSecret,
	}, nil
}

// generateRandomKey generates a random API key with the given prefix.
func generateRandomKey(prefix string) (string, error) {
	const keyLength = 32 // 32 bytes = 64 hex characters.
	bytes := make([]byte, keyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(bytes)), nil
}

// hashKey hashes a key using Argon2id.
func hashKey(key string) string {
	// Use a fixed salt for deterministic hashing (the key itself is the secret).
	salt := []byte("vyzorix-api-key-v1")
	hash := argon2.IDKey([]byte(key), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(hash)
}

// hashKeyValue is a convenience function to hash a key value.
func hashKeyValue(key string) string {
	hash := hashKey(key)
	return hash
}

// deriveAPIKeySigningSecret derives a deterministic HMAC signing secret from.
// the full API key value. This lets API-key-authenticated clients sign requests.
// (Domain A: client↔server request signing) using the same X-Vyzorix-* header.
// scheme as session-authenticated clients, without storing a separate secret.
// The server stores this derived secret; the client derives it from the full.
// key value it already holds.
func deriveAPIKeySigningSecret(fullKey string) string {
	h := sha512.Sum512([]byte(fullKey))
	return hex.EncodeToString(h[:])
}

// toDomainAPIKey converts an infrastructure API key to a domain API key.
func toDomainAPIKey(key *infraStorage.APIKey) *domain.APIKey {
	domain := &domain.APIKey{
		ID:             key.ID,
		UserID:         key.UserID,
		OrganizationID: key.OrganizationID,
		Name:           key.Name,
		KeyPrefix:      key.KeyPrefix,
		SigningSecret:  key.SigningSecret,
		Scope:          domain.Scope(key.Scope),
		IsActive:       key.IsActive,
		RequestCount:   key.RequestCount,
		CreatedAt:      fromMillisVal(key.CreatedAt),
		UpdatedAt:      fromMillisVal(key.UpdatedAt),
	}

	if key.ExpiresAt != nil {
		t := time.UnixMilli(*key.ExpiresAt)
		domain.ExpiresAt = &t
	}

	if key.LastRequest != nil {
		t := time.UnixMilli(*key.LastRequest)
		domain.LastRequest = &t
	}

	if key.RevokedAt != nil {
		t := time.UnixMilli(*key.RevokedAt)
		domain.RevokedAt = &t
	}

	return domain
}

// toMillis converts a time to Unix milliseconds.
func toMillis(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

// fromMillis converts Unix milliseconds to a time.
func fromMillis(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	t := time.UnixMilli(*ms)
	return &t
}

// fromMillisVal converts Unix milliseconds to a time (non-pointer).
func fromMillisVal(ms int64) time.Time {
	return time.UnixMilli(ms)
}

// IsValidScope checks if a string is a valid scope.
func IsValidScope(s string) bool {
	return domain.Scope(strings.ToLower(s)).IsValid()
}

// ListAllKeys lists all API keys across all users (super admin only).
// userID (non-empty) filters to a single user; search (non-empty) does.
// a case-insensitive LIKE match on key name, prefix, and user name/email.
func (s *APIKeyService) ListAllKeys(ctx context.Context, page, limit int, userID, search string) (*domain.ListAllAPIKeysResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	rows, total, err := s.repo.ListAllWithUser(ctx, limit, offset, userID, search)
	if err != nil {
		return nil, fmt.Errorf("failed to list all keys: %w", err)
	}

	responses := make([]domain.AdminAPIKeyResponse, len(rows))
	for i, row := range rows {
		k := toDomainAPIKey(&row.APIKey)
		responses[i] = domain.AdminAPIKeyResponse{
			APIKeyResponse: k.ToResponse(),
			UserID:         k.UserID,
			UserName:       row.UserName,
		}
	}

	totalPages := (total + limit - 1) / limit

	return &domain.ListAllAPIKeysResponse{
		Keys: responses,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// GetGlobalStats returns global API key statistics (super admin only).
// Builds aggregates from the storage layer: total/active/revoked counts,
// cumulative request totals by scope, and top users by request volume.
func (s *APIKeyService) GetGlobalStats(ctx context.Context) (*domain.GlobalAPIKeyStats, error) {
	totalKeys, err := s.repo.CountAllIncludingRevoked(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count all keys: %w", err)
	}
	activeKeys, err := s.repo.CountAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count active keys: %w", err)
	}
	revokedKeys, err := s.repo.CountRevoked(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count revoked keys: %w", err)
	}
	requestsByScope, err := s.repo.SumRequestsByScope(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sum requests by scope: %w", err)
	}
	topOps, totalUsers, err := s.repo.TopUsersByRequests(ctx, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to query top users: %w", err)
	}

	var totalRequests int64
	for _, sum := range requestsByScope {
		totalRequests += sum
	}

	topStats := make([]domain.TopUserStat, 0, len(topOps))
	for _, op := range topOps {
		activeCount, cErr := s.repo.CountByUserThisMonth(ctx, op.UserID)
		if cErr != nil {
			return nil, fmt.Errorf("failed to count operator keys: %w", cErr)
		}
		topStats = append(topStats, domain.TopUserStat{
			UserID:         op.UserID,
			UserName:       op.UserName,
			TotalRequests:  op.TotalRequests,
			ActiveKeyCount: activeCount,
		})
	}

	return &domain.GlobalAPIKeyStats{
		TotalKeys:       totalKeys,
		ActiveKeys:      activeKeys,
		RevokedKeys:     revokedKeys,
		TotalRequests:   totalRequests,
		RequestsByScope: requestsByScope,
		TopUsers:        topStats,
		TotalUsers:      totalUsers,
		MaxPerMonth:     s.config.MaxPerMonth,
	}, nil
}
