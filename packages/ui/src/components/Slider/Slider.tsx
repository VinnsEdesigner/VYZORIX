import { css, cx } from '@emotion/css';
import { forwardRef, type InputHTMLAttributes } from 'react';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';

export type SliderProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>;

const getStyles = (theme: VyzorixTheme) => ({
  slider: css({
    width: '100%',
    accentColor: theme.colors.primary.main,
    cursor: 'pointer',
    '&:disabled': { cursor: 'not-allowed' },
  }),
});

export const Slider = forwardRef<HTMLInputElement, SliderProps>(({ className, ...rest }, ref) => {
  const styles = useStyles2(getStyles);
  return <input ref={ref} type="range" className={cx(styles.slider, className)} {...rest} />;
});

Slider.displayName = 'Slider';
