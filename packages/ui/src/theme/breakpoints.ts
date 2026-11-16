/**
 * Theme breakpoints.
 *
 * Breakpoints are not themeable, so they are a constant factory rather than an
 * input.
 */

export interface ThemeBreakpoints {
  values: Breakpoints;
  up: (key: Breakpoint) => string;
  down: (key: Breakpoint) => string;
}

export type Breakpoints = {
  xs: number;
  sm: number;
  md: number;
  lg: number;
  xl: number;
};

export type Breakpoint = keyof Breakpoints;

const values: Breakpoints = {
  xs: 0,
  sm: 544,
  md: 769,
  lg: 1025,
  xl: 1441,
};

export function createBreakpoints(): ThemeBreakpoints {
  return {
    values,
    up: (key) => `@media (min-width: ${values[key]}px)`,
    down: (key) => `@media (max-width: ${values[key]}px)`,
  };
}
