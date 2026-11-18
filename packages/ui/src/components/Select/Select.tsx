import { css, cx } from '@emotion/css';
import { forwardRef, type SelectHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { controlStyles } from '../_shared/inputStyles';

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean;
}

const getStyles = (theme: VyzorixTheme, invalid?: boolean) => ({
  control: css([controlStyles(theme, invalid), { appearance: 'none', paddingRight: theme.spacing(4), cursor: 'pointer' }]),
});

export const Select = forwardRef<HTMLSelectElement, SelectProps>(
  ({ invalid, className, children, ...rest }, ref) => {
    const styles = useStyles2(getStyles, invalid);
    return (
      <select ref={ref} aria-invalid={invalid || undefined} className={cx(styles.control, className)} {...rest}>
        {children}
      </select>
    );
  },
);

Select.displayName = 'Select';
