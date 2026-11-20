package auth

import (
	"context"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/session"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// stubUserRepo implements user.Repository by embedding the interface, so only the
// methods a test exercises need a body. The embedded nil interface panics on any
// other call, which is what a test wants when it reaches an unexpected path.
type stubUserRepo struct {
	user.Repository
	byID       map[string]*user.User
	setDisable []struct {
		at     *time.Time
		id     string
		called bool
	}
	revokedSessions []string
}

func (r *stubUserRepo) FindByID(_ context.Context, id string) (*user.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, user.ErrNotFound
}

func (r *stubUserRepo) SetDisabled(_ context.Context, id string, at *time.Time) error {
	if _, ok := r.byID[id]; !ok {
		return user.ErrNotFound
	}
	r.setDisable = append(r.setDisable, struct {
		at     *time.Time
		id     string
		called bool
	}{id: id, at: at, called: true})
	return nil
}

// stubSessionRepo records RevokeAllUserSessions, which disabling must call.
type stubSessionRepo struct {
	session.Repository
	revokedAll []string
}

func (r *stubSessionRepo) RevokeAllUserSessions(_ context.Context, userID string) error {
	r.revokedAll = append(r.revokedAll, userID)
	return nil
}

func TestSetUserDisabled_StampsAndRevokesSessions(t *testing.T) {
	u := &user.User{ID: "u1", Email: "ada@acme.io"}
	users := &stubUserRepo{byID: map[string]*user.User{"u1": u}}
	sessions := &stubSessionRepo{}
	svc := &AuthService{userRepo: users, sessionRepo: sessions}

	updated, err := svc.SetUserDisabled(context.Background(), "u1", true)
	if err != nil {
		t.Fatalf("SetUserDisabled: %v", err)
	}
	if !updated.Disabled || updated.DisabledAt == nil {
		t.Error("expected the returned user to be marked disabled with a timestamp")
	}
	if len(users.setDisable) != 1 || users.setDisable[0].at == nil {
		t.Fatalf("expected one disable write with a non-nil time, got %+v", users.setDisable)
	}
	if len(sessions.revokedAll) != 1 || sessions.revokedAll[0] != "u1" {
		t.Errorf("disabling must revoke the user's sessions, got %v", sessions.revokedAll)
	}
}

func TestSetUserDisabled_ReenableClearsTimestamp(t *testing.T) {
	u := &user.User{ID: "u1", Disabled: true}
	users := &stubUserRepo{byID: map[string]*user.User{"u1": u}}
	sessions := &stubSessionRepo{}
	svc := &AuthService{userRepo: users, sessionRepo: sessions}

	updated, err := svc.SetUserDisabled(context.Background(), "u1", false)
	if err != nil {
		t.Fatalf("SetUserDisabled: %v", err)
	}
	if updated.Disabled || updated.DisabledAt != nil {
		t.Error("re-enabling must clear the disabled flag and timestamp")
	}
	if users.setDisable[0].at != nil {
		t.Error("re-enabling must write a nil timestamp")
	}
	if len(sessions.revokedAll) != 0 {
		t.Errorf("re-enabling must not revoke sessions, got %v", sessions.revokedAll)
	}
}

func TestSetUserDisabled_UnknownUser(t *testing.T) {
	users := &stubUserRepo{byID: map[string]*user.User{}}
	svc := &AuthService{userRepo: users}

	if _, err := svc.SetUserDisabled(context.Background(), "nope", true); err != application.ErrUserNotFound {
		t.Errorf("got %v, want ErrUserNotFound", err)
	}
}

// TestValidateSession_RejectsDisabledUser proves a disabled account cannot keep a
// live session: the session is deleted and validation fails.
func TestValidateSession_RejectsDisabledUser(t *testing.T) {
	users := &stubUserRepo{byID: map[string]*user.User{
		"u1": {ID: "u1", Disabled: true},
	}}
	sessions := &deletingSessionRepo{found: &session.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour)}}
	svc := &AuthService{userRepo: users, sessionRepo: sessions}

	if _, _, err := svc.ValidateSession(context.Background(), "s1"); err != application.ErrUserDisabled {
		t.Fatalf("got %v, want ErrUserDisabled", err)
	}
	if len(sessions.deleted) != 1 || sessions.deleted[0] != "s1" {
		t.Errorf("expected the disabled user's session to be deleted, got %v", sessions.deleted)
	}
}

