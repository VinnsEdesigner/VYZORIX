package storage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/transaction"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security/password"
)

// Ensure UserRepository implements user.Repository.
var _ user.Repository = (*UserRepository)(nil)

// UserRepository implements user.Repository using SQLite.
type UserRepository struct {
	db     *sql.DB
	cipher *secretCipher
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// SetSecretCipher enables encryption at rest for reversible secrets (the Slack
// bot token). Without it those values are stored as-is, which keeps tests and
// tools that construct the repository directly working.
func (r *UserRepository) SetSecretCipher(secret string) {
	if secret != "" {
		r.cipher = newSecretCipher(secret)
	}
}

func (r *UserRepository) encryptSecret(plaintext string) string {
	if r.cipher == nil || plaintext == "" {
		return plaintext
	}
	encrypted, err := r.cipher.encrypt(plaintext)
	if err != nil {
		return plaintext
	}
	return encrypted
}

func (r *UserRepository) decryptSecret(stored string) string {
	if r.cipher == nil {
		return stored
	}
	return r.cipher.decrypt(stored)
}

// getQuerier returns the transaction from context if available, otherwise the db.
func (r *UserRepository) getQuerier(ctx context.Context) Querier {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// queryRow is a helper that uses transaction-aware querier.
func (r *UserRepository) queryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return r.getQuerier(ctx).QueryRowContext(ctx, query, args...)
}

// queryRows is a helper that uses transaction-aware querier.
func (r *UserRepository) queryRows(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return r.getQuerier(ctx).QueryContext(ctx, query, args...)
}

// exec is a helper that uses transaction-aware querier.
func (r *UserRepository) exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return r.getQuerier(ctx).ExecContext(ctx, query, args...)
}

// FindByID retrieves a user by ID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*user.User, error) {
	query := `
		SELECT id, email, name, avatar_url, password_hash, google_id, github_id, 
		       mfa_secret, mfa_secret_mac, mfa_enabled, mfa_enabled_at, backup_codes, email_verified, created_at, updated_at, fcm_token,
		       last_organization_id, instance_role, disabled_at
		FROM users WHERE id = ?`

	var op user.User

	var googleID, githubID, mfaSecret, mfaSecretMAC, mfaBackupCodes, fcmToken, lastOrgID sql.NullString
	var instanceRole sql.NullString
	var disabledAt sql.NullInt64
	var mfaEnabledAt sql.NullInt64
	var createdAt, updatedAt interface{} // Handle both TEXT and INTEGER.

	err := r.queryRow(ctx, query, id).Scan(
		&op.ID, &op.Email, &op.Name, &op.AvatarURL, &op.PasswordHash,
		&googleID, &githubID, &mfaSecret, &mfaSecretMAC, &op.MFAEnabled, &mfaEnabledAt,
		&mfaBackupCodes, &op.EmailVerified, &createdAt, &updatedAt, &fcmToken, &lastOrgID, &instanceRole, &disabledAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	op.GoogleID = googleID.String
	op.GitHubID = githubID.String
	op.MFASecret = mfaSecret.String
	op.MFASecretMAC = mfaSecretMAC.String
	op.FCMToken = fcmToken.String
	op.LastOrganizationID = lastOrgID.String
	op.InstanceRole = permission.ParseInstanceRole(instanceRole.String)
	op.Disabled = disabledAt.Valid

	// Parse timestamps - handle both TEXT and INTEGER formats.
	op.CreatedAt = parseTimestamp(createdAt)
	op.UpdatedAt = parseTimestamp(updatedAt)
	if mfaEnabledAt.Valid {
		t := time.UnixMilli(mfaEnabledAt.Int64)
		op.MFAEnabledAt = &t
	}

	// Parse backup codes from JSON.
	if mfaBackupCodes.Valid && mfaBackupCodes.String != "" {
		if err := json.Unmarshal([]byte(mfaBackupCodes.String), &op.BackupCodes); err != nil {
			op.BackupCodes = []string{}
		}
	} else {
		op.BackupCodes = []string{}
	}

	return &op, nil
}

