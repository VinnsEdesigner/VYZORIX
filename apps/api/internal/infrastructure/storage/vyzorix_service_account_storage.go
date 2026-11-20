package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/serviceaccount"
)

// ServiceAccountRepository is the SQL persistence for service accounts.
type ServiceAccountRepository struct {
	db *sql.DB
}

// NewServiceAccountRepository creates a new ServiceAccountRepository.
func NewServiceAccountRepository(db *sql.DB) *ServiceAccountRepository {
	return &ServiceAccountRepository{db: db}
}

// Save upserts a service account.
func (r *ServiceAccountRepository) Save(ctx context.Context, sa *serviceaccount.ServiceAccount) error {
	enabled := 0
	if sa.Enabled {
		enabled = 1
	}
	query := `
		INSERT INTO service_accounts (id, org_id, name, enabled, created_at, updated_at,
			disabled_at, disabled_by, enabled_by, disabled_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at,
			disabled_at = excluded.disabled_at,
			disabled_by = excluded.disabled_by,
			enabled_by = excluded.enabled_by,
			disabled_reason = excluded.disabled_reason
	`
	_, err := r.db.ExecContext(ctx, query,
		sa.ID, sa.OrgID, sa.Name, enabled,
		sa.CreatedAt.UnixMilli(), sa.UpdatedAt.UnixMilli(),
		nullTimeMillis(sa.DisabledAt), nullIfEmpty(sa.DisabledBy),
		nullIfEmpty(sa.EnabledBy), nullIfEmpty(sa.DisabledReason),
	)
	return err
}

// nullTimeMillis renders an optional timestamp as Unix millis, or nil for NULL.
func nullTimeMillis(t *time.Time) interface{} {
	if t == nil {
		return nil
	}

	return t.UnixMilli()
}

// nullIfEmpty stores an absent string as SQL NULL, so "never set" and "" are the
// same thing on the wire rather than two spellings of it.
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}

	return s
}

const serviceAccountColumns = `id, org_id, name, enabled, created_at, updated_at,
	disabled_at, disabled_by, enabled_by, disabled_reason`

func scanServiceAccount(scanner interface{ Scan(...any) error }) (*serviceaccount.ServiceAccount, error) {
	var sa serviceaccount.ServiceAccount
	var enabled int
	var createdAt, updatedAt int64
	var tokenCount int
	var disabledAt sql.NullInt64
	var disabledBy, enabledBy, disabledReason sql.NullString
	err := scanner.Scan(
		&sa.ID, &sa.OrgID, &sa.Name, &enabled,
		&createdAt, &updatedAt,
		&disabledAt, &disabledBy, &enabledBy, &disabledReason,
		&tokenCount,
	)
	if err != nil {
		return nil, err
	}
	sa.Enabled = enabled == 1
	sa.CreatedAt = time.UnixMilli(createdAt)
	sa.UpdatedAt = time.UnixMilli(updatedAt)
	sa.DisabledBy = disabledBy.String
	sa.EnabledBy = enabledBy.String
	sa.DisabledReason = disabledReason.String
	if disabledAt.Valid {
		t := time.UnixMilli(disabledAt.Int64)
		sa.DisabledAt = &t
	}
	sa.TokenCount = tokenCount
	return &sa, nil
}

// selectServiceAccountWithCount returns the account columns plus its live token
// count. The count is computed on read (a correlated subquery) rather than
// stored, so it can never drift from service_account_tokens.
const selectServiceAccountWithCount = `
	SELECT ` + serviceAccountColumns + `,
	       (SELECT COUNT(*) FROM service_account_tokens t
	        WHERE t.service_id = service_accounts.id AND t.valid = 1) AS token_count
	FROM service_accounts`

