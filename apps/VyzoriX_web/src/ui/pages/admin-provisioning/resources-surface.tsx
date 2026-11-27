import { useMemo, useState } from 'react';
import { css } from '@emotion/css';
import { Button, Combobox, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import {
  RESOURCE_KINDS,
  resourceKindLabel,
  useProvisioningRepositories,
  useProvisioningResources,
  type ProvisionedResource,
  type ResourceKind,
} from '@/hooks/provisioning';
import { formatTimestamp } from '@/lib/preferences/format';
import { asQueryLike, emptyWhenList, queryView } from '@/lib/provisioning/query-view';
import { filterTableStyles } from '@/ui/components/shared/table';
import {
  DevicePresenceLine,
  ResourceListEmpty,
  ResourceListError,
  ResourceListLoading,
  ResourceListNoMatches,
  ResourceStateBadge,
} from './states';
import { ResourceDetail } from './resource-detail';

export function ResourcesSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const repositories = useProvisioningRepositories();
  const repos = useMemo(() => repositories.repositories, [repositories.repositories]);
  const [sourceId, setSourceId] = useState<string>('');
  const [kind, setKind] = useState<ResourceKind | 'all'>('all');
  const [selected, setSelected] = useState<ProvisionedResource | undefined>();

  const activeId = sourceId || repos[0]?.id || '';
  const resources = useProvisioningResources(activeId || undefined);

  const sourceView = queryView(asQueryLike(repositories), () => emptyWhenList(repositories.repositories));

  if (sourceView.state === 'loading') return <ResourceListLoading />;
  if (sourceView.state === 'error') {
    return <ResourceListError error={sourceView.error} onRetry={() => void repositories.refetch()} />;
  }
  if (sourceView.state === 'empty') return <ResourceListEmpty />;

  const view = queryView(asQueryLike(resources), () => emptyWhenList(resources.resources));
  const activeRepo = repos.find((repo) => repo.id === activeId);

  if (selected) {
    return (
      <ResourceDetail
        repositoryId={activeId}
        resource={selected}
        canWrite={canWrite}
        repositorySyncing={activeRepo?.syncState === 'working'}
        onClose={() => setSelected(undefined)}
      />
    );
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.controls}>
        <div className={styles.control}>
          <span className={styles.controlLabel} id="source-filter-label">
            Source
          </span>
          <div className={styles.combobox}>
            <Combobox
              options={repos.map((repo) => ({ value: repo.id, label: repo.title || repo.name }))}
              value={activeId || null}
              onChange={(next) => setSourceId(next ?? '')}
              aria-label="Source"
            />
          </div>
        </div>

        <div className={styles.control}>
          <span className={styles.controlLabel} id="kind-filter-label">
            Kind
          </span>
          <div className={styles.combobox}>
            <Combobox
              options={[
                { value: 'all', label: 'All kinds' },
                ...RESOURCE_KINDS.map((value) => ({ value, label: resourceKindLabel(value) })),
              ]}
              value={kind}
              onChange={(next) => setKind((next as ResourceKind | 'all' | null) ?? 'all')}
              aria-label="Kind"
            />
          </div>
        </div>
      </div>

      {view.state === 'loading' ? <ResourceListLoading /> : null}
      {view.state === 'error' ? (
        <ResourceListError error={view.error} onRetry={() => void resources.refetch()} />
      ) : null}
      {view.state === 'empty' ? <ResourceListEmpty /> : null}
      {view.state === 'ready' ? (
        <ResourceTable
          resources={resources.resources}
          kind={kind}
          onClearKind={() => setKind('all')}
          onOpen={setSelected}
        />
      ) : null}
    </div>
  );
}

function ResourceTable({
  resources,
  kind,
  onClearKind,
  onOpen,
}: {
  resources: ProvisionedResource[];
  kind: ResourceKind | 'all';
  onClearKind: () => void;
  onOpen: (resource: ProvisionedResource) => void;
}) {
  const table = useStyles2(filterTableStyles);
  const styles = useStyles2(getStyles);
  const filtered = kind === 'all' ? resources : resources.filter((resource) => resource.kind === kind);

  if (filtered.length === 0) return <ResourceListNoMatches onClear={onClearKind} />;

  return (
    <div className={table.wrapper}>
      <table className={table.table}>
        <thead>
          <tr>
            <th className={table.headCell}>Resource</th>
            <th className={table.headCell}>Kind</th>
            <th className={table.headCell}>State</th>
            <th className={table.headCell}>Device</th>
            <th className={table.headCell}>Version</th>
            <th className={table.headCell}>Applied</th>
            <th className={table.headCell} />
          </tr>
        </thead>
        <tbody>
          {filtered.map((resource) => (
            <tr key={resource.id}>
              <td className={table.cell}>
                <div className={styles.nameCell}>
                  <span className={styles.name}>{resource.deviceId || resource.path}</span>
                  <span className={styles.path}>{resource.path}</span>
                </div>
              </td>
              <td className={table.cell}>{resourceKindLabel(resource.kind)}</td>
              <td className={table.cell}>
                <ResourceStateBadge state={resource.state} />
              </td>
              <td className={table.cell}>
                {resource.deviceId ? <DevicePresenceLine online={resource.deviceOnline} /> : '—'}
              </td>
              <td className={table.cell}>v{resource.configVersion}</td>
              <td className={table.cell}>
                {resource.lastAppliedAt ? formatTimestamp(resource.lastAppliedAt.getTime(), { relative: true }) : '—'}
              </td>
              <td className={table.cell}>
                <Button variant="secondary" size="sm" fill="text" onClick={() => onOpen(resource)}>
                  View
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  controls: css({ display: 'flex', gap: theme.spacing(2), flexWrap: 'wrap' }),
  control: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  controlLabel: css({
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  combobox: css({ minWidth: 220 }),
  nameCell: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  name: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  path: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
});
