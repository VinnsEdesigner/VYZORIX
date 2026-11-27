import type { ReactNode } from 'react';
import { css, cx } from '@emotion/css';
import { Icon, useStyles2, type IconName, type VyzorixTheme } from '@vyzorix/ui';
import { emptyStateAvatar, type EmptyIllustration } from '@/lib/avatar';

export interface SurfaceStateProps {
  title: string;
  detail?: ReactNode;
  actions?: ReactNode;
  className?: string;
}

interface IllustrationProps extends SurfaceStateProps {
  illustration: EmptyIllustration;
  children?: ReactNode;
}

interface StatusProps extends SurfaceStateProps {
  icon: IconName;
  tone: 'error' | 'warning' | 'success' | 'progress';
  children?: ReactNode;
}

export function EmptySurface({ illustration, title, detail, actions, children, className }: IllustrationProps) {
  const styles = useStyles2(getStyles);

  return (
    <div className={cx(styles.empty, className)} role="status">
      <img
        src={emptyStateAvatar(illustration)}
        alt=""
        aria-hidden="true"
        className={styles.illustration}
        width={260}
        height={260}
        draggable={false}
      />
      <p className={styles.emptyTitle}>{title}</p>
      {detail ? <div className={styles.emptyDetail}>{detail}</div> : null}
      {children}
      {actions ? <div className={styles.actions}>{actions}</div> : null}
    </div>
  );
}

export function StatusSurface({ icon, tone, title, detail, actions, children, className }: StatusProps) {
  const styles = useStyles2(getStyles, tone);

  return (
    <div className={cx(styles.status, className)} role="status" data-tone={tone}>
      <span className={styles.statusIcon} aria-hidden="true">
        <Icon name={icon} size="xl" />
      </span>
      <div className={styles.statusBody}>
        <p className={styles.statusTitle}>{title}</p>
        {detail ? <div className={styles.statusDetail}>{detail}</div> : null}
        {children}
        {actions ? <div className={styles.actions}>{actions}</div> : null}
      </div>
    </div>
  );
}

function toneColor(theme: VyzorixTheme, tone: StatusProps['tone']): string {
  switch (tone) {
    case 'error':
      return theme.colors.error.text;
    case 'warning':
      return theme.colors.warning.text;
    case 'success':
      return theme.colors.success.text;
    default:
      return theme.colors.primary.text;
  }
}

const getStyles = (theme: VyzorixTheme, tone?: StatusProps['tone']) => ({
  empty: css({
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: theme.spacing(1.5),
    padding: `${theme.spacing(6)} ${theme.spacing(3)}`,
    textAlign: 'center',
  }),
  illustration: css({
    width: 260,
    height: 260,

    marginBottom: theme.spacing(1),
    userSelect: 'none',
    [theme.breakpoints.down('md')]: { width: 200, height: 200 },
  }),
  emptyTitle: css({
    margin: 0,
    fontSize: theme.typography.lg.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
    letterSpacing: '-0.01em',
  }),
  emptyDetail: css({
    margin: 0,
    maxWidth: 460,
    fontSize: theme.typography.sm.fontSize,
    lineHeight: 1.65,
    color: theme.colors.text.secondary,
  }),
  status: css({
    display: 'flex',
    gap: theme.spacing(1.5),
    alignItems: 'flex-start',
    padding: theme.spacing(2),
    border: `1px dashed ${theme.colors.border.weak}`,
    borderRadius: theme.shape.radius.default,
    color: theme.colors.text.secondary,
  }),
  statusIcon: css({ display: 'flex', flexShrink: 0, paddingTop: 2, color: tone ? toneColor(theme, tone) : undefined }),
  statusBody: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.75), minWidth: 0 }),
  statusTitle: css({
    margin: 0,
    fontSize: theme.typography.base.fontSize,
    fontWeight: theme.typography.fontWeightMedium,
    color: theme.colors.text.primary,
  }),
  statusDetail: css({ fontSize: theme.typography.sm.fontSize, color: theme.colors.text.secondary }),
  actions: css({ display: 'flex', flexWrap: 'wrap', gap: theme.spacing(1), marginTop: theme.spacing(1) }),
});
