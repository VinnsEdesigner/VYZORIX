import { useEffect, useState } from 'react';
import { css } from '@emotion/css';
import {
  Button,
  Combobox,
  Field,
  Input,
  Modal,
  Switch,
  TextArea,
  useStyles2,
  type VyzorixTheme,
} from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import {
  DEFAULT_SYNC_INTERVAL_SECONDS,
  MIN_SYNC_INTERVAL_SECONDS,
  formatPatternList,
  parsePatternList,
  useCreateProvisioningRepository,
  useProvisioningConnections,
  useProvisioningDiscovery,
  useUpdateProvisioningRepository,
  validateRepositoryDraft,
  type ProvisioningRepository,
} from '@/hooks/provisioning';
import { notifySuccess } from '@/stores/toast-store';

interface SourceDialogProps {
  isOpen: boolean;
  onDismiss: () => void;
  repository?: ProvisioningRepository;
}

export function SourceDialog({ isOpen, onDismiss, repository }: SourceDialogProps) {
  const styles = useStyles2(getStyles);
  const create = useCreateProvisioningRepository();
  const update = useUpdateProvisioningRepository();
  const connections = useProvisioningConnections();

  const editing = Boolean(repository);
  const [name, setName] = useState('');
  const [title, setTitle] = useState('');
  const [url, setUrl] = useState('');
  const [branch, setBranch] = useState('main');
  const [path, setPath] = useState('/');
  const [connectionId, setConnectionId] = useState('');
  const [intervalSeconds, setIntervalSeconds] = useState(DEFAULT_SYNC_INTERVAL_SECONDS);
  const [enabled, setEnabled] = useState(true);
  const [include, setInclude] = useState('');
  const [exclude, setExclude] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [serverError, setServerError] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    setName(repository?.name ?? '');
    setTitle(repository?.title ?? '');
    setUrl(repository?.url ?? '');
    setBranch(repository?.branch || 'main');
    setPath(repository?.path || '/');
    setConnectionId(repository?.connectionId ?? '');
    setIntervalSeconds(repository?.syncIntervalSeconds ?? DEFAULT_SYNC_INTERVAL_SECONDS);
    setEnabled(repository?.enabled ?? true);
    setInclude(formatPatternList(repository?.include ?? []));
    setExclude(formatPatternList(repository?.exclude ?? []));
    setErrors({});
    setServerError('');
  }, [isOpen, repository]);

  const discovery = useProvisioningDiscovery(editing ? repository?.id : undefined);
  const discoveredTotal = discovery.discovered.length;

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setServerError('');

    const includeList = parsePatternList(include);
    const excludeList = parsePatternList(exclude);
    const found = validateRepositoryDraft({
      name,
      url,
      connectionId,
      intervalSeconds,
      include: includeList,
      exclude: excludeList,
    });
    if (Object.keys(found).length > 0) {
      setErrors(found);
      return;
    }
    setErrors({});

    const data = {
      name: name.trim(),
      title: title.trim(),
      url: url.trim(),
      branch: branch.trim() || 'main',
      path: path.trim() || '/',
      connection_id: connectionId,
      sync_interval_seconds: intervalSeconds,
      enabled,
      include: includeList,
      exclude: excludeList,
    };

    try {
      if (editing && repository) {
        await update.mutateAsync({ id: repository.id, data });
        notifySuccess('Source updated');
      } else {
        await create.mutateAsync({ data });
        notifySuccess('Source added', 'Run a sync to apply what it declares.');
      }
      onDismiss();
    } catch (error) {

      setServerError(describeError(error));
    }
  };

  const busy = create.isPending || update.isPending;
  const availableConnections = connections.connections;

  return (
    <Modal title={editing ? 'Edit source' : 'Add source'} isOpen={isOpen} onDismiss={onDismiss} width={640}>
      <form onSubmit={submit} className={styles.form} noValidate>
        <div className={styles.twoUp}>
          <Field label="Name" required error={errors.name} description="A short identifier, unique here.">
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="platform-config" autoFocus />
          </Field>
          <Field label="Title" description="What a teammate sees in the list.">
            <Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Platform configuration" />
          </Field>
        </div>

        <Field
          label="Repository URL"
          required
          error={errors.url}
          description="An https or ssh git URL. Credentials belong in the connection, not the URL."
        >
          <Input
            value={url}
            onChange={(event) => setUrl(event.target.value)}
            placeholder="https://github.com/acme/device-config.git"
          />
        </Field>

        <div className={styles.twoUp}>
          <Field label="Branch" required description="One branch per source.">
            <Input value={branch} onChange={(event) => setBranch(event.target.value)} placeholder="main" />
          </Field>
          <Field label="Path" description="A subfolder to read, or / for the whole repository.">
            <Input value={path} onChange={(event) => setPath(event.target.value)} placeholder="/" />
          </Field>
        </div>

        <Field
          label="Credential"
          required
          error={errors.connectionId}
          description={
            availableConnections.length === 0
              ? 'Add a credential first — a source needs one to read a private repository.'
              : 'The stored token this source is read with.'
          }
        >
          <Combobox
            options={availableConnections.map((connection) => ({
              value: connection.id,
              label: connection.name,
            }))}
            value={connectionId || null}
            onChange={(next) => setConnectionId(next ?? '')}
            placeholder="Choose a credential…"
            aria-label="Credential"
          />
        </Field>

        <div className={styles.twoUp}>
          <Field
            label="Sync interval (seconds)"
            error={errors.intervalSeconds}
            description={`How often to check for changes. Minimum ${MIN_SYNC_INTERVAL_SECONDS}.`}
          >
            <Input
              type="number"
              inputMode="numeric"
              value={intervalSeconds}
              onChange={(event) => setIntervalSeconds(Number(event.target.value))}
            />
          </Field>
          <label className={styles.switchRow}>
            <Switch checked={enabled} onChange={(event) => setEnabled(event.target.checked)} />
            <span>
              <span className={styles.switchLabel}>Schedule enabled</span>
              <span className={styles.switchHint}>Turn off to sync only when you ask.</span>
            </span>
          </label>
        </div>

        <div className={styles.scopeBlock}>
          <h3 className={styles.scopeHeading}>What to sync</h3>
          <p className={styles.scopeHint}>
            Leave both empty to sync everything under the path. Exclude wins over include, so a file
            matching both is left alone. A pattern ending in <code>{'/*'}</code> covers the whole
            subtree.
            {editing ? (
              <span className={styles.matchCount}>
                {' '}
                This source declares {discoveredTotal} {discoveredTotal === 1 ? 'file' : 'files'}.
              </span>
            ) : null}
          </p>
          <div className={styles.twoUp}>
            <Field label="Include" error={errors.include} description="One glob per line. Empty means everything.">
              <TextArea
                rows={4}
                value={include}
                onChange={(event) => setInclude(event.target.value)}
                placeholder={'devices/*\norgs/*'}
              />
            </Field>
            <Field label="Exclude" error={errors.exclude} description="One glob per line. Wins over include.">
              <TextArea
                rows={4}
                value={exclude}
                onChange={(event) => setExclude(event.target.value)}
                placeholder={'devices/retired/*'}
              />
            </Field>
          </div>
        </div>

        {serverError ? <p className={styles.serverError}>{serverError}</p> : null}

        <div className={styles.actions}>
          <Button type="button" variant="secondary" onClick={onDismiss}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={busy}
            disabled={availableConnections.length === 0}
          >
            {editing ? 'Save' : 'Add source'}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

const getStyles = (theme: VyzorixTheme) => ({
  form: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(2) }),
  twoUp: css({
    display: 'grid',
    gap: theme.spacing(2),
    gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
    alignItems: 'start',
  }),
  switchRow: css({
    display: 'flex',
    alignItems: 'flex-start',
    gap: theme.spacing(1.5),
    cursor: 'pointer',
    paddingTop: theme.spacing(2.5),
  }),
  switchLabel: css({
    display: 'block',
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  switchHint: css({ display: 'block', fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  scopeBlock: css({
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(1),
    padding: theme.spacing(1.5),
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
  }),
  scopeHeading: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  scopeHint: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  matchCount: css({ color: theme.colors.text.primary, fontWeight: theme.typography.fontWeightMedium }),
  serverError: css({
    margin: 0,
    padding: theme.spacing(1.5),
    borderRadius: theme.shape.radius.default,
    background: theme.colors.error.subtleBackground,
    border: `1px solid ${theme.colors.error.subtleBorder}`,
    color: theme.colors.error.text,
    fontSize: theme.typography.sm.fontSize,
  }),
  actions: css({ display: 'flex', justifyContent: 'flex-end', gap: theme.spacing(1), marginTop: theme.spacing(1) }),
});
