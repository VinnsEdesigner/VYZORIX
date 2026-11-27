import type { ReactNode } from 'react';
import { css } from '@emotion/css';
import { Icon, LoadingPlaceholder, Tag, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { StatusSurface } from './surface-state';
import { describeError } from '@/lib/error';
import type { SyncPhase } from '@/lib/provisioning/query-view';

export function SyncIdle({ detail }: { detail?: ReactNode }) {
  return (
    <StatusSurface
      icon="refresh-cw"
      tone="progress"
      title="No sync has run yet"
      detail={detail ?? 'A sync reads the repository and applies every changed manifest. Nothing is written until you run one.'}
    />
  );
}

export function SyncRunning({ detail }: { detail?: ReactNode }) {
  return (
    <div>
      <LoadingPlaceholder text="Syncing…" />
      {detail ? <p style={{ margin: 0, opacity: 0.75 }}>{detail}</p> : null}
    </div>
  );
}

export function SyncSucceeded({ applied, detail }: { applied: number; detail?: ReactNode }) {
  return (
    <StatusSurface
      icon="check-circle"
      tone="success"
      title={applied > 0 ? `Applied ${applied} ${applied === 1 ? 'manifest' : 'manifests'}` : 'Everything was already up to date'}
      detail={detail}
    />
  );
}

export function SyncPartialFailure({
  applied,
  failed,
  detail,
}: {
  applied: number;
  failed: number;
  detail?: ReactNode;
}) {
  return (
    <StatusSurface
      icon="alert-triangle"
      tone="warning"
      title={`Applied ${applied}, failed ${failed}`}
      detail={
        detail ?? 'The manifests that failed are marked below. One failure never stops the rest of the run.'
      }
    />
  );
}

export function SyncFailed({ error, detail }: { error?: unknown; detail?: ReactNode }) {
  return (
    <StatusSurface
      icon="x-circle"
      tone="error"
      title="The sync failed"
      detail={detail ?? (error ? describeError(error) : 'Nothing was applied.')}
    />
  );
}

export function SyncOutcome({
  phase,
  applied = 0,
  failed = 0,
  error,
}: {
  phase: SyncPhase;
  applied?: number;
  failed?: number;
  error?: unknown;
}) {
  switch (phase) {
    case 'running':
      return <SyncRunning />;
    case 'succeeded':
      return <SyncSucceeded applied={applied} />;
    case 'partial':
      return <SyncPartialFailure applied={applied} failed={failed} />;
    case 'failed':
      return <SyncFailed error={error} />;
    default:
      return <SyncIdle />;
  }
}

export function DeviceSyncRunning({ deviceId }: { deviceId: string }) {
  const styles = useStyles2(getStyles);

  return (
    <span className={styles.inline} role="status">
      <Icon name="refresh-cw" size="sm" />
      Syncing {deviceId}…
    </span>
  );
}

export function DeviceSyncBlocked({ reason }: { reason: string }) {
  return (
    <span className={stylesBlocked.wrap}>
      <Tag name="Sync unavailable" icon="clock" />
      <span>{reason}</span>
    </span>
  );
}

const getStyles = (_theme: VyzorixTheme) => ({
  inline: css({
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 'inherit',
  }),
});

const stylesBlocked = {
  wrap: css({ display: 'inline-flex', alignItems: 'center', gap: 8, fontSize: 'inherit' }),
};