// deletingSessionRepo returns one session and records deletions.
type deletingSessionRepo struct {
	session.Repository
	found   *session.Session
	deleted []string
}

func (r *deletingSessionRepo) FindByID(_ context.Context, id string) (*session.Session, error) {
	if r.found != nil && r.found.ID == id {
		return r.found, nil
	}
	return nil, session.ErrNotFound
}

func (r *deletingSessionRepo) IsSessionRevoked(context.Context, string) (bool, error) {
	return false, nil
}

func (r *deletingSessionRepo) Delete(_ context.Context, id string) error {
	r.deleted = append(r.deleted, id)
	return nil
}

// TestCreateSession_RefusesDisabledAccount guards the single choke point every
// login path funnels through. Password login, token login, MFA completion and
// device recognition all call CreateSession, so a disabled account must be
// refused here or it slips through the paths that forgot to check.
func TestCreateSession_RefusesDisabledAccount(t *testing.T) {
	users := &stubUserRepo{byID: map[string]*user.User{
		"u1": {ID: "u1", Disabled: true},
	}}
	sessions := &countingSessionRepo{}
	svc := &AuthService{userRepo: users, sessionRepo: sessions}

	if _, err := svc.CreateSession(context.Background(), "u1", SessionMeta{}); err != application.ErrUserDisabled {
		t.Fatalf("got %v, want ErrUserDisabled", err)
	}
	if sessions.created != 0 {
		t.Errorf("no session may be created for a disabled account, got %d", sessions.created)
	}
}

// countingSessionRepo records Create calls and satisfies the rest of the
// interface via the embedded nil interface.
type countingSessionRepo struct {
	session.Repository
	created int
}

func (r *countingSessionRepo) ListActiveByUser(context.Context, string) ([]*session.Session, error) {
	return nil, nil
}

func (r *countingSessionRepo) Create(_ context.Context, _ *session.Session) error {
	r.created++
	return nil
}

// memberRepoFor returns a member repository stub whose ListByUser reports the
// given memberships, so role resolution can be exercised without a database.
type memberRepoFor struct {
	organization.MemberRepository
	byUser map[string][]*organization.OrganizationMember
	byOrg  map[string][]*organization.OrganizationMember
}

func (r *memberRepoFor) ListByUser(_ context.Context, userID string) ([]*organization.OrganizationMember, error) {
	return r.byUser[userID], nil
}

func (r *memberRepoFor) FindByOrganization(_ context.Context, orgID string) ([]*organization.OrganizationMember, error) {
	return r.byOrg[orgID], nil
}

func (r *memberRepoFor) FindByUserAndOrg(_ context.Context, userID, orgID string) (*organization.OrganizationMember, error) {
	for _, m := range r.byOrg[orgID] {
		if m.UserID == userID {
			return m, nil
		}
	}

	return nil, organization.ErrMemberNotFound
}

