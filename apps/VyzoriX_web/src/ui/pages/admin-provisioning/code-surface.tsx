import { useMemo, useState } from 'react';
import { css } from '@emotion/css';
import { Combobox, LoadingPlaceholder, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { useProvisioningRepositories } from '@/hooks/provisioning';
import { EmptySurface } from './states';
import { RepoBrowser } from './repo-browser';
import { RepoHistory } from './repo-history';

export function CodeSurface({ canWrite }: { canWrite: boolean }) {
  const styles = useStyles2(getStyles);
  const repositories = useProvisioningRepositories();
  const repos = useMemo(() => repositories.repositories, [repositories.repositories]);
  const [sourceId, setSourceId] = useState('');
  const [view, setView] = useState<'files' | 'history'>('files');

  const activeId = sourceId || repos[0]?.id || '';
  const active = repos.find((repo) => repo.id === activeId);

  if (repositories.isPending) return <LoadingPlaceholder text="Loading sources…" />;

  if (repos.length === 0) {
    return (
      <EmptySurface
        illustration="sources"
        title="No source to browse"
        detail="Add a source first; its repository is what this tab reads."
      />
    );
  }

  return (
    <div className={styles.wrap}>
      <div className={styles.controls}>
        <div className={styles.control}>
          <span className={styles.controlLabel}>Source</span>
          <div className={styles.combobox}>
            <Combobox
              options={repos.map((repo) => ({ value: repo.id, label: repo.title || repo.name }))}
              value={activeId || null}
              onChange={(next) => setSourceId(next ?? '')}
              aria-label="Source"
            />
          </div>
        </div>

        <div className={styles.tabs} role="tablist" aria-label="Repository view">
          <button
            type="button"
            role="tab"
            aria-selected={view === 'files'}
            className={view === 'files' ? styles.tabActive : styles.tab}
            onClick={() => setView('files')}
          >
            Files
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={view === 'history'}
            className={view === 'history' ? styles.tabActive : styles.tab}
            onClick={() => setView('history')}
          >
            History
          </button>
        </div>
      </div>

      {active ? (
        view === 'files' ? (
          <RepoBrowser
            key={active.id}
            repositoryId={active.id}
            repositoryName={active.title || active.name}
            canWrite={canWrite}
          />
        ) : (
          <RepoHistory key={active.id} repositoryId={active.id} />
        )
      ) : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  controls: css({ display: 'flex', alignItems: 'flex-end', gap: theme.spacing(2), flexWrap: 'wrap' }),
  control: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  controlLabel: css({
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    fontWeight: theme.typography.fontWeightMedium,
  }),
  combobox: css({ minWidth: 240 }),
  tabs: css({ display: 'flex', gap: 2, marginLeft: 'auto' }),
  tab: css({
    padding: `${theme.spacing(0.75)} ${theme.spacing(1.5)}`,
    background: 'transparent',
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    font: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
  }),
  tabActive: css({
    padding: `${theme.spacing(0.75)} ${theme.spacing(1.5)}`,
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.medium}`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.primary,
    font: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    cursor: 'pointer',
  }),
});
