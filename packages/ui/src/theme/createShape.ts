const DEFAULT_BORDER_RADIUS_SM = 4;
const DEFAULT_BORDER_RADIUS_MD = 6;
const DEFAULT_BORDER_RADIUS_LG = 10;

export interface Radii {

  default: string;

  md: string;

  sm: string;

  lg: string;

  pill: string;
  circle: string;
}

export interface ThemeShape {

  borderRadius: (amount?: number) => string;
  radius: Radii;
}

export interface ThemeShapeInput {
  borderRadiusSm?: number;
  borderRadius?: number;
  borderRadiusLg?: number;
}

export function createShape({
  borderRadiusSm = DEFAULT_BORDER_RADIUS_SM,
  borderRadius: borderRadiusMd = DEFAULT_BORDER_RADIUS_MD,
  borderRadiusLg = DEFAULT_BORDER_RADIUS_LG,
}: ThemeShapeInput = {}): ThemeShape {
  const radius: Radii = {
    default: `${borderRadiusMd}px`,
    sm: `${borderRadiusSm}px`,
    md: `${borderRadiusMd}px`,
    lg: `${borderRadiusLg}px`,
    pill: '9999px',
    circle: '100%',
  };

  const borderRadius = (amount?: number) => `${(amount ?? 1) * borderRadiusMd}px`;

  return { radius, borderRadius };
}
