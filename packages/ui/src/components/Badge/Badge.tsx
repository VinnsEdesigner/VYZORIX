import { css, cx } from '@emotion/css';
import { forwardRef, type HTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconName } from '../Icon';

export type BadgeColor = 'primary' | 'secondary' | 'success' | 'warning' | 'error' | 'info';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  text?: React.ReactNode;
  color?: BadgeColor;
  icon?: IconName;
}

function getStyles(theme: VyzorixTheme, color: BadgeColor) {
  const rich = theme.colors[color];
  return {
    badge: css({
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(0.5),
      padding: `2px ${theme.spacing(1)}`,
      borderRadius: theme.shape.radius.pill,
      fontSize: theme.typography.sm.fontSize,
      fontWeight: theme.typography.fontWeightMedium,
      lineHeight: 1.4,
      background: rich.subtleBackground,
      color: rich.text,
      border: `1px solid ${rich.subtleBorder}`,
      whiteSpace: 'nowrap',
    }),
  };
}

/** A small status pill. Colour carries meaning, so pair it with text or an icon. */
export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  ({ text, color = 'secondary', icon, className, children, ...rest }, ref) => {
    const styles = useStyles2(getStyles, color);
    return (
      <span ref={ref} className={cx(styles.badge, className)} {...rest}>
        {icon ? <Icon name={icon} size="xs" /> : null}
        {text ?? children}
      </span>
    );
  },
);

Badge.displayName = 'Badge';
