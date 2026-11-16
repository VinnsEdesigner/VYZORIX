/**
 * Core theme types.
 *
 * The theme object shape: colour model, breakpoints, spacing, shape,
 * typography, shadows, transitions and z-indices.
 */

import { type ThemeBreakpoints } from './breakpoints';
import { type ThemeColors } from './createColors';
import { type ThemeShadows } from './createShadows';
import { type ThemeShape } from './createShape';
import { type ThemeSpacing } from './createSpacing';
import { type ThemeTransitions } from './createTransitions';
import { type ThemeTypography } from './createTypography';
import { type ThemeZIndices } from './zIndex';

/** The theme object delivered through context and consumed by useStyles2. */
export interface VyzorixTheme {
  name: string;
  isDark: boolean;
  isLight: boolean;
  colors: ThemeColors;
  breakpoints: ThemeBreakpoints;
  spacing: ThemeSpacing;
  shape: ThemeShape;
  typography: ThemeTypography;
  zIndex: ThemeZIndices;
  shadows: ThemeShadows;
  transitions: ThemeTransitions;
  /** Feature flags that might affect component appearance. */
  flags: {
    /** Use tabular numerals for legend and tooltip values. */
    tabularNums?: boolean;
  } & Record<string, boolean | undefined>;
}

/** The rich, resolved form of a semantic colour (primary, error, …). */
export interface ThemeRichColorInput {
  name?: RichColorName;
  main?: string;
  mainEmphasis?: string;
  background?: string;
  backgroundEmphasis?: string;
  text?: string;
  textEmphasis?: string;
  border?: string;
  borderEmphasis?: string;
  contrastText?: string;
  subtleBackground?: string;
  subtleBorder?: string;
  /** @deprecated use `mainEmphasis` */
  shade?: string;
  /** @deprecated use `background` */
  transparent?: string;
  /** @deprecated use `border` */
  borderTransparent?: string;
}

export type ThemeRichColor = Required<ThemeRichColorInput>;

export type RichColorName =
  | 'primary'
  | 'secondary'
  | 'tertiary'
  | 'accent'
  | 'info'
  | 'success'
  | 'error'
  | 'warning';

/** Recursively make every property required. */
export type DeepRequired<T> = Required<{
  [P in keyof T]: T[P] extends Required<T[P]> ? T[P] : DeepRequired<T[P]>;
}>;
