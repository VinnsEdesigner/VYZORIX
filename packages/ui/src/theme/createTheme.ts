import { createBreakpoints } from './breakpoints';
import { createColors, type ThemeColorsInput } from './createColors';
import { createShadows, type ThemeShadowsInput } from './createShadows';
import { createShape, type ThemeShapeInput } from './createShape';
import { createSpacing, type ThemeSpacingOptions } from './createSpacing';
import { createTransitions } from './createTransitions';
import { createTypography, type ThemeTypographyInput } from './createTypography';
import { type VyzorixTheme } from './types';
import { zIndex } from './zIndex';

export interface NewThemeOptions {
  name: string;
  id: string;
  colors?: ThemeColorsInput;
  spacing?: ThemeSpacingOptions;
  shadows?: ThemeShadowsInput;
  shape?: ThemeShapeInput;
  typography?: ThemeTypographyInput;
}

export function createTheme(options: NewThemeOptions): VyzorixTheme {
  const {
    name,
    colors: colorsInput = {},
    spacing: spacingInput = {},
    shadows: shadowsInput = {},
    shape: shapeInput = {},
    typography: typographyInput = {},
  } = options;

  const colors = createColors(colorsInput);
  const shape = createShape(shapeInput);
  const spacing = createSpacing(spacingInput);
  const typography = createTypography(typographyInput);
  const shadows = createShadows(colors, shadowsInput);

  return {
    name,
    isDark: colors.mode === 'dark',
    isLight: colors.mode === 'light',
    colors,
    breakpoints: createBreakpoints(),
    spacing,
    shape,
    typography,
    shadows,
    transitions: createTransitions(),
    zIndex: { ...zIndex },
    flags: {},
  };
}
