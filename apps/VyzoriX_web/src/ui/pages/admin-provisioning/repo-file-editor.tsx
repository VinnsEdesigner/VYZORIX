import { useState } from 'react';
import { css } from '@emotion/css';
import { Button, Combobox, Field, Icon, Input, TextArea, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import { CodeBlock } from '@/ui/components/shared/code-block';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import {
  editPreviewFromWire,
  editResultFromWire,
  useDropProvisioningCache,
  usePreviewManifestEdit,
  usePushManifestEdit,
  type ManifestPreview,
  type RepoFile,
} from '@/hooks/provisioning';

interface RepoFileEditorProps {
  repositoryId: string;

  resourceId: string;
  file: RepoFile;
  onDone: () => void;
  onCancel: () => void;
}

export function RepoFileEditor({ repositoryId, resourceId, file, onDone, onCancel }: RepoFileEditorProps) {
  const styles = useStyles2(getStyles);
  const preview = usePreviewManifestEdit();
  const push = usePushManifestEdit();
  const dropCache = useDropProvisioningCache();

  const [content, setContent] = useState(file.content);
  const [message, setMessage] = useState('');
  const [mode, setMode] = useState<'pr' | 'direct'>('pr');
  const [diff, setDiff] = useState<ManifestPreview | undefined>();
  const [error, setError] = useState('');

  const runPreview = async () => {
    setError('');
    try {
      const result = await preview.mutateAsync({
        id: repositoryId,
        resourceId,
        data: { content, base_commit: file.commit, mode, commit_message: message || undefined },
      });

      setDiff(editPreviewFromWire(result));
    } catch (caught) {
      setError(describeError(caught));
    }
  };

  const commit = async () => {
    if (!diff) return;
    setError('');

    try {
      const result = await push.mutateAsync({
        id: repositoryId,
        resourceId,
        data: {
          content,
          base_commit: diff.baseCommit,
          mode,
          commit_message: message || `Update ${file.path}`,
        },
      });

      const mapped = editResultFromWire(result);

      if (mapped.pullRequestError) {
        notifyError('Committed, but the pull request failed', mapped.pullRequestError);
      } else if (mapped.pullRequestUrl) {
        notifySuccess('Pull request opened', mapped.pullRequestUrl);
      } else {
        notifySuccess('Committed', mapped.commit ? `Commit ${mapped.commit.slice(0, 8)}` : undefined);
      }

      void dropCache(repositoryId);
      onDone();
    } catch (caught) {
      setError(describeError(caught));
    }
  };

  return (
    <div className={styles.wrap}>
      <Field label="Content" description="The whole file. The diff below shows exactly what changes.">
        <TextArea rows={18} value={content} onChange={(event) => setContent(event.target.value)} spellCheck={false} />
      </Field>

      <div className={styles.twoUp}>
        <Field label="Commit message" description="Left empty, a message is generated from the path.">
          <Input value={message} onChange={(event) => setMessage(event.target.value)} placeholder={`Update ${file.path}`} />
        </Field>
        <Field
          label="How to commit"
          description={
            mode === 'pr'
              ? 'A branch and pull request, so the change is reviewed.'
              : 'Straight onto the tracked branch. No review step.'
          }
        >
          <Combobox
            options={[
              { value: 'pr', label: 'Open a pull request' },
              { value: 'direct', label: 'Commit directly' },
            ]}
            value={mode}
            onChange={(next) => setMode(next === 'direct' ? 'direct' : 'pr')}
            aria-label="How to commit"
          />
        </Field>
      </div>

      {error ? <p className={styles.error}>{error}</p> : null}

      {diff ? (
        <section className={styles.diffBlock}>
          <h3 className={styles.diffHeading}>
            {diff.changed ? 'What this commit changes' : 'No change'}
          </h3>
          {diff.changed ? (
            <CodeBlock code={diff.diff} language="diff" className={styles.diff} />
          ) : (
            <p className={styles.hint}>The file already holds this content, so nothing would be committed.</p>
          )}
        </section>
      ) : null}

      <div className={styles.actions}>
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          variant="secondary"
          onClick={() => void runPreview()}
          loading={preview.isPending}
          disabled={content === file.content && !diff}
        >
          Preview diff
        </Button>
        <Button
          variant="primary"
          onClick={() => void commit()}
          loading={push.isPending}
          disabled={!diff || !diff.changed}
        >
          Commit
        </Button>
      </div>

      {/* A disabled button on its own reads as broken, so it says what is missing and why.
          A tooltip is not enough: it needs a hover, and it never says what to do next. */}
      {!diff ? (
        <p className={styles.nextStep}>
          <Icon name="info" size="sm" />
          <span>
            Preview the diff before committing. The commit has to name the revision it was based
            on, so that a change pushed by someone else in the meantime is refused rather than
            overwritten.
          </span>
        </p>
      ) : null}
      {diff && !diff.changed ? (
        <p className={styles.nextStep}>
          <Icon name="info" size="sm" />
          <span>
            This file already holds that content, so there is nothing to commit. Edit the content
            above, then preview again.
          </span>
        </p>
      ) : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  twoUp: css({
    display: 'grid',
    gap: theme.spacing(2),
    gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
    alignItems: 'start',
  }),
  error: css({
    margin: 0,
    padding: theme.spacing(1.5),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.error.subtleBackground,
    border: `1px solid ${theme.colors.error.subtleBorder}`,
    color: theme.colors.error.text,
    fontSize: theme.typography.sm.fontSize,
  }),
  diffBlock: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1) }),
  diffHeading: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  diff: css({
    margin: 0,
    padding: theme.spacing(1.5),
    maxHeight: '40vh',
    overflow: 'auto',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.6,
    color: theme.colors.text.primary,
    whiteSpace: 'pre',
  }),
  hint: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  nextStep: css({
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(1),
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    color: theme.colors.text.secondary,
  }),
  actions: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1), flexWrap: 'wrap' }),
});
