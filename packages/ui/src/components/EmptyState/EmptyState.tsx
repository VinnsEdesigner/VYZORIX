import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';
import { Text } from '../Text';

export interface EmptyStateProps {
  /** A short, specific headline: what is missing, not "No data". */
  message: string;
  /** The action the user can take to fix it, if any. */
  children?: React.ReactNode;
  icon?: IconName;
  variant?: 'success' | 'info';
  className?: string;
}

function getStyles(theme: VyzorixTheme, variant: 'success' | 'info') {
  return {
    wrapper: css({
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(2),
      padding: theme.spacing(6),
      textAlign: 'center',
      border: `1px dashed ${theme.colors.border.medium}`,
      borderRadius: theme.shape.radius.default,
      color: variant === 'success' ? theme.colors.success.text : theme.colors.text.secondary,
    }),
  };
}

/** The empty state for a list or panel with nothing to show. */
export function EmptyState({ message, children, icon, variant = 'info', className }: EmptyStateProps) {
  const styles = useStyles2(getStyles, variant);
  return (
    <div className={cx(styles.wrapper, className)}>
      {icon ? <Icon name={icon} size="xl" /> : null}
      <Text color={variant === 'success' ? 'success' : 'secondary'}>{message}</Text>
      {children}
    </div>
  );
}
