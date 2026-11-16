import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';
import { Text } from '../Text';

export type AlertSeverity = 'error' | 'warning' | 'success' | 'info';

export interface AlertProps {
  title: string;
  severity?: AlertSeverity;
  children?: React.ReactNode;
  /** Buttons/links that resolve the alert. */
  actions?: React.ReactNode;
  className?: string;
  onRemove?: () => void;
}

const SEVERITY_ICON: Record<AlertSeverity, IconName> = {
  error: 'alert-circle',
  warning: 'alert-triangle',
  success: 'check-circle',
  info: 'info',
};

function getStyles(theme: VyzorixTheme, severity: AlertSeverity) {
  const rich = theme.colors[severity === 'error' ? 'error' : severity === 'warning' ? 'warning' : severity];
  return {
    alert: css({
      display: 'flex',
      gap: theme.spacing(1.5),
      padding: theme.spacing(1.5),
      borderRadius: theme.shape.radius.default,
      background: rich.subtleBackground,
      border: `1px solid ${rich.subtleBorder}`,
      color: rich.text,
    }),
    body: css({ flex: 1, display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
    actions: css({ display: 'flex', gap: theme.spacing(1), marginTop: theme.spacing(0.5) }),
  };
}

/**
 * An inline message.
 *
 * `role="alert"` announces the message immediately, which is right for errors
 * and warnings; for a purely informational note that is too aggressive, but the
 * common case here is a problem the user must see.
 */
export function Alert({ title, severity = 'info', children, actions, className, onRemove }: AlertProps) {
  const styles = useStyles2(getStyles, severity);

  return (
    <div role="alert" className={cx(styles.alert, className)}>
      <Icon name={SEVERITY_ICON[severity]} size="md" />
      <div className={styles.body}>
        <Text variant="base" weight="medium" element="div" color={severity === 'error' ? 'error' : 'primary'}>
          {title}
        </Text>
        {children ? <Text variant="sm" color="secondary">{children}</Text> : null}
        {actions ? <div className={styles.actions}>{actions}</div> : null}
      </div>
      {onRemove ? (
        <button type="button" aria-label="Dismiss" onClick={onRemove} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'inherit' }}>
          <Icon name="x" size="sm" />
        </button>
      ) : null}
    </div>
  );
}
