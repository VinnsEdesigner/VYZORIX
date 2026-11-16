import { cx } from '@emotion/css';
import { forwardRef, type TextareaHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { controlStyles } from '../_shared/inputStyles';
import { css } from '@emotion/css';

export interface TextAreaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

const getStyles = (theme: VyzorixTheme, invalid?: boolean) => ({
  control: css([controlStyles(theme, invalid), { resize: 'vertical', minHeight: theme.spacing(10) }]),
});

/** A multi-line text input. */
export const TextArea = forwardRef<HTMLTextAreaElement, TextAreaProps>(
  ({ invalid, className, ...rest }, ref) => {
    const styles = useStyles2(getStyles, invalid);
    return <textarea ref={ref} aria-invalid={invalid || undefined} className={cx(styles.control, className)} {...rest} />;
  },
);

TextArea.displayName = 'TextArea';
