import { css, cx } from '@emotion/css';
import { useStyles2, useTheme2 } from '../../theme/ThemeContext';
import { useBranding } from './BrandingContext';
import { type VyzorixTheme } from '../../theme/types';

export interface BrandWordmarkProps {

  height?: number;
  className?: string;
  alt?: string;
}

const getStyles = (_theme: VyzorixTheme, height: number) => ({
  wordmark: css({
    height,
    width: 'auto',
    display: 'block',
  }),
});

export function BrandWordmark({ height = 28, className, alt }: BrandWordmarkProps) {
  const theme = useTheme2();
  const branding = useBranding();
  const styles = useStyles2(getStyles, height);

  const src = theme.isDark ? '/brand/vyzorix_wordmark_dark.svg' : '/brand/vyzorix_wordmark_light.svg';

  return <img src={src} alt={alt ?? branding.appTitle} className={cx(styles.wordmark, className)} />;
}
