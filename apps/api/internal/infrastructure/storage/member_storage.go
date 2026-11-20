package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/transaction"
)

// Ensure MemberRepository implements organization.MemberRepository.
var _ organization.MemberRepository = (*MemberStorage)(nil)

// MemberStorage implements organization.MemberRepository using SQLite.
type MemberStorage struct {
	db *sql.DB
}

// NewMemberStorage creates a new MemberStorage.
func NewMemberStorage(db *sql.DB) *MemberStorage {
	return &MemberStorage{db: db}
}

// getQuerier returns the transaction from context if available, otherwise the db.
func (s *MemberStorage) getQuerier(ctx context.Context) Querier {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx
	}
	return s.db
}

// Create creates a new membership.
func (s *MemberStorage) Create(ctx context.Context, member *organization.OrganizationMember) error {
	query := `
		INSERT INTO organization_members (id, organization_id, user_id, role, invited_by, joined_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := s.getQuerier(ctx).ExecContext(ctx, query,
		member.ID,
		member.OrganizationID,
		member.UserID,
		string(member.Role),
		member.InvitedBy,
		member.JoinedAt.UnixMilli(),
		string(member.Lifecycle),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return organization.ErrMemberExists
		}
		return err
	}

	return nil
}

// memberScan holds the raw columns of a membership row, so the single-row and
// multi-row readers share one column list and one lifecycle mapping.
type memberScan struct {
	member      organization.OrganizationMember
	status      string
	invitedBy   sql.NullString
	userName    sql.NullString
	userEmail   sql.NullString
	avatarURL   sql.NullString
	removedAt   sql.NullInt64
	suspendedAt sql.NullInt64
	lastLoginAt sql.NullInt64
	joinedAtMs  int64
}

func (s *memberScan) targets() []any {
	return []any{
		&s.member.ID,
		&s.member.OrganizationID,
		&s.member.UserID,
		&s.member.Role,
		&s.invitedBy,
		&s.joinedAtMs,
		&s.removedAt,
		&s.status,
		&s.suspendedAt,
		&s.userName,
		&s.userEmail,
		&s.avatarURL,
		&s.lastLoginAt,
	}
}

func (s *memberScan) result() *organization.OrganizationMember {
	m := &s.member
	m.JoinedAt = time.UnixMilli(s.joinedAtMs)
	m.Lifecycle = memberStatusToLifecycle(s.status)
	if s.invitedBy.Valid {
		m.InvitedBy = &s.invitedBy.String
	}
	if s.removedAt.Valid {
		t := time.UnixMilli(s.removedAt.Int64)
		m.RemovedAt = &t
	}
	if s.suspendedAt.Valid {
		t := time.UnixMilli(s.suspendedAt.Int64)
		m.SuspendedAt = &t
	}
	if s.lastLoginAt.Valid {
		t := time.UnixMilli(s.lastLoginAt.Int64)
		m.LastSeenAt = &t
	}
	m.UserName = s.userName.String
	m.UserEmail = s.userEmail.String
	m.AvatarURL = s.avatarURL.String

	return m
}

// scanMember scans a single membership row.
func scanMember(row *sql.Row) (*organization.OrganizationMember, error) {
	var sc memberScan
	err := row.Scan(sc.targets()...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, organization.ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}

	return sc.result(), nil
}

// FindByID retrieves a member by ID.
func (s *MemberStorage) FindByID(ctx context.Context, id string) (*organization.OrganizationMember, error) {
	query := `
		SELECT om.id, om.organization_id, om.user_id, om.role, om.invited_by, om.joined_at, om.removed_at, om.status,
			   om.suspended_at,
			   COALESCE(op.name, '') as user_name, COALESCE(op.email, '') as user_email,
			   COALESCE(op.avatar_url, '') as avatar_url, op.last_login_at
		FROM organization_members om
		LEFT JOIN users op ON om.user_id = op.id
		WHERE om.id = ?`

	return scanMember(s.getQuerier(ctx).QueryRowContext(ctx, query, id))
}

// FindByMemberID retrieves a member by MemberID value object.
func (s *MemberStorage) FindByMemberID(ctx context.Context, id organization.MemberID) (*organization.OrganizationMember, error) {
	return s.FindByID(ctx, id.String())
}

// FindByUserAndOrg finds a membership by user ID and organization ID.
func (s *MemberStorage) FindByUserAndOrg(ctx context.Context, userID, orgID string) (*organization.OrganizationMember, error) {
	query := `
		SELECT om.id, om.organization_id, om.user_id, om.role, om.invited_by, om.joined_at, om.removed_at, om.status,
			   om.suspended_at,
			   COALESCE(op.name, '') as user_name, COALESCE(op.email, '') as user_email,
			   COALESCE(op.avatar_url, '') as avatar_url, op.last_login_at
		FROM organization_members om
		LEFT JOIN users op ON om.user_id = op.id
		WHERE om.user_id = ? AND om.organization_id = ? AND om.status = 'active'`

	return scanMember(s.getQuerier(ctx).QueryRowContext(ctx, query, userID, orgID))
}

// FindByOrganization lists all members of an organization.
func (s *MemberStorage) FindByOrganization(ctx context.Context, orgID string) ([]*organization.OrganizationMember, error) {
	query := `
		SELECT om.id, om.organization_id, om.user_id, om.role, om.invited_by, om.joined_at, om.removed_at, om.status,
			   om.suspended_at,
			   COALESCE(op.name, '') as user_name, COALESCE(op.email, '') as user_email,
			   COALESCE(op.avatar_url, '') as avatar_url, op.last_login_at
		FROM organization_members om
		LEFT JOIN users op ON om.user_id = op.id
		WHERE om.organization_id = ? AND om.status = 'active'
		ORDER BY om.joined_at ASC`

	rows, err := s.getQuerier(ctx).QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []*organization.OrganizationMember
	for rows.Next() {
		var sc memberScan
		if err := rows.Scan(sc.targets()...); err != nil {
			return nil, err
		}
		members = append(members, sc.result())
	}

	return members, rows.Err()
}

// FindActiveByOrganizationPaginated lists active members with pagination.
func (s *MemberStorage) FindActiveByOrganizationPaginated(ctx context.Context, orgID string, limit, offset int) ([]*organization.OrganizationMember, int, error) {
	// Get total count.
	countQuery := `
		SELECT COUNT(*) FROM organization_members
		WHERE organization_id = ? AND status = 'active'`

	var total int
	err := s.getQuerier(ctx).QueryRowContext(ctx, countQuery, orgID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results.
	query := `
		SELECT om.id, om.organization_id, om.user_id, om.role, om.invited_by, om.joined_at, om.removed_at, om.status,
			   om.suspended_at,
			   COALESCE(op.name, '') as user_name, COALESCE(op.email, '') as user_email,
			   COALESCE(op.avatar_url, '') as avatar_url, op.last_login_at
		FROM organization_members om
		LEFT JOIN users op ON om.user_id = op.id
		WHERE om.organization_id = ? AND om.status = 'active'
		ORDER BY om.joined_at ASC
		LIMIT ? OFFSET ?`

	rows, err := s.getQuerier(ctx).QueryContext(ctx, query, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var members []*organization.OrganizationMember
	for rows.Next() {
		var sc memberScan
		if err := rows.Scan(sc.targets()...); err != nil {
			return nil, 0, err
		}
		members = append(members, sc.result())
	}

	return members, total, rows.Err()
}

// Update updates an existing membership.
func (s *MemberStorage) Update(ctx context.Context, member *organization.OrganizationMember) error {
	query := `
		UPDATE organization_members
		SET role = ?, status = ?, suspended_at = ?
		WHERE id = ?`

	result, err := s.getQuerier(ctx).ExecContext(ctx, query,
		string(member.Role),
		string(member.Lifecycle),
		timePtrMillis(member.SuspendedAt),
		member.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return organization.ErrMemberNotFound
	}

	return nil
}

// SoftDelete soft-deletes a membership (marks as removed).
func (s *MemberStorage) SoftDelete(ctx context.Context, id string) error {
	query := `
		UPDATE organization_members
		SET status = 'removed', removed_at = ?
		WHERE id = ? AND status = 'active'`

	now := time.Now().UnixMilli()
	result, err := s.getQuerier(ctx).ExecContext(ctx, query, now, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return organization.ErrMemberNotFound
	}

	return nil
}

// CountByOrganization counts members in an organization (including removed).
func (s *MemberStorage) CountByOrganization(ctx context.Context, orgID string) (int, error) {
	query := `SELECT COUNT(*) FROM organization_members WHERE organization_id = ?`

	var count int
	err := s.getQuerier(ctx).QueryRowContext(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountActiveByOrganization counts active (non-removed) members in an organization.
func (s *MemberStorage) CountActiveByOrganization(ctx context.Context, orgID string) (int, error) {
	query := `SELECT COUNT(*) FROM organization_members WHERE organization_id = ? AND status = 'active'`

	var count int
	err := s.getQuerier(ctx).QueryRowContext(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountSuperAdminsByOrganization counts super_admin members in an organization.
func (s *MemberStorage) CountSuperAdminsByOrganization(ctx context.Context, orgID string) (int, error) {
	query := `SELECT COUNT(*) FROM organization_members WHERE organization_id = ? AND role = 'super_admin' AND status = 'active'`

	var count int
	err := s.getQuerier(ctx).QueryRowContext(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// ListByUser lists all memberships for a user.
func (s *MemberStorage) ListByUser(ctx context.Context, userID string) ([]*organization.OrganizationMember, error) {
	query := `
		SELECT om.id, om.organization_id, om.user_id, om.role, om.invited_by, om.joined_at, om.removed_at, om.status,
			   om.suspended_at,
			   COALESCE(op.name, '') as user_name, COALESCE(op.email, '') as user_email,
			   COALESCE(op.avatar_url, '') as avatar_url, op.last_login_at
		FROM organization_members om
		LEFT JOIN users op ON om.user_id = op.id
		WHERE om.user_id = ? AND om.status = 'active'
		ORDER BY om.joined_at DESC`

	rows, err := s.getQuerier(ctx).QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var members []*organization.OrganizationMember
	for rows.Next() {
		var sc memberScan
		if err := rows.Scan(sc.targets()...); err != nil {
			return nil, err
		}
		members = append(members, sc.result())
	}

	return members, rows.Err()
}

// ListByUserPaginated lists memberships for a user with pagination.
func (s *MemberStorage) ListByUserPaginated(ctx context.Context, userID string, limit, offset int) ([]*organization.OrganizationMember, int, error) {
	// Get total count.
	countQuery := `
		SELECT COUNT(*) FROM organization_members
		WHERE user_id = ? AND status = 'active'`

	var total int
	err := s.getQuerier(ctx).QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results.
	query := `
		SELECT om.id, om.organization_id, om.user_id, om.role, om.invited_by, om.joined_at, om.removed_at, om.status,
			   om.suspended_at,
			   COALESCE(op.name, '') as user_name, COALESCE(op.email, '') as user_email,
			   COALESCE(op.avatar_url, '') as avatar_url, op.last_login_at
		FROM organization_members om
		LEFT JOIN users op ON om.user_id = op.id
		WHERE om.user_id = ? AND om.status = 'active'
		ORDER BY om.joined_at DESC
		LIMIT ? OFFSET ?`

	rows, err := s.getQuerier(ctx).QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var members []*organization.OrganizationMember
	for rows.Next() {
		var sc memberScan
		if err := rows.Scan(sc.targets()...); err != nil {
			return nil, 0, err
		}
		members = append(members, sc.result())
	}

	return members, total, rows.Err()
}

// SoftDeleteByUser soft-deletes all memberships for a user (used during user deletion).
func (s *MemberStorage) SoftDeleteByUser(ctx context.Context, userID string) error {
	query := `
		UPDATE organization_members
		SET status = 'removed', removed_at = ?
		WHERE user_id = ? AND status = 'active'`

	now := time.Now().UnixMilli()
	_, err := s.getQuerier(ctx).ExecContext(ctx, query, now, userID)
	return err
}

// FindByUserAndOrgID finds a membership by UserID and OrganizationID value objects.
func (s *MemberStorage) FindByUserAndOrgID(ctx context.Context, userID organization.UserID, orgID organization.OrganizationID) (*organization.OrganizationMember, error) {
	return s.FindByUserAndOrg(ctx, userID.String(), orgID.String())
}

// FindByOrganizationID lists all members by OrganizationID value object.
func (s *MemberStorage) FindByOrganizationID(ctx context.Context, orgID organization.OrganizationID) ([]*organization.OrganizationMember, error) {
	return s.FindByOrganization(ctx, orgID.String())
}

// FindActiveByOrganization lists all active members of an organization.
func (s *MemberStorage) FindActiveByOrganization(ctx context.Context, orgID string) ([]*organization.OrganizationMember, error) {
	return s.FindByOrganization(ctx, orgID)
}

// FindActiveByOrganizationID lists all active members by OrganizationID.
func (s *MemberStorage) FindActiveByOrganizationID(ctx context.Context, orgID organization.OrganizationID) ([]*organization.OrganizationMember, error) {
	return s.FindByOrganization(ctx, orgID.String())
}

// SoftDeleteByMemberID soft-deletes a membership by MemberID.
func (s *MemberStorage) SoftDeleteByMemberID(ctx context.Context, id organization.MemberID) error {
	return s.SoftDelete(ctx, id.String())
}

// SoftDeleteByUserID soft-deletes all memberships for an UserID.
func (s *MemberStorage) SoftDeleteByUserID(ctx context.Context, userID organization.UserID) error {
	return s.SoftDeleteByUser(ctx, userID.String())
}

// SoftDeleteByOrganization soft-deletes all memberships for an organization.
func (s *MemberStorage) SoftDeleteByOrganization(ctx context.Context, orgID string) error {
	query := `
		UPDATE organization_members
		SET status = 'removed', removed_at = ?
		WHERE organization_id = ? AND status = 'active'`

	now := time.Now().UnixMilli()
	_, err := s.getQuerier(ctx).ExecContext(ctx, query, now, orgID)
	return err
}

// CountByOrganizationID counts members by OrganizationID.
func (s *MemberStorage) CountByOrganizationID(ctx context.Context, orgID organization.OrganizationID) (int, error) {
	return s.CountByOrganization(ctx, orgID.String())
}

// CountActiveByOrganizationID counts active members by OrganizationID.
func (s *MemberStorage) CountActiveByOrganizationID(ctx context.Context, orgID organization.OrganizationID) (int, error) {
	return s.CountActiveByOrganization(ctx, orgID.String())
}

// CountSuperAdminsByOrganizationID counts super_admin members by OrganizationID.
func (s *MemberStorage) CountSuperAdminsByOrganizationID(ctx context.Context, orgID organization.OrganizationID) (int, error) {
	return s.CountSuperAdminsByOrganization(ctx, orgID.String())
}

// ListByUserID lists all memberships by UserID.
func (s *MemberStorage) ListByUserID(ctx context.Context, userID organization.UserID) ([]*organization.OrganizationMember, error) {
	return s.ListByUser(ctx, userID.String())
}

// memberStatusToLifecycle maps a status string from the database to MemberLifecycle.
func memberStatusToLifecycle(status string) organization.MemberLifecycle {
	switch status {
	case "active":
		return organization.MemberLifecycleActive
	case "suspended":
		return organization.MemberLifecycleSuspended
	case "removed":
		return organization.MemberLifecycleRemoved
	default:
		return organization.MemberLifecycleActive
	}
}

// timePtrMillis renders an optional timestamp as Unix millis, or nil for SQL NULL.
func timePtrMillis(t *time.Time) interface{} {
	if t == nil {
		return nil
	}

	return t.UnixMilli()
}