// GetByID returns a service account or serviceaccount.ErrNotFound.
func (r *ServiceAccountRepository) GetByID(ctx context.Context, id string) (*serviceaccount.ServiceAccount, error) {
	sa, err := scanServiceAccount(r.db.QueryRowContext(ctx,
		selectServiceAccountWithCount+` WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, serviceaccount.ErrNotFound
	}
	return sa, err
}

// ListByOrg returns all service accounts of an org, each with its token count.
func (r *ServiceAccountRepository) ListByOrg(ctx context.Context, orgID string) ([]*serviceaccount.ServiceAccount, error) {
	rows, err := r.db.QueryContext(ctx,
		selectServiceAccountWithCount+` WHERE org_id = ? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sas []*serviceaccount.ServiceAccount
	for rows.Next() {
		sa, err := scanServiceAccount(rows)
		if err != nil {
			return nil, err
		}
		sas = append(sas, sa)
	}
	return sas, rows.Err()
}

// ListAllOrgs returns the distinct org IDs with at least one service account.
func (r *ServiceAccountRepository) ListAllOrgs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT org_id FROM service_accounts`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var orgIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		orgIDs = append(orgIDs, id)
	}
	return orgIDs, rows.Err()
}

// Delete removes a service account.
func (r *ServiceAccountRepository) Delete(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM service_accounts WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ServiceAccountTokenRepository is the SQL persistence for service account tokens.
type ServiceAccountTokenRepository struct {
	db *sql.DB
}

// NewServiceAccountTokenRepository creates a new ServiceAccountTokenRepository.
func NewServiceAccountTokenRepository(db *sql.DB) *ServiceAccountTokenRepository {
	return &ServiceAccountTokenRepository{db: db}
}

const serviceTokenColumns = `id, service_id, name, key_hash, key_prefix, scopes, valid, expires_at, revoked_at, last_used_at, created_at, basic_role, roles`

func scanServiceToken(scanner interface{ Scan(...any) error }) (*serviceaccount.Token, error) {
	var token serviceaccount.Token
	var scopes string
	var valid int
	var createdAt int64
	var expiresAt, revokedAt, lastUsedAt sql.NullInt64
	var basicRole, rolesJSON sql.NullString
	err := scanner.Scan(
		&token.ID, &token.ServiceID, &token.Name, &token.KeyHash, &token.KeyPrefix,
		&scopes, &valid, &expiresAt, &revokedAt, &lastUsedAt, &createdAt,
		&basicRole, &rolesJSON,
	)
	if err != nil {
		return nil, err
	}
	token.Valid = valid == 1
	token.CreatedAt = time.UnixMilli(createdAt)
	if expiresAt.Valid {
		t := time.UnixMilli(expiresAt.Int64)
		token.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := time.UnixMilli(revokedAt.Int64)
		token.RevokedAt = &t
	}
	if lastUsedAt.Valid {
		t := time.UnixMilli(lastUsedAt.Int64)
		token.LastUsedAt = &t
	}
	if scopes != "" {
		if err := json.Unmarshal([]byte(scopes), &token.Scopes); err != nil {
			return nil, err
		}
	}
	if token.Scopes == nil {
		token.Scopes = []string{}
	}
	token.BasicRole = basicRole.String
	if rolesJSON.Valid && rolesJSON.String != "" {
		if err := json.Unmarshal([]byte(rolesJSON.String), &token.Roles); err != nil {
			return nil, err
		}
	}
	if token.Roles == nil {
		token.Roles = []permission.FixedRoleGrant{}
	}
	return &token, nil
}

// Save upserts a token.
func (r *ServiceAccountTokenRepository) Save(ctx context.Context, token *serviceaccount.Token) error {
	scopesJSON, err := json.Marshal(token.Scopes)
	if err != nil {
		return err
	}
	valid := 0
	if token.Valid {
		valid = 1
	}
	var expiresAt, revokedAt, lastUsedAt interface{}
	if token.ExpiresAt != nil {
		expiresAt = token.ExpiresAt.UnixMilli()
	}
	if token.RevokedAt != nil {
		revokedAt = token.RevokedAt.UnixMilli()
	}
	if token.LastUsedAt != nil {
		lastUsedAt = token.LastUsedAt.UnixMilli()
	}
	rolesJSON, err := json.Marshal(token.Roles)
	if err != nil {
		return err
	}
	var basicRole interface{}
	if token.BasicRole != "" {
		basicRole = token.BasicRole
	}
	query := `
		INSERT INTO service_account_tokens (id, service_id, name, key_hash, key_prefix, scopes, valid, expires_at, revoked_at, last_used_at, created_at, basic_role, roles)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			scopes = excluded.scopes,
			valid = excluded.valid,
			expires_at = excluded.expires_at,
			revoked_at = excluded.revoked_at,
			last_used_at = excluded.last_used_at,
			basic_role = excluded.basic_role,
			roles = excluded.roles
	`
	_, err = r.db.ExecContext(ctx, query,
		token.ID, token.ServiceID, token.Name, token.KeyHash, token.KeyPrefix,
		string(scopesJSON), valid, expiresAt, revokedAt, lastUsedAt, token.CreatedAt.UnixMilli(),
		basicRole, string(rolesJSON),
	)
	return err
}

// GetByID returns a token or serviceaccount.ErrNotFound.
func (r *ServiceAccountTokenRepository) GetByID(ctx context.Context, id string) (*serviceaccount.Token, error) {
	token, err := scanServiceToken(r.db.QueryRowContext(ctx,
		`SELECT `+serviceTokenColumns+` FROM service_account_tokens WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, serviceaccount.ErrNotFound
	}
	return token, err
}

