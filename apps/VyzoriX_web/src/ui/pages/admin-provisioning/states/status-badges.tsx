import { css } from '@emotion/css';
import { Icon, Tag, useStyles2, type IconName, type VyzorixTheme } from '@vyzorix/ui';
import {
  syncStateLabel,
  type ConnectionStatus,
  type RepositorySyncState,
} from '@/hooks/provisioning';

export function SourceSyncBadge({ state }: { state: RepositorySyncState }) {
  const styles = useStyles2(getStyles, state);
  const { icon } = badgeFor(state);

  return (
    <span className={styles.badge}>
      <Icon name={icon} size="xs" />
      <Tag name={syncStateLabel(state)} />
    </span>
  );
}

function badgeFor(state: RepositorySyncState): { icon: IconName } {
  switch (state) {
    case 'success':
      return { icon: 'check-circle' };
    case 'warning':
      return { icon: 'alert-triangle' };
    case 'error':
      return { icon: 'x-circle' };
    case 'working':
      return { icon: 'refresh-cw' };
    case 'disabled':
      return { icon: 'pause' };
    default:
      return { icon: 'clock' };
  }
}

export function CredentialPending() {
  return <CredentialLine status="pending" label="Not checked yet" />;
}

export function CredentialReady({ host }: { host?: string }) {
  return <CredentialLine status="ready" label={host ? `Reached ${host}` : 'Verified'} />;
}

export function CredentialError({ reason }: { reason?: string }) {
  return <CredentialLine status="error" label={reason || 'The check failed'} />;
}

export function CredentialUnreachable({ host }: { host?: string }) {
  return <CredentialLine status="error" label={host ? `Could not reach ${host}` : 'Could not reach the host'} />;
}

function CredentialLine({ status, label }: { status: ConnectionStatus; label: string }) {
  const styles = useStyles2(getStyles, status);
  const icon: IconName = status === 'ready' ? 'check-circle' : status === 'error' ? 'x-circle' : 'clock';

  return (
    <span className={styles.badge}>
      <Icon name={icon} size="xs" />
      <span className={styles.lineLabel}>{label}</span>
    </span>
  );
}

const getStyles = (theme: VyzorixTheme, state?: string) => {
  const color = state === 'success' || state === 'ready'
    ? theme.colors.success.text
    : state === 'warning' || state === 'working' || state === 'pending'
      ? theme.colors.warning.text
      : state === 'error'
        ? theme.colors.error.text
        : theme.colors.text.secondary;

  return {
    badge: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5), color }),
    lineLabel: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  };
};
