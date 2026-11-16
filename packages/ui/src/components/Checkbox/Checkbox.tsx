import { css, cx } from '@emotion/css';
import { forwardRef, type InputHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: string;
  invalid?: boolean;
}

const getStyles = (theme: VyzorixTheme, invalid?: boolean) => ({
  wrapper: css({ display: 'inline-flex', alignItems: 'center', gap: theme.spacing(1), cursor: 'pointer' }),
  input: css({
    width: 16,
    height: 16,
    accentColor: theme.colors.primary.main,
    cursor: 'pointer',
    flexShrink: 0,
    outline: invalid ? `1px solid ${theme.colors.error.border}` : undefined,
    '&:disabled': { cursor: 'not-allowed' },
  }),
  label: css({ fontSize: theme.typography.base.fontSize, color: theme.colors.text.primary }),
});

/** A checkbox. */
export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ label, invalid, className, ...rest }, ref) => {
    const styles = useStyles2(getStyles, invalid);
    return (
      <label className={cx(styles.wrapper, className)}>
        <input ref={ref} type="checkbox" aria-invalid={invalid || undefined} className={styles.input} {...rest} />
        {label ? <span className={styles.label}>{label}</span> : null}
      </label>
    );
  },
);

Checkbox.displayName = 'Checkbox';
