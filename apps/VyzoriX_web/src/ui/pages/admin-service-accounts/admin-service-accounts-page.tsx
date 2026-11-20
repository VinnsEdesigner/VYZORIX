import { useState } from 'react';
import { css } from '@emotion/css';
import { useNavigate } from '@tanstack/react-router';
import {
  Button,
  EmptyState,
  Field,
  Input,
  LoadingPlaceholder,
  Modal,
  useStyles2,
} from '@vyzorix/ui';
import type { ServiceAccount } from '@vyzorix/api-client';
import { Page } from '@/ui/components/layout';
import { SearchInput } from '@/ui/components/shared/search-input';
import { StatusBadge } from '@/ui/components/shared/status-badge';
import { filterTableStyles } from '@/ui/components/shared/table';
import { useCreateServiceAccount, useServiceAccounts } from '@/hooks/service-accounts';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { formatDate } from '@/ui/pages/profile/profile-page';
import { serviceAccountAvatar } from '@/lib/avatar';

export function AdminServiceAccountsPage() {
  const styles = useStyles2(getStyles);
  const table = useStyles2(filterTableStyles);
  const navigate = useNavigate();
  const accounts = useServiceAccounts();
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);

  const rows = accounts.data?.service_accounts ?? [];
  const needle = search.trim().toLowerCase();
  const filtered = needle ? rows.filter((account) => (account.name ?? '').toLowerCase().includes(needle)) : rows;

  return (
    <Page
      title="Service accounts"
      subTitle="Non-human identities for automation and CI"
      actions={
        <Button variant="primary" onClick={() => setCreateOpen(true)}>
          Add service account
        </Button>
      }
    >
      <div className={table.actionBar}>
        <div className={styles.search}>
          <SearchInput value={search} onChange={setSearch} placeholder="Search service accounts" />
        </div>
        <div className={table.actionBarSpacer} />
        <span className={table.muted}>{filtered.length} service accounts</span>
      </div>

      {accounts.isLoading ? <LoadingPlaceholder text="Loading service accounts…" /> : null}

      {accounts.isError ? (
        <EmptyState icon="alert-triangle" message="Could not load service accounts." />
      ) : null}

      {!accounts.isLoading && !accounts.isError && rows.length === 0 ? (
        <EmptyState icon="key" message="You haven't created any service accounts yet">
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            Add service account
          </Button>
        </EmptyState>
      ) : null}

      {!accounts.isLoading && rows.length > 0 && filtered.length === 0 ? (
        <EmptyState icon="key" message="No service accounts found" />
      ) : null}

      {filtered.length > 0 ? (
        <div className={table.wrapper}>
          <table className={table.table}>
            <thead>
              <tr>
                <th className={table.headCell} />
                <th className={table.headCell}>Account</th>
                <th className={table.headCell}>ID</th>
                <th className={table.headCell}>Tokens</th>
                <th className={table.headCell}>Status</th>
                <th className={table.headCell}>Created</th>
                <th className={table.headCell} />
              </tr>
            </thead>
            <tbody>
              {filtered.map((account) => (
                <ServiceAccountRow
                  key={account.id}
                  account={account}
                  onOpen={() => {
                    if (account.id) {
                      void navigate({ to: '/admin/service-accounts/$id', params: { id: account.id } });
                    }
                  }}
                />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}

      <CreateServiceAccountModal isOpen={createOpen} onDismiss={() => setCreateOpen(false)} />
    </Page>
  );
}

function ServiceAccountRow({ account, onOpen }: { account: ServiceAccount; onOpen: () => void }) {
  const table = useStyles2(filterTableStyles);
  const tokenCount = account.token_count ?? 0;

  return (
    <tr>
      <td className={table.cell}>
        <img
          src={serviceAccountAvatar(account.id ?? account.name ?? '')}
          alt=""
          width={24}
          height={24}
          className={table.avatar}
        />
      </td>
      <td className={table.linkCell}>
        <button type="button" className={table.link} onClick={onOpen}>
          {account.name || '—'}
        </button>
      </td>
      <td className={table.cell}>
        <span className={table.ellipsis} title={account.id}>
          {shortId(account.id)}
        </span>
      </td>
      <td className={table.cell}>{tokenCount > 0 ? tokenCount : 'No tokens'}</td>
      <td className={table.cell}>
        <StatusBadge label={account.enabled ? 'Enabled' : 'Disabled'} />
      </td>
      <td className={table.cell}>{formatDate(account.created_at)}</td>
      <td className={table.cell}>
        <div className={table.actions}>
          <Button variant="secondary" size="sm" onClick={onOpen}>
            Manage
          </Button>
        </div>
      </td>
    </tr>
  );
}

function shortId(id: string | undefined): string {
  if (!id) return '—';
  return id.length > 12 ? `${id.slice(0, 8)}…` : id;
}

function CreateServiceAccountModal({ isOpen, onDismiss }: { isOpen: boolean; onDismiss: () => void }) {
  const create = useCreateServiceAccount();
  const navigate = useNavigate();
  const [name, setName] = useState('');

  const close = () => {
    setName('');
    onDismiss();
  };

  const submit = (event: React.FormEvent) => {
    event.preventDefault();
    create.mutate(
      { data: { name } },
      {
        onSuccess: (account) => {
          notifySuccess('Service account created', 'Add a token so it can authenticate.');
          close();
          if (account.id) {
            void navigate({ to: '/admin/service-accounts/$id', params: { id: account.id } });
          }
        },
        onError: () => notifyError('Could not create service account'),
      },
    );
  };

  return (
    <Modal title="Add service account" isOpen={isOpen} onDismiss={close} hideCloseButton>
      <form onSubmit={submit} className={formStyle}>
        <Field label="Display name" required description="What the account is for, e.g. “ci-deployer”.">
          <Input value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
        </Field>
        <div className={formActionsStyle}>
          <Button type="button" variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" loading={create.isPending}>
            Create
          </Button>
        </div>
      </form>
    </Modal>
  );
}

const formStyle = css({ display: 'flex', flexDirection: 'column', gap: 16 });
const formActionsStyle = css({ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 });

const getStyles = () => ({
  search: css({ flex: '0 1 420px', minWidth: 240 }),
});
