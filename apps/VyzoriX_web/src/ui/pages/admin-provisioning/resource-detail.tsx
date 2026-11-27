import { css } from '@emotion/css';
import { Button, LoadingPlaceholder, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { describeError } from '@/lib/error';
import {
  useProvisioningResourceDocument,
  useProvisioningResourceSecrets,
  useSyncProvisionedResource,
  isDeviceKind,
  resourceKindLabel,
  resourceStateLabel,
  type ProvisionedResource,
} from '@/hooks/provisioning';
import { notifyError, notifySuccess } from '@/stores/toast-store';
import { formatTimestamp } from '@/lib/preferences/format';
import {
  DeviceSyncRunning,
  DevicePresenceLine,
  ResourcePending,
  ResourceDrifted,
  ResourceError,
  ResourceIgnored,
  ResourceMissing,
  SecretReadinessList,
  SyncFailed,
  SyncIdle,
  SyncPartialFailure,
  SyncSucceeded,
} from './states';
import { syncPhase } from '@/lib/provisioning/query-view';

interface ResourceDetailProps {
  repositoryId: string;
  resource: ProvisionedResource;
  canWrite: boolean;

  repositorySyncing?: boolean;
  onClose: () => void;
}

export function ResourceDetail({ repositoryId, resource, canWrite, repositorySyncing, onClose }: ResourceDetailProps) {
  const styles = useStyles2(getStyles);
  const secrets = useProvisioningResourceSecrets(repositoryId, resource.id);
  const document = useProvisioningResourceDocument(
    repositoryId,
    isDeviceKind(resource.kind) ? resource.id : undefined,
  );
  const sync = useSyncProvisionedResource();

  const isDevice = isDeviceKind(resource.kind);
  const busy = sync.isPending;
  const phase = syncPhase(sync.data, busy);

  const runSync = async () => {
    try {
      const job = await sync.mutateAsync({ id: repositoryId, resourceId: resource.id, params: {} });
      if (job.failed > 0) {
        notifyError('Sync finished with failures', job.results[0]?.message || `${job.failed} failed.`);
      } else {
        notifySuccess('Resource synced', job.applied > 0 ? 'Applied.' : 'Already up to date.');
      }
    } catch (error) {
      notifyError('Could not sync this resource', describeError(error));
    }
  };

  return (
    <div className={styles.wrap}>
      <header className={styles.head}>
        <div className={styles.headText}>
          <h2 className={styles.title}>{resource.deviceId || resource.path}</h2>
          <p className={styles.meta}>
            {resourceKindLabel(resource.kind)} · v{resource.configVersion} ·{' '}
            {resource.lastAppliedAt ? formatTimestamp(resource.lastAppliedAt.getTime(), { relative: true }) : 'never applied'}
          </p>
        </div>
        <Button variant="secondary" size="sm" fill="text" onClick={onClose}>
          Close
        </Button>
      </header>

      <StateFor resource={resource} />

      {isDevice ? (
        <DevicePresenceLine
          online={resource.deviceOnline}
          lastSeen={
            resource.deviceLastSeen ? formatTimestamp(resource.deviceLastSeen.getTime(), { relative: true }) : undefined
          }
          reportedVersion={resource.deviceReportedVersion}
        />
      ) : null}

      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h3 className={styles.sectionTitle}>Sync this {isDevice ? 'device' : 'file'}</h3>
          {canWrite ? (
            <Button
              variant="primary"
              size="sm"
              icon="refresh-cw"
              onClick={() => void runSync()}
              loading={busy}
              disabled={repositorySyncing}
            >
              {busy ? 'Syncing…' : `Sync this ${isDevice ? 'device' : 'file'}`}
            </Button>
          ) : null}
        </div>

        {busy ? <DeviceSyncRunning deviceId={resource.deviceId || resource.path} /> : null}

        {!busy && repositorySyncing ? (
          <p className={styles.hint}>A repository sync is running. This device can be synced when it finishes.</p>
        ) : null}

        {!busy && !repositorySyncing && phase === 'idle' ? <SyncIdle /> : null}
        {!busy && phase === 'succeeded' ? <SyncSucceeded applied={sync.data?.applied ?? 0} /> : null}
        {!busy && phase === 'partial' ? (
          <SyncPartialFailure applied={sync.data?.applied ?? 0} failed={sync.data?.failed ?? 0} />
        ) : null}
        {!busy && phase === 'failed' ? <SyncFailed error={sync.error} /> : null}
      </section>

      <section className={styles.section}>
        <h3 className={styles.sectionTitle}>Declared secrets</h3>
        {secrets.isPending ? <LoadingPlaceholder text="Reading declared secrets…" /> : null}
        {secrets.isError ? (
          <p className={styles.hint}>
            {isDevice
              ? 'This device has no configuration yet, so it declares no secrets.'
              : 'An organisation file declares no device secrets.'}
          </p>
        ) : null}
        {secrets.data ? <SecretReadinessList readiness={secrets.readiness} /> : null}
      </section>

      {isDevice ? (
        <section className={styles.section}>
          <h3 className={styles.sectionTitle}>Signed document</h3>
          {document.isPending ? <LoadingPlaceholder text="Reading the signed document…" /> : null}
          {document.isError ? <p className={styles.hint}>This device has no signed configuration yet.</p> : null}
          {document.data ? <pre className={styles.document}>{document.data}</pre> : null}
        </section>
      ) : null}
    </div>
  );
}

function StateFor({ resource }: { resource: ProvisionedResource }) {
  const styles = useStyles2(getStyles);

  switch (resource.state) {
    case 'pending':
      return (
        <ResourcePending
          detail={resource.lastError || 'The config is signed and stored; the device has not confirmed it yet.'}
        />
      );
    case 'drifted':
      return <ResourceDrifted detail="The device was changed outside this source. Re-sync to converge it." />;
    case 'missing':
      return <ResourceMissing detail="The file was removed from the repository. The device keeps what it runs." />;
    case 'error':
      return <ResourceError detail={resource.lastError || undefined} />;
    case 'ignored':
      return <ResourceIgnored detail="Outside this source's include/exclude scope. Nothing reverts." />;
    default:
      return <p className={styles.applied}>{resourceStateLabel(resource.state)}</p>;
  }
}

const getStyles = (theme: VyzorixTheme) => ({
  wrap: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(3) }),
  head: css({ display: 'flex', alignItems: 'flex-start', gap: theme.spacing(2) }),
  headText: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5), flex: 1, minWidth: 0 }),
  title: css({
    margin: 0,
    fontSize: theme.typography.lg.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
    overflowWrap: 'anywhere',
  }),
  meta: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  section: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(1.5) }),
  sectionHead: css({ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: theme.spacing(2) }),
  sectionTitle: css({
    margin: 0,
    fontSize: theme.typography.sm.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.secondary,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
  }),
  hint: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  applied: css({ margin: 0, fontSize: theme.typography.sm.fontSize, color: theme.colors.success.text }),
  document: css({
    margin: 0,
    padding: theme.spacing(2),
    maxHeight: 320,
    overflow: 'auto',
    background: theme.colors.background.secondary,
    border: `1px solid ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.6,
    whiteSpace: 'pre',
    color: theme.colors.text.primary,
  }),
});
