// @vyzorix/ui — theme system
//
// The theme is a plain object built by createTheme from a definition, delivered
// through React context, and consumed with useStyles2 (Emotion).
export { createTheme, type NewThemeOptions } from './createTheme';
export { createColors, type ThemeColors, type ThemeColorsInput, type ThemeColorsMode } from './createColors';
export { createShape, type ThemeShape, type Radii } from './createShape';
export { createSpacing, type ThemeSpacing } from './createSpacing';
export { createTypography, type ThemeTypography, type ThemeTypographyVariant } from './createTypography';
export { createShadows, type ThemeShadows } from './createShadows';
export { createTransitions, type ThemeTransitions } from './createTransitions';
export { createBreakpoints, type ThemeBreakpoints } from './breakpoints';
export { zIndex, type ThemeZIndices } from './zIndex';
export { palette } from './palette';
export {
  alpha,
  darken,
  lighten,
  emphasize,
  getContrastRatio,
  getLuminance,
  onBackground,
} from './colorManipulator';
export {
  type VyzorixTheme,
  type ThemeRichColor,
  type ThemeRichColorInput,
  type RichColorName,
} from './types';
export {
  getThemeById,
  getBuiltInThemes,
  themeIds,
  isThemeId,
  resolveThemeId,
  DEFAULT_THEME_ID,
  SYSTEM_THEME_ID,
  THEME_IDS,
  type ThemeId,
  type ThemeRegistryItem,
} from './registry';
export { ThemeContext, useTheme2, useStyles2, mockThemeContext } from './ThemeContext';
export { injectGlobalStyles, appShellStyles } from './globalStyles';
