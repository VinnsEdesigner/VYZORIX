import { useMemo } from 'react';
import { css } from '@emotion/css';
import { Link, useNavigate } from '@tanstack/react-router';
import { useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { asQueryLike, emptyWhenList, queryView } from '@/lib/provisioning/query-view';
import {
  useProvisioningInstance,
  useProvisioningRepositories,
  type ProvisioningRepository,
} from '@/hooks/provisioning';
import { OverviewEmpty, OverviewError, OverviewLoading, SourceSyncBadge } from './states';

export function OverviewSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const navigate = useNavigate();
  const repositories = useProvisioningRepositories();
  const instance = useProvisioningInstance();

  const view = queryView(asQueryLike(repositories), () => emptyWhenList(repositories.repositories));
  const repos = useMemo(() => repositories.repositories, [repositories.repositories]);

  if (view.state === 'loading') return <OverviewLoading />;
  if (view.state === 'error') return <OverviewError error={view.error} onRetry={() => void repositories.refetch()} />;
  if (view.state === 'empty') {
    return (
      <OverviewEmpty
        onAdd={
          canWrite
            ? () => {
                void navigate({ to: '/admin/provisioning', search: { tab: 'providers' } });
              }
            : undefined
        }
      />
    );
  }

  const totals = tallyResources(repos);
  const broken = repos.filter((repo) => repo.syncState === 'error');
  const drifting = repos.filter((repo) => (repo.stats.drifted ?? 0) > 0);

  return (
    <div className={styles.wrap}>
      <section className={styles.figures} aria-label="Provisioning totals">
        <Figure label="Sources" value={instance.summary.repositories || repos.length} to="/admin/provisioning" tab="sources" />
        <Figure label="Resources" value={instance.summary.resources || totals.total} to="/admin/provisioning" tab="resources" />
        <Figure label="Credentials" value={instance.summary.connections} to="/admin/provisioning" tab="credentials" />
        <Figure
          label="Needing attention"
          value={broken.length + drifting.length}
          to="/admin/provisioning"
          tab="sources"
          tone={broken.length + drifting.length > 0 ? 'warning' : undefined}
        />
      </section>

      {broken.length > 0 ? (
        <section className={styles.section}>
          <h2 className={styles.heading}>Sources that are failing</h2>
          <ul className={styles.rows}>
            {broken.map((repo) => (
              <li key={repo.id} className={styles.row}>
                <Link to="/admin/provisioning" search={{ tab: 'sources' }} className={styles.rowName}>
                  {repo.title || repo.name}
                </Link>
                <SourceSyncBadge state={repo.syncState} />
                <span className={styles.reason}>{repo.lastError || 'The last sync failed.'}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {drifting.length > 0 ? (
        <section className={styles.section}>
          <h2 className={styles.heading}>Devices that drifted from their source</h2>
          <ul className={styles.rows}>
            {drifting.map((repo) => (
              <li key={repo.id} className={styles.row}>
                <Link to="/admin/provisioning" search={{ tab: 'resources' }} className={styles.rowName}>
                  {repo.title || repo.name}
                </Link>
                <span className={styles.reason}>
                  {repo.stats.drifted} {repo.stats.drifted === 1 ? 'resource has' : 'resources have'} drifted
                </span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {broken.length === 0 && drifting.length === 0 ? (
        <p className={styles.allGood}>Every source is syncing, and nothing has drifted.</p>
      ) : null}
    </div>
  );
}

function Figure({
  label,
  value,
  to,
  tab,
  tone,
}: {
  label: string;
  value: number;
  to: '/admin/provisioning';
  tab: string;
  tone?: 'warning';
}) {
  const styles = useStyles2(getStyles, tone);

  return (
    <Link to={to} search={{ tab }} className={styles.figure}>
      <span className={styles.figureValue}>{value}</span>
      <span className={styles.figureLabel}>{label}</span>
    </Link>
  );
}

function tallyResources(repos: ProvisioningRepository[]) {
  return repos.reduce(
    (acc, repo) => ({
      total: acc.total + (repo.stats.total ?? 0),
      drifted: acc.drifted + (repo.stats.drifted ?? 0),
    }),
    { total: 0, drifted: 0 },
  );
}

const getStyles = (theme: VyzorixTheme, tone?: 'warning') => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  figures: css({
    display: 'grid',
    gap: theme.spacing(1.5),
    gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
  }),
  figure: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
    padding: theme.spacing(2),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.primary,
    textDecoration: 'none',
    '&:hover': { borderColor: theme.colors.border.medium },
  }),
  figureValue: css({
    fontSize: theme.typography.xxl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: tone === 'warning' ? theme.colors.warning.text : theme.colors.text.primary,
    fontVariantNumeric: 'tabular-nums',
  }),
  figureLabel: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  heading: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  rows: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  row: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
  }),
  rowName: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium, textDecoration: 'none' }),
  reason: css({ color: theme.colors.text.secondary, fontSize: theme.typography.sm.fontSize }),
  allGood: css({ margin: 0, color: theme.colors.success.text, fontSize: theme.typography.sm.fontSize }),
});
