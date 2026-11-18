import { useState } from 'react';
import { useStyles2 } from '@vyzorix/ui';
import { listPageStyles } from '@/ui/styles/list-page';
import { createFileRoute } from '@tanstack/react-router';
import {
  getServiceAccounts,
  type ServiceAccount,
} from '@vyzorix/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCurrentOrganizationId } from '@/hooks/_shared/use-current-context';
import { Page } from '@/ui/components/layout';

const saKeys = {
  list: (orgId: string | null) => ['service-accounts', 'list', orgId] as const,
  tokens: (orgId: string | null, saId: string) => ['service-accounts', 'tokens', orgId, saId] as const,
};

function useServiceAccounts() {
  const organizationId = useCurrentOrganizationId();
  return useQuery({
    queryKey: saKeys.list(organizationId),
    queryFn: async () =>
      (await getServiceAccounts().getServiceAccounts()).service_accounts ?? [],
    enabled: organizationId !== null,
    refetchInterval: 60_000,
  });
}

function ServiceAccountCard({ sa }: { sa: ServiceAccount }) {
  const styles = useStyles2(listPageStyles);
  const queryClient = useQueryClient();
  const deleteAccount = useMutation({
    mutationFn: () => getServiceAccounts().deleteServiceAccountsId(sa.id ?? ''),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['service-accounts'] }),
  });
  const rotateToken = useMutation({
    mutationFn: () => getServiceAccounts().postServiceAccountsIdRotate(sa.id ?? '', {}),
  });

  return (
    <div className={styles.card}>
      <div className={styles.row}>
        <div>
          <h3 className={styles.h3}>{sa.name}</h3>
          <p className={styles.meta}>ID: {(sa.id ?? '').slice(0, 8)}…</p>
        </div>
        <div className={styles.rowActions}>
          <span
            className={styles.badge}
            style={{ backgroundColor: `${sa.enabled ? '#22c55e' : '#6b7280'}20`, color: sa.enabled ? '#22c55e' : '#6b7280' }}
          >
            {sa.enabled ? 'active' : 'disabled'}
          </span>
          <button
            onClick={() => rotateToken.mutate()}
            className={styles.smallBtn}
            title="Rotate a token"
          >
            Rotate
          </button>
          <button
            onClick={() => deleteAccount.mutate()}
            className={styles.smallBtnDanger}
            title="Delete"
          >
            Delete
          </button>
        </div>
      </div>
    </div>
  );
}

function ServiceAccountsPage() {
  const styles = useStyles2(listPageStyles);
  const queryClient = useQueryClient();
  const accounts = useServiceAccounts();
  const [name, setName] = useState('');

  const createAccount = useMutation({
    mutationFn: () => getServiceAccounts().postServiceAccounts({ name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['service-accounts'] });
      setName('');
    },
  });

  return (
    <Page title="Service accounts" subTitle="Non-human credentials for CI and automation">

      <div className={styles.panel}>
        <h2 className={styles.h2}>New service account</h2>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            createAccount.mutate();
          }}
          className={styles.rowFlex}
        >
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Account name (e.g., CI deployer)"
            required
            className={styles.flex1}
          />
          <button
            type="submit"
            disabled={createAccount.isPending}
            className={styles.submit}
          >
            Create
          </button>
        </form>
      </div>

      {accounts.isLoading && <p>Loading service accounts…</p>}
      {accounts.isError && <p className={styles.error}>Failed to load service accounts.</p>}

      <div className={styles.stack}>
        {(accounts.data ?? []).map((sa) => (
          <ServiceAccountCard key={sa.id} sa={sa} />
        ))}
        {(accounts.data ?? []).length === 0 && (
          <p className={styles.meta}>No service accounts yet. Create one above.</p>
        )}
      </div>
    </Page>
  );
}

export const Route = createFileRoute('/admin/service-accounts/')({
  component: ServiceAccountsPage,
});
