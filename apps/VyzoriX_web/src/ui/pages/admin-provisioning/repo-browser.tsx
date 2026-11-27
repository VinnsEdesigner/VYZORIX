import { useState } from 'react';
import { css } from '@emotion/css';
import { Button, Icon, LoadingPlaceholder, Tag, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import { formatTimestamp } from '@/lib/preferences/format';
import { pathCrumbs, parentDir, type RepoFile } from '@/hooks/provisioning';
import { useProvisioningResources, useRepositoryFile, useRepositoryTree } from '@/hooks/provisioning';
import { EmptySurface, StatusSurface } from './states';
import { CodeBlock } from '@/ui/components/shared/code-block';
import { RepoFileEditor } from './repo-file-editor';

interface RepoBrowserProps {
  repositoryId: string;
  repositoryName: string;
  canWrite: boolean;
}

export function RepoBrowser({ repositoryId, repositoryName, canWrite }: RepoBrowserProps) {
  const [dir, setDir] = useState('');
  const [openPath, setOpenPath] = useState<string | undefined>();
  const [editing, setEditing] = useState(false);

  if (openPath) {
    return (
      <FilePane
        repositoryId={repositoryId}
        path={openPath}
        canWrite={canWrite}
        editing={editing}
        onEdit={() => setEditing(true)}
        onCancelEdit={() => setEditing(false)}
        onClose={() => {
          setEditing(false);
          setOpenPath(undefined);
        }}
      />
    );
  }

  return <TreePane repositoryId={repositoryId} repositoryName={repositoryName} dir={dir} onDir={setDir} onOpen={setOpenPath} />;
}

function TreePane({
  repositoryId,
  repositoryName,
  dir,
  onDir,
  onOpen,
}: {
  repositoryId: string;
  repositoryName: string;
  dir: string;
  onDir: (dir: string) => void;
  onOpen: (path: string) => void;
}) {
  const styles = useStyles2(getStyles);
  const tree = useRepositoryTree(repositoryId, dir);
  const crumbs = pathCrumbs(dir);

  return (
    <div className={styles.wrap}>
      <nav className={styles.crumbs} aria-label="Path">
        <button type="button" className={styles.crumb} onClick={() => onDir('')}>
          {repositoryName}
        </button>
        {crumbs.map((crumb) => (
          <span key={crumb.path} className={styles.crumbGroup}>
            <Icon name="chevron-right" size="xs" />
            <button type="button" className={styles.crumb} onClick={() => onDir(crumb.path)}>
              {crumb.label}
            </button>
          </span>
        ))}
      </nav>

      {tree.isPending ? <LoadingPlaceholder text="Reading the repository…" /> : null}
      {tree.isError ? (
        <StatusSurface
          icon="alert-triangle"
          tone="error"
          title="Could not read the repository"
          detail={describeError(tree.error)}
          actions={
            <Button variant="secondary" size="sm" icon="refresh-cw" onClick={() => void tree.refetch()}>
              Try again
            </Button>
          }
        />
      ) : null}
      {tree.isSuccess && tree.tree.entries.length === 0 ? (
        <EmptySurface
          illustration="resources"
          title="This directory is empty"
          detail="Nothing is committed here yet."
        />
      ) : null}

      {tree.tree.entries.length > 0 ? (
        <ul className={styles.list}>
          {dir ? (
            <li>
              <button type="button" className={styles.row} onClick={() => onDir(parentDir(dir))}>
                <Icon name="arrow-left" size="sm" />
                <span className={styles.rowName}>..</span>
              </button>
            </li>
          ) : null}
          {tree.tree.entries.map((entry) => (
            <li key={entry.path}>
              <button
                type="button"
                className={styles.row}
                onClick={() => (entry.kind === 'dir' ? onDir(entry.path) : onOpen(entry.path))}
              >
                <Icon name={entry.kind === 'dir' ? 'folder' : 'file-text'} size="sm" />
                <span className={styles.rowName}>{entry.name}</span>
                {entry.kind === 'file' ? (
                  <span className={styles.rowMeta}>{formatBytes(entry.size)}</span>
                ) : null}
                <Icon name="chevron-right" size="xs" />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function FilePane({
  repositoryId,
  path,
  canWrite,
  editing,
  onEdit,
  onCancelEdit,
  onClose,
}: {
  repositoryId: string;
  path: string;
  canWrite: boolean;
  editing: boolean;
  onEdit: () => void;
  onCancelEdit: () => void;
  onClose: () => void;
}) {
  const styles = useStyles2(getStyles);
  const file = useRepositoryFile(repositoryId, path);
  const resources = useProvisioningResources(repositoryId);

  const resourceId = resources.resources.find((resource) => resource.path === path)?.id;

  if (file.isPending) return <LoadingPlaceholder text="Reading the file…" />;
  if (file.isError) {
    return (
      <StatusSurface
        icon="alert-triangle"
        tone="error"
        title="Could not read the file"
        detail={describeError(file.error)}
        actions={
          <Button variant="secondary" size="sm" onClick={onClose}>
            Back
          </Button>
        }
      />
    );
  }

  const current = file.file as RepoFile;

  return (
    <div className={styles.wrap}>
      <div className={styles.fileHead}>
        <Button variant="secondary" size="sm" fill="text" icon="arrow-left" onClick={onClose}>
          Back
        </Button>
        <span className={styles.filePath}>{current.path}</span>
        {current.truncated ? <Tag name="Truncated" /> : null}
        {canWrite && !editing && resourceId ? (
          <Button variant="secondary" size="sm" icon="pencil" onClick={onEdit}>
            Edit
          </Button>
        ) : null}
        {canWrite && !editing && !resourceId ? (
          <span className={styles.noEdit}>
            This file is not a provisioned resource, so it cannot be edited here.
          </span>
        ) : null}
      </div>

      <p className={styles.fileMeta}>
        {current.message ? `${current.message} · ` : null}
        {current.author ? `${current.author} · ` : null}
        {current.when ? formatTimestamp(current.when.getTime(), { relative: true }) : null}
        {current.commit ? ` · ${current.commit.slice(0, 8)}` : null}
      </p>

      {editing && resourceId ? (
        <RepoFileEditor
          repositoryId={repositoryId}
          resourceId={resourceId}
          file={current}
          onDone={() => {
            onCancelEdit();
            onClose();
          }}
          onCancel={onCancelEdit}
        />
      ) : (
        <CodeBlock code={current.content} language={current.path} className={styles.code} />
      )}
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KiB`;

  return `${Math.round(bytes / (1024 * 1024))} MiB`;
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  crumbs: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.5), flexWrap: 'wrap' }),
  crumbGroup: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5) }),
  crumb: css({
    background: 'none',
    border: 'none',
    padding: 0,
    font: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
    cursor: 'pointer',
    '&:hover': { color: theme.colors.text.primary },
  }),
  list: css({ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 2 }),
  row: css({
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    width: '100%',
    padding: `${theme.spacing(1)} ${theme.spacing(1.5)}`,
    background: 'transparent',
    border: `1px solid transparent`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
    font: 'inherit',
    fontSize: theme.typography.sm.fontSize,
    textAlign: 'left',
    cursor: 'pointer',
    '&:hover': { background: theme.colors.background.secondary, borderColor: theme.colors.border.weak },
  }),
  rowName: css({ flex: 1, minWidth: 0, color: theme.colors.text.primary, overflowWrap: 'anywhere' }),
  rowMeta: css({ color: theme.colors.text.secondary, fontVariantNumeric: 'tabular-nums' }),
  fileHead: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(1.5), flexWrap: 'wrap' }),
  filePath: css({
    flex: 1,
    minWidth: 0,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.primary,
    overflowWrap: 'anywhere',
  }),
  fileMeta: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  noEdit: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  code: css({
    margin: 0,
    padding: theme.spacing(2),
    maxHeight: '60vh',
    overflow: 'auto',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.7,
    color: theme.colors.text.primary,
    whiteSpace: 'pre',
  }),
});
