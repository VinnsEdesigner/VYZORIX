import { css, cx } from '@emotion/css';
import { useStyles2 } from '../../theme/ThemeContext';
import { useBranding } from './BrandingContext';
import { type VyzorixTheme } from '../../theme/types';

export type BrandLogoSize = 'sm' | 'md' | 'lg' | 'xl';

export interface BrandLogoProps {
  /** Which configured asset to show. Defaults to the navigation mark. */
  source?: 'menu' | 'login' | 'loading';
  size?: BrandLogoSize;
  className?: string;
  /** Override the accessible name; defaults to the app title. */
  alt?: string;
}

const SIZE: Record<BrandLogoSize, number> = { sm: 16, md: 24, lg: 40, xl: 64 };

function getStyles(_theme: VyzorixTheme, size: BrandLogoSize) {
  const px = SIZE[size];
  return {
    logo: css({
      width: px,
      height: px,
      objectFit: 'contain',
      display: 'block',
      flexShrink: 0,
    }),
  };
}

/**
 * The product mark.
 *
 * Rendered as an `<img>` rather than an inlined SVG so the browser caches it and
 * a deployment can point it at any URL. The accessible name comes from the app
 * title, so a screen reader announces the product rather than "image".
 */
export function BrandLogo({ source = 'menu', size = 'md', className, alt }: BrandLogoProps) {
  const branding = useBranding();
  const styles = useStyles2(getStyles, size);

  const src =
    source === 'login' ? branding.loginLogo : source === 'loading' ? branding.loadingLogo : branding.menuLogo;

  return <img src={src} alt={alt ?? branding.appTitle} className={cx(styles.logo, className)} />;
}
