import { useState } from 'react';
import { css } from '@emotion/css';
import {
  Button,
  ConfirmModal,
  EmptyState,
  LoadingPlaceholder,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { Page } from '@/ui/components/layout';
import { StatusBadge } from '@/ui/components/shared/status-badge';
import { UserAvatar } from '@/ui/components/shared/user-avatar';
import { useCapabilities } from '@/hooks/_shared/use-capabilities';
import { useCurrentOrganizationId } from '@/hooks/_shared';
import { useAdminUser } from '@/hooks/admin';
import { useReinstateMember, useRemoveMember, useSuspendMember } from '@/hooks/organization';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { formatDate, formatDateTime } from '@/ui/pages/profile/profile-page';

export interface AdminUserDetailPageProps {
  userId: string;
  onBack: () => void;
}

export function AdminUserDetailPage({ userId, onBack }: AdminUserDetailPageProps) {
  const styles = useStyles2(getStyles);
  const user = useAdminUser(userId);
  const capabilities = useCapabilities();

  if (user.isLoading) {
    return (
      <Page title="Member" subTitle="Their membership in this organisation">
        <LoadingPlaceholder text="Loading member…" />
      </Page>
    );
  }

  if (user.isError || !user.data) {
    return (
      <Page title="Member" subTitle="Their membership in this organisation">
        <EmptyState icon="alert-triangle" message="This person is not a member of this organisation.">
          <Button variant="secondary" onClick={onBack}>
            Back to users
          </Button>
        </EmptyState>
      </Page>
    );
  }

  const data = user.data;
  const displayName = data.name || data.email || 'Member';

  return (
    <Page
      title={displayName}
      subTitle="Their membership in this organisation"
      actions={
        <Button variant="secondary" onClick={onBack}>
          Back to users
        </Button>
      }
    >
      <div className={styles.stack}>
        <div className={styles.identity}>
          <UserAvatar name={data.name} email={data.email} src={data.avatar_url} size={72} expandable />
          <div>
            <div className={styles.name}>{displayName}</div>
            {data.email ? <div className={styles.email}>{data.email}</div> : null}
          </div>
        </div>

        <div>
          <h3 className={styles.pageHeading}>Membership</h3>
          <table className={styles.filterTable}>
            <tbody>
              <tr>
                <td className={styles.labelCell}>Role</td>
                <td className={styles.valueCell}>
                  <StatusBadge
                    label={(data.role ?? '—').replace(/_/g, ' ')}
                    icon={data.role === 'super_admin' ? 'shield' : undefined}
                  />
                </td>
              </tr>
              <tr>
                <td className={styles.labelCell}>Status</td>
                <td className={styles.valueCell}>
                  <StatusBadge label={statusLabel(data.status)} />
                </td>
              </tr>
              <tr>
                <td className={styles.labelCell}>Joined</td>
                <td className={styles.valueCell}>{formatDate(data.joined_at)}</td>
              </tr>
              <tr>
                <td className={styles.labelCell}>Last seen</td>
                <td className={styles.valueCell}>{formatDateTime(data.last_seen_at)}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <MembershipActions
          memberId={data.member_id ?? ''}
          status={data.status}
          targetRole={data.role}
          capabilities={capabilities}
          onRemoved={onBack}
        />
      </div>
    </Page>
  );
}

function statusLabel(status: string | undefined): string {
  if (status === 'active') return 'Active';
  if (status === 'suspended') return 'Suspended';
  return status ?? '—';
}

function MembershipActions({
  memberId,
  status,
  targetRole,
  capabilities,
  onRemoved,
}: {
  memberId: string;
  status: string | undefined;
  targetRole: string | undefined;
  capabilities: ReturnType<typeof useCapabilities>;
  onRemoved: () => void;
}) {
  const styles = useStyles2(getStyles);
  const organizationId = useCurrentOrganizationId() ?? '';
  const suspend = useSuspendMember();
  const reinstate = useReinstateMember();
  const remove = useRemoveMember();
  const [confirmRemove, setConfirmRemove] = useState(false);

  if (!capabilities.canManageMember(targetRole)) {
    return (
      <div>
        <h3 className={styles.pageHeading}>Membership actions</h3>
        <p className={styles.hint}>
          {capabilities.can('members.write')
            ? 'You cannot change a member with an equal or higher role.'
            : 'You do not have permission to manage members of this organisation.'}
        </p>
      </div>
    );
  }

  return (
    <div>
      <h3 className={styles.pageHeading}>Membership actions</h3>
      <div className={styles.buttonRow}>
        {status === 'suspended' ? (
          <Button
            variant="secondary"
            loading={reinstate.isPending}
            onClick={() =>
              reinstate.mutate(
                { id: organizationId, memberId },
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
            loading={suspend.isPending}
            onClick={() =>
              suspend.mutate(
                { id: organizationId, memberId },
                {
                  onSuccess: () => notifySuccess('Member suspended', 'Their access to this organisation is paused.'),
                  onError: () => notifyError('Could not suspend member'),
                },
              )
            }
          >
            Suspend
          </Button>
        )}
        <Button variant="destructive" onClick={() => setConfirmRemove(true)}>
          Remove from organisation
        </Button>
      </div>
      <p className={styles.hint}>
        Suspending pauses this person&apos;s access to this organisation. Removing ends the membership; their
        account is not deleted.
      </p>

      <ConfirmModal
        isOpen={confirmRemove}
        title="Remove from organisation"
        body="This ends their membership of this organisation. Their account is not deleted."
        confirmText="Remove"
        destructive
        isConfirming={remove.isPending}
        onDismiss={() => setConfirmRemove(false)}
        onConfirm={() =>
          remove.mutate(
            { id: organizationId, memberId },
            {
              onSuccess: () => {
                notifySuccess('Member removed');
                setConfirmRemove(false);
                onRemoved();
              },
              onError: () => notifyError('Could not remove member'),
            },
          )
        }
      />
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  stack: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),

  identity: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
  }),
  name: css({
    fontSize: theme.typography.lg.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  email: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),

  pageHeading: css({
    margin: `0 0 ${theme.spacing(2)}`,
    fontSize: theme.typography.h4.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),

  filterTable: css({
    width: '100%',
    borderCollapse: 'separate',
    'tbody tr:nth-of-type(odd)': {
      background: theme.colors.emphasize(theme.colors.background.primary, 0.02),
    },
    '& td': {
      padding: `${theme.spacing(0.5)} ${theme.spacing(1)}`,
      height: 30,
      lineHeight: '30px',
      verticalAlign: 'middle',
      fontSize: theme.typography.sm.fontSize,
    },
  }),
  labelCell: css({
    width: 240,
    color: theme.colors.text.secondary,
    fontWeight: theme.typography.fontWeightMedium,
    whiteSpace: 'nowrap',
  }),
  valueCell: css({ color: theme.colors.text.primary }),
  buttonRow: css({ display: 'flex', gap: theme.spacing(1), flexWrap: 'wrap' }),
  hint: css({
    margin: `${theme.spacing(1.5)} 0 0`,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
  }),
});
