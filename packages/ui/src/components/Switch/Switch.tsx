import { css, cx } from '@emotion/css';
import { forwardRef, type InputHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';

export interface SwitchProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(1), cursor: 'pointer' }),
  input: css({
    position: 'absolute',
    opacity: 0,
    width: 0,
    height: 0,
  }),
  track: css({
    position: 'relative',
    width: 32,
    height: 18,
    borderRadius: theme.shape.radius.pill,
    background: theme.colors.action.disabledBackground,
    border: `1px solid ${theme.colors.border.medium}`,
    transition: theme.transitions.create(['background']),
    flexShrink: 0,
    '&::after': {
      content: '""',
      position: 'absolute',
      top: 1,
      left: 1,
      width: 14,
      height: 14,
      borderRadius: theme.shape.radius.circle,
      background: theme.colors.text.secondary,
      transition: theme.transitions.create(['transform', 'background']),
    },
    'input:checked + &': {
      background: theme.colors.primary.main,
      borderColor: theme.colors.primary.main,
    },
    'input:checked + &::after': {
      transform: 'translateX(14px)',
      background: theme.colors.primary.contrastText,
    },
    'input:focus-visible + &': {
      boxShadow: `0 0 0 2px ${theme.colors.primary.subtleBackground}`,
    },
    'input:disabled + &': { opacity: theme.colors.action.disabledOpacity, cursor: 'not-allowed' },
  }),
  label: css({ fontSize: theme.typography.base.fontSize, color: theme.colors.text.primary }),
});

/** An on/off switch. */
export const Switch = forwardRef<HTMLInputElement, SwitchProps>(({ label, className, ...rest }, ref) => {
  const styles = useStyles2(getStyles);
  return (
    <label className={cx(styles.wrapper, className)}>
      <input ref={ref} type="checkbox" className={styles.input} {...rest} />
      <span className={styles.track} />
      {label ? <span className={styles.label}>{label}</span> : null}
    </label>
  );
});

Switch.displayName = 'Switch';
