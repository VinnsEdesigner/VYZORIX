import { css } from '@emotion/css';
import { type VyzorixTheme } from '../../../theme/types';

type CSSInterpolation = Parameters<typeof css>[0];

export type ResponsiveProp<T> = T | Responsive<T>;

type Responsive<T> = {
  xs: T;
  sm?: T;
  md?: T;
  lg?: T;
  xl?: T;
};

type BreakpointKey = 'xs' | 'sm' | 'md' | 'lg' | 'xl';

function breakpointCSS<T>(
  theme: VyzorixTheme,
  prop: Responsive<T>,
  getCSS: (val: T) => CSSInterpolation,
  key: BreakpointKey,
): CSSInterpolation {
  const value = prop[key];
  if (value !== undefined && value !== null) {
    return { [theme.breakpoints.up(key)]: getCSS(value) };
  }
  return undefined;
}

export function getResponsiveStyle<T>(
  theme: VyzorixTheme,
  prop: ResponsiveProp<T> | undefined,
  getCSS: (val: T) => CSSInterpolation,
): CSSInterpolation {
  if (prop === undefined || prop === null) {
    return null;
  }
  if (typeof prop !== 'object' || !('xs' in prop)) {
    return getCSS(prop);
  }

  return [
    breakpointCSS(theme, prop, getCSS, 'xs'),
    breakpointCSS(theme, prop, getCSS, 'sm'),
    breakpointCSS(theme, prop, getCSS, 'md'),
    breakpointCSS(theme, prop, getCSS, 'lg'),
    breakpointCSS(theme, prop, getCSS, 'xl'),
  ];
}