// FindByEmail retrieves a user by email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `
		SELECT id, email, name, avatar_url, password_hash, google_id, github_id, 
		       mfa_secret, mfa_secret_mac, mfa_enabled, mfa_enabled_at, email_verified, created_at, updated_at, fcm_token,
		       last_organization_id, instance_role, disabled_at
		FROM users WHERE email = ?`

	var op user.User

	var googleID, githubID, mfaSecret, mfaSecretMAC, fcmToken, lastOrgID sql.NullString
	var instanceRole sql.NullString
	var disabledAt sql.NullInt64
	var mfaEnabledAt sql.NullInt64
	var createdAt, updatedAt interface{} // Handle both TEXT and INTEGER.

	err := r.queryRow(ctx, query, strings.ToLower(email)).Scan(
		&op.ID, &op.Email, &op.Name, &op.AvatarURL, &op.PasswordHash,
		&googleID, &githubID, &mfaSecret, &mfaSecretMAC, &op.MFAEnabled, &mfaEnabledAt,
		&op.EmailVerified, &createdAt, &updatedAt, &fcmToken, &lastOrgID, &instanceRole, &disabledAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	op.GoogleID = googleID.String
	op.GitHubID = githubID.String
	op.MFASecret = mfaSecret.String
	op.MFASecretMAC = mfaSecretMAC.String
	op.FCMToken = fcmToken.String
	op.LastOrganizationID = lastOrgID.String
	op.InstanceRole = permission.ParseInstanceRole(instanceRole.String)
	op.Disabled = disabledAt.Valid

	// Parse timestamps - handle both TEXT (ISO8601) and INTEGER (Unix ms).
	op.CreatedAt = parseTimestamp(createdAt)
	op.UpdatedAt = parseTimestamp(updatedAt)
	if mfaEnabledAt.Valid {
		t := time.UnixMilli(mfaEnabledAt.Int64)
		op.MFAEnabledAt = &t
	}

	return &op, nil
}

// parseTimestamp handles both TEXT (ISO8601) and INTEGER (Unix milliseconds) timestamp formats.
func parseTimestamp(value interface{}) time.Time {
	if value == nil {
		return time.Time{}
	}
	switch v := value.(type) {
	case time.Time:
		// The driver returns a time.Time for a TIMESTAMP column. Without this case the
		// function fell through to the zero time, so every timestamp read from such a
		// column silently became 0001-01-01.
		return v
	case int64:
		return time.UnixMilli(v)
	case int:
		return time.UnixMilli(int64(v))
	case float64:
		return time.UnixMilli(int64(v))
	case string:
		// Try parsing as Unix timestamp in milliseconds.
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			if ms > 1000000000000 { // Likely milliseconds.
				return time.UnixMilli(ms)
			}
			if ms > 1000000000 { // Likely seconds.
				return time.Unix(ms, 0)
			}
		}
		// Try parsing as ISO8601.
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t
		}
		// Try other common formats.
		formats := []string{
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02T15:04:05",
			// The exact shape the SQLite driver writes for a time.Time value: a space
			// separator with a numeric offset, e.g. "2026-11-04 01:08:22.4558+00:00".
			// Without this the helper silently returned the zero time for every row
			// written that way, which is worse than an error: a caller sees a valid
			// (zero) time and cannot tell it was never parsed.
			"2006-01-02 15:04:05.999999999-07:00",
			"2006-01-02 15:04:05.999999999Z07:00",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05-07:00",
			"2006-01-02 15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// FindByGoogleID retrieves a user by Google ID.
func (r *UserRepository) FindByGoogleID(ctx context.Context, googleID string) (*user.User, error) {
	query := `
		SELECT id, email, name, avatar_url, password_hash, google_id, github_id, 
		       mfa_secret, mfa_secret_mac, mfa_enabled, mfa_enabled_at, backup_codes, email_verified, created_at, updated_at, fcm_token,
		       last_organization_id, instance_role, disabled_at
		FROM users WHERE google_id = ?`

	var op user.User

	var googleIDVal, githubID, mfaSecret, mfaSecretMAC, mfaBackupCodes, fcmToken, lastOrgID sql.NullString
	var instanceRole sql.NullString
	var disabledAt sql.NullInt64
	var mfaEnabledAt sql.NullInt64
	var createdAt, updatedAt interface{} // Handle both TEXT and INTEGER.

	err := r.queryRow(ctx, query, googleID).Scan(
		&op.ID, &op.Email, &op.Name, &op.AvatarURL, &op.PasswordHash,
		&googleIDVal, &githubID, &mfaSecret, &mfaSecretMAC, &op.MFAEnabled, &mfaEnabledAt,
		&mfaBackupCodes, &op.EmailVerified, &createdAt, &updatedAt, &fcmToken, &lastOrgID, &instanceRole, &disabledAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	op.GoogleID = googleIDVal.String
	op.GitHubID = githubID.String
	op.MFASecret = mfaSecret.String
	op.MFASecretMAC = mfaSecretMAC.String
	op.FCMToken = fcmToken.String
	op.LastOrganizationID = lastOrgID.String
	op.InstanceRole = permission.ParseInstanceRole(instanceRole.String)
	op.Disabled = disabledAt.Valid

	// Parse timestamps - handle both TEXT and INTEGER formats.
	op.CreatedAt = parseTimestamp(createdAt)
	op.UpdatedAt = parseTimestamp(updatedAt)
	if mfaEnabledAt.Valid {
		t := time.UnixMilli(mfaEnabledAt.Int64)
		op.MFAEnabledAt = &t
	}

	if mfaBackupCodes.Valid && mfaBackupCodes.String != "" {
		_ = json.Unmarshal([]byte(mfaBackupCodes.String), &op.BackupCodes)
	}

	return &op, nil
}

// FindByGitHubID retrieves a user by GitHub ID.
func (r *UserRepository) FindByGitHubID(ctx context.Context, githubID string) (*user.User, error) {
	query := `
		SELECT id, email, name, avatar_url, password_hash, google_id, github_id, 
		       mfa_secret, mfa_secret_mac, mfa_enabled, mfa_enabled_at, backup_codes, email_verified, created_at, updated_at, fcm_token,
		       last_organization_id, instance_role, disabled_at
		FROM users WHERE github_id = ?`

	var op user.User

	var googleID, githubIDVal, mfaSecret, mfaSecretMAC, mfaBackupCodes, fcmToken, lastOrgID sql.NullString
	var instanceRole sql.NullString
	var disabledAt sql.NullInt64
	var mfaEnabledAt sql.NullInt64
	var createdAt, updatedAt interface{} // Handle both TEXT and INTEGER.

	err := r.queryRow(ctx, query, githubID).Scan(
		&op.ID, &op.Email, &op.Name, &op.AvatarURL, &op.PasswordHash,
		&googleID, &githubIDVal, &mfaSecret, &mfaSecretMAC, &op.MFAEnabled, &mfaEnabledAt,
		&mfaBackupCodes, &op.EmailVerified, &createdAt, &updatedAt, &fcmToken, &lastOrgID, &instanceRole, &disabledAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, user.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	op.GoogleID = googleID.String
	op.GitHubID = githubIDVal.String
	op.MFASecret = mfaSecret.String
	op.MFASecretMAC = mfaSecretMAC.String
	op.FCMToken = fcmToken.String
	op.LastOrganizationID = lastOrgID.String
	op.InstanceRole = permission.ParseInstanceRole(instanceRole.String)
	op.Disabled = disabledAt.Valid

	// Parse timestamps - handle both TEXT and INTEGER formats.
	op.CreatedAt = parseTimestamp(createdAt)
	op.UpdatedAt = parseTimestamp(updatedAt)
	if mfaEnabledAt.Valid {
		t := time.UnixMilli(mfaEnabledAt.Int64)
		op.MFAEnabledAt = &t
	}

	if mfaBackupCodes.Valid && mfaBackupCodes.String != "" {
		_ = json.Unmarshal([]byte(mfaBackupCodes.String), &op.BackupCodes)
	}

	return &op, nil
}

// Create creates a new user.
func (r *UserRepository) Create(ctx context.Context, op *user.User) error {
	query := `
		INSERT INTO users (id, email, name, avatar_url, password_hash, google_id, github_id, 
		                       mfa_secret, mfa_secret_mac, mfa_enabled, email_verified, created_at, updated_at, fcm_token,
		                       last_organization_id, instance_role)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.exec(ctx, query,
		op.ID, strings.ToLower(op.Email), op.Name, op.AvatarURL, op.PasswordHash,
		nullString(op.GoogleID), nullString(op.GitHubID), nullString(op.MFASecret), nullString(op.MFASecretMAC),
		op.MFAEnabled, op.EmailVerified, op.CreatedAt.UnixMilli(), op.UpdatedAt.UnixMilli(), nullString(op.FCMToken), nullString(op.LastOrganizationID), instanceRoleOrDefault(op.InstanceRole),
	)
	// Handle race condition: if UNIQUE constraint fails, return ErrUserExists.
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return user.ErrEmailExists
	}

	return err
}

// Update updates an existing user.
func (r *UserRepository) Update(ctx context.Context, op *user.User) error {
	query := `
		UPDATE users 
		SET email = ?, name = ?, avatar_url = ?, password_hash = ?, google_id = ?, github_id = ?,
		    mfa_secret = ?, mfa_secret_mac = ?, mfa_enabled = ?, email_verified = ?, fcm_token = ?,
		    last_organization_id = ?, updated_at = ?
		WHERE id = ?`

	result, err := r.exec(ctx, query,
		strings.ToLower(op.Email), op.Name, op.AvatarURL, op.PasswordHash,
		nullString(op.GoogleID), nullString(op.GitHubID), nullString(op.MFASecret), nullString(op.MFASecretMAC),
		op.MFAEnabled, op.EmailVerified, nullString(op.FCMToken), nullString(op.LastOrganizationID), time.Now().UnixMilli(), op.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// Delete deletes a user.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	result, err := r.exec(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// Count returns the total number of users.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.queryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)

	return count, err
}

// userSortColumns whitelists the ORDER BY columns for ListFiltered. The chosen
// value is interpolated into the query, so it must never come from raw input.
var userSortColumns = map[string]string{
	"email":         "email",
	"name":          "name",
	"created_at":    "created_at",
	"last_login_at": "last_login_at",
}

// List returns a paginated list of users, newest first.
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*user.User, int, error) {
	return r.ListFiltered(ctx, limit, offset, "", "created_at", true)
}

