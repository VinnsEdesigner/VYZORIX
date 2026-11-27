import { css } from '@emotion/css';
import { Button, Icon, OverlaySheet, Tag, useStyles2, type IconName, type VyzorixTheme } from '@vyzorix/ui';
import type { SyncJob } from '@/hooks/provisioning';

interface PreviewSheetProps {
  job: SyncJob | undefined;
  repositoryName: string;
  onClose: () => void;
  onApply: () => void;
  isApplying: boolean;
}

export function PreviewSheet({ job, repositoryName, onClose, onApply, isApplying }: PreviewSheetProps) {
  const styles = useStyles2(getStyles);
  const results = job?.results ?? [];
  const wouldFail = results.filter((result) => result.outcome === 'failed').length;

  return (
    <OverlaySheet
      open={job !== undefined}
      onClose={onClose}
      label={`Preview of ${repositoryName}`}
      header={
        <div className={styles.header}>
          <span className={styles.headerTitle}>Preview: {repositoryName}</span>
          <span className={styles.headerHint}>Nothing has been written.</span>
        </div>
      }
    >
      {job ? (
        <div className={styles.body}>
          <div className={styles.counts}>
            <Count label="Files to apply" value={job.applied} tone="success" />
            <Count label="Files unchanged" value={job.skipped} tone="neutral" />
            <Count label="Files to fail" value={job.failed} tone={job.failed > 0 ? 'error' : 'neutral'} />
          </div>

          {results.length === 0 ? (
            <p className={styles.empty}>This source declares nothing a sync would act on.</p>
          ) : (
            <ul className={styles.list}>
              {results.map((result) => {
                const { icon, tone } = outcomeVisual(result.outcome);

                return (
                  <li key={`${result.path}-${result.outcome}`} className={styles.row}>
                    <span className={styles.rowIcon} data-tone={tone}>
                      <Icon name={icon} size="sm" />
                    </span>
                    <span className={styles.rowBody}>
                      <span className={styles.rowPath}>{result.path}</span>
                      {result.message ? <span className={styles.rowMessage}>{result.message}</span> : null}
                    </span>
                    <Tag name={outcomeLabel(result.outcome)} />
                  </li>
                );
              })}
            </ul>
          )}

          <div className={styles.actions}>
            <Button variant="secondary" onClick={onClose}>
              Close
            </Button>
            <Button variant="primary" onClick={onApply} loading={isApplying} disabled={wouldFail > 0}>
              {wouldFail > 0 ? 'Fix the failures first' : 'Apply for real'}
            </Button>
          </div>
        </div>
      ) : null}
    </OverlaySheet>
  );
}

function Count({ label, value, tone }: { label: string; value: number; tone: 'success' | 'neutral' | 'error' }) {
  const styles = useStyles2(getStyles, tone);

  return (
    <div className={styles.count}>
      <span className={styles.countValue}>{value}</span>
      <span className={styles.countLabel}>{label}</span>
    </div>
  );
}

function outcomeVisual(outcome: string): { icon: IconName; tone: 'success' | 'neutral' | 'error' | 'warning' } {
  switch (outcome) {
    case 'applied':
    case 'updated':
      return { icon: 'check-circle', tone: 'success' };
    case 'failed':
      return { icon: 'x-circle', tone: 'error' };
    case 'missing':
      return { icon: 'minus', tone: 'warning' };
    case 'ignored':
    case 'skipped':
      return { icon: 'eye-off', tone: 'neutral' };
    default:
      return { icon: 'check', tone: 'neutral' };
  }
}

function outcomeLabel(outcome: string): string {
  switch (outcome) {
    case 'applied':
      return 'Would apply';
    case 'updated':
      return 'Would update';
    case 'unchanged':
      return 'Unchanged';
    case 'missing':
      return 'Missing';
    case 'failed':
      return 'Would fail';
    case 'ignored':
      return 'Out of scope';
    case 'skipped':
      return 'Skipped';
    default:
      return outcome;
  }
}

const getStyles = (theme: VyzorixTheme, tone?: 'success' | 'neutral' | 'error') => ({
  header: css({ display: 'flex', flexDirection: 'column', gap: 2 }),
  headerTitle: css({
    fontSize: theme.typography.md.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  headerHint: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  body: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2), padding: theme.spacing(1.5) }),
  counts: css({ display: 'grid', gap: theme.spacing(1), gridTemplateColumns: 'repeat(3, 1fr)' }),
  count: css({
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    padding: theme.spacing(1.5),
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
    background: theme.colors.background.secondary,
  }),
  countValue: css({
    fontSize: theme.typography.xl.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color:
      tone === 'success'
        ? theme.colors.success.text
        : tone === 'error'
          ? theme.colors.error.text
          : theme.colors.text.primary,
    fontVariantNumeric: 'tabular-nums',
  }),
  countLabel: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  list: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  row: css({
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(1.5),
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    borderRadius: theme.shape.radius.default,
    border: `1px solid ${theme.colors.border.weak}`,
  }),
  rowIcon: css({
    display: 'flex',
    paddingTop: 2,
    '[data-tone="success"]': { color: theme.colors.success.text },
    '[data-tone="error"]': { color: theme.colors.error.text },
    '[data-tone="warning"]': { color: theme.colors.warning.text },
    '[data-tone="neutral"]': { color: theme.colors.text.secondary },
  }),
  rowBody: css({ display: 'flex', flexDirection: 'column', gap: 2, flex: 1, minWidth: 0 }),
  rowPath: css({
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
    overflowWrap: 'anywhere',
  }),
  rowMessage: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  empty: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  actions: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1), paddingTop: theme.spacing(1) }),
});
