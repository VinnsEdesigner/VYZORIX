import { css, cx } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';
import { Icon, type IconSize } from '../Icon';

export interface SpinnerProps {
  className?: string;
  style?: React.CSSProperties;

  inline?: boolean;
  size?: IconSize;
  'aria-label'?: string;
}

const getStyles = (theme: VyzorixTheme) => ({
  wrapper: css({ display: 'flex', alignItems: 'center', justifyContent: 'center' }),
  inline: css({ display: 'inline-block', lineHeight: 0 }),
  spin: css({
    [theme.transitions.handleMotion('no-preference', 'reduce')]: {
      animation: 'vyzorix-spin 2s infinite linear',
    },
  }),
});

export function Spinner({ className, style, inline, size = 'md', 'aria-label': ariaLabel = 'Loading' }: SpinnerProps) {
  const styles = useStyles2(getStyles);

  return (
    <div data-testid="Spinner" style={style} className={cx(styles.wrapper, inline && styles.inline, className)}>
      <Icon name="spinner" size={size} className={styles.spin} aria-label={ariaLabel} />
    </div>
  );
}
