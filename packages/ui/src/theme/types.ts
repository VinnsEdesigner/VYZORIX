import { type ThemeBreakpoints } from './breakpoints';
import { type ThemeColors } from './createColors';
import { type ThemeShadows } from './createShadows';
import { type ThemeShape } from './createShape';
import { type ThemeSpacing } from './createSpacing';
import { type ThemeTransitions } from './createTransitions';
import { type ThemeTypography } from './createTypography';
import { type ThemeZIndices } from './zIndex';

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

  flags: {

    tabularNums?: boolean;
  } & Record<string, boolean | undefined>;
}

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

  shade?: string;

  transparent?: string;

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

export type DeepRequired<T> = Required<{
  [P in keyof T]: T[P] extends Required<T[P]> ? T[P] : DeepRequired<T[P]>;
}>;
