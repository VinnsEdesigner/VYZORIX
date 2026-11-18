import { css, cx } from '@emotion/css';
import { forwardRef, type InputHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { useFieldContext } from '../Field/FieldContext';
import { controlStyles } from '../_shared/inputStyles';

export interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'prefix'> {

  invalid?: boolean;
  prefix?: React.ReactNode;
  suffix?: React.ReactNode;
}

const getStyles = (theme: VyzorixTheme, invalid?: boolean) => ({
  wrapper: css({ position: 'relative', display: 'flex', alignItems: 'center', width: '100%' }),
  control: controlStyles(theme, invalid),
  withPrefix: css({ paddingLeft: theme.spacing(4) }),
  withSuffix: css({ paddingRight: theme.spacing(4) }),
  adornment: css({
    position: 'absolute',
    display: 'flex',
    alignItems: 'center',
    color: theme.colors.text.secondary,
    pointerEvents: 'none',
  }),
  prefix: css({ left: theme.spacing(1.5) }),
  suffix: css({ right: theme.spacing(1.5) }),
});

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ invalid, prefix, suffix, className, id, ...rest }, ref) => {
    const styles = useStyles2(getStyles, invalid);

    const field = useFieldContext();
    const controlId = id ?? field?.id;
    const describedBy = rest['aria-describedby'] ?? field?.['aria-describedby'];
    const isInvalid = invalid ?? field?.['aria-invalid'];

    return (
      <div className={styles.wrapper}>
        {prefix ? <span className={cx(styles.adornment, styles.prefix)}>{prefix}</span> : null}
        <input
          ref={ref}
          id={controlId}
          aria-invalid={isInvalid || undefined}
          aria-describedby={describedBy}
          className={cx(styles.control, Boolean(prefix) && styles.withPrefix, Boolean(suffix) && styles.withSuffix, className)}
          {...rest}
        />
        {suffix ? <span className={cx(styles.adornment, styles.suffix)}>{suffix}</span> : null}
      </div>
    );
  },
);

Input.displayName = 'Input';
