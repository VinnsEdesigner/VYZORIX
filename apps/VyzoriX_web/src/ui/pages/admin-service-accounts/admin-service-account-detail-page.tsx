import { useState } from 'react';
import { css, cx } from '@emotion/css';
import {
  Button,
  ConfirmModal,
  EmptyState,
  Field,
  Icon,
  IconButton,
  InfoBox,
  Input,
  LoadingPlaceholder,
  Modal,
  OverlaySheet,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import type { FixedRoleGrant, ServiceAccount, ServiceAccountToken } from '@/hooks/service-accounts';
import { Page } from '@/ui/components/layout';
import { StatusBadge } from '@/ui/components/shared/status-badge';
import { filterTableStyles } from '@/ui/components/shared/table';
import {
  useCreateServiceAccountToken,
  useDeleteServiceAccount,
  useDeleteServiceAccountToken,
  useRevokeServiceAccountToken,
  useRotateServiceAccountToken,
  useServiceAccountTokens,
  useUpdateServiceAccountTokenRoles,
  useServiceAccounts,
  useUpdateServiceAccount,
} from '@/hooks/service-accounts';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { isDuplicateResource, serverErrorInfo } from '@/lib/error/server-error';
import { RolePicker, type FixedRoleScope } from './role-picker';
import { formatDate, formatDateTime } from '@/ui/pages/profile/profile-page';
import { serviceAccountAvatar } from '@/lib/avatar';

const MAX_REASON_LENGTH = 280;

export interface AdminServiceAccountDetailPageProps {
  accountId: string;
  onBack: () => void;
}

export function AdminServiceAccountDetailPage({ accountId, onBack }: AdminServiceAccountDetailPageProps) {
  const styles = useStyles2(getStyles);
  const accounts = useServiceAccounts();
  const account = (accounts.data?.service_accounts ?? []).find((candidate) => candidate.id === accountId);

  if (accounts.isLoading) {
    return (
      <Page title="Service account" subTitle="Manage settings for an individual service account">
        <LoadingPlaceholder text="Loading service account…" />
      </Page>
    );
  }

  if (!account) {
    return (
      <Page title="Service account" subTitle="Manage settings for an individual service account">
        <EmptyState icon="alert-triangle" message="Could not find this service account.">
          <Button variant="secondary" onClick={onBack}>
            Back to service accounts
          </Button>
        </EmptyState>
      </Page>
    );
  }

  return (
    <Page
      title={account.name || 'Service account'}
      subTitle="Manage settings for an individual service account"
      actions={
        <Button variant="secondary" onClick={onBack}>
          Back to service accounts
        </Button>
      }
    >
      <div className={styles.stack}>
        <InformationSection account={account} />
        <TokensSection account={account} />
        <DeleteSection account={account} onDeleted={onBack} />
      </div>
    </Page>
  );
}

function InformationSection({ account }: { account: ServiceAccount }) {
  const styles = useStyles2(getStyles);
  const update = useUpdateServiceAccount();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(account.name ?? '');

  const [confirmDisable, setConfirmDisable] = useState(false);
  const [reason, setReason] = useState('');
  const reasonLength = [...reason].length;
  const overLimit = reasonLength > MAX_REASON_LENGTH;

  const save = () => {
    if (!account.id) return;
    update.mutate(
      { id: account.id, data: { name: draft } },
      {
        onSuccess: () => {
          notifySuccess('Service account updated');
          setEditing(false);
        },
        onError: () => notifyError('Could not rename service account'),
      },
    );
  };

  const closeDisable = () => {
    setConfirmDisable(false);
    setReason('');
  };

  const applyEnabled = (enabled: boolean, why: string) => {
    if (!account.id) return;
    update.mutate(
      { id: account.id, data: { enabled, reason: why } },
      {
        onSuccess: () => {
          setConfirmDisable(false);
          setReason('');
          notifySuccess(
            enabled ? 'Service account enabled' : 'Service account disabled',
            enabled ? undefined : 'Its tokens no longer authenticate.',
          );
        },
        onError: () => notifyError('Could not change the account state'),
      },
    );
  };

  return (
    <div>
      <div className={styles.identityRow}>
        <img
          src={serviceAccountAvatar(account.id ?? account.name ?? '')}
          alt=""
          width={48}
          height={48}
          className={styles.identityAvatar}
        />
        <h3 className={styles.pageHeading}>Information</h3>
      </div>
      <table className={styles.filterTable}>
        <tbody>
          <tr>
            <td className={styles.labelCell}>Numerical identifier</td>
            <td className={styles.valueCell}>{account.id ?? '—'}</td>
            <td />
          </tr>
          <tr>
            <td className={styles.labelCell}>Name</td>
            <td className={styles.valueCell}>
              {editing ? (
                <Input
                  value={draft}
                  autoFocus
                  onChange={(event) => setDraft(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') save();
                    if (event.key === 'Escape') setEditing(false);
                  }}
                />
              ) : (
                account.name || '—'
              )}
            </td>
            <td className={styles.actionCell}>
              {editing ? (
                <div className={styles.actions}>
                  <Button size="sm" variant="primary" loading={update.isPending} onClick={save}>
                    Save
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => setEditing(false)}>
                    Cancel
                  </Button>
                </div>
              ) : (
                <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
                  Edit
                </Button>
              )}
            </td>
          </tr>
          <tr>
            <td className={styles.labelCell}>Status</td>
            <td className={styles.valueCell}>
              <StatusBadge label={account.enabled ? 'Enabled' : 'Disabled'} />
            </td>
            <td className={styles.actionCell}>
              <Button
                size="sm"
                variant="secondary"
                loading={update.isPending}
                onClick={() => (account.enabled ? setConfirmDisable(true) : applyEnabled(true, ''))}
              >
                {account.enabled ? 'Disable' : 'Enable'}
              </Button>
            </td>
          </tr>
          <tr>
            <td className={styles.labelCell}>Creation date</td>
            <td className={styles.valueCell}>{formatDate(account.created_at)}</td>
            <td />
          </tr>
        </tbody>
      </table>

      {}
      <Modal title="Disable service account" isOpen={confirmDisable} onDismiss={closeDisable}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            applyEnabled(false, reason.trim());
          }}
          className={styles.disableForm}
        >
          <p className={styles.hint}>
            Every request made with this account&apos;s tokens will be rejected until it is enabled again. The
            tokens themselves are kept.
          </p>
          <Field
            label="Reason"
            description={
              overLimit
                ? `Too long by ${reasonLength - MAX_REASON_LENGTH} characters.`
                : `Shown on this page while the account is disabled. ${MAX_REASON_LENGTH - reasonLength} left.`
            }
            invalid={overLimit}
          >
            <Input
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder="e.g. rotating credentials"
              maxLength={MAX_REASON_LENGTH}
              invalid={overLimit}
              autoFocus
            />
          </Field>
          <div className={styles.disableActions}>
            <Button type="button" variant="secondary" onClick={closeDisable}>
              Cancel
            </Button>
            <Button type="submit" variant="destructive" loading={update.isPending} disabled={overLimit}>
              Disable
            </Button>
          </div>
        </form>
      </Modal>

      {!account.enabled ? (
        <div className={styles.disabledNotice}>
          <InfoBox title="This service account is disabled" icon="alert-triangle">
            {disabledSummary(account)}
          </InfoBox>
        </div>
      ) : null}
    </div>
  );
}