// ListFiltered returns a paginated list of users, optionally narrowed by a
// case-insensitive search over email and name, ordered by a whitelisted column.
// An unknown sort column falls back to created_at rather than erroring, so a
// stale client cannot break the list.
func (r *UserRepository) ListFiltered(ctx context.Context, limit, offset int, search, sort string, desc bool) ([]*user.User, int, error) {
	col, ok := userSortColumns[sort]
	if !ok {
		col = "created_at"
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}

	where := ""
	var filterArgs []interface{}
	if search != "" {
		where = " WHERE (email LIKE ? OR name LIKE ?)"
		like := "%" + search + "%"
		filterArgs = append(filterArgs, like, like)
	}

	query := `
		SELECT id, email, name, avatar_url, password_hash, google_id, github_id,
		       mfa_secret, mfa_secret_mac, mfa_enabled, mfa_enabled_at, email_verified, created_at, updated_at,
		       last_organization_id, instance_role, disabled_at
		FROM users` + where + ` ORDER BY ` + col + ` ` + dir + ` LIMIT ? OFFSET ?`

	args := append(append([]interface{}{}, filterArgs...), limit, offset)
	rows, err := r.queryRows(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var users []*user.User
	for rows.Next() {
		var op user.User
		var googleID, githubID, mfaSecret, mfaSecretMAC, lastOrgID sql.NullString
		var instanceRole sql.NullString
		var disabledAt sql.NullInt64
		var mfaEnabledAt sql.NullInt64
		var createdAt, updatedAt interface{} // Handle both TEXT and INTEGER.

		if err := rows.Scan(
			&op.ID, &op.Email, &op.Name, &op.AvatarURL, &op.PasswordHash,
			&googleID, &githubID, &mfaSecret, &mfaSecretMAC, &op.MFAEnabled, &mfaEnabledAt,
			&op.EmailVerified, &createdAt, &updatedAt, &lastOrgID, &instanceRole, &disabledAt,
		); err != nil {
			return nil, 0, err
		}

		op.GoogleID = googleID.String
		op.GitHubID = githubID.String
		op.MFASecret = mfaSecret.String
		op.MFASecretMAC = mfaSecretMAC.String
		op.LastOrganizationID = lastOrgID.String
		op.InstanceRole = permission.ParseInstanceRole(instanceRole.String)
		op.Disabled = disabledAt.Valid

		op.CreatedAt = parseTimestamp(createdAt)
		op.UpdatedAt = parseTimestamp(updatedAt)
		if mfaEnabledAt.Valid {
			t := time.UnixMilli(mfaEnabledAt.Int64)
			op.MFAEnabledAt = &t
		}

		users = append(users, &op)
	}

	var total int
	if err := r.queryRow(ctx, "SELECT COUNT(*) FROM users"+where, filterArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return users, total, rows.Err()
}

// ListByOrganization returns a page of users holding a membership in orgID,
// joined on that membership so an org-scoped caller can never see an account
// outside its own organisation. status filters the membership lifecycle: empty
// means every state except removed.
//
// The role and status reported are the membership's, not the account's: a
// person's role is per-organisation, so "admin" means admin of this org.
func (r *UserRepository) ListByOrganization(ctx context.Context, orgID string, limit, offset int, search, sort string, desc bool, status string) ([]*user.OrganizationUser, int, error) {
	col, ok := userSortColumns[sort]
	if !ok {
		col = "u.created_at"
	} else {
		col = "u." + col
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}

	where := " WHERE om.organization_id = ?"
	args := []interface{}{orgID}

	if status != "" {
		where += " AND om.status = ?"
		args = append(args, status)
	} else {
		where += " AND om.status != 'removed'"
	}

	if search != "" {
		where += " AND (u.email LIKE ? OR u.name LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	query := `
                SELECT u.id, u.email, u.name, u.avatar_url, u.mfa_enabled, u.email_verified,
                       u.created_at, u.updated_at, u.disabled_at, u.last_login_at,
                       om.id, om.role, om.status, om.joined_at
                FROM organization_members om
                INNER JOIN users u ON u.id = om.user_id` + where +
		` ORDER BY ` + col + ` ` + dir + ` LIMIT ? OFFSET ?`

	pageArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.queryRows(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var users []*user.OrganizationUser
	for rows.Next() {
		var ou user.OrganizationUser
		var disabledAt, lastLoginAt, joinedAtMs sql.NullInt64
		var createdAt, updatedAt interface{}

		if err := rows.Scan(
			&ou.ID, &ou.Email, &ou.Name, &ou.AvatarURL, &ou.MFAEnabled, &ou.EmailVerified,
			&createdAt, &updatedAt, &disabledAt, &lastLoginAt,
			&ou.MemberID, &ou.Role, &ou.Status, &joinedAtMs,
		); err != nil {
			return nil, 0, err
		}

		ou.CreatedAt = parseTimestamp(createdAt)
		ou.UpdatedAt = parseTimestamp(updatedAt)
		ou.Disabled = disabledAt.Valid
		if joinedAtMs.Valid {
			ou.JoinedAt = time.UnixMilli(joinedAtMs.Int64)
		}
		if lastLoginAt.Valid {
			t := time.UnixMilli(lastLoginAt.Int64)
			ou.LastSeenAt = &t
		}

		users = append(users, &ou)
	}

	countQuery := `
                SELECT COUNT(*)
                FROM organization_members om
                INNER JOIN users u ON u.id = om.user_id` + where

	var total int
	if err := r.queryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return users, total, rows.Err()
}

// SetDisabled stamps or clears the account-disable timestamp. A nil time
// re-enables the account. Returns user.ErrNotFound when no row matched.
func (r *UserRepository) SetDisabled(ctx context.Context, id string, disabledAt *time.Time) error {
	var value interface{}
	if disabledAt != nil {
		value = disabledAt.UnixMilli()
	}
	result, err := r.exec(ctx,
		"UPDATE users SET disabled_at = ?, updated_at = ? WHERE id = ?",
		value, time.Now().UnixMilli(), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return user.ErrNotFound
	}
	return nil
}

// UpdatePassword updates the password hash for a user.
func (r *UserRepository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?",
		passwordHash, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateMFA updates MFA settings for a user.
// When enabled is true, mfaEnabledAt should be set to the current time.
// When enabled is false, mfaEnabledAt is ignored (column will be set to nil).
func (r *UserRepository) UpdateMFA(ctx context.Context, id, secret, secretMAC string, enabled bool, mfaEnabledAt *time.Time) error {
	var result sql.Result
	var err error

	if enabled && mfaEnabledAt != nil {
		// Setting mfa_enabled_at when enabling MFA.
		result, err = r.exec(ctx,
			"UPDATE users SET mfa_secret = ?, mfa_secret_mac = ?, mfa_enabled = ?, mfa_enabled_at = ?, updated_at = ? WHERE id = ?",
			secret, secretMAC, enabled, mfaEnabledAt.UnixMilli(), time.Now(), id,
		)
	} else if enabled {
		// Enabling but no timestamp provided - use current time.
		now := time.Now()
		result, err = r.exec(ctx,
			"UPDATE users SET mfa_secret = ?, mfa_secret_mac = ?, mfa_enabled = ?, mfa_enabled_at = ?, updated_at = ? WHERE id = ?",
			secret, secretMAC, enabled, now.UnixMilli(), now, id,
		)
	} else {
		// Disabling MFA - clear mfa_enabled_at.
		result, err = r.exec(ctx,
			"UPDATE users SET mfa_secret = ?, mfa_secret_mac = ?, mfa_enabled = ?, mfa_enabled_at = NULL, updated_at = ? WHERE id = ?",
			secret, secretMAC, enabled, time.Now(), id,
		)
	}

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateUserMFA updates the MFA secret, MAC, and backup codes for a user.
func (r *UserRepository) UpdateUserMFA(ctx context.Context, userID, mfaSecret, mfaSecretMAC string, backupCodes []string) error {
	backupCodesJSON := "[]"

	if len(backupCodes) > 0 {
		data, err := json.Marshal(backupCodes)
		if err != nil {
			return err
		}

		backupCodesJSON = string(data)
	}

	result, err := r.exec(ctx,
		"UPDATE users SET mfa_secret = ?, mfa_secret_mac = ?, mfa_enabled = 1, mfa_backup_codes = ?, updated_at = ? WHERE id = ?",
		mfaSecret, mfaSecretMAC, backupCodesJSON, time.Now(), userID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// VerifyEmail marks a user's email as verified.
func (r *UserRepository) VerifyEmail(ctx context.Context, id string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET email_verified = 1, updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateEmailVerified updates the email verified status for a user.
func (r *UserRepository) UpdateEmailVerified(ctx context.Context, id string, verified bool) error {
	result, err := r.exec(ctx,
		"UPDATE users SET email_verified = ?, updated_at = ? WHERE id = ?",
		verified, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateGoogleID updates the Google ID for a user.
func (r *UserRepository) UpdateGoogleID(ctx context.Context, id, googleID string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET google_id = ?, updated_at = ? WHERE id = ?",
		googleID, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateGitHubID updates the GitHub ID for a user.
func (r *UserRepository) UpdateGitHubID(ctx context.Context, id, githubID string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET github_id = ?, updated_at = ? WHERE id = ?",
		githubID, time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateName updates the display name for a user.
func (r *UserRepository) UpdateName(ctx context.Context, id, name string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET name = ?, updated_at = ? WHERE id = ?",
		strings.TrimSpace(name), time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}
	return nil
}

// UpdateAvatarURL updates the avatar image URL for a user. An empty string
// clears it, so the client falls back to its default glyph.
func (r *UserRepository) UpdateAvatarURL(ctx context.Context, id, avatarURL string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET avatar_url = ?, updated_at = ? WHERE id = ?",
		strings.TrimSpace(avatarURL), time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// RecordLogin stamps the user's most recent login time.
func (r *UserRepository) RecordLogin(ctx context.Context, id string, at time.Time) error {
	result, err := r.exec(ctx,
		"UPDATE users SET last_login_at = ? WHERE id = ?",
		at.UnixMilli(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// LastLoginAt returns the user's most recent login time. The bool is false when
// the column is NULL (no login recorded since the column was introduced).
func (r *UserRepository) LastLoginAt(ctx context.Context, id string) (time.Time, bool, error) {
	var ms sql.NullInt64
	if err := r.queryRow(ctx, "SELECT last_login_at FROM users WHERE id = ?", id).Scan(&ms); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, false, user.ErrNotFound
		}
		return time.Time{}, false, err
	}

	if !ms.Valid {
		return time.Time{}, false, nil
	}

	return time.UnixMilli(ms.Int64), true, nil
}

// UpdateThresholds updates the alert thresholds for a user.
func (r *UserRepository) UpdateThresholds(ctx context.Context, id string, th user.Thresholds) error {
	result, err := r.exec(ctx,
		`UPDATE user_settings SET 
		 risk_warn = ?, risk_crit = ?, thermal_warn = ?, thermal_crit = ?, 
		 buffer_warn = ?, buffer_crit = ?, updated_at = ? WHERE user_id = ?`,
		th.RiskWarn, th.RiskCrit, th.ThermalWarn, th.ThermalCrit,
		th.BufferWarn, th.BufferCrit, time.Now().UnixMilli(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		// Settings row doesn't exist - insert with threshold values.
		result, err = r.exec(ctx,
			`INSERT INTO user_settings (user_id, risk_warn, risk_crit, thermal_warn, thermal_crit, 
			 buffer_warn, buffer_crit, updated_at, created_at)
			 SELECT ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)`,
			id, th.RiskWarn, th.RiskCrit, th.ThermalWarn, th.ThermalCrit,
			th.BufferWarn, th.BufferCrit, time.Now().UnixMilli(), time.Now().UnixMilli(), id,
		)
		if err != nil {
			return err
		}

		// Verify the INSERT actually created a row.
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			return fmt.Errorf("operator %s not found", id)
		}
	}

	return nil
}

// GetThresholds retrieves the alert thresholds for a user.
func (r *UserRepository) GetThresholds(ctx context.Context, id string) (user.Thresholds, error) {
	row := r.queryRow(ctx,
		`SELECT risk_warn, risk_crit, thermal_warn, thermal_crit, buffer_warn, buffer_crit 
		 FROM user_settings WHERE user_id = ?`,
		id,
	)

	var th user.Thresholds
	err := row.Scan(&th.RiskWarn, &th.RiskCrit, &th.ThermalWarn, &th.ThermalCrit, &th.BufferWarn, &th.BufferCrit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return defaults if no settings exist.
			return user.Thresholds{
				RiskWarn:    70,
				RiskCrit:    85,
				ThermalWarn: 45,
				ThermalCrit: 50,
				BufferWarn:  30,
				BufferCrit:  15,
			}, nil
		}
		return user.Thresholds{}, err
	}

	return th, nil
}

// UpdateClientSettings updates the client preferences for a user.
func (r *UserRepository) UpdateClientSettings(ctx context.Context, id string, cs user.ClientSettings) error {
	result, err := r.exec(ctx,
		`UPDATE user_settings SET 
		 server_url = ?, device_id = ?, request_timeout_ms = ?, auto_reconnect = ?, 
		 strict_hmac = ?, log_buffer_limit = ?, signal_history_limit = ?, 
		 notifications_enabled = ?, updated_at = ? WHERE user_id = ?`,
		cs.ServerURL, cs.DeviceID, cs.RequestTimeoutMs, cs.AutoReconnect,
		cs.StrictHmac, cs.LogBufferLimit, cs.SignalHistoryLimit,
		cs.NotificationsEnabled, time.Now().UnixMilli(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		// Settings row doesn't exist - insert with client settings.
		result, err = r.exec(ctx,
			`INSERT INTO user_settings (user_id, server_url, device_id, request_timeout_ms, 
			 auto_reconnect, strict_hmac, log_buffer_limit, signal_history_limit, 
			 notifications_enabled, updated_at, created_at)
			 SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)`,
			id, cs.ServerURL, cs.DeviceID, cs.RequestTimeoutMs,
			cs.AutoReconnect, cs.StrictHmac, cs.LogBufferLimit, cs.SignalHistoryLimit,
			cs.NotificationsEnabled, time.Now().UnixMilli(), time.Now().UnixMilli(), id,
		)
		if err != nil {
			return err
		}

		// Verify the INSERT actually created a row.
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			return fmt.Errorf("operator %s not found", id)
		}
	}

	return nil
}

func (r *UserRepository) GetPreferencesRaw(ctx context.Context, userID string) (string, error) {
	var prefs sql.NullString
	err := r.queryRow(ctx, `SELECT preferences FROM user_settings WHERE user_id = ?`, userID).Scan(&prefs)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return prefs.String, nil
}

func (r *UserRepository) SavePreferencesRaw(ctx context.Context, userID string, prefsJSON string) error {
	_, err := r.exec(ctx,
		`INSERT INTO user_settings (user_id, preferences, risk_warn, risk_crit, thermal_warn, thermal_crit,
		                                 buffer_warn, buffer_crit, updated_at, created_at)
		 VALUES (?, ?, 70, 85, 45, 50, 30, 15, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET preferences = excluded.preferences,
		                                     updated_at = excluded.updated_at`,
		userID, prefsJSON, time.Now().UnixMilli(), time.Now().UnixMilli())
	return err
}

// ResetSettings resets all settings to defaults for a user.
func (r *UserRepository) ResetSettings(ctx context.Context, id string) error {
	result, err := r.exec(ctx,
		`UPDATE user_settings SET 
		 risk_warn = 70, risk_crit = 85, thermal_warn = 45, thermal_crit = 50, 
		 buffer_warn = 30, buffer_crit = 15,
		 strict_hmac = 0, auto_reconnect = 1, notifications_enabled = 1,
		 server_url = '', device_id = '', request_timeout_ms = 8000, 
		 log_buffer_limit = 500, signal_history_limit = 240,
		 updated_at = ? WHERE user_id = ?`,
		time.Now().UnixMilli(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		// Settings row doesn't exist - insert defaults for this user.
		result, err = r.exec(ctx,
			`INSERT INTO user_settings (user_id, risk_warn, risk_crit, thermal_warn, thermal_crit, 
			 buffer_warn, buffer_crit, strict_hmac, auto_reconnect, notifications_enabled,
			 server_url, device_id, request_timeout_ms, log_buffer_limit, signal_history_limit,
			 updated_at, created_at)
			 SELECT ?, 70, 85, 45, 50, 30, 15, 0, 1, 1, '', '', 8000, 500, 240, ?, ?
			 WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)`,
			id, time.Now().UnixMilli(), time.Now().UnixMilli(), id,
		)
		if err != nil {
			return err
		}

		// Verify the INSERT actually created a row.
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			return fmt.Errorf("operator %s not found", id)
		}
	}

	return nil
}

// GetEmailVerified returns whether a user has verified their email.
func (r *UserRepository) GetEmailVerified(ctx context.Context, id string) (bool, error) {
	var verified int

	err := r.queryRow(ctx,
		"SELECT email_verified FROM users WHERE id = ?",
		id,
	).Scan(&verified)
	if errors.Is(err, sql.ErrNoRows) {
		return false, user.ErrNotFound
	}

	if err != nil {
		return false, err
	}

	return verified != 0, nil
}

// DisableMFA disables MFA for a user by clearing the MFA secret and backup codes.
func (r *UserRepository) DisableMFA(ctx context.Context, id string) error {
	result, err := r.exec(ctx,
		"UPDATE users SET mfa_secret = '', mfa_secret_mac = '', mfa_enabled = 0, mfa_backup_codes = '', updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// GetSetting retrieves a setting value by key.
func (r *UserRepository) GetSetting(ctx context.Context, key string) (string, error) {
	var value string

	err := r.queryRow(ctx,
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}

	return value, err
}

// SetSetting updates or inserts a setting value.
func (r *UserRepository) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.exec(ctx,
		`INSERT OR REPLACE INTO settings(key, value, updated_at) VALUES(?, ?, ?)`,
		key, value, time.Now().UTC().UnixMilli(),
	)

	return err
}

// GetEnforceHMAC returns whether HMAC enforcement is enabled.
func (r *UserRepository) GetEnforceHMAC(ctx context.Context) (bool, error) {
	val, err := r.GetSetting(ctx, "enforce_hmac")
	if err != nil || val == "" {
		return false, err
	}

	return val == "true" || val == "1", nil
}

// SetEnforceHMAC updates the HMAC enforcement setting.
func (r *UserRepository) SetEnforceHMAC(ctx context.Context, enforce bool) error {
	val := "false"
	if enforce {
		val = "true"
	}

	return r.SetSetting(ctx, "enforce_hmac", val)
}

// GetHMACWindowSeconds returns the HMAC timestamp window in seconds.
func (r *UserRepository) GetHMACWindowSeconds(ctx context.Context) (int, error) {
	val, err := r.GetSetting(ctx, "hmac_window_seconds")
	if err != nil {
		return 30, err
	}

	if val == "" {
		return 30, nil // default 30 seconds.
	}

	var seconds int

	_, err = fmt.Sscanf(val, "%d", &seconds)
	if err != nil {
		return 30, err
	}

	return seconds, nil
}

// SetHMACWindowSeconds updates the HMAC timestamp window.
func (r *UserRepository) SetHMACWindowSeconds(ctx context.Context, seconds int) error {
	return r.SetSetting(ctx, "hmac_window_seconds", strconv.Itoa(seconds))
}

// GetUserSettings retrieves all settings for a user from user_settings table.
// OPTIMIZATION: Fetches all fields in single query (was making 2 queries before).
func (r *UserRepository) GetUserSettings(ctx context.Context, userID string) (*user.UserSettings, error) {
	// Single query fetches ALL settings fields.
	query := `
		SELECT 
			-- Client settings
			server_url, device_id, request_timeout_ms, auto_reconnect, strict_hmac,
			log_buffer_limit, signal_history_limit, notifications_enabled,
			-- Thresholds
			risk_warn, risk_crit, thermal_warn, thermal_crit, buffer_warn, buffer_crit,
			-- Notification settings
			notify_email, notify_push, notify_webhook,
			webhook_url, webhook_secret, webhook_types,
			notify_threshold_breach, notify_device_offline, notify_device_online,
			notify_update_available, notify_command_failed, notify_registration_request,
			notify_email_address,
			slack_enabled, slack_team_id, slack_team_name, slack_user_id,
			slack_access_token, slack_channel_id, slack_channel_name,
			notify_slack
		FROM user_settings WHERE user_id = ?`

	var settings user.UserSettings
	var serverURL, deviceID, notifyEmail, webhookURL, webhookSecret, webhookTypes sql.NullString
	var requestTimeoutMs, logBufferLimit, signalHistoryLimit int
	var autoReconnect, strictHmac, notificationsEnabled int
	var riskWarn, riskCrit, thermalWarn, thermalCrit, bufferWarn, bufferCrit int
	var notifyPush, notifyWebhook int
	var notifyThresholdBreach, notifyDeviceOffline, notifyDeviceOnline int
	var notifyUpdateAvailable, notifyCommandFailed, notifyRegistrationRequest int
	var notifyEmailAddress, slackTeamID, slackTeamName, slackUserID, slackAccessToken, slackChannelID, slackChannelName sql.NullString
	var slackEnabled, notifySlack int

	err := r.queryRow(ctx, query, userID).Scan(
		// Client.
		&serverURL, &deviceID, &requestTimeoutMs, &autoReconnect, &strictHmac,
		&logBufferLimit, &signalHistoryLimit, &notificationsEnabled,
		// Thresholds.
		&riskWarn, &riskCrit, &thermalWarn, &thermalCrit, &bufferWarn, &bufferCrit,
		// Notifications.
		&notifyEmail, &notifyPush, &notifyWebhook,
		&webhookURL, &webhookSecret, &webhookTypes,
		&notifyThresholdBreach, &notifyDeviceOffline, &notifyDeviceOnline,
		&notifyUpdateAvailable, &notifyCommandFailed, &notifyRegistrationRequest,
		&notifyEmailAddress,
		&slackEnabled, &slackTeamID, &slackTeamName, &slackUserID,
		&slackAccessToken, &slackChannelID, &slackChannelName,
		&notifySlack,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Return defaults if no settings exist.
		return &user.UserSettings{
			Client:        *user.DefaultClientSettings(),
			Thresholds:    user.Thresholds{},
			Notifications: user.DefaultNotificationSettings(),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	// Build client settings.
	settings.Client = user.ClientSettings{
		ServerURL:            serverURL.String,
		DeviceID:             deviceID.String,
		RequestTimeoutMs:     requestTimeoutMs,
		AutoReconnect:        autoReconnect == 1,
		StrictHmac:           strictHmac == 1,
		LogBufferLimit:       logBufferLimit,
		SignalHistoryLimit:   signalHistoryLimit,
		NotificationsEnabled: notificationsEnabled == 1,
	}

	// Build thresholds.
	settings.Thresholds = user.Thresholds{
		RiskWarn:    riskWarn,
		RiskCrit:    riskCrit,
		ThermalWarn: thermalWarn,
		ThermalCrit: thermalCrit,
		BufferWarn:  bufferWarn,
		BufferCrit:  bufferCrit,
	}

	// Build notifications (extracted from same row).
	ns := &user.NotificationSettings{
		Enabled:      notificationsEnabled == 1,
		Channels:     []string{},
		EmailAddress: notifyEmailAddress.String,
		Slack: user.SlackNotifications{
			Connected:   slackEnabled == 1,
			TeamID:      slackTeamID.String,
			TeamName:    slackTeamName.String,
			SlackUserID: slackUserID.String,
			ChannelID:   slackChannelID.String,
			ChannelName: slackChannelName.String,
			AccessToken: r.decryptSecret(slackAccessToken.String),
		},
	}

	// Parse channels.
	if notifyEmail.Valid && notifyEmail.String != "" {
		ns.Channels = append(ns.Channels, "email")
	}
	if notifyPush == 1 {
		ns.Channels = append(ns.Channels, "push")
	}
	if notifyWebhook == 1 {
		ns.Channels = append(ns.Channels, "webhook")
	}
	if notifySlack == 1 {
		ns.Channels = append(ns.Channels, "slack")
	}

	// Email settings.
	ns.Email = user.EmailNotifications{
		ThresholdBreach:     notifyThresholdBreach == 1,
		DeviceOffline:       notifyDeviceOffline == 1,
		DeviceOnline:        notifyDeviceOnline == 1,
		UpdateAvailable:     notifyUpdateAvailable == 1,
		CommandFailed:       notifyCommandFailed == 1,
		RegistrationRequest: notifyRegistrationRequest == 1,
	}

	// Push settings (same as email in this implementation).
	ns.Push = user.PushNotifications{
		ThresholdBreach:     notifyThresholdBreach == 1,
		DeviceOffline:       notifyDeviceOffline == 1,
		DeviceOnline:        notifyDeviceOnline == 1,
		UpdateAvailable:     notifyUpdateAvailable == 1,
		CommandFailed:       notifyCommandFailed == 1,
		RegistrationRequest: notifyRegistrationRequest == 1,
	}

	// Webhook settings (SECURITY: masked secret).
	ns.Webhook = user.WebhookNotifications{
		Enabled: notifyWebhook == 1,
	}
	if webhookURL.Valid {
		ns.Webhook.URL = webhookURL.String
	}
	// SECURITY: Return masked placeholder, not actual secret.
	if webhookSecret.Valid && webhookSecret.String != "" {
		ns.Webhook.Secret = "••••••••"
	}
	if webhookTypes.Valid && webhookTypes.String != "" {
		_ = json.Unmarshal([]byte(webhookTypes.String), &ns.Webhook.Types)
	} else {
		ns.Webhook.Types = []string{}
	}

	settings.Notifications = ns
	return &settings, nil
}

// GetNotifications retrieves notification settings for a user.
// Note: For full settings, use GetUserSettings which fetches all fields in single query.
// SECURITY: Returns masked secret placeholder, not actual secret.
func (r *UserRepository) GetNotifications(ctx context.Context, userID string) (*user.NotificationSettings, error) {
	query := `
		SELECT notifications_enabled, notify_email, notify_push, notify_webhook,
		       webhook_url, webhook_secret, webhook_types,
		       notify_threshold_breach, notify_device_offline, notify_device_online,
		       notify_update_available, notify_command_failed, notify_registration_request,
		       notify_email_address,
		       slack_enabled, slack_team_id, slack_team_name, slack_user_id,
		       slack_access_token, slack_channel_id, slack_channel_name,
		       notify_slack
		FROM user_settings WHERE user_id = ?`

	var ns user.NotificationSettings
	var notifyEmail sql.NullString
	var notifyPush, notifyWebhook int
	var webhookURL, webhookSecret, webhookTypes sql.NullString
	var notifyThresholdBreach, notifyDeviceOffline, notifyDeviceOnline int
	var notifyUpdateAvailable, notifyCommandFailed, notifyRegistrationRequest int
	var notifyEmailAddress, slackTeamID, slackTeamName, slackUserID, slackAccessToken, slackChannelID, slackChannelName sql.NullString
	var slackEnabled, notifySlack int

	err := r.queryRow(ctx, query, userID).Scan(
		&ns.Enabled, &notifyEmail, &notifyPush, &notifyWebhook,
		&webhookURL, &webhookSecret, &webhookTypes,
		&notifyThresholdBreach, &notifyDeviceOffline, &notifyDeviceOnline,
		&notifyUpdateAvailable, &notifyCommandFailed, &notifyRegistrationRequest,
		&notifyEmailAddress,
		&slackEnabled, &slackTeamID, &slackTeamName, &slackUserID,
		&slackAccessToken, &slackChannelID, &slackChannelName,
		&notifySlack,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Return defaults if no settings exist.
		return user.DefaultNotificationSettings(), nil
	}
	if err != nil {
		return nil, err
	}

	// Parse channels.
	ns.Channels = []string{}
	if notifyEmail.Valid && notifyEmail.String != "" {
		ns.Channels = append(ns.Channels, "email")
	}
	if notifyPush == 1 {
		ns.Channels = append(ns.Channels, "push")
	}
	if notifyWebhook == 1 {
		ns.Channels = append(ns.Channels, "webhook")
	}
	if notifySlack == 1 {
		ns.Channels = append(ns.Channels, "slack")
	}

	// Parse email settings from notify_email JSON or use defaults.
	ns.Email = user.EmailNotifications{
		ThresholdBreach:     notifyThresholdBreach == 1,
		DeviceOffline:       notifyDeviceOffline == 1,
		DeviceOnline:        notifyDeviceOnline == 1,
		UpdateAvailable:     notifyUpdateAvailable == 1,
		CommandFailed:       notifyCommandFailed == 1,
		RegistrationRequest: notifyRegistrationRequest == 1,
	}

	// Parse push settings - same as email in this implementation.
	ns.Push = user.PushNotifications{
		ThresholdBreach:     notifyThresholdBreach == 1,
		DeviceOffline:       notifyDeviceOffline == 1,
		DeviceOnline:        notifyDeviceOnline == 1,
		UpdateAvailable:     notifyUpdateAvailable == 1,
		CommandFailed:       notifyCommandFailed == 1,
		RegistrationRequest: notifyRegistrationRequest == 1,
	}

	ns.EmailAddress = notifyEmailAddress.String
	ns.Slack = user.SlackNotifications{
		Connected:   slackEnabled == 1,
		TeamID:      slackTeamID.String,
		TeamName:    slackTeamName.String,
		SlackUserID: slackUserID.String,
		ChannelID:   slackChannelID.String,
		ChannelName: slackChannelName.String,
		AccessToken: r.decryptSecret(slackAccessToken.String),
	}

	// Parse webhook settings.
	// SECURITY: Never return the raw webhook secret - only indicate if it exists.
	ns.Webhook = user.WebhookNotifications{
		Enabled: notifyWebhook == 1,
	}
	if webhookURL.Valid {
		ns.Webhook.URL = webhookURL.String
	}
	// Don't expose secret in GET responses - only return placeholder if set.
	if webhookSecret.Valid && webhookSecret.String != "" {
		ns.Webhook.Secret = "••••••••"
	} else {
		ns.Webhook.Secret = ""
	}
	if webhookTypes.Valid && webhookTypes.String != "" {
		_ = json.Unmarshal([]byte(webhookTypes.String), &ns.Webhook.Types)
	} else {
		ns.Webhook.Types = []string{}
	}

	return &ns, nil
}

// UpdateNotifications updates notification settings for a user.
func (r *UserRepository) UpdateNotifications(ctx context.Context, userID string, settings *user.NotificationSettings) error {
	if err := r.validateNotificationSettings(settings); err != nil {
		return err
	}

	notifyEmail := r.determineNotifyEmail(settings.Channels)
	notifyPush, notifyWebhook, notifySlack := r.determineChannelFlags(settings.Channels)
	webhookTypesJSON := r.serializeWebhookTypes(settings.Webhook.Types)
	hashedSecret, err := r.hashWebhookSecret(ctx, userID, settings.Webhook.Secret)
	if err != nil {
		return err
	}

	query := `
		UPDATE user_settings SET
			notifications_enabled = ?,
			notify_email = ?,
			notify_push = ?,
			notify_webhook = ?,
			notify_slack = ?,
			webhook_url = ?,
			webhook_secret = ?,
			webhook_types = ?,
			notify_threshold_breach = ?,
			notify_device_offline = ?,
			notify_device_online = ?,
			notify_update_available = ?,
			notify_command_failed = ?,
			notify_registration_request = ?,
			notify_email_address = ?,
			slack_enabled = ?,
			slack_team_id = ?,
			slack_team_name = ?,
			slack_user_id = ?,
			slack_access_token = ?,
			slack_channel_id = ?,
			slack_channel_name = ?,
			updated_at = ?
		WHERE user_id = ?`

	result, err := r.exec(ctx, query,
		settings.Enabled, notifyEmail, notifyPush, notifyWebhook, notifySlack,
		settings.Webhook.URL, hashedSecret, string(webhookTypesJSON),
		settings.Email.ThresholdBreach, settings.Email.DeviceOffline, settings.Email.DeviceOnline,
		settings.Email.UpdateAvailable, settings.Email.CommandFailed, settings.Email.RegistrationRequest,
		settings.EmailAddress,
		slackEnabledFlag(settings), settings.Slack.TeamID, settings.Slack.TeamName, settings.Slack.SlackUserID,
		r.encryptSecret(settings.Slack.AccessToken), settings.Slack.ChannelID, settings.Slack.ChannelName,
		time.Now().UnixMilli(), userID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		// Settings row doesn't exist - insert with the notification settings.
		result, err = r.exec(ctx,
			`INSERT INTO user_settings (user_id, notifications_enabled, notify_email, notify_push, notify_webhook,
			 notify_slack,
			 webhook_url, webhook_secret, webhook_types,
			 notify_threshold_breach, notify_device_offline, notify_device_online,
			 notify_update_available, notify_command_failed, notify_registration_request,
			 notify_email_address,
			 slack_enabled, slack_team_id, slack_team_name, slack_user_id,
			 slack_access_token, slack_channel_id, slack_channel_name,
			 updated_at, created_at)
			 SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
			 WHERE EXISTS (SELECT 1 FROM users WHERE id = ?)`,
			userID, settings.Enabled, notifyEmail, notifyPush, notifyWebhook, notifySlack,
			settings.Webhook.URL, hashedSecret, string(webhookTypesJSON),
			settings.Email.ThresholdBreach, settings.Email.DeviceOffline, settings.Email.DeviceOnline,
			settings.Email.UpdateAvailable, settings.Email.CommandFailed, settings.Email.RegistrationRequest,
			settings.EmailAddress,
			slackEnabledFlag(settings), settings.Slack.TeamID, settings.Slack.TeamName, settings.Slack.SlackUserID,
			r.encryptSecret(settings.Slack.AccessToken), settings.Slack.ChannelID, settings.Slack.ChannelName,
			time.Now().UnixMilli(), time.Now().UnixMilli(), userID,
		)
		if err != nil {
			return err
		}

		// Verify the INSERT actually created a row.
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted == 0 {
			return fmt.Errorf("operator %s not found", userID)
		}
	}

	return nil
}

// RotateWebhookSecret generates a new webhook secret for a user.
// SECURITY: Returns plaintext secret only on rotation (one-time view), stores hashed.
func (r *UserRepository) RotateWebhookSecret(ctx context.Context, userID string) (string, error) {
	// Generate a new secret.
	plaintextSecret := generateWebhookSecret()

	// Hash the secret before storing.
	hashedSecret, err := password.HashSecret(plaintextSecret)
	if err != nil {
		return "", fmt.Errorf("failed to hash webhook secret: %w", err)
	}

	query := `UPDATE user_settings SET webhook_secret = ?, updated_at = ? WHERE user_id = ?`
	result, err := r.exec(ctx, query, hashedSecret, time.Now().UnixMilli(), userID)
	if err != nil {
		return "", err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	if rows == 0 {
		return "", fmt.Errorf("operator settings not found for operator %s", userID)
	}

	// Return plaintext secret - this is the ONLY time it's exposed to the user.
	return plaintextSecret, nil
}

// UpdateFCMToken updates the FCM token for a user.
func (r *UserRepository) UpdateFCMToken(ctx context.Context, userID, fcmToken string) error {
	query := `UPDATE users SET fcm_token = ?, updated_at = ? WHERE id = ?`

	result, err := r.exec(ctx, query, nullString(fcmToken), time.Now(), userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// UpdateInstanceRole sets the user's deployment-wide role. It is a dedicated
// statement rather than a field on Update so a profile write cannot reset the
// role, and so the write is auditable on its own.
func (r *UserRepository) UpdateInstanceRole(ctx context.Context, userID string, role permission.InstanceRole) error {
	query := `UPDATE users SET instance_role = ?, updated_at = ? WHERE id = ?`

	result, err := r.exec(ctx, query, instanceRoleOrDefault(role), time.Now().UnixMilli(), userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return user.ErrNotFound
	}

	return nil
}

// validateNotificationSettings validates notification settings.
func (r *UserRepository) validateNotificationSettings(settings *user.NotificationSettings) error {
	if settings.Webhook.Enabled && settings.Webhook.URL == "" {
		return fmt.Errorf("webhook URL is required when webhook is enabled")
	}

	if settings.Webhook.URL != "" {
		parsedURL, err := url.Parse(settings.Webhook.URL)
		if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			return fmt.Errorf("invalid webhook URL: must be a valid HTTP/HTTPS URL")
		}
		if parsedURL.Host == "" {
			return fmt.Errorf("invalid webhook URL: missing host")
		}
	}
	return nil
}

// GetNotificationsReadAt returns the user's notification read watermark in Unix
// milliseconds, or 0 when they have never marked anything read.
func (r *UserRepository) GetNotificationsReadAt(ctx context.Context, userID string) (int64, error) {
	var readAt sql.NullInt64
	err := r.queryRow(ctx, `SELECT notifications_read_at FROM user_settings WHERE user_id = ?`, userID).Scan(&readAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !readAt.Valid {
		return 0, nil
	}
	return readAt.Int64, nil
}

// SetNotificationsReadAt stamps the read watermark. A user with no settings row
// yet is a no-op: there is nothing to mark read.
func (r *UserRepository) SetNotificationsReadAt(ctx context.Context, userID string, at int64) error {
	_, err := r.exec(ctx, `UPDATE user_settings SET notifications_read_at = ? WHERE user_id = ?`, at, userID)
	return err
}

// slackEnabledFlag returns the Slack channel flag as an int for the SQLite column.
func slackEnabledFlag(settings *user.NotificationSettings) int {
	if settings.Slack.Connected {
		return 1
	}
	return 0
}

// determineNotifyEmail determines if email notifications are enabled.
func (r *UserRepository) determineNotifyEmail(channels []string) string {
	for _, ch := range channels {
		if ch == "email" {
			return "true"
		}
	}
	return ""
}

// determineChannelFlags determines push and webhook enabled flags from channels.
func (r *UserRepository) determineChannelFlags(channels []string) (notifyPush, notifyWebhook, notifySlack int) {
	for _, ch := range channels {
		switch ch {
		case "push":
			notifyPush = 1
		case "webhook":
			notifyWebhook = 1
		case "slack":
			notifySlack = 1
		}
	}
	return
}

// serializeWebhookTypes serializes webhook types to JSON.
func (r *UserRepository) serializeWebhookTypes(types []string) []byte {
	webhookTypesJSON, err := json.Marshal(types)
	if err != nil {
		return []byte("[]")
	}
	return webhookTypesJSON
}

// hashWebhookSecret hashes the webhook secret before storing.
func (r *UserRepository) hashWebhookSecret(ctx context.Context, userID, secret string) (string, error) {
	if secret != "" && secret != "••••••••" {
		hashed, err := password.HashSecret(secret)
		if err != nil {
			return "", fmt.Errorf("failed to hash webhook secret: %w", err)
		}
		return hashed, nil
	}

	if secret == "" {
		return "", nil
	}

	// Masked placeholder - keep existing secret.
	existing, err := r.GetNotifications(ctx, userID)
	if err == nil && existing != nil {
		if existing.Webhook.Secret != "••••••••" {
			return existing.Webhook.Secret, nil
		}
	}
	return "", nil
}

// generateWebhookSecret generates a secure random secret for webhooks.
func generateWebhookSecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// nullString returns a sql.NullString for optional string fields.
// instanceRoleOrDefault maps an empty InstanceRole to the stored 'none' value so
// the column is never NULL and every read yields a known role.
func instanceRoleOrDefault(r permission.InstanceRole) string {
	if r == "" {
		return string(permission.InstanceNone)
	}
	return string(r)
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}

	return sql.NullString{String: s, Valid: true}
}

// nullInt returns a sql.NullInt64 for an optional timestamp pointer, so a nil
// stores as SQL NULL rather than as a zero that reads back as the epoch.
func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: *p, Valid: true}
}
