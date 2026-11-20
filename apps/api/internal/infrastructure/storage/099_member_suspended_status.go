package storage

import (
	"context"
	"database/sql"
	"strings"
)

// migrateMemberSuspendedStatus makes `suspended` a storable membership state.
//
// The table was created (039) with `CHECK (status IN ('active','removed'))`, but
// the domain has carried a `suspended` lifecycle since then: SuspendMember
// transitions to it and MemberStorage.Update writes it. On any backend that
// enforces the constraint the UPDATE fails, so suspending a member 500s. The
// `suspended_at` timestamp the domain sets was never persisted at all.
//
// SQLite cannot widen a CHECK in place, so the table is rebuilt: shadow copy
// with the widened constraint and the new column, copy, drop, rename. The copy
// uses the live column set from PRAGMA table_info so it is resilient to columns
// added by later migrations. Idempotent: it skips when the constraint is already
// widened.
func migrateMemberSuspendedStatus(tx *sql.Tx) error {
	ctx := context.Background()

	_, _ = tx.ExecContext(ctx, "DROP TABLE IF EXISTS _organization_members_new")

	cols, needsRebuild, err := inspectMemberColumns(tx)
	if err != nil {
		return err
	}
	if !needsRebuild {
		return nil
	}

	var def strings.Builder
	def.WriteString("CREATE TABLE _organization_members_new (")
	for i, ci := range cols {
		if i > 0 {
			def.WriteString(", ")
		}
		def.WriteString(quoteIdent(ci.name))
		def.WriteString(" ")
		def.WriteString(memberColumnType(ci))
		if ci.dflt.Valid {
			def.WriteString(" DEFAULT ")
			def.WriteString(ci.dflt.String)
		}
		if ci.pk == 1 {
			def.WriteString(" PRIMARY KEY")
		}
	}
	if !hasMemberColumn(cols, "suspended_at") {
		def.WriteString(", \"suspended_at\" INTEGER")
	}
	def.WriteString(")")

	if _, err = tx.ExecContext(ctx, def.String()); err != nil {
		return err
	}

	names := make([]string, 0, len(cols))
	for _, ci := range cols {
		names = append(names, quoteIdent(ci.name))
	}
	colList := strings.Join(names, ", ")

	copyQuery := "INSERT INTO _organization_members_new (" + colList + ") SELECT " + colList + " FROM organization_members"
	if _, err = tx.ExecContext(ctx, copyQuery); err != nil {
		_, _ = tx.ExecContext(ctx, "DROP TABLE _organization_members_new")
		return err
	}

	if _, err = tx.ExecContext(ctx, "DROP TABLE organization_members"); err != nil {
		_, _ = tx.ExecContext(ctx, "DROP TABLE _organization_members_new")
		return err
	}
	if _, err = tx.ExecContext(ctx, "ALTER TABLE _organization_members_new RENAME TO organization_members"); err != nil {
		return err
	}

	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_org_members_operator ON organization_members(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_org_members_org ON organization_members(organization_id)",
	} {
		if _, execErr := tx.ExecContext(ctx, stmt); execErr != nil {
			continue
		}
	}

	return nil
}

type memberColInfo struct {
	name    string
	typ     string
	dflt    sql.NullString
	notnull int
	pk      int
}

// memberColumnType renders a column's type, widening the status CHECK to admit
// `suspended` and preserving NOT NULL for every other column.
func memberColumnType(ci memberColInfo) string {
	if strings.EqualFold(ci.name, "status") {
		return "TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'removed'))"
	}
	if ci.notnull == 1 {
		return ci.typ + " NOT NULL"
	}
	return ci.typ
}

func hasMemberColumn(cols []memberColInfo, name string) bool {
	for _, ci := range cols {
		if strings.EqualFold(ci.name, name) {
			return true
		}
	}
	return false
}

// inspectMemberColumns reads the organization_members schema and reports whether
// the status CHECK still excludes `suspended`.
func inspectMemberColumns(tx *sql.Tx) ([]memberColInfo, bool, error) {
	ctx := context.Background()

	rows, err := tx.QueryContext(ctx, "PRAGMA table_info(organization_members)")
	if err != nil {
		return nil, false, err
	}
	var cols []memberColInfo
	func() {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var ci memberColInfo
			var cid int
			if scanErr := rows.Scan(&cid, &ci.name, &ci.typ, &ci.notnull, &ci.dflt, &ci.pk); scanErr != nil {
				err = scanErr
				return
			}
			cols = append(cols, ci)
		}
		if err = rows.Err(); err != nil {
			return
		}
	}()
	if err != nil {
		return nil, false, err
	}
	if len(cols) == 0 {
		return nil, false, nil
	}

	var ddl string
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(sql, '') FROM sqlite_master WHERE type='table' AND name='organization_members'`,
	).Scan(&ddl); err != nil {
		return nil, false, err
	}

	needsRebuild := !strings.Contains(ddl, "'suspended'") || !hasMemberColumn(cols, "suspended_at")

	return cols, needsRebuild, nil
}