function disabledSummary(account: ServiceAccount): string {
  const who = account.disabled_by_name || account.disabled_by;
  const when = account.disabled_at ? formatDateTime(account.disabled_at) : '';
  const reason = account.disabled_reason?.trim();

  const parts: string[] = [];
  if (who && when) {
    parts.push(`Disabled by ${who} on ${when}.`);
  } else if (who) {
    parts.push(`Disabled by ${who}.`);
  } else if (when) {
    parts.push(`Disabled on ${when}.`);
  }
  if (reason) {

    parts.push(`Reason given: ${reason.replace(/[.!?]$/, '')}.`);
  }

  const count = account.token_count ?? 0;
  const tokens =
    count === 0
      ? 'It has no tokens.'
      : count === 1
        ? 'Its 1 token still exists and is listed below, but requests made with it are rejected'
        : `Its ${count} tokens still exist and are listed below, but requests made with them are rejected`;

  const tail =
    count === 0
      ? ' Enabling it restores access immediately.'
      : ' until the account is enabled again. Nothing is deleted, so enabling it restores access immediately — no token needs reissuing.';

  return `${parts.join(' ')} ${tokens}${tail}`.replace(/\s+/g, ' ').trim();
}

function TokensSection({ account }: { account: ServiceAccount }) {
  const styles = useStyles2(getStyles);
  const table = useStyles2(filterTableStyles);
  const tokens = useServiceAccountTokens(account.id);
  const revoke = useRevokeServiceAccountToken();
  const rotate = useRotateServiceAccountToken();
  const removeToken = useDeleteServiceAccountToken();
  const [editingRoles, setEditingRoles] = useState<ServiceAccountToken | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [issuedSecret, setIssuedSecret] = useState('');

  const rows = tokens.data?.tokens ?? [];

  return (
    <div>
      <div className={styles.sectionHead}>
        <h3 className={styles.pageHeading}>Tokens</h3>
        <Button variant="primary" onClick={() => setCreateOpen(true)} disabled={!account.enabled}>
          Add service account token
        </Button>
      </div>

      {!account.enabled ? (
        <p className={styles.hint}>Enable the account before adding tokens.</p>
      ) : null}

      {tokens.isLoading ? <LoadingPlaceholder text="Loading tokens…" /> : null}
      {!tokens.isLoading && rows.length === 0 ? <p className={styles.hint}>No tokens yet.</p> : null}

      {rows.length > 0 ? (
        <table className={cx(table.table, styles.tokenTable)}>
          <thead>
            <tr>
              <th className={table.headCell}>Name</th>
              <th className={table.headCell}>Roles</th>
              <th className={table.headCell}>Status</th>
              <th className={table.headCell} />
            </tr>
          </thead>
          <tbody>
            {rows.map((token) => (
              <TokenRow
                key={token.id}
                token={token}
                accountName={account.name ?? ''}
                busy={revoke.isPending || rotate.isPending || removeToken.isPending}
                onEditRoles={() => setEditingRoles(token)}
                onDelete={() =>
                  removeToken.mutate(
                    { id: account.id ?? '', token: token.id ?? '', params: { purge: true } },
                    {
                      onSuccess: () => {
                        notifySuccess('Token deleted', token.name || undefined);
                        void tokens.refetch();
                      },
                      onError: (error) =>
                        notifyError('Could not delete token', serverErrorInfo(error).message || undefined),
                    },
                  )
                }
                onRevoke={() =>
                  revoke.mutate(
                    { id: account.id ?? '', token: token.id ?? '' },
                    {
                      onSuccess: () => {
                        notifySuccess('Token revoked');
                        void tokens.refetch();
                      },
                      onError: () => notifyError('Could not revoke token'),
                    },
                  )
                }
                onRotate={() =>
                  rotate.mutate(

                    { id: account.id ?? '', token: token.id ?? '', data: {} },
                    {
                      onSuccess: (result) => {
                        setIssuedSecret(result.secret ?? '');
                        void tokens.refetch();
                      },
                      onError: (error) =>
                        notifyError('Could not rotate token', serverErrorInfo(error).message || undefined),
                    },
                  )
                }
              />
            ))}
          </tbody>
        </table>
      ) : null}

      <CreateTokenModal
        isOpen={createOpen}
        accountId={account.id ?? ''}
        accountName={account.name ?? ''}
        onDismiss={() => setCreateOpen(false)}
        onCreated={(secret) => {
          setCreateOpen(false);
          setIssuedSecret(secret);
          void tokens.refetch();
        }}
      />

      <SecretModal secret={issuedSecret} onDismiss={() => setIssuedSecret('')} />

      {editingRoles ? (
        <EditTokenRolesModal
          token={editingRoles}
          accountId={account.id ?? ''}
          onDismiss={() => setEditingRoles(null)}
          onSaved={() => {
            setEditingRoles(null);
            void tokens.refetch();
          }}
        />
      ) : null}
    </div>
  );
}

