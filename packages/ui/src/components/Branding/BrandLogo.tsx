import { css, cx } from '@emotion/css';
import { useStyles2, useTheme2 } from '../../theme/ThemeContext';
import { useBranding } from './BrandingContext';
import { type VyzorixTheme } from '../../theme/types';

export type BrandLogoSize = 'sm' | 'md' | 'lg' | 'xl';

export interface BrandLogoProps {
  source?: 'menu' | 'login' | 'loading';
  size?: BrandLogoSize;
  className?: string;

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

export function BrandLogo({ source = 'menu', size = 'md', className, alt }: BrandLogoProps) {
  const theme = useTheme2();
  const branding = useBranding();
  const styles = useStyles2(getStyles, size);

  const configured =
    source === 'login' ? branding.loginLogo : source === 'loading' ? branding.loadingLogo : branding.menuLogo;

  const src =
    configured || (theme.isDark ? '/brand/vyzorix_icon_on_dark.svg' : '/brand/vyzorix_icon_on_light.svg');

  return <img src={src} alt={alt ?? branding.appTitle} className={cx(styles.logo, className)} />;
}