// TestListOrgUsers_ReportsMembershipFacts guards the org-scoped listing: the role,
// status and joined date reported for each row are the membership's, not a
// default, and the account's last login is carried through as last seen.
func TestListOrgUsers_ReportsMembershipFacts(t *testing.T) {
	joined := time.UnixMilli(1_700_000_000_000)
	lastSeen := time.UnixMilli(1_700_000_500_000)
	users := &orgUserRepo{rows: []*user.OrganizationUser{{
		User:       user.User{ID: "u1", Email: "ada@acme.io", Name: "Ada"},
		MemberID:   "m1",
		Role:       string(organization.RoleViewer),
		Status:     string(organization.MemberLifecycleSuspended),
		JoinedAt:   joined,
		LastSeenAt: &lastSeen,
	}}}
	svc := &AuthService{userRepo: users}

	got, total, err := svc.ListOrgUsers(context.Background(), ListUsersQuery{OrgID: "org-a", Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("ListOrgUsers: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("expected 1 row, got %d (total %d)", len(got), total)
	}
	if got[0].Role != string(organization.RoleViewer) {
		t.Errorf("role = %q, want viewer", got[0].Role)
	}
	if got[0].Status != string(organization.MemberLifecycleSuspended) {
		t.Errorf("status = %q, want suspended", got[0].Status)
	}
	if got[0].MemberID != "m1" {
		t.Errorf("member id = %q, want m1", got[0].MemberID)
	}
	if got[0].JoinedAt != joined.UnixMilli() {
		t.Errorf("joined = %d, want %d", got[0].JoinedAt, joined.UnixMilli())
	}
	if got[0].LastSeenAt == nil || *got[0].LastSeenAt != lastSeen.UnixMilli() {
		t.Errorf("last seen = %v, want %d", got[0].LastSeenAt, lastSeen.UnixMilli())
	}
}

// orgUserRepo returns a fixed page of membership-joined rows and records the org
// it was asked for, so the test can assert the listing is org-scoped.
type orgUserRepo struct {
	user.Repository
	orgID string
	rows  []*user.OrganizationUser
}

func (r *orgUserRepo) ListByOrganization(_ context.Context, orgID string, _, _ int, _, _ string, _ bool, _ string) ([]*user.OrganizationUser, int, error) {
	r.orgID = orgID

	return r.rows, len(r.rows), nil
}

// TestListOrgUsers_ScopesToTheGivenOrganisation pins that the listing is handed
// the caller's organisation: there is no deployment-wide path, so the org id is
// the only thing that decides which rows come back.
func TestListOrgUsers_ScopesToTheGivenOrganisation(t *testing.T) {
	users := &orgUserRepo{}
	svc := &AuthService{userRepo: users}

	if _, _, err := svc.ListOrgUsers(context.Background(), ListUsersQuery{OrgID: "org-a", Page: 1, Limit: 10}); err != nil {
		t.Fatalf("ListOrgUsers: %v", err)
	}
	if users.orgID != "org-a" {
		t.Errorf("listed org = %q, want org-a", users.orgID)
	}
}

// TestGetOrgUser_RefusesNonMember pins the read boundary: a user with no
// membership in the acting organisation is reported as not found, so an
// administrator of one org learns nothing about another org's accounts.
func TestGetOrgUser_RefusesNonMember(t *testing.T) {
	members := &memberRepoFor{}
	svc := &AuthService{userRepo: &stubUserRepo{}, memberRepo: members}

	if _, err := svc.GetOrgUser(context.Background(), "org-a", "u1"); err != application.ErrUserNotFound {
		t.Fatalf("got %v, want ErrUserNotFound", err)
	}
}

// TestUserStats_CountsRolesAndPendingInvites guards the tab summary: it counts
// non-removed members per role and reports the org's pending invitations.
func TestUserStats_CountsRolesAndPendingInvites(t *testing.T) {
	members := &memberRepoFor{byOrg: map[string][]*organization.OrganizationMember{
		"org-a": {
			{ID: "m1", Role: organization.RoleAdmin, Lifecycle: organization.MemberLifecycleActive},
			{ID: "m2", Role: organization.RoleAdmin, Lifecycle: organization.MemberLifecycleSuspended},
			{ID: "m3", Role: organization.RoleViewer, Lifecycle: organization.MemberLifecycleActive},
			{ID: "m4", Role: organization.RoleViewer, Lifecycle: organization.MemberLifecycleRemoved},
		},
	}}
	invites := &invitationRepoFor{pending: map[string]int{"org-a": 3}}
	svc := &AuthService{memberRepo: members, invitationRepo: invites}

	stats, err := svc.UserStats(context.Background(), "org-a")
	if err != nil {
		t.Fatalf("UserStats: %v", err)
	}
	if stats.ByRole[string(organization.RoleAdmin)] != 2 {
		t.Errorf("admin count = %d, want 2 (active + suspended)", stats.ByRole[string(organization.RoleAdmin)])
	}
	if stats.ByRole[string(organization.RoleViewer)] != 1 {
		t.Errorf("viewer count = %d, want 1 (removed excluded)", stats.ByRole[string(organization.RoleViewer)])
	}
	if stats.Total != 3 {
		t.Errorf("total = %d, want 3", stats.Total)
	}
	if stats.PendingInvites != 3 {
		t.Errorf("pending invites = %d, want 3", stats.PendingInvites)
	}
}

// invitationRepoFor reports pending invitation counts per organisation.
type invitationRepoFor struct {
	organization.InvitationRepository
	pending map[string]int
}

func (r *invitationRepoFor) CountPendingByOrganization(_ context.Context, orgID string) (int, error) {
	return r.pending[orgID], nil
}