function describeRoles(token: ServiceAccountToken): string {
  const parts: string[] = [];
  if (token.basic_role) parts.push(token.basic_role);
  for (const grant of token.roles ?? []) {
    const short = (grant.role ?? '').split(':')[1] ?? grant.role;
    parts.push(grant.scope === 'all' ? `${short} (all)` : short ?? '');
  }
  if (parts.length === 0) return (token.scopes ?? []).join(', ') || '—';
  return parts.filter(Boolean).join(', ');
}

function EditTokenRolesModal({
  token,
  accountId,
  onDismiss,
  onSaved,
}: {
  token: ServiceAccountToken;
  accountId: string;
  onDismiss: () => void;
  onSaved: () => void;
}) {
  const update = useUpdateServiceAccountTokenRoles();
  const [basicRole, setBasicRole] = useState(token.basic_role ?? '');
  const [roles, setRoles] = useState<FixedRoleGrant[]>(
    (token.roles ?? []).map((grant) => ({ role: grant.role ?? '', scope: grant.scope ?? 'organization' })),
  );

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    update.mutate(
      {
        id: accountId,
        token: token.id ?? '',
        data: {
          basic_role: basicRole || undefined,
          roles: roles.map((grant) => ({ role: grant.role, scope: grant.scope as FixedRoleScope })),
        },
      },
      {
        onSuccess: () => {
          notifySuccess('Roles updated', token.name || undefined);
          onSaved();
        },
        onError: (error) => notifyError('Could not update roles', serverErrorInfo(error).message || undefined),
      },
    );
  };

  return (
    <Modal title="Edit token roles" isOpen onDismiss={onDismiss} width={620} hideCloseButton>
      <form onSubmit={submit} className={formStyle}>
        <RolePicker
          basicRole={basicRole}
          onBasicRoleChange={setBasicRole}
          roles={roles}
          onRolesChange={setRoles}
        />
        <div className={formActionsStyle}>
          <Button type="button" variant="secondary" onClick={onDismiss}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" loading={update.isPending}>
            Save roles
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function TokenRow({
  token,
  accountName,
  busy,
  onRevoke,
  onRotate,
  onDelete,
  onEditRoles,
}: {
  token: ServiceAccountToken;
  accountName: string;
  busy: boolean;
  onRevoke: () => void;
  onRotate: () => void;
  onDelete: () => void;
  onEditRoles: () => void;
}) {
  const table = useStyles2(filterTableStyles);
  const styles = useStyles2(getStyles);
  const [menuOpen, setMenuOpen] = useState(false);

  const [confirmDelete, setConfirmDelete] = useState(false);
  const revoked = Boolean(token.revoked_at);
  const expired = Boolean(token.expires_at && token.expires_at < Date.now());
  const status = revoked ? 'Revoked' : expired ? 'Expired' : token.valid ? 'Active' : 'Inactive';
  const canRotate = !busy && !revoked && !expired;
  const canRevoke = !busy && !revoked;

  const details: Array<[string, string]> = [
    ['Prefix', token.key_prefix || '—'],
    ['Expires', token.expires_at ? formatDate(token.expires_at) : 'Never'],
    ['Created', formatDate(token.created_at)],
    ['Last used', token.last_used_at ? formatDateTime(token.last_used_at) : 'Never'],
  ];

  const act = (run: () => void) => () => {
    setMenuOpen(false);
    run();
  };

  return (
    <>
      <tr>
        <td className={table.cell}>
          <span className={table.ellipsis} title={token.name || undefined}>
            {token.name || '—'}
          </span>
        </td>
        <td className={table.cell}>
          <span className={table.ellipsis} title={describeRoles(token)}>
            {describeRoles(token)}
          </span>
        </td>
        <td className={table.cell}>
          <StatusBadge label={status} />
        </td>
        <td className={table.cell}>
          <div className={table.actions}>
            <IconButton
              name="more-horizontal"
              aria-label={`Actions for token ${token.name ?? ''}`.trim()}
              title="Token actions"
              variant="secondary"
              fill="text"
              size="sm"
              disabled={busy}
              onClick={() => setMenuOpen(true)}
            />
          </div>
        </td>
      </tr>

      <OverlaySheet
        open={menuOpen}
        onClose={() => setMenuOpen(false)}
        label={`Token ${token.name ?? ''}`.trim()}
        header={accountName ? `${token.name || 'Token'} · ${accountName}` : token.name || 'Token'}
      >
        <dl className={styles.tokenDetails}>
          {details.map(([label, value]) => (
            <div key={label} className={styles.tokenDetailRow}>
              <dt className={styles.tokenDetailLabel}>{label}</dt>
              <dd className={styles.tokenDetailValue}>{value}</dd>
            </div>
          ))}
        </dl>

        <div className={styles.tokenMenu}>
          <button type="button" className={styles.tokenMenuItem} onClick={act(onEditRoles)}>
            <Icon name="settings" size="lg" />
            Edit roles
          </button>
          <button
            type="button"
            className={styles.tokenMenuItem}
            disabled={!canRotate}
            onClick={act(onRotate)}
          >
            <Icon name="refresh-cw" size="lg" />
            Rotate
          </button>
          <button
            type="button"
            className={styles.tokenMenuItem}
            disabled={!canRevoke}
            onClick={act(onRevoke)}
          >
            <Icon name="x" size="lg" />
            Revoke
          </button>
          <button
            type="button"
            className={cx(styles.tokenMenuItem, styles.tokenMenuItemDanger)}
            disabled={busy}
            onClick={act(() => setConfirmDelete(true))}
          >
            <Icon name="trash" size="lg" />
            Delete
          </button>
        </div>
      </OverlaySheet>

      <ConfirmModal
        isOpen={confirmDelete}
        title="Delete token"
        body={`Delete ${token.name || 'this token'} permanently? It stops working immediately and its record is removed — this cannot be undone. To keep an audit trail, use Revoke instead.`}
        confirmText="Delete permanently"
        destructive
        isConfirming={busy}
        onDismiss={() => setConfirmDelete(false)}
        onConfirm={() => {
          setConfirmDelete(false);
          onDelete();
        }}
      />
    </>
  );
}

function CreateTokenModal({
  isOpen,
  accountId,
  accountName,
  onDismiss,
  onCreated,
}: {
  isOpen: boolean;
  accountId: string;
  accountName: string;
  onDismiss: () => void;
  onCreated: (secret: string) => void;
}) {
  const create = useCreateServiceAccountToken();
  const [name, setName] = useState('');
  const [expires, setExpires] = useState('');
  const [basicRole, setBasicRole] = useState('viewer');
  const [roles, setRoles] = useState<FixedRoleGrant[]>([]);

  const [nameError, setNameError] = useState('');

  const [nameErrorSignal, setNameErrorSignal] = useState(0);

  const close = () => {
    setName('');
    setExpires('');
    setBasicRole('viewer');
    setRoles([]);
    setNameError('');
    onDismiss();
  };

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    create.mutate(
      {
        id: accountId,
        data: {
          name: name || `${accountName}-token`,
          basic_role: basicRole || undefined,
          roles: roles.map((grant) => ({ role: grant.role, scope: grant.scope as FixedRoleScope })),
          expires_at: expires ? new Date(expires).toISOString() : undefined,
        },
      },
      {
        onSuccess: (result) => onCreated(result.secret ?? ''),
        onError: (error) => {
          const { message } = serverErrorInfo(error);
          if (isDuplicateResource(error)) {
            setNameError(message || 'That name is already used on this service account.');
            setNameErrorSignal((n) => n + 1);
          }
          notifyError('Could not create token', message || undefined);
        },
      },
    );
  };

  return (
    <Modal title="Add service account token" isOpen={isOpen} onDismiss={close} width={550} hideCloseButton>
      <form onSubmit={submit} className={formStyle}>
        <Field
          label="Display name"
          description="Name to easily identify the token."
          invalid={Boolean(nameError)}
          error={nameError || undefined}
        >
          <Input
            value={name}
            placeholder={`${accountName}-token`}
            invalid={Boolean(nameError)}
            invalidSignal={nameErrorSignal}
            aria-invalid={Boolean(nameError) || undefined}
            onChange={(event) => {
              setName(event.target.value);
              if (nameError) setNameError('');
            }}
            autoFocus
          />
        </Field>
        <Field label="Roles" description="What this token is allowed to reach.">
          <RolePicker
            basicRole={basicRole}
            onBasicRoleChange={setBasicRole}
            roles={roles}
            onRolesChange={setRoles}
          />
        </Field>
        <Field label="Expiration" description="Leave empty for a token that lives until revoked.">
          <Input type="date" value={expires} onChange={(event) => setExpires(event.target.value)} />
        </Field>
        <div className={formActionsStyle}>
          <Button type="button" variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" loading={create.isPending}>
            Generate token
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function SecretModal({ secret, onDismiss }: { secret: string; onDismiss: () => void }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
      notifySuccess('Copied to clipboard');
    } catch {
      notifyError('Could not copy', 'Select the token and copy it manually.');
    }
  };

  return (
    <Modal
      title="Service account token created"
      isOpen={secret !== ''}
      onDismiss={onDismiss}
      width={550}
      hideCloseButton
    >
      <div className={formStyle}>
        <Field
          label="Token"
          description="Copy the token now — you will not be able to see it again. Losing it means creating a new one."
        >
          <Input value={secret} readOnly onFocus={(event) => event.currentTarget.select()} />
        </Field>
        <div className={formActionsStyle}>
          <Button variant="secondary" onClick={onDismiss}>
            Close
          </Button>
          <Button variant="primary" onClick={copy}>
            {copied ? 'Copied' : 'Copy to clipboard'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function DeleteSection({ account, onDeleted }: { account: ServiceAccount; onDeleted: () => void }) {
  const styles = useStyles2(getStyles);
  const remove = useDeleteServiceAccount();
  const [confirm, setConfirm] = useState(false);
  const tokenCount = account.token_count ?? 0;

  return (
    <div>
      <h3 className={styles.pageHeading}>Delete</h3>
      <p className={styles.hint}>
        Deleting revokes {tokenCount} token{tokenCount === 1 ? '' : 's'} and cannot be undone.
      </p>
      <div className={styles.sectionFooter}>
        <Button variant="destructive" onClick={() => setConfirm(true)}>
          Delete service account
        </Button>
      </div>

      <ConfirmModal
        isOpen={confirm}
        title="Delete service account"
        body={`Delete ${account.name || 'this service account'}${
          tokenCount > 0 ? ` and its ${tokenCount} token${tokenCount === 1 ? '' : 's'}` : ''
        }? This cannot be undone.`}
        confirmText="Delete"
        destructive
        isConfirming={remove.isPending}
        onDismiss={() => setConfirm(false)}
        onConfirm={() =>
          remove.mutate(
            { id: account.id ?? '' },
            {
              onSuccess: () => {
                notifySuccess('Service account deleted');
                onDeleted();
              },
              onError: () => notifyError('Could not delete service account'),
            },
          )
        }
      />
    </div>
  );
}

const formStyle = css({ display: 'flex', flexDirection: 'column', gap: 16 });
const formActionsStyle = css({ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 });

const getStyles = (theme: VyzorixTheme) => ({
  stack: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  pageHeading: css({
    margin: `0 0 ${theme.spacing(2)}`,
    fontSize: theme.typography.h4.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  sectionHead: css({
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    marginBottom: theme.spacing(2),
    '& h3': { margin: 0 },
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
  actionCell: css({ textAlign: 'right', width: 200 }),
  actions: css({ display: 'flex', gap: theme.spacing(1), justifyContent: 'flex-end' }),
  sectionFooter: css({ marginTop: theme.spacing(1.5) }),
  identityRow: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1.5) }),
  identityAvatar: css({ borderRadius: theme.shape.radius.circle, display: 'block' }),
  disabledNotice: css({ marginTop: theme.spacing(1.5) }),
  disableForm: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  disableActions: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1) }),
  hint: css({ margin: 0, color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),

  tokenDetails: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    margin: `0 0 ${theme.spacing(2)}`,
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
  }),
  tokenDetailRow: css({
    display: 'flex',
    alignItems: 'baseline',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    minHeight: 28,
  }),
  tokenDetailLabel: css({
    margin: 0,
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    flexShrink: 0,
  }),
  tokenDetailValue: css({
    margin: 0,
    color: theme.colors.text.primary,
    fontSize: theme.typography.sm.fontSize,
    textAlign: 'right',
    minWidth: 0,
    overflowWrap: 'anywhere',
  }),
  tokenTable: css({
    tableLayout: 'fixed',

    '& th:nth-of-type(1), & td:nth-of-type(1)': { width: '38%' },
    '& th:nth-of-type(2), & td:nth-of-type(2)': { width: 'auto' },
    '& th:nth-of-type(3), & td:nth-of-type(3)': { width: 110 },
    '& th:nth-of-type(4), & td:nth-of-type(4)': { width: 48 },
  }),
  tokenMenu: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  tokenMenuItem: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.75),
    width: '100%',
    minHeight: 44,
    padding: theme.spacing(1, 1.5),
    border: 0,
    borderRadius: theme.shape.radius.default,
    background: 'transparent',
    color: theme.colors.text.primary,
    fontFamily: 'inherit',
    fontSize: theme.typography.base.fontSize,
    textAlign: 'left',
    cursor: 'pointer',
    '&:hover:not(:disabled)': { background: theme.colors.action.hover },
    '&:disabled': { color: theme.colors.action.disabledText, cursor: 'not-allowed' },
    '& svg': { color: 'currentColor' },
  }),
  tokenMenuItemDanger: css({
    color: theme.colors.error.text,
    '& svg': { color: 'currentColor' },
  }),
});
