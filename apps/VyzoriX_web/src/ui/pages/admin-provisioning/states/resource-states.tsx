import type { ReactNode } from 'react';
import { css } from '@emotion/css';
import { Icon, Tag, useStyles2, type VyzorixTheme } from '@vyzorix/ui';
import { devicePresenceLabel, resourceStateLabel, type ResourceState } from '@/hooks/provisioning';

const TONE: Record<ResourceState, { icon: Parameters<typeof Icon>[0]['name']; tone: string }> = {
  applied: { icon: 'check-circle', tone: 'success' },

  pending: { icon: 'clock', tone: 'progress' },
  drifted: { icon: 'alert-triangle', tone: 'warning' },
  missing: { icon: 'minus', tone: 'neutral' },
  error: { icon: 'x-circle', tone: 'error' },
  ignored: { icon: 'eye-off', tone: 'neutral' },
};

export function ResourceStateBadge({ state }: { state: ResourceState }) {
  const styles = useStyles2(getStyles);
  const { icon, tone } = TONE[state];

  return (
    <span className={styles.badge} data-tone={tone}>
      <Icon name={icon} size="xs" />
      <Tag name={resourceStateLabel(state)} />
    </span>
  );
}

export function ResourceApplied({ detail }: { detail?: ReactNode }) {
  return <ResourceStateLine state="applied" detail={detail} />;
}

export function ResourcePending({ detail }: { detail?: ReactNode }) {
  return <ResourceStateLine state="pending" detail={detail} />;
}

export function ResourceDrifted({ detail }: { detail?: ReactNode }) {
  return <ResourceStateLine state="drifted" detail={detail} />;
}

export function ResourceMissing({ detail }: { detail?: ReactNode }) {
  return <ResourceStateLine state="missing" detail={detail} />;
}

export function ResourceError({ detail }: { detail?: ReactNode }) {
  return <ResourceStateLine state="error" detail={detail} />;
}

export function ResourceIgnored({ detail }: { detail?: ReactNode }) {
  return <ResourceStateLine state="ignored" detail={detail} />;
}

export function ResourceStateLine({ state, detail }: { state: ResourceState; detail?: ReactNode }) {
  const styles = useStyles2(getStyles, state);
  const { icon } = TONE[state];

  return (
    <div className={styles.line}>
      <Icon name={icon} size="md" />
      <span className={styles.label}>{resourceStateLabel(state)}</span>
      {detail ? <span className={styles.detail}>{detail}</span> : null}
    </div>
  );
}

/**
 * DevicePresenceLine says whether the device behind a resource can actually be seen.
 *
 * A state is a claim about a device, so it is read next to this: "Applied" beside "Offline"
 * tells a different story from "Applied" beside "Online". `online` is null when the device is
 * unknown (an org resource, or a device the server could not read) and is rendered as unknown
 * rather than as offline, so a failed query never reads as a down device.
 */
export function DevicePresenceLine({
  online,
  lastSeen,
  reportedVersion,
}: {
  online: boolean | null;
  lastSeen?: ReactNode;
  reportedVersion?: number | null;
}) {
  const styles = useStyles2(getStyles);
  const icon = online === null ? 'help-circle' : online ? 'circle-dot' : 'eye-off';

  return (
    <div className={styles.presence}>
      <Icon name={icon} size="sm" />
      <span className={styles.presenceLabel}>{devicePresenceLabel(online)}</span>
      {lastSeen ? <span className={styles.detail}>· last seen {lastSeen}</span> : null}
      {reportedVersion !== null && reportedVersion !== undefined ? (
        <span className={styles.detail}>· reports v{reportedVersion}</span>
      ) : null}
    </div>
  );
}

const getStyles = (theme: VyzorixTheme, state?: ResourceState) => {
  const color = state === 'applied'
    ? theme.colors.success.text
    : state === 'pending'
      ? theme.colors.primary.text
    : state === 'drifted'
      ? theme.colors.warning.text
      : state === 'error'
        ? theme.colors.error.text
        : theme.colors.text.secondary;

  return {
    badge: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(0.5), color }),
    line: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(1),
      color,
      fontSize: theme.typography.sm.fontSize,
    }),
    label: css({ fontWeight: theme.typography.fontWeightMedium }),
    detail: css({ color: theme.colors.text.secondary }),
    presence: css({
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(1),
      fontSize: theme.typography.sm.fontSize,
      color: theme.colors.text.secondary,
    }),
    presenceLabel: css({ fontWeight: theme.typography.fontWeightMedium, color: theme.colors.text.primary }),
  };
};