// GetByKeyHash returns a token by its argon2 hash.
func (r *ServiceAccountTokenRepository) GetByKeyHash(ctx context.Context, keyHash string) (*serviceaccount.Token, error) {
	token, err := scanServiceToken(r.db.QueryRowContext(ctx,
		`SELECT `+serviceTokenColumns+` FROM service_account_tokens WHERE key_hash = ?`, keyHash))
	if err == sql.ErrNoRows {
		return nil, serviceaccount.ErrNotFound
	}
	return token, err
}

// GetByPrefix returns a token by its display prefix.
func (r *ServiceAccountTokenRepository) GetByPrefix(ctx context.Context, prefix string) (*serviceaccount.Token, error) {
	token, err := scanServiceToken(r.db.QueryRowContext(ctx,
		`SELECT `+serviceTokenColumns+` FROM service_account_tokens WHERE key_prefix = ?`, prefix))
	if err == sql.ErrNoRows {
		return nil, serviceaccount.ErrNotFound
	}
	return token, err
}

// ListByService returns all tokens of a service account.
func (r *ServiceAccountTokenRepository) ListByService(ctx context.Context, serviceID string) ([]*serviceaccount.Token, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+serviceTokenColumns+` FROM service_account_tokens WHERE service_id = ? ORDER BY created_at DESC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tokens []*serviceaccount.Token
	for rows.Next() {
		token, err := scanServiceToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

// ListExpired returns tokens past their expiry that are still usable.
func (r *ServiceAccountTokenRepository) ListExpired(ctx context.Context, now time.Time) ([]*serviceaccount.Token, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+serviceTokenColumns+` FROM service_account_tokens
		 WHERE valid = 1 AND expires_at IS NOT NULL AND expires_at < ?`,
		now.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tokens []*serviceaccount.Token
	for rows.Next() {
		token, err := scanServiceToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

// Revoke sets the revoked_at timestamp.
func (r *ServiceAccountTokenRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE service_account_tokens SET revoked_at = ?, valid = 0 WHERE id = ?`,
		revokedAt.UnixMilli(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return serviceaccount.ErrNotFound
	}
	return nil
}

// TouchLastUsed records when the token last authenticated.
func (r *ServiceAccountTokenRepository) TouchLastUsed(ctx context.Context, id string, usedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE service_account_tokens SET last_used_at = ? WHERE id = ?`,
		usedAt.UnixMilli(), id)
	return err
}

// Delete removes a token.
func (r *ServiceAccountTokenRepository) Delete(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM service_account_tokens WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
