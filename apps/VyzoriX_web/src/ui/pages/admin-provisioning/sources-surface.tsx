import { useState } from 'react';
import { css } from '@emotion/css';
import { Button, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import {
  useDropProvisioningCache,
  useProvisioningRepositories,
  useSyncProvisioningRepository,
  type ProvisioningRepository,
  type SyncJob,
  type ResourceStats,
} from '@/hooks/provisioning';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { SourceDialog } from './dialogs/source-dialog';
import { RowActionsMenu } from './row-actions-menu';
import { PreviewSheet } from './preview-sheet';
import { formatTimestamp } from '@/lib/preferences/format';
import { asQueryLike, emptyWhenList, queryView } from '@/lib/provisioning/query-view';
import { SearchInput } from '@/ui/components/shared/search-input';
import { filterTableStyles } from '@/ui/components/shared/table';
import {
  SourceListEmpty,
  SourceListError,
  SourceListLoading,
  SourceListNoMatches,
  SourceSyncBadge,
} from './states';

export function SourcesSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const table = useStyles2(filterTableStyles);
  const repositories = useProvisioningRepositories();
  const [search, setSearch] = useState('');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<ProvisioningRepository | undefined>();

  const view = queryView(asQueryLike(repositories), () => emptyWhenList(repositories.repositories));

  const openAdd = () => {
    setEditing(undefined);
    setDialogOpen(true);
  };

  const openEdit = (repo: ProvisioningRepository) => {
    setEditing(repo);
    setDialogOpen(true);
  };

  if (view.state === 'loading') return <SourceListLoading />;
  if (view.state === 'error') {
    return <SourceListError error={view.error} onRetry={() => void repositories.refetch()} />;
  }
  if (view.state === 'empty') {
    return (
      <>
        <SourceListEmpty onAdd={canWrite ? openAdd : undefined} />
        <SourceDialog isOpen={dialogOpen} onDismiss={() => setDialogOpen(false)} repository={editing} />
      </>
    );
  }

  const repos = repositories.repositories;
  const needle = search.trim().toLowerCase();
  const filtered = needle
    ? repos.filter(
        (repo) =>
          (repo.title || repo.name).toLowerCase().includes(needle) || repo.url.toLowerCase().includes(needle),
      )
    : repos;

  return (
    <div className={styles.wrap}>
      <div className={table.actionBar}>
        <div className={styles.search}>
          <SearchInput value={search} onChange={setSearch} placeholder="Search sources" />
        </div>
        <div className={table.actionBarSpacer} />
        <span className={table.muted}>
          {filtered.length} {filtered.length === 1 ? 'source' : 'sources'}
        </span>
        {canWrite ? (
          <Button variant="primary" size="sm" icon="plus" onClick={openAdd}>
            Add source
          </Button>
        ) : null}
      </div>

      {filtered.length === 0 ? (
        <SourceListNoMatches onClear={() => setSearch('')} />
      ) : (
        <div className={table.wrapper}>
          <table className={table.table}>
            <thead>
              <tr>
                <th className={table.headCell}>Source</th>
                <th className={table.headCell}>Sync</th>
                <th className={table.headCell}>Resources</th>
                <th className={table.headCell}>Last sync</th>
                <th className={table.headCell} />
              </tr>
            </thead>
            <tbody>
              {filtered.map((repo) => (
                <tr key={repo.id}>
                  <td className={table.cell}>
                    <div className={styles.nameCell}>
                      <span className={styles.name}>{repo.title || repo.name}</span>
                      <span className={styles.url} title={repo.url}>
                        {repo.branch} · {repo.url}
                      </span>
                    </div>
                  </td>
                  <td className={table.cell}>
                    <SourceSyncBadge state={repo.syncState} />
                  </td>
                  <td className={table.cell}>
                    <ResourceCounts stats={repo.stats} />
                  </td>
                  <td className={table.cell}>
                    {repo.lastSyncAt ? formatTimestamp(repo.lastSyncAt.getTime(), { relative: true }) : 'Never'}
                  </td>
                  <td className={table.cell}>
                    <SourceRowActions
                      repository={repo}
                      canWrite={canWrite}
                      onEdit={() => openEdit(repo)}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <SourceDialog isOpen={dialogOpen} onDismiss={() => setDialogOpen(false)} repository={editing} />
    </div>
  );
}

function SourceRowActions({
  repository,
  canWrite,
  onEdit,
}: {
  repository: ProvisioningRepository;
  canWrite: boolean;
  onEdit: () => void;
}) {
  const styles = useStyles2(getStyles);
  const sync = useSyncProvisioningRepository();
  const dropCache = useDropProvisioningCache();
  const [busy, setBusy] = useState<'dry' | 'apply' | null>(null);
  const [preview, setPreview] = useState<SyncJob | undefined>();

  const run = async (dryRun: boolean) => {
    setBusy(dryRun ? 'dry' : 'apply');
    try {
      const job = await sync.mutateAsync({ id: repository.id, params: { dry_run: dryRun } });

      if (dryRun) {

        setPreview(job);
      } else {

        void dropCache(repository.id);

        if (job.failed > 0) {
          notifyError('Sync finished with failures', `${job.applied} applied, ${job.failed} failed.`);
        } else {
          notifySuccess('Sync complete', `${job.applied} applied, ${job.skipped} unchanged.`);
        }
      }
    } catch (error) {
      notifyError('Sync failed', describeError(error));
    } finally {
      setBusy(null);
    }
  };

  if (!canWrite) return <span className={styles.muted}>—</span>;

  return (
    <div className={styles.actions}>
      <Button variant="primary" size="sm" onClick={() => void run(false)} loading={busy === 'apply'}>
        Sync
      </Button>
      <RowActionsMenu
        label={repository.title || repository.name}
        actions={[
          { label: 'Preview changes', icon: 'eye', onClick: () => void run(true), disabled: busy !== null },
          { label: 'Edit source', icon: 'pencil', onClick: onEdit },
        ]}
      />
      <PreviewSheet
        job={preview}
        repositoryName={repository.title || repository.name}
        onClose={() => setPreview(undefined)}
        onApply={() => {
          setPreview(undefined);
          void run(false);
        }}
        isApplying={busy === 'apply'}
      />
    </div>
  );
}

function ResourceCounts({ stats }: { stats: ResourceStats }) {
  const styles = useStyles2(getStyles);
  const total = stats.total ?? 0;
  if (total === 0) return <span className={styles.muted}>None</span>;

  return (
    <span className={styles.counts}>
      <span>{total} total</span>
      {stats.drifted ? <span className={styles.drifted}>{stats.drifted} drifted</span> : null}
      {stats.missing ? <span className={styles.muted}>{stats.missing} missing</span> : null}
      {stats.error ? <span className={styles.error}>{stats.error} failed</span> : null}
    </span>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  search: css({ flex: '0 1 420px', minWidth: 240 }),
  nameCell: css({ display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }),
  name: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  url: css({
    color: theme.colors.text.secondary,
    fontSize: theme.typography.sm.fontSize,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    maxWidth: 420,
    whiteSpace: 'nowrap',
  }),
  counts: css({ display: 'inline-flex', gap: theme.spacing(1), fontSize: theme.typography.sm.fontSize }),
  drifted: css({ color: theme.colors.warning.text }),
  error: css({ color: theme.colors.error.text }),
  muted: css({ color: theme.colors.text.secondary }),
  actions: css({ display: 'flex', gap: theme.spacing(1), justifyContent: 'flex-end' }),
});
