import { css } from '@emotion/css';
import { Button, LoadingPlaceholder, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import { formatTimestamp } from '@/lib/preferences/format';
import { useRepositoryHistory } from '@/hooks/provisioning';
import { EmptySurface, StatusSurface } from './states';

export function RepoHistory({ repositoryId }: { repositoryId: string }) {
  const styles = useStyles2(getStyles);
  const history = useRepositoryHistory(repositoryId, 20);

  if (history.isPending) return <LoadingPlaceholder text="Reading the commit history…" />;

  if (history.isError) {
    return (
      <StatusSurface
        icon="alert-triangle"
        tone="error"
        title="Could not read the commit history"
        detail={describeError(history.error)}
        actions={
          <Button variant="secondary" size="sm" icon="refresh-cw" onClick={() => void history.refetch()}>
            Try again
          </Button>
        }
      />
    );
  }

  if (history.history.commits.length === 0) {
    return (
      <EmptySurface
        illustration="resources"
        title="No commits yet"
        detail="This branch has no history VYZORIX can read."
      />
    );
  }

  return (
    <div className={styles.wrap}>
      <p className={styles.hint}>
        The last {history.history.commits.length} commits, newest first. The read covers the most
        recent {history.history.depth || 50}.
      </p>
      <ul className={styles.list}>
        {history.history.commits.map((commit) => (
          <li key={commit.sha} className={styles.row}>
            <span className={styles.message}>{commit.message}</span>
            <span className={styles.meta}>
              {commit.author}
              {commit.when ? ` · ${formatTimestamp(commit.when.getTime(), { relative: true })}` : ''}
              {' · '}
              <code className={styles.sha}>{commit.sha.slice(0, 8)}</code>
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  hint: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  list: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  row: css({
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
  }),
  message: css({
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
    overflowWrap: 'anywhere',
  }),
  meta: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  sha: css({
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
  }),
});
