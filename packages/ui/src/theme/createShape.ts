/**
 * Theme shape (corner radii).
 *
 * Corner radii shared by every theme.
 */

const DEFAULT_BORDER_RADIUS_SM = 4;
const DEFAULT_BORDER_RADIUS_MD = 6;
const DEFAULT_BORDER_RADIUS_LG = 10;

export interface Radii {
  /** Use for most things (inputs, buttons, cards, panels). Same as `md`. */
  default: string;
  /** Same as `default`. */
  md: string;
  /** Smaller things like chips, tags and badges. */
  sm: string;
  /** Large things, like modals and containers. */
  lg: string;
  /** Maximum half-circle sides, e.g. for pills. */
  pill: string;
  circle: string;
}

export interface ThemeShape {
  /** @deprecated Use `theme.shape.radius.*` instead. */
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
