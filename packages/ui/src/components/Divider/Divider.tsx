import { css } from '@emotion/css';
import { type VyzorixTheme } from '../../theme/types';
import { useStyles2 } from '../../theme/ThemeContext';

export interface DividerProps {
  direction?: 'horizontal' | 'vertical';
  className?: string;
  spacing?: number;
}

function getStyles(theme: VyzorixTheme, direction: 'horizontal' | 'vertical', spacing: number) {
  return {
    line: css({
      border: 'none',
      margin: 0,
      background: theme.colors.border.weak,
      flexShrink: 0,
      ...(direction === 'horizontal'
        ? { width: '100%', height: 1, marginTop: theme.spacing(spacing), marginBottom: theme.spacing(spacing) }
        : { width: 1, height: '100%', marginLeft: theme.spacing(spacing), marginRight: theme.spacing(spacing), alignSelf: 'stretch' }),
    }),
  };
}

export function Divider({ direction = 'horizontal', spacing = 1, className }: DividerProps) {
  const styles = useStyles2(getStyles, direction, spacing);
  return <hr className={`${styles.line} ${className ?? ''}`} aria-orientation={direction} />;
}
