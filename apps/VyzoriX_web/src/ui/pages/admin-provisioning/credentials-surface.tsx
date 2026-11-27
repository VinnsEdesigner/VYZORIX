import { useState } from 'react';
import { css } from '@emotion/css';
import { Button, ConfirmModal, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import {
  providerLabel,
  useCheckProvisioningConnection,
  useDeleteProvisioningConnection,
  useProvisioningConnections,
  type ProvisioningConnection,
} from '@/hooks/provisioning';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { CredentialDialog } from './dialogs/credential-dialog';
import { RowActionsMenu } from './row-actions-menu';
import { formatTimestamp } from '@/lib/preferences/format';
import { asQueryLike, emptyWhenList, queryView } from '@/lib/provisioning/query-view';
import { SearchInput } from '@/ui/components/shared/search-input';
import { filterTableStyles } from '@/ui/components/shared/table';
import {
  CredentialError,
  CredentialListEmpty,
  CredentialListError,
  CredentialListLoading,
  CredentialPending,
  CredentialReady,
} from './states';

export function CredentialsSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const table = useStyles2(filterTableStyles);
  const connections = useProvisioningConnections();
  const [search, setSearch] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<ProvisioningConnection | undefined>();
  const [deleting, setDeleting] = useState<ProvisioningConnection | undefined>();

  const view = queryView(asQueryLike(connections), () => emptyWhenList(connections.connections));

  const openAdd = () => {
    setEditing(undefined);
    setDialogOpen(true);
  };

  const openEdit = (connection: ProvisioningConnection) => {
    setEditing(connection);
    setDialogOpen(true);
  };

  if (view.state === 'loading') return <CredentialListLoading />;
  if (view.state === 'error') {
    return <CredentialListError error={view.error} onRetry={() => void connections.refetch()} />;
  }
  if (view.state === 'empty') {
    return (
      <>
        <CredentialListEmpty onAdd={canWrite ? openAdd : undefined} />
        <CredentialDialog isOpen={dialogOpen} onDismiss={() => setDialogOpen(false)} connection={editing} />
      </>
    );
  }

  const all = connections.connections;
  const needle = search.trim().toLowerCase();
  const filtered = needle
    ? all.filter((conn) => conn.name.toLowerCase().includes(needle) || conn.host.toLowerCase().includes(needle))
    : all;

  return (
    <div className={styles.wrap}>
      <div className={table.actionBar}>
        <div className={styles.search}>
          <SearchInput value={search} onChange={setSearch} placeholder="Search credentials" />
        </div>
        <div className={table.actionBarSpacer} />
        <span className={table.muted}>
          {filtered.length} {filtered.length === 1 ? 'credential' : 'credentials'}
        </span>
        {canWrite ? (
          <Button variant="primary" size="sm" icon="plus" onClick={openAdd}>
            Add credential
          </Button>
        ) : null}
      </div>

      <div className={table.wrapper}>
        <table className={table.table}>
          <thead>
            <tr>
              <th className={table.headCell}>Name</th>
              <th className={table.headCell}>Provider</th>
              <th className={table.headCell}>Health</th>
              <th className={table.headCell}>Access</th>
              <th className={table.headCell}>Checked</th>
              <th className={table.headCell} />
            </tr>
          </thead>
          <tbody>
            {filtered.map((conn) => (
              <tr key={conn.id}>
                <td className={table.cell}>
                  <div className={styles.nameCell}>
                    <span className={styles.name}>{conn.name}</span>
                    {conn.host ? <span className={styles.host}>{conn.host}</span> : null}
                  </div>
                </td>
                <td className={table.cell}>{providerLabel(conn.type)}</td>
                <td className={table.cell}>
                  {conn.status === 'ready' ? <CredentialReady host={conn.host} /> : null}
                  {conn.status === 'error' ? <CredentialError reason={conn.lastError} /> : null}
                  {conn.status === 'pending' ? <CredentialPending /> : null}
                </td>
                <td className={table.cell}>{conn.writeEnabled ? 'Read and write' : 'Read only'}</td>
                <td className={table.cell}>
                  {conn.checkedAt ? formatTimestamp(conn.checkedAt.getTime(), { relative: true }) : 'Never'}
                </td>
                <td className={table.cell}>
                  {canWrite ? (
                    <CredentialRowActions
                      connection={conn}
                      onEdit={() => openEdit(conn)}
                      onDelete={() => setDeleting(conn)}
                    />
                  ) : (
                    <span className={styles.host}>—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <CredentialDialog isOpen={dialogOpen} onDismiss={() => setDialogOpen(false)} connection={editing} />
      <DeleteCredentialModal connection={deleting} onDismiss={() => setDeleting(undefined)} />
    </div>
  );
}

function CredentialRowActions({
  connection,
  onEdit,
  onDelete,
}: {
  connection: ProvisioningConnection;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const styles = useStyles2(getStyles);
  const check = useCheckProvisioningConnection();

  const runCheck = async () => {
    try {
      const result = await check.mutateAsync({ id: connection.id });
      if (result.status === 'error') {
        notifyError('Check failed', result.last_error || 'The host rejected the token.');
      } else {
        notifySuccess('Check passed', `Reached ${result.host || 'the host'}.`);
      }
    } catch (error) {
      notifyError('Could not run the check', describeError(error));
    }
  };

  return (
    <div className={styles.actions}>
      <Button variant="secondary" size="sm" onClick={() => void runCheck()} loading={check.isPending}>
        Check now
      </Button>
      <RowActionsMenu
        label={connection.name}
        actions={[
          { label: 'Edit credential', icon: 'pencil', onClick: onEdit },
          { label: 'Delete credential', icon: 'trash', onClick: onDelete, destructive: true },
        ]}
      />
    </div>
  );
}

function DeleteCredentialModal({
  connection,
  onDismiss,
}: {
  connection: ProvisioningConnection | undefined;
  onDismiss: () => void;
}) {
  const remove = useDeleteProvisioningConnection();

  const confirm = async () => {
    if (!connection) return;
    try {
      await remove.mutateAsync({ id: connection.id });
      notifySuccess('Credential deleted');
      onDismiss();
    } catch (error) {

      notifyError('Could not delete the credential', describeError(error));
    }
  };

  return (
    <ConfirmModal
      title="Delete credential"
      body={
        connection
          ? `Delete “${connection.name}”? Any source still using it must be re-pointed first.`
          : ''
      }
      confirmText="Delete"
      destructive
      isConfirming={remove.isPending}
      isOpen={Boolean(connection)}
      onConfirm={() => void confirm()}
      onDismiss={onDismiss}
    />
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  search: css({ flex: '0 1 420px', minWidth: 240 }),
  nameCell: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  name: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  host: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  actions: css({ display: 'flex', gap: theme.spacing(1), justifyContent: 'flex-end' }),
});
