import { useMemo, useState } from 'react';
import { css } from '@emotion/css';
import { useNavigate } from '@tanstack/react-router';
import {
  Button,
  Combobox,
  ConfirmModal,
  EmptyState,
  Field,
  Input,
  LoadingPlaceholder,
  Modal,
  Tabs,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import type { OrgUser } from '@/hooks/admin';
import { Page } from '@/ui/components/layout';
import { SearchInput } from '@/ui/components/shared/search-input';
import { StatusBadge } from '@/ui/components/shared/status-badge';
import { UserAvatar } from '@/ui/components/shared/user-avatar';
import { filterTableStyles } from '@/ui/components/shared/table';
import { useDebounce } from '@/hooks/_shared/use-debounce';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';
import { useCurrentOrganizationId, useCurrentUser } from '@/hooks/_shared';
import { useAdminUsers, useAdminUserStats } from '@/hooks/admin';
import { useInviteMember, useReinstateMember, useRemoveMember, useSuspendMember } from '@/hooks/organization';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { formatDate, formatDateTime } from '@/ui/pages/profile/profile-page';

const PAGE_SIZE = 20;

type TabKey = 'members' | 'all';
type SortKey = 'created_at' | 'email' | 'name' | 'last_login_at';

const TABS: Array<{ label: string; value: TabKey }> = [
  { label: 'Members', value: 'members' },
  { label: 'All users', value: 'all' },
];

const SORT_OPTIONS: Array<{ label: string; value: SortKey }> = [
  { label: 'Newest', value: 'created_at' },
  { label: 'Email', value: 'email' },
  { label: 'Name', value: 'name' },
  { label: 'Last active', value: 'last_login_at' },
];

const ASCENDING_SORTS: ReadonlySet<SortKey> = new Set<SortKey>(['email', 'name']);

const ROLE_ORDER = ['super_admin', 'admin', 'operator', 'viewer'] as const;

function roleLabel(role: string | undefined): string {
  if (!role) return '—';
  return role.replace(/_/g, ' ');
}

export function AdminUsersPage() {
  const styles = useStyles2(getStyles);
  const table = useStyles2(filterTableStyles);
  const navigate = useNavigate();
  const currentUser = useCurrentUser();
  const organizationId = useCurrentOrganizationId();
  const capabilities = useCapabilities();

  const [tab, setTab] = useState<TabKey>('members');
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<SortKey>('created_at');
  const [page, setPage] = useState(1);
  const [inviteOpen, setInviteOpen] = useState(false);

  const debouncedSearch = useDebounce(search, 300);
  const query = useAdminUsers({
    page,
    limit: PAGE_SIZE,
    q: debouncedSearch,
    sort,
    order: ASCENDING_SORTS.has(sort) ? 'asc' : 'desc',
    status: tab === 'members' ? 'active' : undefined,
  });
  const stats = useAdminUserStats();

  const users = query.data?.users ?? [];
  const total = query.data?.total ?? 0;
  const totalPages = query.data?.total_pages ?? 0;
  const isSearching = debouncedSearch.trim() !== '';

  const roleCounts = useMemo(() => {
    const byRole = stats.data?.by_role ?? {};
    return ROLE_ORDER.map((role) => ({ role, count: byRole[role] ?? 0 })).filter((entry) => entry.count > 0);
  }, [stats.data]);

  const changeFilter = (next: () => void) => {
    setPage(1);
    next();
  };

  const openUser = (user: OrgUser) => {
    if (user.id) void navigate({ to: '/admin/users/$userId', params: { userId: user.id } });
  };

  return (
    <Page
      title="Users"
      subTitle="Members of this organisation"
      actions={
        capabilities.can('members.write') ? (
          <Button variant="primary" onClick={() => setInviteOpen(true)}>
            Invite member
          </Button>
        ) : null
      }
    >
      <Tabs
        aria-label="User views"
        tabs={TABS}
        value={tab}
        onChange={(next) => changeFilter(() => setTab(next))}
        className={styles.tabs}
      />

      {tab === 'all' ? (
        <div className={styles.statsRow}>
          {roleCounts.map((entry) => (
            <div key={entry.role} className={styles.stat}>
              <span className={styles.statValue}>{entry.count}</span>
              <span className={styles.statLabel}>{roleLabel(entry.role)}</span>
            </div>
          ))}
          <div className={styles.stat}>
            <span className={styles.statValue}>{stats.data?.pending_invites ?? 0}</span>
            <span className={styles.statLabel}>Pending invites</span>
          </div>
        </div>
      ) : null}

      <div className={table.actionBar}>
        <div className={styles.search}>
          <SearchInput
            value={search}
            onChange={(value) => changeFilter(() => setSearch(value))}
            placeholder="Search user by email or name"
          />
        </div>
        <Combobox
          aria-label="Sort users"
          value={sort}
          options={SORT_OPTIONS.map((option) => ({ value: option.value, label: option.label }))}
          onChange={(next) => next && changeFilter(() => setSort(next as SortKey))}
          isClearable={false}
        />
        <div className={table.actionBarSpacer} />
        <span className={table.muted}>{total} {tab === 'members' ? 'members' : 'users'}</span>
      </div>

      {query.isLoading ? <LoadingPlaceholder text="Loading users…" /> : null}

      {query.isError ? (
        <EmptyState icon="alert-triangle" message="Could not load users. Check your permissions and try again." />
      ) : null}

      {!query.isLoading && !query.isError && users.length === 0 ? (
        <EmptyState icon="users" message={isSearching ? 'No users found' : 'No users yet'} />
      ) : null}

      {users.length > 0 ? (
        <div className={table.wrapper}>
          <table className={table.table}>
            <thead>
              <tr>
                <th className={table.headCell} aria-label="Avatar" />
                <th className={table.headCell}>Name</th>
                <th className={table.headCell}>Email</th>
                <th className={table.headCell}>Role</th>
                <th className={table.headCell}>Status</th>
                <th className={table.headCell}>Joined</th>
                <th className={table.headCell}>Last seen</th>
                <th className={table.headCell} />
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <UserRow
                  key={user.member_id ?? user.id}
                  user={user}
                  isSelf={user.id === currentUser.id}
                  capabilities={capabilities}
                  onOpen={() => openUser(user)}
                />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      {totalPages > 1 ? (
        <div className={styles.pager}>
          <Button
            variant="secondary"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
          >
            Previous
          </Button>
          <span className={table.muted}>
            Page {page} of {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
          >
            Next
          </Button>
        </div>
      ) : null}

      {inviteOpen && organizationId ? (
        <InviteModal organizationId={organizationId} onDismiss={() => setInviteOpen(false)} />
      ) : null}
    </Page>
  );
}

function UserRow({
  user,
  isSelf,
  capabilities,
  onOpen,
}: {
  user: OrgUser;
  isSelf: boolean;
  capabilities: ReturnType<typeof useCapabilities>;
  onOpen: () => void;
}) {
  const table = useStyles2(filterTableStyles);
  const suspend = useSuspendMember();
  const reinstate = useReinstateMember();
  const remove = useRemoveMember();
  const [confirmRemove, setConfirmRemove] = useState(false);

  const organizationId = useCurrentOrganizationId();
  const memberId = user.member_id ?? '';
  const suspended = user.status === 'suspended';
  const manageable = capabilities.canManageMember(user.role);

  return (
    <tr>
      <td className={table.cell}>
        <UserAvatar
          name={user.name}
          email={user.email}
          src={user.avatar_url}
          size={28}
          expandable
        />
      </td>
      <td className={table.cell}>
        {user.name || '—'}
        {isSelf ? <span className={table.subLine}>you</span> : null}
      </td>
      <td className={table.cell}>{user.email ?? '—'}</td>
      <td className={table.cell}>
        <StatusBadge label={roleLabel(user.role)} icon={user.role === 'super_admin' ? 'shield' : undefined} />
      </td>
      <td className={table.cell}>
        <StatusBadge label={suspended ? 'Suspended' : user.status === 'active' ? 'Active' : (user.status ?? '—')} />
      </td>
      <td className={table.cell}>{formatDate(user.joined_at)}</td>
      <td className={table.cell}>{formatDateTime(user.last_seen_at)}</td>
      <td className={table.cell}>
        <div className={table.actions}>
          <Button variant="secondary" size="sm" onClick={onOpen}>
            Details
          </Button>
          {manageable ? (
            suspended ? (
              <Button
                variant="secondary"
                size="sm"
                loading={reinstate.isPending}
                onClick={() =>
                  reinstate.mutate(
                    { id: organizationId ?? '', memberId },
                    {
                      onSuccess: () => notifySuccess('Member reinstated'),
                      onError: () => notifyError('Could not reinstate member'),
                    },
                  )
                }
              >
                Reinstate
              </Button>
            ) : (
              <Button
                variant="secondary"
                size="sm"
                loading={suspend.isPending}
                onClick={() =>
                  suspend.mutate(
                    { id: organizationId ?? '', memberId },
                    {
                      onSuccess: () => notifySuccess('Member suspended', 'Their access to this organisation is paused.'),
                      onError: () => notifyError('Could not suspend member'),
                    },
                  )
                }
              >
                Suspend
              </Button>
            )
          ) : null}
          {manageable ? (
            <Button variant="destructive" size="sm" onClick={() => setConfirmRemove(true)}>
              Remove
            </Button>
          ) : null}
        </div>
      </td>

      <ConfirmModal
        isOpen={confirmRemove}
        title="Remove member"
        body={`Remove ${user.email || 'this member'} from this organisation? Their account is not deleted.`}
        confirmText="Remove"
        destructive
        isConfirming={remove.isPending}
        onDismiss={() => setConfirmRemove(false)}
        onConfirm={() =>
          remove.mutate(
            { id: organizationId ?? '', memberId },
            {
              onSuccess: () => {
                notifySuccess('Member removed');
                setConfirmRemove(false);
              },
              onError: () => notifyError('Could not remove member'),
            },
          )
        }
      />
    </tr>
  );
}

function InviteModal({ organizationId, onDismiss }: { organizationId: string; onDismiss: () => void }) {
  const invite = useInviteMember();
  const [email, setEmail] = useState('');
  const [role, setRole] = useState('operator');

  const close = () => {
    setEmail('');
    setRole('operator');
    onDismiss();
  };

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    invite.mutate(
      { data: { organizationId, email, role } },
      {
        onSuccess: () => {
          notifySuccess('Invitation sent', email);
          close();
        },
        onError: () => notifyError('Could not send invitation', 'Check the address and try again.'),
      },
    );
  };

  return (
    <Modal title="Invite member" isOpen onDismiss={close} hideCloseButton>
      <form onSubmit={submit} className={formStyle}>
        <Field label="Email" required description="They join this organisation only after accepting.">
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </Field>
        <Field label="Role">
          <Combobox
            aria-label="Role"
            value={role}
            options={[
              { value: 'operator', label: 'Operator' },
              { value: 'viewer', label: 'Viewer' },
              { value: 'admin', label: 'Administrator' },
            ]}
            onChange={(next) => setRole(next ?? 'operator')}
            isClearable={false}
          />
        </Field>
        <div className={formActionsStyle}>
          <Button type="button" variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" loading={invite.isPending}>
            Send invitation
          </Button>
        </div>
      </form>
    </Modal>
  );
}

const formStyle = css({ display: 'flex', flexDirection: 'column', gap: 16 });
const formActionsStyle = css({ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 });

const getStyles = (theme: VyzorixTheme) => ({
  tabs: css({ marginBottom: theme.spacing(2) }),
  search: css({ flex: '0 1 420px', minWidth: 240 }),
  statsRow: css({
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(3),
    marginBottom: theme.spacing(2.5),
  }),
  stat: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  statValue: css({
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  statLabel: css({
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    textTransform: 'capitalize',
  }),
  pager: css({
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'flex-end',
    gap: theme.spacing(1.5),
    marginTop: theme.spacing(2),
  }),
});
