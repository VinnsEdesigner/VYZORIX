import { css, cx } from '@emotion/css';
import { useId } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Text } from '../Text';
import { FieldContext } from './FieldContext';

export interface FieldProps {
  label: string;

  labelHidden?: boolean;
  description?: string;
  error?: string;
  invalid?: boolean;
  required?: boolean;

  children: React.ReactNode;
  className?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  field: css({ display: 'flex', flexDirection: 'column', gap: theme.spacing(0.5) }),
  labelRow: css({ display: 'flex', alignItems: 'center', gap: theme.spacing(0.5) }),
  visuallyHidden: css({
    position: 'absolute',
    width: 1,
    height: 1,
    padding: 0,
    margin: -1,
    overflow: 'hidden',
    clip: 'rect(0,0,0,0)',
    whiteSpace: 'nowrap',
    border: 0,
  }),
  required: css({ color: theme.colors.error.text }),
});

export function Field({
  label,
  labelHidden,
  description,
  error,
  invalid,
  required,
  children,
  className,
}: FieldProps) {
  const styles = useStyles2(getStyles);
  const id = useId();
  const describedBy = [description ? `${id}-desc` : null, error ? `${id}-err` : null].filter(Boolean).join(' ') || undefined;

  return (
    <div className={cx(styles.field, className)}>
      <label htmlFor={id} className={cx(styles.labelRow, labelHidden && styles.visuallyHidden)}>
        <Text variant="sm" weight="medium">
          {label}
        </Text>
        {required ? (
          <span className={styles.required} aria-hidden>
            *
          </span>
        ) : null}
      </label>
      {description ? (
        <Text variant="sm" color="secondary" id={`${id}-desc`}>
          {description}
        </Text>
      ) : null}
      <FieldContext.Provider value={{ id, 'aria-describedby': describedBy, 'aria-invalid': invalid || undefined }}>
        {children}
      </FieldContext.Provider>
      {error ? (
        <Text variant="sm" color="error" id={`${id}-err`} role="alert">
          {error}
        </Text>
      ) : null}
    </div>
  );
}
