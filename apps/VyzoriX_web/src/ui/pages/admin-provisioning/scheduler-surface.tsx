import { css } from '@emotion/css';
import { Link } from '@tanstack/react-router';
import { Button, Icon, LoadingPlaceholder, Tag, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import { formatTimestamp } from '@/lib/preferences/format';
import { asQueryLike, emptyWhenList, queryView } from '@/lib/provisioning/query-view';
import { useProvisioningRepositories, useProvisioningScheduler } from '@/hooks/provisioning';
import { EmptySurface, StatusSurface } from './states';

export function SchedulerSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const scheduler = useProvisioningScheduler();
  const repositories = useProvisioningRepositories();

  const view = queryView(asQueryLike(scheduler), (data) => data === undefined);
  const reposView = queryView(asQueryLike(repositories), () => emptyWhenList(repositories.repositories));

  if (view.state === 'loading') return <LoadingPlaceholder text="Reading the scheduler…" />;
  if (view.state === 'error') {
    return (
      <StatusSurface
        icon="alert-triangle"
        tone="error"
        title="Could not read the scheduler"
        detail={describeError(view.error)}
        actions={
          <Button variant="secondary" size="sm" icon="refresh-cw" onClick={() => void scheduler.refetch()}>
            Try again
          </Button>
        }
      />
    );
  }

  const state = scheduler.data;
  const running = state?.running ?? false;
  const dueNow = state?.due_now ?? [];
  const enabled = repositories.repositories.filter((repo) => repo.enabled);

  return (
    <div className={styles.wrap}>
      <section className={styles.summary}>
        <span className={styles.stateIcon} data-running={running} aria-hidden="true">
          <Icon name={running ? 'calendar-clock' : 'pause'} size="xl" />
        </span>
        <div className={styles.summaryBody}>
          <p className={styles.summaryTitle}>
            {running ? 'The scheduler is running' : 'The scheduler is not running'}
          </p>
          <p className={styles.summaryText}>
            {running
              ? `Every ${state?.tick_seconds ?? 0} seconds it looks for sources whose interval has elapsed and syncs them. Nothing needs to be pressed.`
              : 'Sources only sync when you press Sync. Nothing will change on a timer until the server starts one.'}
          </p>
        </div>
      </section>

      {running ? (
        <section className={styles.figures}>
          <Figure label="Enabled sources" value={String(enabled.length)} />
          <Figure label="Due now" value={String(dueNow.length)} tone={dueNow.length > 0 ? 'warning' : undefined} />
          <Figure
            label="Next due"
            value={state?.next_due_at ? relativeFuture(state.next_due_at) : 'Nothing scheduled'}
          />
          <Figure label="Check interval" value={`${state?.tick_seconds ?? 0}s`} />
        </section>
      ) : null}

      {dueNow.length > 0 ? (
        <section className={styles.dueBlock}>
          <h3 className={styles.sectionTitle}>Due on the next tick</h3>
          <ul className={styles.dueList}>
            {dueNow.map((name) => (
              <li key={name} className={styles.dueRow}>
                <Icon name="clock" size="sm" />
                <span>{name}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <section className={styles.scheduleBlock}>
        <div className={styles.scheduleHead}>
          <h3 className={styles.sectionTitle}>The schedule, per source</h3>
          {canWrite ? (
            <Link to="/admin/provisioning" search={{ tab: 'sources' }} className={styles.link}>
              <Button variant="secondary" size="sm" icon="sliders">
                Change the schedule
              </Button>
            </Link>
          ) : null}
        </div>
        <p className={styles.scheduleHint}>
          Each source carries its own interval and on/off switch, so one repository can sync
          every minute while another waits an hour. Edit a source to change them.
        </p>

        {reposView.state === 'empty' ? (
          <EmptySurface
            illustration="sources"
            title="No sources to schedule"
            detail="Add a source first; its interval is what the scheduler acts on."
          />
        ) : (
          <ul className={styles.sourceList}>
            {repositories.repositories.map((repo) => (
              <li key={repo.id} className={styles.sourceRow}>
                <span className={styles.sourceName}>{repo.title || repo.name}</span>
                <Tag name={repo.enabled ? `Every ${repo.syncIntervalSeconds}s` : 'Manual only'} />
                <span className={styles.sourceMeta}>
                  {repo.enabled
                    ? repo.lastSyncAt
                      ? `Last synced ${formatTimestamp(repo.lastSyncAt.getTime(), { relative: true })}`
                      : 'Never synced; due immediately'
                    : 'Not scheduled'}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function relativeFuture(ms: number): string {
  const seconds = Math.round((ms - Date.now()) / 1000);
  if (seconds <= 0) return 'Due now';
  if (seconds < 60) return `in ${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `in ${minutes}m`;

  return `in ${Math.round(minutes / 60)}h`;
}

function Figure({ label, value, tone }: { label: string; value: string; tone?: 'warning' }) {
  const styles = useStyles2(getStyles, tone);

  return (
    <div className={styles.figure}>
      <span className={styles.figureValue}>{value}</span>
      <span className={styles.figureLabel}>{label}</span>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme, tone?: 'warning') => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3), maxWidth: 900 }),
  summary: css({
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(2),
    padding: theme.spacing(2.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.lg,
    background: theme.colors.background.primary,
  }),
  stateIcon: css({
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 44,
    height: 44,
    flexShrink: 0,
    borderRadius: theme.shape.radius.default,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    '[data-running="true"]': { color: theme.colors.success.text },
  }),
  summaryBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), minWidth: 0 }),
  summaryTitle: css({
    margin: 0,
    fontSize: theme.typography.md.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  summaryText: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.6,
    color: theme.colors.text.secondary,
  }),
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
  }),
  figureValue: css({
    fontSize: theme.typography.xl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: tone === 'warning' ? theme.colors.warning.text : theme.colors.text.primary,
    fontVariantNumeric: 'tabular-nums',
  }),
  figureLabel: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  dueBlock: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  dueList: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  dueRow: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
  }),
  scheduleBlock: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  scheduleHead: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: theme.spacing(2) }),
  sectionTitle: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  scheduleHint: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.65,
    color: theme.colors.text.secondary,
  }),
  link: css({ textDecoration: 'none' }),
  sourceList: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  sourceRow: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    flexWrap: 'wrap',
    padding: `${theme.spacing(1.25)} ${theme.spacing(1.5)}`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
  }),
  sourceName: css({
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  sourceMeta: css({ marginLeft: 'auto', fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
});
