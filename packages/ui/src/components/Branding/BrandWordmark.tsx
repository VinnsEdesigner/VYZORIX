import { css, cx } from '@emotion/css';
import { useStyles2, useTheme2 } from '../../theme/ThemeContext';
import { useBranding } from './BrandingContext';
import { type VyzorixTheme } from '../../theme/types';

export interface BrandWordmarkProps {
  /** Cap height in px. Width scales with the artwork's aspect ratio. */
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

/**
 * The mark plus wordmark, in one asset.
 *
 * Two files exist — one for dark surfaces, one for light — and the active theme
 * picks between them, because a single-colour wordmark cannot read on both.
 */
export function BrandWordmark({ height = 28, className, alt }: BrandWordmarkProps) {
  const theme = useTheme2();
  const branding = useBranding();
  const styles = useStyles2(getStyles, height);

  const src = theme.isDark ? '/brand/vyzorix_wordmark_dark.svg' : '/brand/vyzorix_wordmark_light.svg';

  return <img src={src} alt={alt ?? branding.appTitle} className={cx(styles.wordmark, className)} />;
}
